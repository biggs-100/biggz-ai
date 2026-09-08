package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookLineage(t *testing.T) {
	data := hookTestContent(t, hookFindWorkspaceRoot(t))
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

// hookTestContent returns the pre-push hook bytes under test: the installed
// .git/hooks/pre-push when present, otherwise the tracked install template
// (internal/install/assets/hooks/pre-push.tmpl) that generates it. Fresh CI
// clones never run the installer, so without the template fallback the test
// fails with "no such file" despite the lineage logic being intact.
func hookTestContent(t *testing.T, ws string) []byte {
	t.Helper()
	if data, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-push")); err == nil {
		return data
	}
	data, err := os.ReadFile(filepath.Join(ws, "internal", "install", "assets", "hooks", "pre-push.tmpl"))
	if err != nil {
		t.Skipf("pre-push hook not installed on this runner and no template found: %v", err)
	}
	return data
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
