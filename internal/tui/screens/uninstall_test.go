package screens

import (
	"errors"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/uninstall"
)

func TestUninstallReportSuccess(t *testing.T) {
	res := &uninstall.Result{
		AgentResults: []uninstall.AgentResult{
			{AgentID: "opencode", Name: "OpenCode", RemovedFiles: 42, RewrittenConfigs: 2},
			{AgentID: "qwen", Name: "Qwen", RemovedFiles: 0, RewrittenConfigs: 0},
		},
		RemovedFiles:     42,
		RewrittenConfigs: 2,
		Summary:          "1 agents uninstalled, 0 failed, kept: bigmem, backups, custom-agents",
	}

	status, reportErr := uninstallReport(res)
	if reportErr != "" {
		t.Fatalf("expected no error report, got %q", reportErr)
	}
	for _, want := range []string{
		"opencode: removed 42 files, 2 configs rewritten",
		"1 agents uninstalled, 0 failed",
		"Kept: BigMem memory data",
		"biggz uninstall --purge",
	} {
		if !strings.Contains(status, want) {
			t.Errorf("status missing %q:\n%s", want, status)
		}
	}
	if strings.Contains(status, "qwen: removed") {
		t.Errorf("status must skip zero-result agents:\n%s", status)
	}
}

func TestUninstallReportNothingToRemove(t *testing.T) {
	res := &uninstall.Result{Summary: "0 agents uninstalled, 0 failed, kept: bigmem, backups, custom-agents"}
	status, reportErr := uninstallReport(res)
	if reportErr != "" {
		t.Fatalf("expected no error report, got %q", reportErr)
	}
	if !strings.Contains(status, "nothing to remove") {
		t.Errorf("expected 'nothing to remove', got:\n%s", status)
	}
}

func TestUninstallReportPartialFailure(t *testing.T) {
	res := &uninstall.Result{
		AgentResults: []uninstall.AgentResult{
			{AgentID: "opencode", Name: "OpenCode", RemovedFiles: 12, RewrittenConfigs: 1},
		},
		RemovedFiles:     12,
		RewrittenConfigs: 1,
		Failed: []uninstall.AgentFailure{
			{Agent: "claude", Op: "remove skill sdd-init/SKILL.md", Err: errors.New("directory not empty")},
		},
		Summary: "1 agents uninstalled, 1 failed, kept: bigmem, backups, custom-agents",
	}

	status, reportErr := uninstallReport(res)
	if !strings.Contains(status, "opencode: removed 12 files") {
		t.Errorf("status must keep successful agents:\n%s", status)
	}
	for _, want := range []string{
		"claude: FAILED remove skill sdd-init/SKILL.md: directory not empty",
		"Press [R] to retry",
	} {
		if !strings.Contains(reportErr, want) {
			t.Errorf("error report missing %q:\n%s", want, reportErr)
		}
	}
}
