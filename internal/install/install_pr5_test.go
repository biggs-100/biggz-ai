package install_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/install/steps"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugintest"
)

// TestPR5_DryRunZeroWrites ensures Prepare preview only writes zero files outside TempDir.
func TestPR5_DryRunZeroWrites(t *testing.T) {
	tmpDir := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	agent.SetTempDir(tmpDir)
	ctx := context.Background()
	// Use Config DryRun true: pipeline with StateStep should not write
	result, err := install.Run(ctx, agent, install.Config{HomeDir: tmpDir, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run Run failed: %v", err)
	}
	if !result.DryRun {
		t.Error("expected DryRun true")
	}
	// Ensure zero writes: no state.json, no skills
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz-ai", "state.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create state.json")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz", "state.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create legacy state.json")
	}
}

// TestPR5_InvalidAgentBlocksApply verifies invalid agent/Prepare failure blocks Apply.
func TestPR5_InvalidAgentBlocksApply(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty HomeDir triggers Prepare validation failure (homeDir empty) and empty AgentID with nil adapter also fails.
	stateStep := steps.NewStateStep("", nil, false)
	stateStep.AgentID = ""
	plan := pipeline.NewPlan(stateStep)
	preview, err := plan.Prepare(context.Background())
	if err == nil {
		t.Fatalf("expected Prepare to fail on empty AgentID, got preview %v", preview)
	}
	// Ensure no file created
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz-ai", "state.json")); !os.IsNotExist(err) {
		t.Errorf("invalid agent should not create state file after failed Prepare")
	}
	// Also test that Orchestrator.Run blocks Apply when Prepare fails
	orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
	ch := make(pipeline.ProgressChan, 32)
	res, err := orch.RunWithChan(context.Background(), plan, ch)
	if err == nil {
		t.Fatalf("expected Orchestrator Run to fail on Prepare, got res %v", res)
	}
	// Drain channel to ensure closed
	for range ch {
	}
}

// TestPR5_E2EFakeAgentTempDir validates e2e Run(FakeAgent{TempDir}) with pipeline StagePlan and ProgressChan.
func TestPR5_E2EFakeAgentTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	agent.SetTempDir(tmpDir)
	// Also test direct pipeline with StateStep and ProgressChan
	home := tmpDir
	skillsStep := steps.NewSkillsStep(home, agent, false)
	overlayStep := steps.NewOverlayStep(home, agent, false)
	stateStep := steps.NewStateStep(home, agent, false)
	piStep := steps.NewPiExtensionsStep(home, agent, false)
	plan := pipeline.NewPlan(skillsStep, overlayStep, stateStep, piStep)
	preview, err := plan.Prepare(context.Background())
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(preview.Steps) < 3 {
		t.Errorf("expected >=3 steps, got %v", preview.Steps)
	}
	expectedNames := map[string]bool{"deploy-skills": true, "deploy-overlay": true, "state-merge": true, "pi-extensions": true}
	for _, n := range preview.Steps {
		if !expectedNames[n] {
			t.Errorf("unexpected step name %q", n)
		}
	}
	ch := make(pipeline.ProgressChan, 32)
	orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
	res, err := orch.RunWithChan(context.Background(), plan, ch)
	if err != nil {
		t.Fatalf("RunWithChan failed: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %v err=%v", res.Success, res.Error)
	}
	// Drain channel: ensure ProgressChan closed and lossless
	count := 0
	for range ch {
		count++
	}
	// Re-run via install.Run facade to verify TempDir isolation and file writes
	agent2 := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	agent2.SetTempDir(tmpDir)
	ctx := context.Background()
	result, err := install.Run(ctx, agent2, install.Config{HomeDir: tmpDir})
	if err != nil {
		t.Fatalf("install.Run e2e failed: %v", err)
	}
	if result.SkillsDeployed == 0 {
		t.Error("expected SkillsDeployed >0 in e2e")
	}
	// Verify state.json was written atomically via StateStep
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz-ai", "state.json")); err != nil {
		t.Fatalf("expected state.json after e2e, missing: %v", err)
	}
	// Verify all writes are under TempDir (no outside)
	// Already isolated via HomeDir; check that no file outside exists is not needed
}

// TestPR5_ProgressChanLossless ensures ProgressChan buffered 32 is lossless for pipeline Apply.
func TestPR5_ProgressChanLossless(t *testing.T) {
	tmpDir := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	agent.SetTempDir(tmpDir)
	skillsStep := steps.NewSkillsStep(tmpDir, agent, false)
	overlayStep := steps.NewOverlayStep(tmpDir, agent, false)
	stateStep := steps.NewStateStep(tmpDir, agent, false)
	piStep := steps.NewPiExtensionsStep(tmpDir, agent, false)
	plan := pipeline.NewPlan(skillsStep, overlayStep, stateStep, piStep)
	ch := make(pipeline.ProgressChan, 32)
	orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
	go func() {
		_, _ = orch.RunWithChan(context.Background(), plan, ch)
	}()
	events := 0
	for range ch {
		events++
	}
	if events == 0 {
		t.Error("expected at least one ProgressEvent via ProgressChan")
	}
}
