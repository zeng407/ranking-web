package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// FetchTimeout matches the 10 second timeout on the Laravel HTTP calls.
const FetchTimeout = 10 * time.Second

// MaxSourceBytes caps a download. The original has no limit at all: it reads the
// whole body into memory with file_get_contents, so one oversized element could
// exhaust the worker. A cap turns that into one failed job.
const MaxSourceBytes = 64 << 20 // 64 MiB

var (
	// ErrEmptySource means the URL served zero bytes. The caller falls back to the
	// element's source_url, matching the original's behaviour.
	ErrEmptySource = errors.New("media: source returned no bytes")
	// ErrBlockedURL means the URL is not safe to fetch.
	ErrBlockedURL = errors.New("media: url is not allowed")
	// ErrTooLarge means the source exceeded MaxSourceBytes.
	ErrTooLarge = errors.New("media: source exceeds the size limit")
)

// AllowPrivateSourcesEnv opts out of the private-address block.
//
// It exists because the block makes local media testing impossible: a fixture
// served from a Docker network or from localhost is on a private address, so
// nothing can exercise the fetch, probe, encode and upload path end to end without
// reaching the public internet.
//
// It must never be set in production. The whole point of the block is that these
// URLs are user-submitted and the worker runs inside the VPC, so switching it off
// restores the SSRF primitive the block exists to remove. The worker logs loudly
// when it is on.
const AllowPrivateSourcesEnv = "MEDIA_ALLOW_PRIVATE_SOURCES"

// PrivateSourcesAllowed reports whether the escape hatch is on, so a caller can
// warn about it.
//
// It tests the value, not merely the presence of the variable: compose sets it to
// "false" by default, and a warning that fired on every start would be ignored by
// the time it mattered.
func PrivateSourcesAllowed() bool {
	return privateSourcesAllowed()
}

// privateSourcesAllowed reads the escape hatch. It is read per call rather than
// cached so a test can set it with t.Setenv.
func privateSourcesAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(AllowPrivateSourcesEnv))) {
	case "true", "1", "on":
		return true
	default:
		return false
	}
}

// ValidateSourceURL checks a user-supplied media URL before the worker fetches it.
//
// The original validates only the scheme and leaves the rest as a comment:
// "Optionally, add more SSRF protections here (e.g., block private IPs)". These
// URLs come from user submissions and the worker runs inside the VPC, so an
// unvalidated fetch is a server-side request forgery primitive against anything
// reachable from there, including cloud metadata endpoints. This implements the
// block that comment describes.
//
// Resolution happens here and the address is checked, but the dialler re-checks
// each connection: between this call and the dial, DNS could return a different
// answer, so the control has to live in the transport too.
func ValidateSourceURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return fmt.Errorf("%w: empty", ErrBlockedURL)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("%w: unparseable", ErrBlockedURL)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: scheme %q", ErrBlockedURL, parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: no host", ErrBlockedURL)
	}

	if privateSourcesAllowed() {
		// Scheme and host are still validated; only the address check is skipped.
		return nil
	}

	addresses, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve %q", ErrBlockedURL, host)
	}
	for _, address := range addresses {
		if !IsPublicIP(address) {
			return fmt.Errorf("%w: %q resolves to non-public address %s", ErrBlockedURL, host, address)
		}
	}
	return nil
}

// IsPublicIP reports whether an address is safe for the worker to connect to.
//
// Loopback, private, link-local (which covers 169.254.169.254, the cloud metadata
// endpoint), multicast and unspecified addresses are all rejected.
func IsPublicIP(address net.IP) bool {
	if address == nil || address.IsUnspecified() || address.IsLoopback() {
		return false
	}
	if address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
		return false
	}
	if address.IsMulticast() || address.IsInterfaceLocalMulticast() {
		return false
	}
	// Carrier-grade NAT, 100.64.0.0/10. Not covered by IsPrivate and used by some
	// cloud providers for internal services.
	if v4 := address.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return false
	}
	// IPv4-mapped IPv6 of a blocked range would already be caught above via To4.
	return true
}

// Fetcher downloads media sources.
type Fetcher struct {
	client *http.Client
}

// NewSafeTransport builds the transport whose dialler refuses non-public addresses, so a
// hostname that resolves differently after validation still cannot be reached.
//
// Exported because more than one thing needs it: the fetcher below downloads media with
// it, and the ingest package's HEAD prober asks what a pasted URL serves. Two copies of a
// dialler that decides what is reachable would be two places for the rule to drift.
func NewSafeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: FetchTimeout}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if !privateSourcesAllowed() {
				if parsed := net.ParseIP(host); parsed != nil && !IsPublicIP(parsed) {
					return nil, fmt.Errorf("%w: dial to non-public address %s", ErrBlockedURL, host)
				}
			}
			return dialer.DialContext(ctx, network, address)
		},
		TLSHandshakeTimeout: FetchTimeout,
	}
}

// NewFetcher builds a fetcher over that transport.
func NewFetcher() *Fetcher {
	transport := NewSafeTransport()
	return &Fetcher{
		client: &http.Client{
			Transport: transport,
			Timeout:   FetchTimeout,
			// Each hop of a redirect chain must be validated too, otherwise a public
			// URL can redirect the worker to a private one.
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("media: too many redirects")
				}
				return ValidateSourceURL(request.URL.String())
			},
		},
	}
}

// Fetch downloads a source into memory.
//
// It returns ErrEmptySource for a zero-length body, which the caller treats as
// "use source_url instead" rather than as a failure. That mirrors the original,
// which checks both the Content-Length of a HEAD and the size of the downloaded
// file.
func (fetcher *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	if err := ValidateSourceURL(rawURL); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("media: build request: %w", err)
	}

	response, err := fetcher.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("media: fetch %q: %w", rawURL, err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("media: fetch %q: status %d", rawURL, response.StatusCode)
	}

	// One byte over the cap is read so the limit can be distinguished from a body
	// that happens to be exactly MaxSourceBytes.
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxSourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("media: read %q: %w", rawURL, err)
	}
	if len(body) > MaxSourceBytes {
		return nil, fmt.Errorf("%w: %q", ErrTooLarge, rawURL)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrEmptySource, rawURL)
	}
	return body, nil
}
