package review

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

func TestComputeFileHash_MatchesFilemerge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := []byte("sample content for hash")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h, err := ComputeFileHash(path)
	if err != nil {
		t.Fatalf("ComputeFileHash: %v", err)
	}
	expected := filemerge.ComputeHash(content)
	if h != expected {
		t.Fatalf("ComputeFileHash = %s, want %s", h, expected)
	}
	// missing file => empty hash
	missing := filepath.Join(dir, "missing.txt")
	emptyHash := filemerge.ComputeHash(nil)
	h2, err := ComputeFileHash(missing)
	if err != nil {
		t.Fatalf("missing ComputeFileHash: %v", err)
	}
	if h2 != emptyHash {
		t.Fatalf("missing hash = %s, want empty %s", h2, emptyHash)
	}
}

func TestReadFileWithHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	data := []byte("read with hash fixture")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	gotData, gotHash, err := ReadFileWithHash(path)
	if err != nil {
		t.Fatalf("ReadFileWithHash: %v", err)
	}
	if string(gotData) != string(data) {
		t.Fatalf("data = %q, want %q", string(gotData), string(data))
	}
	if gotHash != filemerge.ComputeHash(data) {
		t.Fatalf("hash = %s, want %s", gotHash, filemerge.ComputeHash(data))
	}
}

func TestPrepareCorrection_StoresBeforeHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "prep.txt")
	content := []byte("initial")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	corr, data, err := PrepareCorrection(path, "test reason")
	if err != nil {
		t.Fatalf("PrepareCorrection: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("data mismatch")
	}
	expectedHash := filemerge.ComputeHash(content)
	if corr.BeforeHash != expectedHash {
		t.Fatalf("BeforeHash = %s, want %s", corr.BeforeHash, expectedHash)
	}
	if corr.Reason != "test reason" {
		t.Fatalf("reason mismatch")
	}
}

func TestApplyCorrection_StaleSecondWriterGetsFreshHashH2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.txt")
	initial := []byte("initial content")
	if err := os.WriteFile(path, initial, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Both writers read same h1 via PrepareCorrection
	corrA, _, err := PrepareCorrection(path, "writer A")
	if err != nil {
		t.Fatalf("prepare A: %v", err)
	}
	corrB := corrA // B holds stale copy
	h1 := corrA.BeforeHash

	// Writer A succeeds with h1
	newA := []byte("writer A new content")
	freshA, err := ApplyCorrection(corrA, path, newA, false)
	if err != nil {
		t.Fatalf("writer A ApplyCorrection: %v", err)
	}
	h2 := filemerge.ComputeHash(newA)
	if freshA != h2 {
		t.Fatalf("freshA = %s, want h2 %s", freshA, h2)
	}
	// Verify h1 != h2
	if h1 == h2 {
		t.Fatal("h1 should differ from h2")
	}
	// Writer B with stale h1 must get needs_attention + freshHash h2, no overwrite
	newB := []byte("writer B stale")
	freshB, err := ApplyCorrection(corrB, path, newB, false)
	if err == nil {
		t.Fatal("expected HashMismatchError for stale B")
	}
	var hm *filemerge.HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("expected HashMismatchError, got %T: %v", err, err)
	}
	if hm.Code != "needs_attention" {
		t.Fatalf("code = %q", hm.Code)
	}
	if hm.FreshHash != h2 {
		t.Fatalf("FreshHash = %s, want h2 %s", hm.FreshHash, h2)
	}
	if freshB != h2 {
		t.Fatalf("returned fresh = %s, want %s", freshB, h2)
	}
	// File must still be A's content
	got, _ := os.ReadFile(path)
	if string(got) != string(newA) {
		t.Fatalf("file after B mismatch = %q, want %q", string(got), string(newA))
	}
	// Force bypass: B with force=true should overwrite even with stale hash
	freshForce, err := ApplyCorrection(corrB, path, newB, true)
	if err != nil {
		t.Fatalf("force should succeed: %v", err)
	}
	if freshForce != filemerge.ComputeHash(newB) {
		t.Fatalf("force fresh mismatch")
	}
	got, _ = os.ReadFile(path)
	if string(got) != string(newB) {
		t.Fatalf("after force, file = %q", string(got))
	}
}

func TestWriteFileWithHash_ForceAndMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "write.txt")
	content := []byte("orig")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h := filemerge.ComputeHash(content)
	// mismatch without force
	_, err := WriteFileWithHash(path, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", []byte("new"), false)
	if err == nil {
		t.Fatal("expected mismatch")
	}
	var hm *filemerge.HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("expected HashMismatchError")
	}
	if hm.FreshHash != h {
		t.Fatalf("fresh = %s, want %s", hm.FreshHash, h)
	}
	// force should overwrite
	_, err = WriteFileWithHash(path, "stale", []byte("forced"), true)
	if err != nil {
		t.Fatalf("force write: %v", err)
	}
}
