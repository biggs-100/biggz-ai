package review

import (
	"fmt"
	"os/exec"
	"strings"
)

// ─── Gate Diagnostics ────────────────────────────────────────────────────────

// GateDenialReason describes why a gate check failed.
type GateDenialReason struct {
	Stage   string `json:"stage"`   // e.g. "chain_integrity", "receipt_validation", "scope_change"
	Code    string `json:"code"`    // e.g. "chain_corrupt", "receipt_mismatch", "files_changed"
	Message string `json:"message"` // Human-readable explanation
}

// ScopeChangeDetail describes a detected scope change.
type ScopeChangeDetail struct {
	ChangedFiles []string `json:"changed_files,omitempty"`
	ExpectedTree string   `json:"expected_tree,omitempty"`
	ActualTree   string   `json:"actual_tree,omitempty"`
	LinesChanged int      `json:"lines_changed,omitempty"`
}

// BaseAdvanceEvidence describes a base branch advance that is compatible.
type BaseAdvanceEvidence struct {
	OldBase string `json:"old_base,omitempty"`
	NewBase string `json:"new_base,omitempty"`
	Commits int    `json:"commits,omitempty"`
}

// GateDiagnostics carries detailed diagnostic information for a gate result.
type GateDiagnostics struct {
	LineageID           string               `json:"lineage_id,omitempty"`
	Generation          int                  `json:"generation,omitempty"`
	DenialReasons       []GateDenialReason   `json:"denial_reasons,omitempty"`
	ScopeChange         *ScopeChangeDetail   `json:"scope_change,omitempty"`
	BaseAdvance         *BaseAdvanceEvidence `json:"base_advance,omitempty"`
	ChainValid          bool                 `json:"chain_valid"`
	ReceiptValid        bool                 `json:"receipt_valid"`
	HasBlockingFindings bool                 `json:"has_blocking_findings"`
}

// detectScopeChange runs git diff-tree to find changed files between trees.
func detectScopeChange(baseTree, candidateTree string) (*ScopeChangeDetail, error) {
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "-r", "--name-only", baseTree, candidateTree)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree: %w", err)
	}
	files := strings.Fields(string(out))
	if len(files) == 0 {
		return nil, nil
	}

	// Count changed lines
	statCmd := exec.Command("git", "diff", "--shortstat", baseTree, candidateTree)
	statOut, _ := statCmd.Output()
	lines := 0
	if _, err := fmt.Sscanf(string(statOut), "%d file changed, %d insertions", new(int), &lines); err != nil {
		// Try alternate format
		fmt.Sscanf(string(statOut), "%d files changed, %d insertions", new(int), &lines)
	}

	return &ScopeChangeDetail{
		ChangedFiles: files,
		ExpectedTree: baseTree,
		ActualTree:   candidateTree,
		LinesChanged: lines,
	}, nil
}

// detectBaseAdvance checks if the base branch has advanced.
func detectBaseAdvance(oldBase, upstreamRef string) *BaseAdvanceEvidence {
	cmd := exec.Command("git", "rev-parse", upstreamRef)
	newBase, err := cmd.Output()
	if err != nil {
		return nil
	}
	newBaseStr := strings.TrimSpace(string(newBase))
	if newBaseStr == oldBase {
		return nil
	}

	// Count new commits
	logCmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", oldBase, newBaseStr))
	logOut, _ := logCmd.Output()
	commits := 0
	fmt.Sscanf(string(logOut), "%d", &commits)

	return &BaseAdvanceEvidence{
		OldBase: oldBase,
		NewBase: newBaseStr,
		Commits: commits,
	}
}

// EnhancedGateResult extends GateResult with structured diagnostics.
type EnhancedGateResult struct {
	Gate        GateKind            `json:"gate"`
	Passed      bool                `json:"passed"`
	DryRun      bool                `json:"dry_run"`
	Disposition DeliveryDisposition `json:"disposition"`
	Diagnostics *GateDiagnostics    `json:"diagnostics,omitempty"`
	Reasons     []string            `json:"reasons,omitempty"`
}
