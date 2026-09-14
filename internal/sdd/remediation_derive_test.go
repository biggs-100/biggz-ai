package sdd

import (
	"errors"
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

// TestRemediationCorrectiveAdmittedAfterGenerationAdvance is the generation
// boundary side of the remediation contract: the ledger advances to a
// successor work unit, the successor's verification fails, and the bounded
// correction is still admitted against the preserved chain. Once that
// correction passes, the same failed evidence can no longer be claimed.
func TestRemediationCorrectiveAdmittedAfterGenerationAdvance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
	workspace := t.TempDir()
	gitInitTemp(t, workspace)
	changeRoot := seedFailingVerifyChange(t, workspace, "remed-advance")

	// Generation 1: the apply work unit passes and completes its own scope.
	apply, err := sddattempt.Acquire(sddattempt.AcquireParams{
		ChangeName: "remed-advance", RepoRoot: workspace, RequestID: "remed-advance-apply",
		WorkUnit: "apply", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("acquire apply: %v", err)
	}
	if _, err := sddattempt.Settle(sddattempt.SettleParams{
		ChangeName: "remed-advance", RepoRoot: workspace, Token: apply.Token,
		RequestID: "remed-advance-apply-settle", Outcome: "passed",
		EvidenceRevision: "sha256:" + strings.Repeat("b", 64), Diagnosis: "apply gates passed",
	}); err != nil {
		t.Fatalf("settle apply passed: %v", err)
	}

	// The successor names the verify work unit: the generation advances.
	verify, err := sddattempt.Acquire(sddattempt.AcquireParams{
		ChangeName: "remed-advance", RepoRoot: workspace, RequestID: "remed-advance-verify",
		WorkUnit: "verify", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("acquire verify after a passed apply: %v", err)
	}
	if _, err := sddattempt.Settle(sddattempt.SettleParams{
		ChangeName: "remed-advance", RepoRoot: workspace, Token: verify.Token,
		RequestID: "remed-advance-verify-settle", Outcome: "failed",
		EvidenceRevision: failedEvidence, Diagnosis: "verification failed",
	}); err != nil {
		t.Fatalf("settle verify failed: %v", err)
	}

	// The corrective acquire declares exactly the failed evidence: admitted,
	// generation boundary included.
	corrective, err := sddattempt.Acquire(sddattempt.AcquireParams{
		ChangeName: "remed-advance", RepoRoot: workspace, RequestID: "remed-advance-corrective",
		WorkUnit: "verify", MaxAttempts: 3, MaxLines: 400,
		RemediatesEvidenceRevision: failedEvidence,
	})
	if err != nil {
		t.Fatalf("corrective acquire after a generation advance: %v", err)
	}
	if _, err := sddattempt.Settle(sddattempt.SettleParams{
		ChangeName: "remed-advance", RepoRoot: workspace, Token: corrective.Token,
		RequestID: "remed-advance-corrective-settle", Outcome: "passed",
		EvidenceRevision:           "sha256:" + strings.Repeat("c", 64),
		Diagnosis:                  "focused tests pass; rollback recorded",
		RemediatesEvidenceRevision: failedEvidence,
	}); err != nil {
		t.Fatalf("remediating settle after a generation advance: %v", err)
	}

	// The change derives the cleared remediation state across the boundary.
	cs, err := readChange(changeRoot, "remed-advance", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.RemediationState != (RemediationState{}) {
		t.Errorf("RemediationState = %#v, want zero value (cleared)", cs.RemediationState)
	}
	if cs.Dependencies.Verify != DependencyReady {
		t.Errorf("Dependencies.Verify = %q, want ready", cs.Dependencies.Verify)
	}
	if cs.NextRecommended != "verify" {
		t.Errorf("NextRecommended = %q, want verify", cs.NextRecommended)
	}

	// Already-remediated evidence can no longer be claimed: a successor
	// acquire naming it is refused instead of spending an attempt.
	_, err = sddattempt.Acquire(sddattempt.AcquireParams{
		ChangeName: "remed-advance", RepoRoot: workspace, RequestID: "remed-advance-restale",
		WorkUnit: "verify-followup", MaxAttempts: 3, MaxLines: 400,
		RemediatesEvidenceRevision: failedEvidence,
	})
	var blocked *sddattempt.BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != sddattempt.BlockedReasonInvalidContinuation {
		t.Fatalf("re-claimed remediated evidence = %v, want blocked(%s)", err, sddattempt.BlockedReasonInvalidContinuation)
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

// TestStaleDecisionCompletedLedgerRoutesUnstranded is the W2 covering test
// for the work_unit_complete arm of isStaleDecisionRequired: a legitimately
// complete work unit whose chain still carries an unremediated failure (a
// settle obligation) must free verify/archive from the unmanaged-remediation
// block instead of stranding the change, while a genuinely anomalous
// completion keeps the corrupt_authority classification.
func TestStaleDecisionCompletedLedgerRoutesUnstranded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
	workspace := t.TempDir()
	gitInitTemp(t, workspace)
	change := "stale-complete"
	changeRoot := seedFailingVerifyChange(t, workspace, change)

	// The begin/finish family completes the work unit while the chain keeps
	// the failed evidence open (finish carries no remediation gate, unlike
	// settle): complete + settle obligation is exactly the state the W2 arm
	// must route.
	begun, err := sddattempt.Begin(sddattempt.BeginParams{
		ChangeName: change, RepoRoot: workspace, WorkUnit: "verify",
		MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := sddattempt.Finish(sddattempt.FinishParams{
		ChangeName: change, RepoRoot: workspace, ExpectedRev: begun.Revision,
		Outcome: "failed", EvidenceRevision: failedEvidence, Diagnosis: "verification failed",
	}); err != nil {
		t.Fatalf("finish(failed): %v", err)
	}
	begun, err = sddattempt.Begin(sddattempt.BeginParams{
		ChangeName: change, RepoRoot: workspace, WorkUnit: "verify",
		MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("begin #2: %v", err)
	}
	if _, err := sddattempt.Finish(sddattempt.FinishParams{
		ChangeName: change, RepoRoot: workspace, ExpectedRev: begun.Revision,
		Outcome: "passed", EvidenceRevision: "sha256:" + strings.Repeat("b", 64),
		Diagnosis: "passing completion",
	}); err != nil {
		t.Fatalf("finish(passed): %v", err)
	}

	status, err := sddattempt.StatusWithInstance(change, workspace, "")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if !status.Complete || status.BlockedReason != sddattempt.BlockedReasonWorkUnitComplete {
		t.Fatalf("completed ledger probe = complete:%v reason:%q, want the legitimate completion",
			status.Complete, status.BlockedReason)
	}
	if status.SettleObligation == nil || status.SettleObligation.EvidenceRevision != failedEvidence {
		t.Fatalf("completed ledger obligation = %+v, want the preserved failed evidence", status.SettleObligation)
	}
	if !isStaleDecisionRequired(change, workspace, "") {
		t.Fatal("a complete work unit carrying a settle obligation must classify as a stale decision — verify/archive would be stranded")
	}

	// Routing: verify is freed from the unmanaged-remediation block and the
	// change continues through verify instead of being stranded in remediate.
	cs, err := readChange(changeRoot, change, false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.RemediationState != (RemediationState{}) {
		t.Fatalf("RemediationState = %#v, want zero value (the completion is not unmanaged remediation)", cs.RemediationState)
	}
	if cs.Dependencies.Verify != DependencyReady {
		t.Fatalf("Dependencies.Verify = %q, want ready (never stranded by the completed work unit)", cs.Dependencies.Verify)
	}
	if cs.NextRecommended != "verify" {
		t.Fatalf("NextRecommended = %q, want verify", cs.NextRecommended)
	}

	// A genuinely anomalous completion (complete + decision required) keeps
	// corrupt_authority: the extension must not reclassify real anomalies as
	// legitimate completions.
	store, err := sddattempt.LoadStore(change, workspace)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	anomaly := *store
	anomaly.DecisionRequired = true
	if err := sddattempt.SaveStore(&anomaly, workspace); err != nil {
		t.Fatalf("SaveStore anomalous completion: %v", err)
	}
	anomalyStatus, err := sddattempt.StatusWithInstance(change, workspace, "")
	if err != nil {
		t.Fatalf("StatusWithInstance (anomaly): %v", err)
	}
	if anomalyStatus.BlockedReason != sddattempt.BlockedReasonCorruptAuthority {
		t.Fatalf("anomalous completion reason = %q, want %q", anomalyStatus.BlockedReason, sddattempt.BlockedReasonCorruptAuthority)
	}
}
