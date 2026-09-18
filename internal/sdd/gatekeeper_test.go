package sdd

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"
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

	gk := Gatekeeper(openspecRoot, "test-change", "explore", ArtifactStoreOpenSpec, result)
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

	gk := Gatekeeper(openspecRoot, "test-change", "explore", ArtifactStoreOpenSpec, nil)
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
	gk := Gatekeeper(openspecRoot, "test-change", "explore", ArtifactStoreOpenSpec, result)
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

	gk := Gatekeeper(openspecRoot, "test-change", "propose", ArtifactStoreOpenSpec, result)
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

	gk := Gatekeeper(openspecRoot, "test-change", "propose", ArtifactStoreOpenSpec, result)
	if gk.Passed {
		t.Error("expected gatekeeper to fail with invalid routing")
	}

	// Check routing_coherence failed against the dependency authority, which
	// resolves spec from the proposal record, never against the retired table.
	routing := findCheck(gk, "routing_coherence")
	if routing == nil {
		t.Fatal("expected a routing_coherence detail")
	}
	if routing.Passed || routing.Skipped {
		t.Errorf("expected a failing routing_coherence, got %+v", routing)
	}
	if !strings.Contains(routing.Reason, "spec") {
		t.Errorf("routing_coherence reason %q must name the expected successor %q", routing.Reason, "spec")
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
	gk := Gatekeeper(openspecRoot, "test-change", "spec", ArtifactStoreOpenSpec, result)
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
	// Create prerequisite artifacts for apply phase. The delta spec must sit at
	// the canonical specs/<domain>/spec.md location so the dependency authority
	// derives apply; tasks below all-done let apply loop.
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest\n"), 0644)
	os.MkdirAll(filepath.Join(changeDir, "specs", "sdd"), 0755)
	os.WriteFile(filepath.Join(changeDir, "specs", "sdd", "spec.md"), []byte("# Spec\n\n## Requirements\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "design.md"), []byte("# Design\n\n## Architecture\n\nTest\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("# Tasks\n\n- [x] Task 1\n- [ ] Task 2\n"), 0644)
	os.WriteFile(filepath.Join(changeDir, "apply-progress.md"), []byte("# Apply Progress\n\n## Completed\n\nTask 1 done\n"), 0644)

	result := &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "Applied 1/2 tasks",
		Artifacts: []ArtifactRef{
			{Path: "apply-progress.md", Type: "apply-progress"},
		},
		NextRecommended: "apply (1/2 tasks)", // apply can loop while tasks remain
	}

	gk := Gatekeeper(openspecRoot, "test-change", "apply", ArtifactStoreOpenSpec, result)
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
	// Create prerequisite artifacts for verify phase. The delta spec must sit
	// at the canonical specs/<domain>/spec.md location so the dependency
	// authority reaches the remediation branch instead of re-planning.
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest\n"), 0644)
	os.MkdirAll(filepath.Join(changeDir, "specs", "sdd"), 0755)
	os.WriteFile(filepath.Join(changeDir, "specs", "sdd", "spec.md"), []byte("# Spec\n\n## Requirements\n\nTest\n"), 0644)
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
		NextRecommended: "remediate", // verify can remediate back through a bounded correction
	}

	gk := Gatekeeper(openspecRoot, "test-change", "verify", ArtifactStoreOpenSpec, result)
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

// TestGatekeeper_StoreAwareArtifactResolution pins the store-aware artifact
// contract (design D4): the canonical artifact for the completed phase
// decides pass/fail under the active store, declared paths resolve from the
// workspace root or the change dir but are never proof of existence, a
// missing canonical artifact names the absolute path it looked for, store
// "" skips the artifact check with an explicit reason while routing fails
// closed, and an unknown store fails closed.
func TestGatekeeper_StoreAwareArtifactResolution(t *testing.T) {
	tests := []struct {
		name             string
		store            ArtifactStore
		phase            string
		next             string
		setup            func(t *testing.T, changeDir string)
		declared         []ArtifactRef
		wantPass         bool
		wantArtifactSkip bool
		wantArtifactFail bool
		wantReasonPaths  []string
		wantReasonText   []string
	}{
		{
			name:  "repo-relative declaration resolves from the workspace root",
			store: ArtifactStoreOpenSpec,
			phase: "spec",
			next:  "design",
			setup: func(t *testing.T, changeDir string) {
				t.Helper()
				writeGatekeeperFixture(t, changeDir, "proposal.md")
				writeGatekeeperFixture(t, filepath.Join(changeDir, "specs", "sdd"), "spec.md")
			},
			declared: []ArtifactRef{{Path: "openspec/changes/test-change/specs/sdd/spec.md", Type: "spec"}},
			wantPass: true,
		},
		{
			name:  "change-relative declaration still resolves",
			store: ArtifactStoreOpenSpec,
			phase: "propose",
			next:  "spec",
			setup: func(t *testing.T, changeDir string) {
				t.Helper()
				writeGatekeeperFixture(t, changeDir, "proposal.md")
			},
			declared: []ArtifactRef{{Path: "proposal.md", Type: "proposal"}},
			wantPass: true,
		},
		{
			name:  "declared path is never proof of the canonical artifact",
			store: ArtifactStoreOpenSpec,
			phase: "spec",
			next:  "design",
			setup: func(t *testing.T, changeDir string) {
				t.Helper()
				writeGatekeeperFixture(t, changeDir, "proposal.md")
				// The declared path exists, but the delta spec at the canonical
				// location does not: the declaration must not save the check.
				writeGatekeeperFixture(t, changeDir, "spec.md")
			},
			declared:         []ArtifactRef{{Path: "spec.md", Type: "spec"}},
			wantArtifactFail: true,
			wantReasonPaths:  []string{"specs"},
		},
		{
			name:             "missing canonical artifact names the absolute path it looked for",
			store:            ArtifactStoreOpenSpec,
			phase:            "propose",
			next:             "spec",
			declared:         []ArtifactRef{{Path: "sdd/test-change/proposal", Type: "proposal"}},
			wantArtifactFail: true,
			wantReasonPaths:  []string{"proposal.md"},
		},
		{
			name:             "hybrid with only BigMem topics fails on the missing copy",
			store:            ArtifactStoreHybrid,
			phase:            "propose",
			next:             "spec",
			declared:         []ArtifactRef{{Path: "sdd/test-change/proposal", Type: "proposal"}},
			wantArtifactFail: true,
			wantReasonPaths:  []string{"proposal.md"},
		},
		{
			name:             "none store reports skip with a reason and routing fails closed",
			store:            "",
			phase:            "propose",
			next:             "spec",
			declared:         []ArtifactRef{{Path: "sdd/test-change/proposal", Type: "proposal"}},
			wantPass:         false,
			wantArtifactSkip: true,
			wantReasonText:   []string{"artifact store is none"},
		},
		{
			name:  "unknown store fails closed",
			store: ArtifactStore("carrier-pigeon"),
			phase: "propose",
			next:  "spec",
			setup: func(t *testing.T, changeDir string) {
				t.Helper()
				writeGatekeeperFixture(t, changeDir, "proposal.md")
			},
			declared:         []ArtifactRef{{Path: "proposal.md", Type: "proposal"}},
			wantArtifactFail: true,
			wantReasonText:   []string{"unknown artifact store"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			openspecRoot := filepath.Join(t.TempDir(), "openspec")
			changeDir := filepath.Join(openspecRoot, "changes", "test-change")
			if err := os.MkdirAll(changeDir, 0755); err != nil {
				t.Fatal(err)
			}
			if tt.setup != nil {
				tt.setup(t, changeDir)
			}
			result := &PhaseResult{
				Status:           "success",
				ExecutiveSummary: "phase done",
				Artifacts:        tt.declared,
				NextRecommended:  tt.next,
			}
			gk := Gatekeeper(openspecRoot, "test-change", tt.phase, tt.store, result)
			if gk.Passed != tt.wantPass {
				t.Fatalf("gatekeeper passed=%v, want %v; details: %+v", gk.Passed, tt.wantPass, gk.Details)
			}
			artifact := findCheck(gk, "artifact_existence")
			if artifact == nil {
				t.Fatal("expected artifact_existence detail")
			}
			if artifact.Skipped != tt.wantArtifactSkip {
				t.Errorf("artifact_existence skipped=%v, want %v (reason: %q)", artifact.Skipped, tt.wantArtifactSkip, artifact.Reason)
			}
			if tt.wantArtifactFail && (artifact.Passed || artifact.Skipped) {
				t.Errorf("expected a failing artifact_existence, got %+v", artifact)
			}
			if tt.wantPass && !tt.wantArtifactSkip && !artifact.Passed {
				t.Errorf("expected a passing artifact_existence, got %+v", artifact)
			}
			if tt.wantArtifactSkip && artifact.Passed {
				t.Errorf("a skipped artifact_existence must not be reported as passed, got %+v", artifact)
			}
			for _, path := range tt.wantReasonPaths {
				want := filepath.Join(changeDir, path)
				if !strings.Contains(artifact.Reason, want) {
					t.Errorf("artifact_existence reason %q must name the absolute path %q", artifact.Reason, want)
				}
			}
			for _, text := range tt.wantReasonText {
				if !strings.Contains(artifact.Reason, text) {
					t.Errorf("artifact_existence reason %q must contain %q", artifact.Reason, text)
				}
			}
			if h := findCheck(gk, "no_hallucination"); h == nil || !h.Passed {
				t.Errorf("declarations must resolve or be recognized as non-paths, got %+v", h)
			}
		})
	}
}

// TestGatekeeper_RoutingCoherence pins the routing contract against the
// dependency authority (design test matrix, spec scenarios S1-S6): the
// declared successor must equal the authority-resolved successor, terminal
// done/empty passes, labels normalize, phases outside the routing contract
// skip with a reason, and underivable state fails closed naming the cause.
func TestGatekeeper_RoutingCoherence(t *testing.T) {
	bigMemRoot := t.TempDir()
	SetBigMemStoreRootForTest(bigMemRoot)
	defer SetBigMemStoreRootForTest("")
	seedBigMemChange(t, bigMemRoot, "test-change", map[string]string{
		"proposal": "# Proposal\n\n## Intent\n\nBigMem proposal fixture\n",
	})

	writeProposal := func(t *testing.T, changeDir string) {
		t.Helper()
		writeGatekeeperFixture(t, changeDir, "proposal.md")
	}
	writePlanning := func(t *testing.T, changeDir string) {
		t.Helper()
		writeProposal(t, changeDir)
		writeGatekeeperFixture(t, filepath.Join(changeDir, "specs", "sdd"), "spec.md")
		writeGatekeeperFixture(t, changeDir, "design.md")
	}
	writeApply := func(t *testing.T, changeDir string) {
		t.Helper()
		writePlanning(t, changeDir)
		if err := os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("# Tasks\n\n- [x] Task 1\n- [ ] Task 2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		writeGatekeeperFixture(t, changeDir, "apply-progress.md")
	}
	withoutRecord := func(t *testing.T, changeDir string) {
		t.Helper()
		if err := os.RemoveAll(changeDir); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name            string
		change          string
		phase           string
		store           ArtifactStore
		next            string
		setup           func(t *testing.T, changeDir string)
		wantPass        bool
		wantRoutingPass bool
		wantRoutingSkip bool
		wantReasonText  []string
	}{
		{
			name:            "dependency-correct successor accepted (S1)",
			phase:           "spec",
			store:           ArtifactStoreOpenSpec,
			next:            "tasks",
			setup:           writePlanning,
			wantPass:        true,
			wantRoutingPass: true,
		},
		{
			name:           "retired static table rejected (S2)",
			phase:          "spec",
			store:          ArtifactStoreOpenSpec,
			next:           "design",
			setup:          writePlanning,
			wantPass:       false,
			wantReasonText: []string{`expected "tasks"`},
		},
		{
			name:           "genuinely incoherent successor rejected (S3)",
			phase:          "propose",
			store:          ArtifactStoreOpenSpec,
			next:           "archive",
			setup:          writeProposal,
			wantPass:       false,
			wantReasonText: []string{`expected "spec"`},
		},
		{
			name:            "done stays terminal (S4)",
			phase:           "spec",
			store:           ArtifactStoreOpenSpec,
			next:            "done",
			setup:           writePlanning,
			wantPass:        true,
			wantRoutingPass: true,
		},
		{
			name:  "empty stays terminal for routing (S4)",
			phase: "spec",
			store: ArtifactStoreOpenSpec,
			next:  "",
			setup: writePlanning,
			// The contract check rejects the empty next_recommended while
			// routing itself passes terminal.
			wantPass:        false,
			wantRoutingPass: true,
		},
		{
			name:           "underivable: no record names store and absolute change dir (S5)",
			phase:          "spec",
			store:          ArtifactStoreOpenSpec,
			next:           "design",
			setup:          withoutRecord,
			wantPass:       false,
			wantReasonText: []string{`no record under store "openspec": {changeDir}`},
		},
		{
			name:           "underivable: store none fails closed (S5)",
			phase:          "propose",
			store:          "",
			next:           "spec",
			setup:          writeProposal,
			wantPass:       false,
			wantReasonText: []string{`cannot resolve the dependency successor for change "test-change"`, "artifact store is none"},
		},
		{
			name:  "underivable: instance-marker read error wraps (S5)",
			phase: "propose",
			store: ArtifactStoreOpenSpec,
			next:  "spec",
			setup: func(t *testing.T, changeDir string) {
				t.Helper()
				writeProposal(t, changeDir)
				if err := os.MkdirAll(filepath.Join(changeDir, ".biggz-instance"), 0755); err != nil {
					t.Fatal(err)
				}
			},
			wantPass:       false,
			wantReasonText: []string{"read change-instance marker"},
		},
		{
			name:           "underivable: unknown store fails closed (D4)",
			phase:          "propose",
			store:          ArtifactStore("carrier-pigeon"),
			next:           "spec",
			setup:          writeProposal,
			wantPass:       false,
			wantReasonText: []string{`unknown artifact store "carrier-pigeon"`},
		},
		{
			name:           "underivable: hybrid without record or BigMem topics (D4)",
			change:         "empty-hybrid",
			phase:          "propose",
			store:          ArtifactStoreHybrid,
			next:           "spec",
			setup:          withoutRecord,
			wantPass:       false,
			wantReasonText: []string{`no record under store "hybrid": {changeDir}`, "no BigMem topics under sdd/empty-hybrid/"},
		},
		{
			name:            "normalized progress label matches (S6)",
			phase:           "apply",
			store:           ArtifactStoreOpenSpec,
			next:            "apply (3/5 tasks)",
			setup:           writeApply,
			wantPass:        true,
			wantRoutingPass: true,
		},
		{
			name:  "zero-artifact record stays derivable (D4)",
			phase: "explore",
			store: ArtifactStoreOpenSpec,
			next:  "propose",
			setup: func(t *testing.T, changeDir string) {
				t.Helper()
				writeGatekeeperFixture(t, changeDir, "state.yaml")
			},
			wantPass:        true,
			wantRoutingPass: true,
		},
		{
			name:            "phase outside the routing contract skips with a reason (D3)",
			phase:           "sync",
			store:           ArtifactStoreOpenSpec,
			next:            "apply",
			wantPass:        true,
			wantRoutingPass: true,
			wantRoutingSkip: true,
			wantReasonText:  []string{`phase "sync" is outside the routing contract; no dependency successor is defined`},
		},
		{
			name:            "hybrid resolves from BigMem when no record exists (D2)",
			phase:           "explore",
			store:           ArtifactStoreHybrid,
			next:            "spec",
			setup:           withoutRecord,
			wantPass:        true,
			wantRoutingPass: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			change := cmp.Or(tt.change, "test-change")
			openspecRoot := filepath.Join(t.TempDir(), "openspec")
			changeDir := filepath.Join(openspecRoot, "changes", change)
			if err := os.MkdirAll(changeDir, 0755); err != nil {
				t.Fatal(err)
			}
			if tt.setup != nil {
				tt.setup(t, changeDir)
			}
			result := &PhaseResult{
				Status:           "success",
				ExecutiveSummary: "phase done",
				Artifacts:        []ArtifactRef{{Path: "sdd/" + change + "/state", Type: "state"}},
				NextRecommended:  tt.next,
			}
			gk := Gatekeeper(openspecRoot, change, tt.phase, tt.store, result)

			routing := findCheck(gk, "routing_coherence")
			if routing == nil {
				t.Fatal("expected a routing_coherence detail")
			}
			if routing.Passed != tt.wantRoutingPass || routing.Skipped != tt.wantRoutingSkip {
				t.Errorf("routing_coherence passed=%v skipped=%v, want passed=%v skipped=%v (reason: %q)",
					routing.Passed, routing.Skipped, tt.wantRoutingPass, tt.wantRoutingSkip, routing.Reason)
			}
			for _, text := range tt.wantReasonText {
				text = strings.ReplaceAll(text, "{changeDir}", changeDir)
				if !strings.Contains(routing.Reason, text) {
					t.Errorf("routing_coherence reason %q must contain %q", routing.Reason, text)
				}
			}
			if gk.Passed != tt.wantPass {
				t.Errorf("gatekeeper passed=%v, want %v; details: %+v", gk.Passed, tt.wantPass, gk.Details)
			}
		})
	}
}

// writeGatekeeperFixture writes a non-trivial artifact fixture so existence
// and content-size checks agree on it.
func writeGatekeeperFixture(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n\nfixture body\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
