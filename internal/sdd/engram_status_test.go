package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

// seedBigMemChange writes a BigMem change with given topic suffix -> content.
// It opens a bigmem store at storeRoot (directory) and saves observations.
func seedBigMemChange(t *testing.T, storeRoot, change string, artifacts map[string]string) {
	t.Helper()
	store, err := bigmem.Open(storeRoot)
	if err != nil {
		t.Fatalf("open bigmem store %s: %v", storeRoot, err)
	}
	defer store.Close()
	for suffix, content := range artifacts {
		topic := "sdd/" + change + "/" + suffix
		obs := &bigmem.Observation{
			Title:    topic,
			TopicKey: topic,
			Type:     "sdd",
			Content:  content,
			Project:  inferBigMemProjectForTest(t, storeRoot), // will be ignored when override set, but set anyway
			Scope:    "project",
		}
		// When testing with override, project filtering is disabled, so any
		// project is fine. Use a deterministic value.
		// Override the inferred project with empty to ensure visibility in both
		// filtered and unfiltered modes.
		obs.Project = "unfiltered-project"
		// For override mode, we actually want observations to be visible
		// regardless of project, so we use a value that will be ignored.
		// Save with empty project would also be visible? But our collector
		// skips filtering when override != "", so any project passes.
		if err := store.Save(obs); err != nil {
			t.Fatalf("save %s: %v", topic, err)
		}
	}
}

// inferBigMemProjectForTest returns a project that matches what
// collectBigMemChangesWithArchive will infer for a given workspace.
// In override mode filtering is disabled, so this is only for
// documentation; we store a dummy project.
func inferBigMemProjectForTest(t *testing.T, _ string) string {
	t.Helper()
	return "unfiltered-project"
}

// directSeedBigMem uses raw SQLite to avoid project filtering complexity
// for the simplest tests: it saves with project matching the workspace
// inference when needed, but we use the override-disabled path.
func directSeedBigMemWithStore(t *testing.T, storeRoot, change string, artifacts map[string]string, project string) {
	t.Helper()
	store, err := bigmem.Open(storeRoot)
	if err != nil {
		t.Fatalf("open bigmem: %v", err)
	}
	defer store.Close()
	for suffix, content := range artifacts {
		topic := "sdd/" + change + "/" + suffix
		obs := &bigmem.Observation{
			Title:    topic,
			TopicKey: topic,
			Type:     "sdd",
			Content:  content,
			Project:  project,
			Scope:    "project",
		}
		if err := store.Save(obs); err != nil {
			t.Fatalf("save %s: %v", topic, err)
		}
	}
}

func TestCollectBigMemChanges_Hybrid(t *testing.T) {
	storeRoot := t.TempDir()
	workspace := t.TempDir()
	// Need openspec/changes so StatusWithOptions doesn't fail on missing dir
	os.MkdirAll(filepath.Join(workspace, "openspec", "changes", "archive"), 0755)

	// Override to temp store
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	// Seed two BigMem changes: one active, one archived
	directSeedBigMemWithStore(t, storeRoot, "bigmem-active", map[string]string{
		"proposal": "# Proposal\n",
		"spec":     "### Requirement: R1\n#### Scenario: S1\n",
		"design":   "# Design\n",
		"tasks":    "- [x] T1\n- [ ] T2\n",
	}, "unfiltered-project")
	directSeedBigMemWithStore(t, storeRoot, "bigmem-archived", map[string]string{
		"proposal":       "# Proposal\n",
		"spec":           "### Requirement: R1\n#### Scenario: S1\n",
		"design":         "# Design\n",
		"tasks":          "- [x] T1\n",
		"verify-report":  "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 1/1\nscenarios: 1/1\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n",
		"archive-report": "# archived\n",
	}, "unfiltered-project")

	active, archived, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("collectBigMemChangesWithArchive: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d: %#v", len(active), active)
	}
	if active[0].Name != "bigmem-active" {
		t.Errorf("active name = %q, want bigmem-active", active[0].Name)
	}
	if active[0].NextRecommended != "apply" {
		t.Errorf("active NextRecommended = %q, want apply", active[0].NextRecommended)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived, got %d", len(archived))
	}
	if archived[0].Name != "bigmem-archived" {
		t.Errorf("archived name = %q, want bigmem-archived", archived[0].Name)
	}
	if !archived[0].IsArchived || archived[0].NextRecommended != "done" {
		t.Errorf("archived status = IsArchived %v Next %q, want true/done", archived[0].IsArchived, archived[0].NextRecommended)
	}
}

func TestCollectBigMemChanges_NoStoreFallsBackToNil(t *testing.T) {
	workspace := t.TempDir()
	os.MkdirAll(filepath.Join(workspace, "openspec", "changes"), 0755)
	// Point override to a directory that has no bigmem.db
	emptyStore := t.TempDir()
	SetBigMemStoreRootForTest(emptyStore)
	defer SetBigMemStoreRootForTest("")
	active, archived, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(active) != 0 || len(archived) != 0 {
		t.Errorf("expected nil slices when no sdd observations, got active %d archived %d", len(active), len(archived))
	}
	// also test the spec-compliant single-slice wrapper
	single, err := collectBigMemChanges(workspace)
	if err != nil {
		t.Fatalf("collectBigMemChanges: %v", err)
	}
	if len(single) != 0 {
		t.Errorf("single-slice expected 0, got %d", len(single))
	}
}

func TestMergeFilesystemAndBigMem_FilesystemWinsOnConflict(t *testing.T) {
	fsActive := []ChangeStatus{{Name: "same-change", NextRecommended: "spec"}}
	fsArchived := []ChangeStatus{{Name: "old-change", IsArchived: true}}
	memActive := []ChangeStatus{{Name: "same-change", NextRecommended: "apply"}, {Name: "bigmem-only", NextRecommended: "propose"}}
	memArchived := []ChangeStatus{{Name: "old-change", IsArchived: true}, {Name: "bigmem-archived", IsArchived: true}}

	active, archived := mergeFilesystemAndBigMem(fsActive, fsArchived, memActive, memArchived)

	if len(active) != 2 {
		t.Fatalf("active len = %d, want 2 (filesystem wins + bigmem-only)", len(active))
	}
	// filesystem version must be kept
	for _, cs := range active {
		if cs.Name == "same-change" && cs.NextRecommended != "spec" {
			t.Errorf("filesystem wins: same-change Next = %q, want spec", cs.NextRecommended)
		}
	}
	// bigmem-only must appear
	found := false
	for _, cs := range active {
		if cs.Name == "bigmem-only" {
			found = true
		}
	}
	if !found {
		t.Error("expected bigmem-only in active")
	}
	if len(archived) != 2 {
		t.Fatalf("archived len = %d, want 2", len(archived))
	}
	// ensure bigmem-archived present, old-change kept as filesystem version
	names := map[string]bool{}
	for _, cs := range archived {
		names[cs.Name] = true
	}
	if !names["old-change"] || !names["bigmem-archived"] {
		t.Errorf("archived names = %v, want old-change and bigmem-archived", names)
	}
}

func TestMergeFilesystemAndBigMem_CrossListDedup(t *testing.T) {
	// Change is archived on filesystem but active in BigMem — filesystem archived wins, active duplicate dropped
	fsActive := []ChangeStatus{}
	fsArchived := []ChangeStatus{{Name: "dup", IsArchived: true, NextRecommended: "done"}}
	memActive := []ChangeStatus{{Name: "dup", NextRecommended: "apply"}}
	memArchived := []ChangeStatus{}
	active, archived := mergeFilesystemAndBigMem(fsActive, fsArchived, memActive, memArchived)
	if len(active) != 0 {
		t.Errorf("active should be empty when dup is archived on fs, got %d", len(active))
	}
	if len(archived) != 1 || archived[0].Name != "dup" {
		t.Errorf("archived should contain fs dup, got %#v", archived)
	}
}

func TestStatusWithOptions_HybridMergesBigMem(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	changesDir := filepath.Join(openspecRoot, "changes")
	os.MkdirAll(filepath.Join(changesDir, "archive"), 0755)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	// Filesystem change
	fsChangeDir := filepath.Join(changesDir, "fs-change")
	os.MkdirAll(fsChangeDir, 0755)
	os.WriteFile(filepath.Join(fsChangeDir, "proposal.md"), []byte("# Proposal\n"), 0644)

	// BigMem change
	directSeedBigMemWithStore(t, storeRoot, "bigmem-change", map[string]string{
		"proposal": "# Proposal\n",
		"spec":     "### Requirement: R1\n#### Scenario: S1\n",
		"design":   "# Design\n",
		"tasks":    "- [x] T1\n- [ ] T2\n",
	}, "unfiltered-project")

	active, archived, err := StatusWithOptions(openspecRoot, StatusOptions{})
	if err != nil {
		t.Fatalf("StatusWithOptions: %v", err)
	}
	_ = archived
	if len(active) != 2 {
		t.Fatalf("expected 2 active (fs + bigmem), got %d: %v", len(active), func() []string {
			var n []string
			for _, c := range active {
				n = append(n, c.Name)
			}
			return n
		}())
	}
	names := map[string]bool{}
	for _, cs := range active {
		names[cs.Name] = true
	}
	if !names["fs-change"] || !names["bigmem-change"] {
		t.Errorf("active names = %v, want fs-change and bigmem-change", names)
	}
	// BigMem change should have derived status apply
	for _, cs := range active {
		if cs.Name == "bigmem-change" && cs.NextRecommended != "apply" {
			t.Errorf("bigmem-change Next = %q, want apply", cs.NextRecommended)
		}
	}
}

func TestStatusWithOptions_FilesystemWinsOnConflictHybrid(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	changesDir := filepath.Join(openspecRoot, "changes")
	os.MkdirAll(filepath.Join(changesDir, "archive"), 0755)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	// Filesystem change "conflict-change" with only proposal -> next spec
	fsDir := filepath.Join(changesDir, "conflict-change")
	os.MkdirAll(fsDir, 0755)
	os.WriteFile(filepath.Join(fsDir, "proposal.md"), []byte("# Proposal\n"), 0644)

	// BigMem change with same name but fully planned (would route to apply)
	directSeedBigMemWithStore(t, storeRoot, "conflict-change", map[string]string{
		"proposal": "# Proposal\n",
		"spec":     "### Requirement: R1\n#### Scenario: S1\n",
		"design":   "# Design\n",
		"tasks":    "- [x] T1\n- [ ] T2\n",
	}, "unfiltered-project")

	active, _, err := StatusWithOptions(openspecRoot, StatusOptions{})
	if err != nil {
		t.Fatalf("StatusWithOptions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active (filesystem wins), got %d", len(active))
	}
	if active[0].Name != "conflict-change" {
		t.Fatalf("name = %q, want conflict-change", active[0].Name)
	}
	// Filesystem version should win: only proposal -> spec
	if active[0].NextRecommended != "spec" {
		t.Errorf("filesystem wins: Next = %q, want spec (fs version), not apply (bigmem version)", active[0].NextRecommended)
	}
	// also verify ChangeRoot is filesystem path, not bigmem:
	if !strings.Contains(active[0].ChangeRoot, "conflict-change") || strings.HasPrefix(active[0].ChangeRoot, "bigmem:") {
		t.Errorf("ChangeRoot = %q, want filesystem path", active[0].ChangeRoot)
	}
}

func TestStatusWithOptions_BigMemInstructions(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	os.MkdirAll(filepath.Join(openspecRoot, "changes", "archive"), 0755)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	directSeedBigMemWithStore(t, storeRoot, "instr-change", map[string]string{
		"proposal": "# Proposal\n",
		"spec":     "### Requirement: R1\n#### Scenario: S1\n",
		"design":   "# Design\n",
		"tasks":    "- [x] T1\n- [ ] T2\n",
	}, "unfiltered-project")

	active, _, err := StatusWithOptions(openspecRoot, StatusOptions{IncludeInstructions: true})
	if err != nil {
		t.Fatalf("StatusWithOptions: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d", len(active))
	}
	if active[0].PhaseInstructions == nil {
		t.Fatal("expected PhaseInstructions != nil when IncludeInstructions true")
	}
	if len(active[0].PhaseInstructions.Apply) == 0 {
		t.Error("expected non-empty Apply instructions")
	}
}

func TestStatusWithOptions_FilesystemOnlyWhenBigMemEmpty(t *testing.T) {
	workspace := t.TempDir()
	openspecRoot := filepath.Join(workspace, "openspec")
	changesDir := filepath.Join(openspecRoot, "changes")
	os.MkdirAll(filepath.Join(changesDir, "archive"), 0755)
	// filesystem change
	dir := filepath.Join(changesDir, "only-fs")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("# P\n"), 0644)

	// BigMem store with no sdd observations
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")
	// create empty store so DB exists but no sdd data
	store, _ := bigmem.Open(storeRoot)
	store.Close()

	active, _, err := StatusWithOptions(openspecRoot, StatusOptions{})
	if err != nil {
		t.Fatalf("StatusWithOptions: %v", err)
	}
	if len(active) != 1 || active[0].Name != "only-fs" {
		t.Errorf("expected only-fs, got %v", active)
	}
}
