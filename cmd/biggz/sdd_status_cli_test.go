package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runSDDStatusCLIArgs invokes sddStatusRun in-process with explicit flags,
// capturing stdout and stderr through temp files.
func runSDDStatusCLIArgs(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs := os.Args
	os.Args = append([]string{"biggz", "sdd-status"}, args...)
	defer func() { os.Args = oldArgs }()

	outFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("create stdout capture: %v", err)
	}
	errFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("create stderr capture: %v", err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	code = sddStatusRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

// statusEnvelope mirrors the emitted {active, archived, review_disabled}
// envelope with the fields the CLI test asserts.
type statusEnvelope struct {
	Active   []statusChange `json:"active"`
	Archived []statusChange `json:"archived"`
}

type statusChange struct {
	Name        string `json:"Name"`
	HasProposal bool   `json:"HasProposal"`
	HasSpecs    bool   `json:"HasSpecs"`
	HasDesign   bool   `json:"HasDesign"`
	HasTasks    bool   `json:"HasTasks"`
	TasksTotal  int    `json:"TasksTotal"`
	TasksDone   int    `json:"TasksDone"`
	HasVerify   bool   `json:"HasVerify"`
	IsArchived  bool   `json:"IsArchived"`

	NextRecommended string            `json:"nextRecommended"`
	BlockedReasons  []string          `json:"blockedReasons"`
	ApplyState      string            `json:"applyState"`
	Dependencies    map[string]string `json:"dependencies"`
	TaskProgress    struct {
		Total       int  `json:"total"`
		Completed   int  `json:"completed"`
		Pending     int  `json:"pending"`
		AllComplete bool `json:"allComplete"`
	} `json:"taskProgress"`
	ActionContext struct {
		Mode             string   `json:"mode"`
		WorkspaceRoot    string   `json:"workspaceRoot"`
		AllowedEditRoots []string `json:"allowedEditRoots"`
	} `json:"actionContext"`
	PhaseInstructions map[string][]string `json:"phaseInstructions"`
}

// seedCompleteChange writes a change whose derivation reaches archive
// readiness: all planning artifacts done, every task checkbox complete, and
// a passing verify report whose counts match the spec.
func seedCompleteChange(t *testing.T, planning, name string) {
	t.Helper()
	changeRoot := filepath.Join(planning, "openspec", "changes", name)
	files := map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": "### Requirement: Capability\n#### Scenario: Works\n",
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"verify-report.md": "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\n" +
			"requirements: 1/1\nscenarios: 1/1\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n",
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
}

// TestSDDStatusJSONEnvelopeDerivesStructuredFields is T4: --json keeps the
// legacy file-probe keys and adds the derived structured fields
// (nextRecommended, blockedReasons, applyState, dependencies,
// taskProgress.allComplete, actionContext.allowedEditRoots).
func TestSDDStatusJSONEnvelopeDerivesStructuredFields(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	seedCompleteChange(t, planning, "json-change")

	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", planning, "--json")
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %q)", code, stderr)
	}
	var envelope statusEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("parse envelope: %v\n%s", err, stdout)
	}
	if len(envelope.Active) != 1 {
		t.Fatalf("active changes = %d, want 1", len(envelope.Active))
	}
	cs := envelope.Active[0]

	// Legacy keys stay present (PascalCase wire names, read-compat).
	if !cs.HasProposal || !cs.HasSpecs || !cs.HasDesign || !cs.HasTasks || !cs.HasVerify {
		t.Fatalf("legacy booleans not all true: %#v", cs)
	}
	if cs.TasksTotal != 1 || cs.TasksDone != 1 {
		t.Fatalf("legacy task counters = %d/%d, want 1/1", cs.TasksDone, cs.TasksTotal)
	}

	// Derived keys.
	if cs.NextRecommended != "archive" {
		t.Errorf("nextRecommended = %q, want archive", cs.NextRecommended)
	}
	if cs.ApplyState != "all_done" {
		t.Errorf("applyState = %q, want all_done", cs.ApplyState)
	}
	if len(cs.BlockedReasons) != 0 {
		t.Errorf("blockedReasons = %#v, want empty", cs.BlockedReasons)
	}
	if cs.Dependencies["verify"] != "all_done" || cs.Dependencies["archive"] != "ready" || cs.Dependencies["apply"] != "all_done" {
		t.Errorf("dependencies = %#v, want verify/apply all_done and archive ready", cs.Dependencies)
	}
	if !cs.TaskProgress.AllComplete || cs.TaskProgress.Total != 1 || cs.TaskProgress.Completed != 1 {
		t.Errorf("taskProgress = %#v, want 1/1 all complete", cs.TaskProgress)
	}
	if cs.ActionContext.Mode != "repo-local" || cs.ActionContext.WorkspaceRoot != planning {
		t.Errorf("actionContext = %#v", cs.ActionContext)
	}
	allowed := strings.Join(cs.ActionContext.AllowedEditRoots, ",")
	if !strings.Contains(allowed, planning) {
		t.Errorf("allowedEditRoots = %#v, want containing %q", cs.ActionContext.AllowedEditRoots, planning)
	}
}

// TestSDDStatusInstructionsAddsPhaseInstructions is T4's --instructions half:
// the phaseInstructions block appears with exactly the four phase keys, each
// an instruction list.
func TestSDDStatusInstructionsAddsPhaseInstructions(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	seedCompleteChange(t, planning, "instr-change")

	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", planning, "--json", "--instructions")
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %q)", code, stderr)
	}
	var envelope statusEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("parse envelope: %v\n%s", err, stdout)
	}
	if len(envelope.Active) != 1 {
		t.Fatalf("active changes = %d, want 1", len(envelope.Active))
	}
	instructions := envelope.Active[0].PhaseInstructions
	if instructions == nil {
		t.Fatal("phaseInstructions missing with --instructions")
	}
	if len(instructions) != 4 {
		t.Fatalf("phaseInstructions has %d keys, want exactly 4: %#v", len(instructions), instructions)
	}
	for _, phase := range []string{"apply", "verify", "remediate", "archive"} {
		list, ok := instructions[phase]
		if !ok || len(list) == 0 {
			t.Errorf("phaseInstructions[%q] = %#v, want a non-empty instruction list", phase, list)
		}
	}
	joined := strings.Join(instructions["apply"], "\n")
	if !strings.Contains(joined, "biggz sdd-attempt begin") || !strings.Contains(joined, "biggz sdd-attempt finish") {
		t.Errorf("apply instructions lack the runtime begin/finish block: %s", joined)
	}
	remediated := strings.Join(instructions["remediate"], "\n")
	if !strings.Contains(remediated, "--remediates-evidence-revision") {
		t.Errorf("remediate instructions lack --remediates-evidence-revision: %s", remediated)
	}
}

// TestSDDStatusArchivedReportsDone is T4's archived half: an archived change
// reports nextRecommended "done".
func TestSDDStatusArchivedReportsDone(t *testing.T) {
	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	if err := os.MkdirAll(planning, 0755); err != nil {
		t.Fatalf("mkdir planning: %v", err)
	}
	seedCompleteChange(t, planning, "archived-change")

	archiveDir := filepath.Join(planning, "openspec", "changes", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}
	if err := os.Rename(filepath.Join(planning, "openspec", "changes", "archived-change"), filepath.Join(archiveDir, "archived-change")); err != nil {
		t.Fatalf("move into archive: %v", err)
	}

	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", planning, "--json")
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %q)", code, stderr)
	}
	var envelope statusEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("parse envelope: %v\n%s", err, stdout)
	}
	if len(envelope.Active) != 0 {
		t.Fatalf("active changes = %d, want 0", len(envelope.Active))
	}
	if len(envelope.Archived) != 1 {
		t.Fatalf("archived changes = %d, want 1", len(envelope.Archived))
	}
	if !envelope.Archived[0].IsArchived {
		t.Fatal("archived entry lacks IsArchived")
	}
	if envelope.Archived[0].NextRecommended != "done" {
		t.Errorf("archived nextRecommended = %q, want done", envelope.Archived[0].NextRecommended)
	}
}
