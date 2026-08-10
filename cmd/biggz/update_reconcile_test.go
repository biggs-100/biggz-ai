package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
	"github.com/biggs-100/biggz-ai/plugintest"
)

func boolPtr(b bool) *bool { return &b }

func reconcileFakeAgent(installed bool) map[string]plugin.AgentAdapter {
	return map[string]plugin.AgentAdapter{
		"fake": &plugintest.FakeAgent{
			AgentID:           "fake",
			AgentName:         "Fake",
			Installed:         installed,
			BinaryPath:        "/usr/local/bin/fake",
			AgentSystemPrompt: boolPtr(true),
			AgentMCP:          boolPtr(true),
			AgentMCPStrategy:  model.StrategyMergeIntoSettings,
		},
	}
}

// TestPostUpdateReconcile_Success asserts the reconcile report line shape
// after a successful binary replacement.
func TestPostUpdateReconcile_Success(t *testing.T) {
	report := postUpdateReconcile(context.Background(), reconcileFakeAgent(true), t.TempDir(), false)
	wantParts := []string{"Reconciled:", "skills", "commands", "plugins", "prompts", "config merged", "MCP not deployed"}
	for _, part := range wantParts {
		if !strings.Contains(report, part) {
			t.Errorf("report %q missing %q", report, part)
		}
	}
}

// TestPostUpdateReconcile_NoReconcileSkipsDeployment asserts --no-reconcile
// skips the deployment entirely.
func TestPostUpdateReconcile_NoReconcileSkipsDeployment(t *testing.T) {
	home := t.TempDir()
	report := postUpdateReconcile(context.Background(), reconcileFakeAgent(true), home, true)
	if !strings.Contains(report, "Reconcile skipped") {
		t.Errorf("report = %q, want skip message", report)
	}
	// Nothing was deployed into the home directory.
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected no deployment with --no-reconcile, found %d entries", len(entries))
	}
}

// TestPostUpdateReconcile_FailureIsNonFatal asserts a reconcile failure is
// reported as a warning with the manual fallback instead of failing the
// update.
func TestPostUpdateReconcile_FailureIsNonFatal(t *testing.T) {
	report := postUpdateReconcile(context.Background(), reconcileFakeAgent(false), t.TempDir(), false)
	if !strings.Contains(report, "warning: reconcile failed") {
		t.Errorf("report = %q, want warning", report)
	}
	if !strings.Contains(report, "biggz sync --all") {
		t.Errorf("report = %q, want manual fallback hint", report)
	}
}

// TestUpdate_HelpNoReconcile verifies --no-reconcile appears in the update
// help (no network involved).
func TestUpdate_HelpNoReconcile(t *testing.T) {
	cmd := goRunBiggz(t, "update", "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("update --help exited with error: %v", err)
	}
	if !strings.Contains(stderr.String(), "--no-reconcile") {
		t.Errorf("expected update help to mention --no-reconcile, got: %s", stderr.String())
	}
}
