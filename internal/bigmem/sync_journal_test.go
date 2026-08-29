package bigmem

import (
	"encoding/json"
	"testing"
)

func TestEnqueueOrdered(t *testing.T) {
	s := openTestStore(t)
	tx, _ := s.db.Begin()
	p1, _ := json.Marshal(map[string]string{"v": "1"})
	seq1, err := enqueueSyncMutationTx(tx, "proj-j", "observation", "obs-1", "upsert", p1)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("enqueue1: %v", err)
	}
	p2, _ := json.Marshal(map[string]string{"v": "2"})
	seq2, err := enqueueSyncMutationTx(tx, "proj-j", "observation", "obs-2", "upsert", p2)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("enqueue2: %v", err)
	}
	_ = tx.Commit()
	if seq2 != seq1+1 {
		t.Fatalf("seq2=%d want %d", seq2, seq1+1)
	}
	list, _ := s.ListPendingMutations("proj-j", 10)
	if len(list) != 2 || list[0].Seq != seq1 || list[1].Seq != seq2 {
		t.Fatalf("ordered pending len=%d seq %d,%d", len(list), list[0].Seq, list[1].Seq)
	}
	if list[0].Disposition != "pending" {
		t.Errorf("disp %q", list[0].Disposition)
	}
	st, _ := s.GetSyncState("proj-j")
	if st.Lifecycle != SyncLifecyclePending || st.LastEnqueuedSeq != seq2 {
		t.Errorf("state %q enq %d", st.Lifecycle, st.LastEnqueuedSeq)
	}
}
func TestSyncEnqueueOrdered(t *testing.T) { TestEnqueueOrdered(t) }
func TestSyncJournalAckHealthy(t *testing.T) {
	s := openTestStore(t)
	tx, _ := s.db.Begin()
	seq, _ := enqueueSyncMutationTx(tx, "proj-ack", "observation", "obs-a", "upsert", []byte(`{}`))
	_ = tx.Commit()
	if err := s.AckSyncMutation(seq); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	list, _ := s.ListPendingMutations("proj-ack", 10)
	if len(list) != 0 {
		t.Fatalf("pending %d", len(list))
	}
	st, _ := s.GetSyncState("proj-ack")
	if st.Lifecycle != SyncLifecycleHealthy || st.LastAckedSeq != seq {
		t.Errorf("ack state %q %d", st.Lifecycle, st.LastAckedSeq)
	}
}
func TestSyncJournalViaSave(t *testing.T) {
	s := openTestStore(t)
	obs := &Observation{Title: "J1", Type: "decision", Content: "via-save", Project: "proj-save"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	list, _ := s.ListPendingMutations("proj-save", 10)
	if len(list) != 1 || list[0].EntityKey != obs.ID {
		t.Fatalf("via Save pending %d key %q", len(list), list[0].EntityKey)
	}
	st, _ := s.GetSyncState("proj-save")
	if st.Lifecycle != SyncLifecyclePending {
		t.Errorf("lifecycle %q", st.Lifecycle)
	}
}
func TestSyncJournalPendingLimit(t *testing.T) {
	s := openTestStore(t)
	tx, _ := s.db.Begin()
	for i := 0; i < 5; i++ {
		_, _ = enqueueSyncMutationTx(tx, "proj-lim", "observation", "k", "upsert", []byte(`{}`))
	}
	_ = tx.Commit()
	list, _ := s.ListPendingMutations("proj-lim", 2)
	if len(list) != 2 {
		t.Fatalf("limit %d", len(list))
	}
}
