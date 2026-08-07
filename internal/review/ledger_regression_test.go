package review

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/contracts"
)

// mustMarshalReceipt renders a receipt exactly as writeReceiptLocked does.
func mustMarshalReceipt(t *testing.T, receipt PersistedReceipt) []byte {
	t.Helper()
	payload, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	return payload
}

// Baked fixture hashes (testdata/ledger-chain was generated once by the real
// engine, post-processed to platform-canonical forward-slash paths, and
// frozen; regenerating it would defeat the regression).
const (
	ledgerFixtureHeadHash    = "fca8736a18a58be144dd02ebc2ef2451bbfffc1f687a626d1df71aec8dc04993"
	ledgerFixtureGenesisHash = "6a2afd9b95a547c869f95cbf3f13fafa43c2d8d3796cd1ab544227bdaac9521a"
	ledgerFixtureEventCount  = 5
)

// contractSchemaIDs maps a chain event operation to the $id of the schema
// that formalizes its payload envelope (mirrors the mapping documented in
// contracts/README.md).
var contractSchemaIDs = map[string]string{
	"start_review":          "https://biggz-ai.dev/contracts/review-integration/v1/schemas/start.schema.json",
	LensResultOperation:     "https://biggz-ai.dev/contracts/review-integration/v1/schemas/lens-result-event.schema.json",
	CompleteReviewOperation: "https://biggz-ai.dev/contracts/review-integration/v1/schemas/complete-event.schema.json",
	RefutationOperation:     "https://biggz-ai.dev/contracts/review-integration/v1/schemas/refutation-event.schema.json",
	InvalidateOperation:     "https://biggz-ai.dev/contracts/review-integration/v1/schemas/invalidate-event.schema.json",
	WithdrawOperation:       "https://biggz-ai.dev/contracts/review-integration/v1/schemas/withdraw-event.schema.json",
	DisposeOperation:        "https://biggz-ai.dev/contracts/review-integration/v1/schemas/dispose-event.schema.json",
	ReopenOperation:         "https://biggz-ai.dev/contracts/review-integration/v1/schemas/reopen-event.schema.json",
}

const (
	recordSchemaID  = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/record.schema.json"
	receiptSchemaID = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/receipt.schema.json"
)

// copyLedgerFixture materializes the frozen testdata/ledger-chain store into
// a fresh temp directory and returns its store.
func copyLedgerFixture(t *testing.T) *Store {
	t.Helper()
	src := filepath.Join("testdata", "ledger-chain")
	dir := t.TempDir()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
	if err != nil {
		t.Fatalf("materialize fixture: %v", err)
	}
	return OpenWithDir(dir, "ledger-regression")
}

// TestLedgerRegression_BakedChainStillLoads pins additivity: a chain baked
// BEFORE the contracts formalization layer loads, validates, and finalizes
// its receipt identically WITH the layer present. The contracts layer is
// test-only and observational; it must never change a ledger byte.
func TestLedgerRegression_BakedChainStillLoads(t *testing.T) {
	store := copyLedgerFixture(t)

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != ledgerFixtureEventCount {
		t.Fatalf("Count = %d, want %d", chain.Count, ledgerFixtureEventCount)
	}
	if chain.HeadHash != ledgerFixtureHeadHash {
		t.Errorf("HeadHash = %s, want the frozen %s", chain.HeadHash, ledgerFixtureHeadHash)
	}
	if chain.GenesisHash != ledgerFixtureGenesisHash {
		t.Errorf("GenesisHash = %s, want the frozen %s", chain.GenesisHash, ledgerFixtureGenesisHash)
	}
	// The baked genesis is a start_review with the frozen start plan.
	if chain.Records[0].Operation != "start_review" {
		t.Errorf("genesis operation = %q, want start_review", chain.Records[0].Operation)
	}
	var plan StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil {
		t.Fatalf("genesis payload: %v", err)
	}
	if plan.Schema != ReviewStartEventSchema || plan.CorrectionBudget <= 0 {
		t.Errorf("genesis start plan = %+v, want a frozen plan with the start schema", plan)
	}

	verdict := store.Validate()
	if !verdict.Valid {
		t.Fatalf("IntegrityVerdict invalid: %s", verdict.Reason)
	}

	ref := receiptArtifactOf(chain)
	if ref == nil {
		t.Fatal("receiptArtifactOf found no receipt reference")
	}
	receiptBytes, err := os.ReadFile(filepath.Join(store.Dir, ref.Path))
	if err != nil {
		t.Fatalf("read receipt artifact: %v", err)
	}
	stored, err := readReceiptFile(store, completeEventPayload{
		Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash,
	})
	if err != nil {
		t.Fatalf("readReceiptFile: %v", err)
	}
	if err := stored.Validate(); err != nil {
		t.Fatalf("PersistedReceipt.Validate: %v", err)
	}
	if stored.ReceiptHash != ref.Hash {
		t.Errorf("receipt self-hash = %s, want the recorded reference %s", stored.ReceiptHash, ref.Hash)
	}
	if !bytes.Equal(receiptBytes, mustMarshalReceipt(t, stored)) {
		t.Error("persisted receipt bytes differ from the marshaled record (additivity violated)")
	}
}

// TestLedgerRegression_BakedChainConformsToContracts asserts the frozen
// bytes ALSO conform to the contracts formalization layer — every event
// payload, every record, and the persisted receipt validate against their
// schemas — proving the layer describes real emitted bytes without touching
// them.
func TestLedgerRegression_BakedChainConformsToContracts(t *testing.T) {
	store := copyLedgerFixture(t)
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	for index := range chain.Records {
		rec := chain.Records[index]
		recordBytes, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("record %d marshal: %v", index, err)
		}
		if err := contracts.ValidateEnvelope(recordSchemaID, recordBytes); err != nil {
			t.Fatalf("frozen record %d (%s) rejected by record.schema.json: %v", index, rec.Operation, err)
		}
		id, ok := contractSchemaIDs[rec.Operation]
		if !ok {
			// Marker events (e.g. in_review) carry no payload envelope and
			// nothing to formalize beyond the record itself.
			if len(rec.Payload) == 0 || bytes.Equal(bytes.TrimSpace(rec.Payload), []byte("null")) {
				continue
			}
			t.Fatalf("frozen record %d operation %q has no formalized payload schema", index, rec.Operation)
		}
		if err := contracts.ValidateEnvelope(id, rec.Payload); err != nil {
			t.Fatalf("frozen record %d payload (%s) rejected by its event schema: %v", index, rec.Operation, err)
		}
	}
	ref := receiptArtifactOf(chain)
	if ref == nil {
		t.Fatal("receiptArtifactOf found no receipt reference")
	}
	receiptBytes, err := os.ReadFile(filepath.Join(store.Dir, ref.Path))
	if err != nil {
		t.Fatalf("read receipt artifact: %v", err)
	}
	if err := contracts.ValidateEnvelope(receiptSchemaID, receiptBytes); err != nil {
		t.Fatalf("frozen receipt rejected by receipt.schema.json: %v", err)
	}
}
