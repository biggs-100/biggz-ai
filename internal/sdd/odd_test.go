package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedOddTask writes one fixture under root/odd/tasks with a pinned mtime.
func seedOddTask(t *testing.T, root, rel, content string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(root, "odd", "tasks", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
	}
}

// TestOddScanDocuments is SS-ODD-001-S1/S3 at the scanner boundary: checkbox
// counts (- [x] and - [X] complete), deterministic path sorting, RFC3339 UTC
// lastTouched, and the skip rules (non-.md, directories named *.md standing
// in for the unreadable-file path, missing directories, a non-directory
// odd/tasks). Zero-checkbox prose stays listed with 0/0, the result is never
// nil, and scanning never creates odd/.
func TestOddScanDocuments(t *testing.T) {
	t.Run("counts sorts and stamps UTC", func(t *testing.T) {
		root := t.TempDir()
		mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("UTC-5", -5*60*60))
		seedOddTask(t, root, "b.md", "- [ ] one\n- [ ] two\n", mod)
		seedOddTask(t, root, "a.md", "- [x] 1\n- [X] 2\n- [x] 3\n- [ ] 4\n- [ ] 5\n", mod)

		docs := ScanOddDocuments(root)
		if len(docs) != 2 {
			t.Fatalf("documents = %d, want 2: %#v", len(docs), docs)
		}
		if docs[0].Path != "odd/tasks/a.md" || docs[1].Path != "odd/tasks/b.md" {
			t.Fatalf("paths = [%q, %q], want sorted a.md,b.md", docs[0].Path, docs[1].Path)
		}
		if docs[0].TaskProgress != (OddTaskProgress{Total: 5, Completed: 3}) {
			t.Errorf("a.md progress = %#v, want 5/3", docs[0].TaskProgress)
		}
		if docs[1].TaskProgress != (OddTaskProgress{Total: 2, Completed: 0}) {
			t.Errorf("b.md progress = %#v, want 2/0", docs[1].TaskProgress)
		}
		parsed, err := time.Parse(time.RFC3339, docs[0].LastTouched)
		if err != nil {
			t.Fatalf("lastTouched %q is not RFC3339: %v", docs[0].LastTouched, err)
		}
		if !parsed.Equal(mod.UTC()) {
			t.Errorf("lastTouched = %s, want %s", parsed, mod.UTC())
		}
	})

	t.Run("skips non-documents and keeps prose", func(t *testing.T) {
		root := t.TempDir()
		seedOddTask(t, root, "listed.md", "- [x] done\n", time.Time{})
		seedOddTask(t, root, "script.sh", "- [x] done\n", time.Time{})
		seedOddTask(t, root, "prose.md", "no checkboxes\n", time.Time{})
		if err := os.MkdirAll(filepath.Join(root, "odd", "tasks", "dir.md"), 0o755); err != nil {
			t.Fatalf("mkdir dir.md: %v", err)
		}

		docs := ScanOddDocuments(root)
		if len(docs) != 2 || docs[0].Path != "odd/tasks/listed.md" || docs[1].Path != "odd/tasks/prose.md" {
			t.Fatalf("documents = %#v, want only listed.md and prose.md", docs)
		}
		if docs[0].TaskProgress != (OddTaskProgress{Total: 1, Completed: 1}) {
			t.Errorf("listed.md progress = %#v, want 1/1", docs[0].TaskProgress)
		}
		if docs[1].TaskProgress != (OddTaskProgress{}) {
			t.Errorf("prose.md progress = %#v, want 0/0", docs[1].TaskProgress)
		}
	})

	t.Run("missing directory is empty and read-only", func(t *testing.T) {
		root := t.TempDir()
		if docs := ScanOddDocuments(root); docs == nil || len(docs) != 0 {
			t.Fatalf("documents = %#v, want non-nil empty", docs)
		}
		if _, err := os.Stat(filepath.Join(root, "odd")); !os.IsNotExist(err) {
			t.Errorf("scan created odd/ (stat err = %v)", err)
		}
		if err := os.MkdirAll(filepath.Join(root, "odd"), 0o755); err != nil {
			t.Fatalf("mkdir odd: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "odd", "tasks"), []byte("file"), 0o644); err != nil {
			t.Fatalf("write odd/tasks: %v", err)
		}
		if docs := ScanOddDocuments(root); docs == nil || len(docs) != 0 {
			t.Fatalf("documents with file odd/tasks = %#v, want non-nil empty", docs)
		}
	})
}

// TestOddRenderDocuments pins the human line format and the omitted-empty
// section.
func TestOddRenderDocuments(t *testing.T) {
	if got := RenderOddDocuments(nil); got != "" {
		t.Errorf("RenderOddDocuments(nil) = %q, want empty", got)
	}
	got := RenderOddDocuments([]OddDocument{
		{Path: "odd/tasks/a.md", TaskProgress: OddTaskProgress{Total: 5, Completed: 3}, LastTouched: "2026-01-02T08:04:05Z"},
		{Path: "odd/tasks/b.md", TaskProgress: OddTaskProgress{Total: 2, Completed: 0}, LastTouched: "2026-01-03T00:00:00Z"},
	})
	for _, want := range []string{
		"odd/tasks/a.md — 3/5 tasks — 2026-01-02T08:04:05Z",
		"odd/tasks/b.md — 0/2 tasks — 2026-01-03T00:00:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q:\n%s", want, got)
		}
	}
}
