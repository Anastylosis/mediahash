// Package mediahash identifies video files the way Stash does: oshash (the
// OpenSubtitles moviehash) and the 64-bit perceptual video hash, computed to
// be bit-identical to the values a Stash instance stores. It answers "what
// is this file", never "what is in it" — metadata extraction is out of
// scope by design.
//
// The subpackages are independent:
//
//   - oshash: pure-Go, hashes any io.ReaderAt.
//   - videophash: shells out to ffmpeg/ffprobe binaries the caller
//     provides; nothing is bundled or downloaded.
package mediahash
