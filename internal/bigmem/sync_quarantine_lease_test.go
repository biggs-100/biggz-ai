package bigmem

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestQuarantineLogBlocked(t *testing.T) {
	s := openTestStore(t)
	tx, _ := s.db.Begin()
	pl1, _ := json.Marshal(map[string]string{"v": "10"})
	seq1, _ := enqueueSyncMutationTx(tx, "proj-q-blocked", "observation", "obs-10", "upsert", pl1)
	pl2, _ := json.Marshal(map[string]string{"v": "11"})
	seq2, _ := enqueueSyncMutationTx(tx, "proj-q-blocked", "observation", "obs-11", "upsert", pl2)
	_ = tx.Commit()
	ev := `{"reason":"irreparable","validator":"deterministic"}`
	if err := s.QuarantineIrreparable(seq1, ev); err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	var disp, got string
	_ = s.db.QueryRow(`SELECT disposition, evidence FROM sync_mutations WHERE seq=?`, seq1).Scan(&disp, &got)
	if disp != "quarantined" || got != ev {
		t.Fatalf("disp %q ev %q want quarantined %q", disp, got, ev)
	}
	list, _ := s.ListPendingMutations("proj-q-blocked", 10)
	if len(list) != 1 || list[0].Seq != seq2 {
		t.Fatalf("ListPending len %d want 1 seq %d", len(list), seq2)
	}
	st, _ := s.GetSyncState("proj-q-blocked")
	if st.Lifecycle != SyncLifecycleDegraded || st.ReasonCode == nil || *st.ReasonCode != "irreparable" || st.LastAckedSeq != seq1 {
		t.Fatalf("state %v", st)
	}
	if err := s.QuarantineIrreparable(seq1, ev); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	m := SyncMutation{Project: "p", Entity: "observation", EntityKey: "k", Payload: []byte(`{invalid`)}
	e1 := ValidateSyncMutation(m)
	e2 := ValidateSyncMutation(m)
	if (e1 == nil) != (e2 == nil) || (e1 != nil && e2 != nil && e1.Error() != e2.Error()) {
		t.Fatalf("validator not deterministic %v vs %v", e1, e2)
	}
}
func TestQuarantinePayloadTamper(t *testing.T) {
	s := openTestStore(t)
	tx, _ := s.db.Begin()
	pl, _ := json.Marshal(map[string]string{"id": "obs-tamper"})
	seq, _ := enqueueSyncMutationTx(tx, "proj-tamper", "observation", "obs-tamper", "upsert", pl)
	_ = tx.Commit()
	corrupt := []byte(`{invalid json`)
	m := SyncMutation{Project: "proj-tamper", Entity: "observation", EntityKey: "obs-tamper", Payload: corrupt}
	if err := ValidateSyncMutation(m); err == nil {
		t.Fatalf("expected irreparable")
	}
	ok, ev := IsIrreparablePayload(corrupt)
	if !ok || !json.Valid([]byte(ev)) {
		t.Fatalf("IsIrreparablePayload %v %q", ok, ev)
	}
	if err := s.QuarantineIrreparable(seq, ev); err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	var disp string
	_ = s.db.QueryRow(`SELECT disposition FROM sync_mutations WHERE seq=?`, seq).Scan(&disp)
	if disp != "quarantined" {
		t.Fatalf("disp %q", disp)
	}
	ok2, ev2 := IsIrreparablePayload(corrupt)
	if ok != ok2 || ev != ev2 {
		t.Fatalf("not deterministic")
	}
	if ok, _ := IsIrreparablePayload([]byte(`{"id":"ok"}`)); ok {
		t.Fatalf("valid should not be irreparable")
	}
}
func TestLeaseSplitBrain(t *testing.T) {
	s := openTestStore(t)
	target := "proj-lease-split"
	ok, err := s.AcquireSyncLease(target, "A", time.Minute)
	if err != nil || !ok {
		t.Fatalf("A acquire %v %v", ok, err)
	}
	st, _ := s.GetSyncState(target)
	if st.LeaseOwner == nil || *st.LeaseOwner != "A" || st.LeaseUntil == nil {
		t.Fatalf("lease %v", st)
	}
	if tu, _ := time.Parse(time.RFC3339, *st.LeaseUntil); !tu.After(time.Now()) {
		t.Fatalf("lease_until not future")
	}
	ok, _ = s.AcquireSyncLease(target, "B", time.Minute)
	if ok {
		t.Fatalf("B should be denied")
	}
	if err := s.ReleaseSyncLease(target, "B"); err == nil {
		t.Fatalf("B release should fail")
	}
	if err := s.ReleaseSyncLease(target, "A"); err != nil {
		t.Fatalf("A release %v", err)
	}
	st, _ = s.GetSyncState(target)
	if st.LeaseOwner != nil {
		t.Fatalf("lease_owner not nil after release %v", *st.LeaseOwner)
	}
	ok, _ = s.AcquireSyncLease(target, "A", time.Minute)
	if !ok {
		t.Fatalf("A reacquire")
	}
	_, _ = s.db.Exec(`UPDATE sync_state SET lease_until=? WHERE target_key=? AND project=?`, time.Now().Add(-time.Minute).Format(time.RFC3339), target, target)
	ok, _ = s.AcquireSyncLease(target, "C", time.Minute)
	if !ok {
		t.Fatalf("C post-expiry should succeed")
	}
	ok, _ = s.AcquireSyncLease(target, "C", time.Minute)
	if !ok {
		t.Fatalf("C renew should succeed")
	}
	var wg sync.WaitGroup
	res := make([]bool, 2)
	target2 := "proj-lease-concurrent"
	wg.Add(2)
	go func() { defer wg.Done(); ok, _ := s.AcquireSyncLease(target2, "owner1", time.Minute); res[0] = ok }()
	go func() { defer wg.Done(); time.Sleep(10 * time.Millisecond); ok, _ := s.AcquireSyncLease(target2, "owner2", time.Minute); res[1] = ok }()
	wg.Wait()
	if res[0] == res[1] {
		t.Fatalf("concurrent exactly one should succeed %v", res)
	}
}
func TestLeaseBackoff(t *testing.T) {
	s := openTestStore(t)
	target, project := "proj-backoff", "proj-backoff"
	if err := s.MarkSyncFailed(target, project, "network"); err != nil {
		t.Fatalf("failed1 %v", err)
	}
	st, _ := s.GetSyncState(project)
	if st.ConsecutiveFailures != 1 || st.BackoffUntil == nil {
		t.Fatalf("failures %d backoff %v", st.ConsecutiveFailures, st.BackoffUntil)
	}
	t1, _ := time.Parse(time.RFC3339, *st.BackoffUntil)
	if !t1.After(time.Now()) {
		t.Fatalf("backoff not future")
	}
	time.Sleep(10 * time.Millisecond)
	if err := s.MarkSyncFailed(target, project, "network"); err != nil {
		t.Fatalf("failed2 %v", err)
	}
	st2, _ := s.GetSyncState(project)
	if st2.ConsecutiveFailures != 2 {
		t.Fatalf("failures %d want 2", st2.ConsecutiveFailures)
	}
	t2, _ := time.Parse(time.RFC3339, *st2.BackoffUntil)
	if !t2.After(t1) || time.Until(t2) <= time.Until(t1) {
		t.Fatalf("backoff not increasing")
	}
	if err := s.MarkSyncSucceeded(target, project); err != nil {
		t.Fatalf("succeeded %v", err)
	}
	st3, _ := s.GetSyncState(project)
	if st3.ConsecutiveFailures != 0 || st3.BackoffUntil != nil || st3.Lifecycle != SyncLifecycleHealthy {
		t.Fatalf("reset %v", st3)
	}
}
func TestQuarantine(t *testing.T) { TestQuarantineLogBlocked(t) }
func TestLease(t *testing.T) { TestLeaseSplitBrain(t) }
func TestLogBlocked(t *testing.T) { TestQuarantineLogBlocked(t) }
func TestPayloadTamper(t *testing.T) { TestQuarantinePayloadTamper(t) }
