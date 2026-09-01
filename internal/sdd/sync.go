package sdd

import (
	"os"
	"path/filepath"
	"strings"
)

// SyncResult describes the outcome of a sync operation.
type SyncResult string

const (
	SyncApplied       SyncResult = "applied"
	SyncNotApplicable SyncResult = "not-applicable"
	SyncBlocked       SyncResult = "blocked"
)

// Sync synchronizes delta specs from openspec/changes/{change}/specs/ to openspec/specs/
// without archiving. It respects store gate, verify PASS, guardrails, and allowedEditRoots.
// It returns the result, a human message, and error for I/O failures.
func Sync(change, workspaceRoot, promptText string) (SyncResult, string, error) {
	store, res, msg := syncCheckStoreGate(workspaceRoot)
	if res != "" {
		return res, msg, nil
	}
	changeRoot, res, msg, err := syncResolveChangeRoot(change, workspaceRoot)
	if err != nil {
		return "", "", err
	}
	if res != "" {
		return res, msg, nil
	}
	if res, msg, err := syncVerifyMustPass(changeRoot, store); err != nil {
		return "", "", err
	} else if res != "" {
		return res, msg, nil
	}
	promptLower := strings.ToLower(promptText)
	tasksContent := readTasksContent(changeRoot)
	hasResolveViaEngram := strings.Contains(promptLower, "resolve-via-engram") || strings.Contains(strings.ToLower(tasksContent), "resolve-via-engram")
	infos, res, msg, err := syncParseDomainInfos(changeRoot)
	if err != nil {
		return "", "", err
	}
	if res != "" {
		return res, msg, nil
	}
	if res, msg := syncGuardRenamed(infos, hasResolveViaEngram); res != "" {
		return res, msg, nil
	}
	guardrails := syncCollectGuardrails(infos, workspaceRoot, change, promptLower, hasResolveViaEngram)
	if res, msg := syncBlockOnGuardrails(guardrails, promptLower, hasResolveViaEngram); res != "" {
		return res, msg, nil
	}
	if res, msg, err := syncApplyDeltas(infos, workspaceRoot); err != nil {
		return "", "", err
	} else if res != "" {
		return res, msg, nil
	}
	if err := syncEnsureNotDisappeared(changeRoot); err != nil {
		return "", "", err
	}
	return SyncApplied, "sync applied for change " + change, nil
}

func readTasksContent(changeRoot string) string {
	data, err := os.ReadFile(filepath.Join(changeRoot, "tasks.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// isSyncNeeded checks if main specs already reflect deltas.
func isSyncNeeded(change, workspaceRoot string) bool {
	changeRoot := filepath.Join(workspaceRoot, "openspec", "changes", change)
	files := findSpecFiles(filepath.Join(changeRoot, "specs"))
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		contentBytes, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		pr, err := ParseDeltaSpec(string(contentBytes))
		if err != nil {
			continue
		}
		if pr.HasRenamed {
			return true
		}
		domain := filepath.Base(filepath.Dir(f))
		mainPath := filepath.Join(workspaceRoot, "openspec", "specs", domain, "spec.md")
		mainBytes, err := os.ReadFile(mainPath)
		mainContent := ""
		if err == nil {
			mainContent = string(mainBytes)
		}
		applied, err := ApplyDeltas(mainContent, pr.Deltas)
		if err != nil {
			return true
		}
		if strings.TrimSpace(applied) != strings.TrimSpace(mainContent) {
			return true
		}
	}
	return false
}
