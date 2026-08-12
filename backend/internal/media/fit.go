// Package media generates the image and video thumbnails that back the element
// previews.
//
// It replaces App\Services\ImageThumbnailService, App\Jobs\ResizeElementImage and
// App\Jobs\MakeVideoThumbnail.
//
// All pixel work is done by ffmpeg rather than an image library. The output format
// is WebP, and Go has no pure-Go WebP encoder — golang.org/x/image/webp decodes
// only — so the alternatives were a cgo binding to libwebp or an external binary.
// ffmpeg is already in the worker image for video frames, its build carries the
// libwebp encoder and the webp and gif decoders, and using it keeps the binary
// CGO_ENABLED=0 and statically linked.
package media

import "math"

// FitBox shrinks a box to fit inside a maximum while preserving aspect ratio.
//
// Port of the sizing in ImageThumbnailService::makeThumbnail:
//
//	$ratio = $originalWidth / $originalHeight;
//	$maxRatio = $maxWidth / $maxHeight;
//	if ($ratio > $maxRatio) { $maxHeight = round($maxWidth / $ratio); }
//	else                    { $maxWidth  = round($maxHeight * $ratio); }
//
// One of the two returned dimensions therefore always equals the corresponding
// maximum, and the other is rounded. The result is fed to ffmpeg as exact
// dimensions, so getting the rounding wrong changes both the output pixels and the
// storage path, which embeds the size.
//
// PHP's round() is half-away-from-zero, which is math.Round, not Go's
// integer-truncating conversion.
func FitBox(originalWidth, originalHeight, maxWidth, maxHeight int) (width, height int) {
	if originalWidth <= 0 || originalHeight <= 0 || maxWidth <= 0 || maxHeight <= 0 {
		return 0, 0
	}

	ratio := float64(originalWidth) / float64(originalHeight)
	maxRatio := float64(maxWidth) / float64(maxHeight)

	if ratio > maxRatio {
		// Wider than the box: width is the constraint.
		return maxWidth, int(math.Round(float64(maxWidth) / ratio))
	}
	// Taller than or as wide as the box: height is the constraint.
	return int(math.Round(float64(maxHeight) * ratio)), maxHeight
}
