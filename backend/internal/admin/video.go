package admin

import (
	"context"
	"errors"
	"strings"

	"2pick.app/backend/internal/ingest"
)

// Resolving a carousel slide's video URL, which is what HomeCarouselService did before
// storing one.
//
// The classification and the metadata lookups are the ingest package's, unchanged: a
// carousel slide is a YouTube or Bilibili video by the same rules an element is, and a
// second copy of "what is this URL" would be a second set of hosts to keep in step.
//
// TWITCH IS RECOGNISED AND REFUSED, as it is for elements: the original resolved it through
// an app-token API whose credentials this deployment no longer refreshes, so a Twitch slide
// created here would carry no title and no preview. Twitch slides already in the table keep
// working — nothing rewrites them.

// ErrVideoUnavailable means the URL is a video this deployment cannot read.
var ErrVideoUnavailable = errors.New("admin: the video could not be read")

// IngestVideos resolves a carousel video URL with the ingest package's lookups.
type IngestVideos struct {
	youtube ingest.YouTubeLookup
	pages   ingest.PageLookup
}

// NewIngestVideos wires the resolver. Either lookup may be nil, which makes that source
// unresolvable rather than making the whole resolver unusable: a deployment with no YouTube
// API key can still add a Bilibili slide.
func NewIngestVideos(youtube ingest.YouTubeLookup, pages ingest.PageLookup) *IngestVideos {
	return &IngestVideos{youtube: youtube, pages: pages}
}

func (videos *IngestVideos) Resolve(ctx context.Context, videoURL string) (ResolvedVideo, error) {
	videoURL = strings.TrimSpace(videoURL)
	// No prober: a carousel slide is a video, so a URL no pattern recognises is not one
	// worth a network request to find out about.
	class := ingest.Classify(ctx, videoURL, nil)

	switch class.Source {
	case ingest.SourceYouTube, ingest.SourceYouTubeEmbed:
		if videos.youtube == nil {
			return ResolvedVideo{}, ErrVideoUnavailable
		}
		info, err := videos.youtube.Video(ctx, class.ID)
		if err != nil {
			return ResolvedVideo{}, ErrVideoUnavailable
		}
		source := ingest.VideoSourceYouTube
		if class.Source == ingest.SourceYouTubeEmbed {
			source = ingest.VideoSourceYouTubeEmbed
		}
		return ResolvedVideo{
			Title:    ingest.TrimTitle(info.Title),
			ThumbURL: info.ThumbURL,
			Source:   source,
			ID:       class.ID,
			URL:      videoURL,
		}, nil

	case ingest.SourceBilibili:
		if videos.pages == nil {
			return ResolvedVideo{}, ErrVideoUnavailable
		}
		info, err := videos.pages.Page(ctx, videoURL)
		if err != nil {
			return ResolvedVideo{}, ErrVideoUnavailable
		}
		// The preview is hotlinked rather than copied into this site's storage, unlike an
		// element's: a carousel slide is created by a moderator one at a time, and the
		// broken-referer problem shows up immediately on the page they are editing.
		return ResolvedVideo{
			Title:    ingest.TrimTitle(info.Title),
			ThumbURL: ingest.NormalizeThumbnailURL(info.ThumbURL),
			Source:   ingest.VideoSourceBilibili,
			ID:       class.ID,
			URL:      videoURL,
		}, nil
	}

	return ResolvedVideo{}, ErrVideoUnavailable
}
