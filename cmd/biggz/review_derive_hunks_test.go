package main

// Task 1.5: deriveLensHunks must be wired to the frozen-tree inspector so
// buildLensInput receives real per-path hunks derived from frozen trees, and
// a parser-error candidate produces non-empty readability findings over the
// materialized hunks (non-vacuity).

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	readability "github.com/biggs-100/biggz-ai/internal/review/lens/readability"
)

// deriveHunksFixture builds a two-commit temp repository whose candidate adds
// a Go file that fails go/parser and touches a text file.
func deriveHunksFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test")
	runGit(t, repo, "config", "user.email", "test@test.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(repo, "broken.go"), []byte("package broken\nfunc broken( {\n"), 0644); err != nil {
		t.Fatalf("write broken.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\nchange\n"), 0644); err != nil {
		t.Fatalf("write README change: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "candidate")
	return repo, runGitOutput(t, repo, "rev-parse", "HEAD")
}

func TestDeriveLensHunksFeedLensFindings(t *testing.T) {
	repo, sha := deriveHunksFixture(t)

	hunks, err := deriveLensHunks(repo, sha, "")
	if err != nil {
		t.Fatalf("deriveLensHunks: %v", err)
	}
	if len(hunks) == 0 {
		t.Fatal("deriveLensHunks returned no hunks for a two-path candidate")
	}
	paths := hunkPaths(hunks)
	for _, want := range []string{"README.md", "broken.go"} {
		if !slices.Contains(paths, want) {
			t.Fatalf("hunks are missing candidate path %q: %v", want, paths)
		}
	}
	broken := hunks["broken.go"]
	if len(broken) == 0 {
		t.Fatal("hunks are missing the parser-error candidate file")
	}
	if !bytes.Contains(broken, []byte("+func broken( {")) {
		t.Fatalf("materialized hunk does not carry the candidate change:\n%s", broken)
	}

	lensInput, err := buildLensInput(repo, sha, "", hunks)
	if err != nil {
		t.Fatalf("buildLensInput: %v", err)
	}
	if len(lensInput.Hunks) != len(hunks) {
		t.Fatalf("LensInput hunks = %d, want %d", len(lensInput.Hunks), len(hunks))
	}
	if !slices.Contains(lensInput.Paths, "broken.go") {
		t.Fatalf("LensInput paths are missing the candidate file: %v", lensInput.Paths)
	}

	result, err := (&readability.Lens{}).Analyze(t.Context(), lensInput)
	if err != nil {
		t.Fatalf("readability.Analyze: %v", err)
	}
	if len(result.Findings) == 0 {
		t.Fatalf("readability findings are empty over the materialized hunks: %+v", result)
	}
}

func hunkPaths(hunks map[string][]byte) []string {
	paths := make([]string, 0, len(hunks))
	for path := range hunks {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}
