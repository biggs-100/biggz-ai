package sddattempt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setStoreRoot redirects the ledger store to a temp dir for the test.
func setStoreRoot(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sdd-runtime")
	old := storeRootOverride
	storeRootOverride = dir
	t.Cleanup(func() { storeRootOverride = old })
	return dir
}

// storeFileBytes returns the current HEAD record bytes of the new
// clone-scoped layout (<override>/<change>/record-<head>.json), so tests can
// assert that a replay leaves the committed record unchanged.
func storeFileBytes(t *testing.T, changeName string) []byte {
	t.Helper()
	dir := filepath.Join(storeRootOverride, changeName)
	headData, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	data, err := os.ReadFile(filepath.Join(dir, "record-"+head+".json"))
	if err != nil {
		t.Fatalf("read record %s: %v", head, err)
	}
	return data
}

func TestBegin_RequestIDReplay(t *testing.T) {
	setStoreRoot(t)

	params := BeginParams{ChangeName: "ch", RepoRoot: "r", ObjectiveID: "obj", WorkUnit: "w", RequestID: "req-begin-a"}
	first, err := Begin(params)
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	afterFirst := storeFileBytes(t, "ch")

	replay, err := Begin(params)
	if err != nil {
		t.Fatalf("Begin(replay) error: %v", err)
	}
	if *replay != *first {
		t.Fatalf("replay result %+v != first result %+v", replay, first)
	}
	afterReplay := storeFileBytes(t, "ch")
	if string(afterReplay) != string(afterFirst) {
		t.Fatal("store file content changed on replay — double-apply detected")
	}
}

func TestBegin_RequestIDReplayConvergesAcrossInterveningOp(t *testing.T) {
	setStoreRoot(t)

	begin := BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w", RequestID: "req-begin-a"}
	first, err := Begin(begin)
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}

	// An intervening different operation moves the ledger on.
	finish, err := Finish(FinishParams{
		ChangeName: "ch", RepoRoot: "r",
		ExpectedRev: first.Revision,
		Outcome:     "passed",
		RequestID:   "req-finish-b",
	})
	if err != nil {
		t.Fatalf("Finish() error: %v", err)
	}
	_ = finish
	afterFinish := storeFileBytes(t, "ch")

	// Replaying the begin request still returns the recorded outcome, not the
	// current state, and does not mutate the store.
	replay, err := Begin(begin)
	if err != nil {
		t.Fatalf("Begin(replay) error: %v", err)
	}
	if *replay != *first {
		t.Fatalf("replay result %+v != first result %+v — replay must be convergent", replay, first)
	}
	afterReplay := storeFileBytes(t, "ch")
	if string(afterReplay) != string(afterFinish) {
		t.Fatal("store file content changed on convergent replay")
	}
}

func TestBegin_RequestIDReusedWithDifferentInputs(t *testing.T) {
	setStoreRoot(t)

	params := BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w", RequestID: "req-begin-a"}
	if _, err := Begin(params); err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	afterFirst := storeFileBytes(t, "ch")

	params.WorkUnit = "different-input"
	_, err := Begin(params)
	if err == nil || !strings.Contains(err.Error(), "reused with different inputs") {
		t.Fatalf("expected reuse conflict error, got %v", err)
	}
	afterConflict := storeFileBytes(t, "ch")
	if string(afterConflict) != string(afterFirst) {
		t.Fatal("store file content changed on failed reuse")
	}
}

func TestFinish_RequestIDReplay(t *testing.T) {
	setStoreRoot(t)

	begin, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}

	finish := FinishParams{
		ChangeName: "ch", RepoRoot: "r",
		ExpectedRev: begin.Revision,
		Outcome:     "failed",
		Diagnosis:   "tests broke",
		RequestID:   "req-finish-c",
	}
	first, err := Finish(finish)
	if err != nil {
		t.Fatalf("Finish() error: %v", err)
	}
	afterFirst := storeFileBytes(t, "ch")

	replay, err := Finish(finish)
	if err != nil {
		t.Fatalf("Finish(replay) error: %v", err)
	}
	if *replay != *first {
		t.Fatalf("replay result %+v != first result %+v", replay, first)
	}
	afterReplay := storeFileBytes(t, "ch")
	if string(afterReplay) != string(afterFirst) {
		t.Fatal("store file content changed on finish replay — double-apply detected")
	}
}

func TestFinish_RequestIDReplayWithoutActiveAttempt(t *testing.T) {
	setStoreRoot(t)

	begin, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	finish := FinishParams{
		ChangeName: "ch", RepoRoot: "r",
		ExpectedRev: begin.Revision,
		Outcome:     "passed",
		RequestID:   "req-finish-d",
	}
	if _, err := Finish(finish); err != nil {
		t.Fatalf("Finish() error: %v", err)
	}
	afterFirst := storeFileBytes(t, "ch")

	// The ledger is now complete (no active attempt): a replay of the same
	// request must still return the recorded outcome instead of failing on
	// the current state.
	replay, err := Finish(finish)
	if err != nil {
		t.Fatalf("Finish(replay) error: %v", err)
	}
	if !replay.Complete {
		t.Fatalf("replay result %+v: want recorded completed outcome", replay)
	}
	afterReplay := storeFileBytes(t, "ch")
	if string(afterReplay) != string(afterFirst) {
		t.Fatal("store file content changed on finish replay")
	}
}

func TestReset_RequestIDReplay(t *testing.T) {
	setStoreRoot(t)

	if _, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w"}); err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	reset := ResetParams{ChangeName: "ch", RepoRoot: "r", Reason: "new objective", RequestID: "req-reset-e"}
	first, err := Reset(reset)
	if err != nil {
		t.Fatalf("Reset() error: %v", err)
	}
	afterFirst := storeFileBytes(t, "ch")

	replay, err := Reset(reset)
	if err != nil {
		t.Fatalf("Reset(replay) error: %v", err)
	}
	if *replay != *first {
		t.Fatalf("replay result %+v != first result %+v", replay, first)
	}
	afterReplay := storeFileBytes(t, "ch")
	if string(afterReplay) != string(afterFirst) {
		t.Fatal("store file content changed on reset replay")
	}
}

func TestRequestID_InvalidFormat(t *testing.T) {
	setStoreRoot(t)

	params := BeginParams{ChangeName: "ch", RepoRoot: "r", RequestID: "UPPER-case"}
	_, err := Begin(params)
	if err == nil || !strings.Contains(err.Error(), "canonical lowercase") {
		t.Fatalf("expected canonical lowercase error, got %v", err)
	}
}

func TestNoRequestID_LegacyBehaviorUnchanged(t *testing.T) {
	setStoreRoot(t)

	first, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	// Without a request ID, a second begin while active reports AlreadyActive
	// instead of replaying.
	second, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("second Begin() error: %v", err)
	}
	if !second.AlreadyActive {
		t.Fatalf("second begin = %+v, want AlreadyActive", second)
	}
	_ = first

	store, _, err := loadStore("ch", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if store.Requests != nil && len(store.Requests) != 0 {
		t.Fatal("ledger must not record requests when no request ID is supplied")
	}
}
