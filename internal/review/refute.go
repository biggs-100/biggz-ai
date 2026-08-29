// Refuter batch registration — machine-enforced per-finding refutation of
// severe candidate-causal findings before finalize.
//
// This is biggz-ai's port of gentle-ai's refutation routing
// (internal/reviewtransaction/transaction.go ClassifyEvidence +
// ApplyRefuterOutcomes), adapted to the content-addressed event store:
//
//   - Deterministic findings need no refuter: they are auto-blocking and can
//     never be refuted (only a correction resolves them).
//   - Inferential findings with a candidate-causal disposition (introduced,
//     behavior-activated, worsened) go through exactly ONE read-only refuter
//     batch: `biggz review refute <lineage> --input -` registers the batch
//     as a `refutation` event. Verdicts are per-finding: `refuted` joins the
//     receipt's resolved set (no longer blocking); `stands` stays blocking
//     and is recorded in `standing_finding_ids`.
//   - unknown/insufficient evidence must escalate: refutation never papers
//     over them, and finalize refuses until the lens is re-captured.
package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const (
	RefutationSchema         = "biggz-ai.review-refutation/v1"
	RefutationEventSchema    = "biggz-ai.review-refutation-event/v1"
	RefutationOperation      = "refutation"
	RefutationVerdictRefuted = "refuted"
	RefutationVerdictStands  = "stands"
	refuterRole              = "Refuter"
)

// RefutationVerdict is one per-finding refuter verdict: `refuted` clears the
// finding (it no longer blocks), `stands` keeps it blocking.
type RefutationVerdict struct {
	FindingID string `json:"finding_id"`
	Verdict   string `json:"verdict"`
	Evidence  string `json:"evidence"`
}

// RefutationInput is the strict wire envelope accepted by
// `biggz review refute <lineage> --input -`. Unknown fields are rejected.
type RefutationInput struct {
	Schema   string              `json:"schema"`
	Lineage  string              `json:"lineage"`
	Verdicts []RefutationVerdict `json:"verdicts"`
}

// RefuteOutcome describes a registered refutation batch: the appended
// `refutation` revision and the per-finding verdict sets. Idempotent is true
// when the identical batch was already registered and nothing was appended.
type RefuteOutcome struct {
	LineageID  string   `json:"lineage_id"`
	Revision   string   `json:"revision"`
	Idempotent bool     `json:"idempotent"`
	Refuted    []string `json:"refuted"`
	Stands     []string `json:"stands"`
}

// refutationEventPayload is the durable `refutation` event payload.
type refutationEventPayload struct {
	Schema    string              `json:"schema"`
	LineageID string              `json:"lineage_id"`
	Verdicts  []RefutationVerdict `json:"verdicts"`
}

// RefutationSummary is the status surface of one lineage's refutation state:
// total required findings (inferential + candidate-causal), the verdict
// counts, and how many are still pending.
type RefutationSummary struct {
	Total   int `json:"total"`
	Refuted int `json:"refuted"`
	Stands  int `json:"stands"`
	Pending int `json:"pending"`
}

// refutationState is everything derived from the chain about refutation: the
// required finding IDs, the recorded verdicts, the captured findings keyed by
// ID, and the number of refutation batches.
type refutationState struct {
	requirements []string
	findings     map[string]ArtifactFinding
	verdicts     map[string]RefutationVerdict
	batches      int
	refuted      []string
	stands       []string
}

// DecodeRefutationInput strictly decodes a refutation input JSON object with
// unknown fields rejected and a single JSON value enforced.
func DecodeRefutationInput(payload []byte) (RefutationInput, error) {
	var input RefutationInput
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return RefutationInput{}, fmt.Errorf("refutation input: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return RefutationInput{}, errors.New("refutation input contains multiple JSON values")
	}
	return input, nil
}

// canonicalRefutationVerdict trims and validates one verdict shape: a
// non-empty finding id, a supported verdict value, and concrete evidence.
func canonicalRefutationVerdict(verdict RefutationVerdict) (RefutationVerdict, error) {
	verdict.FindingID = strings.TrimSpace(verdict.FindingID)
	verdict.Evidence = strings.TrimSpace(verdict.Evidence)
	if verdict.FindingID == "" || strings.ContainsRune(verdict.FindingID, '\x00') {
		return RefutationVerdict{}, errors.New("finding_id is required")
	}
	switch verdict.Verdict {
	case RefutationVerdictRefuted, RefutationVerdictStands:
	default:
		return RefutationVerdict{}, fmt.Errorf("unsupported verdict %q (use %q or %q)",
			verdict.Verdict, RefutationVerdictRefuted, RefutationVerdictStands)
	}
	if !isConcreteEvidence(verdict.Evidence) {
		return RefutationVerdict{}, errors.New("evidence must be concrete")
	}
	return verdict, nil
}

// isCandidateCausalDisposition reports whether the disposition is one of the
// blocking classes (introduced/behavior-activated/worsened).
func isCandidateCausalDisposition(disposition CausalDisposition) bool {
	switch disposition {
	case CausalIntroduced, CausalBehaviorActivated, CausalWorsened:
		return true
	}
	return false
}

// collectRefutationState walks the chain once and derives the required
// refutation set (severe + candidate-causal + inferential findings across all
// completed captures) and the recorded verdicts. A malformed refutation event
// fails loudly: events are content-addressed, so a payload that no longer
// parses means the chain was tampered with.
func collectRefutationState(chain ValidatedChain) (refutationState, error) {
	state := refutationState{
		findings: make(map[string]ArtifactFinding),
		verdicts: make(map[string]RefutationVerdict),
	}
	requirementSet := make(map[string]struct{})
	refutedSet := make(map[string]struct{})
	standsSet := make(map[string]struct{})
	for index := range chain.Records {
		rec := &chain.Records[index]
		switch rec.Operation {
		case LensResultOperation:
			handleLensResultRecord(chain, index, rec, &state, requirementSet)
		case RefutationOperation:
			if err := handleRefutationRecord(index, rec, &state, refutedSet, standsSet); err != nil {
				return refutationState{}, err
			}
		}
	}
	if err := finalizeRefutationState(&state, requirementSet, refutedSet, standsSet); err != nil {
		return refutationState{}, err
	}
	return state, nil
}

func handleLensResultRecord(chain ValidatedChain, index int, rec *Record, state *refutationState, requirementSet map[string]struct{}) {
	var payload lensResultEventPayload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil || payload.AdmissionDecision != AdmissionCompleted {
		return
	}
	if isSlotSuperseded(chain, index, payload.Lens, payload.SelectedOrder) {
		return
	}
	for _, finding := range payload.Result.Findings {
		state.findings[finding.ID] = finding
		if !isSevereSeverity(finding.Severity) || !isCandidateCausalDisposition(finding.CausalDisposition) {
			continue
		}
		if finding.EvidenceClass == EvidenceInferential {
			requirementSet[finding.ID] = struct{}{}
		}
	}
}

func handleRefutationRecord(index int, rec *Record, state *refutationState, refutedSet, standsSet map[string]struct{}) error {
	state.batches++
	var payload refutationEventPayload
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		return fmt.Errorf("chain refutation event %d is malformed: %w", index, err)
	}
	if payload.Schema != RefutationEventSchema {
		return fmt.Errorf("chain refutation event %d has an unsupported schema", index)
	}
	for _, verdict := range payload.Verdicts {
		canonical, err := canonicalRefutationVerdict(verdict)
		if err != nil {
			return fmt.Errorf("chain refutation event %d: %w", index, err)
		}
		state.verdicts[canonical.FindingID] = canonical
		if canonical.Verdict == RefutationVerdictRefuted {
			refutedSet[canonical.FindingID] = struct{}{}
		} else {
			standsSet[canonical.FindingID] = struct{}{}
		}
	}
	return nil
}

func finalizeRefutationState(state *refutationState, requirementSet, refutedSet, standsSet map[string]struct{}) error {
	var err error
	if state.requirements, err = canonicalStrings(sortedSetKeys(requirementSet), "refutation requirement id"); err != nil {
		return fmt.Errorf("refutation requirements: %w", err)
	}
	if state.refuted, err = canonicalStrings(sortedSetKeys(refutedSet), "refuted finding id"); err != nil {
		return fmt.Errorf("refutation verdicts: %w", err)
	}
	if state.stands, err = canonicalStrings(sortedSetKeys(standsSet), "standing finding id"); err != nil {
		return fmt.Errorf("refutation verdicts: %w", err)
	}
	return nil
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// RefutationSummaryOf derives the refutation surface of a lineage for status
// output: total required findings, recorded verdicts, and pending.
func RefutationSummaryOf(chain ValidatedChain) (*RefutationSummary, error) {
	state, err := collectRefutationState(chain)
	if err != nil {
		return nil, err
	}
	summary := &RefutationSummary{
		Total:   len(state.requirements),
		Refuted: len(state.refuted),
		Stands:  len(state.stands),
	}
	summary.Pending = summary.Total - summary.Refuted - summary.Stands
	if summary.Pending < 0 {
		summary.Pending = 0
	}
	return summary, nil
}

func decodeAndValidateRefutationInput(payload []byte, lineageID string) (RefutationInput, error) {
	input, err := DecodeRefutationInput(payload)
	if err != nil {
		return RefutationInput{}, err
	}
	if input.Schema != RefutationSchema {
		return RefutationInput{}, fmt.Errorf("refutation input schema %q is unsupported (want %q)", input.Schema, RefutationSchema)
	}
	if strings.TrimSpace(input.Lineage) != lineageID {
		return RefutationInput{}, fmt.Errorf("refutation input lineage %q does not match the CLI lineage %q", input.Lineage, lineageID)
	}
	return input, nil
}

func loadRefutationChain(repo, lineageID string) (*Store, ValidatedChain, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return nil, ValidatedChain{}, fmt.Errorf("refute: open store: %w", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		return nil, ValidatedChain{}, fmt.Errorf("refute: load chain: %w", err)
	}
	if chain.Count == 0 {
		return nil, ValidatedChain{}, errors.New("refute: lineage has no events")
	}
	verdict := store.Validate()
	if !verdict.Valid {
		return nil, ValidatedChain{}, fmt.Errorf("refute: chain integrity failed: %s", verdict.Reason)
	}
	if chain.Records[0].Operation != "start_review" {
		return nil, ValidatedChain{}, errors.New("refute: lineage genesis is not a review start")
	}
	if hasCompleteReview(chain) {
		return nil, ValidatedChain{}, errors.New("refute: the lineage is already finalized; register the refuter batch before finalize")
	}
	return store, chain, nil
}

func checkExistingRefutation(state refutationState, chain ValidatedChain, lineageID string, eventPayload []byte) (*RefuteOutcome, error) {
	if state.batches == 0 {
		return nil, nil
	}
	last := chain.Records[chain.Count-1]
	if state.batches == 1 && last.Operation == RefutationOperation && bytes.Equal(last.Payload, eventPayload) {
		return &RefuteOutcome{
			LineageID: lineageID, Revision: chain.HeadHash, Idempotent: true,
			Refuted: state.refuted, Stands: state.stands,
		}, nil
	}
	return nil, errors.New("refute: the lineage already carries a refutation batch; exactly one read-only refuter batch per review")
}

func validateFindingRefutable(finding ArtifactFinding, verdict RefutationVerdict) error {
	if !isSevereSeverity(finding.Severity) {
		return fmt.Errorf("refute: finding %q is not a severe finding; only BLOCKER/CRITICAL findings are refutable", verdict.FindingID)
	}
	if finding.EvidenceClass == EvidenceDeterministic {
		return fmt.Errorf("refute: deterministic finding %q is auto-blocking and cannot be refuted; resolve it with a correction", verdict.FindingID)
	}
	if finding.EvidenceClass == EvidenceInsufficient {
		return fmt.Errorf("refute: finding %q has insufficient evidence; it must escalate, not be refuted", verdict.FindingID)
	}
	if finding.CausalDisposition == CausalUnknown {
		return fmt.Errorf("refute: finding %q has unknown causal disposition; it must escalate, not be refuted", verdict.FindingID)
	}
	if finding.CausalDisposition == CausalPreExisting || finding.CausalDisposition == CausalBaseOnly {
		return fmt.Errorf("refute: finding %q is not candidate-causal; only introduced/behavior-activated/worsened findings are refutable", verdict.FindingID)
	}
	return nil
}

func shouldRefute(verdict RefutationVerdict, seen map[string]struct{}, findings map[string]ArtifactFinding) (RefutationVerdict, error) {
	canonical, err := canonicalRefutationVerdict(verdict)
	if err != nil {
		return RefutationVerdict{}, fmt.Errorf("refute: %w", err)
	}
	if _, duplicate := seen[canonical.FindingID]; duplicate {
		return RefutationVerdict{}, fmt.Errorf("refute: duplicate verdict for finding %q", canonical.FindingID)
	}
	finding, ok := findings[canonical.FindingID]
	if !ok {
		return RefutationVerdict{}, fmt.Errorf("refute: unknown finding id %q: it is not a captured finding of this lineage", canonical.FindingID)
	}
	if err := validateFindingRefutable(finding, canonical); err != nil {
		return RefutationVerdict{}, err
	}
	return canonical, nil
}

func refuteWithEvidence(verdicts []RefutationVerdict, state refutationState) (refutedIDs, standsIDs []string, covered map[string]struct{}, err error) {
	refutedIDs = make([]string, 0, len(verdicts))
	standsIDs = make([]string, 0, len(verdicts))
	covered = make(map[string]struct{}, len(verdicts))
	seen := make(map[string]struct{}, len(verdicts))
	for _, raw := range verdicts {
		verdict, vErr := shouldRefute(raw, seen, state.findings)
		if vErr != nil {
			return nil, nil, nil, vErr
		}
		seen[verdict.FindingID] = struct{}{}
		if verdict.Verdict == RefutationVerdictRefuted {
			refutedIDs = append(refutedIDs, verdict.FindingID)
		} else {
			standsIDs = append(standsIDs, verdict.FindingID)
		}
		covered[verdict.FindingID] = struct{}{}
	}
	return refutedIDs, standsIDs, covered, nil
}

func ensureVerdictCoverage(requirements []string, covered map[string]struct{}, verdicts []RefutationVerdict) error {
	missing := stringDifference(requirements, sortedSetKeys(covered))
	if len(missing) > 0 {
		return fmt.Errorf("refute: the batch must cover every inferential candidate-causal finding in one shot; missing verdicts for: %s", strings.Join(missing, ", "))
	}
	if len(verdicts) == 0 {
		return errors.New("refute: no inferential candidate-causal findings require refutation")
	}
	return nil
}

func appendRefutationBatch(store *Store, chain ValidatedChain, eventPayload []byte, refutedIDs, standsIDs []string, lineageID string) (RefuteOutcome, error) {
	var outcome RefuteOutcome
	err := WithFileLock(store.Dir, func() error {
		fresh, err := store.LoadChain()
		if err != nil {
			return fmt.Errorf("refute: load chain: %w", err)
		}
		if fresh.HeadHash != chain.HeadHash {
			return fmt.Errorf("refute: expected revision %s does not match the current head %s", chain.HeadHash, fresh.HeadHash)
		}
		freshState, err := collectRefutationState(fresh)
		if err != nil {
			return fmt.Errorf("refute: %w", err)
		}
		if freshState.batches > 0 {
			return errors.New("refute: the lineage already carries a refutation batch; exactly one read-only refuter batch per review")
		}
		revision, err := store.appendLocked(fresh.HeadHash, Record{
			Operation: RefutationOperation,
			Role:      refuterRole,
			Actor:     refuterRole,
			Timestamp: time.Now().Format(time.RFC3339Nano),
			Payload:   eventPayload,
		})
		if err != nil {
			return fmt.Errorf("refute: append refutation event: %w", err)
		}
		outcome = RefuteOutcome{LineageID: lineageID, Revision: revision, Refuted: refutedIDs, Stands: standsIDs}
		return nil
	})
	return outcome, err
}

// Refute registers the one read-only refuter batch for a lineage: the input
// verdicts must cover EXACTLY the required set — every severe
// candidate-causal finding with inferential evidence — in one shot, mirroring
// gentle-ai's "exactly one read-only refuter batch". Re-running the identical
// batch is idempotent; a second, different batch is rejected.
func Refute(repo, lineageID string, payload []byte) (RefuteOutcome, error) {
	input, err := decodeAndValidateRefutationInput(payload, lineageID)
	if err != nil {
		return RefuteOutcome{}, err
	}
	store, chain, err := loadRefutationChain(repo, lineageID)
	if err != nil {
		return RefuteOutcome{}, err
	}
	state, err := collectRefutationState(chain)
	if err != nil {
		return RefuteOutcome{}, fmt.Errorf("refute: %w", err)
	}
	eventPayload, err := marshalRefutationEvent(lineageID, input.Verdicts)
	if err != nil {
		return RefuteOutcome{}, fmt.Errorf("refute: marshal refutation event: %w", err)
	}
	if existing, err := checkExistingRefutation(state, chain, lineageID, eventPayload); err != nil {
		return RefuteOutcome{}, err
	} else if existing != nil {
		return *existing, nil
	}
	refutedIDs, standsIDs, covered, err := refuteWithEvidence(input.Verdicts, state)
	if err != nil {
		return RefuteOutcome{}, err
	}
	if err := ensureVerdictCoverage(state.requirements, covered, input.Verdicts); err != nil {
		return RefuteOutcome{}, err
	}
	return appendRefutationBatch(store, chain, eventPayload, refutedIDs, standsIDs, lineageID)
}

// marshalRefutationEvent renders the durable refutation event payload for the
// given verdicts (canonical, trimmed).
func marshalRefutationEvent(lineageID string, verdicts []RefutationVerdict) ([]byte, error) {
	canonical := make([]RefutationVerdict, 0, len(verdicts))
	for _, verdict := range verdicts {
		value, err := canonicalRefutationVerdict(verdict)
		if err != nil {
			return nil, err
		}
		canonical = append(canonical, value)
	}
	return json.Marshal(refutationEventPayload{
		Schema: RefutationEventSchema, LineageID: lineageID, Verdicts: canonical,
	})
}
