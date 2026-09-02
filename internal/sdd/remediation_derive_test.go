package sdd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/sddattempt"
)

// gitInitTemp creates a git repository at dir, skipping the test when git is
// unavailable.
func gitInitTemp(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, out)
	}
}

// failingEvidenceReport builds a failing verify report bound to the given
// evidence revision, with counts matching the one-requirement/one-scenario
// spec fixture.
func failingEvidenceReport(evidenceRevision string) string {
	return "```yaml\nschema: biggz-ai.verify-result/v1\nevidence_revision: " + evidenceRevision +
		"\nverdict: fail\nblockers: 0\ncritical_findings: 0\nrequirements: 1/1\nscenarios: 1/1\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n"
}

var failedEvidence = "sha256:" + strings.Repeat("a", 64)

// seedFailingVerifyChange writes a change whose apply is all done and whose
// verify report fails against evidence revision failedEvidence.
func seedFailingVerifyChange(t *testing.T, workspace, change string) string {
	t.Helper()
	changeRoot := seedDeriveChange(t, workspace, change, map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": specFixture,
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"verify-report.md":   failingEvidenceReport(failedEvidence),
	})
	return changeRoot
}

// recordPassedRemediation runs a real begin → finish cycle on the ledger
// whose passed attempt declares --remediates-evidence-revision
// remediatesEvidence.
func recordPassedRemediation(t *testing.T, workspace, change, remediatesEvidence string) {
	t.Helper()
	begun, err := sddattempt.Begin(sddattempt.BeginParams{
		ChangeName:  change,
		RepoRoot:    workspace,
		ObjectiveID: "verify",
		WorkUnit:    "fix-evidence",
		MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := sddattempt.Finish(sddattempt.FinishParams{
		ChangeName:                 change,
		RepoRoot:                   workspace,
		ExpectedRev:                begun.Revision,
		Outcome:                    "passed",
		EvidenceRevision:           "sha256:" + strings.Repeat("b", 64),
		Diagnosis:                  "focused tests pass; rollback recorded",
		RemediatesEvidenceRevision: remediatesEvidence,
	}); err != nil {
		t.Fatalf("finish: %v", err)
	}
}

// TestRemediationCompleteLedgerMatching is T5: a passed ledger attempt whose
// --remediates-evidence-revision matches the failed evidence revision clears
// the remediation state and routes verify → ready, next → verify.
func TestRemediationCompleteLedgerMatching(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
	workspace := t.TempDir()
	gitInitTemp(t, workspace)
	changeRoot := seedFailingVerifyChange(t, workspace, "remed-complete")
	recordPassedRemediation(t, workspace, "remed-complete", failedEvidence)

	cs, err := readChange(changeRoot, "remed-complete", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.RemediationState != (RemediationState{}) {
		t.Errorf("RemediationState = %#v, want zero value (cleared)", cs.RemediationState)
	}
	if cs.Dependencies.Verify != DependencyReady {
		t.Errorf("Dependencies.Verify = %q, want ready", cs.Dependencies.Verify)
	}
	if cs.Dependencies.Archive != DependencyBlocked {
		t.Errorf("Dependencies.Archive = %q, want blocked", cs.Dependencies.Archive)
	}
	if cs.NextRecommended != "verify" {
		t.Errorf("NextRecommended = %q, want verify", cs.NextRecommended)
	}
}

// TestRemediationIncompleteLedgerNonMatching is T5's other side: a passed
// ledger attempt that corrected different evidence does not clear the
// remediation state, which persists with the failed revision and routes
// next → remediate.
func TestRemediationIncompleteLedgerNonMatching(t *testing.T) {
	workspace := t.TempDir()
	gitInitTemp(t, workspace)
	changeRoot := seedFailingVerifyChange(t, workspace, "remed-persists")
	recordPassedRemediation(t, workspace, "remed-persists", "sha256:"+strings.Repeat("c", 64))

	cs, err := readChange(changeRoot, "remed-persists", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if !cs.RemediationState.Required {
		t.Fatal("RemediationState.Required = false, want true for non-matching ledger")
	}
	if cs.RemediationState.FailedEvidenceRevision != failedEvidence {
		t.Errorf("FailedEvidenceRevision = %q, want %q", cs.RemediationState.FailedEvidenceRevision, failedEvidence)
	}
	if cs.Dependencies.Verify != DependencyBlocked {
		t.Errorf("Dependencies.Verify = %q, want blocked", cs.Dependencies.Verify)
	}
	if cs.NextRecommended != "remediate" {
		t.Errorf("NextRecommended = %q, want remediate", cs.NextRecommended)
	}
	if !strings.Contains(strings.Join(cs.BlockedReasons, "\n"), "verify evidence requires unmanaged remediation") {
		t.Errorf("BlockedReasons = %#v, want the unmanaged remediation reason", cs.BlockedReasons)
	}
}

// TestRemediationRequiredBeforeAnyLedger asserts the base state: with no
// ledger record at all, a failing current verify report requires
// remediation.
func TestRemediationRequiredBeforeAnyLedger(t *testing.T) {
	workspace := t.TempDir()
	changeRoot := seedFailingVerifyChange(t, workspace, "remed-fresh")
	cs, err := readChange(changeRoot, "remed-fresh", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if !cs.RemediationState.Required {
		t.Fatal("RemediationState.Required = false, want true without any ledger record")
	}
	if cs.RemediationState.FailedEvidenceRevision != failedEvidence {
		t.Errorf("FailedEvidenceRevision = %q, want %q", cs.RemediationState.FailedEvidenceRevision, failedEvidence)
	}
	if cs.NextRecommended != "remediate" {
		t.Errorf("NextRecommended = %q, want remediate", cs.NextRecommended)
	}
	if cs.ActionContext.WorkspaceRoot != workspace {
		t.Errorf("ActionContext.WorkspaceRoot = %q, want %q", cs.ActionContext.WorkspaceRoot, workspace)
	}
}
