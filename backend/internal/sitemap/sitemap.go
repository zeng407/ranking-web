// Package sitemap builds sitemap.xml.
//
// It replaces App\Console\Commands\GenerateSitemap.
//
// Two deliberate departures from the original.
//
// The crawler is gone. The original runs Spatie's SitemapGenerator against
// config('app.url') with setMaximumDepth(0), which fetches the home page and takes
// whatever links it finds there. Once the home page is the client-rendered SPA its
// links exist only after JavaScript runs, so a crawl would discover nothing; the
// URL set is enumerated explicitly instead.
//
// The output goes to the object store rather than public_path('sitemap.xml'). A Go
// worker has no Laravel public directory, and in the target architecture the
// frontend is static behind a CDN, so the file belongs in the bucket that serves
// it. The key is configurable, and the schedule stays disabled until the frontend
// is actually served from there.
package sitemap

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// RecentWindow matches now()->subMonths(3) in the original: only posts made public
// in the last three months are listed.
const RecentWindow = 3

// PostPageSize is the cursor page size when walking public posts.
const PostPageSize = 500

// MaxURLs guards the sitemap spec's 50,000-URL limit. Exceeding it needs a sitemap
// index, which this does not build, so the generator refuses rather than emitting a
// file search engines will reject.
const MaxURLs = 50000

// ChangeFrequency values used here.
const (
	FrequencyHourly = "hourly"
	FrequencyDaily  = "daily"
)

// Repository reads the posts that belong in the sitemap.
type Repository interface {
	// RecentPublicPostSerials returns the serials of posts made public on or after
	// `since`, with public_posts.id greater than afterID, ordered by that id.
	RecentPublicPostSerials(ctx context.Context, since time.Time, afterID int64, limit int) ([]PublicPost, error)
}

// PublicPost is one row of the walk.
type PublicPost struct {
	// PublicPostID is the cursor, not the post id.
	PublicPostID int64
	PostSerial   string
	UpdatedAt    time.Time
}

// Writer stores the rendered file. media.S3Store satisfies it.
type Writer interface {
	Put(ctx context.Context, key string, body []byte, contentType string) (string, error)
}

// Options configures the generator.
type Options struct {
	Repository Repository
	Writer     Writer
	Logger     *slog.Logger
	// BaseURL is the site origin the URLs are built from, matching
	// config('app.url').
	BaseURL string
	// ObjectKey is where the file is stored, for example "sitemap.xml".
	ObjectKey string
	// HomeImageURL is the og image the original attaches to the home entry. Empty
	// omits the image element.
	HomeImageURL string
	// Now is injectable for tests.
	Now func() time.Time
}

type Generator struct {
	repository   Repository
	writer       Writer
	logger       *slog.Logger
	baseURL      string
	objectKey    string
	homeImageURL string
	now          func() time.Time
}

func NewGenerator(options Options) (*Generator, error) {
	if options.Repository == nil {
		return nil, fmt.Errorf("sitemap: repository is required")
	}
	if options.Writer == nil {
		return nil, fmt.Errorf("sitemap: writer is required")
	}
	if strings.TrimSpace(options.BaseURL) == "" {
		return nil, fmt.Errorf("sitemap: base url is required")
	}
	if strings.TrimSpace(options.ObjectKey) == "" {
		return nil, fmt.Errorf("sitemap: object key is required")
	}
	clock := options.Now
	if clock == nil {
		clock = time.Now
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Generator{
		repository:   options.Repository,
		writer:       options.Writer,
		logger:       logger,
		baseURL:      strings.TrimRight(strings.TrimSpace(options.BaseURL), "/"),
		objectKey:    strings.TrimPrefix(strings.TrimSpace(options.ObjectKey), "/"),
		homeImageURL: strings.TrimSpace(options.HomeImageURL),
		now:          func() time.Time { return clock() },
	}, nil
}

// Entry is one URL in the sitemap.
type Entry struct {
	Location        string
	LastModified    time.Time
	ChangeFrequency string
	Priority        float64
	ImageURL        string
}

// Generate builds the sitemap and stores it, returning the URL it was written to
// and how many entries it holds.
func (generator *Generator) Generate(ctx context.Context) (string, int, error) {
	entries, err := generator.collect(ctx)
	if err != nil {
		return "", 0, err
	}

	body, err := Render(entries)
	if err != nil {
		return "", 0, err
	}

	storedURL, err := generator.writer.Put(ctx, generator.objectKey, body, "application/xml")
	if err != nil {
		return "", 0, fmt.Errorf("sitemap: store %q: %w", generator.objectKey, err)
	}

	generator.logger.Info("sitemap_generated",
		"url", storedURL,
		"key", generator.objectKey,
		"entries", len(entries),
		"bytes", len(body),
	)
	return storedURL, len(entries), nil
}

// collect enumerates the URL set: the home page, then a game and a rank page per
// recently public post.
func (generator *Generator) collect(ctx context.Context) ([]Entry, error) {
	entries := []Entry{{
		Location:        generator.baseURL + "/",
		ChangeFrequency: FrequencyHourly,
		Priority:        1.0,
		ImageURL:        generator.homeImageURL,
	}}

	since := generator.now().AddDate(0, -RecentWindow, 0)
	var afterID int64

	for {
		posts, err := generator.repository.RecentPublicPostSerials(ctx, since, afterID, PostPageSize)
		if err != nil {
			return nil, fmt.Errorf("sitemap: list public posts after %d: %w", afterID, err)
		}
		if len(posts) == 0 {
			break
		}

		for _, post := range posts {
			afterID = post.PublicPostID
			serial := strings.TrimSpace(post.PostSerial)
			if serial == "" {
				// A post with no serial has no reachable URL.
				generator.logger.Warn("sitemap_skipped_post_without_serial", "public_post_id", post.PublicPostID)
				continue
			}

			// The short URLs Laravel serves: g/{serial} and r/{serial}.
			for _, path := range []string{"/g/" + serial, "/r/" + serial} {
				entries = append(entries, Entry{
					Location:        generator.baseURL + path,
					LastModified:    post.UpdatedAt,
					ChangeFrequency: FrequencyDaily,
					Priority:        0.8,
				})
			}
		}

		if len(entries) > MaxURLs {
			// Refusing beats emitting a file search engines reject. Splitting into a
			// sitemap index is a separate change.
			return nil, fmt.Errorf(
				"sitemap: %d urls exceeds the %d limit; a sitemap index is needed", len(entries), MaxURLs)
		}
	}
	return entries, nil
}

// XML shapes. The image element is the Google image sitemap extension, which the
// original adds through Spatie's addImage.
type urlSet struct {
	XMLName    xml.Name  `xml:"urlset"`
	Namespace  string    `xml:"xmlns,attr"`
	ImageNS    string    `xml:"xmlns:image,attr"`
	URLEntries []urlItem `xml:"url"`
}

type urlItem struct {
	Location        string     `xml:"loc"`
	LastModified    string     `xml:"lastmod,omitempty"`
	ChangeFrequency string     `xml:"changefreq,omitempty"`
	Priority        string     `xml:"priority,omitempty"`
	Image           *imageItem `xml:"image:image,omitempty"`
}

type imageItem struct {
	Location string `xml:"image:loc"`
}

// Render produces the XML document. encoding/xml escapes the values, so a serial
// or a URL containing an ampersand cannot break the file.
func Render(entries []Entry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("sitemap: refusing to render an empty sitemap")
	}

	document := urlSet{
		Namespace: "http://www.sitemaps.org/schemas/sitemap/0.9",
		ImageNS:   "http://www.google.com/schemas/sitemap-image/1.1",
	}
	for _, entry := range entries {
		item := urlItem{
			Location:        entry.Location,
			ChangeFrequency: entry.ChangeFrequency,
		}
		if !entry.LastModified.IsZero() {
			// W3C datetime, which is what the sitemap spec asks for.
			item.LastModified = entry.LastModified.UTC().Format(time.RFC3339)
		}
		if entry.Priority > 0 {
			item.Priority = fmt.Sprintf("%.1f", entry.Priority)
		}
		if entry.ImageURL != "" {
			item.Image = &imageItem{Location: entry.ImageURL}
		}
		document.URLEntries = append(document.URLEntries, item)
	}

	encoded, err := xml.MarshalIndent(document, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("sitemap: encode xml: %w", err)
	}
	return append([]byte(xml.Header), append(encoded, '\n')...), nil
}
