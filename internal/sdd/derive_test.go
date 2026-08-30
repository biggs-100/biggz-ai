package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedChange writes the given artifact files (relative path → content) into a
// change directory and returns the change root.
func seedDeriveChange(t *testing.T, workspace, change string, files map[string]string) string {
	t.Helper()
	changeRoot := filepath.Join(workspace, "openspec", "changes", change)
	if err := os.MkdirAll(changeRoot, 0755); err != nil {
		t.Fatalf("mkdir change root: %v", err)
	}
	for rel, content := range files {
		path := filepath.Join(changeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return changeRoot
}

const specFixture = "### Requirement: Capability\n#### Scenario: Works\n"

func passingReport(requirements, scenarios string) string {
	return "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\n" +
		"requirements: " + requirements + "\nscenarios: " + scenarios + "\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n"
}

func failingReport(requirements, scenarios string) string {
	return "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: fail\nblockers: 0\ncritical_findings: 0\n" +
		"requirements: " + requirements + "\nscenarios: " + scenarios + "\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n"
}

// TestDeriveChangeStatusMatrix is the T1 derivation matrix: every route from
// an empty change to archive, plus the genuine-anomaly and partial-artifact
// states. Each row asserts the derived nextRecommended, applyState,
// dependency states, taskProgress, artifact states, and blocked reasons.
func TestDeriveChangeStatusMatrix(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		archived   bool
		wantNext   string
		wantApply  ApplyState
		wantDeps   Dependencies
		wantTasks  TaskProgress
		wantReason string
	}{
		{
			name:      "empty change routes to propose",
			files:     map[string]string{},
			wantNext:  "propose",
			wantApply: ApplyBlocked,
			wantDeps:  Dependencies{Proposal: DependencyBlocked, Specs: DependencyBlocked, Design: DependencyBlocked, Tasks: DependencyBlocked, Apply: DependencyBlocked, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
		},
		{
			name:      "proposal only routes to spec",
			files:     map[string]string{"proposal.md": "# Proposal\n"},
			wantNext:  "spec",
			wantApply: ApplyBlocked,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyBlocked, Design: DependencyBlocked, Tasks: DependencyBlocked, Apply: DependencyBlocked, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
		},
		{
			name:      "proposal plus specs routes to design",
			files:     map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture},
			wantNext:  "design",
			wantApply: ApplyBlocked,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyBlocked, Tasks: DependencyBlocked, Apply: DependencyBlocked, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
		},
		{
			name:      "proposal specs design routes to tasks",
			files:     map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n"},
			wantNext:  "tasks",
			wantApply: ApplyBlocked,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyBlocked, Apply: DependencyBlocked, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
		},
		{
			name:      "tasks partial routes to apply",
			files:     map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "- [x] T1\n- [ ] T2\n"},
			wantNext:  "apply",
			wantApply: ApplyReady,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyAllDone, Apply: DependencyReady, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
			wantTasks: TaskProgress{Total: 2, Completed: 1, Pending: 1},
		},
		{
			name:      "tasks all done without verify routes to verify",
			files:     map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "- [x] T1\n- [x] T2\n"},
			wantNext:  "verify",
			wantApply: ApplyAllDone,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyAllDone, Apply: DependencyAllDone, Verify: DependencyReady, Sync: DependencyBlocked, Archive: DependencyBlocked},
			wantTasks: TaskProgress{Total: 2, Completed: 2, AllComplete: true},
		},
		{
			name:       "failing verify report routes to remediate with reason",
			files:      map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "- [x] T1\n", "verify-report.md": failingReport("1/1", "1/1")},
			wantNext:   "remediate",
			wantApply:  ApplyAllDone,
			wantDeps:   Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyAllDone, Apply: DependencyAllDone, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
			wantTasks:  TaskProgress{Total: 1, Completed: 1, AllComplete: true},
			wantReason: "verify evidence requires unmanaged remediation",
		},
		{
			name:      "passing verify report routes to archive",
			files:     map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "- [x] T1\n", "verify-report.md": passingReport("1/1", "1/1")},
			wantNext:  "archive",
			wantApply: ApplyAllDone,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyAllDone, Apply: DependencyAllDone, Verify: DependencyAllDone, Sync: DependencyAllDone, Archive: DependencyReady},
			wantTasks: TaskProgress{Total: 1, Completed: 1, AllComplete: true},
		},
		{
			name:       "zero-checkbox tasks is a genuine blocker",
			files:      map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "Some prose without any checkboxes.\n"},
			wantNext:   "resolve-blockers",
			wantApply:  ApplyBlocked,
			wantDeps:   Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyAllDone, Apply: DependencyBlocked, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
			wantReason: "tasks.md has no markdown task checkboxes.",
		},
		{
			name:      "empty-file artifacts are partial",
			files:     map[string]string{"proposal.md": "", "specs/core/spec.md": "", "design.md": "", "tasks.md": ""},
			wantNext:  "propose",
			wantApply: ApplyBlocked,
			wantDeps:  Dependencies{Proposal: DependencyBlocked, Specs: DependencyBlocked, Design: DependencyBlocked, Tasks: DependencyBlocked, Apply: DependencyBlocked, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
		},
		{
			name:      "checkbox style variants count uniformly",
			files:     map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "- [ ] A\n* [x] B\n1. [ ] C\n2. [x] D\n- [X] E\n"},
			wantNext:  "apply",
			wantApply: ApplyReady,
			wantDeps:  Dependencies{Proposal: DependencyAllDone, Specs: DependencyAllDone, Design: DependencyAllDone, Tasks: DependencyAllDone, Apply: DependencyReady, Verify: DependencyBlocked, Sync: DependencyBlocked, Archive: DependencyBlocked},
			wantTasks: TaskProgress{Total: 5, Completed: 3, Pending: 2},
		},
		{
			name:     "archived change reports done",
			files:    map[string]string{"proposal.md": "# Proposal\n", "specs/core/spec.md": specFixture, "design.md": "# Design\n", "tasks.md": "- [x] T1\n", "verify-report.md": passingReport("1/1", "1/1")},
			archived: true,
			wantNext: "done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := t.TempDir()
			changeRoot := seedDeriveChange(t, workspace, "matrix-change", tt.files)
			cs, err := readChange(changeRoot, "matrix-change", tt.archived, workspace, false)
			if err != nil {
				t.Fatalf("readChange: %v", err)
			}

			if cs.NextRecommended != tt.wantNext {
				t.Errorf("NextRecommended = %q, want %q", cs.NextRecommended, tt.wantNext)
			}
			if cs.ApplyState != tt.wantApply {
				t.Errorf("ApplyState = %q, want %q", cs.ApplyState, tt.wantApply)
			}
			if cs.Dependencies != tt.wantDeps {
				t.Errorf("Dependencies = %#v, want %#v", cs.Dependencies, tt.wantDeps)
			}
			if tt.wantTasks != (TaskProgress{}) && cs.TaskProgress != tt.wantTasks {
				t.Errorf("TaskProgress = %#v, want %#v", cs.TaskProgress, tt.wantTasks)
			}
			if tt.wantReason != "" {
				found := false
				for _, reason := range cs.BlockedReasons {
					if strings.Contains(reason, tt.wantReason) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("BlockedReasons = %#v, want one containing %q", cs.BlockedReasons, tt.wantReason)
				}
			}
		})
	}
}

// TestDerivePartialArtifacts asserts the artifact map states for the
// empty-file row of the matrix: every empty artifact is partial, and the
// specs entry is partial when the directory holds only empty spec.md files.
func TestDerivePartialArtifacts(t *testing.T) {
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "partial-change", map[string]string{
		"proposal.md":        "",
		"specs/core/spec.md": "",
		"design.md":          "",
		"tasks.md":           "",
		"apply-progress.md":  "",
	})
	cs, err := readChange(changeRoot, "partial-change", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	want := map[string]ArtifactState{
		"proposal":      ArtifactPartial,
		"specs":         ArtifactPartial,
		"design":        ArtifactPartial,
		"tasks":         ArtifactPartial,
		"applyProgress": ArtifactPartial,
		"verifyReport":  ArtifactMissing,
	}
	for key, state := range want {
		if cs.Artifacts[key] != state {
			t.Errorf("Artifacts[%q] = %q, want %q", key, cs.Artifacts[key], state)
		}
	}
}

// TestDeriveSpecsPartialWhenSomeSpecEmpty asserts the specs rule: all found
// spec.md files must have content for specs to be done.
func TestDeriveSpecsPartialWhenSomeSpecEmpty(t *testing.T) {
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "specs-partial", map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/api/spec.md":  specFixture,
		"specs/core/spec.md": "",
	})
	cs, err := readChange(changeRoot, "specs-partial", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.Artifacts["specs"] != ArtifactPartial {
		t.Errorf("Artifacts[specs] = %q, want partial", cs.Artifacts["specs"])
	}
	if cs.Dependencies.Specs != DependencyBlocked {
		t.Errorf("Dependencies.Specs = %q, want blocked", cs.Dependencies.Specs)
	}
	if cs.NextRecommended != "spec" {
		t.Errorf("NextRecommended = %q, want spec", cs.NextRecommended)
	}
}

// TestDeriveExpectedPlanningReasonsHiddenForPlanningRoutes asserts the
// finalize rule: planning routes emit only genuine reasons, so a change
// missing its specs carries an empty blockedReasons list.
func TestDeriveExpectedPlanningReasonsHiddenForPlanningRoutes(t *testing.T) {
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "planning-change", map[string]string{
		"proposal.md": "# Proposal\n",
	})
	cs, err := readChange(changeRoot, "planning-change", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.NextRecommended != "spec" {
		t.Fatalf("NextRecommended = %q, want spec", cs.NextRecommended)
	}
	if len(cs.BlockedReasons) != 0 {
		t.Errorf("BlockedReasons = %#v, want empty for planning routes", cs.BlockedReasons)
	}
}

// TestBlockerReasonsFinalizeRule pins the finalize rule directly: planning
// routes emit only genuine reasons; every other route emits expectedPlanning
// followed by genuine.
func TestBlockerReasonsFinalizeRule(t *testing.T) {
	reasons := blockerReasons{
		expectedPlanning: []string{"proposal.md is missing or partial."},
		genuine:          []string{"tasks.md has no markdown task checkboxes."},
	}
	for _, phase := range []string{"propose", "spec", "design", "tasks"} {
		got := reasons.finalize(phase)
		if len(got) != 1 || got[0] != reasons.genuine[0] {
			t.Errorf("finalize(%q) = %#v, want only the genuine reason", phase, got)
		}
	}
	for _, phase := range []string{"apply", "verify", "remediate", "archive", "resolve-blockers", "done"} {
		got := reasons.finalize(phase)
		want := []string{reasons.expectedPlanning[0], reasons.genuine[0]}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("finalize(%q) = %#v, want %#v", phase, got, want)
		}
	}
}

// TestDeriveSchemaAndActionContext asserts the stable identity fields every
// derived status carries.
func TestDeriveSchemaAndActionContext(t *testing.T) {
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "identity-change", map[string]string{
		"proposal.md": "# Proposal\n",
	})
	cs, err := readChange(changeRoot, "identity-change", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.SchemaName != StatusSchemaName {
		t.Errorf("SchemaName = %q, want %q", cs.SchemaName, StatusSchemaName)
	}
	if cs.SchemaVersion != StatusSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", cs.SchemaVersion, StatusSchemaVersion)
	}
	if cs.ChangeRoot != changeRoot {
		t.Errorf("ChangeRoot = %q, want %q", cs.ChangeRoot, changeRoot)
	}
	if cs.ActionContext.Mode != "repo-local" || cs.ActionContext.WorkspaceRoot != workspace {
		t.Errorf("ActionContext = %#v, want repo-local at %q", cs.ActionContext, workspace)
	}
	if len(cs.ActionContext.AllowedEditRoots) != 1 || cs.ActionContext.AllowedEditRoots[0] != workspace {
		t.Errorf("AllowedEditRoots = %#v, want [%q]", cs.ActionContext.AllowedEditRoots, workspace)
	}
	if len(cs.ArtifactPaths.Proposal) != 1 || cs.ArtifactPaths.Proposal[0] != filepath.Join(changeRoot, "proposal.md") {
		t.Errorf("ArtifactPaths.Proposal = %#v", cs.ArtifactPaths.Proposal)
	}
}
