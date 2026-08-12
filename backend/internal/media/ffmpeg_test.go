package media

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// requireTranscoder skips unless ffmpeg and ffprobe are present. The api and
// scheduler images ship FROM scratch and the test container has no ffmpeg either,
// so these run only in the media worker image.
func requireTranscoder(t *testing.T) *Transcoder {
	t.Helper()
	transcoder, err := NewTranscoder()
	if err != nil {
		t.Skipf("ffmpeg or ffprobe unavailable: %v", err)
	}
	return transcoder
}

// generatePNG makes a real image with ffmpeg so the tests do not carry binary
// fixtures.
func generatePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	command := exec.Command(ffmpegPath,
		"-hide_banner", "-v", "error",
		"-f", "lavfi",
		"-i", "testsrc=size="+itoa(width)+"x"+itoa(height)+":rate=1",
		"-frames:v", "1",
		"-f", "image2", "-c:v", "png",
		"pipe:1",
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("generate png: %v: %s", err, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("generate png produced no output")
	}
	return stdout.Bytes()
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func TestProbeImageReadsDimensions(t *testing.T) {
	transcoder := requireTranscoder(t)
	source := generatePNG(t, 640, 360)

	dimensions, err := transcoder.ProbeImage(context.Background(), source)
	if err != nil {
		t.Fatalf("ProbeImage() error = %v", err)
	}
	if dimensions.Width != 640 || dimensions.Height != 360 {
		t.Fatalf("dimensions = %dx%d, want 640x360", dimensions.Width, dimensions.Height)
	}
}

func TestProbeImageRejectsGarbage(t *testing.T) {
	transcoder := requireTranscoder(t)

	_, err := transcoder.ProbeImage(context.Background(), []byte("this is not an image"))
	if err == nil {
		t.Fatal("ProbeImage() should fail on non-image bytes")
	}
	if !errors.Is(err, ErrProbeFailed) && !errors.Is(err, ErrNoVideoStream) {
		t.Fatalf("ProbeImage() error = %v, want a probe failure", err)
	}
}

// The end-to-end pixel path: probe, fit, encode, and confirm the output is really
// WebP at the requested size.
func TestResizeToWebPProducesWebPAtTheFittedSize(t *testing.T) {
	transcoder := requireTranscoder(t)
	source := generatePNG(t, 1600, 900)

	dimensions, err := transcoder.ProbeImage(context.Background(), source)
	if err != nil {
		t.Fatalf("ProbeImage() error = %v", err)
	}
	width, height := FitBox(dimensions.Width, dimensions.Height, 400, 400)
	if width != 400 || height != 225 {
		t.Fatalf("FitBox() = %dx%d, want 400x225", width, height)
	}

	encoded, err := transcoder.ResizeToWebP(context.Background(), source, width, height)
	if err != nil {
		t.Fatalf("ResizeToWebP() error = %v", err)
	}
	if !looksLikeWebP(encoded) {
		t.Fatalf("output is not WebP: first bytes %q", firstBytes(encoded))
	}

	// Probing the output confirms the encoder honoured the scale rather than
	// silently keeping the source size.
	out, err := transcoder.ProbeImage(context.Background(), encoded)
	if err != nil {
		t.Fatalf("ProbeImage(output) error = %v", err)
	}
	if out.Width != width || out.Height != height {
		t.Fatalf("encoded size = %dx%d, want %dx%d", out.Width, out.Height, width, height)
	}
}

// An animated GIF must yield a single still frame, which is what the original does
// by coalescing and taking getImage().
func TestResizeToWebPTakesTheFirstFrameOfAnAnimation(t *testing.T) {
	transcoder := requireTranscoder(t)
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg unavailable: %v", err)
	}

	// A 10 frame animated GIF.
	command := exec.Command(ffmpegPath,
		"-hide_banner", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=10",
		"-frames:v", "10",
		"-f", "gif", "pipe:1",
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Skipf("could not build a gif fixture: %v: %s", err, stderr.String())
	}

	encoded, err := transcoder.ResizeToWebP(context.Background(), stdout.Bytes(), 160, 120)
	if err != nil {
		t.Fatalf("ResizeToWebP() error = %v", err)
	}
	if !looksLikeWebP(encoded) {
		t.Fatalf("output is not WebP: first bytes %q", firstBytes(encoded))
	}

	frames, err := countFrames(encoded)
	if err != nil {
		t.Fatalf("countFrames() error = %v", err)
	}
	if frames != 1 {
		t.Fatalf("output has %d frames, want 1", frames)
	}
}

func TestResizeToWebPRejectsNonPositiveDimensions(t *testing.T) {
	transcoder := requireTranscoder(t)
	source := generatePNG(t, 100, 100)

	for _, size := range [][2]int{{0, 100}, {100, 0}, {-1, 100}} {
		if _, err := transcoder.ResizeToWebP(context.Background(), source, size[0], size[1]); err == nil {
			t.Errorf("ResizeToWebP(%dx%d) should fail", size[0], size[1])
		}
	}
}

func TestResizeToWebPRejectsGarbage(t *testing.T) {
	transcoder := requireTranscoder(t)

	if _, err := transcoder.ResizeToWebP(context.Background(), []byte("not an image"), 100, 100); err == nil {
		t.Fatal("ResizeToWebP() should fail on non-image bytes")
	}
}

// ffmpeg does its own connecting, so the URL guard has to be applied before the
// process starts.
func TestExtractVideoFrameRefusesUnsafeURLs(t *testing.T) {
	transcoder := requireTranscoder(t)

	for _, rawURL := range []string{
		"file:///etc/passwd",
		"http://127.0.0.1/video.mp4",
		"http://169.254.169.254/latest/meta-data/",
		"",
	} {
		_, err := transcoder.ExtractVideoFrame(context.Background(), rawURL)
		if err == nil {
			t.Errorf("ExtractVideoFrame(%q) should be refused", rawURL)
			continue
		}
		if !errors.Is(err, ErrBlockedURL) {
			t.Errorf("ExtractVideoFrame(%q) error = %v, want ErrBlockedURL", rawURL, err)
		}
	}
}

func TestNewTranscoderReportsMissingBinaries(t *testing.T) {
	// PATH is emptied so LookPath cannot find either binary. A worker image without
	// ffmpeg must fail loudly at wiring time rather than per job.
	t.Setenv("PATH", "")
	if _, err := NewTranscoder(); err == nil {
		t.Fatal("NewTranscoder() should fail when ffmpeg is not on PATH")
	}
}

func looksLikeWebP(data []byte) bool {
	// RIFF....WEBP
	return len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP"))
}

func firstBytes(data []byte) string {
	if len(data) > 16 {
		return string(data[:16])
	}
	return string(data)
}

// countFrames asks ffprobe how many frames the encoded output holds.
func countFrames(encoded []byte) (int, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, err
	}
	file, err := os.CreateTemp("", "webp-*.webp")
	if err != nil {
		return 0, err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(encoded); err != nil {
		file.Close()
		return 0, err
	}
	file.Close()

	command := exec.Command(ffprobePath,
		"-v", "error", "-select_streams", "v:0",
		"-count_frames", "-show_entries", "stream=nb_read_frames",
		"-of", "default=nokey=1:noprint_wrappers=1",
		file.Name(),
	)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	if err := command.Run(); err != nil {
		return 0, err
	}

	count := 0
	for _, character := range bytes.TrimSpace(stdout.Bytes()) {
		if character < '0' || character > '9' {
			break
		}
		count = count*10 + int(character-'0')
	}
	return count, nil
}
