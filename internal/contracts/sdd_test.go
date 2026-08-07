package contracts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sdd"
)

// ---------------------------------------------------------------------------
// SDD envelope conformance (WU4)
// ---------------------------------------------------------------------------

// redirectTestHome points HOME (and USERPROFILE on Windows) at a temp dir so
// the machine-scoped ledger fallback stays inside test state.
func redirectTestHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// initGitRepo turns dir into a Git repository. Only the planning repository
// needs to be resolvable by git rev-parse; sibling service repositories only
// need a .git for root detection.
func initGitRepo(t *testing.T, dir string) {
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

// seedBlockedChange writes a change whose task plan targets a repository
// root outside the authorized edit roots, into a planning workspace.
func seedBlockedChange(t *testing.T, planning, name string) {
	t.Helper()
	changeRoot := filepath.Join(planning, "openspec", "changes", name)
	if err := os.MkdirAll(changeRoot, 0755); err != nil {
		t.Fatalf("mkdir change root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "proposal.md"), []byte("# Proposal\n"), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	tasks := "- [ ] 1.1 Update `../service-a/lib.go` for the rollout\n"
	if err := os.WriteFile(filepath.Join(changeRoot, "tasks.md"), []byte(tasks), 0644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
}

// TestEnvelopeConformance_EditAuthorityConsent drives the real SDD status
// path (internal/sdd readChange → newEditAuthorityConsent) and validates the
// emitted blocking consent envelope against edit-authority-consent.schema.json.
func TestEnvelopeConformance_EditAuthorityConsent(t *testing.T) {
	redirectTestHome(t)
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	initGitRepo(t, planning)
	initGitRepo(t, filepath.Join(workspace, "service-a"))
	seedBlockedChange(t, planning, "blocked-change")

	active, _, err := sdd.Status(filepath.Join(planning, "openspec"))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	var consent *sdd.EditAuthorityConsentResult
	for index := range active {
		if active[index].Name == "blocked-change" {
			consent = active[index].Consent
			break
		}
	}
	if consent == nil {
		t.Fatal("blocked change status carries no consent envelope")
	}
	if err := consent.Validate(); err != nil {
		t.Fatalf("consent envelope fails its own Validate: %v", err)
	}
	payload, err := json.Marshal(consent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(editAuthorityConsentID, payload); err != nil {
		t.Fatalf("edit-authority consent rejected by edit-authority-consent.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_VerifyAdmission drives the real verify-report
// admission path and validates the structured decision, admitted and denied.
func TestEnvelopeConformance_VerifyAdmission(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n\n## Verification Report\n\n**CRITICAL**: None\n"

	t.Run("admitted", func(t *testing.T) {
		admission := sdd.ValidateVerifyReportAdmission([]byte(report), 5, 10)
		if admission.Decision != "admitted" {
			t.Fatalf("decision = %q, want admitted (%s)", admission.Decision, admission.Reason)
		}
		payload, err := json.Marshal(admission)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := ValidateEnvelope(verifyAdmissionID, payload); err != nil {
			t.Fatalf("admission rejected by verify-admission.schema.json: %v", err)
		}
	})
	t.Run("denied", func(t *testing.T) {
		denied := strings.Replace(report, "requirements: 5/5", "requirements: 3/5", 1)
		admission := sdd.ValidateVerifyReportAdmission([]byte(denied), 5, 10)
		if admission.Decision != "denied" {
			t.Fatalf("decision = %q, want denied", admission.Decision)
		}
		payload, err := json.Marshal(admission)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := ValidateEnvelope(verifyAdmissionID, payload); err != nil {
			t.Fatalf("denied admission rejected by verify-admission.schema.json: %v", err)
		}
	})
}
