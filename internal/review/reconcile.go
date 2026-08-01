// Authority reconciliation — `biggz review reconcile-authority <lineage>`
// (Debt D3).
//
// Reconcile verifies consistency between the native lineage store (event
// chain + persisted receipt) and the BigMem mirror topics
// `sdd/<lineage>/review/{transaction,ledger,receipt,gate-context}`, reporting
// which mirrors are missing or stale against the native state. The lineage id
// plays the `{change}` role: biggz reviews are lineage-scoped, and every
// lineage maps to one mirror topic family.
//
// Read-only by default; `--write` refreshes the missing/stale mirrors from
// native state through the internal bigmem package (the same store the
// `biggz bigmem` CLI uses). Mirrors that are already current are never
// rewritten, so refresh does not churn revision counts.
package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/bigmem"
)

const (
	// ReconcileSchema identifies the reconcile report envelope.
	ReconcileSchema = "biggz-ai.review-reconcile/v1"

	mirrorTopicTransaction = "transaction"
	mirrorTopicLedger      = "ledger"
	mirrorTopicReceipt     = "receipt"
	mirrorTopicGateContext = "gate-context"
)

// MirrorStatus classifies one BigMem mirror against the native store.
type MirrorStatus string

const (
	MirrorCurrent MirrorStatus = "current"
	MirrorMissing MirrorStatus = "missing"
	MirrorStale   MirrorStatus = "stale"
)

// MirrorTopicStatus reports one mirror topic's reconciliation state.
type MirrorTopicStatus struct {
	Topic  string       `json:"topic"`
	Status MirrorStatus `json:"status"`
	Detail string       `json:"detail,omitempty"`
}

// ReconcileReport describes the outcome of `review reconcile-authority`.
type ReconcileReport struct {
	Schema     string               `json:"schema"`
	LineageID  string               `json:"lineage_id"`
	Project    string               `json:"project"`
	ChainValid bool                 `json:"chain_valid"`
	Topics     []MirrorTopicStatus  `json:"topics"`
	Refreshed  int                  `json:"refreshed"`
	Wrote      bool                 `json:"wrote"`
}

// mirrorTopic returns the BigMem topic key for one mirror kind of a lineage.
func mirrorTopic(lineageID, kind string) string {
	return "sdd/" + lineageID + "/review/" + kind
}

// mirrorPayload structs — each carries its own schema identifier so stale
// mirrors are recognizable independent of the topic key.

type transactionMirror struct {
	Schema        string `json:"schema"`
	LineageID     string `json:"lineage_id"`
	HeadHash      string `json:"head_hash"`
	GenesisHash   string `json:"genesis_hash"`
	EventCount    int    `json:"event_count"`
	LastOperation string `json:"last_operation"`
	RecordedAt    string `json:"recorded_at"`
}

type ledgerEntryMirror struct {
	Revision  string `json:"revision"`
	Operation string `json:"operation"`
	Schema    string `json:"schema"`
	Role      string `json:"role"`
	Actor     string `json:"actor"`
	Timestamp string `json:"timestamp"`
}

type ledgerMirror struct {
	Schema    string               `json:"schema"`
	LineageID string               `json:"lineage_id"`
	HeadHash  string               `json:"head_hash"`
	Events    []ledgerEntryMirror  `json:"events"`
}

type receiptMirror struct {
	Schema      string             `json:"schema"`
	LineageID   string             `json:"lineage_id"`
	ReceiptPath string             `json:"receipt_path,omitempty"`
	ReceiptHash string             `json:"receipt_hash,omitempty"`
	Receipt     *PersistedReceipt  `json:"receipt,omitempty"`
	Detail      string             `json:"detail,omitempty"`
}

type gateContextMirror struct {
	Schema        string `json:"schema"`
	LineageID     string `json:"lineage_id"`
	ChainValid    bool   `json:"chain_valid"`
	TerminalState string `json:"terminal_state"`
	ReceiptHash   string `json:"receipt_hash,omitempty"`
	LastOperation string `json:"last_operation"`
	EventCount    int    `json:"event_count"`
}

// mirrorPayloads derives the four mirror payloads from native state.
func mirrorPayloads(chain ValidatedChain, store *Store) (map[string][]byte, error) {
	payloads := make(map[string][]byte)
	lastOp := ""
	lastTS := ""
	if chain.Count > 0 {
		last := chain.Records[chain.Count-1]
		lastOp, lastTS = last.Operation, last.Timestamp
	}
	terminal := "in_review"
	if state, _ := terminatedStateOf(chain); state != "" {
		terminal = state
	} else if hasCompleteReview(chain) {
		terminal = "completed"
	}

	transaction := transactionMirror{
		Schema: "biggz-ai.review-mirror-transaction/v1", LineageID: chain.LineageID,
		HeadHash: chain.HeadHash, GenesisHash: chain.GenesisHash,
		EventCount: chain.Count, LastOperation: lastOp, RecordedAt: lastTS,
	}
	data, err := json.Marshal(transaction)
	if err != nil {
		return nil, err
	}
	payloads[mirrorTopicTransaction] = data

	revisions := recordRevisions(chain)
	ledger := ledgerMirror{
		Schema: "biggz-ai.review-mirror-ledger/v1", LineageID: chain.LineageID,
		HeadHash: chain.HeadHash, Events: make([]ledgerEntryMirror, 0, chain.Count),
	}
	for index := range chain.Records {
		rec := &chain.Records[index]
		ledger.Events = append(ledger.Events, ledgerEntryMirror{
			Revision: revisions[index], Operation: rec.Operation,
			Schema: payloadSchemaOf(rec.Payload), Role: rec.Role, Actor: rec.Actor,
			Timestamp: rec.Timestamp,
		})
	}
	data, err = json.Marshal(ledger)
	if err != nil {
		return nil, err
	}
	payloads[mirrorTopicLedger] = data

	receipt := receiptMirror{Schema: "biggz-ai.review-mirror-receipt/v1", LineageID: chain.LineageID}
	if ref := receiptArtifactOf(chain); ref != nil {
		stored, readErr := readReceiptFile(store, completeEventPayload{
			Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash,
		})
		if readErr != nil {
			receipt.Detail = "persisted receipt artifact is unreadable: " + readErr.Error()
		} else {
			receipt.ReceiptPath, receipt.ReceiptHash, receipt.Receipt = ref.Path, ref.Hash, &stored
		}
	} else {
		receipt.Detail = "no receipt (lineage not finalized)"
	}
	data, err = json.Marshal(receipt)
	if err != nil {
		return nil, err
	}
	payloads[mirrorTopicReceipt] = data

	gate := gateContextMirror{
		Schema: "biggz-ai.review-mirror-gate-context/v1", LineageID: chain.LineageID,
		ChainValid: true, TerminalState: terminal, LastOperation: lastOp, EventCount: chain.Count,
	}
	if ref := receiptArtifactOf(chain); ref != nil {
		gate.ReceiptHash = ref.Hash
	}
	data, err = json.Marshal(gate)
	if err != nil {
		return nil, err
	}
	payloads[mirrorTopicGateContext] = data
	return payloads, nil
}

// mirrorKinds is the stable mirror topic order.
var mirrorKinds = []string{mirrorTopicTransaction, mirrorTopicLedger, mirrorTopicReceipt, mirrorTopicGateContext}

// ReconcileAuthority reconciles the lineage's BigMem mirror topics against
// native state. Read-only unless write is true; with write, missing/stale
// mirrors are refreshed from native state. The BigMem store is the default
// user store (same as the `biggz bigmem` CLI).
func ReconcileAuthority(repo, lineageID string, write bool) (ReconcileReport, error) {
	mem, err := bigmem.Open("")
	if err != nil {
		return ReconcileReport{}, fmt.Errorf("reconcile-authority: open bigmem: %w", err)
	}
	defer mem.Close()
	return reconcileWithMem(repo, lineageID, write, mem, "")
}

// reconcileWithMem is the reconciler body with an explicit BigMem store and
// project name ("" auto-detects from the repository), for testability.
func reconcileWithMem(repo, lineageID string, write bool, mem *bigmem.Store, project string) (ReconcileReport, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return ReconcileReport{}, fmt.Errorf("reconcile-authority: open store: %w", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		return ReconcileReport{}, fmt.Errorf("reconcile-authority: load chain: %w", err)
	}
	if chain.Count == 0 {
		return ReconcileReport{}, errors.New("reconcile-authority: lineage has no events")
	}
	verdict := store.Validate()
	if !verdict.Valid {
		return ReconcileReport{}, fmt.Errorf(
			"reconcile-authority: chain integrity failed (%s); run 'biggz review repair %s' or 'biggz review recover %s' before reconciling mirrors",
			verdict.Reason, lineageID, lineageID)
	}
	if project == "" {
		project = detectProjectName(repo)
	}
	payloads, err := mirrorPayloads(chain, store)
	if err != nil {
		return ReconcileReport{}, fmt.Errorf("reconcile-authority: derive mirrors: %w", err)
	}
	report := ReconcileReport{
		Schema: ReconcileSchema, LineageID: lineageID, Project: project,
		ChainValid: true, Wrote: write,
		Topics: make([]MirrorTopicStatus, 0, len(mirrorKinds)),
	}
	for _, kind := range mirrorKinds {
		topic := mirrorTopic(lineageID, kind)
		content := string(payloads[kind])
		status := MirrorCurrent
		detail := ""
		obs, err := findMirror(mem, topic)
		if err != nil {
			return ReconcileReport{}, fmt.Errorf("reconcile-authority: read mirror %s: %w", topic, err)
		}
		switch {
		case obs == nil:
			status = MirrorMissing
		case obs.Content != content:
			status = MirrorStale
			detail = "content differs from native state"
		}
		if write && status != MirrorCurrent {
			if err := writeMirror(mem, obs, topic, mirrorTitle(lineageID, kind), content, project); err != nil {
				return ReconcileReport{}, fmt.Errorf("reconcile-authority: refresh mirror %s: %w", topic, err)
			}
			report.Refreshed++
			status = MirrorCurrent
			detail = ""
		}
		report.Topics = append(report.Topics, MirrorTopicStatus{Topic: topic, Status: status, Detail: detail})
	}
	return report, nil
}

// findMirror returns the latest observation for an exact topic key, or nil.
func findMirror(mem *bigmem.Store, topic string) (*bigmem.Observation, error) {
	results, err := mem.Search(topic, bigmem.SearchOptions{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// writeMirror refreshes one mirror topic: an existing observation is updated
// in place (its project is preserved); a missing one is saved with the
// detected project.
func writeMirror(mem *bigmem.Store, existing *bigmem.Observation, topic, title, content, project string) error {
	if existing != nil {
		_, err := mem.Update(existing.ID, map[string]any{"title": title, "content": content})
		return err
	}
	return mem.Save(&bigmem.Observation{
		TopicKey: topic, Title: title, Content: content,
		Type: "review", Scope: "project", Project: project,
	})
}

func mirrorTitle(lineageID, kind string) string {
	return "Review mirror " + kind + " — lineage " + lineageID
}

// detectProjectName derives the BigMem project name from the repository
// top-level directory name, mirroring the `biggz bigmem` CLI's auto-detection.
func detectProjectName(repo string) string {
	args := []string{"rev-parse", "--show-toplevel"}
	if repo != "" {
		args = append([]string{"-C", repo}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return ""
	}
	return filepath.Base(filepath.Clean(root))
}
