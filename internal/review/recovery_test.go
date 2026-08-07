// Debt D3 tests: recover, reclaim, reconcile-authority, dispose-result,
// reopen-results, inspect, schema, retry-final-verification.
package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/model"
)

// ---------------------------------------------------------------------------
// recover
// ---------------------------------------------------------------------------

func TestRecover_AuthorityIntactIsNoOp(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "recover-intact", 3)

	report, err := Recover(repo, "recover-intact")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if report.Recovered {
		t.Fatalf("recover = %+v, want no-op", report)
	}
	if report.Action != "none" || report.HeadHash != revisions[2] || report.EventCount != 3 {
		t.Fatalf("report = %+v, want intact with head %s", report, revisions[2])
	}
	if report.Detail != "authority intact" {
		t.Errorf("detail = %q, want authority intact", report.Detail)
	}
	head, err := readHEAD(store.Dir)
	if err != nil || head != revisions[2] {
		t.Fatalf("HEAD = %q (err %v), must be untouched", head, err)
	}
}

func TestRecover_RestoresLostHEAD(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "recover-lost", 4)

	if err := os.Remove(filepath.Join(store.Dir, "HEAD")); err != nil {
		t.Fatalf("remove HEAD: %v", err)
	}

	report, err := Recover(repo, "recover-lost")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Recovered || report.Action != "head_restored" {
		t.Fatalf("report = %+v, want head_restored", report)
	}
	if report.HeadHash != revisions[3] || report.EventCount != 4 {
		t.Fatalf("head = %s (count %d), want %s (4)", report.HeadHash, report.EventCount, revisions[3])
	}
	head, err := readHEAD(store.Dir)
	if err != nil || head != revisions[3] {
		t.Fatalf("HEAD = %q (err %v), want restored %s", head, err, revisions[3])
	}
	if verdict := store.Validate(); !verdict.Valid {
		t.Fatalf("chain must validate after recovery: %s", verdict.Reason)
	}
}

func TestRecover_DeepestVerifiedChainWins(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "recover-orphan", 2)
	// An orphan chain not linked to the main chain: 3 records chained off
	// revisions[0]. The main chain is 2 deep, the orphan 3 deep.
	orphanPrev := revisions[0]
	for i := 0; i < 3; i++ {
		rev, err := store.Append(orphanPrev, Record{
			Operation: "orphan_event",
			Role:      "Lead",
			Actor:     "tester",
			Timestamp: "2026-07-28T00:00:00Z",
			Payload:   []byte(`{"orphan":true}`),
		})
		if err != nil {
			t.Fatalf("append orphan %d: %v", i, err)
		}
		orphanPrev = rev
	}
	// Lose HEAD: recover must pick the DEEPEST verified chain (the orphan).
	if err := os.Remove(filepath.Join(store.Dir, "HEAD")); err != nil {
		t.Fatalf("remove HEAD: %v", err)
	}

	report, err := Recover(repo, "recover-orphan")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if !report.Recovered || report.HeadHash != orphanPrev || report.EventCount != 4 {
		t.Fatalf("report = %+v, want deepest chain head %s with 4 events", report, orphanPrev)
	}
}

func TestRecover_MidChainCorruptionRefuses(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "recover-mid", 4)

	middle := revisions[1]
	if err := os.WriteFile(filepath.Join(store.Dir, middle), []byte("garbage"), 0644); err != nil {
		t.Fatalf("corrupt middle: %v", err)
	}

	_, err := Recover(repo, "recover-mid")
	if err == nil {
		t.Fatal("mid-chain corruption must refuse to recover")
	}
	if !strings.Contains(err.Error(), "mid-chain") || !strings.Contains(err.Error(), "biggz review export") {
		t.Fatalf("error = %v, want mid-chain refusal naming export", err)
	}
	head, err := readHEAD(store.Dir)
	if err != nil || head != revisions[3] {
		t.Fatalf("HEAD = %q (err %v), must be untouched", head, err)
	}
}

func TestRecover_HEADNamingCorruptTailRefuses(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "recover-tail", 3)

	// The HEAD event file itself is corrupt: that is a corrupt TAIL, which
	// belongs to repair — recover only restores a lost HEAD and never
	// truncates.
	tail := revisions[2]
	if err := os.WriteFile(filepath.Join(store.Dir, tail), []byte("garbage"), 0644); err != nil {
		t.Fatalf("corrupt tail: %v", err)
	}

	_, err := Recover(repo, "recover-tail")
	if err == nil {
		t.Fatal("a HEAD naming a corrupt event must refuse recovery")
	}
	if !strings.Contains(err.Error(), "repair") || !strings.Contains(err.Error(), "recover only restores a missing HEAD") {
		t.Fatalf("error = %v, want repair refusal for a corrupt tail", err)
	}
	head, err := readHEAD(store.Dir)
	if err != nil || head != revisions[2] {
		t.Fatalf("HEAD = %q (err %v), must be untouched", head, err)
	}
}

func TestRecover_EmptyStoreIsNoOp(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if _, err := Open(repo, "recover-empty"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	report, err := Recover(repo, "recover-empty")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if report.Recovered || report.EventCount != 0 {
		t.Fatalf("report = %+v, want intact empty store", report)
	}
}

// ---------------------------------------------------------------------------
// reclaim
// ---------------------------------------------------------------------------

// finalizedLineage drives a full start + capture + finalize lifecycle and
// returns the repo, lineage id, and store.
func finalizedLineage(t *testing.T, lineageID string) (string, *Store) {
	t.Helper()
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, lineageID, []string{"risk"}, "")
	captureLens(t, repo, lineageID, head, "risk", 0)
	if _, err := Finalize(repo, lineageID); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	return repo, store
}

// writeStrayArtifact writes an orphaned file with a self-consistent
// content-addressed name under the lineage dir.
func writeStrayArtifact(t *testing.T, store *Store, dirName, base string) string {
	t.Helper()
	content := []byte(`{"schema":"biggz-ai.stray/v1","note":"orphaned ` + base + `"}`)
	name := sha256Hex(content)
	rel := dirName + "/" + name + ".json"
	path := filepath.Join(store.Dir, dirName, name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write stray: %v", err)
	}
	return rel
}

func TestReclaim_MovesOrphansOnly(t *testing.T) {
	repo, store := finalizedLineage(t, "reclaim-happy")

	// The referenced manifest + receipt exist after finalize.
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	referenced, err := referencedArtifacts(chain)
	if err != nil {
		t.Fatalf("referencedArtifacts: %v", err)
	}
	if len(referenced) != 2 {
		t.Fatalf("referenced artifacts = %d, want 2 (manifest + receipt)", len(referenced))
	}
	eventsBefore := len(listEventFiles(t, store.Dir))

	// Two orphaned artifacts: one manifest, one receipt.
	strayManifest := writeStrayArtifact(t, store, ManifestsDirName, "manifest")
	strayReceipt := writeStrayArtifact(t, store, ReceiptsDirName, "receipt")

	report, err := Reclaim(repo, "reclaim-happy")
	if err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
	if report.Reclaimed != 2 {
		t.Fatalf("reclaimed = %d, want 2 (%+v)", report.Reclaimed, report.Paths)
	}
	if report.TrashDir == "" {
		t.Fatal("trash dir must be reported")
	}
	wantPaths := []string{strayManifest, strayReceipt}
	if !reflect.DeepEqual(report.Paths, wantPaths) {
		t.Errorf("paths = %v, want %v", report.Paths, wantPaths)
	}

	// Orphans moved: gone from their homes, present under trash/<ts>/.
	for _, rel := range wantPaths {
		home := filepath.Join(store.Dir, filepath.FromSlash(rel))
		if _, err := os.Stat(home); !os.IsNotExist(err) {
			t.Errorf("orphan %s must not remain at its original path", rel)
		}
		trashed := filepath.Join(store.Dir, filepath.FromSlash(report.TrashDir), filepath.FromSlash(rel))
		if _, err := os.Stat(trashed); err != nil {
			t.Errorf("orphan %s must be preserved under trash: %v", rel, err)
		}
	}
	// Referenced artifacts untouched.
	for rel := range referenced {
		if _, err := os.Stat(filepath.Join(store.Dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("referenced artifact %s must be untouched: %v", rel, err)
		}
	}
	// Chain events untouched.
	if after := len(listEventFiles(t, store.Dir)); after != eventsBefore {
		t.Errorf("event files changed by reclaim: %d → %d", eventsBefore, after)
	}
	if verdict := store.Validate(); !verdict.Valid {
		t.Errorf("chain must stay valid after reclaim: %s", verdict.Reason)
	}
}

func TestReclaim_NoOrphansIsNoOp(t *testing.T) {
	repo, store := finalizedLineage(t, "reclaim-clean")

	report, err := Reclaim(repo, "reclaim-clean")
	if err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
	if report.Reclaimed != 0 {
		t.Fatalf("reclaimed = %d, want 0", report.Reclaimed)
	}
	if report.Detail != "no orphaned artifacts" {
		t.Errorf("detail = %q", report.Detail)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, "trash")); !os.IsNotExist(err) {
		t.Error("no trash dir may be created when there is nothing to reclaim")
	}
}

func TestReclaim_BrokenChainFailsClosed(t *testing.T) {
	repo, store := finalizedLineage(t, "reclaim-broken")

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) != 64 {
			continue
		}
		path := filepath.Join(store.Dir, entry.Name())
		original, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(original, []byte("tamper")...), 0644); err != nil {
			t.Fatalf("tamper: %v", err)
		}
		break
	}

	_, err = Reclaim(repo, "reclaim-broken")
	if err == nil || !strings.Contains(err.Error(), "load chain") {
		t.Fatalf("error = %v, want fail-closed load chain error", err)
	}
}

// ---------------------------------------------------------------------------
// reconcile-authority
// ---------------------------------------------------------------------------

func TestReconcileAuthority_MissingThenWriteThenCurrent(t *testing.T) {
	repo, _ := finalizedLineage(t, "reconcile-cycle")
	memDir := t.TempDir()
	mem, err := bigmem.Open(memDir)
	if err != nil {
		t.Fatalf("bigmem.Open: %v", err)
	}
	defer mem.Close()

	// Read-only: every mirror topic is missing; nothing refreshed.
	report, err := reconcileWithMem(repo, "reconcile-cycle", false, mem, "")
	if err != nil {
		t.Fatalf("reconcile read-only: %v", err)
	}
	if !report.ChainValid {
		t.Error("chain must be valid")
	}
	if report.Project == "" {
		t.Error("project must be detected")
	}
	if len(report.Topics) != 4 {
		t.Fatalf("topics = %d, want 4", len(report.Topics))
	}
	for _, topic := range report.Topics {
		if topic.Status != MirrorMissing {
			t.Errorf("topic %s status = %s, want missing", topic.Topic, topic.Status)
		}
	}
	if report.Refreshed != 0 {
		t.Errorf("refreshed = %d, want 0 read-only", report.Refreshed)
	}

	// --write: all four refreshed from native state.
	report, err = reconcileWithMem(repo, "reconcile-cycle", true, mem, "")
	if err != nil {
		t.Fatalf("reconcile --write: %v", err)
	}
	if report.Refreshed != 4 {
		t.Errorf("refreshed = %d, want 4", report.Refreshed)
	}
	for _, topic := range report.Topics {
		if topic.Status != MirrorCurrent {
			t.Errorf("topic %s status = %s, want current after write", topic.Topic, topic.Status)
		}
	}

	// Second read-only run: everything current, nothing to refresh.
	report, err = reconcileWithMem(repo, "reconcile-cycle", true, mem, "")
	if err != nil {
		t.Fatalf("reconcile second write: %v", err)
	}
	if report.Refreshed != 0 {
		t.Errorf("refreshed = %d, want 0 when current", report.Refreshed)
	}

	// The receipt mirror carries the persisted receipt surface.
	obs, err := findMirror(mem, mirrorTopic("reconcile-cycle", mirrorTopicReceipt))
	if err != nil {
		t.Fatalf("findMirror: %v", err)
	}
	if obs == nil {
		t.Fatal("receipt mirror missing after write")
	}
	var receiptMirror map[string]json.RawMessage
	if err := json.Unmarshal([]byte(obs.Content), &receiptMirror); err != nil {
		t.Fatalf("receipt mirror is not JSON: %v", err)
	}
	if _, ok := receiptMirror["receipt"]; !ok {
		t.Error("receipt mirror must carry the persisted receipt")
	}
}

func TestReconcileAuthority_DetectsStaleMirror(t *testing.T) {
	repo, _ := finalizedLineage(t, "reconcile-stale")
	memDir := t.TempDir()
	mem, err := bigmem.Open(memDir)
	if err != nil {
		t.Fatalf("bigmem.Open: %v", err)
	}
	defer mem.Close()

	if _, err := reconcileWithMem(repo, "reconcile-stale", true, mem, ""); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Tamper with one mirror's content: it must come back stale.
	topic := mirrorTopic("reconcile-stale", mirrorTopicTransaction)
	obs, err := findMirror(mem, topic)
	if err != nil || obs == nil {
		t.Fatalf("findMirror: obs=%v err=%v", obs, err)
	}
	if _, err := mem.Update(obs.ID, map[string]any{"content": "{\"tampered\":true}"}); err != nil {
		t.Fatalf("tamper mirror: %v", err)
	}

	report, err := reconcileWithMem(repo, "reconcile-stale", false, mem, "")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	for _, status := range report.Topics {
		if status.Topic == topic && status.Status != MirrorStale {
			t.Errorf("tampered topic status = %s, want stale", status.Status)
		}
	}
}

func TestReconcileAuthority_BrokenChainRefuses(t *testing.T) {
	repo, store := finalizedLineage(t, "reconcile-broken")
	memDir := t.TempDir()
	mem, err := bigmem.Open(memDir)
	if err != nil {
		t.Fatalf("bigmem.Open: %v", err)
	}
	defer mem.Close()

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) != 64 {
			continue
		}
		path := filepath.Join(store.Dir, entry.Name())
		original, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(original, []byte("tamper")...), 0644); err != nil {
			t.Fatalf("tamper: %v", err)
		}
		break
	}

	_, err = reconcileWithMem(repo, "reconcile-broken", true, mem, "")
	if err == nil || (!strings.Contains(err.Error(), "chain integrity failed") && !strings.Contains(err.Error(), "load chain")) {
		t.Fatalf("error = %v, want chain integrity refusal", err)
	}
}

// ---------------------------------------------------------------------------
// dispose-result / reopen-results
// ---------------------------------------------------------------------------

func TestDisposeResult_CycleBlocksAndReleasesFinalize(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "dispose-cycle", []string{"risk"}, "")
	captureLens(t, repo, "dispose-cycle", head, "risk", 0)

	revision, err := DisposeResult(repo, "dispose-cycle", "risk", 0, "scope changed")
	if err != nil {
		t.Fatalf("DisposeResult: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 4 || chain.HeadHash != revision {
		t.Fatalf("chain = %d events, head %s; want 4 with the dispose revision", chain.Count, chain.HeadHash)
	}
	last := chain.Records[chain.Count-1]
	if last.Operation != DisposeOperation || last.Role != string(model.RoleAuthor) {
		t.Fatalf("last event = %+v, want dispose by Author", last)
	}
	var payload disposeEventPayload
	if err := json.Unmarshal(last.Payload, &payload); err != nil {
		t.Fatalf("parse dispose payload: %v", err)
	}
	if payload.Schema != DisposeEventSchema || payload.Lens != "risk" || payload.Order != 0 || payload.Reason != "scope changed" {
		t.Errorf("dispose payload = %+v", payload)
	}

	// Finalize refuses the disposed planned slot without a fresh capture.
	_, err = Finalize(repo, "dispose-cycle")
	if err == nil {
		t.Fatal("finalize must refuse a disposed planned slot")
	}
	if !strings.Contains(err.Error(), "disposed") || !strings.Contains(err.Error(), "re-captured") {
		t.Errorf("error = %v, want disposed-and-not-re-captured refusal", err)
	}

	// Re-capture supersedes the disposal; finalize now succeeds.
	captureLens(t, repo, "dispose-cycle", head, "risk", 0)
	outcome, err := Finalize(repo, "dispose-cycle")
	if err != nil {
		t.Fatalf("Finalize after re-capture: %v", err)
	}
	payloadData, err := os.ReadFile(filepath.Join(store.Dir, outcome.ReceiptPath))
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	var receipt PersistedReceipt
	if err := json.Unmarshal(payloadData, &receipt); err != nil {
		t.Fatalf("parse receipt: %v", err)
	}
	// Only the live capture counts: exactly one risk lens subject.
	if len(receipt.LensSubjects) != 1 || receipt.LensSubjects[0].Lens != "risk" {
		t.Errorf("lens subjects = %+v, want exactly the one live risk slot", receipt.LensSubjects)
	}
}

func TestDisposeResult_RefusesUncapturedOrTwiceDisposedSlot(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "dispose-refuse", []string{"risk"}, "")
	captureLens(t, repo, "dispose-refuse", head, "risk", 0)

	// A slot that was never captured cannot be disposed.
	_, err := DisposeResult(repo, "dispose-refuse", "readability", 1, "")
	if err == nil || !strings.Contains(err.Error(), "no captured reviewer result") {
		t.Fatalf("error = %v, want no-captured-refusal", err)
	}
	// An unsupported lens is refused up front.
	if _, err := DisposeResult(repo, "dispose-refuse", "bogus", 0, ""); err == nil {
		t.Fatal("unsupported lens must be refused")
	}
	// A second dispose of the same slot is refused.
	if _, err := DisposeResult(repo, "dispose-refuse", "risk", 0, "first"); err != nil {
		t.Fatalf("first dispose: %v", err)
	}
	_, err = DisposeResult(repo, "dispose-refuse", "risk", 0, "second")
	if err == nil || !strings.Contains(err.Error(), "no captured reviewer result") {
		t.Fatalf("second dispose error = %v, want already-disposed refusal", err)
	}
	if chain, _ := store.LoadChain(); chain.Count != 4 {
		t.Errorf("event count = %d, want 4 (refused dispose must not append)", chain.Count)
	}
}

func TestDisposeResult_RefusedOnTerminatedLineage(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "dispose-terminal", []string{"risk"}, "")
	captureLens(t, repo, "dispose-terminal", head, "risk", 0)
	if _, err := Invalidate(repo, "dispose-terminal", "scope changed"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	_, err := DisposeResult(repo, "dispose-terminal", "risk", 0, "")
	if err == nil || !strings.Contains(err.Error(), "already invalidated") {
		t.Fatalf("error = %v, want invalidated refusal", err)
	}
}

func TestReopenResults_DisposesAllCapturedSlots(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "reopen-cycle", []string{"risk", "readability"}, "")
	captureLens(t, repo, "reopen-cycle", head, "risk", 0)
	captureLens(t, repo, "reopen-cycle", head, "readability", 1)

	revision, err := ReopenResults(repo, "reopen-cycle")
	if err != nil {
		t.Fatalf("ReopenResults: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	last := chain.Records[chain.Count-1]
	if last.Operation != ReopenOperation || last.Role != string(model.RoleAuthor) {
		t.Fatalf("last event = %+v, want reopen by Author", last)
	}
	if chain.HeadHash != revision {
		t.Errorf("head %s != reopen revision %s", chain.HeadHash, revision)
	}
	var payload reopenEventPayload
	if err := json.Unmarshal(last.Payload, &payload); err != nil {
		t.Fatalf("parse reopen payload: %v", err)
	}
	if payload.Schema != ReopenEventSchema || len(payload.Slots) != 2 {
		t.Fatalf("reopen payload = %+v, want 2 slots", payload)
	}
	wantSlots := []SlotRef{{Lens: "risk", Order: 0}, {Lens: "readability", Order: 1}}
	if !reflect.DeepEqual(payload.Slots, wantSlots) {
		t.Errorf("reopen slots = %v, want %v", payload.Slots, wantSlots)
	}

	// Finalize refuses every disposed planned slot until re-captured.
	_, err = Finalize(repo, "reopen-cycle")
	if err == nil || !strings.Contains(err.Error(), "disposed") {
		t.Fatalf("finalize error = %v, want disposed refusal", err)
	}
	// Re-capture both slots: finalize succeeds with a fresh receipt.
	captureLens(t, repo, "reopen-cycle", head, "risk", 0)
	captureLens(t, repo, "reopen-cycle", head, "readability", 1)
	if _, err := Finalize(repo, "reopen-cycle"); err != nil {
		t.Fatalf("Finalize after re-capture: %v", err)
	}
}

func TestReopenResults_RefusesWhenNothingCaptured(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "reopen-empty", []string{"risk"}, "")
	_, err := ReopenResults(repo, "reopen-empty")
	if err == nil || !strings.Contains(err.Error(), "no captured lens slots") {
		t.Fatalf("error = %v, want no-captured-refusal", err)
	}
}

// Superseded captures must not drive refutation requirements: dispose removes
// the discarded result's findings from the required refuter set.
func TestDisposeResult_DiscardedFindingsDoNotDemandRefutation(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "dispose-refute", []string{"risk"}, "")
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := CaptureBinding{
		Repo: repo, LineageID: "dispose-refute", TargetIdentity: head,
		Lens: "risk", Order: 0, ExpectedRevision: chain.HeadHash,
	}
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	payload := captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)
	// Rebuild the payload with an INFERENTIAL candidate-causal finding.
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	findings := envelope["findings"].([]any)
	finding := findings[0].(map[string]any)
	finding["evidence_class"] = "inferential"
	payload, err = json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, err := Capture(binding, payload); err != nil {
		t.Fatalf("Capture: %v", err)
	}

	// The finding demands a refuter verdict before finalize.
	if _, err := Finalize(repo, "dispose-refute"); err == nil || !strings.Contains(err.Error(), "refutation pending") {
		t.Fatalf("finalize = %v, want refutation pending", err)
	}

	// Disposing the slot removes the finding from the required set entirely.
	if _, err := DisposeResult(repo, "dispose-refute", "risk", 0, "result discarded"); err != nil {
		t.Fatalf("DisposeResult: %v", err)
	}
	fresh, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	freshSummary, err := RefutationSummaryOf(fresh)
	if err != nil {
		t.Fatalf("RefutationSummaryOf: %v", err)
	}
	if freshSummary.Total != 0 || freshSummary.Pending != 0 {
		t.Errorf("refutation summary = %+v, want total 0 pending 0 after dispose", freshSummary)
	}
}

// ---------------------------------------------------------------------------
// inspect
// ---------------------------------------------------------------------------

func TestInspect_ListsEventsInOrder(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "inspect-lines", []string{"risk"}, "")
	captureLens(t, repo, "inspect-lines", head, "risk", 0)

	result, err := Inspect(repo, "inspect-lines")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if result.Schema != InspectSchema || result.LineageID != "inspect-lines" {
		t.Errorf("result envelope = %+v", result)
	}
	if result.EventCount != 3 {
		t.Fatalf("event count = %d, want 3", result.EventCount)
	}
	if result.Events[0].Operation != "start_review" {
		t.Errorf("genesis operation = %q", result.Events[0].Operation)
	}
	if result.Events[0].Schema != ReviewStartEventSchema {
		t.Errorf("genesis schema = %q, want %q", result.Events[0].Schema, ReviewStartEventSchema)
	}
	// Revisions must be the content hashes in chain order.
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	wantRevisions := recordRevisions(chain)
	for index, event := range result.Events {
		if event.Revision != wantRevisions[index] {
			t.Errorf("event %d revision = %s, want %s", index, event.Revision, wantRevisions[index])
		}
		if event.Size <= 0 {
			t.Errorf("event %d size = %d, want > 0", index, event.Size)
		}
	}

	// lens_result summary: subject_hash + lens + order + manifest, no payload.
	lensEvent := result.Events[2]
	if lensEvent.Operation != LensResultOperation {
		t.Fatalf("event 2 operation = %q, want lens_result", lensEvent.Operation)
	}
	if lensEvent.SubjectHash == "" || lensEvent.Lens != "risk" || lensEvent.Order == nil || *lensEvent.Order != 0 {
		t.Errorf("lens summary = %+v, want risk order 0 with subject hash", lensEvent)
	}
	if !strings.HasPrefix(filepath.ToSlash(lensEvent.ManifestPath), "manifests/") {
		t.Errorf("manifest path = %q, want under manifests/", lensEvent.ManifestPath)
	}
}

// ---------------------------------------------------------------------------
// schema
// ---------------------------------------------------------------------------

func TestSchema_Registry(t *testing.T) {
	wantNames := []string{
		"start_review", "lens_result", "refutation", "dispose", "reopen",
		"invalidate", "withdraw", "complete_review", "receipt", "manifest",
	}
	list := SchemaList()
	if len(list) != len(wantNames) {
		t.Fatalf("schema count = %d, want %d", len(list), len(wantNames))
	}
	for index, info := range list {
		if info.Name != wantNames[index] {
			t.Errorf("schema[%d] = %s, want %s", index, info.Name, wantNames[index])
		}
		if info.SchemaID == "" || len(info.Fields) == 0 {
			t.Errorf("schema %s is incomplete: %+v", info.Name, info)
		}
	}
}

func TestSchema_InfoOf(t *testing.T) {
	info, err := SchemaInfoOf("lens_result")
	if err != nil {
		t.Fatalf("SchemaInfoOf(lens_result): %v", err)
	}
	if info.SchemaID != LensResultEventSchema {
		t.Errorf("schema id = %q, want %q", info.SchemaID, LensResultEventSchema)
	}
	for _, field := range []string{"subject_hash", "lens", "selected_order", "manifest_path", "result_hash"} {
		if !containsString(info.Fields, field) {
			t.Errorf("lens_result fields must contain %q, got %v", field, info.Fields)
		}
	}
	if _, err := SchemaInfoOf("nope"); err == nil {
		t.Error("unknown schema must error")
	}
}

// ---------------------------------------------------------------------------
// retry-final-verification
// ---------------------------------------------------------------------------

func TestRetryFinalVerification_Pass(t *testing.T) {
	repo, _ := finalizedLineage(t, "retry-pass")

	report, err := RetryFinalVerification(repo, "retry-pass")
	if err != nil {
		t.Fatalf("RetryFinalVerification: %v", err)
	}
	if !report.Passed || !report.ChainValid || !report.ReceiptMatch {
		t.Fatalf("report = %+v, want full PASS", report)
	}
	if report.ReceiptReMaterialized {
		t.Error("no re-materialization expected on a healthy lineage")
	}
	if report.ReceiptPath == "" || report.ReceiptHash == "" {
		t.Error("report must carry the receipt reference")
	}
}

func TestRetryFinalVerification_ReMaterializesMissingReceipt(t *testing.T) {
	repo, store := finalizedLineage(t, "retry-remat")

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	ref := receiptArtifactOf(chain)
	if ref == nil {
		t.Fatal("no receipt reference")
	}
	receiptPath := filepath.Join(store.Dir, ref.Path)
	original, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	originalName := filepath.Base(receiptPath)
	if err := os.Remove(receiptPath); err != nil {
		t.Fatalf("remove receipt: %v", err)
	}

	report, err := RetryFinalVerification(repo, "retry-remat")
	if err != nil {
		t.Fatalf("RetryFinalVerification: %v", err)
	}
	if !report.Passed || !report.ReceiptReMaterialized {
		t.Fatalf("report = %+v, want PASS with re-materialization", report)
	}
	rebuilt, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("rebuilt receipt missing: %v", err)
	}
	// Content-addressed: the same name with hash-identical bytes.
	if filepath.Base(receiptPath) != originalName {
		t.Errorf("rebuilt name = %s, want original %s", filepath.Base(receiptPath), originalName)
	}
	if !reflect.DeepEqual(rebuilt, original) {
		t.Error("rebuilt receipt bytes must be hash-identical to the original")
	}
}

func TestRetryFinalVerification_NotFinalizedFails(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "retry-open", []string{"risk"}, "")

	report, err := RetryFinalVerification(repo, "retry-open")
	if err != nil {
		t.Fatalf("RetryFinalVerification: %v", err)
	}
	if report.Passed {
		t.Fatal("an open lineage must not pass terminal verification")
	}
	// The chain itself is healthy; terminal verification still fails because
	// the lineage carries no complete_review receipt reference.
	found := false
	for _, reason := range report.Reasons {
		if strings.Contains(reason, "no complete_review") {
			found = true
		}
	}
	if !found {
		t.Errorf("reasons = %v, want no-complete_review reason", report.Reasons)
	}
}

func TestRetryFinalVerification_TamperedReceiptFailsWithoutOverwrite(t *testing.T) {
	repo, store := finalizedLineage(t, "retry-tamper")

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	ref := receiptArtifactOf(chain)
	receiptPath := filepath.Join(store.Dir, ref.Path)
	if err := os.WriteFile(receiptPath, []byte(`{"tampered":true}`), 0644); err != nil {
		t.Fatalf("tamper receipt: %v", err)
	}

	report, err := RetryFinalVerification(repo, "retry-tamper")
	if err != nil {
		t.Fatalf("RetryFinalVerification: %v", err)
	}
	if report.Passed || report.ReceiptMatch {
		t.Fatalf("report = %+v, want FAIL on a tampered receipt", report)
	}
	if report.ReceiptReMaterialized {
		t.Error("a tampered receipt must never be silently re-materialized over")
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil || string(data) != `{"tampered":true}` {
		t.Errorf("tampered receipt must stay untouched, got %q (err %v)", data, err)
	}
}

func TestRetryFinalVerification_BrokenChainFails(t *testing.T) {
	repo, store := finalizedLineage(t, "retry-broken")

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) != 64 {
			continue
		}
		path := filepath.Join(store.Dir, entry.Name())
		original, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(original, []byte("tamper")...), 0644); err != nil {
			t.Fatalf("tamper: %v", err)
		}
		break
	}

	// A broken chain cannot even be loaded: the retry fails closed with an
	// error naming the load failure (the operator repairs/re-covers first).
	_, err = RetryFinalVerification(repo, "retry-broken")
	if err == nil || !strings.Contains(err.Error(), "load chain") {
		t.Fatalf("error = %v, want load chain failure", err)
	}
}
