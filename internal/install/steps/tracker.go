package steps

import (
	"os"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// tracker records original file bytes for rollback.
type tracker struct {
	orig  map[string][]byte
	order []string
}

func newTracker() *tracker { return &tracker{orig: make(map[string][]byte)} }

func (t *tracker) record(path string) error {
	if _, ok := t.orig[path]; ok {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.orig[path] = nil
			t.order = append(t.order, path)
			return nil
		}
		return err
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	t.orig[path] = cp
	t.order = append(t.order, path)
	return nil
}

func (t *tracker) write(path string, data []byte, perm os.FileMode) error {
	if err := t.record(path); err != nil {
		return err
	}
	_, err := filemerge.WriteFileAtomic(path, data, perm)
	return err
}

func (t *tracker) rollback() error {
	var lastErr error
	for i := len(t.order) - 1; i >= 0; i-- {
		p := t.order[i]
		orig, ok := t.orig[p]
		if !ok {
			continue
		}
		if orig == nil {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				lastErr = err
			}
		} else {
			if _, err := filemerge.WriteFileAtomic(p, orig, 0644); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func (t *tracker) reset() {
	t.orig = make(map[string][]byte)
	t.order = nil
}
