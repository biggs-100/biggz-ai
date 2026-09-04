package sdd

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	_ "modernc.org/sqlite"
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

// seedBigMemRow saves a single observation with explicit project/scope and
// returns its ID. It opens the store per call; batch seeders below reuse one
// handle for the 100-row hydration test.
func seedBigMemRow(t *testing.T, storeRoot, topic, project, scope, content string) string {
	t.Helper()
	store, err := bigmem.Open(storeRoot)
	if err != nil {
		t.Fatalf("open bigmem: %v", err)
	}
	defer store.Close()
	obs := &bigmem.Observation{
		Title:    topic,
		TopicKey: topic,
		Type:     "sdd",
		Content:  content,
		Project:  project,
		Scope:    scope,
	}
	if err := store.Save(obs); err != nil {
		t.Fatalf("save %s: %v", topic, err)
	}
	return obs.ID
}

func mkStatusWorkspace(t *testing.T) (workspace, openspecRoot string) {
	t.Helper()
	workspace = t.TempDir()
	openspecRoot = filepath.Join(workspace, "openspec")
	if err := os.MkdirAll(filepath.Join(openspecRoot, "changes", "archive"), 0755); err != nil {
		t.Fatalf("mkdir changes: %v", err)
	}
	return workspace, openspecRoot
}

func TestCollectBigMemChanges_PersonalExcluded(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	seedBigMemRow(t, storeRoot, "sdd/pub-change/proposal", "unfiltered-project", "project", "# Proposal\n")
	seedBigMemRow(t, storeRoot, "sdd/priv-change/proposal", "unfiltered-project", "personal", "# Proposal\n")
	seedBigMemRow(t, storeRoot, "sdd/priv-change2/proposal", "unfiltered-project", "Personal", "# Proposal\n")

	active, _, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("collectBigMemChangesWithArchive: %v", err)
	}
	if len(active) != 1 || active[0].Name != "pub-change" {
		names := []string{}
		for _, cs := range active {
			names = append(names, cs.Name)
		}
		t.Fatalf("expected only pub-change, got %v", names)
	}
}

func TestCollectBigMemChanges_ProjectOverrideDisablesFilter(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	// Arbitrary projects stay visible while the test override is set.
	seedBigMemRow(t, storeRoot, "sdd/foreign-change/proposal", "some-other-project", "project", "# Proposal\n")
	seedBigMemRow(t, storeRoot, "sdd/local-change/proposal", "unfiltered-project", "project", "# Proposal\n")

	active, _, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("collectBigMemChangesWithArchive: %v", err)
	}
	names := map[string]bool{}
	for _, cs := range active {
		names[cs.Name] = true
	}
	if !names["foreign-change"] || !names["local-change"] {
		t.Errorf("override must disable project filter, got %v", names)
	}
	// Production filtering (non-override) is enforced by the
	// ListByTopicPrefixCtx SQL predicate; see TestListByTopicPrefix_ProjectMatchCaseInsensitive.
}

func TestCollectBigMemChanges_VisibleOnlyHydration(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	store, err := bigmem.Open(storeRoot)
	if err != nil {
		t.Fatalf("open bigmem: %v", err)
	}
	save := func(topic, project, scope, content string) string {
		t.Helper()
		obs := &bigmem.Observation{Title: topic, TopicKey: topic, Type: "sdd", Content: content, Project: project, Scope: scope}
		if err := store.Save(obs); err != nil {
			t.Fatalf("save %s: %v", topic, err)
		}
		return obs.ID
	}
	// 2 visible changes with full planning content.
	visibleTasks := "- [x] T1\n- [x] T2\n- [ ] T3\n"
	save("sdd/vis-one/proposal", "unfiltered-project", "project", "# Proposal\n")
	save("sdd/vis-one/spec", "unfiltered-project", "project", "### Requirement: R1\n#### Scenario: S1\n")
	save("sdd/vis-one/design", "unfiltered-project", "project", "# Design\n")
	save("sdd/vis-one/tasks", "unfiltered-project", "project", visibleTasks)
	save("sdd/vis-two/proposal", "unfiltered-project", "project", "# Proposal\n")
	save("sdd/vis-two/tasks", "unfiltered-project", "project", "- [x] Only\n")
	// 98 noise rows: other prefixes, personal scope, non-pattern topics, deleted.
	var deletedIDs []string
	for i := 0; i < 30; i++ {
		save("other/noise/proposal", "unfiltered-project", "project", "# noise\n")
	}
	for i := 0; i < 30; i++ {
		save("sdd/hidden/proposal", "unfiltered-project", "personal", "# hidden\n")
	}
	for i := 0; i < 20; i++ {
		save("sdd/freeform-note", "unfiltered-project", "project", "# not an artifact topic\n")
	}
	for i := 0; i < 12; i++ {
		deletedIDs = append(deletedIDs, save("sdd/doomed/proposal", "unfiltered-project", "project", "# doomed\n"))
	}
	store.Close()
	// Soft-delete outside the write handle so the rows exist but are filtered.
	delStore, err := bigmem.Open(storeRoot)
	if err != nil {
		t.Fatalf("reopen bigmem: %v", err)
	}
	for _, id := range deletedIDs {
		if err := delStore.Delete(id); err != nil {
			t.Fatalf("delete %s: %v", id, err)
		}
	}
	delStore.Close()

	active, archived, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("collectBigMemChangesWithArchive: %v", err)
	}
	if len(archived) != 0 {
		t.Errorf("expected 0 archived, got %d", len(archived))
	}
	if len(active) != 2 {
		names := []string{}
		for _, cs := range active {
			names = append(names, cs.Name)
		}
		t.Fatalf("expected exactly 2 visible changes, got %v", names)
	}
	byName := map[string]ChangeStatus{}
	for _, cs := range active {
		byName[cs.Name] = cs
	}
	one, ok := byName["vis-one"]
	if !ok {
		t.Fatalf("vis-one missing: %v", byName)
	}
	if one.TasksTotal != 3 || one.TasksDone != 2 {
		t.Errorf("vis-one tasks = %d/%d, want 2/3 (content must hydrate)", one.TasksDone, one.TasksTotal)
	}
	if one.Artifacts["proposal"] != ArtifactDone || one.Artifacts["specs"] != ArtifactDone ||
		one.Artifacts["design"] != ArtifactDone || one.Artifacts["tasks"] != ArtifactDone {
		t.Errorf("vis-one artifacts must match full-hydration states, got %v", one.Artifacts)
	}
	two, ok := byName["vis-two"]
	if !ok {
		t.Fatalf("vis-two missing: %v", byName)
	}
	if two.TasksTotal != 1 || two.TasksDone != 1 {
		t.Errorf("vis-two tasks = %d/%d, want 1/1", two.TasksDone, two.TasksTotal)
	}
}

func TestCollectBigMemChanges_CorruptDBSurfacesError(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	dbPath, err := bigmem.ResolveDBPath(storeRoot)
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("not a sqlite database, corrupt"), 0644); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")

	active, archived, err := collectBigMemChangesWithArchive(workspace, false)
	if err == nil {
		t.Fatalf("expected wrapped error for corrupt DB, got nil (active=%d archived=%d)", len(active), len(archived))
	}
	if !strings.Contains(err.Error(), "bigmem sdd-status") {
		t.Errorf("error must name the operation, got: %v", err)
	}
}

func TestCollectBigMemChanges_CancelledCtxFailsFast(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")
	seedBigMemRow(t, storeRoot, "sdd/x/proposal", "unfiltered-project", "project", "# P\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := collectBigMemChangesWithArchiveCtx(ctx, workspace, false)
	if err == nil {
		t.Fatal("expected wrapped ctx error, got nil")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error must wrap Canceled/DeadlineExceeded, got: %v", err)
	}
}

func TestStatusCtxCancel(t *testing.T) {
	_, openspecRoot := mkStatusWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := StatusWithOptionsCtx(ctx, openspecRoot, StatusOptions{})
	if err == nil {
		t.Fatal("expected wrapped ctx error, got nil")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error must wrap Canceled/DeadlineExceeded, got: %v", err)
	}
}

func TestCollectBigMemChanges_HydrationErrorFails(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")
	seedBigMemRow(t, storeRoot, "sdd/hyd-fail/proposal", "unfiltered-project", "project", "# P\n")
	dbPath, err := bigmem.ResolveDBPath(storeRoot)
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := db.Exec(`UPDATE observations SET revision_count='not-an-int' WHERE topic_key='sdd/hyd-fail/proposal'`); err != nil {
		db.Close()
		t.Fatalf("corrupt row: %v", err)
	}
	db.Close()
	_, _, err = collectBigMemChangesWithArchive(workspace, false)
	if err == nil {
		t.Fatal("expected wrapped hydration error, got nil (partial success not allowed)")
	}
	if !strings.Contains(err.Error(), "bigmem sdd-status") {
		t.Errorf("error must name the operation, got: %v", err)
	}
}

func TestStatusWithOptions_HybridPropagatesBigMemError(t *testing.T) {
	workspace, openspecRoot := mkStatusWorkspace(t)
	fsDir := filepath.Join(openspecRoot, "changes", "fs-ok")
	os.MkdirAll(fsDir, 0755)
	os.WriteFile(filepath.Join(fsDir, "proposal.md"), []byte("# P\n"), 0644)
	storeRoot := t.TempDir()
	dbPath, err := bigmem.ResolveDBPath(storeRoot)
	if err != nil {
		t.Fatalf("resolve db path: %v", err)
	}
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	if err := os.WriteFile(dbPath, []byte("not a sqlite database, corrupt"), 0644); err != nil {
		t.Fatalf("write corrupt db: %v", err)
	}
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")
	_, _, err = StatusWithOptions(openspecRoot, StatusOptions{})
	if err == nil {
		t.Fatal("expected hybrid BigMem error to propagate, got nil (silent success not allowed)")
	}
	if !strings.Contains(err.Error(), "bigmem sdd-status") {
		t.Errorf("error must name the operation, got: %v", err)
	}
	_ = workspace
}

func TestCollectBigMemChanges_ExploreExcludedFromSeen(t *testing.T) {
	workspace, _ := mkStatusWorkspace(t)
	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")
	seedBigMemRow(t, storeRoot, "sdd/ghost/explore", "unfiltered-project", "project", "# explore\n")
	seedBigMemRow(t, storeRoot, "sdd/ghost2/state", "unfiltered-project", "project", "# state\n")
	seedBigMemRow(t, storeRoot, "sdd/real/proposal", "unfiltered-project", "project", "# P\n")
	seedBigMemRow(t, storeRoot, "sdd/real/explore", "unfiltered-project", "project", "# explore\n")
	active, _, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	names := map[string]bool{}
	for _, cs := range active {
		names[cs.Name] = true
	}
	if names["ghost"] || names["ghost2"] {
		t.Errorf("explore/state-only changes must stay invisible, got %v", names)
	}
	if !names["real"] {
		t.Errorf("proposal+explore change must stay visible, got %v", names)
	}
}
