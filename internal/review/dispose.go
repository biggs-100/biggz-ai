// Captured lens slot disposition — `review dispose-result` and
// `review reopen-results` (Debt D3).
//
// This is biggz-ai's port of gentle-ai's reviewer-result disposition and
// bulk reopen semantics (compact_result_disposition.go + compact_result_reopen.go),
// adapted to the content-addressed event store:
//
//   - `review dispose-result <lineage> --lens <name> --order <n>` appends a
//     `dispose` event (role Author, optional reason) that marks one captured
//     lens slot discarded. The slot's captured result no longer counts
//     anywhere (status, refutation requirements, finalize evidence), and a
//     fresh capture for the same slot is allowed afterwards — the new capture
//     supersedes the disposed one.
//   - `review reopen-results <lineage>` appends one `reopen` event that
//     disposes EVERY captured lens slot in a single bulk transition, so a
//     review whose scope changed can be re-collected from scratch.
//
// Finalize refuses a disposed slot that sits in the frozen lens plan without
// a fresh capture: the disposition is a discard, never a silent pass.
package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/biggz-ai/biggz/model"
)

const (
	// DisposeEventSchema identifies the dispose event payload.
	DisposeEventSchema = "biggz-ai.review-dispose-event/v1"
	// ReopenEventSchema identifies the reopen event payload.
	ReopenEventSchema = "biggz-ai.review-reopen-event/v1"

	// DisposeOperation is the event operation for `review dispose-result`.
	DisposeOperation = "dispose"
	// ReopenOperation is the event operation for `review reopen-results`.
	ReopenOperation = "reopen"

	slotStateCaptured = "captured"
	slotStateDisposed = "disposed"
)

// SlotRef identifies one captured lens slot by lens + selected order.
type SlotRef struct {
	Lens  string `json:"lens"`
	Order int    `json:"order"`
}

// disposeEventPayload is the durable `dispose` event payload.
type disposeEventPayload struct {
	Schema string `json:"schema"`
	Lens   string `json:"lens"`
	Order  int    `json:"order"`
	Reason string `json:"reason,omitempty"`
}

// reopenEventPayload is the durable `reopen` event payload: the complete list
// of captured slots the bulk disposition discards.
type reopenEventPayload struct {
	Schema string    `json:"schema"`
	Slots  []SlotRef `json:"slots"`
}

// slotKey is the canonical identity of one lens slot.
type slotKey struct {
	lens  string
	order int
}

// slotMarkedBy reports whether a dispose/reopen record marks the given slot.
func slotMarkedBy(rec *Record, lens string, order int) bool {
	switch rec.Operation {
	case DisposeOperation:
		var payload disposeEventPayload
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return false
		}
		return payload.Lens == lens && payload.Order == order
	case ReopenOperation:
		var payload reopenEventPayload
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return false
		}
		for _, slot := range payload.Slots {
			if slot.Lens == lens && slot.Order == order {
				return true
			}
		}
	}
	return false
}

// isSlotSuperseded reports whether the chain marks the (lens, order) slot
// disposed strictly after the event at index. A superseded capture no longer
// counts: it was discarded and (usually) re-captured later.
func isSlotSuperseded(chain ValidatedChain, index int, lens string, order int) bool {
	for i := index + 1; i < len(chain.Records); i++ {
		if slotMarkedBy(&chain.Records[i], lens, order) {
			return true
		}
	}
	return false
}

// slotStates derives, for every slot the chain ever captured or disposed, the
// state left by the LATEST event affecting it: "captured" or "disposed".
// A malformed dispose/reopen event fails loudly — events are content-
// addressed, so a payload that no longer parses means the chain was tampered.
func slotStates(chain ValidatedChain) (map[slotKey]string, error) {
	states := make(map[slotKey]string)
	for index := range chain.Records {
		rec := &chain.Records[index]
		switch rec.Operation {
		case LensResultOperation:
			var payload lensResultEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err != nil || payload.AdmissionDecision != AdmissionCompleted {
				continue
			}
			states[slotKey{lens: payload.Lens, order: payload.SelectedOrder}] = slotStateCaptured
		case DisposeOperation:
			var payload disposeEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err != nil {
				return nil, fmt.Errorf("chain dispose event %d is malformed: %w", index, err)
			}
			states[slotKey{lens: payload.Lens, order: payload.Order}] = slotStateDisposed
		case ReopenOperation:
			var payload reopenEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err != nil {
				return nil, fmt.Errorf("chain reopen event %d is malformed: %w", index, err)
			}
			for _, slot := range payload.Slots {
				states[slotKey{lens: slot.Lens, order: slot.Order}] = slotStateDisposed
			}
		}
	}
	return states, nil
}

// disposedSlots returns every slot whose latest chain event is a disposition,
// canonically ordered by order then lens.
func disposedSlots(chain ValidatedChain) ([]SlotRef, error) {
	states, err := slotStates(chain)
	if err != nil {
		return nil, err
	}
	slots := make([]SlotRef, 0)
	for key, state := range states {
		if state == slotStateDisposed {
			slots = append(slots, SlotRef{Lens: key.lens, Order: key.order})
		}
	}
	return sortSlotRefs(slots), nil
}

// capturedSlots returns every slot whose latest chain event is a completed
// capture, canonically ordered by order then lens.
func capturedSlots(chain ValidatedChain) ([]SlotRef, error) {
	states, err := slotStates(chain)
	if err != nil {
		return nil, err
	}
	slots := make([]SlotRef, 0)
	for key, state := range states {
		if state == slotStateCaptured {
			slots = append(slots, SlotRef{Lens: key.lens, Order: key.order})
		}
	}
	return sortSlotRefs(slots), nil
}

func sortSlotRefs(slots []SlotRef) []SlotRef {
	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Order != slots[j].Order {
			return slots[i].Order < slots[j].Order
		}
		return slots[i].Lens < slots[j].Lens
	})
	return slots
}

// DisposeResult discards one captured lens slot. It appends a `dispose` event
// (role Author); the slot stops counting everywhere and a fresh capture for
// it is allowed afterwards. Refused when the slot has no captured reviewer
// result to dispose (never captured, or already disposed). Returns the new
// head revision.
func DisposeResult(repo, lineageID, lens string, order int, reason string) (string, error) {
	if !isSupportedLens(lens) {
		return "", fmt.Errorf("dispose-result: unsupported lens %q", lens)
	}
	if order < 0 {
		return "", errors.New("dispose-result: order must be zero or greater")
	}
	reason = strings.TrimSpace(reason)
	store, err := Open(repo, lineageID)
	if err != nil {
		return "", fmt.Errorf("dispose-result: open store: %w", err)
	}
	payload, err := json.Marshal(disposeEventPayload{
		Schema: DisposeEventSchema, Lens: lens, Order: order, Reason: reason,
	})
	if err != nil {
		return "", fmt.Errorf("dispose-result: marshal event: %w", err)
	}
	var revision string
	err = WithFileLock(store.Dir, func() error {
		chain, err := store.LoadChain()
		if err != nil {
			return fmt.Errorf("dispose-result: load chain: %w", err)
		}
		if chain.Count == 0 {
			return errors.New("dispose-result: lineage has no events")
		}
		verdict := store.Validate()
		if !verdict.Valid {
			return fmt.Errorf("dispose-result: chain integrity failed (%s); run 'biggz review repair %s' or export the lineage first", verdict.Reason, lineageID)
		}
		if state, _ := terminatedStateOf(chain); state != "" {
			return fmt.Errorf("dispose-result: lineage is already %s", state)
		}
		states, err := slotStates(chain)
		if err != nil {
			return fmt.Errorf("dispose-result: %w", err)
		}
		if states[slotKey{lens: lens, order: order}] != slotStateCaptured {
			return fmt.Errorf("dispose-result: lens slot %q order %d has no captured reviewer result to dispose", lens, order)
		}
		revision, err = store.appendLocked(chain.HeadHash, Record{
			Operation: DisposeOperation,
			Role:      string(model.RoleAuthor),
			Actor:     string(model.RoleAuthor),
			Timestamp: time.Now().Format(time.RFC3339Nano),
			Payload:   payload,
		})
		if err != nil {
			return fmt.Errorf("dispose-result: append event: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return revision, nil
}

// ReopenResults discards EVERY captured lens slot in one bulk transition,
// e.g. after a scope change: one `reopen` event (role Author) lists all slots
// disposed, and finalize refuses until every planned slot is re-captured.
// Returns the new head revision.
func ReopenResults(repo, lineageID string) (string, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return "", fmt.Errorf("reopen-results: open store: %w", err)
	}
	var revision string
	err = WithFileLock(store.Dir, func() error {
		chain, err := store.LoadChain()
		if err != nil {
			return fmt.Errorf("reopen-results: load chain: %w", err)
		}
		if chain.Count == 0 {
			return errors.New("reopen-results: lineage has no events")
		}
		verdict := store.Validate()
		if !verdict.Valid {
			return fmt.Errorf("reopen-results: chain integrity failed (%s); run 'biggz review repair %s' or export the lineage first", verdict.Reason, lineageID)
		}
		if state, _ := terminatedStateOf(chain); state != "" {
			return fmt.Errorf("reopen-results: lineage is already %s", state)
		}
		slots, err := capturedSlots(chain)
		if err != nil {
			return fmt.Errorf("reopen-results: %w", err)
		}
		if len(slots) == 0 {
			return errors.New("reopen-results: no captured lens slots to reopen")
		}
		payload, err := json.Marshal(reopenEventPayload{Schema: ReopenEventSchema, Slots: slots})
		if err != nil {
			return fmt.Errorf("reopen-results: marshal event: %w", err)
		}
		revision, err = store.appendLocked(chain.HeadHash, Record{
			Operation: ReopenOperation,
			Role:      string(model.RoleAuthor),
			Actor:     string(model.RoleAuthor),
			Timestamp: time.Now().Format(time.RFC3339Nano),
			Payload:   payload,
		})
		if err != nil {
			return fmt.Errorf("reopen-results: append event: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return revision, nil
}
