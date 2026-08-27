package hashline

import (
	"os"
	"sync"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

// Store keeps a bounded per-batch snapshot of file contents captured by the
// read hook. It is cleared after the batch completes, so growth is bounded
// by the number of files touched in that batch.
type Store struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *Store) ensure() {
	if s.m == nil {
		s.m = make(map[string][]byte)
	}
}

// Capture stores a copy of content for path.
func (s *Store) Capture(path string, content []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensure()
	cp := make([]byte, len(content))
	copy(cp, content)
	s.m[path] = cp
}

// Restore writes the snapshot for path back to disk via WriteFileAtomic.
// It returns an error when no snapshot exists for path.
func (s *Store) Restore(path string) error {
	s.mu.Lock()
	data, ok := s.m[path]
	s.mu.Unlock()
	if !ok {
		return os.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	perm := os.FileMode(0644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	_, err := filemerge.WriteFileAtomic(path, cp, perm)
	return err
}

// Clear removes all snapshots.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m = make(map[string][]byte)
}

// Size returns the number of entries currently held.
func (s *Store) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// Get returns a copy of the snapshot for path and whether it exists.
func (s *Store) Get(path string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[path]
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, true
}
