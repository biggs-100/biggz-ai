package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/policy"
	"github.com/biggs-100/biggz-ai/model"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Delivery Disposition
// ---------------------------------------------------------------------------

// DeliveryDisposition describes how gate evaluation handles a delivery
// based on RDD state and receipt presence.
type DeliveryDisposition int

const (
	DispositionReceiptGoverned DeliveryDisposition = iota
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
	// Full gate kind set (publication gates, mirroring gentle-ai).
	GatePostApply GateKind = "post-apply"
	GatePreCommit GateKind = "pre-commit"
	GatePrePush   GateKind = "pre-push"
	GatePrePR     GateKind = "pre-pr"
	GateRelease   GateKind = "release"
)

// Delivery dispositions reported on gate results. None of these values is an
// approval or a PASS.
const (
	// DeliveryReceiptGoverned means an existing receipt governs delivery.
	DeliveryReceiptGoverned = "receipt_governed"
	// DeliveryDisabledUnmanaged is delivery of work produced with the kill
	// switch off and no receipt.
	DeliveryDisabledUnmanaged = "disabled/unmanaged"
	// DeliveryUnmanaged is delivery with the switch on but no receipt yet.
	DeliveryUnmanaged = "unmanaged"
	// DeliveryBurned is delivery after a burned ephemeral receipt: the gate
	// becomes informational and ordinary repository policy governs.
	DeliveryBurned = "burned/unmanaged"
)

// supportedGateKind reports whether the kind is one of the five publication
// gates.
func supportedGateKind(kind GateKind) bool {
	switch kind {
	case GatePostApply, GatePreCommit, GatePrePush, GatePrePR, GateRelease:
		return true
	}
	return false
}

// GateFindingsSummary summarizes the recomputed finding state of a lineage at
// gate time: candidate-causal (blocking), candidate-causal resolved by the
// receipt, and pre-existing/base-only (follow-up) findings.
type GateFindingsSummary struct {
	Blocking int `json:"blocking"`
	Resolved int `json:"resolved"`
	FollowUp int `json:"follow_up"`
}

// GateResult describes the outcome of a gate validation.
//
// Passed is the legacy exit-zero indicator: it is true ONLY on a real pass,
// or under --dry-run (report without failing). Allowed is the honest
// authorization decision: false for every denial including the disabled
// disposition. Delivery names what governs delivery and never fabricates an
// approval.
// LensGateFinding is the structured breakdown of a lens finding for gate JSON reporting.
// It carries the lens ID, finding class (inferential|deterministic), ProofRefs, and location.
type LensGateFinding struct {
	LensID    string   `json:"lens_id"`
	ID        string   `json:"id"`
	Class     string   `json:"class"` // inferential | deterministic
	ProofRefs []string `json:"proof_refs,omitempty"`
	Message   string   `json:"message,omitempty"`
	File      string   `json:"file,omitempty"`
	Line      int      `json:"line,omitempty"`
}

type GateResult struct {
	Gate         GateKind             `json:"gate"`
	LineageID    string               `json:"lineage,omitempty"`
	Passed       bool                 `json:"passed"`
	Allowed      bool                 `json:"allowed"`
	Delivery     string               `json:"delivery,omitempty"`
	Reason       string               `json:"reason,omitempty"`
	ReceiptHash  string               `json:"receipt_hash,omitempty"`
	Findings     *GateFindingsSummary `json:"findings,omitempty"`
	Reasons      []string             `json:"reasons,omitempty"`
	DryRun       bool                 `json:"dry_run"`
	LensFindings []LensGateFinding    `json:"lens_findings,omitempty"`
}

// GateOptions carries the extra inputs some gate kinds need. Repo is the
// repository root; empty means the current working directory.
type GateOptions struct {
	Repo               string
	BaseRef            string // pre-pr only: explicit base ref boundary
	PrePRCIAttestation string // pre-pr only: signed CI attestation file (best-effort: presence + parse)
	DryRun             bool
}

// ---------------------------------------------------------------------------
// Gate Configuration
// ---------------------------------------------------------------------------

// gateConfigYAML is the on-disk structure for .biggz/config.yaml.
type gateConfigYAML struct {
	Gate struct {
		PrePR struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"pre-pr"`
		PrePush struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"pre-push"`
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
// Safety pre-check — verbatim DENIED[6]/SENSITIVE[8]/GUARDED[5] via internal/policy.
// Parity across 3 surfaces: biggz-synthesis-gate.js (Pi), safety.ts (opencode), gate.go (pre-check).
// Denied/sensitive→Allowed=false (block), guarded per ClassifyGuardedCommand.
// Logs surface+kind+pattern/path; never blocks non-denied human actions.
// ---------------------------------------------------------------------------

// SafetyPreCheck evaluates a tool call against the shared policy.
// Returns non-nil GateResult with Allowed=false when the call must be blocked,
// or with Reason indicating confirm; nil means not-guarded/allowed.
func SafetyPreCheck(tool string, args map[string]any, cwd string) *GateResult {
	cmd, _ := args["command"].(string)
	if cmd != "" && policy.IsDenied(cmd) {
		log.Printf("[safety] block surface=gate kind=block pattern=IsDenied cmd=%.80s", cmd)
		return &GateResult{Gate: GatePrePR, Allowed: false, Passed: false, Reason: "Gentle AI safety policy blocked a destructive shell command.", Reasons: []string{"safety: IsDenied"}}
	}
	if dec := policy.EvaluateSensitivePathTool(tool, args); dec != nil && dec.Kind == policy.DecisionBlock {
		log.Printf("[safety] block surface=gate kind=block path=%s", dec.Reason)
		return &GateResult{Gate: GatePrePR, Allowed: false, Passed: false, Reason: dec.Reason, Reasons: []string{"safety: sensitive path"}}
	}
	if cmd != "" {
		cfg := policy.LoadRuntimeGuardrailsConfig(cwd)
		switch policy.ClassifyGuardedCommand(cmd, cfg) {
		case "block":
			log.Printf("[safety] block surface=gate kind=block guarded cmd=%.80s", cmd)
			return &GateResult{Gate: GatePrePR, Allowed: false, Passed: false, Reason: "Gentle AI safety policy blocked guarded command."}
		case "confirm":
			log.Printf("[safety] confirm surface=gate kind=confirm guarded cmd=%.80s", cmd)
			return &GateResult{Gate: GatePrePR, Allowed: true, Passed: true, Reason: "safety: confirm"}
		}
	}
	return nil
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
		return GateResult{Gate: GatePrePR, Passed: true, Allowed: false, Delivery: DeliveryDisabledUnmanaged, Reason: "RDD disabled: delivery unmanaged", DryRun: dryRun}
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
		return GateResult{Gate: GatePrePush, Passed: true, Allowed: false, Delivery: DeliveryDisabledUnmanaged, Reason: "RDD disabled: delivery unmanaged", DryRun: dryRun}
	}
	return evaluateGate(GatePrePush, chain, findings, receipt, snapshotTree, dryRun)
}

// evaluateGate runs the gate checks common to pre-PR and pre-push.
//
// The caller MUST provide a chain loaded from the event store. The gate
// checks chain content integrity (Validate), receipt validity, blocking
// findings, and (for pre-push) scope changes.
func gateChainReason(chain ValidatedChain) string {
	if !chain.Valid {
		return "review chain is invalid (integrity check failed)"
	}
	return ""
}

func gateReceiptReasons(chain ValidatedChain, receipt *Receipt) []string {
	if receipt != nil {
		if err := receipt.Verify(chain); err != nil {
			return []string{fmt.Sprintf("receipt verification failed: %v", err)}
		}
		return nil
	}
	if chain.Count == 0 {
		return nil
	}
	r := NewReceipt(chain)
	if err := r.Verify(chain); err != nil {
		return []string{fmt.Sprintf("auto-receipt verification failed: %v", err)}
	}
	return nil
}

func gateFindingReasons(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		if f.Severity != SeverityCritical && f.Severity != SeverityWarning {
			continue
		}
		msg := fmt.Sprintf("unresolved finding [%s]: %s", f.Severity, f.Message)
		if f.File != "" {
			msg = fmt.Sprintf("%s (file: %s, line: %d)", msg, f.File, f.Line)
		}
		out = append(out, msg)
	}
	return out
}

func gateEmptyReason(chain ValidatedChain) string {
	if chain.Count == 0 {
		return "review chain is empty (no events)"
	}
	return ""
}

func gateScopeReasons(kind GateKind, snapshotTree string) []string {
	if kind != GatePrePush || snapshotTree == "" {
		return nil
	}
	changed, err := ScopeDiff(snapshotTree)
	if err != nil {
		return []string{fmt.Sprintf("scope detection error: %v", err)}
	}
	if len(changed) == 0 {
		return nil
	}
	out := []string{fmt.Sprintf("unacknowledged scope change detected (%d file(s))", len(changed))}
	for _, f := range changed {
		out = append(out, fmt.Sprintf("  changed: %s", f))
	}
	return out
}

func evaluateGate(kind GateKind, chain ValidatedChain, findings []Finding, receipt *Receipt, snapshotTree string, dryRun bool) GateResult {
	result := GateResult{Gate: kind, DryRun: dryRun}
	if reason := gateChainReason(chain); reason != "" {
		result.Reasons = append(result.Reasons, reason)
	}
	result.Reasons = append(result.Reasons, gateReceiptReasons(chain, receipt)...)
	result.Reasons = append(result.Reasons, gateFindingReasons(findings)...)
	if reason := gateEmptyReason(chain); reason != "" {
		result.Reasons = append(result.Reasons, reason)
	}
	result.Reasons = append(result.Reasons, gateScopeReasons(kind, snapshotTree)...)
	result.Passed = len(result.Reasons) == 0 || dryRun
	return result
}

func enhancedChainCheck(chain ValidatedChain, result *EnhancedGateResult) {
	if chain.Valid {
		return
	}
	reason := "review chain is invalid (integrity check failed)"
	result.Reasons = append(result.Reasons, reason)
	result.Diagnostics.DenialReasons = append(result.Diagnostics.DenialReasons, GateDenialReason{Stage: "chain_integrity", Code: "chain_corrupt", Message: reason})
}

func enhancedReceiptCheck(chain ValidatedChain, receipt *Receipt, result *EnhancedGateResult) {
	if receipt != nil {
		if err := receipt.Verify(chain); err != nil {
			msg := fmt.Sprintf("receipt verification failed: %v", err)
			result.Reasons = append(result.Reasons, msg)
			result.Diagnostics.ReceiptValid = false
			result.Diagnostics.DenialReasons = append(result.Diagnostics.DenialReasons, GateDenialReason{Stage: "receipt_validation", Code: "receipt_mismatch", Message: msg})
		}
		return
	}
	if chain.Count == 0 {
		return
	}
	r := NewReceipt(chain)
	if err := r.Verify(chain); err != nil {
		msg := fmt.Sprintf("auto-receipt verification failed: %v", err)
		result.Reasons = append(result.Reasons, msg)
		result.Diagnostics.DenialReasons = append(result.Diagnostics.DenialReasons, GateDenialReason{Stage: "receipt_validation", Code: "receipt_mismatch", Message: msg})
	}
}

func enhancedFindingsCheck(findings []Finding, result *EnhancedGateResult) {
	for _, f := range findings {
		if f.Severity != SeverityCritical && f.Severity != SeverityWarning {
			continue
		}
		result.Diagnostics.HasBlockingFindings = true
		msg := fmt.Sprintf("unresolved finding [%s]: %s", f.Severity, f.Message)
		if f.File != "" {
			msg = fmt.Sprintf("%s (file: %s, line: %d)", msg, f.File, f.Line)
		}
		result.Reasons = append(result.Reasons, msg)
	}
}

func enhancedEmptyCheck(chain ValidatedChain, result *EnhancedGateResult) {
	if chain.Count != 0 {
		return
	}
	msg := "review chain is empty (no events)"
	result.Reasons = append(result.Reasons, msg)
	result.Diagnostics.DenialReasons = append(result.Diagnostics.DenialReasons, GateDenialReason{Stage: "chain_empty", Code: "no_events", Message: msg})
}

func enhancedScopeCheck(kind GateKind, snapshotTree string, result *EnhancedGateResult) {
	if kind != GatePrePush || snapshotTree == "" {
		return
	}
	scopeChange, err := detectScopeChange(snapshotTree, "HEAD")
	if err != nil || scopeChange == nil {
		return
	}
	result.Diagnostics.ScopeChange = scopeChange
	msg := fmt.Sprintf("unacknowledged scope change detected (%d file(s))", len(scopeChange.ChangedFiles))
	result.Reasons = append(result.Reasons, msg)
	result.Diagnostics.DenialReasons = append(result.Diagnostics.DenialReasons, GateDenialReason{Stage: "scope_change", Code: "files_changed", Message: msg})
}

// EvaluateEnhancedGate runs gate evaluation and returns structured diagnostics.
// This is the enhanced version that includes GateDiagnostics for better
// error reporting and debugging.
func EvaluateEnhancedGate(kind GateKind, chain ValidatedChain, findings []Finding, receipt *Receipt, snapshotTree string, dryRun bool, gitDir string) EnhancedGateResult {
	disp := ResolveDeliveryDisposition(gitDir)
	if disp == DispositionDisabledUnmanaged {
		return EnhancedGateResult{Gate: kind, Passed: true, DryRun: dryRun, Disposition: disp}
	}
	result := EnhancedGateResult{
		Gate: kind, DryRun: dryRun, Disposition: disp,
		Diagnostics: &GateDiagnostics{ChainValid: chain.Valid, ReceiptValid: receipt == nil || receipt.Verify(chain) == nil},
	}
	enhancedChainCheck(chain, &result)
	enhancedReceiptCheck(chain, receipt, &result)
	enhancedFindingsCheck(findings, &result)
	enhancedEmptyCheck(chain, &result)
	enhancedScopeCheck(kind, snapshotTree, &result)
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

// ---------------------------------------------------------------------------
// Full gate evaluation (Phase A3 — review-workflow parity)
// ---------------------------------------------------------------------------
//
// EvaluateGate is the single gate entry point behind `biggz review gate
// <kind> <lineage>`. It resolves the RDD kill switch, loads the persisted
// receipt (A2) and the captured lens results (A1), recomputes blocking
// findings from candidate-causal dispositions, validates the receipt binding
// against the live chain and repository trees, and runs the kind-specific
// checks adapted from gentle-ai's compact gate semantics at a practical
// level:
//
//   - post-apply: chain + receipt + findings + current HEAD tree matches the
//     reviewed candidate tree (when derivable).
//   - pre-commit: staged projection — the staged tree must reproduce the
//     reviewed candidate tree exactly, and no staged path may lie outside the
//     reviewed path manifest. biggz manifests freeze only tracked paths, so
//     intended-untracked retention reduces to the staged-subset check: any
//     untracked path entering the index fails it (documented reduced scope).
//   - pre-push: publication range — the reviewed commit must be an ancestor of
//     HEAD with no unreviewed commits after it. A correction receipt
//     (fix_delta_hash != empty) may deliver exactly one fix commit whose tree
//     equals the reviewed candidate (fix-diff delivery).
//   - pre-pr: base-ref boundary — the PR diff (base vs candidate) must stay
//     inside the reviewed path manifest; files outside the manifest fail.
//     Optional --pre-pr-ci-attestation accepts a signed JSON file by presence
//     and parseability (signature verification is out of scope).
//   - release: receipt + findings + freshness (HEAD tree == reviewed candidate
//     tree). biggz release is tag-based, so the configuration/generated/
//     provenance/publication-boundary artifact checks are reduced away
//     (documented reduced scope).
//
// Disabled-mode semantics: when the kill switch is off, ANY gate returns
// Passed:true, Allowed:false with delivery "disabled/unmanaged" (exit 0 but
// delivery unmanaged, consistent with PrePRGate/PrePushGate). This is the
// honest unmanaged disposition: delivery follows ordinary repository policy,
// never an approval. An UNREADABLE switch is NOT a disabled switch:
// RDDStatus errors resolve as managed and the gate evaluates normally —
// delivery never fabricates "disabled/unmanaged" from a corrupt file.

// gateBinding is the repository-derived state every gate check shares: the
// frozen start plan, the immutable base/candidate trees, the ordered
// changed-path manifest entries frozen by the review, and its digest.
type gateBinding struct {
	plan           StartEventPayload
	baseTree       string
	candidateTree  string
	manifestPaths  []string
	manifestSHA256 string
}

// deriveGateBinding re-derives the frozen candidate binding from the genesis
// subject and the live repository.
func deriveGateBinding(repo string, chain ValidatedChain) (gateBinding, error) {
	if chain.Count == 0 {
		return gateBinding{}, errors.New("lineage has no events")
	}
	var plan StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil || strings.TrimSpace(plan.CommitSHA) == "" {
		return gateBinding{}, errors.New("genesis event does not carry a review subject")
	}
	baseTree, candidateTree, entries, err := candidateManifest(repo, plan.CommitSHA)
	if err != nil {
		return gateBinding{}, fmt.Errorf("derive candidate binding: %w", err)
	}
	digest, err := ChangedPathManifestDigest(entries)
	if err != nil {
		return gateBinding{}, fmt.Errorf("derive candidate binding: manifest digest: %w", err)
	}
	return gateBinding{
		plan: plan, baseTree: baseTree, candidateTree: candidateTree,
		manifestPaths: ManifestPaths(entries), manifestSHA256: digest,
	}, nil
}

// EvaluateGate runs the full gate evaluation for one publication gate kind
// against a lineage. Gate denials are encoded in the result; error is
// reserved for hard infrastructure failures (store or repository unusable).
func EvaluateGate(kind GateKind, repo, lineageID string, opts GateOptions) (GateResult, error) {
	if !supportedGateKind(kind) {
		return GateResult{}, fmt.Errorf("unsupported review gate %q (use: post-apply, pre-commit, pre-push, pre-pr, or release)", kind)
	}
	if opts.Repo != "" {
		repo = opts.Repo
	}
	result := GateResult{Gate: kind, LineageID: lineageID, DryRun: opts.DryRun}
	if stopped, res := evaluateGateKillSwitch(repo, result); stopped {
		return res, nil
	}
	store, chain, err := evaluateGateLoadChain(repo, lineageID)
	if err != nil {
		return GateResult{}, err
	}
	if burned, res := evaluateGateBurned(store, chain, result); burned {
		return res, nil
	}
	reasons := collectGateReasons(repo, store, chain, kind, opts, &result)
	result.Reasons = reasons
	result.Reason = strings.Join(reasons, "; ")
	if result.Reason == "" {
		result.Reason = "gate passed"
	}
	result.Allowed = len(reasons) == 0
	result.Passed = result.Allowed || opts.DryRun
	if result.Allowed {
		result.Delivery = DeliveryReceiptGoverned
	}
	return result, nil
}

func evaluateGateKillSwitch(repo string, result GateResult) (bool, GateResult) {
	worktreeDir, commonDir := detectRDDDirs(repo)
	if status, rddErr := RDDStatus(worktreeDir, commonDir); rddErr == nil && status.EffectiveMode == RDDModeDisabled {
		result.Passed = true
		result.Allowed = false
		result.Delivery = DeliveryDisabledUnmanaged
		result.Reason = "RDD disabled: delivery unmanaged"
		return true, result
	}
	return false, result
}

func evaluateGateLoadChain(repo, lineageID string) (*Store, ValidatedChain, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return nil, ValidatedChain{}, fmt.Errorf("gate: open store: %w", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		return nil, ValidatedChain{}, fmt.Errorf("gate: load chain: %w", err)
	}
	return store, chain, nil
}

func evaluateGateBurned(store *Store, chain ValidatedChain, result GateResult) (bool, GateResult) {
	if IsChainBurned(chain) || store.IsBurned() {
		result.Delivery = DeliveryBurned
		result.Reason = "review burned: receipt is ephemeral and burned after finalize; delivery via ordinary repository policy"
		result.Reasons = []string{result.Reason}
		result.Allowed = true
		result.Passed = false
		return true, result
	}
	return false, result
}

func collectGateReasons(repo string, store *Store, chain ValidatedChain, kind GateKind, opts GateOptions, result *GateResult) []string {
	var reasons []string
	reasons = append(reasons, gateChainReasons(store, chain)...)
	reasons = append(reasons, gateTerminalReasons(chain)...)
	receipt, binding, receiptErr, bindingErr := loadGateReceiptAndBinding(repo, store, chain, result)
	reasons = appendReceiptReasons(reasons, receiptErr, receipt, binding, bindingErr, chain, result)
	if receiptErr == nil && bindingErr == nil {
		reasons = append(reasons, gateKindReasons(kind, repo, opts, receipt, binding)...)
	}
	return reasons
}

func gateChainReasons(store *Store, chain ValidatedChain) []string {
	var reasons []string
	verdict := store.Validate()
	if !verdict.Valid {
		reasons = append(reasons, "review chain is invalid (integrity check failed): "+verdict.Reason)
	}
	if chain.Count == 0 {
		reasons = append(reasons, "review chain is empty (no events)")
	}
	return reasons
}

func gateTerminalReasons(chain ValidatedChain) []string {
	state, reason := terminatedStateOf(chain)
	if state == "" {
		return nil
	}
	if state == "invalidated" && reason != "" {
		return []string{"lineage is invalidated: " + reason}
	}
	return []string{"lineage is " + state}
}

func loadGateReceiptAndBinding(repo string, store *Store, chain ValidatedChain, result *GateResult) (PersistedReceipt, gateBinding, error, error) {
	var receipt PersistedReceipt
	var binding gateBinding
	var bindingErr error
	if chain.Count > 0 {
		if binding, bindingErr = deriveGateBinding(repo, chain); bindingErr != nil {
			// bindingErr returned to caller for reasons
		}
	}
	loadedReceipt, receiptErr := loadPersistedReceipt(store, chain)
	if receiptErr == nil {
		receipt = loadedReceipt
		result.ReceiptHash = receipt.ReceiptHash
	}
	return receipt, binding, receiptErr, bindingErr
}

func appendReceiptReasons(reasons []string, receiptErr error, receipt PersistedReceipt, binding gateBinding, bindingErr error, chain ValidatedChain, result *GateResult) []string {
	if bindingErr != nil {
		reasons = append(reasons, "gate binding: "+bindingErr.Error())
	}
	if receiptErr != nil {
		reasons = append(reasons, receiptErr.Error())
		return reasons
	}
	if bindingErr == nil {
		reasons = append(reasons, verifyReceiptBinding(receipt, binding, chain)...)
	}
	summary, findingReasons := recomputeGateFindings(chain, receipt)
	result.Findings = &summary
	reasons = append(reasons, findingReasons...)
	result.LensFindings = BuildLensFindingsBreakdown(chain)
	return reasons
}

func gateKindReasons(kind GateKind, repo string, opts GateOptions, receipt PersistedReceipt, binding gateBinding) []string {
	switch kind {
	case GatePostApply:
		return postApplyChecks(repo, receipt)
	case GatePreCommit:
		return preCommitChecks(repo, receipt, binding)
	case GatePrePush:
		return prePushChecks(repo, receipt, binding)
	case GatePrePR:
		return prePRChecks(repo, opts, receipt, binding)
	case GateRelease:
		return releaseChecks(repo, receipt)
	default:
		return nil
	}
}

// loadPersistedReceipt loads the terminal receipt artifact referenced by the
// lineage's complete_review event. A missing artifact names finalize as the
// required step.
func loadPersistedReceipt(store *Store, chain ValidatedChain) (PersistedReceipt, error) {
	ref := receiptArtifactOf(chain)
	if ref == nil {
		return PersistedReceipt{}, errors.New("missing persisted review receipt: run 'biggz review finalize <lineage>' to finalize the captured review")
	}
	return readReceiptFile(store, completeEventPayload{
		Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash,
	})
}

// verifyReceiptBinding checks that the persisted receipt matches the live
// lineage state: the chain genesis and pre-finalize head revisions, and the
// repository-derived base/candidate trees and path manifest digest. A
// tampered or foreign receipt fails here.
func verifyReceiptBinding(receipt PersistedReceipt, binding gateBinding, chain ValidatedChain) []string {
	var reasons []string
	revisions := recordRevisions(chain)
	if len(revisions) < 2 {
		return append(reasons, "receipt binding: lineage has no captured head to bind")
	}
	if receipt.GenesisRevision != revisions[0] {
		reasons = append(reasons, fmt.Sprintf(
			"receipt binding: genesis revision %s does not match the lineage genesis %s; the receipt is foreign or the chain was replaced",
			receipt.GenesisRevision, revisions[0]))
	}
	completeIndex := -1
	for index := len(chain.Records) - 1; index >= 0; index-- {
		if chain.Records[index].Operation == CompleteReviewOperation {
			completeIndex = index
			break
		}
	}
	if completeIndex < 1 {
		return append(reasons, "receipt binding: chain carries no terminal complete_review event")
	}
	if receipt.HeadRevision != revisions[completeIndex-1] {
		reasons = append(reasons, fmt.Sprintf(
			"receipt binding: head revision %s does not match the captured head %s; the chain changed after finalize",
			receipt.HeadRevision, revisions[completeIndex-1]))
	}
	if receipt.BaseTree != binding.baseTree {
		reasons = append(reasons, fmt.Sprintf(
			"receipt binding: base tree %s does not match the frozen candidate base %s; the receipt is foreign or the repository changed",
			receipt.BaseTree, binding.baseTree))
	}
	if receipt.InitialReviewTree != binding.candidateTree {
		reasons = append(reasons, fmt.Sprintf(
			"receipt binding: initial review tree %s does not match the frozen candidate tree %s",
			receipt.InitialReviewTree, binding.candidateTree))
	}
	if receipt.FinalCandidateTree != binding.candidateTree {
		reasons = append(reasons, fmt.Sprintf(
			"receipt binding: final candidate tree %s does not match the frozen candidate tree %s",
			receipt.FinalCandidateTree, binding.candidateTree))
	}
	if receipt.PathsDigest != binding.manifestSHA256 {
		reasons = append(reasons, fmt.Sprintf(
			"receipt binding: paths digest %s does not match the frozen candidate manifest %s",
			receipt.PathsDigest, binding.manifestSHA256))
	}
	return reasons
}

// recomputeGateFindings recomputes the finding summary from the captured lens
// results (A1): candidate-causal finding IDs recorded at admission are
// blocking unless the receipt (or its fix delta) shows them resolved; refuted
// findings appear resolved, standing and deterministic findings block; unknown
// dispositions or insufficient evidence on a severe finding escalate.
// Lens findings are candidate-causal gate inputs: heuristic findings
// default to inferential and surface as warnings (exit 0) unless they are
// deterministic with concrete ProofRefs or the refuter verdict is stands,
// in which case they block pre-pr (exit 1) with --json pass:false.
// DeriveRiskInput is reused for lens evidence — no second git diff --numstat parse.
func recomputeGateFindings(chain ValidatedChain, receipt PersistedReceipt) (GateFindingsSummary, []string) {
	var summary GateFindingsSummary
	var reasons []string
	fixDeltaDelivered := receipt.FixDeltaHash != "" && receipt.FixDeltaHash != EmptyFixDeltaHash
	lastComplete := findLastComplete(chain)
	findingsByID := make(map[string]ArtifactFinding)
	for index := range chain.Records {
		rec := &chain.Records[index]
		if rec.Operation != LensResultOperation {
			continue
		}
		payload, ok := parseLensResultPayload(rec)
		if !ok {
			continue
		}
		for _, finding := range payload.Result.Findings {
			findingsByID[finding.ID] = finding
			processFindingDisposition(finding, &summary, &reasons)
		}
		for _, id := range payload.CandidateCausalFindingIDs {
			processCandidateCausal(id, index, lastComplete, findingsByID, receipt, fixDeltaDelivered, &summary, &reasons)
		}
	}
	return summary, reasons
}

// findLastComplete returns the index of the last complete_review event, or -1.
func findLastComplete(chain ValidatedChain) int {
	for i := len(chain.Records) - 1; i >= 0; i-- {
		if chain.Records[i].Operation == CompleteReviewOperation {
			return i
		}
	}
	return -1
}

// parseLensResultPayload unmarshals a lens result payload and validates admission.
func parseLensResultPayload(rec *Record) (lensResultEventPayload, bool) {
	var payload lensResultEventPayload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil || payload.AdmissionDecision != AdmissionCompleted {
		return payload, false
	}
	return payload, true
}

// processFindingDisposition classifies a single finding's disposition.
func processFindingDisposition(finding ArtifactFinding, summary *GateFindingsSummary, reasons *[]string) {
	if !isSevereSeverity(finding.Severity) {
		if finding.CausalDisposition == CausalPreExisting || finding.CausalDisposition == CausalBaseOnly {
			summary.FollowUp++
		}
		return
	}
	switch finding.CausalDisposition {
	case CausalPreExisting, CausalBaseOnly:
		summary.FollowUp++
	case CausalIntroduced, CausalBehaviorActivated, CausalWorsened:
		if finding.EvidenceClass == EvidenceInsufficient {
			*reasons = append(*reasons, fmt.Sprintf("finding [%s] has insufficient evidence class; the lineage must escalate and re-capture the lens", finding.ID))
		}
	default:
		*reasons = append(*reasons, fmt.Sprintf("finding [%s] has unknown causal disposition %q; the lineage must escalate", finding.ID, finding.CausalDisposition))
	}
}

// isBlockedFinding reports whether a finding blocks the gate.
func isBlockedFinding(finding ArtifactFinding, isStanding bool) bool {
	return isStanding || finding.EvidenceClass == EvidenceDeterministic
}

// gateForFile reports whether the finding for a file should gate delivery.
func gateForFile(finding ArtifactFinding, receipt PersistedReceipt) bool {
	isStanding := slices.Contains(receipt.StandingFindingIDs, finding.ID)
	return isBlockedFinding(finding, isStanding)
}

// processCandidateCausal evaluates a candidate-causal finding ID for blocking status.
func processCandidateCausal(id string, index, lastComplete int, findingsByID map[string]ArtifactFinding, receipt PersistedReceipt, fixDeltaDelivered bool, summary *GateFindingsSummary, reasons *[]string) {
	isPost := lastComplete >= 0 && index > lastComplete
	if slices.Contains(receipt.ResolvedFindingIDs, id) || (fixDeltaDelivered && !isPost) {
		summary.Resolved++
		return
	}
	finding, known := findingsByID[id]
	_ = known
	isStanding := slices.Contains(receipt.StandingFindingIDs, id)
	if !isBlockedFinding(finding, isStanding) {
		summary.FollowUp++
		return
	}
	summary.Blocking++
	*reasons = append(*reasons, blockedFindingMessage(id, isStanding, finding.EvidenceClass == EvidenceDeterministic))
}

// blockedFindingMessage returns the blocking reason for an unresolved finding.
func blockedFindingMessage(id string, isStanding, isDeterministic bool) string {
	switch {
	case isStanding:
		return fmt.Sprintf("unresolved finding [%s]: the refuter verdict stands; the finding remains blocking", id)
	case isDeterministic:
		return fmt.Sprintf("unresolved finding [%s]: deterministic finding is auto-blocking and cannot be refuted; resolve it with a correction", id)
	default:
		return fmt.Sprintf("unresolved finding [%s]: candidate-causal finding is not resolved by the persisted receipt; review it and re-finalize the lineage", id)
	}
}

// BuildLensFindingsBreakdown returns the structured lens finding breakdown for
// gate --json output, grouping by inferential|deterministic with ProofRefs.
// No duplicate git diff is performed — lens evidence reuses DeriveRiskInput.
func BuildLensFindingsBreakdown(chain ValidatedChain) []LensGateFinding {
	out := []LensGateFinding{}
	for index := range chain.Records {
		rec := &chain.Records[index]
		if rec.Operation != LensResultOperation {
			continue
		}
		var payload lensResultEventPayload
		if err := json.Unmarshal(rec.Payload, &payload); err != nil || payload.AdmissionDecision != AdmissionCompleted {
			continue
		}
		for _, f := range payload.Result.Findings {
			class := string(f.EvidenceClass)
			if class == "" {
				class = string(EvidenceInferential)
			}
			proofRefs := f.ProofRefs
			// ArtifactFinding uses ProofRefs, but LensFinding fallback is Location as ProofRef
			if len(proofRefs) == 0 && f.Location != "" {
				proofRefs = []string{f.Location}
			}
			out = append(out, LensGateFinding{
				LensID:    f.Lens,
				ID:        f.ID,
				Class:     class,
				ProofRefs: proofRefs,
				Message:   f.Claim,
			})
		}
	}
	return out
}

// postApplyChecks verifies the applied candidate is still the reviewed
// candidate: current HEAD tree matches the receipt's final candidate tree,
// when derivable.
func postApplyChecks(repo string, receipt PersistedReceipt) []string {
	headTree, err := headTreeSHA(repo)
	if err != nil {
		return nil // only when available
	}
	if headTree != receipt.FinalCandidateTree {
		return []string{fmt.Sprintf(
			"post-apply: current HEAD tree %s does not match the reviewed candidate tree %s", headTree, receipt.FinalCandidateTree)}
	}
	return nil
}

// preCommitChecks verifies the staged projection reproduces the reviewed
// candidate: every staged path must lie inside the reviewed path manifest,
// and the staged tree must equal the receipt's final candidate tree.
// Intended-untracked retention is reduced to the staged-subset check: biggz
// manifests freeze only tracked paths, so untracked paths are outside the
// reviewed scope by construction and any of them entering the index fails the
// subset check.
func preCommitChecks(repo string, receipt PersistedReceipt, binding gateBinding) []string {
	var reasons []string
	staged, err := stagedPaths(repo)
	if err != nil {
		return append(reasons, fmt.Sprintf("pre-commit: staged projection cannot be derived: %v", err))
	}
	var outside []string
	for _, path := range staged {
		if !containsString(binding.manifestPaths, path) {
			outside = append(outside, path)
		}
	}
	if len(outside) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"pre-commit: staged path(s) outside the reviewed candidate: %s", strings.Join(outside, ", ")))
	}
	stagedTree, err := gitIn(repo, "write-tree")
	if err != nil {
		return append(reasons, fmt.Sprintf("pre-commit: staged tree cannot be derived: %v", err))
	}
	if stagedTree != receipt.FinalCandidateTree {
		reasons = append(reasons, fmt.Sprintf(
			"pre-commit: staged tree %s does not reproduce the reviewed candidate tree %s; stage exactly the reviewed candidate before committing",
			stagedTree, receipt.FinalCandidateTree))
	}
	return reasons
}

// prePushChecks verifies the publication range: the reviewed commit must be
// an ancestor of HEAD with no unreviewed commits after it. A correction
// receipt (fix-diff delivery) may deliver exactly one commit whose tree equals
// the reviewed candidate.
func prePushChecks(repo string, receipt PersistedReceipt, binding gateBinding) []string {
	fixDeltaDelivered := receipt.FixDeltaHash != "" && receipt.FixDeltaHash != EmptyFixDeltaHash
	ancestor, err := commitIsAncestor(repo, binding.plan.CommitSHA, "HEAD")
	if err != nil {
		return []string{fmt.Sprintf("pre-push: publication range cannot be derived: %v", err)}
	}
	if !ancestor {
		return []string{fmt.Sprintf(
			"pre-push: the reviewed commit %s is not on the current HEAD lineage", binding.plan.CommitSHA)}
	}
	count, err := gitIn(repo, "rev-list", "--count", binding.plan.CommitSHA+"..HEAD")
	if err != nil {
		return []string{fmt.Sprintf("pre-push: delivered commit count cannot be derived: %v", err)}
	}
	commits := strings.TrimSpace(count)
	if commits != "0" {
		if fixDeltaDelivered && commits == "1" {
			headTree, treeErr := headTreeSHA(repo)
			if treeErr != nil {
				return []string{fmt.Sprintf("pre-push: fix-diff delivery tree cannot be derived: %v", treeErr)}
			}
			if headTree == receipt.FinalCandidateTree {
				return nil // fix-diff delivery of the reviewed correction
			}
		}
		return []string{fmt.Sprintf(
			"pre-push: %s unreviewed commit(s) after the reviewed head %s; review them before pushing",
			commits, binding.plan.CommitSHA)}
	}
	return nil
}

// prePRChecks verifies the PR diff against the base boundary stays inside the
// reviewed candidate scope, and that HEAD still matches the reviewed
// candidate. The base boundary is the explicit --base-ref tree when given,
// otherwise the reviewed commit's parent tree (the frozen receipt base).
// The optional CI attestation is accepted best-effort: presence + parse of a
// signed JSON file; signature verification is out of scope.
func prePRChecks(repo string, opts GateOptions, receipt PersistedReceipt, binding gateBinding) []string {
	var reasons []string
	headTree, err := headTreeSHA(repo)
	if err != nil {
		return append(reasons, fmt.Sprintf("pre-pr: current HEAD tree cannot be derived: %v", err))
	}
	if headTree != receipt.FinalCandidateTree {
		return append(reasons, fmt.Sprintf(
			"pre-pr: current HEAD tree %s does not match the reviewed candidate tree %s", headTree, receipt.FinalCandidateTree))
	}
	boundary := opts.BaseRef
	boundaryTree := ""
	if boundary != "" {
		if boundaryTree, err = gitIn(repo, "rev-parse", boundary+"^{tree}"); err != nil {
			return append(reasons, fmt.Sprintf("pre-pr: base boundary %q cannot be resolved: %v", boundary, err))
		}
	} else if parentTree, parentErr := gitIn(repo, "rev-parse", binding.plan.CommitSHA+"^^{tree}"); parentErr == nil {
		boundaryTree = parentTree
	} else {
		boundaryTree = receipt.BaseTree // root commit: the frozen empty-tree base
	}
	diff, err := gitIn(repo, "diff", "--name-only", "-z", "--no-renames", boundaryTree, receipt.FinalCandidateTree)
	if err != nil {
		return append(reasons, fmt.Sprintf("pre-pr: diff against the base boundary cannot be derived: %v", err))
	}
	var outside []string
	for _, path := range splitNulPaths(diff) {
		if !containsString(binding.manifestPaths, path) {
			outside = append(outside, path)
		}
	}
	if len(outside) > 0 {
		reasons = append(reasons, fmt.Sprintf(
			"pre-pr: diff against base %s touches path(s) outside the reviewed candidate scope: %s",
			boundaryTree, strings.Join(outside, ", ")))
	}
	if opts.PrePRCIAttestation != "" {
		data, readErr := os.ReadFile(opts.PrePRCIAttestation)
		switch {
		case readErr != nil:
			reasons = append(reasons, fmt.Sprintf("pre-pr: CI attestation %q cannot be read: %v", opts.PrePRCIAttestation, readErr))
		case !json.Valid(data):
			reasons = append(reasons, fmt.Sprintf("pre-pr: CI attestation %q is not valid signed JSON", opts.PrePRCIAttestation))
		}
	}
	return reasons
}

// releaseChecks verifies release freshness: current HEAD tree must match the
// reviewed candidate tree. biggz release is tag-based, so the release
// configuration/generated/provenance/publication-boundary/freshness ARTIFACT
// checks are reduced away and only the receipt + freshness gates remain
// (documented reduced scope).
func releaseChecks(repo string, receipt PersistedReceipt) []string {
	headTree, err := headTreeSHA(repo)
	if err != nil {
		return []string{fmt.Sprintf("release: freshness cannot be derived: %v", err)}
	}
	if headTree != receipt.FinalCandidateTree {
		return []string{fmt.Sprintf(
			"release: current HEAD tree %s does not match the reviewed candidate %s; release requires the reviewed commit at HEAD",
			headTree, receipt.FinalCandidateTree)}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Git helpers
// ---------------------------------------------------------------------------

// detectRDDDirs resolves the worktree and common git dirs for RDD resolution.
// Outside a git repository both are empty and RDDStatus falls back to the
// global mode.
func detectRDDDirs(repo string) (worktreeDir, commonDir string) {
	worktreeDir = revParseRepoDir(repo, "--git-dir")
	commonDir = revParseRepoDir(repo, "--git-common-dir")
	if commonDir == "" {
		commonDir = worktreeDir
	}
	return worktreeDir, commonDir
}

// revParseRepoDir runs `git rev-parse <flag>` in repo and resolves the result
// to an absolute path. Returns "" on any failure.
func revParseRepoDir(repo, flag string) string {
	args := []string{"rev-parse", flag}
	if repo != "" {
		args = append([]string{"-C", repo}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return ""
	}
	if !filepath.IsAbs(dir) {
		base := repo
		if base == "" {
			base, _ = os.Getwd()
		}
		dir = filepath.Join(base, dir)
	}
	return filepath.Clean(dir)
}

// gitIn runs a git command in repo and returns trimmed stdout.
func gitIn(repo string, args ...string) (string, error) {
	full := make([]string, 0, len(args)+2)
	if repo != "" {
		full = append(full, "-C", repo)
	}
	full = append(full, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// headTreeSHA resolves the current HEAD tree. Only used "when available".
func headTreeSHA(repo string) (string, error) {
	return gitIn(repo, "rev-parse", "HEAD:")
}

// stagedPaths returns the NUL-separated path list of the current index.
func stagedPaths(repo string) ([]string, error) {
	out, err := gitIn(repo, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	return splitNulPaths(out), nil
}

// commitIsAncestor reports whether ancestor is an ancestor of head.
func commitIsAncestor(repo, ancestor, head string) (bool, error) {
	_, err := gitIn(repo, "merge-base", "--is-ancestor", ancestor, head)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// splitNulPaths splits NUL-separated git output into non-empty paths.
func splitNulPaths(out string) []string {
	var paths []string
	for _, value := range strings.Split(out, "\x00") {
		if value = strings.TrimSpace(value); value != "" {
			paths = append(paths, value)
		}
	}
	return paths
}
