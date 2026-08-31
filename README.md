# mediahash

[![CI](https://github.com/Anastylosis/mediahash/actions/workflows/ci.yml/badge.svg)](https://github.com/Anastylosis/mediahash/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/Anastylosis/mediahash/branch/master/graph/badge.svg)](https://codecov.io/gh/Anastylosis/mediahash)
[![Go Reference](https://pkg.go.dev/badge/github.com/Anastylosis/mediahash.svg)](https://pkg.go.dev/github.com/Anastylosis/mediahash)
[![Go Report Card](https://goreportcard.com/badge/github.com/Anastylosis/mediahash)](https://goreportcard.com/report/github.com/Anastylosis/mediahash)

Video fingerprints, computed the way [Stash](https://stashapp.cc) computes
them — bit-identical to the values a Stash instance stores, without
importing the Stash module or running a Stash.

This library answers "what is this file", never "what is in it". Metadata
extraction (codecs, resolutions, tags) is a different job with different
scope pressure; it does not live here.

## Packages

| package | what it computes |
|---|---|
| `oshash` | The OpenSubtitles moviehash, ported from Stash's `pkg/hash/oshash` — including Stash's small-file behavior, which the original reference implementation rejects. Pure Go, hashes any `io.ReaderAt`. |
| `videophash` | Stash's 64-bit perceptual video hash: 25 frames sampled across the middle 90% of the video via ffmpeg, composited into a 5×5 sprite, hashed with goimagehash's `PerceptionHash`. Also carries the ffprobe duration helper, because the sample positions are computed from Stash's two-decimal-rounded duration. |

```go
import (
    "github.com/Anastylosis/mediahash/oshash"
    "github.com/Anastylosis/mediahash/videophash"
)

osh, err := oshash.FromFile("/videos/scene.mp4")

phash, err := videophash.FromFile(ctx, "ffmpeg", "ffprobe", "/videos/scene.mp4")
// or, when the duration is already known from a probe:
d, err := videophash.Duration(ctx, "ffprobe", "/videos/scene.mp4")
phash, err = videophash.Generate(ctx, "ffmpeg", "/videos/scene.mp4", d)
```

## Bring your own ffmpeg

`videophash` shells out to the `ffmpeg`/`ffprobe` paths you pass it —
nothing is bundled, downloaded, or discovered. That is deliberate: the
library's job is to make the *invocation* exactly Stash's, and every
constant in it (seek positions, `scale=160:-2`, BMP over rawvideo, the
`-ss` float formatting) is pinned by tests.

What it cannot pin is ffmpeg itself. Different ffmpeg builds can decode
different frames at the same seek, which flips hash bits. If you need your
hashes to agree with a particular Stash instance, run an ffmpeg build of
the same lineage Stash auto-downloads, and verify (below).

## Verifying bit-exactness

`videophash`'s definition of done is zero mismatches against a real Stash
library. The harness is `TestBitExact`, driven by a manifest of
tab-separated `<phash hex><TAB><absolute path>` lines (phash exactly as
Stash's GraphQL emits it — unpadded hex):

```sh
MEDIAHASH_VERIFY_MANIFEST=/path/to/manifest.tsv \
MEDIAHASH_FFMPEG=/path/to/ffmpeg \
MEDIAHASH_FFPROBE=/path/to/ffprobe \
go test ./videophash -run TestBitExact -v
```

Pick manifest entries spanning containers (mp4, wmv, flv, avi, mkv, mov):
container demuxing is exactly where ffmpeg builds diverge. Every mismatch
prints the Hamming distance — 1–2 bits means a decode drift to chase,
tens of bits means the wrong file.

## Dependencies

`github.com/corona10/goimagehash` (the hash Stash uses, at Stash's pinned
version) and `golang.org/x/image` (BMP decoding). Nothing else, by policy.
