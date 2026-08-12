package ingest

import (
	"context"
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Deciding what a pasted URL is, from Services\ElementSourceGuess.
//
// TWO DIFFERENCES FROM THE ORIGINAL, BOTH DELIBERATE.
//
// It reordered its own priority list after every match, moving the winner to the front,
// and kept that order on the injected instance for the rest of the batch. For a URL that
// two guesses would both accept, which one won depended on what had been pasted before it
// in the same request. The order here is fixed.
//
// Its first guess called getimagesize() on the URL — a full download of the image header
// from wherever it points — before anything had checked where that was. The order below
// puts every pattern match first, so the network is touched only for a URL that nothing
// recognised, and only through a fetcher that refuses private addresses.

// Source is what a URL turned out to be.
type Source string

const (
	SourceImage        Source = "image"
	SourceYouTube      Source = "youtube"
	SourceYouTubeEmbed Source = "youtube_embed"
	SourceBilibili     Source = "bilibili_video"
	SourceTwitch       Source = "twitch"
	SourceVideoFile    Source = "url"
	SourceUnknown      Source = ""
)

// Classification is a decided URL.
type Classification struct {
	Source Source
	// ID is the video id for the sources that have one.
	ID string
	// Size is what the probe reported, or 0 when nothing was probed and -1 when the
	// source would not say.
	Size int64
}

// Prober answers what a URL serves, for the cases no pattern settles. It must refuse
// private addresses: these URLs come from anyone with an account.
//
// It reports the size as well as the type because the same HEAD answers both questions,
// and a direct video is accepted on the strength of that one request — downloading it to
// find out how big it is would be the thing the limit exists to prevent.
type Prober interface {
	// Probe returns the content type and length the URL reports. An unreadable URL
	// answers with an empty type, and an unknown length with -1.
	Probe(ctx context.Context, rawURL string) (contentType string, size int64)
}

var (
	// imageExtensions is the original's list. Note it did not include webp or jpeg,
	// which is why those only ever matched through the network probe.
	imageExtensions = map[string]bool{".jpg": true, ".png": true, ".gif": true}

	youTubeEmbedPattern = regexp.MustCompile(
		`^<iframe[\s\S]*?src="(https://www\.youtube\.com/embed/[^"]+)"[\s\S]*?</iframe>$`)
	youTubeEmbedID = regexp.MustCompile(`^[A-Za-z0-9?&;=_-]+$`)
	// youTubeHost is every host the site's own data uses. The original's regex allowed
	// only www. and m., which would have refused the 4,909 elements that came in through
	// music.youtube.com — those got through because parseVideoId read ?v= before it
	// looked at the host at all.
	youTubeHost = regexp.MustCompile(`(^|\.)(youtube(-nocookie)?\.com|youtu\.be)$`)
	youTubeID   = regexp.MustCompile(`^[\w-]{11}$`)

	bilibiliHost = regexp.MustCompile(`(^|\.)bilibili\.com$`)
	twitchHost   = regexp.MustCompile(`(^|\.)twitch\.tv$`)
	bilibiliID   = regexp.MustCompile(`/video/([\w-]+)`)
)

// Classify decides what a pasted value is.
//
// The prober may be nil, in which case a URL that no pattern recognises is unknown rather
// than fetched.
func Classify(ctx context.Context, value string, prober Prober) Classification {
	value = strings.TrimSpace(value)
	if value == "" {
		return Classification{Source: SourceUnknown}
	}

	// An embed is a block of HTML, not a URL, so it is settled before anything tries to
	// parse one.
	if id, ok := YouTubeEmbedID(value); ok {
		return Classification{Source: SourceYouTubeEmbed, ID: id}
	}

	if id, ok := YouTubeVideoID(value); ok {
		return Classification{Source: SourceYouTube, ID: id}
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return Classification{Source: SourceUnknown}
	}
	host := strings.ToLower(parsed.Hostname())

	switch {
	case bilibiliHost.MatchString(host):
		return Classification{Source: SourceBilibili, ID: bilibiliVideoID(parsed.Path)}
	case twitchHost.MatchString(host):
		return Classification{Source: SourceTwitch}
	}

	// The extension, before the network. A query string is already off it because this
	// reads the parsed path rather than the raw string — which is what the original's
	// pathinfo() did not do, leaving it to compare "jpg?w=100" against its list.
	if imageExtensions[strings.ToLower(path.Ext(parsed.Path))] {
		return Classification{Source: SourceImage}
	}

	if prober == nil {
		return Classification{Source: SourceUnknown}
	}
	contentType, size := prober.Probe(ctx, value)
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return Classification{Source: SourceImage, Size: size}
	case strings.HasPrefix(contentType, "video/"):
		return Classification{Source: SourceVideoFile, Size: size}
	}
	return Classification{Source: SourceUnknown}
}

// YouTubeEmbedID reads the video id out of an <iframe> embed block.
//
// The original required the whole value to be one iframe and capped the src at 120
// characters, both of which are kept: the value is pasted by a user and ends up in a page
// this site serves.
func YouTubeEmbedID(value string) (string, bool) {
	matches := youTubeEmbedPattern.FindStringSubmatch(strings.TrimSpace(value))
	if matches == nil {
		return "", false
	}
	source := strings.TrimPrefix(matches[1], "https://www.youtube.com/embed/")
	if source == "" || len(source) > 120 || !youTubeEmbedID.MatchString(source) {
		return "", false
	}
	// Everything before the query is the id; the rest is the si/clip parameters YouTube
	// adds to a share link.
	return strings.SplitN(source, "?", 2)[0], true
}

// YouTubeVideoID reads a video id out of the forms YoutubeService::parseVideoId accepted:
// a bare id, ?v=, youtu.be/, /embed/, /v/, /shorts/ and /clip/.
//
// THE HOST IS CHECKED, WHICH THE ORIGINAL DID NOT DO. Its first move was to read ?v= out
// of the query whatever the URL was, and to return it if present — so pasting
// https://cdn.example.com/clip.mp4?v=7 produced a YouTube element with the id "7", which
// then failed its metadata lookup and reported the URL as broken. Requiring a YouTube
// host removes that, and widening the host set to any *.youtube.com is what keeps
// music.youtube.com — 4,909 elements — working, since that is the case the original was
// accidentally serving through the query shortcut.
func YouTubeVideoID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if youTubeID.MatchString(value) {
		// A bare eleven-character id, which the original accepted by length alone.
		return value, true
	}

	parsed, err := url.Parse(value)
	if err != nil || !youTubeHost.MatchString(strings.ToLower(parsed.Hostname())) {
		return "", false
	}
	if identifier := parsed.Query().Get("v"); identifier != "" {
		return identifier, true
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	last := segments[len(segments)-1]
	// A clip id is not a video id — the clip page has to be read to find the video — but
	// the original returned it here all the same, and the metadata lookup is what fails
	// on it. Returning it keeps that behaviour rather than inventing a new refusal.
	if last == "" {
		return "", false
	}
	return last, true
}

func bilibiliVideoID(urlPath string) string {
	matches := bilibiliID.FindStringSubmatch(urlPath)
	if matches == nil {
		return ""
	}
	return matches[1]
}

// SplitTitledURLs turns the batch field into its parts.
//
// The original accepted one comma-separated string where each entry could be "url title",
// split on the first space. That shape is kept: the old editor's textarea posts exactly
// this, and the SPA will too until there is a reason to change both at once.
func SplitTitledURLs(value string) []TitledURL {
	entries := make([]TitledURL, 0, 8)
	seen := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		url, title, _ := strings.Cut(part, " ")
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		entries = append(entries, TitledURL{URL: url, Title: strings.TrimSpace(title)})
	}
	return entries
}

// TitledURL is one entry from the batch field.
type TitledURL struct {
	URL   string
	Title string
}
