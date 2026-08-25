package lens

import "sync"

// Registry is the build-time lens registry populated at cmd/biggz init.
// It maps stable lens IDs (risk, resilience, readability, reliability) to
// their Lens implementations. Duplicates last-win; unknown IDs are skipped
// by Ordered. Access is synchronized for test isolation; production use is
// single-threaded at init.

var (
	mu       sync.RWMutex
	registry = map[string]Lens{}
)

// RegisterLens registers a Lens under its ID. Duplicate IDs last-win
// deterministically; the last caller wins.
func RegisterLens(l Lens) {
	mu.Lock()
	defer mu.Unlock()
	registry[l.ID()] = l
}

// Ordered returns the lenses for the given IDs in the exact input order,
// skipping unknown IDs silently. The returned slice aliases the registry
// entries but not the registry map itself.
func Ordered(ids []string) []Lens {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Lens, 0, len(ids))
	for _, id := range ids {
		if l, ok := registry[id]; ok {
			out = append(out, l)
		}
	}
	return out
}

// Registry returns a shallow copy of the current registry map.
// Callers must not mutate the returned map; use RegisterLens to modify.
func Registry() map[string]Lens {
	mu.RLock()
	defer mu.RUnlock()
	copy := make(map[string]Lens, len(registry))
	for k, v := range registry {
		copy[k] = v
	}
	return copy
}

// ResetRegistry clears the registry. Exported for tests only to ensure
// isolated ordered/last-win/skip scenarios.
func ResetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	registry = map[string]Lens{}
}
