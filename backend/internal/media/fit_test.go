package media

import (
	"net"
	"testing"
)

// The fit must match ImageThumbnailService's arithmetic exactly: the resulting
// dimensions go into both the output pixels and the storage path, which embeds the
// size.
func TestFitBox(t *testing.T) {
	cases := []struct {
		name                          string
		originalWidth, originalHeight int
		maxWidth, maxHeight           int
		wantWidth, wantHeight         int
	}{
		{"square into square", 1000, 1000, 400, 400, 400, 400},
		{"landscape into square", 1600, 900, 400, 400, 400, 225},
		{"portrait into square", 900, 1600, 400, 400, 225, 400},
		// The example in the project notes: .../low/267x400/...
		{"portrait 2:3 into 400 box", 800, 1200, 400, 400, 267, 400},
		{"already smaller is still scaled up", 100, 50, 400, 400, 400, 200},
		{"medium box", 1600, 900, 800, 800, 800, 450},
		// Rounding: 1000/3 = 333.33 -> 333
		{"rounds down", 3000, 1000, 1000, 1000, 1000, 333},
		// 1000*2/3 = 666.67 -> 667, so PHP's round() half-away-from-zero, not
		// truncation.
		{"rounds up", 2000, 3000, 1000, 1000, 667, 1000},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			width, height := FitBox(test.originalWidth, test.originalHeight, test.maxWidth, test.maxHeight)
			if width != test.wantWidth || height != test.wantHeight {
				t.Fatalf("FitBox(%d, %d, %d, %d) = %dx%d, want %dx%d",
					test.originalWidth, test.originalHeight, test.maxWidth, test.maxHeight,
					width, height, test.wantWidth, test.wantHeight)
			}
		})
	}
}

// One dimension always equals the corresponding maximum, and the aspect ratio is
// preserved within a pixel of rounding.
func TestFitBoxPreservesRatioAndTouchesTheBox(t *testing.T) {
	maxWidth, maxHeight := 400, 400
	for originalWidth := 50; originalWidth <= 2000; originalWidth += 37 {
		for originalHeight := 50; originalHeight <= 2000; originalHeight += 53 {
			width, height := FitBox(originalWidth, originalHeight, maxWidth, maxHeight)
			if width > maxWidth || height > maxHeight {
				t.Fatalf("FitBox(%d,%d) = %dx%d exceeds the box", originalWidth, originalHeight, width, height)
			}
			if width != maxWidth && height != maxHeight {
				t.Fatalf("FitBox(%d,%d) = %dx%d touches neither maximum", originalWidth, originalHeight, width, height)
			}
			original := float64(originalWidth) / float64(originalHeight)
			fitted := float64(width) / float64(height)

			// Tolerance has to scale with the smaller dimension. Rounding moves it
			// by at most half a pixel, so the relative ratio error is about
			// 0.5/smaller — 0.1% at 400px but 3% at 16px, which is what an extreme
			// aspect ratio like 50x1216 produces. A flat percentage would either
			// pass everything or fail correct output.
			smaller := width
			if height < smaller {
				smaller = height
			}
			tolerance := 1 + 0.6/float64(smaller)
			if fitted/original > tolerance || original/fitted > tolerance {
				t.Fatalf("FitBox(%d,%d) = %dx%d distorts the ratio %.4f -> %.4f beyond %.4f",
					originalWidth, originalHeight, width, height, original, fitted, tolerance)
			}
		}
	}
}

func TestFitBoxRejectsNonPositiveInput(t *testing.T) {
	for _, test := range [][4]int{
		{0, 100, 400, 400}, {100, 0, 400, 400}, {100, 100, 0, 400}, {100, 100, 400, 0},
		{-1, 100, 400, 400},
	} {
		if width, height := FitBox(test[0], test[1], test[2], test[3]); width != 0 || height != 0 {
			t.Errorf("FitBox%v = %dx%d, want 0x0", test, width, height)
		}
	}
}

// The SSRF block the original leaves as a comment. These URLs are user-submitted
// and the worker runs inside the VPC.
func TestIsPublicIPRejectsInternalAddresses(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"::1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		// The cloud metadata endpoint, the address this control exists for.
		"169.254.169.254",
		"0.0.0.0",
		"224.0.0.1",
		"fe80::1",
		"fc00::1",
		// Carrier-grade NAT, used by some providers for internal services.
		"100.64.0.1",
		"100.127.255.255",
	}
	for _, address := range blocked {
		parsed := net.ParseIP(address)
		if parsed == nil {
			t.Fatalf("test address %q does not parse", address)
		}
		if IsPublicIP(parsed) {
			t.Errorf("IsPublicIP(%s) = true, want false", address)
		}
	}

	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946", "99.99.99.99", "100.63.255.255", "100.128.0.1"}
	for _, address := range allowed {
		parsed := net.ParseIP(address)
		if parsed == nil {
			t.Fatalf("test address %q does not parse", address)
		}
		if !IsPublicIP(parsed) {
			t.Errorf("IsPublicIP(%s) = false, want true", address)
		}
	}
}

func TestIsPublicIPRejectsNil(t *testing.T) {
	if IsPublicIP(nil) {
		t.Fatal("IsPublicIP(nil) = true, want false")
	}
}

func TestValidateSourceURLRejectsBadSchemesAndHosts(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"whitespace":    "   ",
		"file scheme":   "file:///etc/passwd",
		"gopher scheme": "gopher://example.com/",
		"no scheme":     "example.com/image.png",
		"no host":       "http://",
		"loopback name": "http://localhost/image.png",
		"loopback ip":   "http://127.0.0.1/image.png",
		"metadata ip":   "http://169.254.169.254/latest/meta-data/",
		"private ip":    "http://10.1.2.3/image.png",
		"unparseable":   "http://[::1",
	}
	for name, rawURL := range cases {
		if err := ValidateSourceURL(rawURL); err == nil {
			t.Errorf("%s: ValidateSourceURL(%q) should fail", name, rawURL)
		}
	}
}

// The escape hatch must be off unless explicitly set, and must still validate the
// scheme when it is on. Without it nothing can exercise the media path locally,
// because a fixture served from a container is on a private address.
func TestAllowPrivateSourcesEscapeHatch(t *testing.T) {
	// Default: blocked.
	if err := ValidateSourceURL("http://127.0.0.1/image.png"); err == nil {
		t.Fatal("a private address must be blocked by default")
	}

	t.Setenv(AllowPrivateSourcesEnv, "true")
	if err := ValidateSourceURL("http://127.0.0.1/image.png"); err != nil {
		t.Fatalf("with the hatch on, a private address should be allowed: %v", err)
	}
	// The scheme check is not part of the hatch.
	if err := ValidateSourceURL("file:///etc/passwd"); err == nil {
		t.Fatal("the hatch must not allow a non-http scheme")
	}
	if err := ValidateSourceURL("http://"); err == nil {
		t.Fatal("the hatch must not allow a missing host")
	}

	t.Setenv(AllowPrivateSourcesEnv, "false")
	if err := ValidateSourceURL("http://127.0.0.1/image.png"); err == nil {
		t.Fatal("explicitly false must block again")
	}
}

// The warning predicate must follow the value, not the presence of the variable.
// compose sets it to "false" by default, so warning on presence would fire on
// every production start and be ignored by the time it mattered.
func TestPrivateSourcesAllowedFollowsTheValue(t *testing.T) {
	t.Setenv(AllowPrivateSourcesEnv, "false")
	if PrivateSourcesAllowed() {
		t.Error(`"false" must not count as allowed`)
	}
	t.Setenv(AllowPrivateSourcesEnv, "")
	if PrivateSourcesAllowed() {
		t.Error("unset must not count as allowed")
	}
	for _, value := range []string{"true", "1", "on", "TRUE"} {
		t.Setenv(AllowPrivateSourcesEnv, value)
		if !PrivateSourcesAllowed() {
			t.Errorf("%q must count as allowed", value)
		}
	}
}
