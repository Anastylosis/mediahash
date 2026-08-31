package videophash

import (
	"bufio"
	"context"
	"fmt"
	"math/bits"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestBitExact is the bit-exactness harness: it replays a manifest of
// videos whose phash a real Stash instance has already computed and demands
// zero mismatches. This is the package's definition of done — the unit
// tests prove the pipeline is internally faithful, only this proves the
// shipped ffmpeg build decodes the same bytes Stash's did.
//
// It skips unless MEDIAHASH_VERIFY_MANIFEST names a file of
// tab-separated lines:
//
//	<phash hex, as Stash emits it — unpadded><TAB><absolute video path>
//
// MEDIAHASH_FFMPEG and MEDIAHASH_FFPROBE override which binaries run
// (default: PATH lookup). Any mismatch is reported with the Hamming
// distance, because distance 1–2 means a decode drift to chase while a
// large distance means the wrong file or a truncated copy.
func TestBitExact(t *testing.T) {
	manifest := os.Getenv("MEDIAHASH_VERIFY_MANIFEST")
	if manifest == "" {
		t.Skip("MEDIAHASH_VERIFY_MANIFEST not set")
	}
	ffmpeg := os.Getenv("MEDIAHASH_FFMPEG")
	if ffmpeg == "" {
		var err error
		if ffmpeg, err = exec.LookPath("ffmpeg"); err != nil {
			t.Fatal("ffmpeg not on PATH and MEDIAHASH_FFMPEG not set")
		}
	}
	ffprobe := os.Getenv("MEDIAHASH_FFPROBE")
	if ffprobe == "" {
		var err error
		if ffprobe, err = exec.LookPath("ffprobe"); err != nil {
			t.Fatal("ffprobe not on PATH and MEDIAHASH_FFPROBE not set")
		}
	}

	f, err := os.Open(manifest)
	if err != nil {
		t.Fatalf("opening manifest: %v", err)
	}
	defer func() { _ = f.Close() }()

	ctx := context.Background()
	var total, mismatches int
	scanner := bufio.NewScanner(f)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		wantHex, path, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s:%d: want <phash hex><TAB><path>, got %q", manifest, lineNo, line)
		}
		want, err := strconv.ParseUint(wantHex, 16, 64)
		if err != nil {
			t.Fatalf("%s:%d: parsing phash %q: %v", manifest, lineNo, wantHex, err)
		}

		total++
		got, err := FromFile(ctx, ffmpeg, ffprobe, path)
		if err != nil {
			mismatches++
			t.Errorf("%s: %v", path, err)
			continue
		}
		if got != want {
			mismatches++
			t.Errorf("%s: got %016x, want %016x (hamming %d)",
				path, got, want, bits.OnesCount64(got^want))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	if total == 0 {
		t.Fatalf("manifest %s contains no entries", manifest)
	}
	fmt.Printf("bit-exact: %d/%d matched\n", total-mismatches, total)
}
