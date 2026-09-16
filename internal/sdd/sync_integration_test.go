package sdd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// --- fixtures ---

// syncAddedDeltaContent is delta shaped and must keep behaving as before.
const syncAddedDeltaContent = "# Delta for Added Domain\n\n" +
	"## ADDED Requirements\n\n" +
	"### Requirement: Added Domain Requirement\n\n" +
	"The system SHALL do new.\n\n" +
	"#### Scenario: New works\n\n" +
	"- **WHEN** new runs\n" +
	"- **THEN** new succeeds\n"

const syncExistingLivingSpec = "# Existing Domain Specification\n\n" +
	"## Purpose\n\n" +
	"State the purpose of the existing domain.\n\n" +
	"## Requirements\n\n" +
	"### Requirement: Existing Requirement\n\n" +
	"The system SHALL be old.\n\n" +
	"#### Scenario: Old works\n\n" +
	"- **WHEN** old runs\n" +
	"- **THEN** old works\n\n" +
	"### Requirement: Untouched Requirement\n\n" +
	"The system SHALL stay untouched.\n\n" +
	"#### Scenario: Untouched works\n\n" +
	"- **WHEN** untouched runs\n" +
	"- **THEN** untouched works\n"

const syncModifiedDeltaContent = "## MODIFIED Requirements\n\n" +
	"### Requirement: Existing Requirement\n\n" +
	"The system SHALL be NEW.\n\n" +
	"#### Scenario: New works\n\n" +
	"- **WHEN** new runs\n" +
	"- **THEN** new works\n"

// --- workspace builder ---

type syncFixture struct {
	t      *testing.T
	root   string
	change string
}

func newSyncFixture(t *testing.T, change string) *syncFixture {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "changes", change, "specs"), 0o755); err != nil {
		t.Fatalf("mkdir change tree: %v", err)
	}
	return &syncFixture{t: t, root: root, change: change}
}

func (f *syncFixture) changeRoot() string {
	return filepath.Join(f.root, "openspec", "changes", f.change)
}

func (f *syncFixture) writeChangeSpec(domain, content string) {
	f.t.Helper()
	path := filepath.Join(f.changeRoot(), "specs", domain, "spec.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatalf("mkdir delta dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write delta spec: %v", err)
	}
}

func (f *syncFixture) writeLivingSpec(domain, content string) {
	f.t.Helper()
	path := filepath.Join(f.root, "openspec", "specs", domain, "spec.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatalf("mkdir living dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatalf("write living spec: %v", err)
	}
}

// writeVerifyReport writes a passing biggz-ai.verify-result/v1 envelope whose
// requirement/scenario totals equal the actual change-spec heading counts, so
// syncVerifyMustPass admits the change.
func (f *syncFixture) writeVerifyReport() {
	f.t.Helper()
	files := findSpecFiles(filepath.Join(f.changeRoot(), "specs"))
	contents := make([]string, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			f.t.Fatalf("read spec %s: %v", path, err)
		}
		contents = append(contents, string(data))
	}
	counts := countSpecRequirementsAndScenarios(contents)
	report := fmt.Sprintf("```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\n"+
		"requirements: %d/%d\nscenarios: %d/%d\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n",
		counts.Requirements, counts.Requirements, counts.Scenarios, counts.Scenarios)
	if err := os.WriteFile(filepath.Join(f.changeRoot(), "verify-report.md"), []byte(report), 0o644); err != nil {
		f.t.Fatalf("write verify report: %v", err)
	}
}

// livingSpecsSnapshot lists every living spec as "relpath=sizefile" so a test
// can prove that a run created or modified nothing.
func (f *syncFixture) livingSpecsSnapshot() []string {
	f.t.Helper()
	specsRoot := filepath.Join(f.root, "openspec", "specs")
	var entries []string
	_ = filepath.WalkDir(specsRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(specsRoot, path)
		if relErr != nil {
			f.t.Fatalf("rel %s: %v", path, relErr)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			f.t.Fatalf("read %s: %v", path, readErr)
		}
		entries = append(entries, fmt.Sprintf("%s=%d", filepath.ToSlash(rel), len(data)))
		return nil
	})
	return entries
}

// --- integration tests ---

// T1: a mixed change applies; the full-spec new domain is copied verbatim
// (non-empty, byte-identical) and the change dir survives. Pre-fix this
// created a 0-byte living spec under `applied` (the observed false-green).
func TestSyncMixedFullSpecVerbatim(t *testing.T) {
	f := newSyncFixture(t, "chg-mixed")
	f.writeChangeSpec("fullspec-domain", syncFullSpecContent)
	f.writeChangeSpec("added-domain", syncAddedDeltaContent)
	f.writeVerifyReport()

	result, msg, err := Sync(f.change, f.root, "")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result != SyncApplied {
		t.Fatalf("result = %q, want %q (msg %q)", result, SyncApplied, msg)
	}
	livingPath := filepath.Join(f.root, "openspec", "specs", "fullspec-domain", "spec.md")
	got, err := os.ReadFile(livingPath)
	if err != nil {
		t.Fatalf("read living spec: %v", err)
	}
	if len(bytes.TrimSpace(got)) == 0 {
		t.Fatalf("living spec %s is empty under result %q (false-green)", livingPath, result)
	}
	if !bytes.Equal(got, []byte(syncFullSpecContent)) {
		t.Fatalf("living spec is not byte-identical to its delta file:\n%s", got)
	}
	if _, err := os.Stat(f.changeRoot()); err != nil {
		t.Fatalf("change dir must survive sync: %v", err)
	}

	// D6: the byte-equal re-run must stay `applied` and must not alter bytes.
	result, msg, err = Sync(f.change, f.root, "")
	if err != nil {
		t.Fatalf("re-run Sync: %v", err)
	}
	if result != SyncApplied {
		t.Fatalf("re-run result = %q, want %q (msg %q)", result, SyncApplied, msg)
	}
	again, err := os.ReadFile(livingPath)
	if err != nil {
		t.Fatalf("re-run read: %v", err)
	}
	if !bytes.Equal(again, []byte(syncFullSpecContent)) {
		t.Fatalf("re-run altered the living spec:\n%s", again)
	}
}

// T2 (pin): a full-spec-only change is skipped with `not-applicable` naming
// the change, and zero files under openspec/specs/ are created or modified.
func TestSyncFullSpecOnlyNotApplicable(t *testing.T) {
	f := newSyncFixture(t, "chg-fullspec")
	f.writeChangeSpec("fullspec-domain", syncFullSpecContent)
	before := f.livingSpecsSnapshot()

	result, msg, err := Sync(f.change, f.root, "")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result != SyncNotApplicable {
		t.Fatalf("result = %q, want %q (msg %q)", result, SyncNotApplicable, msg)
	}
	if !strings.Contains(msg, f.change) {
		t.Fatalf("message %q must name the change", msg)
	}
	if after := f.livingSpecsSnapshot(); !slices.Equal(before, after) {
		t.Fatalf("living specs changed: before %v after %v", before, after)
	}
}

// T3: content without requirement blocks fails closed naming the offending
// file and the remedy, and no living spec is created or overwritten — even
// though a valid sibling domain resolved earlier in the same run.
func TestSyncContentWithoutBlocksBlocked(t *testing.T) {
	f := newSyncFixture(t, "chg-noblocks")
	f.writeChangeSpec("noblocks-domain", syncContentWithoutBlocks)
	f.writeChangeSpec("added-domain", syncAddedDeltaContent)
	f.writeVerifyReport()
	before := f.livingSpecsSnapshot()

	result, msg, err := Sync(f.change, f.root, "")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result != SyncBlocked {
		t.Fatalf("result = %q, want %q (msg %q)", result, SyncBlocked, msg)
	}
	for _, needle := range []string{"noblocks-domain", "spec.md", "Requirement"} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("message %q must contain %q", msg, needle)
		}
	}
	if after := f.livingSpecsSnapshot(); !slices.Equal(before, after) {
		t.Fatalf("living specs changed: before %v after %v", before, after)
	}
}

// T4 (pin): MODIFIED applies in place; the existing header, ## Purpose and
// unmodified requirements survive.
func TestSyncModifiedInPlace(t *testing.T) {
	f := newSyncFixture(t, "chg-modified")
	f.writeChangeSpec("existing-domain", syncModifiedDeltaContent)
	f.writeLivingSpec("existing-domain", syncExistingLivingSpec)
	f.writeVerifyReport()

	result, msg, err := Sync(f.change, f.root, "")
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result != SyncApplied {
		t.Fatalf("result = %q, want %q (msg %q)", result, SyncApplied, msg)
	}
	data, err := os.ReadFile(filepath.Join(f.root, "openspec", "specs", "existing-domain", "spec.md"))
	if err != nil {
		t.Fatalf("read living spec: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "SHALL be NEW") {
		t.Fatalf("updated requirement missing:\n%s", got)
	}
	if strings.Contains(got, "SHALL be old") {
		t.Fatalf("old requirement body survived:\n%s", got)
	}
	for _, needle := range []string{"# Existing Domain Specification", "## Purpose", "### Requirement: Untouched Requirement", "Untouched works"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("preserved content %q missing:\n%s", needle, got)
		}
	}
}

// T5: the change-level gate is bypassed, so the writer itself is exercised on
// a full-spec-only change. It must never create a 0-byte living spec and must
// not report `applied` at the writer layer.
func TestSyncGateBypassNoEmptyWrite(t *testing.T) {
	f := newSyncFixture(t, "chg-fullspec")
	f.writeChangeSpec("fullspec-domain", syncFullSpecContent)

	infos, res, msg, err := syncParseDomainInfos(f.changeRoot())
	if err != nil {
		t.Fatalf("syncParseDomainInfos: %v", err)
	}
	if res != "" {
		t.Fatalf("parse result = %q msg = %q", res, msg)
	}
	if len(infos) != 1 {
		t.Fatalf("infos = %d, want 1", len(infos))
	}
	writeRes, writeMsg, err := syncApplyDeltas(infos, f.root)
	if err != nil {
		t.Fatalf("syncApplyDeltas: %v", err)
	}
	if writeRes == SyncApplied {
		t.Fatalf("honest status violated: writer reported %q without a full Sync (msg %q)", writeRes, writeMsg)
	}
	livingPath := filepath.Join(f.root, "openspec", "specs", "fullspec-domain", "spec.md")
	data, err := os.ReadFile(livingPath)
	if err != nil {
		t.Fatalf("living spec not created: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("living spec %s is 0 bytes (false-green)", livingPath)
	}
	if !bytes.Equal(data, []byte(syncFullSpecContent)) {
		t.Fatalf("living spec is not byte-identical to the delta file:\n%s", data)
	}
}
