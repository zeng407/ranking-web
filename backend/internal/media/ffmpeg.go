package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Encoding settings, transcribed from the original.
const (
	// WebPQuality matches Imagick's setImageCompressionQuality(80).
	WebPQuality = 80
	// VideoFrameSeconds matches TimeCode::fromSeconds(0.1).
	VideoFrameSeconds = 0.1
	// ProbeTimeout bounds one ffprobe call.
	ProbeTimeout = 20 * time.Second
	// TranscodeTimeout bounds one ffmpeg call. Video frame extraction against a
	// remote URL is the slow case.
	TranscodeTimeout = 60 * time.Second
)

var (
	// ErrNoVideoStream means ffprobe found nothing to take a frame from.
	ErrNoVideoStream = errors.New("media: no video stream")
	// ErrProbeFailed means the source could not be inspected.
	ErrProbeFailed = errors.New("media: probe failed")
)

// Transcoder shells out to ffmpeg and ffprobe.
type Transcoder struct {
	ffmpegPath  string
	ffprobePath string
}

// NewTranscoder resolves the binaries, failing fast if they are missing. The api
// and scheduler images ship FROM scratch and have neither, so a misrouted job
// surfaces here rather than as a confusing runtime error.
func NewTranscoder() (*Transcoder, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("media: ffmpeg not found: %w", err)
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("media: ffprobe not found: %w", err)
	}
	return &Transcoder{ffmpegPath: ffmpegPath, ffprobePath: ffprobePath}, nil
}

// Dimensions is a probed frame size.
type Dimensions struct {
	Width  int
	Height int
}

type probeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"streams"`
}

// ProbeImage reads the first frame's dimensions from bytes on stdin.
//
// The original reads Imagick's getImagePage(), which is the first frame's page
// geometry; for an animated GIF that is the logical canvas rather than the whole
// animation, so probing the first video stream matches it.
func (transcoder *Transcoder) ProbeImage(ctx context.Context, source []byte) (Dimensions, error) {
	probeContext, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	command := exec.CommandContext(probeContext, transcoder.ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,codec_type",
		"-of", "json",
		"-i", "pipe:0",
	)
	command.Stdin = bytes.NewReader(source)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return Dimensions{}, fmt.Errorf("%w: %v: %s", ErrProbeFailed, err, truncate(stderr.String()))
	}

	var output probeOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return Dimensions{}, fmt.Errorf("%w: decode ffprobe output: %v", ErrProbeFailed, err)
	}
	for _, stream := range output.Streams {
		if stream.Width > 0 && stream.Height > 0 {
			return Dimensions{Width: stream.Width, Height: stream.Height}, nil
		}
	}
	return Dimensions{}, ErrNoVideoStream
}

// ResizeToWebP scales bytes on stdin to exactly width by height and encodes WebP.
//
// The dimensions are already aspect-correct because FitBox computed them, so no
// letterboxing is needed and the scale is exact, matching Imagick's resizeImage
// with bestfit left off.
//
// -frames:v 1 takes only the first frame, which is what the original does for GIFs
// by coalescing and calling getImage(). Applying it to every input means a still
// image and an animation follow the same path.
//
// The Lanczos flag matches Imagick::FILTER_LANCZOS.
func (transcoder *Transcoder) ResizeToWebP(ctx context.Context, source []byte, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("media: resize needs positive dimensions, got %dx%d", width, height)
	}

	transcodeContext, cancel := context.WithTimeout(ctx, TranscodeTimeout)
	defer cancel()

	command := exec.CommandContext(transcodeContext, transcoder.ffmpegPath,
		"-hide_banner", "-v", "error",
		"-i", "pipe:0",
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:%d:flags=lanczos", width, height),
		"-c:v", "libwebp",
		"-quality", strconv.Itoa(WebPQuality),
		"-f", "webp",
		"pipe:1",
	)
	command.Stdin = bytes.NewReader(source)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("media: encode webp %dx%d: %v: %s", width, height, err, truncate(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("media: encode webp %dx%d produced no output: %s", width, height, truncate(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// ExtractVideoFrame grabs a JPEG frame from a remote video URL.
//
// It reads the URL directly rather than downloading first, as the original does by
// handing the URL to FFMpeg::open. ffmpeg only needs the container header and the
// first keyframe, so seeking beats pulling a whole video into memory.
//
// The URL must already have been validated: ffmpeg does its own connecting, so the
// dialler guard in Fetcher does not apply here.
func (transcoder *Transcoder) ExtractVideoFrame(ctx context.Context, sourceURL string) ([]byte, error) {
	if err := ValidateSourceURL(sourceURL); err != nil {
		return nil, err
	}

	transcodeContext, cancel := context.WithTimeout(ctx, TranscodeTimeout)
	defer cancel()

	command := exec.CommandContext(transcodeContext, transcoder.ffmpegPath,
		"-hide_banner", "-v", "error",
		// Restricting the protocols is what stops a crafted URL from making ffmpeg
		// read a local file or use some other protocol it supports.
		"-protocol_whitelist", "http,https,tcp,tls",
		"-ss", strconv.FormatFloat(VideoFrameSeconds, 'f', -1, 64),
		"-i", sourceURL,
		"-frames:v", "1",
		"-f", "image2",
		"-c:v", "mjpeg",
		"pipe:1",
	)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("media: extract frame from %q: %v: %s", sourceURL, err, truncate(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("media: extract frame from %q produced no output: %s", sourceURL, truncate(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// truncate keeps ffmpeg's stderr out of the logs at full length.
func truncate(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", "; "))
	const limit = 400
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
