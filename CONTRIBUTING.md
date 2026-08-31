# Contributing

## The one rule

`videophash` is bit-exact with Stash or it is worthless. Any change to its
constants, ffmpeg arguments, float expressions, or compositing is presumed
wrong until the bit-exactness harness (README, "Verifying bit-exactness")
has been re-run against a real Stash library with zero mismatches. The
unit tests freeze the invocation; only the harness proves the pipeline.

## Cutting a release

Releases are tagged `vMAJOR.MINOR.PATCH`. Pushing the tag runs
[`.github/workflows/release.yml`](.github/workflows/release.yml), which calls
the shared `go-release.yml` with `artifacts: false`.

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

There is no gate and no release asset: a Go module is consumed straight from
its git tag through the module proxy, so there is nothing to build or approve
— the release page exists for the changelog, generated from the tag's
annotation and the commits since the last one.

## Tests

`go test ./...` must pass with **no outbound network** — nothing in this
module ever makes one, and CI enforces it with `offline-isolation`. Tests
that need ffmpeg/ffprobe skip when the binaries are not on PATH; run them
locally with ffmpeg installed before sending a videophash change.
