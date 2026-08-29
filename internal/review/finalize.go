// Compact review finalization — correction budget freeze, forecast and
// cumulative enforcement, and the persisted content-addressed receipt.
//
// This is biggz-ai's Phase A2 port of gentle-ai's compact review semantics
// (internal/reviewtransaction/compact.go + risk.go), adapted to the
// content-addressed event store:
//
//   - `review start` freezes CorrectionBudget = min(200, ceil(original changed
//     lines / 2)) into the start_review event payload.
//   - `review finalize` is the terminal transition: every selected lens slot
//     must be captured, the canonical payloads are read back from the events
//     for re-verification, a complete_review event is appended, and the full
//     receipt is materialized under <lineage>/receipts/<sha256>.json.
//   - The receipt hash binds genesis + head + trees + paths digest + findings,
//     so gate validation can recompute it from the chain alone.
package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/model"
)

const (
	ReviewStartEventSchema       = "biggz-ai.review-start-event/v1"
	FinalizeEventSchema          = "biggz-ai.review-complete-event/v1"
	ReviewReceiptSchema          = "biggz-ai.review-receipt/v1"
	ReceiptBindingDomain         = "biggz-ai.review-receipt-binding/v1"
	ReviewEvidenceDomain         = "biggz-ai.review-evidence/v1"
	ReceiptsDirName              = "receipts"
	CompleteReviewOperation      = "complete_review"
	MaxCompactCorrectionAttempts = 1
	CorrectionBudgetCap          = 200
	ReviewReceiptTerminalState   = "completed"
)

// SessionStopState captures closure invariants for CanStopSession.
type SessionStopState struct {
	PendingFindings int `json:"pending_findings"`
	PendingLenses   int `json:"pending_lenses"`
}

// CanStopSession is pure and idempotent: true only when closure invariants hold.
func CanStopSession(s SessionStopState) bool {
	return s.PendingFindings == 0 && s.PendingLenses == 0
}

// ErrAlreadyBurned reports that the lineage receipt is already burned
// (ephemeral receipt consumed). It prevents replay of the same receipt.
var ErrAlreadyBurned = errors.New("review: lineage already burned after successful finalize")

// BurnEnabled controls whether Finalize burns the receipt after successful
// finalize. It is true by default (ephemeral receipt). Tests that need a
// durable receipt for gate parity can temporarily set it false.
var BurnEnabled = true

// EmptyFixDeltaHash is the honest empty-input fix delta identity, mirroring
// gentle-ai: SHA-256 of zero bytes. It is frozen until a correction binds a
// real fix delta.
const EmptyFixDeltaHash = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// FixDeltaDomain is the fix-delta binding domain.
const FixDeltaDomain = "biggz-ai.fix-delta/v1"

// LegacyFixDeltaDomain is the pre-2026-08-28 domain. Legacy domain "fix-delta/v1" accepted for backward compat (pre-2026-08-28).
const LegacyFixDeltaDomain = "fix-delta/v1"

// FixDeltaHashForSnapshot returns EmptyFixDeltaHash when cumulative==0 else
// domainHash("biggz-ai.fix-delta/v1" + "\x00" + writeLengthPrefixed(baseTree,candidateTree,pathsDigest,cumulative,ledgerIDs...)).
// Verbatim gentle-ai fix-delta binding (cumulative + content-addressed trees).
func FixDeltaHashForSnapshot(baseTree, candidateTree, pathsDigest string, cumulative int, ledgerIDs []string) string {
	if cumulative <= 0 {
		return EmptyFixDeltaHash
	}
	ids, _ := canonicalStrings(ledgerIDs, "ledger id")
	fields := [][]byte{
		[]byte(baseTree),
		[]byte(candidateTree),
		[]byte(pathsDigest),
		[]byte(strconv.Itoa(cumulative)),
	}
	for _, id := range ids {
		fields = append(fields, []byte(id))
	}
	payload := writeLengthPrefixed(fields...)
	return domainHash(FixDeltaDomain, payload)
}

// computeFixDeltaHash is legacy wrapper kept for backward compat.
// It binds only cumulative via FixDeltaHashForSnapshot with empty trees.
func computeFixDeltaHash(cumulativeLines int) string {
	return FixDeltaHashForSnapshot("", "", "", cumulativeLines, nil)
}

// legacyFixDeltaHashForSnapshot computes the pre-2026-08-28 domain hash.
// Legacy domain "fix-delta/v1" accepted for backward compat (pre-2026-08-28).
func legacyFixDeltaHashForSnapshot(baseTree, candidateTree, pathsDigest string, cumulative int, ledgerIDs []string) string {
	if cumulative <= 0 {
		return EmptyFixDeltaHash
	}
	ids, _ := canonicalStrings(ledgerIDs, "ledger id")
	fields := [][]byte{
		[]byte(baseTree),
		[]byte(candidateTree),
		[]byte(pathsDigest),
		[]byte(strconv.Itoa(cumulative)),
	}
	for _, id := range ids {
		fields = append(fields, []byte(id))
	}
	payload := writeLengthPrefixed(fields...)
	return domainHash(LegacyFixDeltaDomain, payload)
}

// deriveCumulativeAndFixDelta derives cumulative correction lines and its delta hash
// from the chain: prior receipt cumulative plus post-finalize correction lines.
// It prefers an explicit "cumulative" field when present, falling back to
// summing "lines_changed" for backward compat. If no correction events exist,
// cumulative stays at the prior receipt value (Empty stays correct).
func deriveCumulativeAndFixDelta(chain ValidatedChain, store *Store) (int, string) {
	prior := priorCumulativeFromReceipt(chain, store)
	lastComplete := lastCompleteIndex(chain)
	post, explicitCumulative := scanPostCorrections(chain, lastComplete)
	cumulative := prior + post
	if explicitCumulative != nil {
		cumulative = *explicitCumulative
		if cumulative < prior {
			cumulative = prior
		}
	}
	if cumulative < 0 {
		cumulative = 0
	}
	return cumulative, computeFixDeltaHash(cumulative)
}

func priorCumulativeFromReceipt(chain ValidatedChain, store *Store) int {
	ref := receiptArtifactOf(chain)
	if ref == nil {
		return 0
	}
	receipt, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	if err != nil {
		return 0
	}
	return receipt.CumulativeCorrectionLines
}

func scanPostCorrections(chain ValidatedChain, lastComplete int) (int, *int) {
	if lastComplete < 0 {
		return 0, nil
	}
	post := 0
	var explicit *int
	for i := lastComplete + 1; i < len(chain.Records); i++ {
		rec := chain.Records[i]
		if rec.Operation != "correction" {
			continue
		}
		var payload struct {
			LinesChanged int  `json:"lines_changed"`
			Cumulative   *int `json:"cumulative"`
		}
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			continue
		}
		if payload.Cumulative != nil {
			explicit = payload.Cumulative
		} else if payload.LinesChanged > 0 {
			post += payload.LinesChanged
		}
	}
	return post, explicit
}

// ---------------------------------------------------------------------------
// Start plan
// ---------------------------------------------------------------------------

// StartEventPayload is the extended genesis payload frozen by `review start`.
// It keeps the ReviewSubject fields (repository, commit_sha) at the top level
// so legacy readers that unmarshal the genesis into model.ReviewSubject keep
// working, and adds the derived budget, the content-based risk tier, and the
// lens selection. SelectedLenses is the FROZEN lens plan: declared lenses win,
// otherwise PlanLenses derives it from the risk tier — so next_transition and
// finalize can rely on the selection without inferring it from captures later.
type StartEventPayload struct {
	Schema                string   `json:"schema,omitempty"`
	Repository            string   `json:"repository"`
	CommitSHA             string   `json:"commit_sha"`
	BaseRef               string   `json:"base_ref,omitempty"`
	OriginalChangedLines  int      `json:"original_changed_lines"`
	CorrectionBudget      int      `json:"correction_budget"`
	MaxCorrectionAttempts int      `json:"max_correction_attempts"`
	SelectedLenses        []string `json:"lenses,omitempty"`
	RiskTier              string   `json:"risk_tier,omitempty"`
	LensPlan              []string `json:"lens_plan,omitempty"`
}

// ParseSelectedLenses canonicalizes a comma-separated --lenses value: trim,
// validate every lens, sort, and deduplicate. An empty value selects nothing
// (PlanLenses then derives the frozen selection from the risk tier).
func ParseSelectedLenses(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	lenses := make([]string, 0, len(parts))
	for _, part := range parts {
		lens := strings.TrimSpace(part)
		if !isSupportedLens(lens) {
			return nil, fmt.Errorf("unsupported review lens %q", lens)
		}
		lenses = append(lenses, lens)
	}
	return canonicalStrings(lenses, "selected lens")
}

// FrozenBudgetInfo mirrors the frozen start plan for `review status` output.
type FrozenBudgetInfo struct {
	CorrectionLines      int `json:"correction_lines"`
	MaxAttempts          int `json:"max_attempts"`
	OriginalChangedLines int `json:"original_changed_lines"`
}

// frozenBudgetOf reads the frozen correction budget from the chain genesis.
// Legacy lineages started without a plan have no frozen budget and report nil.
func frozenBudgetOf(chain ValidatedChain) *FrozenBudgetInfo {
	if chain.Count == 0 {
		return nil
	}
	var plan StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil || plan.CorrectionBudget <= 0 {
		return nil
	}
	return &FrozenBudgetInfo{
		CorrectionLines: plan.CorrectionBudget, MaxAttempts: plan.MaxCorrectionAttempts,
		OriginalChangedLines: plan.OriginalChangedLines,
	}
}

// ---------------------------------------------------------------------------
// Correction budget derivation and enforcement
// ---------------------------------------------------------------------------

// DeriveCorrectionBudget freezes the maximum correction size from the
// original authored candidate, mirroring gentle-ai's CorrectionBudget with
// the floor_two policy (issue #2247): min(200, max(2, ceil(lines/2))).
// Every non-negative input admits at least two changed lines so a single-line
// replacement (one addition + one deletion) is always correctable.
func DeriveCorrectionBudget(originalChangedLines int) (int, error) {
	if originalChangedLines < 0 {
		return 0, errors.New("original changed lines cannot be negative")
	}
	ceil := (originalChangedLines + 1) / 2
	if ceil < 2 {
		ceil = 2
	}
	if ceil > CorrectionBudgetCap {
		ceil = CorrectionBudgetCap
	}
	return ceil, nil
}

// DeriveOriginalChangedLines computes the authored changed-line count of the
// subject commit against a base tree (additions + deletions via
// `git diff --numstat`). The base is the explicit baseRef tree when given;
// otherwise the subject commit's parent tree, falling back to git's empty
// tree for a root commit — the same base derivation candidateManifest uses.
// It returns the resolved base tree SHA and the line count.
func DeriveOriginalChangedLines(repo, commitSHA, baseRef string) (base string, lines int, err error) {
	repoArgs := func(args ...string) []string {
		if repo != "" {
			return append([]string{"-C", repo}, args...)
		}
		return args
	}
	// Legacy subjects (diff/files only, no commit SHA) bind to the current
	// HEAD: the candidate tree is HEAD's tree and the base is HEAD's parent.
	target := commitSHA
	if target == "" {
		target = "HEAD"
	}
	candidate, err := gitOutput(exec.Command("git", repoArgs("rev-parse", target+"^{tree}")...))
	if err != nil {
		return "", 0, wrapRuntimeCandidateUnavailable(fmt.Errorf("derive original changed lines: resolve candidate tree for %s: %w", commitSHA, err))
	}
	if strings.TrimSpace(candidate) == "" {
		return "", 0, wrapRuntimeCandidateUnavailable(fmt.Errorf("derive original changed lines: candidate tree for %s is empty", commitSHA))
	}
	if baseRef != "" {
		base, err = gitOutput(exec.Command("git", repoArgs("rev-parse", baseRef+"^{tree}")...))
		if err != nil {
			return "", 0, fmt.Errorf("derive original changed lines: resolve base tree for %s: %w", baseRef, err)
		}
	} else {
		base, err = gitOutput(exec.Command("git", repoArgs("rev-parse", target+"^^{tree}")...))
		if err != nil {
			base = emptyTreeSHA
		}
	}
	raw, err := gitOutput(exec.Command("git", repoArgs("diff", "--numstat", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--ignore-submodules=none", base, candidate, "--")...))
	if err != nil {
		return "", 0, fmt.Errorf("derive original changed lines: diff %s vs %s: %w", base, candidate, err)
	}
	lines, err = countNumstatLines(raw)
	if err != nil {
		return "", 0, fmt.Errorf("derive original changed lines: %w", err)
	}
	return base, lines, nil
}

// countNumstatLines sums additions and deletions from `git diff --numstat`
// output. Binary entries (dashes) count zero, mirroring gentle-ai's
// CountChangedLines.
func countNumstatLines(raw string) (int, error) {
	total := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "Binary files") {
			return 0, wrapRuntimeCandidateUnavailable(fmt.Errorf("binary files differ: %q", line))
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			return 0, fmt.Errorf("malformed numstat line %q", line)
		}
		if fields[0] == "-" || fields[1] == "-" {
			continue
		}
		additions, addErr := strconv.Atoi(fields[0])
		deletions, delErr := strconv.Atoi(fields[1])
		if addErr != nil || delErr != nil {
			return 0, fmt.Errorf("malformed numstat line %q", line)
		}
		total += additions + deletions
	}
	return total, nil
}

// ValidateCorrectionForecast rejects a correction forecast that exceeds the
// frozen correction budget. The error names the budget so a consumer can
// escalate exactly like gentle-ai's BeginCorrection over-budget escalation.
func ValidateCorrectionForecast(forecastLines, budget int) error {
	if forecastLines <= 0 {
		return fmt.Errorf("correction forecast must be a positive line count, got %d", forecastLines)
	}
	if forecastLines > budget {
		return fmt.Errorf("correction forecast of %d lines exceeds the frozen correction budget of %d lines; the lineage must escalate", forecastLines, budget)
	}
	return nil
}

// ValidateCorrectionActual re-validates the actual lines of a recorded
// correction against the frozen budget at completion, including cumulative
// accounting. Over-budget completion must escalate.
func ValidateCorrectionActual(actualLines, cumulativeLines, budget int) error {
	if actualLines < 0 || cumulativeLines < 0 {
		return errors.New("correction line counts cannot be negative")
	}
	if cumulativeLines+actualLines > budget {
		return fmt.Errorf("cumulative correction lines %d exceed the frozen correction budget of %d lines; the lineage must escalate",
			cumulativeLines+actualLines, budget)
	}
	return nil
}

// CorrectionAttemptConsumed reports whether the cumulative attempt accounting
// has exhausted the single compact correction attempt, mirroring gentle-ai's
// MaxCompactCorrectionAttempts = 1.
func CorrectionAttemptConsumed(cumulativeAttempts int) bool {
	return cumulativeAttempts >= MaxCompactCorrectionAttempts
}

// ResumeForecastGate validates a correction forecast against the lineage's
// frozen correction budget before a resume event appends.
func ResumeForecastGate(chain ValidatedChain, forecastLines int) error {
	if chain.Count == 0 {
		return errors.New("resume: lineage has no events")
	}
	var plan StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil || plan.CorrectionBudget <= 0 {
		return errors.New("resume: lineage has no frozen correction budget; start the review with the budget derivation enabled")
	}
	return ValidateCorrectionForecast(forecastLines, plan.CorrectionBudget)
}

// ---------------------------------------------------------------------------
// Persisted receipt artifact
// ---------------------------------------------------------------------------

// ReceiptLensSubject binds one captured lens slot in the receipt: the lens,
// its selected order, the provider-owned subject hash, the result hash, and
// the content-addressed manifest reference.
type ReceiptLensSubject struct {
	Lens          string `json:"lens"`
	SelectedOrder int    `json:"order"`
	SubjectHash   string `json:"subject_hash"`
	ResultHash    string `json:"result_hash"`
	ManifestPath  string `json:"manifest_path,omitempty"`
}

// PersistedReceipt is the full terminal receipt materialized by finalize
// under receipts/<sha256>.json. It mirrors gentle-ai's CompactReceipt field
// set adapted to biggz, plus genesis_revision and head_revision so the
// receipt hash can bind the whole lineage. PolicyHash stays empty until gates
// bind; FixDeltaHash stays at EmptyFixDeltaHash until a correction.
//
// Finding routing (machine-enforced refutation): resolved_finding_ids are the
// findings the refuter batch refuted — they no longer block; standing_finding_ids
// are the findings that stood — they stay blocking. Deterministic
// candidate-causal findings are auto-blocking and appear in NEITHER set (only
// a correction resolves them).
type PersistedReceipt struct {
	Schema                    string               `json:"schema"`
	LineageID                 string               `json:"lineage_id"`
	Generation                int                  `json:"generation"`
	GenesisRevision           string               `json:"genesis_revision"`
	HeadRevision              string               `json:"head_revision"`
	BaseTree                  string               `json:"base_tree"`
	InitialReviewTree         string               `json:"initial_review_tree"`
	FinalCandidateTree        string               `json:"final_candidate_tree"`
	PathsDigest               string               `json:"paths_digest"`
	FixDeltaHash              string               `json:"fix_delta_hash"`
	PolicyHash                string               `json:"policy_hash"`
	EvidenceHash              string               `json:"evidence_hash"`
	RiskTier                  string               `json:"risk_tier"`
	SelectedLenses            []string             `json:"selected_lenses"`
	LensSubjects              []ReceiptLensSubject `json:"lens_subjects"`
	ResolvedFindingIDs        []string             `json:"resolved_finding_ids"`
	StandingFindingIDs        []string             `json:"standing_finding_ids"`
	TerminalState             string               `json:"terminal_state"`
	CumulativeCorrectionLines int                  `json:"cumulative_correction_lines,omitempty"`
	ReceiptHash               string               `json:"receipt_hash"`
}

// computeHash derives the binding hash over every receipt field except the
// hash itself, so validation can recompute it from the persisted bytes.
func (r PersistedReceipt) computeHash() string {
	preimage := struct {
		Schema                    string               `json:"schema"`
		LineageID                 string               `json:"lineage_id"`
		Generation                int                  `json:"generation"`
		GenesisRevision           string               `json:"genesis_revision"`
		HeadRevision              string               `json:"head_revision"`
		BaseTree                  string               `json:"base_tree"`
		InitialReviewTree         string               `json:"initial_review_tree"`
		FinalCandidateTree        string               `json:"final_candidate_tree"`
		PathsDigest               string               `json:"paths_digest"`
		FixDeltaHash              string               `json:"fix_delta_hash"`
		PolicyHash                string               `json:"policy_hash"`
		EvidenceHash              string               `json:"evidence_hash"`
		RiskTier                  string               `json:"risk_tier"`
		SelectedLenses            []string             `json:"selected_lenses"`
		LensSubjects              []ReceiptLensSubject `json:"lens_subjects"`
		ResolvedFindingIDs        []string             `json:"resolved_finding_ids"`
		StandingFindingIDs        []string             `json:"standing_finding_ids"`
		TerminalState             string               `json:"terminal_state"`
		CumulativeCorrectionLines int                  `json:"cumulative_correction_lines"`
	}{
		Schema: r.Schema, LineageID: r.LineageID, Generation: r.Generation,
		GenesisRevision: r.GenesisRevision, HeadRevision: r.HeadRevision,
		BaseTree: r.BaseTree, InitialReviewTree: r.InitialReviewTree, FinalCandidateTree: r.FinalCandidateTree,
		PathsDigest: r.PathsDigest, FixDeltaHash: r.FixDeltaHash, PolicyHash: r.PolicyHash,
		EvidenceHash: r.EvidenceHash, RiskTier: r.RiskTier,
		SelectedLenses: r.SelectedLenses, LensSubjects: r.LensSubjects,
		ResolvedFindingIDs: r.ResolvedFindingIDs, StandingFindingIDs: r.StandingFindingIDs,
		TerminalState:             r.TerminalState,
		CumulativeCorrectionLines: r.CumulativeCorrectionLines,
	}
	payload, _ := json.Marshal(preimage)
	return domainHash(ReceiptBindingDomain, payload)
}

// computeLegacyHash is the pre-cumulative binding hash (without cumulative field)
// kept for backward compat with receipts written before fix-budget-accounting.
func (r PersistedReceipt) computeLegacyHash() string {
	preimage := struct {
		Schema             string               `json:"schema"`
		LineageID          string               `json:"lineage_id"`
		Generation         int                  `json:"generation"`
		GenesisRevision    string               `json:"genesis_revision"`
		HeadRevision       string               `json:"head_revision"`
		BaseTree           string               `json:"base_tree"`
		InitialReviewTree  string               `json:"initial_review_tree"`
		FinalCandidateTree string               `json:"final_candidate_tree"`
		PathsDigest        string               `json:"paths_digest"`
		FixDeltaHash       string               `json:"fix_delta_hash"`
		PolicyHash         string               `json:"policy_hash"`
		EvidenceHash       string               `json:"evidence_hash"`
		RiskTier           string               `json:"risk_tier"`
		SelectedLenses     []string             `json:"selected_lenses"`
		LensSubjects       []ReceiptLensSubject `json:"lens_subjects"`
		ResolvedFindingIDs []string             `json:"resolved_finding_ids"`
		StandingFindingIDs []string             `json:"standing_finding_ids"`
		TerminalState      string               `json:"terminal_state"`
	}{
		Schema: r.Schema, LineageID: r.LineageID, Generation: r.Generation,
		GenesisRevision: r.GenesisRevision, HeadRevision: r.HeadRevision,
		BaseTree: r.BaseTree, InitialReviewTree: r.InitialReviewTree, FinalCandidateTree: r.FinalCandidateTree,
		PathsDigest: r.PathsDigest, FixDeltaHash: r.FixDeltaHash, PolicyHash: r.PolicyHash,
		EvidenceHash: r.EvidenceHash, RiskTier: r.RiskTier,
		SelectedLenses: r.SelectedLenses, LensSubjects: r.LensSubjects,
		ResolvedFindingIDs: r.ResolvedFindingIDs, StandingFindingIDs: r.StandingFindingIDs,
		TerminalState: r.TerminalState,
	}
	payload, _ := json.Marshal(preimage)
	return domainHash(ReceiptBindingDomain, payload)
}

// Validate verifies the receipt's canonical shape and self-hash without
// consulting mutable repository state.
func (r PersistedReceipt) Validate() error {
	if err := r.validateReceiptIdentity(); err != nil {
		return err
	}
	if err := r.validateCumulativeLines(); err != nil {
		return err
	}
	if err := r.validateHashBinding(); err != nil {
		return err
	}
	if err := r.validateLensSelection(); err != nil {
		return err
	}
	if err := r.validateLensSubjects(); err != nil {
		return err
	}
	if err := r.validateFindingRouting(); err != nil {
		return err
	}
	if err := r.validateEvidenceChain(); err != nil {
		return err
	}
	return nil
}

func (r PersistedReceipt) validateReceiptIdentity() error {
	if r.Schema != ReviewReceiptSchema {
		return errors.New("receipt schema is unsupported")
	}
	if strings.TrimSpace(r.LineageID) == "" || strings.ContainsAny(r.LineageID, "\x00\r\n") {
		return errors.New("receipt lineage identity is incomplete")
	}
	if r.Generation < 1 {
		return errors.New("receipt generation must be positive")
	}
	if !validSHA256Hex(r.GenesisRevision) || !validSHA256Hex(r.HeadRevision) {
		return errors.New("receipt genesis and head revisions must be SHA-256 event revisions")
	}
	for _, tree := range []string{r.BaseTree, r.InitialReviewTree, r.FinalCandidateTree} {
		if !validCommitSHA(tree) {
			return errors.New("receipt tree identities are invalid")
		}
	}
	return nil
}

func (r PersistedReceipt) validateCumulativeLines() error {
	if r.CumulativeCorrectionLines < 0 {
		return errors.New("receipt cumulative correction lines cannot be negative")
	}
	if r.CumulativeCorrectionLines > 0 && r.FixDeltaHash != EmptyFixDeltaHash {
		flat := payloadSHA256([]byte(fmt.Sprintf("fix-delta:%d", r.CumulativeCorrectionLines)))
		if r.FixDeltaHash == flat {
			return errors.New("receipt fix-delta hash is flat; expected domain-bound hash")
		}
	}
	return nil
}

func (r PersistedReceipt) validateHashBinding() error {
	for _, identity := range []string{r.PathsDigest, r.FixDeltaHash, r.EvidenceHash} {
		if !validSHA256Identity(identity) {
			return errors.New("receipt paths, fix-delta, and evidence hashes are invalid")
		}
	}
	if r.PolicyHash != "" && !validSHA256Identity(r.PolicyHash) {
		return errors.New("receipt policy hash is invalid")
	}
	return nil
}

func (r PersistedReceipt) validateLensSelection() error {
	lenses, err := canonicalStrings(r.SelectedLenses, "selected lens")
	if err != nil || !equalStrings(lenses, r.SelectedLenses) {
		return errors.New("receipt selected lenses must be canonical")
	}
	if len(lenses) != len(r.LensSubjects) {
		return errors.New("receipt lens subjects must cover every selected lens exactly once")
	}
	if !validRiskTier(r.RiskTier) {
		return fmt.Errorf("receipt risk tier %q is unsupported", r.RiskTier)
	}
	if r.TerminalState != ReviewReceiptTerminalState {
		return fmt.Errorf("receipt terminal state %q is unsupported", r.TerminalState)
	}
	return nil
}

func (r PersistedReceipt) validateLensSubjects() error {
	lenses, err := canonicalStrings(r.SelectedLenses, "selected lens")
	if err != nil || !equalStrings(lenses, r.SelectedLenses) {
		return errors.New("receipt selected lenses must be canonical")
	}
	seen := make(map[string]struct{}, len(r.LensSubjects))
	for index, subject := range r.LensSubjects {
		if !isSupportedLens(subject.Lens) || !containsString(lenses, subject.Lens) {
			return fmt.Errorf("receipt lens subject[%d] does not bind a selected lens", index)
		}
		if subject.SelectedOrder < 0 {
			return fmt.Errorf("receipt lens subject[%d] order is invalid", index)
		}
		if !validSHA256Identity(subject.SubjectHash) || !validSHA256Identity(subject.ResultHash) {
			return fmt.Errorf("receipt lens subject[%d] hashes are invalid", index)
		}
		if _, duplicate := seen[subject.Lens]; duplicate {
			return fmt.Errorf("receipt lens subject %q is recorded twice", subject.Lens)
		}
		seen[subject.Lens] = struct{}{}
		if index > 0 && !isLensSubjectsOrdered(r.LensSubjects[index-1], subject) {
			return errors.New("receipt lens subjects must be canonically ordered by order then lens")
		}
	}
	return nil
}

func isLensSubjectsOrdered(prev, curr ReceiptLensSubject) bool {
	if curr.SelectedOrder < prev.SelectedOrder {
		return false
	}
	if curr.SelectedOrder == prev.SelectedOrder && curr.Lens <= prev.Lens {
		return false
	}
	return true
}

func (r PersistedReceipt) validateFindingRouting() error {
	ids, err := canonicalStrings(r.ResolvedFindingIDs, "resolved finding id")
	if err != nil || !equalStrings(ids, r.ResolvedFindingIDs) {
		return errors.New("receipt resolved finding IDs must be canonical")
	}
	standing, err := canonicalStrings(r.StandingFindingIDs, "standing finding id")
	if err != nil || !equalStrings(standing, r.StandingFindingIDs) {
		return errors.New("receipt standing finding IDs must be canonical")
	}
	for _, id := range r.ResolvedFindingIDs {
		if containsString(r.StandingFindingIDs, id) {
			return fmt.Errorf("receipt finding %q cannot be both resolved and standing", id)
		}
	}
	return nil
}

func (r PersistedReceipt) validateEvidenceChain() error {
	if !validSHA256Identity(r.ReceiptHash) {
		return errors.New("receipt hash does not match the receipt binding")
	}
	computed := r.computeHash()
	if r.ReceiptHash == computed {
		return nil
	}
	if r.CumulativeCorrectionLines == 0 && r.ReceiptHash == r.computeLegacyHash() {
		return nil
	}
	return errors.New("receipt hash does not match the receipt binding")
}

// validRiskTier reports whether the tier is one of the derivation outputs.
func validRiskTier(tier string) bool {
	switch tier {
	case "low", "medium", "high":
		return true
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// deriveRiskTier is the legacy lens-count tier proxy, kept only as a fallback
// for lineages frozen before the content-based classifier (their start plan
// has no risk_tier): no lenses → low, one to three → medium, four or more →
// high. New lineages carry the classifier tier in the frozen start plan and
// never reach this fallback.
func deriveRiskTier(lensCount int) string {
	switch {
	case lensCount >= 4:
		return "high"
	case lensCount >= 1:
		return "medium"
	default:
		return "low"
	}
}

// ReceiptArtifactRef names the persisted receipt artifact for status output.
type ReceiptArtifactRef struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// receiptArtifactOf reads the persisted receipt reference from the terminal
// complete_review event, if any. The scan walks the whole chain from the end:
// a lineage resumed after finalize still carries its terminal receipt, and new
// events after it are exactly what the gate must surface as uncovered.
func receiptArtifactOf(chain ValidatedChain) *ReceiptArtifactRef {
	for index := len(chain.Records) - 1; index >= 0; index-- {
		rec := &chain.Records[index]
		if rec.Operation != CompleteReviewOperation {
			continue
		}
		var evt completeEventPayload
		if err := json.Unmarshal(rec.Payload, &evt); err != nil || evt.ReceiptPath == "" {
			return nil
		}
		return &ReceiptArtifactRef{Path: evt.ReceiptPath, Hash: evt.ReceiptHash}
	}
	return nil
}

// completeEventPayload is the durable complete_review event payload: a
// reference to the persisted receipt artifact. The receipt itself binds the
// pre-finalize head, so the event and the receipt are mutually non-circular.
type completeEventPayload struct {
	Schema      string `json:"schema"`
	ReceiptPath string `json:"receipt_path"`
	ReceiptHash string `json:"receipt_hash"`
}

// FinalizeOutcome describes a terminal finalize: the persisted receipt
// artifact reference and the appended complete_review revision. Idempotent is
// true when the lineage was already finalized and nothing was appended.
type FinalizeOutcome struct {
	LineageID   string `json:"lineage_id"`
	ReceiptPath string `json:"receipt_path"`
	ReceiptHash string `json:"receipt_hash"`
	Revision    string `json:"revision"`
	Idempotent  bool   `json:"idempotent"`
}

// finalizeSlot is one captured lens_result event together with its parsed
// payload, ordered by (selected order, lens).
type finalizeSlot struct {
	payload lensResultEventPayload
}

// finalizeData is everything derived from the chain + repository that the
// receipt binds: the frozen candidate trees and manifest, the re-verified
// captured slots, and the aggregate evidence and finding sets. resolvedIDs
// are the refuted findings; standingIDs are the findings that stood the
// refuter batch and remain blocking.
type finalizeData struct {
	baseTree                  string
	candidateTree             string
	manifestDigest            string
	slots                     []finalizeSlot
	evidenceHash              string
	resolvedIDs               []string
	standingIDs               []string
	riskTier                  string
	cumulativeCorrectionLines int
	fixDeltaHash              string
}

// Finalize runs the terminal transition for a lineage: every selected lens
// slot must be captured, the captured canonical payloads are read back from
// the events for re-verification, a complete_review event is appended, and
// the receipt is materialized under receipts/<sha256>.json. A second finalize
// on an unchanged lineage is idempotent: it returns the same receipt without
// appending anything.
func Finalize(repo, lineageID string) (FinalizeOutcome, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return FinalizeOutcome{}, fmt.Errorf("finalize: open store: %w", err)
	}
	var outcome FinalizeOutcome
	err = WithFileLock(store.Dir, func() error {
		chain, revisions, plan, err := loadAndValidateChain(store)
		if err != nil {
			return err
		}
		last := chain.Records[chain.Count-1]
		if last.Operation == CompleteReviewOperation {
			return finalizeIdempotent(store, repo, chain, revisions, plan, &outcome)
		}
		if last.Operation == BurnOperation {
			return fmt.Errorf("finalize: %w", ErrAlreadyBurned)
		}
		_, receipt, path, err := deriveAndPublishSnapshot(repo, &chain, plan, store, revisions)
		if err != nil {
			return err
		}
		revision, err := burnAndGateDelivery(store, chain.HeadHash, receipt, path)
		if err != nil {
			return err
		}
		outcome = FinalizeOutcome{
			LineageID: lineageID, ReceiptPath: path, ReceiptHash: receipt.ReceiptHash, Revision: revision,
		}
		return enqueueSyncIfNeeded(store, chain)
	})
	return outcome, err
}

func loadAndValidateChain(store *Store) (ValidatedChain, []string, StartEventPayload, error) {
	chain, err := store.LoadChain()
	if err != nil {
		return ValidatedChain{}, nil, StartEventPayload{}, fmt.Errorf("finalize: load chain: %w", err)
	}
	if chain.Count == 0 {
		return ValidatedChain{}, nil, StartEventPayload{}, errors.New("finalize: lineage has no events")
	}
	if IsChainBurned(chain) {
		return ValidatedChain{}, nil, StartEventPayload{}, fmt.Errorf("finalize: %w", ErrAlreadyBurned)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, BurnedMarkerFile)); err == nil {
		return ValidatedChain{}, nil, StartEventPayload{}, fmt.Errorf("finalize: %w", ErrAlreadyBurned)
	}
	if state, reason := terminatedStateOf(chain); state != "" {
		if state == "invalidated" && reason != "" {
			return ValidatedChain{}, nil, StartEventPayload{}, fmt.Errorf("finalize: lineage is invalidated: %s", reason)
		}
		return ValidatedChain{}, nil, StartEventPayload{}, fmt.Errorf("finalize: lineage is %s", state)
	}
	verdict := store.Validate()
	if !verdict.Valid {
		return ValidatedChain{}, nil, StartEventPayload{}, fmt.Errorf("finalize: chain integrity failed: %s", verdict.Reason)
	}
	revisions := recordRevisions(chain)
	genesis := chain.Records[0]
	if genesis.Operation != "start_review" {
		return ValidatedChain{}, nil, StartEventPayload{}, errors.New("finalize: lineage genesis is not a review start")
	}
	var plan StartEventPayload
	if err := json.Unmarshal(genesis.Payload, &plan); err != nil || strings.TrimSpace(plan.CommitSHA) == "" {
		return ValidatedChain{}, nil, StartEventPayload{}, errors.New("finalize: genesis event does not carry a review subject")
	}
	return chain, revisions, plan, nil
}

func deriveAndPublishSnapshot(repo string, chain *ValidatedChain, plan StartEventPayload, store *Store, revisions []string) (finalizeData, PersistedReceipt, string, error) {
	data, err := deriveFinalizeData(repo, *chain, plan)
	if err != nil {
		return finalizeData{}, PersistedReceipt{}, "", err
	}
	cum, _ := deriveCumulativeAndFixDelta(*chain, store)
	priorCum := 0
	if ref := receiptArtifactOf(*chain); ref != nil {
		if r, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash}); err == nil {
			priorCum = r.CumulativeCorrectionLines
		}
	}
	if err := ensureCorrectionEmitted(store, chain, cum, priorCum); err != nil {
		return finalizeData{}, PersistedReceipt{}, "", err
	}
	data.cumulativeCorrectionLines = cum
	data.fixDeltaHash = FixDeltaHashForSnapshot(data.baseTree, data.candidateTree, data.manifestDigest, cum, nil)
	receipt := buildReceipt(chain.LineageID, revisions[0], chain.HeadHash, data)
	if err := receipt.Validate(); err != nil {
		return finalizeData{}, PersistedReceipt{}, "", fmt.Errorf("finalize: receipt validation failed: %w", err)
	}
	path, err := writeReceiptLocked(store, receipt)
	if err != nil {
		return finalizeData{}, PersistedReceipt{}, "", fmt.Errorf("finalize: persist receipt: %w", err)
	}
	return data, receipt, path, nil
}

func ensureCorrectionEmitted(store *Store, chain *ValidatedChain, cum, priorCum int) error {
	if cum <= priorCum {
		return nil
	}
	if !shouldEmitCorrection(*chain, cum) {
		return nil
	}
	payload, _ := json.Marshal(map[string]int{"lines_changed": cum - priorCum, "cumulative": cum})
	newRev, err := store.appendLocked(chain.HeadHash, Record{
		Operation: "correction",
		Role:      string(model.RoleLead),
		Actor:     string(model.RoleLead),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Payload:   payload,
	})
	if err != nil {
		return fmt.Errorf("finalize: emit correction event: %w", err)
	}
	chain.Records = append(chain.Records, Record{Operation: "correction", Payload: payload})
	chain.HeadHash = newRev
	chain.Count++
	return nil
}

func shouldEmitCorrection(chain ValidatedChain, cum int) bool {
	lastCompleteIdx := lastCompleteIndex(chain)
	if lastCompleteIdx < 0 {
		return false
	}
	for i := len(chain.Records) - 1; i > lastCompleteIdx; i-- {
		if chain.Records[i].Operation != "correction" {
			continue
		}
		var p struct {
			Cumulative *int `json:"cumulative"`
		}
		if err := json.Unmarshal(chain.Records[i].Payload, &p); err == nil && p.Cumulative != nil && *p.Cumulative == cum {
			return false
		}
		break
	}
	return true
}

func burnAndGateDelivery(store *Store, chainHead string, receipt PersistedReceipt, receiptPath string) (string, error) {
	eventPayload, err := json.Marshal(completeEventPayload{
		Schema: FinalizeEventSchema, ReceiptPath: receiptPath, ReceiptHash: receipt.ReceiptHash,
	})
	if err != nil {
		return "", fmt.Errorf("finalize: marshal complete event: %w", err)
	}
	revision, err := store.appendLocked(chainHead, Record{
		Operation: CompleteReviewOperation,
		Role:      string(model.RoleLead),
		Actor:     string(model.RoleLead),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Payload:   eventPayload,
	})
	if err != nil {
		return "", fmt.Errorf("finalize: append complete_review event: %w", err)
	}
	if BurnEnabled {
		if err := burnReceiptLocked(store, receipt, receiptPath, revision); err != nil {
			return "", fmt.Errorf("finalize: burn receipt: %w", err)
		}
	}
	return revision, nil
}

func enqueueSyncIfNeeded(store *Store, chain ValidatedChain) error {
	return nil
}

// burnEventPayload is the durable burn_review event payload.
type burnEventPayload struct {
	Schema      string `json:"schema"`
	ReceiptHash string `json:"receipt_hash"`
	ReceiptPath string `json:"receipt_path"`
}

// burnReceiptLocked burns the persisted receipt: appends a burn_review event,
// writes the burned marker, and deletes the receipt file so it becomes
// ephemeral. The caller holds the lineage file lock.
func burnReceiptLocked(store *Store, receipt PersistedReceipt, receiptPath, completeRevision string) error {
	payload, err := json.Marshal(burnEventPayload{
		Schema: BurnEventSchema, ReceiptHash: receipt.ReceiptHash, ReceiptPath: receiptPath,
	})
	if err != nil {
		return fmt.Errorf("marshal burn event: %w", err)
	}
	if _, err := store.appendLocked(completeRevision, Record{
		Operation: BurnOperation,
		Role:      string(model.RoleLead),
		Actor:     string(model.RoleLead),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Payload:   payload,
	}); err != nil {
		return fmt.Errorf("append burn_review event: %w", err)
	}
	markerPath := filepath.Join(store.Dir, BurnedMarkerFile)
	markerPayload, _ := json.Marshal(map[string]string{
		"receipt_hash": receipt.ReceiptHash,
		"receipt_path": receiptPath,
		"burned_at":    time.Now().Format(time.RFC3339Nano),
	})
	if err := os.WriteFile(markerPath, markerPayload, 0644); err != nil {
		return fmt.Errorf("write burned marker: %w", err)
	}
	// Delete the receipt file to make it ephemeral. Genealogical reference
	// remains in the complete_review event, but the artifact is gone.
	fullPath := filepath.Join(store.Dir, receiptPath)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove receipt file: %w", err)
	}
	return nil
}

// finalizeIdempotent handles a second finalize on an already-terminal
// lineage: the receipt is re-derived from the chain and compared with the
// persisted artifact. Nothing is appended.
func finalizeIdempotent(store *Store, repo string, chain ValidatedChain, revisions []string, plan StartEventPayload, outcome *FinalizeOutcome) error {
	last := chain.Records[chain.Count-1]
	var evt completeEventPayload
	if err := json.Unmarshal(last.Payload, &evt); err != nil || evt.ReceiptPath == "" || !validSHA256Identity(evt.ReceiptHash) {
		return errors.New("finalize: lineage is already completed but carries no persisted receipt artifact")
	}
	stored, err := readReceiptFile(store, evt)
	if err != nil {
		return fmt.Errorf("finalize: persisted receipt artifact is unreadable: %w", err)
	}
	if chain.Count < 2 {
		return errors.New("finalize: completed lineage lacks a captured review head")
	}
	data, err := deriveFinalizeData(repo, chain, plan)
	if err != nil {
		return err
	}
	// Use stored cumulative so idempotent recomputation is hash-identical.
	data.cumulativeCorrectionLines = stored.CumulativeCorrectionLines
	data.fixDeltaHash = stored.FixDeltaHash
	receipt := buildReceipt(chain.LineageID, revisions[0], revisions[chain.Count-2], data)
	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("finalize: receipt validation failed: %w", err)
	}
	if !reflect.DeepEqual(stored, receipt) {
		return errors.New("finalize: persisted receipt does not match the current lineage state")
	}
	*outcome = FinalizeOutcome{
		LineageID: chain.LineageID, ReceiptPath: evt.ReceiptPath,
		ReceiptHash: evt.ReceiptHash, Revision: chain.HeadHash, Idempotent: true,
	}
	return nil
}

// deriveFinalizeData assembles and re-verifies the completed review from the
// captured canonical payloads: selection parity, repository-derived trees and
// manifest, per-slot subject/hash re-verification, and the aggregate evidence
// and resolved-finding sets.
func deriveFinalizeData(repo string, chain ValidatedChain, plan StartEventPayload) (finalizeData, error) {
	slots, capturedNames, err := deriveHeader(chain, plan)
	if err != nil {
		return finalizeData{}, err
	}
	return deriveBody(repo, chain, plan, slots, capturedNames)
}

func deriveHeader(chain ValidatedChain, plan StartEventPayload) ([]finalizeSlot, []string, error) {
	slots, err := collectFinalizeSlots(chain)
	if err != nil {
		return nil, nil, err
	}
	capturedNames := sortedUniqueLensNames(slots)
	if err := validateFinalizeSelection(plan, capturedNames, chain); err != nil {
		return nil, nil, err
	}
	return slots, capturedNames, nil
}

func deriveBody(repo string, chain ValidatedChain, plan StartEventPayload, slots []finalizeSlot, capturedNames []string) (finalizeData, error) {
	baseTree, candidateTree, manifestDigest, err := deriveFinalizeManifest(repo, plan)
	if err != nil {
		return finalizeData{}, err
	}
	if err := verifyFinalizeSlots(slots, chain, plan, baseTree, candidateTree, manifestDigest); err != nil {
		return finalizeData{}, err
	}
	evidenceHash, resolved, standing, err := collectFinalizeEvidence(slots, chain)
	if err != nil {
		return finalizeData{}, err
	}
	riskTier := plan.RiskTier
	if riskTier == "" {
		riskTier = deriveRiskTier(len(capturedNames))
	}
	return finalizeData{
		baseTree: baseTree, candidateTree: candidateTree, manifestDigest: manifestDigest,
		slots: slots, evidenceHash: evidenceHash, resolvedIDs: resolved,
		standingIDs: standing, riskTier: riskTier,
		cumulativeCorrectionLines: 0,
		fixDeltaHash:              EmptyFixDeltaHash,
	}, nil
}

func collectFinalizeSlots(chain ValidatedChain) ([]finalizeSlot, error) {
	slots := make([]finalizeSlot, 0)
	for index := range chain.Records {
		rec := &chain.Records[index]
		if rec.Operation != LensResultOperation {
			continue
		}
		var payload lensResultEventPayload
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return nil, fmt.Errorf("finalize: lens result event %d is malformed: %w", index, err)
		}
		if payload.AdmissionDecision != AdmissionCompleted {
			return nil, fmt.Errorf("finalize: lens slot %q order %d was not captured (admission %s); all selected lens slots must be captured before finalize", payload.Lens, payload.SelectedOrder, payload.AdmissionDecision)
		}
		if isSlotSuperseded(chain, index, payload.Lens, payload.SelectedOrder) {
			continue
		}
		slots = append(slots, finalizeSlot{payload: payload})
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].payload.SelectedOrder != slots[j].payload.SelectedOrder {
			return slots[i].payload.SelectedOrder < slots[j].payload.SelectedOrder
		}
		return slots[i].payload.Lens < slots[j].payload.Lens
	})
	return slots, nil
}

func validateFinalizeSelection(plan StartEventPayload, capturedNames []string, chain ValidatedChain) error {
	declared, err := canonicalStrings(plan.SelectedLenses, "selected lens")
	if err != nil {
		return fmt.Errorf("finalize: frozen lens selection is not canonical: %w", err)
	}
	if len(declared) > 0 {
		disposed, err := disposedSlots(chain)
		if err != nil {
			return fmt.Errorf("finalize: slot disposition state: %w", err)
		}
		for _, slot := range disposed {
			if containsString(declared, slot.Lens) {
				return fmt.Errorf("finalize: lens slot %q order %d is disposed and not re-captured; run 'biggz review capture-result' for that slot again before finalize", slot.Lens, slot.Order)
			}
		}
		if equalStrings(declared, capturedNames) {
			return nil
		}
		missing := stringDifference(declared, capturedNames)
		if len(missing) > 0 {
			return fmt.Errorf("finalize: missing captured lens slot(s): %s; capture every selected lens before finalize", strings.Join(missing, ", "))
		}
		return fmt.Errorf("finalize: captured lens slot(s) outside the frozen selection: %s", strings.Join(stringDifference(capturedNames, declared), ", "))
	}
	if len(capturedNames) == 0 {
		return errors.New("finalize: no captured lens slots; nothing to finalize")
	}
	return nil
}

func deriveFinalizeManifest(repo string, plan StartEventPayload) (string, string, string, error) {
	baseTree, candidateTree, entries, err := candidateManifest(repo, plan.CommitSHA)
	if err != nil {
		return "", "", "", fmt.Errorf("finalize: derive candidate manifest: %w", err)
	}
	manifestDigest, err := ChangedPathManifestDigest(entries)
	if err != nil {
		return "", "", "", fmt.Errorf("finalize: manifest digest: %w", err)
	}
	return baseTree, candidateTree, manifestDigest, nil
}

func verifyFinalizeSlots(slots []finalizeSlot, chain ValidatedChain, plan StartEventPayload, baseTree, candidateTree, manifestDigest string) error {
	for _, slot := range slots {
		if err := verifySingleFinalizeSlot(slot, chain, plan, baseTree, candidateTree, manifestDigest); err != nil {
			return err
		}
	}
	return nil
}

func verifySingleFinalizeSlot(slot finalizeSlot, chain ValidatedChain, plan StartEventPayload, baseTree, candidateTree, manifestDigest string) error {
	payload := slot.payload
	want, err := NewArtifactSubject(chain.LineageID, payload.ExpectedRevision, plan.CommitSHA,
		baseTree, candidateTree, manifestDigest, payload.Lens, payload.SelectedOrder)
	if err != nil {
		return fmt.Errorf("finalize: re-derive artifact subject for lens %q order %d: %w", payload.Lens, payload.SelectedOrder, err)
	}
	if want.SubjectHash != payload.SubjectHash {
		return fmt.Errorf("finalize: lens %q order %d subject hash does not match the frozen candidate binding", payload.Lens, payload.SelectedOrder)
	}
	if payload.ManifestSHA256 != manifestDigest {
		return fmt.Errorf("finalize: lens %q order %d manifest digest does not match the frozen candidate", payload.Lens, payload.SelectedOrder)
	}
	if payloadSHA256(payload.CanonicalPayload) != payload.CanonicalPayloadSHA256 {
		return fmt.Errorf("finalize: lens %q order %d canonical payload hash mismatch", payload.Lens, payload.SelectedOrder)
	}
	if LensResultHash(payload.Result) != payload.ResultHash {
		return fmt.Errorf("finalize: lens %q order %d result hash mismatch", payload.Lens, payload.SelectedOrder)
	}
	return nil
}

func collectFinalizeEvidence(slots []finalizeSlot, chain ValidatedChain) (string, []string, []string, error) {
	evidenceHash, err := reviewEvidenceHash(slots)
	if err != nil {
		return "", nil, nil, err
	}
	refState, err := collectRefutationState(chain)
	if err != nil {
		return "", nil, nil, fmt.Errorf("finalize: refutation state: %w", err)
	}
	if refState.batches > 1 {
		return "", nil, nil, fmt.Errorf("finalize: lineage carries %d refutation batches; exactly one read-only refuter batch per review", refState.batches)
	}
	if err := validateSevereFindings(slots); err != nil {
		return "", nil, nil, err
	}
	if err := validateRefutationCoverage(refState, chain); err != nil {
		return "", nil, nil, err
	}
	resolved, err := canonicalStrings(refState.refuted, "resolved finding id")
	if err != nil {
		return "", nil, nil, fmt.Errorf("finalize: refuted finding IDs: %w", err)
	}
	standing, err := canonicalStrings(refState.stands, "standing finding id")
	if err != nil {
		return "", nil, nil, fmt.Errorf("finalize: standing finding IDs: %w", err)
	}
	return evidenceHash, resolved, standing, nil
}

func validateSevereFindings(slots []finalizeSlot) error {
	for _, slot := range slots {
		for _, finding := range slot.payload.Result.Findings {
			if !isSevereSeverity(finding.Severity) {
				continue
			}
			switch finding.CausalDisposition {
			case CausalPreExisting, CausalBaseOnly:
				continue
			case CausalUnknown:
				return fmt.Errorf("finalize: finding [%s] has unknown causal disposition; the lineage must escalate — re-capture the lens before finalize (refutation cannot cover it)", finding.ID)
			}
			if finding.EvidenceClass == EvidenceInsufficient {
				return fmt.Errorf("finalize: finding [%s] has insufficient evidence class; the lineage must escalate — re-capture the lens before finalize (refutation cannot cover it)", finding.ID)
			}
		}
	}
	return nil
}

func validateRefutationCoverage(refState refutationState, chain ValidatedChain) error {
	if len(refState.requirements) == 0 {
		return nil
	}
	covered := make(map[string]struct{}, len(refState.verdicts))
	for _, verdict := range refState.verdicts {
		covered[verdict.FindingID] = struct{}{}
	}
	pending := stringDifference(refState.requirements, sortedSetKeys(covered))
	if len(pending) == 0 {
		return nil
	}
	return fmt.Errorf("finalize: refutation pending for finding(s): %s; run 'biggz review refute %s --input -' to register the refuter batch before finalize", strings.Join(pending, ", "), chain.LineageID)
}

// buildReceipt assembles the terminal receipt from the derived finalize data.
// SelectedLenses are canonical (sorted, unique); LensSubjects keep the slot
// order (selected order, then lens).
func buildReceipt(lineageID, genesisRevision, headRevision string, data finalizeData) PersistedReceipt {
	subjects := make([]ReceiptLensSubject, 0, len(data.slots))
	for _, slot := range data.slots {
		subjects = append(subjects, ReceiptLensSubject{
			Lens: slot.payload.Lens, SelectedOrder: slot.payload.SelectedOrder,
			SubjectHash: slot.payload.SubjectHash, ResultHash: slot.payload.ResultHash,
			ManifestPath: slot.payload.ManifestPath,
		})
	}
	fixHash := data.fixDeltaHash
	if fixHash == "" {
		fixHash = FixDeltaHashForSnapshot(data.baseTree, data.candidateTree, data.manifestDigest, data.cumulativeCorrectionLines, nil)
	}
	receipt := PersistedReceipt{
		Schema: ReviewReceiptSchema, LineageID: lineageID, Generation: 1,
		GenesisRevision: genesisRevision, HeadRevision: headRevision,
		BaseTree: data.baseTree, InitialReviewTree: data.candidateTree, FinalCandidateTree: data.candidateTree,
		PathsDigest: data.manifestDigest, FixDeltaHash: fixHash,
		EvidenceHash: data.evidenceHash, RiskTier: data.riskTier,
		SelectedLenses: sortedUniqueLensNames(data.slots), LensSubjects: subjects,
		ResolvedFindingIDs: data.resolvedIDs, StandingFindingIDs: data.standingIDs,
		TerminalState:             ReviewReceiptTerminalState,
		CumulativeCorrectionLines: data.cumulativeCorrectionLines,
	}
	receipt.ReceiptHash = receipt.computeHash()
	return receipt
}

// reviewEvidenceHash binds the complete set of captured canonical payloads.
func reviewEvidenceHash(slots []finalizeSlot) (string, error) {
	type slotPreimage struct {
		Lens             string          `json:"lens"`
		SelectedOrder    int             `json:"order"`
		CanonicalSHA256  string          `json:"canonical_sha256"`
		ResultHash       string          `json:"result_hash"`
		CanonicalPayload json.RawMessage `json:"canonical_payload"`
	}
	preimage := struct {
		Schema string         `json:"schema"`
		Slots  []slotPreimage `json:"slots"`
	}{Schema: ReviewEvidenceDomain, Slots: make([]slotPreimage, 0, len(slots))}
	for _, slot := range slots {
		preimage.Slots = append(preimage.Slots, slotPreimage{
			Lens: slot.payload.Lens, SelectedOrder: slot.payload.SelectedOrder,
			CanonicalSHA256: slot.payload.CanonicalPayloadSHA256, ResultHash: slot.payload.ResultHash,
			CanonicalPayload: slot.payload.CanonicalPayload,
		})
	}
	payload, err := json.Marshal(preimage)
	if err != nil {
		return "", fmt.Errorf("finalize: marshal evidence: %w", err)
	}
	return domainHash(ReviewEvidenceDomain, payload), nil
}

// sortedUniqueLensNames returns the deduplicated, sorted lens names of the
// captured slots.
func sortedUniqueLensNames(slots []finalizeSlot) []string {
	names := make(map[string]struct{}, len(slots))
	for _, slot := range slots {
		names[slot.payload.Lens] = struct{}{}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	return sorted
}

func stringDifference(left, right []string) []string {
	excluded := make(map[string]struct{}, len(right))
	for _, value := range right {
		excluded[value] = struct{}{}
	}
	difference := make([]string, 0)
	for _, value := range left {
		if _, ok := excluded[value]; !ok {
			difference = append(difference, value)
		}
	}
	return difference
}

// recordRevisions returns each record's content-hash revision, oldest first.
func recordRevisions(chain ValidatedChain) []string {
	if chain.Count == 0 {
		return nil
	}
	revisions := make([]string, chain.Count)
	hash := chain.HeadHash
	for index := chain.Count - 1; index >= 0; index-- {
		revisions[index] = hash
		hash = chain.Records[index].PrevRevision
	}
	return revisions
}

// writeReceiptLocked persists the content-addressed receipt under
// <lineage>/receipts/<sha256>.json. The caller holds the lineage file lock.
// Same content is a no-op; different content is an error.
func writeReceiptLocked(store *Store, receipt PersistedReceipt) (string, error) {
	payload, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", err
	}
	digest := sha256Hex(payload)
	path := filepath.Join(store.Dir, ReceiptsDirName, digest+".json")
	if err := publishNoReplace(path, payload); err != nil {
		return "", err
	}
	return filepath.Join(ReceiptsDirName, digest+".json"), nil
}

// readReceiptFile loads a persisted receipt artifact and verifies its
// content-address, schema, and recorded hash.
func readReceiptFile(store *Store, evt completeEventPayload) (PersistedReceipt, error) {
	if filepath.Dir(evt.ReceiptPath) != ReceiptsDirName || !strings.HasSuffix(evt.ReceiptPath, ".json") {
		return PersistedReceipt{}, fmt.Errorf("receipt path %q is not under receipts/", evt.ReceiptPath)
	}
	payload, err := os.ReadFile(filepath.Join(store.Dir, evt.ReceiptPath))
	if err != nil {
		return PersistedReceipt{}, err
	}
	name := strings.TrimSuffix(filepath.Base(evt.ReceiptPath), ".json")
	if !validSHA256Hex(name) || sha256Hex(payload) != name {
		return PersistedReceipt{}, fmt.Errorf("receipt artifact name %q does not match its content hash", name)
	}
	var receipt PersistedReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return PersistedReceipt{}, fmt.Errorf("parse receipt artifact: %w", err)
	}
	// Legacy compat: missing cumulativeLines decodes as 0 (zero value); missing fix_delta_hash as "" → normalize to Empty.
	if receipt.FixDeltaHash == "" {
		receipt.FixDeltaHash = EmptyFixDeltaHash
	}
	// Recompute hash binding only after normalization: legacy receipts written before
	// CumulativeCorrectionLines existed have preimage without that field (0); our new
	// computeHash includes it as 0, so recomputed hash matches after normalization.
	if err := receipt.Validate(); err != nil {
		return PersistedReceipt{}, fmt.Errorf("validate receipt artifact: %w", err)
	}
	if receipt.ReceiptHash != evt.ReceiptHash {
		return PersistedReceipt{}, errors.New("receipt artifact hash does not match the complete_review event reference")
	}
	return receipt, nil
}
