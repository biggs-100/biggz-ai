// ODD lane projection (#add-odd-lane, Slice 2): odd/tasks/*.md as a
// READ-ONLY observability array for `biggz sdd-status`. It never feeds
// routing, nextRecommended, blockedReasons, or any gate, and scanning is
// non-fatal and side-effect free (no writes, no directory creation).
package sdd

import (
	"cmp"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// OddTaskProgress is one document's checkbox counts. Deliberately narrower
// than TaskProgress: allComplete would be vacuously true for a zero-checkbox
// document, which reads as a gate signal the ODD lane must never carry.
type OddTaskProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
}

// OddDocument is the read-only projection of one odd/tasks/<slug>.md.
type OddDocument struct {
	Path         string          `json:"path"`
	TaskProgress OddTaskProgress `json:"taskProgress"`
	LastTouched  string          `json:"lastTouched"`
}

// ScanOddDocuments lists regular .md files under <workspaceRoot>/odd/tasks,
// sorted by repo-relative path. Missing directories, unreadable files,
// non-regular entries, and non-.md names are skipped silently; the result is
// always non-nil so JSON renders `odd: []`. It never creates directories and
// never returns an error.
func ScanOddDocuments(workspaceRoot string) []OddDocument {
	dir := filepath.Join(workspaceRoot, "odd", "tasks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []OddDocument{}
	}
	docs := make([]OddDocument, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		progress := countTaskProgressText(string(content))
		docs = append(docs, OddDocument{
			Path:         path.Join("odd", "tasks", entry.Name()),
			TaskProgress: OddTaskProgress{Total: progress.Total, Completed: progress.Completed},
			LastTouched:  info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	slices.SortFunc(docs, func(a, b OddDocument) int { return cmp.Compare(a.Path, b.Path) })
	return docs
}

// RenderOddDocuments renders one `<path> — <completed>/<total> tasks —
// <lastTouched>` line per document; "" when no documents exist (the section
// is omitted).
func RenderOddDocuments(docs []OddDocument) string {
	if len(docs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("ODD tasks:\n")
	for _, doc := range docs {
		fmt.Fprintf(&b, "%s — %d/%d tasks — %s\n", doc.Path, doc.TaskProgress.Completed, doc.TaskProgress.Total, doc.LastTouched)
	}
	return b.String()
}
