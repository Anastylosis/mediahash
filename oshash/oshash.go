// Package oshash computes Stash's oshash: the OpenSubtitles moviehash
// algorithm, ported from Stash's pkg/hash/oshash so the output is identical
// to what a Stash instance stores — including Stash's small-file behavior,
// which differs from the original OpenSubtitles reference implementation
// (see Compute).
package oshash

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// chunkSize is the head/tail window size the OpenSubtitles algorithm reads
// from each end of the file.
const chunkSize int64 = 64 * 1024

// Compute hashes r: file size plus the little-endian uint64 sum of 8-byte
// words in the first and last chunk of the file, formatted as %016x — the
// 16-character zero-padded lowercase hex string Stash stores and emits.
//
// For files smaller than 2*chunkSize, Stash does NOT error like the
// original OpenSubtitles reference implementation — chunkSize shrinks to
// the largest multiple of 8 not exceeding the file size, so the head and
// tail reads overlap (files under 64KiB may double-count almost every
// byte). Files of 8 bytes or fewer are rejected: there is no room for even
// one 8-byte word.
func Compute(r io.ReaderAt, size int64) (string, error) {
	if size <= 8 {
		return "", fmt.Errorf("oshash: size must be > 8 bytes, got %d", size)
	}

	chunk := chunkSize
	if size < chunk {
		// Round down to a multiple of 8 so sumUint64LE never sees a partial
		// word; this is what makes head and tail overlap for small files
		// instead of the original implementation's hard error.
		chunk = (size / 8) * 8
	}

	head := make([]byte, chunk)
	if err := readAtFull(r, head, 0); err != nil {
		return "", fmt.Errorf("oshash: reading head: %w", err)
	}
	tail := make([]byte, chunk)
	if err := readAtFull(r, tail, size-chunk); err != nil {
		return "", fmt.Errorf("oshash: reading tail: %w", err)
	}

	headSum, err := sumUint64LE(head)
	if err != nil {
		return "", fmt.Errorf("oshash: head: %w", err)
	}
	tailSum, err := sumUint64LE(tail)
	if err != nil {
		return "", fmt.Errorf("oshash: tail: %w", err)
	}

	// uint64 addition wraps mod 2^64 by design — the algorithm relies on
	// that overflow behavior, it is not a bug to guard against.
	result := headSum + tailSum + uint64(size)
	return fmt.Sprintf("%016x", result), nil
}

// FromFile opens path, stats it, and hashes it with Compute.
func FromFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("oshash: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("oshash: %w", err)
	}
	return Compute(f, info.Size())
}

// sumUint64LE sums buf as consecutive little-endian uint64 words.
func sumUint64LE(buf []byte) (uint64, error) {
	if len(buf)%8 != 0 {
		return 0, errors.New("buffer is not a multiple of 8")
	}
	var sum uint64
	for i := 0; i+8 <= len(buf); i += 8 {
		sum += binary.LittleEndian.Uint64(buf[i : i+8])
	}
	return sum, nil
}

// readAtFull reads exactly len(buf) bytes at off. io.ReaderAt's contract
// allows a full read to come back with err == io.EOF (common for the last
// chunk of a file), so that combination is not itself an error — only a
// short read is.
func readAtFull(r io.ReaderAt, buf []byte, off int64) error {
	n, err := r.ReadAt(buf, off)
	if n == len(buf) {
		return nil
	}
	if err == nil {
		err = io.ErrUnexpectedEOF
	}
	return err
}
