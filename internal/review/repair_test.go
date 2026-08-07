package review

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/model"
)

// appendTestRecords appends n simple records to a fresh store inside a git
// repo (Repair resolves the store from the repo), returning the repo, the
// store, and the head revision after each append.
func appendTestRecords(t *testing.T, lineageID string, n int) (string, *Store, []string) {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	revisions := make([]string, 0, n)
	prev := ""
	for i := 0; i < n; i++ {
		rec := Record{
			Operation: "test_event",
			Role:      string(model.RoleReviewer),
			Actor:     string(model.RoleReviewer),
			Timestamp: time.Now().Format(time.RFC3339Nano),
			Payload:   []byte(`{"n":` + strconv.Itoa(i) + `}`),
		}
		rev, err := store.Append(prev, rec)
		if err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
		revisions = append(revisions, rev)
		prev = rev
	}
	return repo, store, revisions
}

func TestRepair_ChainIntactIsNoOp(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "repair-intact", 3)

	report, err := Repair(repo, "repair-intact")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if report.Repaired {
		t.Fatalf("repair = %+v, want no-op", report)
	}
	if report.Action != "none" || report.HeadHash != revisions[2] || report.EventCount != 3 {
		t.Fatalf("report = %+v, want intact with head %s", report, revisions[2])
	}
	head, err := readHEAD(store.Dir)
	if err != nil || head != revisions[2] {
		t.Fatalf("HEAD = %q (err %v), must be untouched", head, err)
	}
}

func TestRepair_TruncatesCorruptTail(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "repair-tail", 4)

	// Corrupt the HEAD event file (content that no longer parses as JSON).
	tail := revisions[3]
	if err := os.WriteFile(filepath.Join(store.Dir, tail), []byte("garbage"), 0644); err != nil {
		t.Fatalf("corrupt tail: %v", err)
	}

	report, err := Repair(repo, "repair-tail")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if !report.Repaired || report.Action != "truncated_tail" {
		t.Fatalf("report = %+v, want truncated_tail", report)
	}
	if report.HeadHash != revisions[2] || report.EventCount != 3 {
		t.Fatalf("head = %s (count %d), want %s (3)", report.HeadHash, report.EventCount, revisions[2])
	}
	if report.Truncated < 1 {
		t.Fatalf("truncated = %d, want >= 1", report.Truncated)
	}

	// The store is now valid and status recovers.
	verdict := store.Validate()
	if !verdict.Valid {
		t.Fatalf("chain must validate after repair: %s", verdict.Reason)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain after repair: %v", err)
	}
	if chain.Count != 3 || chain.HeadHash != revisions[2] {
		t.Fatalf("chain = %d events, head %s; want 3, %s", chain.Count, chain.HeadHash, revisions[2])
	}
}

func TestRepair_MidChainCorruptionRefuses(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "repair-mid", 4)

	// Corrupt a MIDDLE event (not the tail).
	middle := revisions[1]
	if err := os.WriteFile(filepath.Join(store.Dir, middle), []byte("garbage"), 0644); err != nil {
		t.Fatalf("corrupt middle: %v", err)
	}

	_, err := Repair(repo, "repair-mid")
	if err == nil {
		t.Fatal("mid-chain corruption must refuse to repair")
	}
	if !strings.Contains(err.Error(), "mid-chain") || !strings.Contains(err.Error(), "biggz review export") {
		t.Fatalf("error = %v, want mid-chain refusal naming export", err)
	}

	// Nothing was mutated.
	head, err := readHEAD(store.Dir)
	if err != nil || head != revisions[3] {
		t.Fatalf("HEAD = %q (err %v), must be untouched", head, err)
	}
}

func TestRepair_RederivesLostHEAD(t *testing.T) {
	repo, store, revisions := appendTestRecords(t, "repair-head", 3)

	// The HEAD file is lost; events remain.
	if err := os.Remove(filepath.Join(store.Dir, "HEAD")); err != nil {
		t.Fatalf("remove HEAD: %v", err)
	}

	report, err := Repair(repo, "repair-head")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if !report.Repaired || report.HeadHash != revisions[2] || report.EventCount != 3 {
		t.Fatalf("report = %+v, want re-derived head %s", report, revisions[2])
	}
	if verdict := store.Validate(); !verdict.Valid {
		t.Fatalf("chain must validate after HEAD re-derivation: %s", verdict.Reason)
	}
}

func TestRepair_EmptyStoreIsIntact(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if _, err := Open(repo, "repair-empty"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	report, err := Repair(repo, "repair-empty")
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if report.Repaired || report.EventCount != 0 {
		t.Fatalf("report = %+v, want intact empty store", report)
	}
}
