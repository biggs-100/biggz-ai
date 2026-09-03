package bigmem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Export must bundle blob bytes so import into a fresh root restores content
// instead of inserting an orphan blob:sha256: ref that Get hands back raw.
func TestSyncExportImport_BlobRoundTrip(t *testing.T) {
	homeA := t.TempDir()
	t.Setenv("HOME", homeA)
	t.Setenv("USERPROFILE", homeA)
	storeA, err := Open("")
	if err != nil {
		t.Fatalf("Open A: %v", err)
	}
	tail := "BLOB-ROUNDTRIP-TAIL"
	content := strings.Repeat("q", 150*1024-len(tail)) + tail
	addr, err := PutBlob([]byte(content))
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	obs := &Observation{Title: "blob roundtrip", Type: "note", Content: addr, Project: "roundtrip"}
	if err := storeA.Save(obs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	id := obs.ID
	projectRoot := t.TempDir()
	if err := storeA.SyncExport("roundtrip", projectRoot); err != nil {
		t.Fatalf("SyncExport: %v", err)
	}
	storeA.Close()

	// Fresh root: new HOME, empty store, same transport dir.
	homeB := t.TempDir()
	t.Setenv("HOME", homeB)
	t.Setenv("USERPROFILE", homeB)
	storeB, err := Open("")
	if err != nil {
		t.Fatalf("Open B: %v", err)
	}
	defer storeB.Close()
	n, err := storeB.SyncImport(projectRoot)
	if err != nil {
		t.Fatalf("SyncImport: %v", err)
	}
	if n == 0 {
		t.Fatalf("SyncImport imported 0 rows")
	}
	got, err := storeB.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != content {
		t.Fatalf("roundtrip loss: got %d bytes want %d", len(got.Content), len(content))
	}
	if !strings.HasSuffix(got.Content, tail) {
		t.Fatal("tail marker lost in export/import roundtrip")
	}
}

// A blob ref without bytes on import must produce a visible error and must
// not be inserted as an orphan row.
func TestSyncImport_OrphanBlobRefReported(t *testing.T) {
	_ = isolatedHome(t)
	store, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	projectRoot := t.TempDir()
	dir := filepath.Join(projectRoot, ".bigmem")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	missing := BlobPrefix + strings.Repeat("0", 64)
	obs := Observation{ID: "obs-orphan-1", Title: "orphan", Type: "note", Content: missing, Project: "orphan"}
	line, _ := json.Marshal(&obs)
	if err := os.WriteFile(filepath.Join(dir, "sync-orphan.ndjson"), append(line, '\n'), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	n, err := store.SyncImport(projectRoot)
	if err == nil {
		t.Fatalf("SyncImport must report a visible error for orphan blob ref, got nil (n=%d)", n)
	}
	var count int
	_ = store.db.QueryRow("SELECT COUNT(*) FROM observations WHERE id=?", "obs-orphan-1").Scan(&count)
	if count != 0 {
		t.Fatalf("orphan blob ref was inserted (%d rows)", count)
	}
}
