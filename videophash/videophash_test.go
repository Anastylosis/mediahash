package videophash

import (
	"context"
	"errors"
	"image"
	"image/color"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFrameArgs freezes the exact ffmpeg invocation. This argument list is
// Stash's transcoder.ScreenshotTime output verbatim — changing anything here
// changes which bytes ffmpeg emits and breaks bit-exactness, so the test
// spells the full slice rather than checking fragments.
func TestFrameArgs(t *testing.T) {
	got := frameArgs("/videos/a b.mp4", 8.6)
	want := []string{
		"-v", "error",
		"-y",
		"-ss", "8.6",
		"-i", "/videos/a b.mp4",
		"-frames:v", "1",
		"-vf", "scale=160:-2",
		"-c:v", "bmp",
		"-f", "rawvideo",
		"-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frameArgs = %q, want %q", got, want)
	}
}

// TestFrameArgs_SeekFormatting pins the -ss formatting to fmt.Sprint's
// shortest-form output, since that is how Stash renders its seek values.
func TestFrameArgs_SeekFormatting(t *testing.T) {
	cases := []struct {
		t    float64
		want string
	}{
		{5, "5"},
		{36.706, "36.706"},
		{63.13432, "63.13432"},
		{670.98568, "670.98568"},
	}
	for _, c := range cases {
		args := frameArgs("v.mp4", c.t)
		if args[4] != c.want {
			t.Errorf("frameArgs(t=%v): -ss value %q, want %q", c.t, args[4], c.want)
		}
	}
}

// TestSampleTimes pins the sample positions for two durations (one exact,
// one two-decimal like real probe output) against values computed with
// Stash's exact float expressions. A drift here — reordered arithmetic,
// a different constant — moves every seek and changes hashes silently.
func TestSampleTimes(t *testing.T) {
	cases := []struct {
		duration float64
		i        int
		want     float64
	}{
		{100, 0, 5},
		{100, 1, 8.6},
		{100, 12, 48.2},
		{100, 24, 91.4},
		{734.12, 0, 36.706},
		{734.12, 1, 63.13432},
		{734.12, 12, 353.84584},
		{734.12, 24, 670.98568},
	}
	for _, c := range cases {
		times := sampleTimes(c.duration)
		if len(times) != FrameCount {
			t.Fatalf("sampleTimes(%v): %d times, want %d", c.duration, len(times), FrameCount)
		}
		// Exact equality on purpose: bit-identical floats are the point.
		if times[c.i] != c.want {
			t.Errorf("sampleTimes(%v)[%d] = %v, want %v", c.duration, c.i, times[c.i], c.want)
		}
	}
}

// TestCombine verifies row-major 5×5 placement and that opaque source
// pixels survive the RGBA→NRGBA copy unchanged.
func TestCombine(t *testing.T) {
	const w, h = 3, 2
	frames := make([]image.Image, FrameCount)
	for i := range frames {
		// Opaque premultiplied RGBA, like a decoded 24-bit BMP.
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		c := color.RGBA{R: uint8(i * 10), G: uint8(255 - i*10), B: uint8(i), A: 255}
		for x := 0; x < w; x++ {
			for y := 0; y < h; y++ {
				img.SetRGBA(x, y, c)
			}
		}
		frames[i] = img
	}

	sprite := combine(frames)
	if got, want := sprite.Bounds(), image.Rect(0, 0, w*columns, h*rows); got != want {
		t.Fatalf("sprite bounds = %v, want %v", got, want)
	}
	for i := 0; i < FrameCount; i++ {
		x := w*(i%columns) + 1
		y := h*(i/columns) + 1
		got := sprite.NRGBAAt(x, y)
		want := color.NRGBA{R: uint8(i * 10), G: uint8(255 - i*10), B: uint8(i), A: 255}
		if got != want {
			t.Errorf("frame %d: sprite pixel (%d,%d) = %v, want %v", i, x, y, got, want)
		}
	}
}

// TestGenerate_EndToEnd exercises the real ffmpeg/ffprobe pipeline on a
// synthesized clip when the binaries are on PATH: probe → 25 frames →
// sprite → hash, twice, asserting determinism. The hash value itself is
// not asserted — it depends on the ffmpeg build; bit-exactness against
// Stash is TestBitExact's job.
func TestGenerate_EndToEnd(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}

	ctx := context.Background()
	clip := filepath.Join(t.TempDir(), "clip.mp4")
	synth := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=duration=3:size=320x240:rate=10",
		clip)
	if out, err := synth.CombinedOutput(); err != nil {
		t.Fatalf("synthesizing clip: %v: %s", err, out)
	}

	d, err := Duration(ctx, ffprobe, clip)
	if err != nil {
		t.Fatalf("Duration: %v", err)
	}
	if d < 2.5 || d > 3.5 {
		t.Fatalf("Duration = %v, want ~3", d)
	}

	first, err := Generate(ctx, ffmpeg, clip, d)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	second, err := FromFile(ctx, ffmpeg, ffprobe, clip)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	if first != second {
		t.Errorf("hash not deterministic: %016x then %016x", first, second)
	}
}

func TestGenerate_MissingFile(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not on PATH")
	}
	_, err = Generate(context.Background(), ffmpeg, filepath.Join(t.TempDir(), "absent.mp4"), 100)
	var fe *FrameError
	if !errors.As(err, &fe) {
		t.Fatalf("Generate(missing) error = %v, want *FrameError", err)
	}
	if fe.Index != 0 {
		t.Errorf("FrameError.Index = %d, want 0 (fails on the first frame)", fe.Index)
	}
}
