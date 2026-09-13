package sdd

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// laneTestEnv isolates HOME/USERPROFILE and disables RDD globally so lane
// derivations are deterministic regardless of the ambient environment.
func laneTestEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
}

// enableRDDForTest isolates HOME and enables RDD globally so the terminal
// gates are exercised: a lane change must hit exactly the same gate as a
// full-pipeline change.
func enableRDDForTest(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, err := review.RDDEnable("", ""); err != nil {
		t.Fatalf("RDDEnable: %v", err)
	}
	t.Cleanup(func() { _, _ = review.RDDDisable("", "", "global") })
}

// lanePlan is the canonical fast-lane scaffold: the ### Requirement: /
// #### Scenario: headings feed verify admission counts and the checklist
// feeds task progress.
const lanePlan = "### Requirement: Fast Lane\n#### Scenario: Plan-only change reaches apply\n\n- [ ] T1\n- [ ] T2\n"

// lanePlanDone is the same scaffold with every checkbox complete.
var lanePlanDone = strings.ReplaceAll(lanePlan, "- [ ]", "- [x]")

// rddGateReasonPrefix returns the typed prefix of the first RDD gate reason
// in the list ("" when the gate produced no reason).
func rddGateReasonPrefix(reasons []string) string {
	for _, reason := range reasons {
		for _, prefix := range []string{"rdd_receipt_missing", "rdd_unproducible"} {
			if strings.HasPrefix(reason, prefix) {
				return prefix
			}
		}
	}
	return ""
}

// assertSlotPath pins a resolved artifact path slot to exactly one path.
func assertSlotPath(t *testing.T, slot string, got []string, want string) {
	t.Helper()
	if len(got) != 1 || got[0] != want {
		t.Errorf("ArtifactPaths.%s = %#v, want [%s]", slot, got, want)
	}
}

// TestLanePlanOnlyReachesApplyThenVerifyThenArchive is REQ-ALIAS scenario
// "Plan-only change reaches apply": a plan-only fixture through the real
// dispatcher (readChange) resolves all four planning slots to plan.md, apply
// is ready, and the change walks apply -> verify -> archive as work lands.
func TestLanePlanOnlyReachesApplyThenVerifyThenArchive(t *testing.T) {
	laneTestEnv(t)
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "lane-core", map[string]string{"plan.md": lanePlan})

	cs, err := readChange(changeRoot, "lane-core", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	for _, slot := range []string{"proposal", "specs", "design", "tasks"} {
		if cs.Artifacts[slot] != ArtifactDone {
			t.Errorf("Artifacts[%s] = %q, want done", slot, cs.Artifacts[slot])
		}
	}
	if cs.Artifacts["verifyReport"] != ArtifactMissing {
		t.Errorf("Artifacts[verifyReport] = %q, want missing (never aliases)", cs.Artifacts["verifyReport"])
	}
	if cs.ApplyState != ApplyReady || cs.NextRecommended != "apply" {
		t.Fatalf("applyState/next = %q/%q, want ready/apply", cs.ApplyState, cs.NextRecommended)
	}
	planPath := filepath.Join(changeRoot, "plan.md")
	assertSlotPath(t, "Proposal", cs.ArtifactPaths.Proposal, planPath)
	assertSlotPath(t, "Specs", cs.ArtifactPaths.Specs, planPath)
	assertSlotPath(t, "Design", cs.ArtifactPaths.Design, planPath)
	assertSlotPath(t, "Tasks", cs.ArtifactPaths.Tasks, planPath)
	if len(cs.ArtifactPaths.ApplyProgress) != 0 || len(cs.ArtifactPaths.VerifyReport) != 0 {
		t.Errorf("apply/verify slots must never alias: %#v", cs.ArtifactPaths)
	}

	// Every checkbox complete: apply all_done, verify ready.
	seedDeriveChange(t, workspace, "lane-core", map[string]string{"plan.md": lanePlanDone})
	cs, err = readChange(changeRoot, "lane-core", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange (all done): %v", err)
	}
	if cs.ApplyState != ApplyAllDone || cs.Dependencies.Verify != DependencyReady || cs.NextRecommended != "verify" {
		t.Fatalf("all-done route = %q/%q/%q, want all_done/ready/verify", cs.ApplyState, cs.Dependencies.Verify, cs.NextRecommended)
	}

	// Passing report whose totals match the plan headings: verify all_done,
	// archive ready.
	seedDeriveChange(t, workspace, "lane-core", map[string]string{"verify-report.md": passingReport("1/1", "1/1")})
	cs, err = readChange(changeRoot, "lane-core", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange (archive): %v", err)
	}
	if cs.Dependencies.Verify != DependencyAllDone || cs.NextRecommended != "archive" || cs.Dependencies.Archive != DependencyReady {
		t.Fatalf("archive route = %q/%q/%q, want all_done/archive/ready", cs.Dependencies.Verify, cs.NextRecommended, cs.Dependencies.Archive)
	}
}

// TestLanePerSlotPrecedence is REQ-ALIAS scenario "Real artifact wins per
// slot": real design.md and tasks.md win their slots while proposal and
// specs keep resolving to the plan.
func TestLanePerSlotPrecedence(t *testing.T) {
	laneTestEnv(t)
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "lane-precedence", map[string]string{
		"plan.md":   "### Requirement: Alias\n#### Scenario: Alias\n- [ ] P1\n",
		"design.md": "# Design\n",
		"tasks.md":  "- [x] T1\n- [ ] T2\n",
	})
	cs, err := readChange(changeRoot, "lane-precedence", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	assertSlotPath(t, "Design", cs.ArtifactPaths.Design, filepath.Join(changeRoot, "design.md"))
	assertSlotPath(t, "Tasks", cs.ArtifactPaths.Tasks, filepath.Join(changeRoot, "tasks.md"))
	assertSlotPath(t, "Proposal", cs.ArtifactPaths.Proposal, filepath.Join(changeRoot, "plan.md"))
	assertSlotPath(t, "Specs", cs.ArtifactPaths.Specs, filepath.Join(changeRoot, "plan.md"))

	// Progress comes from the real tasks.md, not from the plan checklist.
	if want := (TaskProgress{Total: 2, Completed: 1, Pending: 1}); cs.TaskProgress != want {
		t.Errorf("TaskProgress = %#v, want %#v", cs.TaskProgress, want)
	}
	if cs.NextRecommended != "apply" {
		t.Errorf("NextRecommended = %q, want apply", cs.NextRecommended)
	}
}

// TestLaneInPlaceGraduation is REQ-ALIAS scenario "In-place graduation":
// writing tasks.md later graduates the slot with no migration.
func TestLaneInPlaceGraduation(t *testing.T) {
	laneTestEnv(t)
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "lane-graduate", map[string]string{"plan.md": lanePlan})

	cs, err := readChange(changeRoot, "lane-graduate", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	assertSlotPath(t, "Tasks", cs.ArtifactPaths.Tasks, filepath.Join(changeRoot, "plan.md"))

	seedDeriveChange(t, workspace, "lane-graduate", map[string]string{"tasks.md": "- [ ] T1\n"})
	cs, err = readChange(changeRoot, "lane-graduate", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange (graduated): %v", err)
	}
	assertSlotPath(t, "Tasks", cs.ArtifactPaths.Tasks, filepath.Join(changeRoot, "tasks.md"))
	assertSlotPath(t, "Proposal", cs.ArtifactPaths.Proposal, filepath.Join(changeRoot, "plan.md"))
	if cs.TaskProgress.Total != 1 {
		t.Errorf("TaskProgress.Total = %d, want 1 from the graduated tasks.md", cs.TaskProgress.Total)
	}
}

// TestLaneChecklistReadFromPlan is REQ-ALIAS scenario "Checklist read from
// plan": task progress counts the checkboxes of the plan file itself.
func TestLaneChecklistReadFromPlan(t *testing.T) {
	laneTestEnv(t)
	workspace := t.TempDir()
	plan := "### Requirement: R1\n#### Scenario: S1\n- [x] A\n- [x] B\n- [ ] C\n"
	changeRoot := seedDeriveChange(t, workspace, "lane-checklist", map[string]string{"plan.md": plan})
	cs, err := readChange(changeRoot, "lane-checklist", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if want := (TaskProgress{Total: 3, Completed: 2, Pending: 1}); cs.TaskProgress != want {
		t.Errorf("TaskProgress = %#v, want %#v", cs.TaskProgress, want)
	}
	if cs.ApplyState != ApplyReady || cs.NextRecommended != "apply" {
		t.Errorf("applyState/next = %q/%q, want ready/apply", cs.ApplyState, cs.NextRecommended)
	}
}

// TestLaneVerifyAdmissionCountsFromPlan is REQ-ALIAS scenario "Canonical
// headings admitted": the canonical ### Requirement: / #### Scenario:
// headings in the plan feed verify admission, which keeps rejecting totals
// that do not match.
func TestLaneVerifyAdmissionCountsFromPlan(t *testing.T) {
	tests := []struct {
		name       string
		plan       string
		report     string
		wantNext   string
		wantVerify DependencyState
		wantReason string
	}{
		{
			name:       "canonical headings match and archive",
			plan:       lanePlanDone,
			report:     passingReport("1/1", "1/1"),
			wantNext:   "archive",
			wantVerify: DependencyAllDone,
		},
		{
			name:       "inflated totals stay blocked",
			plan:       lanePlanDone,
			report:     passingReport("2/2", "2/2"),
			wantNext:   "remediate",
			wantVerify: DependencyBlocked,
			wantReason: "does not match actual requirement count",
		},
		{
			name:       "non-canonical headings count zero",
			plan:       "## Requirement: Fast Lane\n### Scenario: Plan-only\n- [x] T1\n",
			report:     passingReport("1/1", "1/1"),
			wantNext:   "remediate",
			wantVerify: DependencyBlocked,
			wantReason: "does not match actual requirement count",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			laneTestEnv(t)
			workspace := t.TempDir()
			changeRoot := seedDeriveChange(t, workspace, "lane-admission", map[string]string{
				"plan.md":          tt.plan,
				"verify-report.md": tt.report,
			})
			cs, err := readChange(changeRoot, "lane-admission", false, workspace, false)
			if err != nil {
				t.Fatalf("readChange: %v", err)
			}
			if cs.NextRecommended != tt.wantNext {
				t.Errorf("NextRecommended = %q, want %q", cs.NextRecommended, tt.wantNext)
			}
			if cs.Dependencies.Verify != tt.wantVerify {
				t.Errorf("Dependencies.Verify = %q, want %q", cs.Dependencies.Verify, tt.wantVerify)
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

// TestLaneCrossStoreParity is REQ-ALIAS scenario "Cross-store parity": the
// same plan-only change derived from filesystem and BigMem reports the same
// artifact set and apply readiness, with both resolvers applying the alias.
func TestLaneCrossStoreParity(t *testing.T) {
	laneTestEnv(t)
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "lane-parity", map[string]string{"plan.md": lanePlan})
	fs, err := readChange(changeRoot, "lane-parity", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}

	storeRoot := t.TempDir()
	SetBigMemStoreRootForTest(storeRoot)
	defer SetBigMemStoreRootForTest("")
	seedBigMemChange(t, storeRoot, "lane-parity", map[string]string{"plan": lanePlan})
	active, _, err := collectBigMemChangesWithArchive(workspace, false)
	if err != nil {
		t.Fatalf("collectBigMemChangesWithArchive: %v", err)
	}
	var mem *ChangeStatus
	for i := range active {
		if active[i].Name == "lane-parity" {
			mem = &active[i]
		}
	}
	if mem == nil {
		t.Fatalf("plan-only BigMem change not collected: %#v", active)
	}
	if !maps.Equal(fs.Artifacts, mem.Artifacts) {
		t.Errorf("artifact sets diverge: fs %#v vs bigmem %#v", fs.Artifacts, mem.Artifacts)
	}
	if fs.ApplyState != mem.ApplyState || fs.ApplyState != ApplyReady {
		t.Errorf("apply state diverges: fs %q vs bigmem %q, want ready", fs.ApplyState, mem.ApplyState)
	}
	if fs.NextRecommended != "apply" || mem.NextRecommended != "apply" {
		t.Errorf("next diverges: fs %q vs bigmem %q, want apply", fs.NextRecommended, mem.NextRecommended)
	}
	if fs.TaskProgress != mem.TaskProgress {
		t.Errorf("task progress diverges: fs %#v vs bigmem %#v", fs.TaskProgress, mem.TaskProgress)
	}
	planTopic := "bigmem:sdd/lane-parity/plan"
	assertSlotPath(t, "Proposal", mem.ArtifactPaths.Proposal, planTopic)
	assertSlotPath(t, "Specs", mem.ArtifactPaths.Specs, planTopic)
	assertSlotPath(t, "Design", mem.ArtifactPaths.Design, planTopic)
	assertSlotPath(t, "Tasks", mem.ArtifactPaths.Tasks, planTopic)
	if len(mem.ArtifactPaths.ApplyProgress) != 0 || len(mem.ArtifactPaths.VerifyReport) != 0 {
		t.Errorf("apply/verify BigMem slots must never alias: %#v", mem.ArtifactPaths)
	}
}

// TestLaneTerminalGatesNotRelaxed is REQ-ALIAS scenario "Gates not relaxed"
// plus the legacy-path regression: with RDD enabled, a lane change and a
// full four-artifact change both reach apply all_done and both are blocked
// by the same RDD receipt gate, and a scratch plan.md never changes the
// legacy resolution.
func TestLaneTerminalGatesNotRelaxed(t *testing.T) {
	enableRDDForTest(t)
	workspace := t.TempDir()
	fullRoot := seedDeriveChange(t, workspace, "full-gates", map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": specFixture,
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"plan.md":            lanePlan,
	})
	laneRoot := seedDeriveChange(t, workspace, "lane-gates", map[string]string{"plan.md": lanePlanDone})

	full, err := readChange(fullRoot, "full-gates", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange (full): %v", err)
	}
	lane, err := readChange(laneRoot, "lane-gates", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange (lane): %v", err)
	}

	for name, cs := range map[string]ChangeStatus{"full": full, "lane": lane} {
		if cs.ApplyState != ApplyAllDone {
			t.Errorf("%s applyState = %q, want all_done", name, cs.ApplyState)
		}
		if cs.Dependencies.Verify != DependencyBlocked {
			t.Errorf("%s Dependencies.Verify = %q, want blocked by the RDD gate", name, cs.Dependencies.Verify)
		}
		if prefix := rddGateReasonPrefix(cs.BlockedReasons); prefix == "" {
			t.Errorf("%s BlockedReasons = %#v, want an RDD receipt reason", name, cs.BlockedReasons)
		}
	}
	if fullPrefix, lanePrefix := rddGateReasonPrefix(full.BlockedReasons), rddGateReasonPrefix(lane.BlockedReasons); fullPrefix != lanePrefix {
		t.Errorf("gate reasons diverge: full %q vs lane %q", fullPrefix, lanePrefix)
	}

	// The scratch plan never disturbs a legacy four-artifact change.
	assertSlotPath(t, "Design", full.ArtifactPaths.Design, filepath.Join(fullRoot, "design.md"))
	assertSlotPath(t, "Tasks", full.ArtifactPaths.Tasks, filepath.Join(fullRoot, "tasks.md"))
	if want := (TaskProgress{Total: 1, Completed: 1, AllComplete: true}); full.TaskProgress != want {
		t.Errorf("full TaskProgress = %#v, want %#v", full.TaskProgress, want)
	}
}

// TestLaneEngramResolverToleratesBigMemPaths pins the engram branch of
// resolveArtifactPaths: firstPath keeps tolerating bigmem:sdd/{name}/plan
// topic paths (no filesystem IO, readText degrades to "").
func TestLaneEngramResolverToleratesBigMemPaths(t *testing.T) {
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "lane-engram", map[string]string{"plan.md": lanePlan})
	paths := resolveArtifactPaths(changeRoot, ArtifactStoreEngram)
	if got, want := firstPath(paths.Tasks), "bigmem:sdd/lane-engram/tasks"; got != want {
		t.Errorf("firstPath(Tasks) = %q, want %q", got, want)
	}
	if got := readText(firstPath(paths.Tasks)); got != "" {
		t.Errorf("readText(bigmem path) = %q, want empty (tolerated)", got)
	}
}
