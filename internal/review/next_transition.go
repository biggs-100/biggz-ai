// Next-transition derivation — Phase C2 review-workflow parity.
//
// `biggz review status <lineage> --json` gains a derived `next_transition`
// envelope that is the ONLY routing authority for the orchestrator. It is
// derived purely from persisted bytes (the event chain, the content-address
// verdict, and the RDD kill switch); no new machine surface is added and no
// existing status field changes.
//
// Derivation order (stop rules before workflow rules):
//
//  1. Chain broken/tampered                       → {action:"stop", reason:"chain_invalid"}
//  2. Kill switch off (delivery unmanaged)        → {action:"stop", reason:"rdd_disabled"}
//  3. Terminal state (invalidated/withdrawn/
//     escalated/blocked/superseded)               → {action:"stop", reason:<state>}
//  4. Lenses declared, any slot missing           → {action:"collect", lens:<name>, order:<n>}
//  5. All captured, not finalized                 → {action:"finalize"}
//  6. Finalized, blocking findings unresolved     → {action:"correction", budget_remaining:<n>}
//  7. Finalized clean                             → {action:"gate", gates:[...]}
//
// The spec lists the terminal-state rule last, but evaluated top-down it
// would be masked by the workflow rules: an invalidated lineage that already
// captured every lens (or was finalized) would derive collect/finalize/gate.
// Terminal stop states therefore sit with the other stop rules, ahead of the
// workflow rules.
package review

import (
	"encoding/json"
	"strings"
)

// NextTransition is the derived routing envelope for one lineage.
type NextTransition struct {
	Action          string   `json:"action"` // execute | collect | finalize | correction | gate | stop
	Reason          string   `json:"reason,omitempty"`
	Lens            string   `json:"lens,omitempty"`
	Order           *int     `json:"order,omitempty"`
	BudgetRemaining int      `json:"budget_remaining,omitempty"`
	Gates           []string `json:"gates,omitempty"`
}

// gateOrder is the canonical publication gate order the orchestrator runs
// after a clean finalize.
var gateOrder = []string{"post-apply", "pre-commit", "pre-push", "pre-pr", "release"}

// deriveNextTransition computes the routing envelope for a lineage from its
// chain, the content-address verdict, and the RDD kill switch. A lineage with
// no events has nothing to route and yields nil.
func deriveNextTransition(store *Store, repo string, chain ValidatedChain, verdict IntegrityVerdict) *NextTransition {
	if chain.Count == 0 {
		return nil
	}

	// 1. A tampered chain stops everything: no derivation may trust it.
	if !verdict.Valid {
		return &NextTransition{Action: "stop", Reason: "chain_invalid"}
	}

	// 2. Kill switch off: delivery is unmanaged, so there is nothing a
	//    managed workflow can route to. An UNREADABLE switch is not a
	//    disabled switch (fail closed), exactly like the gates.
	worktreeDir, commonDir := detectRDDDirs(repo)
	if status, rddErr := RDDStatus(worktreeDir, commonDir); rddErr == nil && status.EffectiveMode == RDDModeDisabled {
		return &NextTransition{Action: "stop", Reason: "rdd_disabled"}
	}

	// 3. Terminal states stop the workflow before any capture-state rule.
	if state, _ := terminatedStateOf(chain); state != "" {
		return &NextTransition{Action: "stop", Reason: state}
	}

	// 4. Collect: the first declared lens slot without a completed capture,
	//    in declared (canonical, sorted) order. The missing slot's order is
	//    its index in the declared list — the same index capture-result uses
	//    for the sibling slots of a multi-lens review.
	declared, _ := declaredLensSelection(chain)
	captured := capturedSlotNames(chain)
	for _, lens := range declared {
		if !containsString(captured, lens) {
			order := 0
			for i, name := range declared {
				if name == lens {
					order = i
					break
				}
			}
			return &NextTransition{Action: "collect", Lens: lens, Order: &order}
		}
	}

	// 5. Every declared lens captured (or a declaration-free capture) and no
	//    terminal complete_review event: finalize.
	if !hasCompleteReview(chain) && (len(declared) > 0 || len(captured) > 0) {
		return &NextTransition{Action: "finalize"}
	}

	// 6/7. Finalized: blocking findings unresolved → correction; else gate.
	if ref := receiptArtifactOf(chain); ref != nil {
		blocking := unresolvedBlockingCount(store, chain, ref)
		if blocking > 0 {
			remaining := 0
			if budget := frozenBudgetOf(chain); budget != nil {
				// This port has no correction-line consumption accounting
				// (the receipt fix delta stays at EmptyFixDeltaHash), so the
				// frozen budget is still fully remaining.
				remaining = budget.CorrectionLines
			}
			return &NextTransition{Action: "correction", BudgetRemaining: remaining}
		}
		return &NextTransition{Action: "gate", Gates: gateOrder}
	}

	// No rule matched (e.g. a declaration-free lineage with no captures):
	// nothing to route.
	return nil
}

// declaredLensSelection returns the canonical declared lens list from the
// frozen start plan, together with the parsed plan.
func declaredLensSelection(chain ValidatedChain) ([]string, StartEventPayload) {
	var plan StartEventPayload
	if chain.Count == 0 {
		return nil, plan
	}
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil {
		return nil, plan
	}
	declared, err := canonicalStrings(plan.SelectedLenses, "selected lens")
	if err != nil {
		return nil, plan
	}
	return declared, plan
}

// capturedSlotNames returns the lens names with an AdmissionCompleted slot.
func capturedSlotNames(chain ValidatedChain) []string {
	var names []string
	for _, lens := range CapturedLenses(chain) {
		if lens.Status == CapturedLensStatus {
			names = append(names, lens.Lens)
		}
	}
	return names
}

// hasCompleteReview reports whether the chain carries a complete_review event.
func hasCompleteReview(chain ValidatedChain) bool {
	for _, rec := range chain.Records {
		if rec.Operation == CompleteReviewOperation {
			return true
		}
	}
	return false
}

// unresolvedBlockingCount recomputes how many candidate-causal findings the
// persisted receipt leaves unresolved. An unreadable receipt cannot prove
// resolution: every candidate-causal finding then counts as blocking
// (fail safe — a routing bug must never fabricate a pass).
func unresolvedBlockingCount(store *Store, chain ValidatedChain, ref *ReceiptArtifactRef) int {
	receipt, err := readReceiptFile(store, completeEventPayload{
		Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash,
	})
	if err != nil {
		return countCandidateCausalFindings(chain)
	}
	summary, _ := recomputeGateFindings(chain, receipt)
	return summary.Blocking
}

// countCandidateCausalFindings counts the unique candidate-causal finding IDs
// recorded across completed lens captures.
func countCandidateCausalFindings(chain ValidatedChain) int {
	seen := make(map[string]struct{})
	for index := range chain.Records {
		rec := &chain.Records[index]
		if rec.Operation != LensResultOperation {
			continue
		}
		var payload lensResultEventPayload
		if err := json.Unmarshal(rec.Payload, &payload); err != nil || payload.AdmissionDecision != AdmissionCompleted {
			continue
		}
		for _, id := range payload.CandidateCausalFindingIDs {
			if _, dup := seen[id]; !dup {
				seen[id] = struct{}{}
			}
		}
	}
	return len(seen)
}

// stopStates maps terminal event operations to their FSM stop state.
var stopStates = map[string]string{
	"invalidate": "invalidated",
	"withdraw":   "withdrawn",
	"escalate":   "escalated",
	"block":      "blocked",
	"supersede":  "superseded",
}

// terminatedStateOf returns the terminal stop state named by the lineage's
// LAST event operation, if any. Only the last event matters: a terminated
// lineage is terminal, and nothing may append after termination except an
// import replay of the same bytes.
func terminatedStateOf(chain ValidatedChain) (state, reason string) {
	if chain.Count == 0 {
		return "", ""
	}
	last := chain.Records[chain.Count-1]
	state, ok := stopStates[last.Operation]
	if !ok {
		return "", ""
	}
	if last.Operation == "invalidate" {
		var payload invalidateEventPayload
		if err := json.Unmarshal(last.Payload, &payload); err == nil {
			reason = strings.TrimSpace(payload.Reason)
		}
	}
	return state, reason
}
