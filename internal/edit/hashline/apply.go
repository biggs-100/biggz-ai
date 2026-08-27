package hashline

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// Hash4 returns the 4-hex prefix (upper) of a full SHA-256 hex digest.
func Hash4(fullSHA string) string {
	if len(fullSHA) < 4 {
		return strings.ToUpper(fullSHA)
	}
	return strings.ToUpper(fullSHA[:4])
}

// NoopLoopGuard returns true when newContent equals current range content,
// indicating the edit would be a no-op and must not trigger a write.
func NoopLoopGuard(current, newContent []byte) bool {
	return bytes.Equal(current, newContent)
}

// HashMismatchError signals a stale hash: expected != fresh. Code is always
// "needs_attention" and FreshHash carries Hash4 of the current range so the
// caller can retry. The file is not overwritten and the batch must continue.
type HashMismatchError struct {
	Code      string
	FreshHash string
	Path      string
	Expected  string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("needs_attention: %s expected %s fresh %s", e.Path, e.Expected, e.FreshHash)
}

func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i+1])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func extractRange(content []byte, start, end int) ([]byte, error) {
	if start < 1 || end < start {
		return nil, fmt.Errorf("invalid range %d.= %d", start, end)
	}
	lines := splitLines(content)
	if len(lines) == 0 {
		return []byte{}, nil
	}
	if start > len(lines) || end > len(lines) {
		return nil, fmt.Errorf("range %d.= %d out of bounds (file has %d lines)", start, end, len(lines))
	}
	seg := lines[start-1 : end]
	return bytes.Join(seg, nil), nil
}

// Apply validates seen, guards no-ops, checks the hash of the exact range
// against the directive's #A1B2, and on match writes atomically via
// WriteFileAtomic. On mismatch it returns HashMismatchError with freshHash
// without overwriting. CUT removes the range on match.
func Apply(path string, d Directive, seen [][2]int, snap *Store, newContent []byte) (string, error) {
	if err := ValidateSeen(d, seen); err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		content = nil
	}
	currentRange, err := extractRange(content, d.Start, d.End)
	if err != nil {
		// Treat out-of-bounds as hash mismatch with fresh hash of what exists
		// so caller gets freshHash. Fall back to hashing available content.
		freshFull := filemerge.ComputeHash(content)
		fresh4 := Hash4(freshFull)
		return fresh4, &HashMismatchError{Code: "needs_attention", FreshHash: fresh4, Path: path, Expected: d.HashTag}
	}
	if d.Op == OpPUT && NoopLoopGuard(currentRange, newContent) {
		fresh4 := Hash4(filemerge.ComputeHash(currentRange))
		return fresh4, nil
	}
	freshFull := filemerge.ComputeHash(currentRange)
	fresh4 := Hash4(freshFull)
	if fresh4 != d.HashTag {
		return fresh4, &HashMismatchError{Code: "needs_attention", FreshHash: fresh4, Path: path, Expected: d.HashTag}
	}
	// Hash matches — build the new file bytes
	var newFile []byte
	lines := splitLines(content)
	if d.Op == OpPUT {
		var head, tail []byte
		if d.Start > 1 {
			head = bytes.Join(lines[:d.Start-1], nil)
		}
		if d.End < len(lines) {
			tail = bytes.Join(lines[d.End:], nil)
		}
		newFile = append(head, newContent...)
		newFile = append(newFile, tail...)
	} else { // CUT
		var head, tail []byte
		if d.Start > 1 {
			head = bytes.Join(lines[:d.Start-1], nil)
		}
		if d.End < len(lines) {
			tail = bytes.Join(lines[d.End:], nil)
		}
		newFile = append(head, tail...)
	}
	perm := os.FileMode(0644)
	if fi, statErr := os.Stat(path); statErr == nil {
		perm = fi.Mode().Perm()
	}
	if _, wErr := filemerge.WriteFileAtomic(path, newFile, perm); wErr != nil {
		return "", wErr
	}
	if d.Op == OpPUT {
		return Hash4(filemerge.ComputeHash(newContent)), nil
	}
	return Hash4(filemerge.ComputeHash(newFile)), nil
}
