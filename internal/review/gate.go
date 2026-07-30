package review

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/model"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Delivery Disposition
// ---------------------------------------------------------------------------

// DeliveryDisposition describes how gate evaluation handles a delivery
// based on RDD state and receipt presence.
type DeliveryDisposition int

const (
	DispositionReceiptGoverned   DeliveryDisposition = iota
	DispositionDisabledUnmanaged
	DispositionUnmanaged
)

// ResolveDeliveryDisposition resolves the delivery disposition for the given
// git directory by checking RDD status. RDD-disabled repos return
// DispositionDisabledUnmanaged; otherwise the disposition is governed by the
// review receipt.
func ResolveDeliveryDisposition(gitDir string) DeliveryDisposition {
	// For gate resolution, pass the same dir for both — if it's a linked
	// worktree, we still need to check worktree-level overrides.
	status, err := RDDStatus(gitDir, gitDir)
	if err != nil {
		return DispositionUnmanaged
	}
	if status.EffectiveMode == RDDModeDisabled {
		return DispositionDisabledUnmanaged
	}
	return DispositionReceiptGoverned
}

// ---------------------------------------------------------------------------
// Gate Types
// ---------------------------------------------------------------------------

// GateKind represents a type of publication gate.
type GateKind string

const (
	// New gate kinds (publication gates).
	GatePrePR   GateKind = "pre-pr"
	GatePrePush GateKind = "pre-push"

	// Legacy gate kinds (deprecated, kept for backward compat).
	GatePreCommit GateKind = "pre-commit"
	GateRelease   GateKind = "release"
)

// GateResult describes the outcome of a gate validation.
type GateResult struct {
	Gate    GateKind `json:"gate"`
	Passed  bool     `json:"passed"`
	Reasons []string `json:"reasons,omitempty"`
	DryRun  bool     `json:"dry_run"`
}

// ---------------------------------------------------------------------------
// Gate Configuration
// ---------------------------------------------------------------------------

// gateConfigYAML is the on-disk structure for .biggz/config.yaml.
type gateConfigYAML struct {
	Gate struct {
		PrePR  struct{ Enabled bool `yaml:"enabled"` }  `yaml:"pre-pr"`
		PrePush struct{ Enabled bool `yaml:"enabled"` } `yaml:"pre-push"`
	} `yaml:"gate"`
}

// GateConfig defines per-repo gate opt-out settings.
type GateConfig struct {
	enabled map[GateKind]bool
}

// DefaultGateConfig returns a config with all gates enabled.
func DefaultGateConfig() *GateConfig {
	return &GateConfig{
		enabled: map[GateKind]bool{
			GatePrePR:   true,
			GatePrePush: true,
		},
	}
}

// IsEnabled checks whether the given gate kind is enabled.
func (c *GateConfig) IsEnabled(kind GateKind) bool {
	if c == nil || c.enabled == nil {
		return true
	}
	return c.enabled[kind]
}

// LoadGateConfig loads gate configuration from .biggz/config.yaml at the
// repository root. If the file does not exist or cannot be read, returns
// a default config with all gates enabled.
func LoadGateConfig(repo string) *GateConfig {
	gitDir, err := resolveGitDir(repo)
	if err != nil {
		return DefaultGateConfig()
	}

	repoRoot := filepath.Dir(gitDir)
	configPath := filepath.Join(repoRoot, ".biggz", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultGateConfig()
	}

	var raw gateConfigYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return DefaultGateConfig()
	}

	cfg := DefaultGateConfig()
	cfg.enabled[GatePrePR] = raw.Gate.PrePR.Enabled
	cfg.enabled[GatePrePush] = raw.Gate.PrePush.Enabled
	return cfg
}

// ---------------------------------------------------------------------------
// Scope Change Detection
// ---------------------------------------------------------------------------

// ScopeDiff detects files that changed between the snapshot tree and the
// current HEAD tree using `git diff-tree --no-commit-id -r --name-only`.
// Returns the list of changed file paths. An empty list means no change.
func ScopeDiff(snapshotTree string) ([]string, error) {
	if snapshotTree == "" {
		return nil, nil
	}

	out, err := exec.Command("git", "diff-tree", "--no-commit-id", "-r",
		"--name-only", snapshotTree, "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("scope diff: git diff-tree: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	lines := strings.Split(output, "\n")
	var files []string
	for _, f := range lines {
		if f = strings.TrimSpace(f); f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// ---------------------------------------------------------------------------
// Gate Evaluation
// ---------------------------------------------------------------------------

// PrePRGate evaluates the pre-PR gate conditions:
//  1. The review chain must have events.
//  2. The receipt must be valid against the chain.
//  3. There must be no unaddressed blocking findings (CRITICAL / WARNING).
//
// If dryRun is true, the result reports all blocking reasons but always
// passes (exit zero).
//
// gitDir is the .git directory for RDD resolution; pass "" to skip RDD checks.
func PrePRGate(chain ValidatedChain, findings []Finding, receipt *Receipt, dryRun bool, gitDir string) GateResult {
	disp := ResolveDeliveryDisposition(gitDir)
	if disp == DispositionDisabledUnmanaged {
		fmt.Fprintln(os.Stderr, "RDD disabled: delivery unmanaged")
		return GateResult{Gate: GatePrePR, Passed: true, DryRun: dryRun}
	}
	return evaluateGate(GatePrePR, chain, findings, receipt, "", dryRun)
}

// PrePushGate evaluates the pre-push gate conditions:
//  1. All pre-PR conditions.
//  2. No unacknowledged scope change (detected via git diff-tree).
//
// If dryRun is true, the result reports all blocking reasons but always
// passes (exit zero).
//
// gitDir is the .git directory for RDD resolution; pass "" to skip RDD checks.
func PrePushGate(chain ValidatedChain, findings []Finding, receipt *Receipt, snapshotTree string, dryRun bool, gitDir string) GateResult {
	disp := ResolveDeliveryDisposition(gitDir)
	if disp == DispositionDisabledUnmanaged {
		fmt.Fprintln(os.Stderr, "RDD disabled: delivery unmanaged")
		return GateResult{Gate: GatePrePush, Passed: true, DryRun: dryRun}
	}
	return evaluateGate(GatePrePush, chain, findings, receipt, snapshotTree, dryRun)
}

// evaluateGate runs the gate checks common to pre-PR and pre-push.
//
// The caller MUST provide a chain loaded from the event store. The gate
// checks chain content integrity (Validate), receipt validity, blocking
// findings, and (for pre-push) scope changes.
func evaluateGate(kind GateKind, chain ValidatedChain, findings []Finding, receipt *Receipt, snapshotTree string, dryRun bool) GateResult {
	result := GateResult{
		Gate:   kind,
		DryRun: dryRun,
	}

	// 0. Check chain integrity: recompute SHA-256(content) == file name for
	// each event file. This catches tampered content even if the chain
	// structure (genesis → head links) appears consistent.
	if !chain.Valid {
		result.Reasons = append(result.Reasons, "review chain is invalid (integrity check failed)")
	}

	// 1. Validate receipt against chain.
	if receipt != nil {
		if err := receipt.Verify(chain); err != nil {
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("receipt verification failed: %v", err))
		}
	} else if chain.Count > 0 {
		// Auto-create receipt from chain and verify.
		r := NewReceipt(chain)
		if err := r.Verify(chain); err != nil {
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("auto-receipt verification failed: %v", err))
		}
	}

	// 2. Check for unaddressed blocking findings.
	for _, f := range findings {
		if f.Severity == SeverityCritical || f.Severity == SeverityWarning {
			msg := fmt.Sprintf("unresolved finding [%s]: %s", f.Severity, f.Message)
			if f.File != "" {
				msg = fmt.Sprintf("%s (file: %s, line: %d)", msg, f.File, f.Line)
			}
			result.Reasons = append(result.Reasons, msg)
		}
	}

	// 3. Check chain has events.
	if chain.Count == 0 {
		result.Reasons = append(result.Reasons, "review chain is empty (no events)")
	}

	// 4. (Pre-push only) Scope change detection.
	if kind == GatePrePush && snapshotTree != "" {
		changed, err := ScopeDiff(snapshotTree)
		if err != nil {
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("scope detection error: %v", err))
		} else if len(changed) > 0 {
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("unacknowledged scope change detected (%d file(s))", len(changed)))
			for _, f := range changed {
				result.Reasons = append(result.Reasons, fmt.Sprintf("  changed: %s", f))
			}
		}
	}

	// Passed: no blocking reasons, or dry-run mode (reports but does not fail).
	result.Passed = len(result.Reasons) == 0 || dryRun

	return result
}

// ---------------------------------------------------------------------------
// Backward-compatible wrappers (kept for existing tests and callers)
// ---------------------------------------------------------------------------

// GateConfigLegacy defines the rules for a specific gate (old format).
// Deprecated: use GateConfig / LoadGateConfig for new code.
type GateConfigLegacy struct {
	Kind              GateKind
	RequireReceipt    bool
	RequireNoFindings bool
	RequireApproved   bool
}

// DefaultGateConfigs returns the default configuration for each gate.
// Deprecated: use LoadGateConfig for new code.
func DefaultGateConfigs() map[GateKind]GateConfigLegacy {
	return map[GateKind]GateConfigLegacy{
		GatePreCommit: {
			Kind:              GatePreCommit,
			RequireReceipt:    true,
			RequireNoFindings: false,
			RequireApproved:   false,
		},
		GatePrePush: {
			Kind:              GatePrePush,
			RequireReceipt:    true,
			RequireNoFindings: true,
			RequireApproved:   false,
		},
		GateRelease: {
			Kind:              GateRelease,
			RequireReceipt:    true,
			RequireNoFindings: true,
			RequireApproved:   true,
		},
	}
}

// GateResultLegacy describes the outcome of a gate validation (old format).
// Deprecated: use GateResult for new code.
type GateResultLegacy struct {
	Gate    GateKind `json:"gate"`
	Allowed bool     `json:"allowed"`
	Reason  string   `json:"reason,omitempty"`
}

// ValidateCheck runs a single gate check against the review state.
// Deprecated: use PrePRGate / PrePushGate for new code.
func ValidateCheck(kind GateKind, state *model.ReviewState, cfg GateConfigLegacy, receipt *Receipt) *GateResultLegacy {
	if cfg.RequireReceipt {
		if receipt == nil {
			return &GateResultLegacy{
				Gate:    kind,
				Allowed: false,
				Reason:  fmt.Sprintf("%s: no receipt found", kind),
			}
		}
		if !VerifyReceipt(receipt, state) {
			return &GateResultLegacy{
				Gate:    kind,
				Allowed: false,
				Reason:  fmt.Sprintf("%s: receipt does not match review state", kind),
			}
		}
	}

	if state.Status != model.StatusCompleted {
		return &GateResultLegacy{
			Gate:    kind,
			Allowed: false,
			Reason:  fmt.Sprintf("%s: review status is %s, expected completed", kind, state.Status),
		}
	}

	return &GateResultLegacy{
		Gate:    kind,
		Allowed: true,
		Reason:  fmt.Sprintf("%s: passed", kind),
	}
}
