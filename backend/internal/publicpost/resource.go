package publicpost

import (
	"regexp"
	"strings"
)

// Video sources that the frontend can render inline, from
// PostResource::isPreviewable and App\Enums\VideoSource.
const (
	VideoSourceYouTube      = "youtube"
	VideoSourceYouTubeEmbed = "youtube_embed"
	VideoSourceBilibili     = "bilibili_video"
	VideoSourceTwitchVideo  = "twitch_video"
	VideoSourceTwitchClip   = "twitch_clip"
	VideoSourceURL          = "url"
)

// ElementTypeImage is App\Enums\ElementType::IMAGE.
const ElementTypeImage = "image"

// imageExtension matches PostResource::isImageType, which is
// preg_match('/\.(jpeg|jpg|png|gif|webp)$/', $url).
//
// Anchored at the end and case-sensitive, both deliberately: the PHP pattern has no
// /i flag and no trailing anchor beyond $, so ".PNG" does not match and neither does
// a URL with a query string after the extension. Copying the looseness matters more
// than improving it, because the value decides whether the browser tries to render
// the element.
var imageExtension = regexp.MustCompile(`\.(jpeg|jpg|png|gif|webp)$`)

// DefaultThumbURL is Element::getDefaultThumbUrl: the plain thumb, with no fallback.
func DefaultThumbURL(element ElementRow) *string {
	return element.ThumbURL
}

// MediumThumbURL is Element::getMediumThumbUrl: the medium derivative if it exists,
// otherwise the plain thumb.
func MediumThumbURL(element ElementRow) *string {
	if element.MediumThumbURL != nil && strings.TrimSpace(*element.MediumThumbURL) != "" {
		return element.MediumThumbURL
	}
	return element.ThumbURL
}

// Previewable reports whether the frontend can render this element on a card.
//
// Port of PostResource::isPreviewable. The last clause is the subtle one: a plain URL
// video source counts as previewable only when its *thumb_url* looks like an image
// file, which is how a link to an image ends up rendering while a link to a page does
// not.
func Previewable(element ElementRow) bool {
	if element.Type != nil && *element.Type == ElementTypeImage {
		return true
	}
	if element.VideoSource == nil {
		return false
	}
	switch *element.VideoSource {
	case VideoSourceYouTube, VideoSourceYouTubeEmbed, VideoSourceBilibili,
		VideoSourceTwitchVideo, VideoSourceTwitchClip:
		return true
	case VideoSourceURL:
		return element.ThumbURL != nil && imageExtension.MatchString(*element.ThumbURL)
	}
	return false
}

// BuildElement turns a stored element into the preview shape.
//
// A nil element produces the all-null placeholder the PHP emits when a post has
// fewer than two usable elements. internal/publiccontent already recognises that
// shape and drops it, so the placeholder must keep exactly these fields.
func BuildElement(element *ElementRow) Element {
	if element == nil {
		return Element{Previewable: false}
	}
	id := element.ID
	return Element{
		VideoSource: element.VideoSource,
		Type:        element.Type,
		ID:          &id,
		URL:         DefaultThumbURL(*element),
		URL2:        MediumThumbURL(*element),
		Title:       element.Title,
		Previewable: Previewable(*element),
	}
}

// SelectPreviewElements picks the two elements a card shows.
//
// Port of the opening of PostResource::toArray. The preference is two of the post's
// top-ranked elements; only when fewer than two of those exist does it fall back to
// any two elements.
//
// The choice is RANDOM, which is not an accident of the port: the PHP shuffles both
// collections before popping. So the payload for one post differs between two runs
// even with identical data, and this cannot be verified by comparing against the
// stored rows — only the shape and the selection rules can be.
//
// shuffle is injected so a test can pin the choice. It must permute in place, like
// rand.Shuffle.
func SelectPreviewElements(
	ranked []ElementRow,
	all []ElementRow,
	shuffle func(length int, swap func(i, j int)),
) (*ElementRow, *ElementRow) {
	// `if ($ranks->count() >= 2)`: the ranked set is used only when it can supply
	// both, never mixed with the fallback.
	if len(ranked) >= 2 {
		candidates := make([]ElementRow, len(ranked))
		copy(candidates, ranked)
		shuffle(len(candidates), func(i, j int) {
			candidates[i], candidates[j] = candidates[j], candidates[i]
		})
		// pop() takes from the end, twice.
		last := len(candidates) - 1
		return &candidates[last], &candidates[last-1]
	}

	candidates := make([]ElementRow, len(all))
	copy(candidates, all)
	shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	// `->take(2)` then two pop()s. With fewer than two elements the pops yield null,
	// which becomes the placeholder.
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	switch len(candidates) {
	case 0:
		return nil, nil
	case 1:
		return &candidates[0], nil
	default:
		return &candidates[1], &candidates[0]
	}
}

// DateTimeLayout is Carbon's toDateTimeString().
const DateTimeLayout = "2006-01-02 15:04:05"

// BuildResource assembles the payload for one post.
func BuildResource(
	post PostRow,
	tags []string,
	elementsCount int64,
	playCount int64,
	element1 *ElementRow,
	element2 *ElementRow,
) Resource {
	// An empty tag list must encode as [] rather than null: the column is JSON that
	// the frontend iterates, and null would break it.
	if tags == nil {
		tags = []string{}
	}
	return Resource{
		Title:         post.Title,
		Serial:        post.Serial,
		IsPrivate:     post.IsPrivate,
		Description:   post.Description,
		Element1:      BuildElement(element1),
		Element2:      BuildElement(element2),
		CreatedAt:     post.CreatedAt.Format(DateTimeLayout),
		UpdatedAt:     post.UpdatedAt.Format(DateTimeLayout),
		PlayCount:     playCount,
		ElementsCount: elementsCount,
		Tags:          tags,
		IsCensored:    post.IsCensored,
	}
}
