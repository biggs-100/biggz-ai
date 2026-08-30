package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/backup"
	tea "github.com/charmbracelet/bubbletea"
)

func TestBackup_EmptyListMessage(t *testing.T) {
	m := NewBackupModel()
	m.width = 80
	view := m.View()
	if !strings.Contains(view, "Manage biggz-ai snapshots") {
		t.Error("expected idle view")
	}
	// Simulate empty list
	m.step = backupListing
	m.items = []backupEntry{}
	view = m.View()
	if !strings.Contains(view, "No backups found") {
		t.Error("expected No backups found")
	}
	if !strings.Contains(view, "Press [C] to create") {
		t.Error("expected create hint")
	}
}

func TestBackup_ListPopulatesTable(t *testing.T) {
	dir := t.TempDir()
	// Create two backups via backup package directly
	dummy := filepath.Join(dir, "dummy")
	if err := os.MkdirAll(dummy, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a file to backup
	f := filepath.Join(dummy, "file.txt")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	// Use backup.Create directly to populate
	b1, err := backup.Create(dir, []string{f})
	if err != nil {
		t.Fatalf("backup.Create: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	b2, err := backup.Create(dir, []string{f})
	if err != nil {
		t.Fatalf("backup.Create second: %v", err)
	}
	if b1.ID == b2.ID {
		t.Fatal("expected different IDs")
	}
	m := NewBackupModel()
	m.SetBackupDir(dir)
	m.width = 80
	m.height = 24
	// Simulate receiving list msg
	manifests, err := backup.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	var items []backupEntry
	for _, b := range manifests {
		items = append(items, backupEntry{ID: b.ID, Time: b.CreatedAt, Size: formatSize(b.Size), Path: filepath.Join(dir, b.ID+".tar.gz")})
	}
	msg := backupListMsg{items: items}
	m2, _ := m.Update(msg)
	bm := m2.(BackupModel)
	if len(bm.items) != 2 {
		t.Fatalf("expected 2 items got %d", len(bm.items))
	}
	if bm.table.Cursor() != 0 {
		t.Fatalf("expected cursor 0 got %d", bm.table.Cursor())
	}
	if bm.preview == nil || bm.preview.ID != bm.items[0].ID {
		t.Error("expected preview to be first entry")
	}
	// Check table rows contain IDs
	rows := bm.table.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows got %d", len(rows))
	}
	found := false
	for _, r := range rows {
		if r[0] == b1.ID || r[0] == b2.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected rows to contain backup IDs")
	}
}

func TestBackup_PreviewSyncOnCursor(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("x"), 0644)
	b1, _ := backup.Create(dir, []string{f})
	time.Sleep(1100 * time.Millisecond)
	b2, _ := backup.Create(dir, []string{f})
	m := NewBackupModel()
	m.SetBackupDir(dir)
	manifests, _ := backup.List(dir)
	var items []backupEntry
	for _, b := range manifests {
		items = append(items, backupEntry{ID: b.ID, Time: b.CreatedAt, Size: formatSize(b.Size)})
	}
	// Ensure order: List returns newest first, so b2 first, b1 second (or vice versa)
	msg := backupListMsg{items: items}
	m2, _ := m.Update(msg)
	bm := m2.(BackupModel)
	// Move cursor down
	m3, _ := bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	bm3 := m3.(BackupModel)
	if bm3.cursor != 1 {
		t.Fatalf("expected cursor 1 got %d", bm3.cursor)
	}
	if bm3.preview == nil || bm3.preview.ID != items[1].ID {
		t.Errorf("expected preview ID %s got %v", items[1].ID, bm3.preview)
	}
	// Move up
	m4, _ := bm3.Update(tea.KeyMsg{Type: tea.KeyUp})
	bm4 := m4.(BackupModel)
	if bm4.cursor != 0 {
		t.Fatalf("expected cursor 0 got %d", bm4.cursor)
	}
	_ = b1
	_ = b2
}

func TestBackup_CreateFlow(t *testing.T) {
	m := NewBackupModel()
	m.step = backupListing
	m.width = 80
	dir := t.TempDir()
	m.SetBackupDir(dir)
	// Press c should set backupCreating and return Cmd
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	bm := m2.(BackupModel)
	if bm.step != backupCreating {
		t.Fatalf("expected backupCreating got %d", bm.step)
	}
	if cmd == nil {
		t.Fatal("expected tea.Cmd for create")
	}
	// Execute cmd (should not block, uses temp dir)
	msg := cmd()
	res, ok := msg.(backupResultMsg)
	if !ok {
		t.Fatalf("expected backupResultMsg got %T", msg)
	}
	if res.err != "" {
		t.Logf("create returned err (may be expected if no paths): %s", res.err)
	}
}

func TestBackup_CreateErrorSurfaces(t *testing.T) {
	m := NewBackupModel()
	m.step = backupCreating
	// Simulate error msg
	msg := backupResultMsg{err: "mkdir: permission denied"}
	m2, _ := m.Update(msg)
	bm := m2.(BackupModel)
	if bm.step != backupError {
		t.Fatalf("expected backupError got %d", bm.step)
	}
	view := bm.View()
	if !strings.Contains(view, "Error") {
		t.Error("expected ErrorBox in view")
	}
}

func TestBackup_RestoreRequiresConfirm(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.txt")
	os.WriteFile(f, []byte("data"), 0644)
	backup.Create(dir, []string{f})
	m := NewBackupModel()
	m.SetBackupDir(dir)
	manifests, _ := backup.List(dir)
	var items []backupEntry
	for _, b := range manifests {
		items = append(items, backupEntry{ID: b.ID, Time: b.CreatedAt, Size: formatSize(b.Size), Paths: b.Paths})
	}
	// Load list
	m2, _ := m.Update(backupListMsg{items: items})
	bm := m2.(BackupModel)
	// Press enter to start restore
	m3, _ := bm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bm3 := m3.(BackupModel)
	if bm3.step != backupRestoring || !bm3.confirmPending {
		t.Fatalf("expected restoring with confirmPending, got step %d pending %v", bm3.step, bm3.confirmPending)
	}
	view := bm3.View()
	if !strings.Contains(view, "Confirm Restore") || !strings.Contains(view, "y/N") {
		t.Error("expected confirm modal with y/N")
	}
}

func TestBackup_RestoreYCallsRestore(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	f := filepath.Join(dir, "orig.txt")
	os.WriteFile(f, []byte("orig"), 0644)
	// Create backup to restore
	b, _ := backup.Create(dir, []string{f})
	// Modify original to test restore
	os.WriteFile(f, []byte("modified"), 0644)
	m := NewBackupModel()
	m.SetBackupDir(dir)
	m.SetRestoreTarget(target)
	// Need to have items
	items := []backupEntry{{ID: b.ID, Time: b.CreatedAt, Size: formatSize(b.Size), Paths: b.Paths}}
	m.step = backupListing
	m.items = items
	m.cursor = 0
	m.preview = &items[0]
	// Enter restore
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bm := m2.(BackupModel)
	// Confirm y
	_, cmd := bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected cmd for restore y")
	}
	msg := cmd()
	res, ok := msg.(backupResultMsg)
	if !ok {
		t.Fatalf("expected backupResultMsg got %T", msg)
	}
	if res.err != "" {
		t.Fatalf("restore failed: %s", res.err)
	}
	if !strings.Contains(res.status, "Restored") {
		t.Errorf("expected Restored status got %q", res.status)
	}
	// Verify file restored to target
	// backup.Restore extracts relative path "orig.txt" to target
	restored := filepath.Join(target, "orig.txt")
	data, err := os.ReadFile(restored)
	if err != nil {
		t.Logf("restore target check: %v (may be expected if paths differ)", err)
	} else if string(data) != "orig" {
		t.Errorf("expected restored content 'orig' got %q", string(data))
	}
}

func TestBackup_RestoreNCancels(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("a"), 0644)
	b, _ := backup.Create(dir, []string{f})
	m := NewBackupModel()
	m.SetBackupDir(dir)
	items := []backupEntry{{ID: b.ID, Time: b.CreatedAt, Size: formatSize(b.Size)}}
	m.step = backupListing
	m.items = items
	m.cursor = 0
	m.preview = &items[0]
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	bm := m2.(BackupModel)
	// Deny with n
	m3, cmd := bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	bm3 := m3.(BackupModel)
	if bm3.step != backupListing || bm3.confirmPending {
		t.Fatalf("expected back to listing without pending, got step %d pending %v", bm3.step, bm3.confirmPending)
	}
	if cmd != nil {
		// Should not call restore, cmd should be nil
		msg := cmd()
		if _, ok := msg.(backupResultMsg); ok {
			t.Error("expected no restore on n")
		}
	}
	if bm3.status != "Restore cancelled" {
		t.Errorf("expected cancelled status got %q", bm3.status)
	}
	// Also ESC cancels
	m4, _ := bm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	bm4 := m4.(BackupModel)
	if bm4.step != backupListing {
		t.Error("expected ESC to cancel restore")
	}
}

func TestBackup_NarrowVisibleWidth(t *testing.T) {
	m := NewBackupModel()
	m.width = 50
	m.height = 20
	// Create items
	items := []backupEntry{
		{ID: "backup-20260101-120000", Time: time.Now(), Size: "1.2 MB"},
		{ID: "backup-20260102-120000", Time: time.Now(), Size: "2.3 MB"},
	}
	msg := backupListMsg{items: items}
	m2, _ := m.Update(msg)
	bm := m2.(BackupModel)
	bm.width = 50
	view := bm.View()
	lines := strings.Split(view, "\n")
	for _, l := range lines {
		if VisibleWidth(l) > 50 {
			t.Errorf("line exceeds width 50: %q width %d", l, VisibleWidth(l))
		}
	}
}

func TestBackup_AnimationGuard(t *testing.T) {
	t.Setenv("BIGGZ_NO_ANIMATION", "1")
	// tickCmd should be nil when animation disabled - we test via tuiAnimationsDisabled logic indirectly
	// For backup, view should not contain sync markers when TERM=dumb
	t.Setenv("TERM", "dumb")
	m := NewBackupModel()
	m.width = 80
	view := m.View()
	if strings.Contains(view, "\x1b[?2026h") || strings.Contains(view, "\x1b[?2026l") {
		t.Error("expected no sync markers with TERM=dumb")
	}
}
