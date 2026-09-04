package sdd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatekeeper_AllChecksPass(t *testing.T) {
	// Setup: create a change with proposal.md
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest intent\n\n## Scope\n\nTest scope\n"), 0644)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Proposal created successfully",
		Artifacts: []ArtifactRef{
			{Path: "proposal.md", Type: "proposal", Summary: "Test proposal"},
		},
		NextRecommended: "spec",
	}

	gk := Gatekeeper(openspecRoot, "test-change", "explore", result)
	if !gk.Passed {
		t.Errorf("expected gatekeeper to pass, got reasons: %v", gk.Reasons)
		for _, d := range gk.Details {
			if !d.Passed {
				t.Errorf("  check %q failed: %s", d.Name, d.Reason)
			}
		}
	}
}

func TestGatekeeper_NilResult(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")

	gk := Gatekeeper(openspecRoot, "test-change", "explore", nil)
	if gk.Passed {
		t.Error("expected gatekeeper to fail with nil result")
	}
	if len(gk.Details) == 0 {
		t.Error("expected at least one detail")
	}
	if gk.Details[0].Name != "contract_conformance" {
		t.Errorf("expected contract_conformance check, got %q", gk.Details[0].Name)
	}
}

func TestGatekeeper_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")

	// Missing all fields
	result := &PhaseResult{}
	gk := Gatekeeper(openspecRoot, "test-change", "explore", result)
	if gk.Passed {
		t.Error("expected gatekeeper to fail with missing fields")
	}

	// Check that contract_conformance failed
	for _, d := range gk.Details {
		if d.Name == "contract_conformance" && d.Passed {
			t.Error("expected contract_conformance to fail")
		}
	}
}

func TestGatekeeper_ArtifactNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Test summary",
		Artifacts: []ArtifactRef{
			{Path: "nonexistent.md", Type: "proposal"},
		},
		NextRecommended: "spec",
	}

	gk := Gatekeeper(openspecRoot, "test-change", "propose", result)
	if gk.Passed {
		t.Error("expected gatekeeper to fail with missing artifact")
	}

	// Check artifact_existence failed
	found := false
	for _, d := range gk.Details {
		if d.Name == "artifact_existence" && !d.Passed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected artifact_existence check to fail")
	}
}

func TestGatekeeper_InvalidRouting(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest\n"), 0644)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Test summary",
		Artifacts: []ArtifactRef{
			{Path: "proposal.md", Type: "proposal"},
		},
		NextRecommended: "archive", // Invalid: can't go from propose to archive
	}

	gk := Gatekeeper(openspecRoot, "test-change", "propose", result)
	if gk.Passed {
		t.Error("expected gatekeeper to fail with invalid routing")
	}

	// Check routing_coherence failed
	found := false
	for _, d := range gk.Details {
		if d.Name == "routing_coherence" && !d.Passed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected routing_coherence check to fail")
	}
}

func TestGatekeeper_DriftDetection(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	// Create spec but NOT proposal (prerequisite for spec phase)
	os.WriteFile(filepath.Join(changeDir, "spec.md"), []byte("# Spec\n\n## Requirements\n\nTest\n"), 0644)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Spec created",
		Artifacts: []ArtifactRef{
			{Path: "spec.md", Type: "spec"},
		},
		NextRecommended: "design",
	}

	// spec phase requires propose to be done
	gk := Gatekeeper(openspecRoot, "test-change", "spec", result)
	if gk.Passed {
		t.Error("expected gatekeeper to fail due to drift (missing prerequisite)")
	}

	// Check no_drift failed
	found := false
	for _, d := range gk.Details {
		if d.Name == "no_drift" && !d.Passed {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected no_drift check to fail")
	}
}

func TestGatekeeper_ApplyCanLoop(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	// Create prerequisite artifacts for apply phase
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "spec.md"), []byte("# Spec\n\n## Requirements\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "design.md"), []byte("# Design\n\n## Architecture\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("# Tasks\n\n- [x] Task 1\n- [ ] Task 2\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "apply-progress.md"), []byte("# Apply Progress\n\n## Completed\n\nTask 1 done\n"), 0644)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Applied 1/2 tasks",
		Artifacts: []ArtifactRef{
			{Path: "apply-progress.md", Type: "apply-progress"},
		},
		NextRecommended: "apply (1/2 tasks)", // apply can loop
	}

	gk := Gatekeeper(openspecRoot, "test-change", "apply", result)
	if !gk.Passed {
		t.Errorf("expected gatekeeper to pass for apply loop, got: %v", gk.Reasons)
		for _, d := range gk.Details {
			if !d.Passed {
				t.Errorf("  check %q failed: %s", d.Name, d.Reason)
			}
		}
	}
}

func TestGatekeeper_VerifyCanRemediate(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	// Create prerequisite artifacts for verify phase
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "spec.md"), []byte("# Spec\n\n## Requirements\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "design.md"), []byte("# Design\n\n## Architecture\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("# Tasks\n\n- [x] Task 1\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "apply-progress.md"), []byte("# Apply Progress\n\nDone\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "verify-report.md"), []byte("# Verify Report\n\n## Verdict\n\nFAIL\n"), 0644)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Verification failed, remediation needed",
		Artifacts: []ArtifactRef{
			{Path: "verify-report.md", Type: "verify-report"},
		},
		NextRecommended: "apply", // verify can remediate back to apply
	}

	gk := Gatekeeper(openspecRoot, "test-change", "verify", result)
	if !gk.Passed {
		t.Errorf("expected gatekeeper to pass for verify remediation, got: %v", gk.Reasons)
		for _, d := range gk.Details {
			if !d.Passed {
				t.Errorf("  check %q failed: %s", d.Name, d.Reason)
			}
		}
	}
}

func TestParsePhaseResult_Valid(t *testing.T) {
	jsonStr := `{
		"status": "success",
		"executive_summary": "Test summary",
		"artifacts": [{"path": "proposal.md", "type": "proposal"}],
		"next_recommended": "spec"
	}`

	result, err := ParsePhaseResult(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
	if result.NextRecommended != "spec" {
		t.Errorf("expected next_recommended 'spec', got %q", result.NextRecommended)
	}
}

func TestParsePhaseResult_Invalid(t *testing.T) {
	_, err := ParsePhaseResult("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGatekeeperSummary_Pass(t *testing.T) {
	gr := &GatekeeperResult{
		Passed: true,
		Phase:  "spec",
	}
	summary := GatekeeperSummary(gr)
	if summary != "◆ spec · gatekeeper PASS" {
		t.Errorf("unexpected summary: %q", summary)
	}
}

func TestGatekeeperSummary_Fail(t *testing.T) {
	gr := &GatekeeperResult{
		Passed: false,
		Phase:  "apply",
		Details: []GatekeeperCheck{
			{Name: "artifact_existence", Passed: false},
			{Name: "routing_coherence", Passed: false},
		},
	}
	summary := GatekeeperSummary(gr)
	if summary != "◆ apply · gatekeeper FAIL (artifact_existence, routing_coherence)" {
		t.Errorf("unexpected summary: %q", summary)
	}
}
