package hashline

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "")), 0644); err != nil {
		t.Fatalf("writeLines: %v", err)
	}
}
func hashOfRange(t *testing.T, content []byte, s, e int) string {
	t.Helper()
	lines := splitLines(content)
	seg := bytes.Join(lines[s-1:e], nil)
	return Hash4(filemerge.ComputeHash(seg))
}

func TestNoopLoopGuard_EqualAborts(t *testing.T) {
	if !NoopLoopGuard([]byte("same\n"), []byte("same\n")) {
		t.Fatal("equal should abort")
	}
	if NoopLoopGuard([]byte("a"), []byte("b")) {
		t.Fatal("differ should not abort")
	}
	if !NoopLoopGuard(nil, nil) {
		t.Fatal("nil nil no-op")
	}
	if NoopLoopGuard([]byte("a\n"), []byte("a")) {
		t.Fatal("newline diff not equal")
	}
}
func TestNoopLoopGuard_DifferProceeds(t *testing.T) {
	if NoopLoopGuard(nil, []byte("x")) {
		t.Fatal("nil vs x not abort")
	}
}

func TestApply_MatchWritesPUT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	lines := []string{"line1\n", "line2\n", "line3\n", "line4\n", "line5\n"}
	writeLines(t, path, lines)
	content, _ := os.ReadFile(path)
	h := hashOfRange(t, content, 2, 3)
	d, _ := Parse("PUT 2.=3: #" + h)
	seen := [][2]int{{1, 5}}
	var snap Store
	snap.Capture(path, content)
	if _, err := Apply(path, d, seen, &snap, []byte("NEW2\nNEW3\n")); err != nil {
		t.Fatalf("PUT match: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "line1\nNEW2\nNEW3\nline4\nline5\n" {
		t.Fatalf("PUT got %q", string(got))
	}
}
func TestApply_MismatchWarnAndStop_NoOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"a\n", "b\n", "c\n"})
	content, _ := os.ReadFile(path)
	correct := hashOfRange(t, content, 1, 2)
	stale := "FFFF"
	if stale == correct {
		stale = "EEEE"
	}
	d, _ := Parse("PUT 1.=2: #" + stale)
	seen := [][2]int{{1, 3}}
	var snap Store
	fresh, err := Apply(path, d, seen, &snap, []byte("no\n"))
	if err == nil {
		t.Fatal("expected mismatch")
	}
	var hm *HashMismatchError
	if !errors.As(err, &hm) || hm.Code != "needs_attention" || hm.FreshHash != correct || fresh != correct {
		t.Fatalf("mismatch details bad: %v fresh %s want %s", err, fresh, correct)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Fatalf("file changed on mismatch")
	}
}
func TestApply_CUTMatchingHashRemovesRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"1\n", "2\n", "3\n", "4\n", "5\n"})
	content, _ := os.ReadFile(path)
	h := hashOfRange(t, content, 2, 3)
	d, _ := Parse("CUT 2.=3: #" + h)
	if _, err := Apply(path, d, [][2]int{{1, 5}}, &Store{}, nil); err != nil {
		t.Fatalf("CUT: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "1\n4\n5\n" {
		t.Fatalf("CUT got %q", string(got))
	}
}
func TestApply_CUTMismatchPreservesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"a\n", "b\n", "c\n", "d\n"})
	content, _ := os.ReadFile(path)
	correct := hashOfRange(t, content, 2, 3)
	stale := "AAAA"
	if stale == correct {
		stale = "BBBB"
	}
	d, _ := Parse("CUT 2.=3: #" + stale)
	_, err := Apply(path, d, [][2]int{{1, 4}}, &Store{}, nil)
	if err == nil {
		t.Fatal("expected mismatch")
	}
	var hm *HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("not mismatch")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Fatalf("CUT mismatch changed")
	}
}
func TestApply_CUTSingleLineLT(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"x\n", "y\n", "z\n"})
	content, _ := os.ReadFile(path)
	h := Hash4(filemerge.ComputeHash(bytes.Join(splitLines(content)[1:2], nil)))
	d, _ := Parse("CUT <2 #" + h)
	if _, err := Apply(path, d, [][2]int{{1, 3}}, &Store{}, nil); err != nil {
		t.Fatalf("CUT <2: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "x\nz\n" {
		t.Fatalf("CUT <2 got %q", string(got))
	}
}
func TestApply_PUTSingleLineLTMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"a\n", "b\n", "c\n"})
	content, _ := os.ReadFile(path)
	h := Hash4(filemerge.ComputeHash(bytes.Join(splitLines(content)[1:2], nil)))
	d, _ := Parse("PUT <2 #" + h)
	if _, err := Apply(path, d, [][2]int{{1, 3}}, &Store{}, []byte("B new\n")); err != nil {
		t.Fatalf("PUT <2: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "a\nB new\nc\n" {
		t.Fatalf("PUT <2 got %q", string(got))
	}
}
func TestApply_UnseenRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"a\n", "b\n"})
	_, err := Apply(path, Directive{Op: OpPUT, Start: 50, End: 60, HashTag: "A1B2"}, [][2]int{{1, 20}}, &Store{}, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "unseen") {
		t.Fatalf("unseen expected, got %v", err)
	}
}
func TestApply_BatchSafe(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.txt")
	pathB := filepath.Join(dir, "b.txt")
	contentA := []byte("a1\n a2\n a3\n")
	contentB := []byte("b1\n b2\n")
	os.WriteFile(pathA, contentA, 0644)
	os.WriteFile(pathB, contentB, 0644)
	segB := bytes.Join(splitLines(contentB)[0:1], nil)
	hB := Hash4(filemerge.ComputeHash(segB))
	var snap Store
	snap.Capture(pathA, contentA)
	snap.Capture(pathB, contentB)
	dA, _ := Parse("PUT 1.=1: #FFFF")
	if _, err := Apply(pathA, dA, [][2]int{{1, 3}}, &snap, []byte("newA\n")); err == nil {
		t.Fatal("A should mismatch")
	}
	dB, _ := Parse("PUT 1.=1: #" + hB)
	if _, err := Apply(pathB, dB, [][2]int{{1, 2}}, &snap, []byte("newB\n")); err != nil {
		t.Fatalf("B should succeed: %v", err)
	}
	gotB, _ := os.ReadFile(pathB)
	if string(gotB) != "newB\n b2\n" {
		t.Fatalf("B got %q", string(gotB))
	}
}
func TestApply_NoopAbortsNoWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"keep\n", "other\n"})
	content, _ := os.ReadFile(path)
	h := hashOfRange(t, content, 1, 1)
	d, _ := Parse("PUT 1.=1: #" + h)
	if _, err := Apply(path, d, [][2]int{{1, 2}}, &Store{}, []byte("keep\n")); err != nil {
		t.Fatalf("noop: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "keep\nother\n" {
		t.Fatalf("noop changed")
	}
}
func TestApply_Hash4Helper(t *testing.T) {
	if Hash4("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") != "E3B0" {
		t.Fatal("Hash4")
	}
}
func TestApply_Concurrent_NearbyStaleSecond(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent.txt")
	initial := []byte("line1\nline2\nline3\n")
	os.WriteFile(path, initial, 0644)
	h1 := hashOfRange(t, initial, 1, 2)
	var snap Store
	snap.Capture(path, initial)
	seen := [][2]int{{1, 3}}
	d, _ := Parse("PUT 1.=2: #" + h1)
	if _, err := Apply(path, d, seen, &snap, []byte("writer A\n")); err != nil {
		t.Fatalf("A: %v", err)
	}
	freshB, err := Apply(path, d, seen, &snap, []byte("writer B stale\n"))
	if err == nil {
		t.Fatal("B stale should mismatch")
	}
	var hm *HashMismatchError
	if !errors.As(err, &hm) {
		t.Fatalf("B not mismatch")
	}
	contentAfterA, _ := os.ReadFile(path)
	h2 := hashOfRange(t, contentAfterA, 1, 2)
	if hm.FreshHash != h2 || freshB != h2 {
		t.Fatalf("fresh %s %s want %s", hm.FreshHash, freshB, h2)
	}
}
func TestApply_WriteAtomicFailurePreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	writeLines(t, path, []string{"a\n", "b\n"})
	content, _ := os.ReadFile(path)
	h := hashOfRange(t, content, 1, 1)
	d, _ := Parse("PUT 1.=1: #" + h)
	os.Remove(path)
	os.Mkdir(path, 0755)
	if _, err := Apply(path, d, [][2]int{{1, 2}}, &Store{}, []byte("new\n")); err == nil {
		t.Fatal("expected error dir")
	}
	if fi, _ := os.Stat(path); !fi.IsDir() {
		t.Fatal("dir preserved")
	}
}
