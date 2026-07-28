// Package gitdiff provides shared utilities for parsing git diff output.
// It extracts the common git diff logic used by multiple lens packages
// for analyzing code changes.
package gitdiff

// DiffFile holds the parsed statistics for a single file in a git diff.
type DiffFile struct {
	Path      string
	Additions int
	Deletions int
}
