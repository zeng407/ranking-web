package ingest

import (
	"context"
	"strings"
)

// Adding media by URL, from Api\ElementController::batchCreate and the source handlers
// behind ElementService::massStore.
//
// Imgur is not here: it is 0 rows and was removed from the project. Twitch is not here
// either — 42 elements a year, and its metadata needs an app token this deployment has no
// credentials for. Both are recorded decisions rather than omissions.

// MaxURLsPerRequest is config/setting.php's upload_url_at_a_time.
const MaxURLsPerRequest = 100

// Video sources, matching the elements.video_source column.
const (
	VideoSourceYouTube      = "youtube"
	VideoSourceYouTubeEmbed = "youtube_embed"
	VideoSourceBilibili     = "bilibili_video"
)

// Why one URL in a batch did not become an element.
const (
	ReasonUnrecognised = "unrecognised"
	ReasonTooLarge     = "too_large"
	ReasonUnavailable  = "unavailable"
	ReasonPostFull     = "post_full"
)

// BatchResult is what a batch did, URL by URL.
//
// PARTIAL SUCCESS IS THE NORMAL OUTCOME, not an error. An author pasting thirty links
// will have one that is dead or private, and failing the whole request over it would
// throw away the other twenty-nine. The original reported this way too — it collected the
// failures and only refused outright when nothing at all could be stored.
type BatchResult struct {
	Added  []Stored
	Failed []FailedURL
}

// FailedURL names one URL that did not make it, and why.
type FailedURL struct {
	URL    string
	Reason string
}

// VideoInfo is what a video's own service says about it.
type VideoInfo struct {
	Title    string
	ThumbURL string
	// DurationSecs is 0 when the source does not report one.
	DurationSecs int
}

// YouTubeLookup reads a video's metadata. Nil disables YouTube ingestion, which is the
// state of a deployment with no API key.
type YouTubeLookup interface {
	Video(ctx context.Context, videoID string) (VideoInfo, error)
}

// PageLookup reads the title and preview image out of a page. Bilibili has no public
// metadata API, so its handler scraped the page and this does the same.
type PageLookup interface {
	Page(ctx context.Context, rawURL string) (VideoInfo, error)
}

// Fetcher downloads a URL. It must refuse private addresses: these URLs are supplied by
// anyone with an account, so the fetch is an SSRF surface. media.Fetcher satisfies it.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// AddURLs turns a pasted list into elements.
//
// The list is the original's format: comma-separated, each entry optionally "url title"
// split on the first space.
func (service *Service) AddURLs(
	ctx context.Context, userID int64, serial, list string,
) (BatchResult, error) {
	entries := batchEntries(list)
	if len(entries) == 0 {
		return BatchResult{}, invalid("urls", CodeRequired)
	}
	if len(entries) > MaxURLsPerRequest {
		return BatchResult{}, invalid("urls", CodeTooMany)
	}

	postID, existing, err := service.store.PostForOwner(ctx, userID, serial)
	if err != nil {
		return BatchResult{}, err
	}

	result := BatchResult{Added: []Stored{}, Failed: []FailedURL{}}
	for _, entry := range entries {
		// Counted as they are added rather than re-read: the cap is the post's, and this
		// request is the only thing adding to it.
		if existing+len(result.Added) >= MaxElements {
			result.Failed = append(result.Failed, FailedURL{URL: entry.URL, Reason: ReasonPostFull})
			continue
		}

		element, reason := service.describe(ctx, postID, serial, entry)
		if reason != "" {
			result.Failed = append(result.Failed, FailedURL{URL: entry.URL, Reason: reason})
			continue
		}

		stored, err := service.store.CreateElement(ctx, element)
		if err != nil {
			return BatchResult{}, err
		}
		result.Added = append(result.Added, stored)

		if element.Type == TypeVideo && service.thumbs != nil {
			// Only a stored video file gets a generated thumbnail; a remote video already
			// has one from its own service.
			if element.Path != "" {
				_ = service.thumbs.VideoThumbnail(ctx, stored.ID)
			}
		}
	}
	return result, nil
}

// batchEntries splits the field into what should be ingested.
//
// AN EMBED IS THE WHOLE FIELD, NOT ONE ENTRY IN A LIST. It is a block of HTML full of
// spaces, and the list format splits each entry on its first space to separate a URL from
// a title — so an iframe run through the splitter yields "<iframe" and nothing usable.
// The original checked the entire input for an embed before it split anything, and
// returned that single element when it matched; this does the same.
func batchEntries(list string) []TitledURL {
	if _, ok := YouTubeEmbedID(list); ok {
		return []TitledURL{{URL: strings.TrimSpace(list)}}
	}
	return SplitTitledURLs(list)
}

// describe turns one URL into the row it should become, or the reason it cannot.
func (service *Service) describe(
	ctx context.Context, postID int64, serial string, entry TitledURL,
) (NewElement, string) {
	class := Classify(ctx, entry.URL, service.prober)

	element := NewElement{PostID: postID, SourceURL: entry.URL, Title: TrimTitle(entry.Title)}

	switch class.Source {
	case SourceYouTube, SourceYouTubeEmbed:
		if service.youtube == nil {
			return NewElement{}, ReasonUnavailable
		}
		info, err := service.youtube.Video(ctx, class.ID)
		if err != nil {
			return NewElement{}, ReasonUnavailable
		}
		element.Type = TypeVideo
		element.VideoID = class.ID
		element.VideoSource = VideoSourceYouTube
		if class.Source == SourceYouTubeEmbed {
			element.VideoSource = VideoSourceYouTubeEmbed
		}
		element.ThumbURL = info.ThumbURL
		if element.Title == "" {
			element.Title = TrimTitle(info.Title)
		}
		if info.DurationSecs > 0 {
			duration := info.DurationSecs
			element.DurationSecs = &duration
		}
		return element, ""

	case SourceBilibili:
		if service.pages == nil {
			return NewElement{}, ReasonUnavailable
		}
		info, err := service.pages.Page(ctx, entry.URL)
		if err != nil {
			return NewElement{}, ReasonUnavailable
		}
		element.Type = TypeVideo
		element.VideoID = class.ID
		element.VideoSource = VideoSourceBilibili
		if element.Title == "" {
			element.Title = TrimTitle(info.Title)
		}
		// The preview is copied into this site's own storage, as the original did.
		// Bilibili's CDN refuses requests with a foreign referer, so hotlinking it shows
		// a broken image on every card.
		if preview := NormalizeThumbnailURL(info.ThumbURL); preview != "" {
			if url, key, ok := service.storeRemoteImage(ctx, serial, preview); ok {
				element.ThumbURL, element.Path = url, key
			}
		}
		return element, ""

	case SourceImage:
		if class.Size > MaxFileBytes {
			return NewElement{}, ReasonTooLarge
		}
		url, key, ok := service.storeRemoteImage(ctx, serial, entry.URL)
		if !ok {
			return NewElement{}, ReasonUnavailable
		}
		element.Type = TypeImage
		element.ThumbURL, element.Path = url, key
		if element.Title == "" {
			element.Title = TitleFromFileName(fileNameFromURL(entry.URL))
		}
		return element, ""

	case SourceVideoFile:
		if class.Size > MaxFileBytes {
			return NewElement{}, ReasonTooLarge
		}
		// Left where it is: the original stored no copy of a linked video and used the
		// source as its own thumbnail.
		element.Type = TypeVideo
		element.VideoSource = VideoSourceFile
		element.ThumbURL = entry.URL
		if element.Title == "" {
			element.Title = "video"
		}
		return element, ""

	case SourceTwitch:
		// Recognised, deliberately not ingested: see the note at the top of this file.
		return NewElement{}, ReasonUnavailable
	}

	return NewElement{}, ReasonUnrecognised
}

// storeRemoteImage downloads an image and puts it in this site's own storage.
//
// The fetch goes through the SSRF-safe fetcher, and the bytes are sniffed afterwards:
// a URL that answered image/png to a HEAD can serve anything at all to the GET, and what
// is stored is what was actually received.
func (service *Service) storeRemoteImage(
	ctx context.Context, serial, rawURL string,
) (url, key string, ok bool) {
	if service.fetcher == nil {
		return "", "", false
	}
	content, err := service.fetcher.Fetch(ctx, rawURL)
	if err != nil || len(content) == 0 || len(content) > MaxFileBytes {
		return "", "", false
	}
	kind, recognised := SniffUpload(content)
	if !recognised || kind.Type != TypeImage {
		return "", "", false
	}

	key = service.keyName(serial, kind.Extension)
	stored, err := service.objects.Put(ctx, key, content, kind.ContentType)
	if err != nil {
		return "", "", false
	}
	return stored, key, true
}

// fileNameFromURL is the last path segment, for the title of an image that came with none.
func fileNameFromURL(rawURL string) string {
	trimmed := rawURL
	if index := strings.IndexAny(trimmed, "?#"); index >= 0 {
		trimmed = trimmed[:index]
	}
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		trimmed = trimmed[index+1:]
	}
	return trimmed
}

// CodeTooMany is a batch with more URLs than one request may carry.
const CodeTooMany = "too_many"
