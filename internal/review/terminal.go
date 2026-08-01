// Terminal lineage operations — invalidate and abandon (Phase C2).
//
// This is the pragmatic subset of gentle-ai's review repair/recovery verb
// family. The full gentle surface (recover, reclaim, reconcile-authority,
// dispose-result, reopen-results, quarantine-legacy, retry-final-verification,
// inspect-*) is intentionally NOT ported; these two verbs cover the
// operational core:
//
//   - `review invalidate <lineage> <reason>` appends an `invalidate` event
//     carrying the audit reason; the lineage state becomes invalidated and
//     every subsequent gate fails with that reason. A pass is never
//     fabricated.
//   - `review abandon <lineage>` appends a `withdraw` event; the lineage
//     state becomes withdrawn and gates fail. Export/import remain possible
//     for both states.
//
// Both events are ordinary content-addressed records, so chain integrity,
// receipt re-verification, export, and import all keep working.
package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biggz-ai/biggz/model"
)

const (
	// InvalidateEventSchema identifies the invalidate event payload.
	InvalidateEventSchema = "biggz-ai.review-invalidate-event/v1"
	// WithdrawEventSchema identifies the withdraw event payload.
	WithdrawEventSchema = "biggz-ai.review-withdraw-event/v1"

	// InvalidateOperation is the event operation for `review invalidate`.
	InvalidateOperation = "invalidate"
	// WithdrawOperation is the event operation for `review abandon`.
	WithdrawOperation = "withdraw"
)

// invalidateEventPayload is the durable invalidate event payload.
type invalidateEventPayload struct {
	Schema string `json:"schema"`
	Reason string `json:"reason"`
}

// withdrawEventPayload is the durable withdraw event payload.
type withdrawEventPayload struct {
	Schema string `json:"schema"`
}

// Invalidate marks a lineage invalidated with an audit reason. It appends an
// `invalidate` event; the FSM state becomes invalidated and subsequent gates
// fail with the reason. Returns the new head revision.
//
// Refused when the lineage is empty, its chain is broken (run `biggz review
// repair` first), or it is already terminated.
func Invalidate(repo, lineageID, reason string) (string, error) {
	if strings.TrimSpace(reason) == "" {
		return "", errors.New("invalidate: a non-empty reason is required")
	}
	store, err := Open(repo, lineageID)
	if err != nil {
		return "", fmt.Errorf("invalidate: open store: %w", err)
	}
	payload, err := json.Marshal(invalidateEventPayload{
		Schema: InvalidateEventSchema, Reason: strings.TrimSpace(reason),
	})
	if err != nil {
		return "", fmt.Errorf("invalidate: marshal event: %w", err)
	}
	return appendTerminalEvent(store, lineageID, InvalidateOperation, payload)
}

// Abandon withdraws a lineage. It appends a `withdraw` event; the FSM state
// becomes withdrawn and gates fail. Export/import remain possible. Returns
// the new head revision.
func Abandon(repo, lineageID string) (string, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return "", fmt.Errorf("abandon: open store: %w", err)
	}
	payload, err := json.Marshal(withdrawEventPayload{Schema: WithdrawEventSchema})
	if err != nil {
		return "", fmt.Errorf("abandon: marshal event: %w", err)
	}
	return appendTerminalEvent(store, lineageID, WithdrawOperation, payload)
}

// appendTerminalEvent is the shared append path for invalidate/withdraw: it
// refuses empty or broken chains (fail closed — a terminal event must never
// mask tampering), refuses an already-terminated lineage, and appends the
// event under the lineage lock.
func appendTerminalEvent(store *Store, lineageID, operation string, payload []byte) (string, error) {
	var revision string
	err := WithFileLock(store.Dir, func() error {
		chain, err := store.LoadChain()
		if err != nil {
			return fmt.Errorf("%s: load chain: %w", operation, err)
		}
		if chain.Count == 0 {
			return fmt.Errorf("%s: lineage has no events", operation)
		}
		verdict := store.Validate()
		if !verdict.Valid {
			return fmt.Errorf("%s: chain integrity failed (%s); run 'biggz review repair %s' or export the lineage first", operation, verdict.Reason, lineageID)
		}
		if state, _ := terminatedStateOf(chain); state != "" {
			return fmt.Errorf("%s: lineage is already %s", operation, state)
		}
		role := string(model.RoleAdmin)
		actor := string(model.RoleAdmin)
		if operation == WithdrawOperation {
			// FSM guard: Any → Withdrawn is an Author transition.
			role = string(model.RoleAuthor)
			actor = string(model.RoleAuthor)
		}
		revision, err = store.appendLocked(chain.HeadHash, Record{
			Operation: operation,
			Role:      role,
			Actor:     actor,
			Timestamp: time.Now().Format(time.RFC3339Nano),
			Payload:   payload,
		})
		if err != nil {
			return fmt.Errorf("%s: append event: %w", operation, err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return revision, nil
}
