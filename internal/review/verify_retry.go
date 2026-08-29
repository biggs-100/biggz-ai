// Terminal state re-verification — `biggz review retry-final-verification
// <lineage>` (Debt D3).
//
// Retry re-validates the terminal state of a finalized lineage: chain
// validity plus the persisted-receipt match, exactly the checks finalize and
// validate run. When the receipt artifact is missing but the chain carries a
// complete_review event, the receipt is RE-MATERIALIZED from the canonical
// payloads: the receipts/ file is content-addressed, so re-materialization
// produces the same name with hash-identical content. A receipt that exists
// but fails verification is a tamper signal and is never overwritten.
package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	// VerificationRetrySchema identifies the retry report envelope.
	VerificationRetrySchema = "biggz-ai.review-verification-retry/v1"
)

// VerificationReport describes the outcome of retry-final-verification.
type VerificationReport struct {
	Schema                string   `json:"schema"`
	LineageID             string   `json:"lineage_id"`
	Passed                bool     `json:"passed"`
	ChainValid            bool     `json:"chain_valid"`
	ReceiptMatch          bool     `json:"receipt_match"`
	ReceiptReMaterialized bool     `json:"receipt_re_materialized"`
	ReceiptPath           string   `json:"receipt_path,omitempty"`
	ReceiptHash           string   `json:"receipt_hash,omitempty"`
	Reasons               []string `json:"reasons"`
}

// RetryFinalVerification recomputes chain validity and the receipt match for
// a lineage. When the receipt artifact is missing but the chain carries a
// complete_review reference, the receipt is re-materialized from the canonical
// payloads (hash-identical to the original content-addressed file).
func retryValidateChain(store *Store, chain ValidatedChain, report *VerificationReport) bool {
	if chain.Count == 0 {
		report.Reasons = append(report.Reasons, "lineage has no events")
		return false
	}
	verdict := store.Validate()
	if !verdict.Valid {
		report.Reasons = append(report.Reasons, "chain integrity: FAIL — "+verdict.Reason)
		return false
	}
	report.ChainValid = true
	return true
}

func retryLoadReceipt(store *Store, chain ValidatedChain, repo string, report *VerificationReport) (PersistedReceipt, *ReceiptArtifactRef, bool) {
	ref := receiptArtifactOf(chain)
	if ref == nil {
		report.Reasons = append(report.Reasons, "receipt match: FAIL — the lineage carries no complete_review receipt reference; it is not finalized")
		return PersistedReceipt{}, nil, false
	}
	evt := completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash}
	stored, err := readReceiptFile(store, evt)
	if err == nil {
		return stored, ref, true
	}
	if !os.IsNotExist(err) {
		report.Reasons = append(report.Reasons, "receipt match: FAIL — persisted receipt artifact is invalid ("+err.Error()+"); it is not overwritten (tamper signal)")
		return PersistedReceipt{}, nil, false
	}
	rebuilt, reErr := reMaterializeReceipt(store, repo, chain)
	if reErr != nil {
		report.Reasons = append(report.Reasons, "receipt match: FAIL — receipt artifact missing and re-materialization failed ("+reErr.Error()+")")
		return PersistedReceipt{}, nil, false
	}
	report.ReceiptReMaterialized = true
	report.Reasons = append(report.Reasons, "receipt re-materialized from canonical payloads")
	return rebuilt, ref, true
}

func retryVerifyReceiptMatch(store *Store, repo string, chain ValidatedChain, stored PersistedReceipt, ref *ReceiptArtifactRef, report *VerificationReport) bool {
	expected, err := deriveExpectedReceipt(store, repo, chain)
	if err != nil {
		report.Reasons = append(report.Reasons, "receipt match: FAIL — "+err.Error())
		return false
	}
	if !reflect.DeepEqual(stored, expected) {
		report.Reasons = append(report.Reasons, "receipt match: FAIL — persisted receipt does not match the current lineage state")
		return false
	}
	report.ReceiptMatch = true
	report.ReceiptPath = ref.Path
	report.ReceiptHash = ref.Hash
	return true
}

func retryLocked(store *Store, repo string, report *VerificationReport) error {
	chain, err := store.LoadChain()
	if err != nil {
		return fmt.Errorf("retry-final-verification: load chain: %w", err)
	}
	if !retryValidateChain(store, chain, report) {
		return nil
	}
	stored, ref, ok := retryLoadReceipt(store, chain, repo, report)
	if !ok {
		return nil
	}
	retryVerifyReceiptMatch(store, repo, chain, stored, ref, report)
	return nil
}

func RetryFinalVerification(repo, lineageID string) (VerificationReport, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return VerificationReport{}, fmt.Errorf("retry-final-verification: open store: %w", err)
	}
	report := VerificationReport{Schema: VerificationRetrySchema, LineageID: lineageID, Reasons: make([]string, 0, 3)}
	if err := WithFileLock(store.Dir, func() error { return retryLocked(store, repo, &report) }); err != nil {
		return VerificationReport{}, err
	}
	report.Passed = report.ChainValid && report.ReceiptMatch
	return report, nil
}

// lastCompleteIndex returns the chain index of the LAST complete_review event,
// or -1 when the chain carries none.
func lastCompleteIndex(chain ValidatedChain) int {
	for index := len(chain.Records) - 1; index >= 0; index-- {
		if chain.Records[index].Operation == CompleteReviewOperation {
			return index
		}
	}
	return -1
}

// deriveExpectedReceipt re-derives the terminal receipt from the chain's
// canonical payloads: the frozen start plan, the live captured slots, and the
// repository-derived candidate. The receipt binds the revision BEFORE the
// last complete_review event, exactly like finalize's idempotent path.
func deriveExpectedReceipt(store *Store, repo string, chain ValidatedChain) (PersistedReceipt, error) {
	genesis := chain.Records[0]
	if genesis.Operation != "start_review" {
		return PersistedReceipt{}, errors.New("lineage genesis is not a review start")
	}
	var plan StartEventPayload
	if err := json.Unmarshal(genesis.Payload, &plan); err != nil || strings.TrimSpace(plan.CommitSHA) == "" {
		return PersistedReceipt{}, errors.New("genesis event does not carry a review subject")
	}
	complete := lastCompleteIndex(chain)
	if complete <= 0 {
		return PersistedReceipt{}, errors.New("lineage carries no complete_review event to bind the receipt")
	}
	data, err := deriveFinalizeData(repo, chain, plan)
	if err != nil {
		return PersistedReceipt{}, err
	}
	// Preserve cumulative ledger from persisted receipt if available (hash-identical re-materialization).
	if ref := receiptArtifactOf(chain); ref != nil {
		if stored, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash}); err == nil {
			data.cumulativeCorrectionLines = stored.CumulativeCorrectionLines
			data.fixDeltaHash = stored.FixDeltaHash
		} else if os.IsNotExist(err) {
			// Missing file: derive via cumulative helper (prior + post). For initial finalize, this is 0.
			if cum, delta := deriveCumulativeAndFixDelta(chain, store); true {
				// If derive returns 0 (no prior), keep data defaults (0/Empty) to stay hash-identical for first receipt.
				// For subsequent receipts after correction where file is missing, helper attempts to recompute.
				if cum != 0 || delta != EmptyFixDeltaHash {
					data.cumulativeCorrectionLines = cum
					data.fixDeltaHash = delta
				}
			}
		}
		// Tampered case (err not IsNotExist) is not expected here; deriveExpected will be compared and fail elsewhere.
	}
	revisions := recordRevisions(chain)
	receipt := buildReceipt(chain.LineageID, revisions[0], revisions[complete-1], data)
	if err := receipt.Validate(); err != nil {
		return PersistedReceipt{}, err
	}
	return receipt, nil
}

// reMaterializeReceipt rebuilds the missing receipt artifact from the chain's
// canonical payloads. The file is content-addressed, so the rebuilt file has
// the same name as the original; the complete_review reference must match the
// rebuilt hash (a mismatch means the event was tampered and nothing is
// written).
func reMaterializeReceipt(store *Store, repo string, chain ValidatedChain) (PersistedReceipt, error) {
	receipt, err := deriveExpectedReceipt(store, repo, chain)
	if err != nil {
		return PersistedReceipt{}, err
	}
	ref := receiptArtifactOf(chain)
	if ref == nil {
		return PersistedReceipt{}, errors.New("chain carries no complete_review receipt reference")
	}
	if receipt.ReceiptHash != ref.Hash {
		return PersistedReceipt{}, fmt.Errorf(
			"complete_review reference hash %s does not match the chain-derived receipt hash %s — the event was tampered; refusing to re-materialize",
			ref.Hash, receipt.ReceiptHash)
	}
	path, err := writeReceiptLocked(store, receipt)
	if err != nil {
		return PersistedReceipt{}, err
	}
	if path != ref.Path {
		return PersistedReceipt{}, fmt.Errorf(
			"re-materialized receipt path %s does not match the recorded reference %s", path, ref.Path)
	}
	payload, err := os.ReadFile(filepath.Join(store.Dir, path))
	if err != nil {
		return PersistedReceipt{}, err
	}
	name := strings.TrimSuffix(filepath.Base(path), ".json")
	if sha256Hex(payload) != name {
		return PersistedReceipt{}, errors.New("re-materialized receipt does not match its content address")
	}
	return receipt, nil
}
