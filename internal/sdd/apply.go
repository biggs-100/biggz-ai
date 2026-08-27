package sdd

import (
	"bytes"
	"sync"

	"github.com/biggs-100/biggz-ai/internal/edit/hashline"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

var (
	modeMu     sync.RWMutex
	editMode   = "legacy"
	seenMu     sync.Mutex
	seenRanges = map[string][][2]int{}
	snap       = &hashline.Store{}
)

// SetEditMode sets the edit mode flag. Use "hashline" to enable hashline-lite.
func SetEditMode(m string) {
	modeMu.Lock()
	defer modeMu.Unlock()
	editMode = m
}

// GetEditMode returns the current edit mode.
func GetEditMode() string {
	modeMu.RLock()
	defer modeMu.RUnlock()
	return editMode
}

// IsHashlineMode reports whether hashline-lite is active.
func IsHashlineMode() bool {
	return GetEditMode() == "hashline"
}

// HookRead captures the seen range and snapshot for path. It should be called
// on every file read during sdd-apply. The seen range is [1..numLines].
func HookRead(path string, content []byte) {
	seenMu.Lock()
	defer seenMu.Unlock()
	snap.Capture(path, content)
	n := bytes.Count(content, []byte("\n"))
	if len(content) > 0 && content[len(content)-1] != '\n' {
		n++
	}
	if n == 0 {
		seenRanges[path] = [][2]int{}
		return
	}
	seenRanges[path] = [][2]int{{1, n}}
}

// ClearBatch clears seen ranges and snapshots after a batch.
func ClearBatch() {
	seenMu.Lock()
	defer seenMu.Unlock()
	seenRanges = map[string][][2]int{}
	snap.Clear()
}

// ApplyEdit routes a directive through hashline when enabled, otherwise falls
// back to legacy atomic write. On parse error it falls back to legacy. On hash
// mismatch it returns HashMismatchError with freshHash and no overwrite; batch
// callers must continue applying remaining files.
func ApplyEdit(path, directive string, newContent []byte) (string, error) {
	if !IsHashlineMode() {
		_, err := filemerge.WriteFileAtomic(path, newContent, 0644)
		return "", err
	}
	d, err := hashline.Parse(directive)
	if err != nil {
		// transparent fallback
		_, ferr := filemerge.WriteFileAtomic(path, newContent, 0644)
		if ferr != nil {
			return "", ferr
		}
		return "", nil
	}
	seenMu.Lock()
	seen := append([][2]int(nil), seenRanges[path]...)
	seenMu.Unlock()
	fresh, applyErr := hashline.Apply(path, d, seen, snap, newContent)
	return fresh, applyErr
}
