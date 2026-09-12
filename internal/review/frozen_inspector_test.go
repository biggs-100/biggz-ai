package review

// RED tests for the frozen-tree inspector (Phase 1 tasks 1.1-1.3):
//  1.1 documentation-like paths and binary paths always materialize, and
//      --text is never used (a binary blob must not spill its bytes);
//  1.2 the repository is resolved once: a foreign cwd yields byte-identical
//      output and an unresolvable selection is a typed refusal;
//  1.3 dirty and staged edits never leak: output is byte-identical with a
//      dirty worktree, a staged edit, and a staged .gitattributes file that
//      would otherwise reclassify documentation as binary.
//
// Fixtures live in temp repositories (t.TempDir + gitInit); nothing here
// touches the real checkout.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	frozenDirtyMarker   = "DIRTY-UNSTAGED-MARKER"
	frozenStagedMarker  = "STAGED-ONLY-MARKER"
	frozenBinaryMarker  = "DIRTY-BINARY-MARKER"
	frozenCandidateLine = "+second line"
)

// frozenInspectorFixture builds a temp repository whose candidate commit
// changes exactly three paths: a text documentation file, an executable
// Markdown file with a shebang (added with mode 100755), and a binary blob.
// It returns the repository root and the candidate commit SHA.
func frozenInspectorFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	writeFrozenFixture(t, repo, "docs/guide.md", "first line\n")
	writeFrozenFixture(t, repo, "blob.bin", "\x00\x01base\x02\x00")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	writeFrozenFixture(t, repo, "docs/guide.md", "first line\nsecond line\n")
	writeFrozenFixture(t, repo, "scripts/run.md", "#!/bin/sh\necho one\n")
	writeFrozenFixture(t, repo, "blob.bin", "\x00\x03candidate\x04\x00")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "update-index", "--chmod=+x", "scripts/run.md")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	return repo, runGitInDir(t, repo, "rev-parse", "HEAD")
}

func writeFrozenFixture(t *testing.T, repo, rel, content string) {
	t.Helper()
	full := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func openFrozenInspector(t *testing.T, repo, sha string) *FrozenInspector {
	t.Helper()
	inspector, err := OpenFrozenInspector(repo, sha, "")
	if err != nil {
		t.Fatalf("OpenFrozenInspector(%q): %v", repo, err)
	}
	t.Cleanup(func() {
		if err := inspector.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return inspector
}

func frozenTreeHunks(t *testing.T, repo, sha string) map[string][]byte {
	t.Helper()
	hunks, err := openFrozenInspector(t, repo, sha).Hunks()
	if err != nil {
		t.Fatalf("Hunks: %v", err)
	}
	return hunks
}

func assertFrozenBytesEqual(t *testing.T, want, got map[string][]byte) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("hunk path count = %d, want %d (want=%v got=%v)", len(got), len(want), keysOfBytes(want), keysOfBytes(got))
	}
	for path, wantBytes := range want {
		gotBytes, ok := got[path]
		if !ok {
			t.Errorf("hunk for %q is missing", path)
			continue
		}
		if !bytes.Equal(wantBytes, gotBytes) {
			t.Errorf("hunk for %q is not byte-identical:\nwant %q\ngot  %q", path, wantBytes, gotBytes)
		}
	}
}

func keysOfBytes(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// TestFrozenInspectorMaterializesDocumentationAndBinary is task 1.1: an
// executable .md and a binary path always materialize, and binary blobs stay
// binary (--text must never be used).
func TestFrozenInspectorMaterializesDocumentationAndBinary(t *testing.T) {
	repo, sha := frozenInspectorFixture(t)
	inspector := openFrozenInspector(t, repo, sha)

	if inspector.BaseTree() == "" || inspector.CandidateTree() == "" {
		t.Fatalf("frozen trees must both resolve: base=%q candidate=%q", inspector.BaseTree(), inspector.CandidateTree())
	}
	if inspector.BaseTree() == inspector.CandidateTree() {
		t.Fatalf("base and candidate trees must differ, both %q", inspector.BaseTree())
	}
	paths := inspector.Paths()
	for _, want := range []string{"blob.bin", "docs/guide.md", "scripts/run.md"} {
		if !slices.Contains(paths, want) {
			t.Errorf("frozen manifest %v is missing %q", paths, want)
		}
	}
	if len(paths) != 3 {
		t.Errorf("manifest paths = %v, want exactly the three fixture paths", paths)
	}

	mdPatch, err := inspector.PatchForPath("scripts/run.md")
	if err != nil {
		t.Fatalf("PatchForPath(scripts/run.md): %v", err)
	}
	if len(mdPatch) == 0 {
		t.Fatal("executable .md must materialize patch bytes, got none")
	}
	for _, want := range [][]byte{
		[]byte("new file mode 100755"),
		[]byte("+#!/bin/sh"),
		[]byte("+echo one"),
	} {
		if !bytes.Contains(mdPatch, want) {
			t.Errorf("executable .md patch is missing %q:\n%s", want, mdPatch)
		}
	}
	if bytes.Contains(mdPatch, []byte("Binary files")) {
		t.Errorf("executable .md must not be classified binary:\n%s", mdPatch)
	}

	binPatch, err := inspector.PatchForPath("blob.bin")
	if err != nil {
		t.Fatalf("PatchForPath(blob.bin): %v", err)
	}
	if len(binPatch) == 0 {
		t.Fatal("binary path must still materialize its patch entry, got none")
	}
	if !bytes.Contains(binPatch, []byte("Binary files")) {
		t.Errorf("binary patch must carry Git's binary classification:\n%s", binPatch)
	}
	if bytes.IndexByte(binPatch, 0) >= 0 {
		t.Errorf("binary patch leaked raw blob bytes (--text must never be used):\n%q", binPatch)
	}
	if bytes.Contains(binPatch, []byte("candidate")) {
		t.Errorf("binary patch leaked candidate blob content:\n%s", binPatch)
	}
}

// TestFrozenInspectorRepoSelectionIsFrozenAgainstCwd is task 1.2: every read
// is rooted in the repository resolved once (-C + isolated GIT_DIR), so a
// foreign cwd cannot move a byte.
func TestFrozenInspectorRepoSelectionIsFrozenAgainstCwd(t *testing.T) {
	repo, sha := frozenInspectorFixture(t)
	pristine := frozenTreeHunks(t, repo, sha)

	t.Chdir(t.TempDir())
	afterForeignCwd := frozenTreeHunks(t, repo, sha)
	assertFrozenBytesEqual(t, pristine, afterForeignCwd)

	subdir := filepath.Join(repo, "docs")
	fromSubdir := frozenTreeHunks(t, subdir, sha)
	assertFrozenBytesEqual(t, pristine, fromSubdir)
}

// TestFrozenInspectorUnresolvableInputsRefusedTyped is task 1.2: an
// unresolvable repository selection or candidate commit is a typed refusal,
// never a guess.
func TestFrozenInspectorUnresolvableInputsRefusedTyped(t *testing.T) {
	cases := []struct {
		name  string
		repo  string
		stage string
	}{
		{name: "missing directory", repo: filepath.Join(t.TempDir(), "missing"), stage: "repository"},
		{name: "not a git work tree", repo: t.TempDir(), stage: "repository"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := OpenFrozenInspector(tc.repo, "", "")
			var refusal *FrozenInspectionRefusal
			if !errors.As(err, &refusal) {
				t.Fatalf("OpenFrozenInspector(%q) error = %v, want *FrozenInspectionRefusal", tc.repo, err)
			}
			if refusal.Stage != tc.stage {
				t.Errorf("refusal stage = %q, want %q", refusal.Stage, tc.stage)
			}
		})
	}

	repo, _ := frozenInspectorFixture(t)
	t.Run("unresolvable candidate commit", func(t *testing.T) {
		_, err := OpenFrozenInspector(repo, "0123456789012345678901234567890123456789", "")
		var refusal *FrozenInspectionRefusal
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want *FrozenInspectionRefusal", err)
		}
		if refusal.Stage != "candidate tree" {
			t.Errorf("refusal stage = %q, want candidate tree", refusal.Stage)
		}
	})
}

// TestFrozenInspectorDirtyAndStagedEditsNeverLeak is task 1.3: output is
// derived from frozen trees, so a dirty worktree, a staged edit, and a
// staged .gitattributes reclassification produce byte-identical output.
func TestFrozenInspectorDirtyAndStagedEditsNeverLeak(t *testing.T) {
	repo, sha := frozenInspectorFixture(t)
	pristine := frozenTreeHunks(t, repo, sha)

	writeFrozenFixture(t, repo, "docs/guide.md", "first line\n"+frozenDirtyMarker+"\n")
	writeFrozenFixture(t, repo, "docs/staged.md", frozenStagedMarker+"\n")
	writeFrozenFixture(t, repo, ".gitattributes", "*.md binary\n")
	runGitInDir(t, repo, "add", "docs/staged.md", ".gitattributes")
	writeFrozenFixture(t, repo, "blob.bin", "\x00"+frozenBinaryMarker+"\x00")

	after := frozenTreeHunks(t, repo, sha)
	assertFrozenBytesEqual(t, pristine, after)
	for path, payload := range after {
		for _, marker := range []string{frozenDirtyMarker, frozenStagedMarker, frozenBinaryMarker} {
			if bytes.Contains(payload, []byte(marker)) {
				t.Errorf("hunk for %q leaked the dirty/staged marker %q", path, marker)
			}
		}
	}
	guide := after["docs/guide.md"]
	if bytes.Contains(guide, []byte("Binary files")) {
		t.Errorf("staged .gitattributes reclassified documentation as binary:\n%s", guide)
	}
	if !bytes.Contains(guide, []byte(frozenCandidateLine)) {
		t.Errorf("frozen candidate content is missing from the guide patch:\n%s", guide)
	}
}

// TestFrozenInspectorByteCapsRefuseTyped proves the caps are typed refusals,
// never silent truncation: the offending size is reported and no bytes ship.
func TestFrozenInspectorByteCapsRefuseTyped(t *testing.T) {
	repo, sha := frozenInspectorFixture(t)
	inspector := openFrozenInspector(t, repo, sha)

	_, err := inspector.hunksBounded(MaxFrozenTaskPatchBytes, 4)
	var pathRefusal *FrozenByteCapRefusal
	if !errors.As(err, &pathRefusal) {
		t.Fatalf("per-path overflow error = %v, want *FrozenByteCapRefusal", err)
	}
	if pathRefusal.Scope != "path" || pathRefusal.Cap != 4 || pathRefusal.Actual <= 4 {
		t.Errorf("per-path refusal = %+v, want scope path cap 4 actual > 4", pathRefusal)
	}
	if pathRefusal.Path == "" {
		t.Error("per-path refusal must name the offending path")
	}

	_, err = inspector.hunksBounded(8, MaxFrozenPathPatchBytes)
	var taskRefusal *FrozenByteCapRefusal
	if !errors.As(err, &taskRefusal) {
		t.Fatalf("task overflow error = %v, want *FrozenByteCapRefusal", err)
	}
	if taskRefusal.Scope != "task" || taskRefusal.Cap != 8 || taskRefusal.Actual <= 8 {
		t.Errorf("task refusal = %+v, want scope task cap 8 actual > 8", taskRefusal)
	}
}

// TestFrozenInspectorRuntimeHarness is the work-unit runtime harness: a real
// temp repository with a binary file, an executable .md, and a text file,
// plus staged and unstaged edits on top of the committed candidate. It
// derives the frozen hunks three times (from the repo cwd, from a foreign
// cwd, and under a hostile inherited GIT_* environment) and prints the exact
// per-path git invocation, byte size, SHA-256, and payload of each run, then
// asserts byte-identity across runs and that no staged or unstaged edit
// leaks into the bytes. It also proves the derivation is independent of the
// caller's cwd and of inherited GIT_DIR/GIT_WORK_TREE/GIT_INDEX_FILE exports.
func TestFrozenInspectorRuntimeHarness(t *testing.T) {
	repo, sha := frozenInspectorFixture(t)

	writeFrozenFixture(t, repo, "docs/guide.md", "first line\nHARNESS-UNSTAGED-MARKER\n")
	writeFrozenFixture(t, repo, "docs/harness-staged.md", "HARNESS-STAGED-MARKER\n")
	runGitInDir(t, repo, "add", "docs/harness-staged.md")
	writeFrozenFixture(t, repo, "blob.bin", "\x00HARNESS-BINARY-MARKER\x00")

	first := harnessFrozenRun(t, repo, sha, "run 1 (repo cwd)")
	t.Chdir(t.TempDir())
	second := harnessFrozenRun(t, repo, sha, "run 2 (foreign cwd)")
	assertFrozenBytesEqual(t, first, second)

	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "foreign-git"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "foreign-index"))
	third := harnessFrozenRun(t, repo, sha, "run 3 (hostile GIT_* environment)")
	assertFrozenBytesEqual(t, first, third)

	for path, payload := range second {
		for _, marker := range []string{"HARNESS-UNSTAGED-MARKER", "HARNESS-STAGED-MARKER", "HARNESS-BINARY-MARKER"} {
			if bytes.Contains(payload, []byte(marker)) {
				t.Errorf("harness leak: %q appears in the frozen hunk for %q", marker, path)
			}
		}
	}
}

// harnessFrozenRun derives the frozen hunks for one harness run and logs the
// raw evidence: the exact per-path git invocation, byte size, SHA-256, and
// the payload itself.
func harnessFrozenRun(t *testing.T, repo, sha, label string) map[string][]byte {
	t.Helper()
	inspector := openFrozenInspector(t, repo, sha)
	hunks, err := inspector.Hunks()
	if err != nil {
		t.Fatalf("[%s] Hunks: %v", label, err)
	}
	t.Logf("[%s] repo=%s base=%s candidate=%s", label, repo, inspector.BaseTree(), inspector.CandidateTree())
	for _, path := range inspector.Paths() {
		sum := sha256.Sum256(hunks[path])
		t.Logf("[%s] git %s", label, strings.Join(inspector.patchArgs(path), " "))
		t.Logf("[%s] %s: %d bytes sha256:%x\n%s", label, path, len(hunks[path]), sum, hunks[path])
	}
	return hunks
}

// TestFrozenInspectorUnknownPathAndClosedRefused covers the remaining typed
// refusals: a path outside the frozen manifest, and reads after Close.
func TestFrozenInspectorUnknownPathAndClosedRefused(t *testing.T) {
	repo, sha := frozenInspectorFixture(t)
	inspector := openFrozenInspector(t, repo, sha)

	_, err := inspector.PatchForPath("docs/missing.md")
	var refusal *FrozenInspectionRefusal
	if !errors.As(err, &refusal) || refusal.Stage != "patch" {
		t.Fatalf("unknown path error = %v, want *FrozenInspectionRefusal stage patch", err)
	}

	if err := inspector.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := inspector.Close(); err != nil {
		t.Fatalf("second Close must be a no-op, got %v", err)
	}
	if _, err := inspector.Hunks(); err == nil {
		t.Fatal("Hunks on a closed inspector must refuse")
	}
	if _, err := inspector.PatchForPath("docs/guide.md"); err == nil {
		t.Fatal("PatchForPath on a closed inspector must refuse")
	}
}
