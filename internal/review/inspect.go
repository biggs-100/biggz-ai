// Event inspection and schema registry — `review inspect` and `review schema`
// (Debt D3).
//
// Inspect lists every event file in chain order with its operation, payload
// schema, size, and content hash (the file name); --json emits full event
// summaries without payload dumps — for lens_result events only the subject
// hash, lens, order, and manifest reference are surfaced.
//
// Schema lists the event/artifact schemas biggz understands with their schema
// identifiers; `schema --event <name>` prints the documented field set, kept
// in sync with the code constants.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// InspectSchema identifies the inspect output envelope.
	InspectSchema = "biggz-ai.review-inspect/v1"
)

// EventInspectSummary is the inspection surface of one event. Payloads are
// never dumped: lens_result events surface subject_hash + lens + order +
// manifest only; complete_review events surface the receipt reference;
// dispose/reopen events surface the disposed slot.
type EventInspectSummary struct {
	Revision      string    `json:"revision"`
	Operation     string    `json:"operation"`
	Schema        string    `json:"schema"`
	Role          string    `json:"role"`
	Actor         string    `json:"actor"`
	Timestamp     string    `json:"timestamp"`
	PrevRevision  string    `json:"prev_revision"`
	Size          int64     `json:"size"`
	SubjectHash   string    `json:"subject_hash,omitempty"`
	Lens          string    `json:"lens,omitempty"`
	Order         *int      `json:"order,omitempty"`
	ManifestPath  string    `json:"manifest_path,omitempty"`
	ReceiptPath   string    `json:"receipt_path,omitempty"`
	ReceiptHash   string    `json:"receipt_hash,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	DisposedSlots []SlotRef `json:"disposed_slots,omitempty"`
}

// InspectResult is the full per-event inspection of a lineage in chain order.
type InspectResult struct {
	Schema      string                `json:"schema"`
	LineageID   string                `json:"lineage_id"`
	HeadHash    string                `json:"head_hash"`
	GenesisHash string                `json:"genesis_hash"`
	EventCount  int                   `json:"event_count"`
	Events      []EventInspectSummary `json:"events"`
}

// Inspect returns the per-event inspection of a lineage in chain order.
func Inspect(repo, lineageID string) (InspectResult, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return InspectResult{}, fmt.Errorf("inspect: open store: %w", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		return InspectResult{}, fmt.Errorf("inspect: load chain: %w", err)
	}
	result := InspectResult{
		Schema: InspectSchema, LineageID: lineageID,
		HeadHash: chain.HeadHash, GenesisHash: chain.GenesisHash,
		EventCount: chain.Count, Events: make([]EventInspectSummary, 0, chain.Count),
	}
	revisions := recordRevisions(chain)
	for index := range chain.Records {
		rec := &chain.Records[index]
		summary := EventInspectSummary{
			Revision: revisions[index], Operation: rec.Operation,
			Schema: payloadSchemaOf(rec.Payload), Role: rec.Role, Actor: rec.Actor,
			Timestamp: rec.Timestamp, PrevRevision: rec.PrevRevision,
		}
		if info, err := os.Stat(filepath.Join(store.Dir, revisions[index])); err == nil {
			summary.Size = info.Size()
		}
		switch rec.Operation {
		case LensResultOperation:
			var payload lensResultEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err == nil {
				summary.SubjectHash = payload.SubjectHash
				summary.Lens = payload.Lens
				order := payload.SelectedOrder
				summary.Order = &order
				summary.ManifestPath = payload.ManifestPath
			}
		case CompleteReviewOperation:
			var evt completeEventPayload
			if err := json.Unmarshal(rec.Payload, &evt); err == nil {
				summary.ReceiptPath = evt.ReceiptPath
				summary.ReceiptHash = evt.ReceiptHash
			}
		case DisposeOperation:
			var payload disposeEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err == nil {
				summary.Lens = payload.Lens
				order := payload.Order
				summary.Order = &order
				summary.Reason = payload.Reason
			}
		case ReopenOperation:
			var payload reopenEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err == nil {
				summary.DisposedSlots = payload.Slots
			}
		}
		result.Events = append(result.Events, summary)
	}
	return result, nil
}

// payloadSchemaOf extracts the payload schema identifier, or "" when the
// payload carries none (legacy genesis subjects).
func payloadSchemaOf(payload json.RawMessage) string {
	var envelope struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return ""
	}
	return envelope.Schema
}

// ---------------------------------------------------------------------------
// Schema registry
// ---------------------------------------------------------------------------

// SchemaInfo describes one event/artifact schema biggz understands.
type SchemaInfo struct {
	Name     string   `json:"name"`
	SchemaID string   `json:"schema_id"`
	Fields   []string `json:"fields"`
}

// schemaRegistry is the concise documented field set for every event and
// artifact schema, kept in sync with the code constants.
var schemaRegistry = []SchemaInfo{
	{
		Name: "start_review", SchemaID: ReviewStartEventSchema,
		Fields: []string{"schema", "repository", "commit_sha", "base_ref", "original_changed_lines",
			"correction_budget", "max_correction_attempts", "lenses", "risk_tier", "lens_plan"},
	},
	{
		Name: "lens_result", SchemaID: LensResultEventSchema,
		Fields: []string{"schema", "lineage_id", "expected_revision", "subject_hash", "lens",
			"selected_order", "admission_decision", "inspection", "canonical_payload",
			"canonical_payload_sha256", "result_hash", "candidate_causal_finding_ids",
			"manifest_sha256", "manifest_path", "result"},
	},
	{
		Name: "refutation", SchemaID: RefutationEventSchema,
		Fields: []string{"schema", "lineage_id", "verdicts: [{finding_id, verdict, evidence}]"},
	},
	{
		Name: "dispose", SchemaID: DisposeEventSchema,
		Fields: []string{"schema", "lens", "order", "reason"},
	},
	{
		Name: "reopen", SchemaID: ReopenEventSchema,
		Fields: []string{"schema", "slots: [{lens, order}]"},
	},
	{
		Name: "invalidate", SchemaID: InvalidateEventSchema,
		Fields: []string{"schema", "reason"},
	},
	{
		Name: "withdraw", SchemaID: WithdrawEventSchema,
		Fields: []string{"schema"},
	},
	{
		Name: "complete_review", SchemaID: FinalizeEventSchema,
		Fields: []string{"schema", "receipt_path", "receipt_hash"},
	},
	{
		Name: "receipt", SchemaID: ReviewReceiptSchema,
		Fields: []string{"schema", "lineage_id", "generation", "genesis_revision", "head_revision",
			"base_tree", "initial_review_tree", "final_candidate_tree", "paths_digest",
			"fix_delta_hash", "policy_hash", "evidence_hash", "risk_tier", "selected_lenses",
			"lens_subjects", "resolved_finding_ids", "standing_finding_ids", "terminal_state",
			"receipt_hash"},
	},
	{
		Name: "manifest", SchemaID: ManifestFileSchema,
		Fields: []string{"schema", "lineage_id", "base_tree", "candidate_tree",
			"manifest_sha256", "paths", "entries"},
	},
}

// SchemaList returns every schema biggz understands.
func SchemaList() []SchemaInfo {
	list := make([]SchemaInfo, len(schemaRegistry))
	copy(list, schemaRegistry)
	return list
}

// SchemaInfoOf returns the documented schema for one event/artifact name.
func SchemaInfoOf(name string) (*SchemaInfo, error) {
	for index := range schemaRegistry {
		if schemaRegistry[index].Name == name {
			info := schemaRegistry[index]
			return &info, nil
		}
	}
	return nil, fmt.Errorf("unknown schema %q (use 'biggz review schema' to list the supported names)", name)
}

// SchemaNames returns the supported schema names in registry order.
func SchemaNames() []string {
	names := make([]string, len(schemaRegistry))
	for index, info := range schemaRegistry {
		names[index] = info.Name
	}
	return names
}
