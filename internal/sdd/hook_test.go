package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookLineage(t *testing.T) {
	ws := hookFindWorkspaceRoot(t)
	hookPath := filepath.Join(ws, ".git", "hooks", "pre-push")
	if _, err := os.Stat(hookPath); err != nil {
		hookPath = filepath.Join(ws, ".git", "hooks", "pre-push")
	}
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ls -t") {
		t.Fatalf("hook must contain ls -t, got %q", content)
	}
	if !strings.Contains(content, "git merge-base --is-ancestor") {
		t.Fatalf("hook must contain git merge-base --is-ancestor, got %q", content)
	}
	if strings.Contains(content, "for d in \"$git_common/biggz/review-transactions\"/*; do") && strings.Contains(content, "break") {
		// Old naive pattern should not be present as primary selector
		if !strings.Contains(content, "candidates=$(ls -t") {
			t.Fatalf("hook still uses naive for d; break without ls -t filtering")
		}
	}
	// Ghost 019fbb3a should not be hard-deleted
	if strings.Contains(content, "rm") && strings.Contains(content, "019fbb3a") {
		t.Fatalf("hook must not auto-delete ghosts via rm.*019fbb3a")
	}
	if !strings.Contains(content, "[[:space:]]*") {
		t.Fatalf("hook must grep with [[:space:]]*")
	}
	if !strings.Contains(content, "\"delivery\"[[:space:]]*:[[:space:]]*\"disabled\"") {
		// Allow variant with space
		if !strings.Contains(content, `"delivery"[[:space:]]`) {
			t.Fatalf("hook missing space-tolerant delivery grep")
		}
	}
	if !strings.Contains(content, "\"allowed\"[[:space:]]*:[[:space:]]*false") {
		if !strings.Contains(content, `"allowed"[[:space:]]`) {
			t.Fatalf("hook missing space-tolerant allowed grep")
		}
	}
}

func hookFindWorkspaceRoot(t *testing.T) string {
	t.Helper()
	// Walk up from this file's dir to find openspec
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "openspec")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// fallback to relative
			if wd, err := os.Getwd(); err == nil {
				return wd
			}
			return "."
		}
		dir = parent
	}
}
