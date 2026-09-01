package bigmem

// Recent returns the most recent observations ordered by updated_at DESC.
// It reuses Search with an empty query which maps to ORDER BY o.updated_at DESC at bigmem.go:1801,
// while non-empty queries use ORDER BY rank at bigmem.go:1844 (BM25 relevance).
// Limit is clamped to 50 by Search.
func (s *Store) Recent(opts SearchOptions) ([]*Observation, error) {
	return s.Search("", opts)
}
