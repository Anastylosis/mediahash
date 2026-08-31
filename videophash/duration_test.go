package videophash

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    float64
		wantErr bool
	}{
		{"typical", `{"format":{"duration":"734.123456"}}`, 734.12, false},
		{"rounds half up", `{"format":{"duration":"0.005"}}`, 0.01, false},
		{"already two decimals", `{"format":{"duration":"1288.44"}}`, 1288.44, false},
		{"integer seconds", `{"format":{"duration":"3.000000"}}`, 3, false},
		{"missing duration", `{"format":{}}`, 0, true},
		{"no format", `{}`, 0, true},
		{"garbage duration", `{"format":{"duration":"N/A"}}`, 0, true},
		{"not json", `moov atom not found`, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseDuration([]byte(c.in))
			if c.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q) = %v, want error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("parseDuration(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestDuration_MissingFile(t *testing.T) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not on PATH")
	}
	_, err = Duration(context.Background(), ffprobe, filepath.Join(t.TempDir(), "absent.mp4"))
	var pe *ProbeError
	if !errors.As(err, &pe) {
		t.Fatalf("Duration(missing) error = %v (%T), want *ProbeError", err, err)
	}
}
