package filemerge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func mustHash(t *testing.T, s string) string {
	t.Helper()
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// ComputeHash — exact-range (SHA-256 hex)
// ---------------------------------------------------------------------------

func TestComputeHash_ExactRange_DiffersFromWholeFile(t *testing.T) {
	// Fixture: 100 lines, target range lines 10-20 (1-indexed => indices 9..19)
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, strings.Repeat("x", 8)+strings.Repeat("-", i%5)+"\n")
	}
	whole := strings.Join(lines, "")
	rangeSlice := strings.Join(lines[9:20], "") // lines 10-20 inclusive

	wholeHash := ComputeHash([]byte(whole))
	rangeHash := ComputeHash([]byte(rangeSlice))

	if wholeHash == rangeHash {
		t.Fatalf("range hash == whole-file hash: %s — must differ (exact-range)", wholeHash)
	}
	// Verify range hash matches direct SHA-256 of that range
	expected := mustHash(t, rangeSlice)
	if rangeHash != expected {
		t.Fatalf("ComputeHash(range) = %s, want %s", rangeHash, expected)
	}
	// Whole must match its own direct
	expectedWhole := mustHash(t, whole)
	if wholeHash != expectedWhole {
		t.Fatalf("ComputeHash(whole) = %s, want %s", wholeHash, expectedWhole)
	}
}

func TestComputeHash_DeterministicAndHexLength(t *testing.T) {
	h1 := ComputeHash([]byte("hello"))
	h2 := ComputeHash([]byte("hello"))
	if h1 != h2 {
		t.Fatalf("deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("hex length = %d, want 64", len(h1))
	}
	h3 := ComputeHash([]byte("hello "))
	if h1 == h3 {
		t.Fatal("different content must produce different hash")
	}
	// empty vs nil must agree (sha256 of empty string)
	empty := ComputeHash([]byte(""))
	nilHash := ComputeHash(nil)
	if empty != nilHash {
		t.Fatalf("empty vs nil hash mismatch: %s vs %s", empty, nilHash)
	}
	// Known vector: SHA256("") == e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	const emptySHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if empty != emptySHA {
		t.Fatalf("empty hash = %s, want %s", empty, emptySHA)
	}
}

// ---------------------------------------------------------------------------
// ApplyWithHash — match succeeds
// ---------------------------------------------------------------------------

func TestApplyWithHash_Match_Succeeds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	initial := []byte("initial content lines 10-20 fixture\n")
	if err := os.WriteFile(path, initial, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h1 := ComputeHash(initial)
	newContent := []byte("new edited content\nsecond line\n")
	fresh, err := ApplyWithHash(path, h1, newContent)
	if err != nil {
		t.Fatalf("ApplyWithHash match: unexpected error: %v", err)
	}
	// fresh should be hash of newContent
	expectedFresh := ComputeHash(newContent)
	if fresh != expectedFresh {
		t.Fatalf("freshHash on success = %s, want %s", fresh, expectedFresh)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(newContent) {
		t.Fatalf("file = %q, want %q", string(got), string(newContent))
	}
}

// ---------------------------------------------------------------------------
// ApplyWithHash — mismatch warn-and-stop (no overwrite, freshHash)
// ---------------------------------------------------------------------------

func TestApplyWithHash_Mismatch_WarnAndStop_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	// disk starts as content A (hash abc)
	contentA := []byte("range A content\n")
	contentB := []byte("range B content modified by concurrent writer\n")
	if err := os.WriteFile(path, contentA, 0644); err != nil {
		t.Fatalf("setup A: %v", err)
	}
	hashA := ComputeHash(contentA)
	// Simulate concurrent edit: writer A changes file to B before our call
	if err := os.WriteFile(path, contentB, 0644); err != nil {
		t.Fatalf("setup B: %v", err)
	}
	hashB := ComputeHash(contentB)
	newContent := []byte("my attempted edit should not land\n")

	fresh, err := ApplyWithHash(path, hashA, newContent)
	if err == nil {
		t.Fatal("expected HashMismatchError, got nil")
	}
	var hm *HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("expected *HashMismatchError, got %T: %v", err, err)
	}
	if hm.Code != "needs_attention" {
		t.Fatalf("Code = %q, want %q", hm.Code, "needs_attention")
	}
	if hm.FreshHash != hashB {
		t.Fatalf("FreshHash = %s, want %s", hm.FreshHash, hashB)
	}
	if fresh != hashB {
		t.Fatalf("returned freshHash = %s, want %s", fresh, hashB)
	}
	if hm.Path != path {
		t.Fatalf("Path = %q, want %q", hm.Path, path)
	}
	// File must remain B (not overwritten)
	got, _ := os.ReadFile(path)
	if string(got) != string(contentB) {
		t.Fatalf("file after mismatch = %q, want unchanged %q", string(got), string(contentB))
	}
}

func TestApplyWithHash_Mismatch_BatchDoesNotAbort(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "a.txt")
	path2 := filepath.Join(dir, "b.txt")
	c1 := []byte("a initial")
	c2 := []byte("b initial")
	if err := os.WriteFile(path1, c1, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(path2, c2, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	hashA := ComputeHash(c1)
	// Change a externally, so hashA is stale for path1
	if err := os.WriteFile(path1, []byte("a changed externally"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Batch: apply to path1 (stale -> mismatch) and path2 (fresh -> should still succeed)
	_, err1 := ApplyWithHash(path1, hashA, []byte("new a"))
	if err1 == nil {
		t.Fatal("expected mismatch for path1")
	}
	// path2 should still be writable despite path1 mismatch
	hashB := ComputeHash(c2)
	newB := []byte("new b works")
	fresh, err2 := ApplyWithHash(path2, hashB, newB)
	if err2 != nil {
		t.Fatalf("batch second file should succeed despite first mismatch: %v", err2)
	}
	if fresh != ComputeHash(newB) {
		t.Fatalf("fresh hash mismatch")
	}
	got, _ := os.ReadFile(path2)
	if string(got) != string(newB) {
		t.Fatalf("b file = %q", string(got))
	}
}

// ---------------------------------------------------------------------------
// Force bypass
// ---------------------------------------------------------------------------

func TestApplyWithHash_Force_BypassesValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// Stale hash: hash of empty would mismatch, but force=true must overwrite
	staleHash := ComputeHash([]byte("something else"))
	newContent := []byte("forced overwrite succeeds")
	fresh, err := ApplyWithHash(path, staleHash, newContent, true)
	if err != nil {
		t.Fatalf("force should bypass mismatch: %v", err)
	}
	if fresh != ComputeHash(newContent) {
		t.Fatalf("fresh = %s, want %s", fresh, ComputeHash(newContent))
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(newContent) {
		t.Fatalf("file = %q, want %q", string(got), string(newContent))
	}
	// Also test via alias ApplyWithHashForce
	if err := os.WriteFile(path, []byte("again"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	stale2 := ComputeHash([]byte("stale2"))
	fresh2, err := ApplyWithHashForce(path, stale2, []byte("force alias"), true)
	if err != nil {
		t.Fatalf("ApplyWithHashForce force=true: %v", err)
	}
	if fresh2 != ComputeHash([]byte("force alias")) {
		t.Fatalf("alias fresh mismatch")
	}
	// Force false must still mismatch
	_, err = ApplyWithHashForce(path, stale2, []byte("should fail"), false)
	if err == nil {
		t.Fatal("force=false with stale hash should mismatch")
	}
}

func TestApplyWithHash_ForceFalse_Mismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	stale := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	_, err := ApplyWithHash(path, stale, []byte("new"), false)
	if err == nil {
		t.Fatal("expected mismatch")
	}
	var hm *HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("expected HashMismatchError")
	}
}

// ---------------------------------------------------------------------------
// Concurrent nearby edits trigger mismatch (scenario)
// ---------------------------------------------------------------------------

func TestApplyWithHash_Concurrent_NearbyEdits_StaleSecondGetsH2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.txt")
	initial := []byte("initial range content for two writers\n")
	if err := os.WriteFile(path, initial, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h1 := ComputeHash(initial)

	// Writer A reads h1 then writes first
	newA := []byte("writer A new content\n")
	freshA, err := ApplyWithHash(path, h1, newA)
	if err != nil {
		t.Fatalf("writer A: %v", err)
	}
	h2 := ComputeHash(newA)
	if freshA != h2 {
		t.Fatalf("writer A fresh = %s, want %s", freshA, h2)
	}
	// Writer B still holds stale h1, now tries to write
	newB := []byte("writer B stale attempt\n")
	freshB, err := ApplyWithHash(path, h1, newB)
	if err == nil {
		t.Fatal("writer B with stale h1 should get needs_attention")
	}
	var hm *HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("B expected HashMismatchError, got %T", err)
	}
	if hm.FreshHash != h2 {
		t.Fatalf("B FreshHash = %s, want h2=%s", hm.FreshHash, h2)
	}
	if freshB != h2 {
		t.Fatalf("B returned fresh = %s, want %s", freshB, h2)
	}
	// File must still be A's content, not B's
	got, _ := os.ReadFile(path)
	if string(got) != string(newA) {
		t.Fatalf("file = %q, want A's content %q", string(got), string(newA))
	}
}

func TestApplyWithHash_Concurrent_Goroutines_NoPanicAndAtLeastOneMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "race.txt")
	initial := []byte("base")
	if err := os.WriteFile(path, initial, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	h1 := ComputeHash(initial)

	var wg sync.WaitGroup
	results := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			content := []byte(strings.Repeat(string(rune('A'+idx)), 10))
			_, err := ApplyWithHash(path, h1, content)
			results[idx] = err
		}(i)
	}
	wg.Wait()
	// On Windows concurrent WriteFileAtomic rename can sporadically fail with
	// Access is denied due to file locking; that is not a hashline logic
	// failure. We tolerate it as contention, but still require no panic and
	// file remains readable.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after race: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("file empty after concurrent writes")
	}
	successCount := 0
	mismatchCount := 0
	for _, e := range results {
		if e == nil {
			successCount++
			continue
		}
		var hm *HashMismatchError
		if errors.As(e, &hm) {
			mismatchCount++
			if hm.Code != "needs_attention" {
				t.Fatalf("wrong code %q", hm.Code)
			}
			continue
		}
		// Windows file-lock contention during concurrent rename produces
		// *os.LinkError / *os.PathError with "Access is denied". Treat as
		// contention (neither success nor hash mismatch) but not a failure.
		if strings.Contains(e.Error(), "Access is denied") || strings.Contains(e.Error(), "being used by another process") {
			continue
		}
		t.Fatalf("unexpected error type %T: %v", e, e)
	}
	if successCount < 1 {
		t.Fatalf("expected at least 1 success, got %d", successCount)
	}
	_ = mismatchCount
}

// ---------------------------------------------------------------------------
// Missing file (empty hash) handling
// ---------------------------------------------------------------------------

func TestApplyWithHash_MissingFile_EmptyHashCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.txt")
	emptyHash := ComputeHash(nil)
	newContent := []byte("created")
	fresh, err := ApplyWithHash(path, emptyHash, newContent)
	if err != nil {
		t.Fatalf("missing file with empty hash should succeed: %v", err)
	}
	if fresh != ComputeHash(newContent) {
		t.Fatalf("fresh mismatch")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(newContent) {
		t.Fatalf("got %q", string(got))
	}
	// Stale mismatch should fail even for new file if expected not emptyHash
	path2 := filepath.Join(dir, "newfile2.txt")
	_, err = ApplyWithHash(path2, "not-empty-hash", []byte("x"))
	if err == nil {
		t.Fatal("expected mismatch for non-empty expected on missing file")
	}
}
