// Package videophash computes Stash's 64-bit perceptual video hash:
// 25 frames sampled across the middle 90% of the video, composited into a
// 5×5 sprite, hashed with goimagehash's PerceptionHash.
//
// Bit-exactness with Stash is the contract. Every constant, argument and
// float expression here mirrors Stash v0.31.1 (pkg/hash/videophash and
// pkg/ffmpeg/transcoder, which build the same output on master) — a
// same-bits hash is what lets a caller treat "phash equal" as "same video a
// Stash user would have fingerprinted". The one thing this package cannot
// pin is ffmpeg itself: different ffmpeg builds can decode different bytes
// for the same seek, so callers who need cross-tool agreement must also
// control which ffmpeg they run (see the repository README).
package videophash

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"os/exec"
	"strings"

	"github.com/corona10/goimagehash"

	// Stash decodes the frames ffmpeg emits as BMP via image.Decode; BMP is
	// uncompressed, so any conforming decoder yields the same pixels.
	_ "golang.org/x/image/bmp"
)

const (
	// screenshotWidth is the scale=160:-2 target Stash renders each sampled
	// frame at (height follows aspect ratio, kept a multiple of 2).
	screenshotWidth = 160
	columns         = 5
	rows            = 5

	// FrameCount is the number of frames sampled per video.
	FrameCount = columns * rows
)

// FrameError reports which sampled frame failed and at what timestamp,
// wrapping the underlying ffmpeg or decode error.
type FrameError struct {
	Index int
	Time  float64
	Err   error
}

func (e *FrameError) Error() string {
	return fmt.Sprintf("videophash: frame %d (t=%v): %v", e.Index, e.Time, e.Err)
}

func (e *FrameError) Unwrap() error { return e.Err }

// Generate computes the perceptual hash of the video at videoPath using the
// ffmpeg binary at ffmpegPath. duration MUST be the value Duration returns
// (Stash's two-decimal-rounded container duration): the 25 sample times are
// derived from it, so a duration from any other source shifts every sample
// and changes the hash. FromFile bundles the two calls.
func Generate(ctx context.Context, ffmpegPath, videoPath string, duration float64) (uint64, error) {
	frames := make([]image.Image, 0, FrameCount)
	for i, t := range sampleTimes(duration) {
		img, err := screenshot(ctx, ffmpegPath, videoPath, t)
		if err != nil {
			return 0, &FrameError{Index: i, Time: t, Err: err}
		}
		frames = append(frames, img)
	}

	hash, err := goimagehash.PerceptionHash(combine(frames))
	if err != nil {
		return 0, fmt.Errorf("videophash: hashing sprite: %w", err)
	}
	return hash.GetHash(), nil
}

// FromFile probes videoPath's duration with ffprobe and hashes it with
// ffmpeg — the two-step flow Stash's scanner performs.
func FromFile(ctx context.Context, ffmpegPath, ffprobePath, videoPath string) (uint64, error) {
	d, err := Duration(ctx, ffprobePath, videoPath)
	if err != nil {
		return 0, err
	}
	return Generate(ctx, ffmpegPath, videoPath, d)
}

// sampleTimes returns the 25 sample timestamps: offset by 5% on each end to
// avoid intros/outros, evenly stepped across the middle 90%. The expressions
// keep Stash's exact shape (offset and step computed once, time as
// offset+i*step) so float rounding matches term for term.
func sampleTimes(duration float64) []float64 {
	offset := 0.05 * duration
	stepSize := (0.9 * duration) / float64(FrameCount)
	times := make([]float64, FrameCount)
	for i := range times {
		times[i] = offset + (float64(i) * stepSize)
	}
	return times
}

// frameArgs is Stash's transcoder.ScreenshotTime invocation rendered as the
// literal argument list: fast seek (-ss before -i), one frame, scaled to
// width 160, emitted as BMP over the rawvideo muxer to stdout. The -ss value
// is formatted with fmt.Sprint because that is what Stash's Args.Seek does —
// shortest float form, matching its seek strings exactly.
func frameArgs(videoPath string, t float64) []string {
	return []string{
		"-v", "error",
		"-y",
		"-ss", fmt.Sprint(t),
		"-i", videoPath,
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%d:-2", screenshotWidth),
		"-c:v", "bmp",
		"-f", "rawvideo",
		"-",
	}
}

func screenshot(ctx context.Context, ffmpegPath, videoPath string, t float64) (image.Image, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, frameArgs(videoPath, t)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("ffmpeg: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("decoding frame: %w", err)
	}
	return img, nil
}

// combine pastes the 25 frames onto a 5×5 canvas, row-major. Stash does this
// with imaging.New + imaging.Paste; image.NewNRGBA + draw.Src is the same
// operation without the dependency — both copy source pixels into a
// zero-initialized NRGBA canvas, and for the fully opaque frames ffmpeg's
// BMP output decodes to, the RGBA→NRGBA per-pixel conversion both perform is
// value-identical. The sprite therefore matches Stash's byte for byte, which
// the bit-exactness harness (TestBitExact) proves end to end.
func combine(frames []image.Image) *image.NRGBA {
	width := frames[0].Bounds().Dx()
	height := frames[0].Bounds().Dy()
	sprite := image.NewNRGBA(image.Rect(0, 0, width*columns, height*rows))
	for i, frame := range frames {
		x := width * (i % columns)
		y := height * (i / columns)
		r := image.Rect(x, y, x+width, y+height)
		draw.Draw(sprite, r, frame, frame.Bounds().Min, draw.Src)
	}
	return sprite
}
