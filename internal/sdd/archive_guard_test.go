package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchiveNeverDisable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("archive.go"))
	if err != nil {
		t.Fatalf("read archive.go: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "RDDDisable") || strings.Contains(content, "SetCloneLocalRDDMode") || strings.Contains(content, "RDDEnable") {
		t.Fatalf("archive.go must not call RDDDisable/SetCloneLocalRDDMode/RDDEnable, got %q", content)
	}
	if strings.Contains(content, "rdd-mode") {
		t.Fatalf("archive.go must not write .git/biggz/rdd-mode, got %q", content)
	}
	if !strings.Contains(content, "never auto-disable RDD") {
		t.Fatalf("archive.go missing guard comment // never auto-disable RDD")
	}
	if !strings.Contains(content, "os.Rename") {
		t.Fatalf("archive.go must use os.Rename")
	}
}

func TestArchiveMtime(t *testing.T) {
	ws := t.TempDir()
	openspecRoot := filepath.Join(ws, "openspec")
	change := "mtime-change"
	src := filepath.Join(openspecRoot, "changes", change)
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// create file to get mtime
	f := filepath.Join(src, "proposal.md")
	_ = os.WriteFile(f, []byte("# Proposal\n"), 0644)
	// set mtime T0
	t0 := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_ = os.Chtimes(src, t0, t0)
	beforeStat, _ := os.Stat(src)
	_ = beforeStat
	// archive
	dst, err := ArchiveChange(openspecRoot, change)
	if err != nil {
		t.Fatalf("ArchiveChange: %v", err)
	}
	afterStat, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	// mtime should be preserved (os.Rename preserves)
	if !afterStat.ModTime().Equal(t0) {
		// On some filesystems Rename may preserve but we check close
		if afterStat.ModTime().Sub(t0) > time.Second {
			t.Fatalf("mtime not preserved: got %v want %v", afterStat.ModTime(), t0)
		}
	}
	// Ensure RDD still enabled (global should remain enabled unless test disabled)
	// We just check archive didn't create rdd-mode file under .git
	if _, err := os.Stat(filepath.Join(ws, ".git", "biggz", "rdd-mode")); err == nil {
		t.Fatalf("archive created rdd-mode file")
	}
}
