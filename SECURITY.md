# Security Policy

## Untrusted input

mediahash holds no credentials and makes no network calls. Its inputs are
video files the caller names — treated as untrusted, since they typically
arrive from downloads. `oshash` reads bytes and arithmetic-sums them;
malformed files must produce an error or a hash, never a panic.

`videophash` executes the ffmpeg/ffprobe binaries the *caller* provides,
with the video path passed as an argument vector (never a shell), and
parses only sizes and JSON out of their output. The security boundary for
decoding hostile video files is ffmpeg's, not this library's — callers
hashing untrusted files inherit ffmpeg's attack surface and should keep
their ffmpeg patched.

## Reporting a vulnerability

If you find a security issue, please open a GitHub issue or email the
maintainer directly. There is no bug bounty program.
