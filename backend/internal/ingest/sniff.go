package ingest

import (
	"fmt"

	"github.com/google/uuid"
)

// What an uploaded file is, decided from its leading bytes.
type Kind struct {
	// Type is the elements.type enum: image or video.
	Type string
	// ContentType is what the object store is told, and what a browser will be served.
	ContentType string
	// Extension is what the object key ends in. Taken from the sniff rather than from
	// the uploaded name, so a file cannot be stored under an extension that disagrees
	// with its contents.
	Extension string
}

// SniffUpload identifies an uploaded file.
//
// THE ALLOWED SET IS THE ORIGINAL'S, not everything Go can recognise. Laravel's rule was
// mimetypes:image/jpeg,image/png,image/bmp,image/webp,image/gif,video/avi,video/mpeg,
// video/mp4 — anything outside that is refused, and adding formats here would put media
// on the site that the player and the thumbnail jobs have never been asked to handle.
//
// http.DetectContentType is deliberately not used: it answers for far more than this
// list, including text/html for a file that merely starts with a tag, which is exactly
// the upload not to store under a media URL.
func SniffUpload(content []byte) (Kind, bool) {
	switch {
	case hasPrefix(content, "\x89PNG\r\n\x1a\n"):
		return Kind{TypeImage, "image/png", "png"}, true
	case hasPrefix(content, "\xff\xd8\xff"):
		return Kind{TypeImage, "image/jpeg", "jpg"}, true
	case hasPrefix(content, "GIF87a"), hasPrefix(content, "GIF89a"):
		return Kind{TypeImage, "image/gif", "gif"}, true
	case hasPrefix(content, "BM"):
		return Kind{TypeImage, "image/bmp", "bmp"}, true
	case len(content) >= 12 && hasPrefix(content, "RIFF") && string(content[8:12]) == "WEBP":
		return Kind{TypeImage, "image/webp", "webp"}, true
	case len(content) >= 12 && hasPrefix(content, "RIFF") && string(content[8:12]) == "AVI ":
		return Kind{TypeVideo, "video/x-msvideo", "avi"}, true
	// An MPEG-4 container names its brand in the ftyp box at offset 4. Everything the
	// site serves is one of these; the brand itself is not read, because the list of
	// brands that mean "mp4" is long and the box being there is the actual signal.
	case len(content) >= 12 && string(content[4:8]) == "ftyp":
		return Kind{TypeVideo, "video/mp4", "mp4"}, true
	// An MPEG program or elementary stream starts with a start code. 0x1BA is a pack
	// header and 0x1B3 a sequence header; both are video/mpeg to the original's rule.
	case hasPrefix(content, "\x00\x00\x01\xba"), hasPrefix(content, "\x00\x00\x01\xb3"):
		return Kind{TypeVideo, "video/mpeg", "mpeg"}, true
	}
	return Kind{}, false
}

func hasPrefix(content []byte, prefix string) bool {
	return len(content) >= len(prefix) && string(content[:len(prefix)]) == prefix
}

// DefaultKey is the object key layout Laravel wrote: the post's serial as a directory,
// then a UUID and the extension. Keeping it means the 240,508 objects already in the
// bucket and the ones written from here sit in the same place.
func DefaultKey(directory, extension string) string {
	name := uuid.NewString()
	if extension != "" {
		name += "." + extension
	}
	if directory == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", directory, name)
}
