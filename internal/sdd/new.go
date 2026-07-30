package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewChangeParams defines parameters for creating a new SDD change.
type NewChangeParams struct {
	Root        string // project root (contains openspec/)
	Name        string // change name (kebab-case)
	Description string // short description
	ChangeType  string // "feature", "fix", "refactor", "docs", "other"
	Domain      string // spec domain, e.g. "api", "cli", "core"
}

// NewChangeResult describes the created change.
type NewChangeResult struct {
	ChangePath string `json:"change_path"`
	MetaPath   string `json:"meta_path"`
	ProposalPath string `json:"proposal_path,omitempty"`
}

// NewChange creates a new SDD change directory structure.
// Returns paths to the created files.
func NewChange(params NewChangeParams) (*NewChangeResult, error) {
	openspecRoot := filepath.Join(params.Root, "openspec")

	// Validate change name
	if params.Name == "" {
		return nil, fmt.Errorf("change name is required")
	}
	name := sanitizeChangeName(params.Name)
	if name != params.Name {
		return nil, fmt.Errorf("change name %q contains invalid characters; use lowercase kebab-case (e.g., 'add-dark-mode')", params.Name)
	}

	// Default values
	if params.ChangeType == "" {
		params.ChangeType = "feature"
	}
	if params.Domain == "" {
		params.Domain = "core"
	}

	// Create change directory
	changeDir := filepath.Join(openspecRoot, "changes", name)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		return nil, fmt.Errorf("create change dir: %w", err)
	}

	// Create specs subdirectory
	specsDir := filepath.Join(changeDir, "specs", params.Domain)
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		return nil, fmt.Errorf("create specs dir: %w", err)
	}

	// Write _meta.yaml
	metaPath := filepath.Join(changeDir, "_meta.yaml")
	metaContent := fmt.Sprintf(`# SDD Change: %s
name: %s
description: %s
type: %s
domain: %s
created_at: %s
status: active
---
`, name, name, params.Description, params.ChangeType, params.Domain, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(metaPath, []byte(metaContent), 0644); err != nil {
		return nil, fmt.Errorf("write meta: %w", err)
	}

	// Create state.yaml
	statePath := filepath.Join(changeDir, "state.yaml")
	stateContent := `# DAG state for SDD change
phases:
  propose: pending
  spec: pending
  design: pending
  tasks: pending
  apply: pending
  verify: pending
  archive: pending
`
	if err := os.WriteFile(statePath, []byte(stateContent), 0644); err != nil {
		return nil, fmt.Errorf("write state: %w", err)
	}

	// Create proposal.md stub
	proposalPath := filepath.Join(changeDir, "proposal.md")
	proposalContent := fmt.Sprintf(`# Proposal: %s

## Intent

%s

## Scope

<!-- Define what is in scope and out of scope -->

## Approach

<!-- Describe the proposed approach -->

## Success Criteria

<!-- Define how we know this is done -->

## Rollback Plan

<!-- How to revert if something goes wrong -->
`, name, params.Description)
	if err := os.WriteFile(proposalPath, []byte(proposalContent), 0644); err != nil {
		return nil, fmt.Errorf("write proposal: %w", err)
	}

	result := &NewChangeResult{
		ChangePath:   changeDir,
		MetaPath:     metaPath,
		ProposalPath: proposalPath,
	}

	return result, nil
}

// ListChanges returns all active (non-archived) changes.
func ListChanges(openspecRoot string) ([]string, error) {
	changesDir := filepath.Join(openspecRoot, "changes")
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var changes []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "archive" {
			// Verify it has _meta.yaml
			metaPath := filepath.Join(changesDir, e.Name(), "_meta.yaml")
			if _, err := os.Stat(metaPath); err == nil {
				changes = append(changes, e.Name())
			}
		}
	}
	return changes, nil
}

// sanitizeChangeName ensures the name is valid kebab-case.
func sanitizeChangeName(name string) string {
	var result strings.Builder
	for i, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else if r == ' ' || r == '_' {
			if i > 0 {
				result.WriteRune('-')
			}
		} else if r == '-' {
			result.WriteRune('-')
		}
	}
	return result.String()
}
