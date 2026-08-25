package bigmem

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gzipDataForTest(data []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

func TestSyncIDToID_TableDriven(t *testing.T) {
	cases := []struct {
		name   string
		obs    EngramObservation
		wantID string
		check  func(got string) bool
	}{
		{
			name:   "sync_id preserved id=42 ignored",
			obs:    EngramObservation{ID: 42, SyncID: "obs-abc123", Title: "T", Content: "C"},
			wantID: "obs-abc123",
		},
		{
			name: "empty sync_id fallback deterministic",
			obs:  EngramObservation{ID: 99, SyncID: "", Title: "hello", Content: "world"},
			check: func(got string) bool {
				h := sha256.Sum256([]byte("helloworld"))
				want := "engram-" + hex.EncodeToString(h[:6])
				return got == want
			},
		},
		{
			name: "empty sync_id same content same hash",
			obs:  EngramObservation{ID: 1, SyncID: "", Title: "same", Content: "same"},
			check: func(got string) bool {
				h := sha256.Sum256([]byte("samesame"))
				want := "engram-" + hex.EncodeToString(h[:6])
				if got != want {
					return false
				}
				// deterministic second call
				h2 := sha256.Sum256([]byte("samesame"))
				want2 := "engram-" + hex.EncodeToString(h2[:6])
				return got == want2
			},
		},
		{
			name:   "sync_id with spaces trimmed",
			obs:    EngramObservation{ID: 5, SyncID: "  obs-xyz  ", Title: "T", Content: "C"},
			wantID: "obs-xyz",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := syncIDToID(tc.obs)
			if tc.check != nil {
				if !tc.check(got) {
					t.Fatalf("syncIDToID() = %q, check failed", got)
				}
			} else if got != tc.wantID {
				t.Fatalf("syncIDToID() = %q, want %q", got, tc.wantID)
			}
			// int64 ID must be ignored: same sync_id with different int64 gives same result
			if tc.obs.SyncID != "" {
				alt := tc.obs
				alt.ID = tc.obs.ID + 9999
				if got2 := syncIDToID(alt); got2 != got {
					t.Fatalf("int64 id should be ignored, got %q vs %q", got2, got)
				}
			}
		})
	}
}

func TestEngramFileTransport_ReadManifestAndChunk(t *testing.T) {
	dir := t.TempDir()
	engramDir := filepath.Join(dir, ".engram")
	_ = os.MkdirAll(filepath.Join(engramDir, "chunks"), 0755)

	manifest := SyncManifest{Version: 1, Chunks: []ChunkEntry{{ID: "a3f8c1d2", CreatedBy: "tester"}}}
	data, _ := json.Marshal(manifest)
	_ = os.WriteFile(filepath.Join(engramDir, "manifest.json"), data, 0644)

	chunkPayload := engramChunkData{
		Sessions: []engramSession{{ID: "sess-1", Project: "biggz-ai", StartedAt: "2026-01-01T00:00:00Z"}},
	}
	raw, _ := json.Marshal(chunkPayload)
	gz := gzipDataForTest(raw)
	_ = os.WriteFile(filepath.Join(engramDir, "chunks", "a3f8c1d2.jsonl.gz"), gz, 0644)

	transport := NewEngramFileTransport(engramDir)
	m, err := transport.ReadManifest()
	if err != nil {
		t.Fatalf("ReadManifest error: %v", err)
	}
	if len(m.Chunks) != 1 || m.Chunks[0].ID != "a3f8c1d2" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	chunkData, err := transport.ReadChunk("a3f8c1d2")
	if err != nil {
		t.Fatalf("ReadChunk error: %v", err)
	}
	dec, err := GunzipData(chunkData)
	if err != nil {
		t.Fatalf("GunzipData error: %v", err)
	}
	var decoded engramChunkData
	if err := json.Unmarshal(dec, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(decoded.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(decoded.Sessions))
	}
}

func TestResolveEngramDir(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
	}{
		{"/tmp/.engram", false},
		{".engram", false},
		{"", false},
		{"../../etc/.engram", true},
		{"../.engram", true},
	}
	for _, tc := range cases {
		_, err := ResolveEngramDir(tc.input)
		if tc.wantErr && err == nil {
			t.Errorf("ResolveEngramDir(%q) expected error, got nil", tc.input)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("ResolveEngramDir(%q) unexpected error: %v", tc.input, err)
		}
	}
}

func buildEngramDir(t *testing.T, chunks map[string]engramChunkData, manifestOrder []string) string {
	t.Helper()
	dir := t.TempDir()
	engramDir := filepath.Join(dir, ".engram")
	_ = os.MkdirAll(filepath.Join(engramDir, "chunks"), 0755)
	var entries []ChunkEntry
	for _, id := range manifestOrder {
		entries = append(entries, ChunkEntry{ID: id, CreatedBy: "tester", CreatedAt: "2026-01-01T00:00:00Z"})
		ch := chunks[id]
		raw, _ := json.Marshal(ch)
		gz := gzipDataForTest(raw)
		_ = os.WriteFile(filepath.Join(engramDir, "chunks", id+".jsonl.gz"), gz, 0644)
	}
	manifest := SyncManifest{Version: 1, Chunks: entries}
	data, _ := json.Marshal(manifest)
	_ = os.WriteFile(filepath.Join(engramDir, "manifest.json"), data, 0644)
	return engramDir
}

func TestImportFromEngram_ProjectFilter(t *testing.T) {
	store := openTestStore(t)
	biggz := "biggz-ai"
	other := "other"
	chunks := map[string]engramChunkData{
		"chunk1": {
			Observations: []EngramObservation{
				{ID: 1, SyncID: "obs-1", Title: "A", Content: "c1", Project: &biggz},
				{ID: 2, SyncID: "obs-2", Title: "B", Content: "c2", Project: &other},
			},
		},
		"chunk2": {
			Observations: []EngramObservation{
				{ID: 3, SyncID: "obs-3", Title: "C", Content: "c3", Project: &biggz},
			},
		},
	}
	engramDir := buildEngramDir(t, chunks, []string{"chunk1", "chunk2"})

	res, err := store.ImportFromEngram(engramDir, "biggz-ai")
	if err != nil {
		t.Fatalf("ImportFromEngram error: %v", err)
	}
	if res.ObservationsImported != 2 {
		t.Fatalf("ObservationsImported = %d, want 2 (only biggz-ai)", res.ObservationsImported)
	}
	// Verify only biggz-ai inserted
	if _, err := store.Get("obs-1"); err != nil {
		t.Fatalf("obs-1 should exist: %v", err)
	}
	if _, err := store.Get("obs-3"); err != nil {
		t.Fatalf("obs-3 should exist: %v", err)
	}
	if _, err := store.Get("obs-2"); err == nil {
		t.Fatalf("obs-2 (other) should NOT be imported with filter")
	}
}

func TestImportFromEngram_DedupAndFallback(t *testing.T) {
	store := openTestStore(t)
	proj := "biggz-ai"
	// Empty sync_id fallback
	ch := engramChunkData{
		Observations: []EngramObservation{
			{ID: 10, SyncID: "", Title: "hello", Content: "world", Project: &proj},
		},
	}
	engramDir := buildEngramDir(t, map[string]engramChunkData{"abcd1234": ch}, []string{"abcd1234"})
	res, err := store.ImportFromEngram(engramDir, "")
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if res.ChunksImported != 1 {
		t.Fatalf("expected 1 chunk imported, got %d", res.ChunksImported)
	}
	h := sha256.Sum256([]byte("helloworld"))
	wantID := "engram-" + hex.EncodeToString(h[:6])
	if _, err := store.Get(wantID); err != nil {
		t.Fatalf("fallback ID %q not found: %v", wantID, err)
	}
	// Re-import should be dedup no-op
	res2, err := store.ImportFromEngram(engramDir, "")
	if err != nil {
		t.Fatalf("re-import error: %v", err)
	}
	if res2.ChunksSkipped != 1 || res2.ChunksImported != 0 {
		t.Fatalf("re-import dedup failed: %+v", res2)
	}
	// Verify sync_chunks recorded
	known, _ := store.GetSyncChunks("engram")
	if !known["abcd1234"] {
		t.Fatalf("sync_chunks missing engram chunk")
	}
}

func TestImportFromEngram_CorruptChunkWarnContinue(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	engramDir := filepath.Join(dir, ".engram")
	_ = os.MkdirAll(filepath.Join(engramDir, "chunks"), 0755)

	proj := "biggz-ai"
	goodChunk := engramChunkData{
		Observations: []EngramObservation{{ID: 1, SyncID: "obs-good", Title: "good", Content: "ok", Project: &proj}},
	}
	raw, _ := json.Marshal(goodChunk)
	gz := gzipDataForTest(raw)
	_ = os.WriteFile(filepath.Join(engramDir, "chunks", "good1234.jsonl.gz"), gz, 0644)
	// Bad chunk: invalid gzip
	_ = os.WriteFile(filepath.Join(engramDir, "chunks", "bad5678.jsonl.gz"), []byte("not gzip"), 0644)

	manifest := SyncManifest{Version: 1, Chunks: []ChunkEntry{
		{ID: "bad5678", CreatedBy: "t"}, {ID: "good1234", CreatedBy: "t"},
	}}
	data, _ := json.Marshal(manifest)
	_ = os.WriteFile(filepath.Join(engramDir, "manifest.json"), data, 0644)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	res, err := store.ImportFromEngram(engramDir, "")
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	stderr := buf.String()
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if res.ChunksImported != 1 {
		t.Fatalf("expected 1 chunk imported despite corrupt, got %d", res.ChunksImported)
	}
	if !strings.Contains(stderr, "chunk bad5678") {
		t.Fatalf("stderr should warn with chunk bad5678, got: %q", stderr)
	}
	if _, err := store.Get("obs-good"); err != nil {
		t.Fatalf("good chunk should import")
	}
}

func TestImportFromEngram_StubSession(t *testing.T) {
	store := openTestStore(t)
	proj := "biggz-ai"
	ch := engramChunkData{
		Observations: []EngramObservation{
			{ID: 1, SyncID: "obs-orphan", Title: "orphan", Content: "c", SessionID: "missing-sess", Project: &proj},
		},
	}
	engramDir := buildEngramDir(t, map[string]engramChunkData{"stub0001": ch}, []string{"stub0001"})
	_, err := store.ImportFromEngram(engramDir, "")
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	var cnt int
	_ = store.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", "missing-sess").Scan(&cnt)
	if cnt != 1 {
		t.Fatalf("stub session not created")
	}
	var dir string
	_ = store.db.QueryRow("SELECT directory FROM sessions WHERE id = ?", "missing-sess").Scan(&dir)
	if dir != "(recovered-missing-session)" {
		t.Fatalf("stub directory = %q, want (recovered-missing-session)", dir)
	}
}

func TestImportFromEngram_MissingManifest(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	engramDir := filepath.Join(dir, ".engram")
	_ = os.MkdirAll(engramDir, 0755)
	_, err := store.ImportFromEngram(engramDir, "")
	if err == nil {
		t.Fatalf("expected error for missing manifest")
	}
	if !strings.Contains(err.Error(), "manifest.json") {
		t.Fatalf("error should mention manifest.json, got %q", err.Error())
	}
}

func TestGunzipDataExported(t *testing.T) {
	orig := []byte("hello engram")
	gz := gzipDataForTest(orig)
	dec, err := GunzipData(gz)
	if err != nil {
		t.Fatalf("GunzipData error: %v", err)
	}
	if !bytes.Equal(dec, orig) {
		t.Fatalf("GunzipData mismatch")
	}
	_ = fmt.Sprintf("ok")
}
