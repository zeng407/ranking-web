package sitemap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeRepository struct {
	pages [][]PublicPost
	calls int
	err   error
	since time.Time
}

func (repository *fakeRepository) RecentPublicPostSerials(
	_ context.Context, since time.Time, _ int64, _ int,
) ([]PublicPost, error) {
	if repository.err != nil {
		return nil, repository.err
	}
	repository.since = since
	if repository.calls >= len(repository.pages) {
		return nil, nil
	}
	page := repository.pages[repository.calls]
	repository.calls++
	return page, nil
}

type fakeWriter struct {
	key         string
	body        []byte
	contentType string
	err         error
}

func (writer *fakeWriter) Put(_ context.Context, key string, body []byte, contentType string) (string, error) {
	if writer.err != nil {
		return "", writer.err
	}
	writer.key = key
	writer.body = append([]byte(nil), body...)
	writer.contentType = contentType
	return "https://file.test.local/" + key, nil
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var fixedNow = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

func newGenerator(t *testing.T, repository Repository, writer Writer) *Generator {
	t.Helper()
	generator, err := NewGenerator(Options{
		Repository:   repository,
		Writer:       writer,
		Logger:       quietLogger(),
		BaseURL:      "https://2pick.app/",
		ObjectKey:    "sitemap.xml",
		HomeImageURL: "https://file.2pick.app/og-image.jpeg",
		Now:          func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("NewGenerator() error = %v", err)
	}
	return generator
}

func TestNewGeneratorValidatesOptions(t *testing.T) {
	repository := &fakeRepository{}
	writer := &fakeWriter{}
	cases := map[string]Options{
		"no repository": {Writer: writer, BaseURL: "https://x", ObjectKey: "s.xml"},
		"no writer":     {Repository: repository, BaseURL: "https://x", ObjectKey: "s.xml"},
		"no base url":   {Repository: repository, Writer: writer, ObjectKey: "s.xml"},
		"no object key": {Repository: repository, Writer: writer, BaseURL: "https://x"},
	}
	for name, options := range cases {
		if _, err := NewGenerator(options); err == nil {
			t.Errorf("NewGenerator() should reject the %s case", name)
		}
	}
}

// Two URLs per post plus the home page, matching the original's game.show and
// game.rank pair.
func TestGenerateEmitsHomeAndTwoURLsPerPost(t *testing.T) {
	repository := &fakeRepository{pages: [][]PublicPost{{
		{PublicPostID: 1, PostSerial: "aaa111", UpdatedAt: fixedNow},
		{PublicPostID: 2, PostSerial: "bbb222", UpdatedAt: fixedNow},
	}}}
	writer := &fakeWriter{}
	generator := newGenerator(t, repository, writer)

	url, count, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if count != 5 {
		t.Fatalf("entries = %d, want 5 (home + 2 posts x 2)", count)
	}
	if url != "https://file.test.local/sitemap.xml" {
		t.Fatalf("stored url = %q", url)
	}
	if writer.contentType != "application/xml" {
		t.Fatalf("content type = %q", writer.contentType)
	}

	body := string(writer.body)
	for _, want := range []string{
		"<loc>https://2pick.app/</loc>",
		"<loc>https://2pick.app/g/aaa111</loc>",
		"<loc>https://2pick.app/r/aaa111</loc>",
		"<loc>https://2pick.app/g/bbb222</loc>",
		"<loc>https://2pick.app/r/bbb222</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %s", want)
		}
	}
	// A trailing slash on the configured base must not double up.
	if strings.Contains(body, "https://2pick.app//") {
		t.Error("base url trailing slash was not trimmed")
	}
}

// The home entry keeps its hourly frequency, priority 1.0 and og image.
func TestGenerateKeepsTheHomeEntryAttributes(t *testing.T) {
	writer := &fakeWriter{}
	generator := newGenerator(t, &fakeRepository{}, writer)

	if _, _, err := generator.Generate(context.Background()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	body := string(writer.body)
	for _, want := range []string{
		"<changefreq>hourly</changefreq>",
		"<priority>1.0</priority>",
		"<image:loc>https://file.2pick.app/og-image.jpeg</image:loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %s", want)
		}
	}
}

// The window is three months back from now, matching now()->subMonths(3).
func TestGenerateAsksForTheLastThreeMonths(t *testing.T) {
	repository := &fakeRepository{}
	generator := newGenerator(t, repository, &fakeWriter{})

	if _, _, err := generator.Generate(context.Background()); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	want := fixedNow.AddDate(0, -3, 0)
	if !repository.since.Equal(want) {
		t.Fatalf("since = %s, want %s", repository.since, want)
	}
}

// Cursor paging must keep walking until a page comes back empty.
func TestGenerateWalksEveryPage(t *testing.T) {
	repository := &fakeRepository{pages: [][]PublicPost{
		{{PublicPostID: 1, PostSerial: "a"}},
		{{PublicPostID: 2, PostSerial: "b"}},
		{{PublicPostID: 3, PostSerial: "c"}},
	}}
	generator := newGenerator(t, repository, &fakeWriter{})

	_, count, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	// home + 3 posts x 2
	if count != 7 {
		t.Fatalf("entries = %d, want 7", count)
	}
	if repository.calls != 3 {
		t.Fatalf("repository was called %d times, want 3", repository.calls)
	}
}

// A post with no serial has no reachable URL, so it is skipped rather than
// emitting "/g/".
func TestGenerateSkipsPostsWithoutASerial(t *testing.T) {
	repository := &fakeRepository{pages: [][]PublicPost{{
		{PublicPostID: 1, PostSerial: ""},
		{PublicPostID: 2, PostSerial: "   "},
		{PublicPostID: 3, PostSerial: "ok"},
	}}}
	writer := &fakeWriter{}
	generator := newGenerator(t, repository, writer)

	_, count, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("entries = %d, want 3 (home + one usable post x 2)", count)
	}
	if strings.Contains(string(writer.body), "<loc>https://2pick.app/g/</loc>") {
		t.Error("emitted a url with an empty serial")
	}
}

// Values are escaped, so a serial carrying XML metacharacters cannot break the
// document.
func TestRenderEscapesValues(t *testing.T) {
	body, err := Render([]Entry{{Location: "https://2pick.app/g/a&b<c>", ChangeFrequency: FrequencyDaily}})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	text := string(body)
	if strings.Contains(text, "a&b<c>") {
		t.Fatalf("metacharacters were not escaped: %s", text)
	}
	if !strings.Contains(text, "a&amp;b&lt;c&gt;") {
		t.Fatalf("expected escaped output, got: %s", text)
	}
}

func TestRenderProducesAValidDocumentShell(t *testing.T) {
	body, err := Render([]Entry{{Location: "https://2pick.app/", Priority: 1.0}})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	text := string(body)
	if !strings.HasPrefix(text, "<?xml") {
		t.Error("missing the xml declaration")
	}
	for _, want := range []string{
		`xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`,
		`xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"`,
		"<urlset", "</urlset>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Error("file should end with a newline")
	}
}

func TestRenderRejectsAnEmptySitemap(t *testing.T) {
	if _, err := Render(nil); err == nil {
		t.Fatal("Render(nil) should fail rather than write an empty sitemap")
	}
}

// lastmod is only emitted when known; a zero time would render as year 1.
func TestRenderOmitsUnknownLastModified(t *testing.T) {
	body, err := Render([]Entry{{Location: "https://2pick.app/"}})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(body), "<lastmod>") {
		t.Fatalf("lastmod should be omitted when unknown: %s", body)
	}
}

func TestGeneratePropagatesErrors(t *testing.T) {
	repositoryFailure := newGenerator(t, &fakeRepository{err: errors.New("connection reset")}, &fakeWriter{})
	if _, _, err := repositoryFailure.Generate(context.Background()); err == nil {
		t.Error("a repository failure must be reported")
	}

	writerFailure := newGenerator(t, &fakeRepository{}, &fakeWriter{err: errors.New("access denied")})
	if _, _, err := writerFailure.Generate(context.Background()); err == nil {
		t.Error("a storage failure must be reported")
	}
}
