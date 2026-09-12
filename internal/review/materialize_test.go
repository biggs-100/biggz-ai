package review

// Materializer tests (tasks 2.1–2.5): marker composition, typed refusals for
// vacuous evidence and the whole-task byte cap, and the read-only +
// determinism invariants that prove `--materialize` captures nothing.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

// materializeStartLineage starts a review lineage on an existing repo and
// returns the store plus the provider-owned capture binding.
func materializeStartLineage(t *testing.T, repo, lineageID string) (*Store, CaptureBinding) {
	t.Helper()
	sha := runGitInDir(t, repo, "rev-parse", "HEAD")
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	lifecycle := New(model.ReviewSubject{Repository: repo, CommitSHA: sha})
	lifecycle.State.Role = model.RoleReviewer
	lifecycle.WithStore(store)
	if err := lifecycle.Start(t.Context()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	return store, CaptureBinding{
		Repo: repo, LineageID: lineageID, TargetIdentity: sha,
		Lens: "risk", Order: 0, ExpectedRevision: chain.HeadHash,
	}
}

// materializeSection is one `${marker} ${arg}` head plus the body bytes
// between it and the next marker line.
type materializeSection struct {
	Marker string
	Arg    string
	Body   []byte
}

// materializeSections splits a materialized task into its marker sections.
// A marker only matches at the start of a line, so patch context/content
// lines (prefixed by ' ', '+', '-') never split a section.
func materializeSections(t *testing.T, payload []byte) []materializeSection {
	t.Helper()
	markers := []string{
		MaterializeBindingMarker, MaterializeContextMarker,
		MaterializeNameStatusMarker, MaterializeNumStatMarker, MaterializePatchMarker,
	}
	sections := make([]materializeSection, 0)
	var current *materializeSection
	for _, chunk := range bytes.SplitAfter(payload, []byte("\n")) {
		line := bytes.TrimSuffix(chunk, []byte("\n"))
		if marker, arg, ok := matchMaterializeMarker(line, markers); ok {
			sections = append(sections, materializeSection{Marker: marker, Arg: arg})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil {
			t.Fatalf("materialize payload starts outside any marker section: %q", line)
		}
		current.Body = append(current.Body, chunk...)
	}
	return sections
}

// matchMaterializeMarker reports whether a line opens a marker section.
func matchMaterializeMarker(line []byte, markers []string) (marker, arg string, ok bool) {
	for _, candidate := range markers {
		rest, found := bytes.CutPrefix(line, []byte(candidate))
		if !found {
			continue
		}
		if len(rest) == 0 {
			return candidate, "", true
		}
		if rest[0] == ' ' {
			return candidate, string(rest[1:]), true
		}
	}
	return "", "", false
}

// materializeSnapshot fingerprints every file below dir by relative path and
// content hash, proving whether a materialize run mutated the lineage store.
func materializeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		payload, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		sum := sha256.Sum256(payload)
		snapshot[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return snapshot
}

// materializeSubjectHash is the provider-owned subject hash grammar.
var materializeSubjectHash = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ---------------------------------------------------------------------------
// 2.1 — marker composition: binding, context, name-status, numstat, patches
// ---------------------------------------------------------------------------

func TestMaterializeReviewerTaskSections(t *testing.T) {
	binding := captureFixtureBinding(t)
	result, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	payload, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("MaterializeReviewerTask: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("materialized reviewer task is empty")
	}
	if !bytes.HasSuffix(payload, []byte("\n")) {
		t.Fatalf("materialized reviewer task must end with a newline, got %q", payload[len(payload)-1:])
	}

	sections := materializeSections(t, payload)
	if len(sections) != 6 {
		t.Fatalf("sections = %d, want 6 (binding, context, name-status, numstat, 2 patches): %+v", len(sections), sections)
	}

	// Binding section: one-line provider-owned JSON mirroring the host
	// binding literal (lineage/target/lens/order/revision/subject_hash).
	if sections[0].Marker != MaterializeBindingMarker {
		t.Fatalf("section[0] marker = %q, want %q", sections[0].Marker, MaterializeBindingMarker)
	}
	var bindingJSON struct {
		Lineage     string `json:"lineage"`
		Target      string `json:"target"`
		Lens        string `json:"lens"`
		Order       int    `json:"order"`
		Revision    string `json:"revision"`
		SubjectHash string `json:"subject_hash"`
	}
	if err := json.Unmarshal([]byte(sections[0].Arg), &bindingJSON); err != nil {
		t.Fatalf("binding section is not one-line JSON: %v\n%s", err, sections[0].Arg)
	}
	if bindingJSON.Lineage != result.LineageID || bindingJSON.Target != result.TargetIdentity ||
		bindingJSON.Lens != result.Lens || bindingJSON.Order != result.SelectedOrder ||
		bindingJSON.Revision != result.ExpectedRevision {
		t.Errorf("binding JSON = %+v, want lineage %s target %s lens %s order %d revision %s",
			bindingJSON, result.LineageID, result.TargetIdentity, result.Lens, result.SelectedOrder, result.ExpectedRevision)
	}
	if !materializeSubjectHash.MatchString(bindingJSON.SubjectHash) {
		t.Errorf("binding subject_hash = %q, want a sha256: identity", bindingJSON.SubjectHash)
	}
	if bindingJSON.SubjectHash != result.Subject.SubjectHash {
		t.Errorf("binding subject_hash = %q, want the preflight subject %q", bindingJSON.SubjectHash, result.Subject.SubjectHash)
	}

	// Context section: the complete preflight JSON.
	if sections[1].Marker != MaterializeContextMarker {
		t.Fatalf("section[1] marker = %q, want %q", sections[1].Marker, MaterializeContextMarker)
	}
	var context PreflightResult
	if err := json.Unmarshal([]byte(sections[1].Arg), &context); err != nil {
		t.Fatalf("context section is not one-line JSON: %v\n%s", err, sections[1].Arg)
	}
	if context.Schema != PreflightSchema || context.LineageID != result.LineageID ||
		context.TargetIdentity != result.TargetIdentity || context.Subject.SubjectHash != result.Subject.SubjectHash {
		t.Errorf("context JSON = schema %q lineage %q target %q subject_hash %q", context.Schema, context.LineageID, context.TargetIdentity, context.Subject.SubjectHash)
	}
	if len(context.ChangedPathManifest) != 2 {
		t.Errorf("context manifest entries = %d, want 2", len(context.ChangedPathManifest))
	}

	// Name-status section.
	if sections[2].Marker != MaterializeNameStatusMarker {
		t.Fatalf("section[2] marker = %q, want %q", sections[2].Marker, MaterializeNameStatusMarker)
	}
	for _, want := range []string{"A\ta.txt\n", "A\tb.txt\n"} {
		if !bytes.Contains(sections[2].Body, []byte(want)) {
			t.Errorf("name-status section is missing %q:\n%s", want, sections[2].Body)
		}
	}

	// Numstat section.
	if sections[3].Marker != MaterializeNumStatMarker {
		t.Fatalf("section[3] marker = %q, want %q", sections[3].Marker, MaterializeNumStatMarker)
	}
	for _, want := range []string{"3\t0\ta.txt\n", "2\t0\tb.txt\n"} {
		if !bytes.Contains(sections[3].Body, []byte(want)) {
			t.Errorf("numstat section is missing %q:\n%s", want, sections[3].Body)
		}
	}

	// Per-path patch sections in manifest order, byte-equal to the frozen
	// inspector's per-path patches.
	inspector, err := OpenFrozenInspector(binding.Repo, binding.TargetIdentity, result.BaseTree)
	if err != nil {
		t.Fatalf("OpenFrozenInspector: %v", err)
	}
	if err != nil {
		t.Fatalf("OpenFrozenInspector: %v", err)
	}
	defer func() { _ = inspector.Close() }()
	wantPaths := inspector.Paths()
	if len(wantPaths) != 2 {
		t.Fatalf("inspector paths = %v, want 2", wantPaths)
	}
	for index, path := range wantPaths {
		section := sections[4+index]
		if section.Marker != MaterializePatchMarker || section.Arg != path {
			t.Fatalf("section[%d] = %q %q, want %q %q", 4+index, section.Marker, section.Arg, MaterializePatchMarker, path)
		}
		frozen, err := inspector.PatchForPath(path)
		if err != nil {
			t.Fatalf("PatchForPath(%s): %v", path, err)
		}
		if !bytes.Equal(section.Body, frozen) {
			t.Errorf("patch section for %s does not match the frozen per-path patch", path)
		}
	}
	for _, want := range []string{"+one", "+two", "+three"} {
		if !bytes.Contains(sections[4].Body, []byte(want)) {
			t.Errorf("a.txt patch is missing %q:\n%s", want, sections[4].Body)
		}
	}
	for _, want := range []string{"+x", "+y"} {
		if !bytes.Contains(sections[5].Body, []byte(want)) {
			t.Errorf("b.txt patch is missing %q:\n%s", want, sections[5].Body)
		}
	}
}

// captureFixtureBinding exposes the shared capture fixture's binding.
func captureFixtureBinding(t *testing.T) CaptureBinding {
	t.Helper()
	_, binding, _ := captureFixture(t)
	return binding
}

// ---------------------------------------------------------------------------
// 2.2 — typed refusals: vacuous evidence and the whole-task byte cap
// ---------------------------------------------------------------------------

func TestMaterializeReviewerTaskRefusesVacuousEvidence(t *testing.T) {
	t.Run("empty path set", func(t *testing.T) {
		repo := t.TempDir()
		gitInit(t, repo)
		if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
			t.Fatalf("write base: %v", err)
		}
		runGitInDir(t, repo, "add", ".")
		runGitInDir(t, repo, "commit", "-m", "base")
		// The candidate commit changes nothing: the frozen manifest is empty.
		runGitInDir(t, repo, "commit", "--allow-empty", "-m", "candidate")
		_, binding := materializeStartLineage(t, repo, "materialize-vacuous-empty")

		_, err := MaterializeReviewerTask(binding)
		var refusal *MaterializeRefusal
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want a typed *MaterializeRefusal", err)
		}
		t.Logf("harness refusal: %v", err)
		if refusal.Code != MaterializeVacuousCode {
			t.Errorf("refusal code = %q, want %q", refusal.Code, MaterializeVacuousCode)
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("refusal should name the empty evidence, got: %v", err)
		}
	})

	t.Run("empty patch for a content-changing path", func(t *testing.T) {
		result := &PreflightResult{Schema: PreflightSchema, LineageID: "lineage", TargetIdentity: strings.Repeat("a", 40), Lens: "risk"}
		facts := materializeFactsStub{
			paths:      []string{"a.txt"},
			nameStatus: []byte("A\ta.txt\n"),
			numStat:    []byte("1\t0\ta.txt\n"),
			patches:    map[string][]byte{"a.txt": {}},
		}
		_, err := composeReviewerTask(result, facts)
		var refusal *MaterializeRefusal
		if !errors.As(err, &refusal) {
			t.Fatalf("error = %v, want a typed *MaterializeRefusal", err)
		}
		if refusal.Code != MaterializeVacuousCode {
			t.Errorf("refusal code = %q, want %q", refusal.Code, MaterializeVacuousCode)
		}
		if !strings.Contains(err.Error(), "a.txt") {
			t.Errorf("refusal should name the empty-patch path, got: %v", err)
		}
	})
}

// materializeFactsStub feeds the composition seam with controlled facts. An
// empty patch for a manifest path cannot be produced by a real repository
// (a tree-diff entry always renders a non-empty patch), so the vacuity guard
// is exercised through the seam.
type materializeFactsStub struct {
	paths      []string
	nameStatus []byte
	numStat    []byte
	patches    map[string][]byte
}

func (s materializeFactsStub) Paths() []string             { return s.paths }
func (s materializeFactsStub) NameStatus() ([]byte, error) { return s.nameStatus, nil }
func (s materializeFactsStub) NumStat() ([]byte, error)    { return s.numStat, nil }
func (s materializeFactsStub) PatchForPath(path string) ([]byte, error) {
	patch, ok := s.patches[path]
	if !ok {
		return nil, fmt.Errorf("stub: unknown path %q", path)
	}
	return patch, nil
}

func TestMaterializeReviewerTaskRefusesOverCap(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	// Two text files of ~2.7 MiB each: every per-path patch stays under the
	// per-path cap while the composed task exceeds the 4 MiB whole-task cap.
	filler := strings.Repeat("0123456789abcdef\n", 170_000)
	for _, name := range []string{"big-a.txt", "big-b.txt"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(filler), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	_, binding := materializeStartLineage(t, repo, "materialize-over-cap")

	_, err := MaterializeReviewerTask(binding)
	var capRefusal *MaterializeCapRefusal
	if !errors.As(err, &capRefusal) {
		t.Fatalf("error = %v, want a typed *MaterializeCapRefusal", err)
	}
	t.Logf("harness refusal: %v", err)
	if capRefusal.Cap != ArtifactResultLimit {
		t.Errorf("cap = %d, want %d", capRefusal.Cap, ArtifactResultLimit)
	}
	if capRefusal.Actual <= capRefusal.Cap {
		t.Errorf("refusal actual = %d, want > cap %d", capRefusal.Actual, capRefusal.Cap)
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d-byte", ArtifactResultLimit)) {
		t.Errorf("refusal must name the cap, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 2.5 — read-only, deterministic, foreign-cwd
// ---------------------------------------------------------------------------

func TestMaterializeReviewerTaskReadOnlyAndDeterministic(t *testing.T) {
	store, binding, _ := captureFixture(t)
	before := materializeSnapshot(t, store.Dir)

	first, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	second, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("materialize is not byte-identical across two runs")
	}

	// A foreign cwd must not change a single byte (the binding pins the repo).
	t.Chdir(t.TempDir())
	third, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("foreign-cwd materialize: %v", err)
	}
	if !bytes.Equal(first, third) {
		t.Fatal("materialize is not byte-identical from a foreign cwd")
	}

	after := materializeSnapshot(t, store.Dir)
	if !maps.Equal(before, after) {
		t.Errorf("materialize mutated the lineage store:\nbefore=%v\nafter=%v", before, after)
	}
}

// TestMaterializeRuntimeHarness is the real integration path: a temp lineage
// over a candidate touching a binary blob, an executable Markdown file, a
// deleted path and a modified doc; it logs the raw materialized evidence and
// proves non-empty bytes, byte-identity across runs and a foreign cwd, and
// that dirty/staged edits never leak.
func TestMaterializeRuntimeHarness(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	writeHarnessFile(t, repo, "blob.bin", "HARNESS-BINARY-SECRET-PAYLOAD\x00\x01")
	writeHarnessFile(t, repo, "docs/guide.md", "one\n")
	writeHarnessFile(t, repo, "old.txt", "removed\n")
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	writeHarnessFile(t, repo, "blob.bin", "HARNESS-BINARY-SECRET-PAYLOAD\x00\x02different")
	writeHarnessFile(t, repo, "docs/guide.md", "one\ntwo\n")
	if err := os.Remove(filepath.Join(repo, "old.txt")); err != nil {
		t.Fatalf("remove old.txt: %v", err)
	}
	writeHarnessFile(t, repo, "scripts/run.md", "#!/bin/sh\necho one\n")
	if err := os.Chmod(filepath.Join(repo, "scripts", "run.md"), 0o755); err != nil {
		t.Fatalf("chmod run.md: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	// Windows cannot carry the executable bit through the filesystem, so the
	// index mode is set explicitly: the frozen tree must record 100755.
	runGitInDir(t, repo, "update-index", "--chmod=+x", "scripts/run.md")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	store, binding := materializeStartLineage(t, repo, "materialize-harness")

	// Dirty + staged edits on top of the candidate must never leak.
	writeHarnessFile(t, repo, "docs/HARNESS-staged.md", "HARNESS-STAGED\n")
	runGitInDir(t, repo, "add", "docs/HARNESS-staged.md")
	writeHarnessFile(t, repo, "docs/guide.md", "one\ntwo\nHARNESS-UNSTAGED\n")

	first, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("materialized reviewer task is empty")
	}
	sum := sha256.Sum256(first)
	t.Logf("harness: lineage=%s target=%s bytes=%d sha256=%s",
		binding.LineageID, binding.TargetIdentity, len(first), hex.EncodeToString(sum[:]))
	for _, section := range materializeSections(t, first) {
		t.Logf("harness: section marker=%q arg=%q body_bytes=%d", section.Marker, section.Arg, len(section.Body))
	}
	if bytes.Contains(first, []byte("HARNESS-BINARY-SECRET-PAYLOAD")) {
		t.Error("binary blob bytes leaked into the materialized task")
	}
	if bytes.Contains(first, []byte("HARNESS-")) {
		t.Error("dirty/staged edits leaked into the materialized task")
	}
	for _, want := range []string{
		"Binary files a/blob.bin and b/blob.bin differ",
		"+two",
		"deleted file mode 100644",
		"new file mode 100755",
	} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("materialized task is missing frozen evidence %q", want)
		}
	}

	second, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("materialize is not byte-identical across two runs in the harness")
	}
	secondSum := sha256.Sum256(second)
	t.Logf("harness: second run sha256=%s (identical=%t)", hex.EncodeToString(secondSum[:]), bytes.Equal(first, second))

	t.Chdir(t.TempDir())
	third, err := MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("foreign-cwd materialize: %v", err)
	}
	thirdSum := sha256.Sum256(third)
	t.Logf("harness: foreign cwd sha256=%s (identical=%t)", hex.EncodeToString(thirdSum[:]), bytes.Equal(first, third))
	if !bytes.Equal(first, third) {
		t.Fatal("materialize is not byte-identical from a foreign cwd in the harness")
	}

	before := materializeSnapshot(t, store.Dir)
	if _, err := MaterializeReviewerTask(binding); err != nil {
		t.Fatalf("post-snapshot materialize: %v", err)
	}
	after := materializeSnapshot(t, store.Dir)
	t.Logf("harness: lineage store files before=%d after=%d (unchanged=%t)", len(before), len(after), maps.Equal(before, after))
	if !maps.Equal(before, after) {
		t.Error("harness materialize mutated the lineage store")
	}
}

// writeHarnessFile writes one fixture file below repo, creating parents.
func writeHarnessFile(t *testing.T, repo, name, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
