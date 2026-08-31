package videophash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

// ProbeError reports an ffprobe invocation or parse failure.
type ProbeError struct {
	Path string
	Err  error
}

func (e *ProbeError) Error() string {
	return fmt.Sprintf("videophash: probing %s: %v", e.Path, e.Err)
}

func (e *ProbeError) Unwrap() error { return e.Err }

// Duration returns the video's duration the way Stash records it: ffprobe's
// container-level format.duration, parsed as float64 and rounded to two
// decimals (Stash's ffprobe wrapper rounds FileDuration exactly so, and its
// scanner stores that rounded value on the file the phash sampler reads).
// Generate must be fed this value — a stream duration or an unrounded probe
// shifts every sample time and yields a different hash.
func Duration(ctx context.Context, ffprobePath, videoPath string) (float64, error) {
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		videoPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = fmt.Errorf("%w: %s", err, msg)
		}
		return 0, &ProbeError{Path: videoPath, Err: err}
	}

	d, err := parseDuration(stdout.Bytes())
	if err != nil {
		return 0, &ProbeError{Path: videoPath, Err: err}
	}
	return d, nil
}

// parseDuration extracts format.duration from ffprobe's JSON output and
// rounds to two decimals. A missing or unparsable duration is an error here
// where Stash silently records 0 — hashing 25 frames at t=0 would succeed
// and return a wrong-but-plausible hash, exactly the kind of silent
// corruption this package exists to prevent.
func parseDuration(probeJSON []byte) (float64, error) {
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(probeJSON, &probe); err != nil {
		return 0, fmt.Errorf("parsing ffprobe output: %w", err)
	}
	if probe.Format.Duration == "" {
		return 0, fmt.Errorf("ffprobe reported no format duration")
	}
	d, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing duration %q: %w", probe.Format.Duration, err)
	}
	return math.Round(d*100) / 100, nil
}
