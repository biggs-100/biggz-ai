package sddattempt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ledgerDir returns the ledger directory for a change under the test store
// root override.
func ledgerDir(changeName string) string {
	return filepath.Join(storeRootOverride, RuntimeVersion, changeName)
}

// recordCount counts the immutable content-addressed records in a ledger.
// A ledger that was never created counts as zero.
func recordCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read ledger dir: %v", err)
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "record-") && strings.HasSuffix(name, ".json") {
			count++
		}
	}
	return count
}

// canonicalDir resolves a path the way grant normalization would (absolute,
// symlink-evaluated), for assertions about projected roots.
func canonicalDir(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks %s: %v", path, err)
	}
	return resolved
}

// makeSymlink creates a directory symlink, skipping the test when the
// platform lacks the privilege (Windows requires developer mode or an
// elevated shell).
func makeSymlink(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
}

// TestGrantCommitsAndProjectsGrantedRoots is the grant round-trip: a
// committed grant projects its canonical absolute symlink-evaluated roots,
// and a second grant chains ExpectedRevision and accumulates, deduplicating
// the already-granted root.
func TestGrantCommitsAndProjectsGrantedRoots(t *testing.T) {
	setStoreRoot(t)

	sibling := canonicalDir(t, t.TempDir())
	link := filepath.Join(t.TempDir(), "sibling-link")
	makeSymlink(t, link, sibling)

	// The caller passes the SYMLINK path; the recorded and projected root
	// must be the canonical evaluated target.
	granted, err := Grant(GrantParams{
		ChangeName: "grant-roots", RepoRoot: "r",
		Roots: []string{link}, Reason: "maintainer authorized sibling repository edits",
		Actor: "maintainer", RequestID: "grant-1", ChangeInstance: "grant-roots-instance",
	})
	if err != nil {
		t.Fatalf("grant refused: %v", err)
	}
	if len(granted.GrantedRoots) != 1 || granted.GrantedRoots[0] != sibling ||
		granted.Revision == "" || recordCount(t, ledgerDir("grant-roots")) != 1 {
		t.Fatalf("granted result = %#v records=%d, want one record granting %q",
			granted, recordCount(t, ledgerDir("grant-roots")), sibling)
	}

	// StatusWithInstance replays the persisted chain: the projection
	// round-trips.
	status, err := StatusWithInstance("grant-roots", "r", "grant-roots-instance")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if len(status.GrantedRoots) != 1 || status.GrantedRoots[0] != sibling {
		t.Fatalf("replayed granted roots = %#v, want [%q]", status.GrantedRoots, sibling)
	}

	// A second grant chains ExpectedRevision on the first and accumulates,
	// deduplicating the already-granted root.
	second := canonicalDir(t, t.TempDir())
	accumulated, err := Grant(GrantParams{
		ChangeName: "grant-roots", RepoRoot: "r",
		ExpectedRev: granted.Revision, Roots: []string{second, sibling},
		Reason: "maintainer widened the change to a second sibling",
		Actor:  "maintainer", RequestID: "grant-2", ChangeInstance: "grant-roots-instance",
	})
	if err != nil {
		t.Fatalf("second grant refused: %v", err)
	}
	if len(accumulated.GrantedRoots) != 2 || accumulated.GrantedRoots[0] != sibling ||
		accumulated.GrantedRoots[1] != second || recordCount(t, ledgerDir("grant-roots")) != 2 {
		t.Fatalf("accumulated granted roots = %#v records=%d, want [%q %q] over 2 records",
			accumulated.GrantedRoots, recordCount(t, ledgerDir("grant-roots")), sibling, second)
	}
}

// TestGrantDuplicateRequestIsIdempotent proves the request-id contract: an
// exact replay returns the committed revision without a new record; reuse
// with different inputs is refused.
func TestGrantDuplicateRequestIsIdempotent(t *testing.T) {
	setStoreRoot(t)

	root := canonicalDir(t, t.TempDir())
	params := GrantParams{
		ChangeName: "grant-idempotent", RepoRoot: "r",
		Roots: []string{root}, Reason: "maintainer authorized sibling repository edits",
		Actor: "maintainer", RequestID: "grant-once", ChangeInstance: "grant-idempotent-instance",
	}
	granted, err := Grant(params)
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	replayed, err := Grant(params)
	if err != nil || replayed.Revision != granted.Revision ||
		recordCount(t, ledgerDir("grant-idempotent")) != 1 {
		t.Fatalf("grant replay = %#v err=%v records=%d, want committed revision %q and 1 record",
			replayed, err, recordCount(t, ledgerDir("grant-idempotent")), granted.Revision)
	}

	other := canonicalDir(t, t.TempDir())
	params.Roots = []string{other}
	if _, err := Grant(params); err == nil || !strings.Contains(err.Error(), "reused with different inputs") {
		t.Fatalf("conflicting grant reuse = %v, want the reused-with-different-inputs refusal", err)
	}
}

// TestGrantRequiresChangeInstance contains the write path: a grant without a
// change-instance identity is refused before anything is recorded, and
// malformed identities are refused too.
func TestGrantRequiresChangeInstance(t *testing.T) {
	setStoreRoot(t)

	root := canonicalDir(t, t.TempDir())
	base := GrantParams{
		ChangeName: "grant-instance-required", RepoRoot: "r",
		Roots: []string{root}, Reason: "maintainer authorized sibling repository edits",
		Actor: "maintainer", RequestID: "grant-no-instance",
	}
	invalid := []struct {
		name     string
		instance string
	}{
		{name: "empty", instance: ""},
		{name: "untrimmed", instance: " token "},
		{name: "multi-line", instance: "line1\nline2"},
		{name: "carriage-return", instance: "line1\rline2"},
		{name: "overlong", instance: strings.Repeat("x", 129)},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			params := base
			params.ChangeInstance = tt.instance
			if _, err := Grant(params); err == nil {
				t.Fatalf("grant with instance %q was accepted", tt.instance)
			}
		})
	}
	if recordCount(t, ledgerDir("grant-instance-required")) != 0 {
		t.Fatalf("refused grants recorded %d records, want none", recordCount(t, ledgerDir("grant-instance-required")))
	}
}

// TestGrantWithoutInstanceProjectsNothing pins the conservative containment:
// a reader that declares no change-instance identity projects no granted
// roots, while a read scoped to the grant's own token projects them.
func TestGrantWithoutInstanceProjectsNothing(t *testing.T) {
	setStoreRoot(t)

	root := canonicalDir(t, t.TempDir())
	if _, err := Grant(GrantParams{
		ChangeName: "grant-projection", RepoRoot: "r",
		Roots: []string{root}, Reason: "maintainer authorized sibling repository edits",
		Actor: "maintainer", RequestID: "grant-projection-1", ChangeInstance: "projection-token",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	status, err := Status("grant-projection", "r")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.GrantedRoots != nil {
		t.Fatalf("undeclared-instance status projected granted roots: %#v", status.GrantedRoots)
	}

	scoped, err := StatusWithInstance("grant-projection", "r", "projection-token")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if len(scoped.GrantedRoots) != 1 || scoped.GrantedRoots[0] != root {
		t.Fatalf("scoped status projected %#v, want [%q]", scoped.GrantedRoots, root)
	}
}

// TestArchivedNameReuseDoesNotResurrectGrantedRoots is the containment
// fixture: the ledger directory is keyed by change name alone, so a change
// reusing an archived name reopens the SAME chain. Its reader must not
// inherit the archived change's grants: per-change authority must not become
// workspace-permanent authority through the back door.
func TestArchivedNameReuseDoesNotResurrectGrantedRoots(t *testing.T) {
	setStoreRoot(t)

	root := canonicalDir(t, t.TempDir())
	if _, err := Grant(GrantParams{
		ChangeName: "reused-change-name", RepoRoot: "r",
		Roots: []string{root}, Reason: "maintainer authorized sibling repository edits for the first change",
		Actor: "maintainer", RequestID: "grant-first-instance", ChangeInstance: "first-change-instance",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	// A recreated change with the same name replays the same chain under a
	// different identity: it inherits none of the archived grants.
	second, err := StatusWithInstance("reused-change-name", "r", "second-change-instance")
	if err != nil {
		t.Fatalf("StatusWithInstance(second): %v", err)
	}
	if len(second.GrantedRoots) != 0 {
		t.Fatalf("recreated change instance resurrected granted roots: %#v", second.GrantedRoots)
	}

	// The original instance keeps its authority: containment narrows the
	// projection to the consented instance, it revokes nothing.
	first, err := StatusWithInstance("reused-change-name", "r", "first-change-instance")
	if err != nil {
		t.Fatalf("StatusWithInstance(first): %v", err)
	}
	if len(first.GrantedRoots) != 1 || first.GrantedRoots[0] != root {
		t.Fatalf("original instance projected %#v, want its granted root %q", first.GrantedRoots, root)
	}
}

// TestGrantRootCountBounds pins the bounded root list: zero roots and more
// than maximumGrantRoots are refused, and a symlink plus its target collapse
// to one canonical identity.
func TestGrantRootCountBounds(t *testing.T) {
	setStoreRoot(t)

	base := GrantParams{
		ChangeName: "grant-bounds", RepoRoot: "r",
		Reason: "maintainer authorized sibling repository edits",
		Actor:  "maintainer", RequestID: "grant-bounds-1", ChangeInstance: "bounds-token",
	}
	if _, err := Grant(base); err == nil || !strings.Contains(err.Error(), "between 1 and") {
		t.Fatalf("zero-root grant = %v, want the bounded-list refusal", err)
	}

	many := make([]string, maximumGrantRoots+1)
	for i := range many {
		many[i] = canonicalDir(t, t.TempDir())
	}
	base.Roots = many
	if _, err := Grant(base); err == nil || !strings.Contains(err.Error(), "between 1 and") {
		t.Fatalf("oversized grant = %v, want the bounded-list refusal", err)
	}
	if recordCount(t, ledgerDir("grant-bounds")) != 0 {
		t.Fatal("refused grants must not record")
	}

	// Link + target collapse to one canonical identity.
	target := canonicalDir(t, t.TempDir())
	link := filepath.Join(t.TempDir(), "bounds-link")
	makeSymlink(t, link, target)
	base.RequestID = "grant-bounds-2"
	base.Roots = []string{target, link}
	granted, err := Grant(base)
	if err != nil {
		t.Fatalf("link+target grant: %v", err)
	}
	if len(granted.GrantedRoots) != 1 || granted.GrantedRoots[0] != target {
		t.Fatalf("link+target collapse granted %#v, want one canonical root %q", granted.GrantedRoots, target)
	}
}

// TestGrantSnapshotForgeryRefused is the snapshot-model forgery pattern: a
// record whose Grants were widened after publication no longer matches its
// content address, so replay refuses the chain on the next read.
func TestGrantSnapshotForgeryRefused(t *testing.T) {
	setStoreRoot(t)

	root := canonicalDir(t, t.TempDir())
	if _, err := Grant(GrantParams{
		ChangeName: "grant-forged-snapshot", RepoRoot: "r",
		Roots: []string{root}, Reason: "maintainer authorized one sibling repository",
		Actor: "maintainer", RequestID: "grant-forge-1", ChangeInstance: "forged-token",
	}); err != nil {
		t.Fatalf("grant: %v", err)
	}

	// Forged: the record's grant history is widened in place, keeping the
	// same revision file name (the content address the original commit
	// produced).
	dir := ledgerDir("grant-forged-snapshot")
	headData, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	recordPath := filepath.Join(dir, "record-"+head+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	var grants []RuntimeGrant
	if err := json.Unmarshal(document["grants"], &grants); err != nil || len(grants) != 1 {
		t.Fatalf("parse grants: %v (len=%d)", err, len(grants))
	}
	widened := canonicalDir(t, t.TempDir())
	grants[0].Roots = append(grants[0].Roots, widened)
	document["grants"], _ = json.Marshal(grants)
	payload, _ := json.Marshal(document)
	if err := os.WriteFile(recordPath, payload, 0644); err != nil {
		t.Fatalf("rewrite record: %v", err)
	}

	if _, err := Status("grant-forged-snapshot", "r"); err == nil ||
		!strings.Contains(err.Error(), "content address") {
		t.Fatalf("replay of forged widened grant = %v, want the content-address rejection", err)
	}
}

// TestLegacyStoreWithoutGrantsMigratesUnchanged pins the phase-1
// compatibility constraint: a legacy single-file ledger recorded before
// grants existed replays exactly as before, projects no granted roots, and
// serializes no grants member (omitempty is load-bearing).
func TestLegacyStoreWithoutGrantsMigratesUnchanged(t *testing.T) {
	root := setStoreRoot(t)

	legacy := &RuntimeStore{
		ObjectiveID: "obj-legacy-grant", MaxAttempts: 3,
		Attempts: []RuntimeAttempt{{
			Ordinal: 1, ObjectiveID: "obj-legacy-grant", WorkUnit: "w",
			BeganAt: "2026-07-31T00:00:00Z", Outcome: "passed", EndedAt: "2026-07-31T01:00:00Z",
		}},
		ActiveAttempt: 0,
		Complete:      true,
		NextAction:    "complete",
	}
	writeLegacyLedger(t, root, "ch-grant-legacy", legacy)

	status, err := StatusWithInstance("ch-grant-legacy", "r", "legacy-token")
	if err != nil {
		t.Fatalf("StatusWithInstance: %v", err)
	}
	if !status.Migrated {
		t.Fatal("first access must report migration")
	}
	if status.GrantedRoots != nil {
		t.Fatalf("grant-free chain projected granted roots: %#v", status.GrantedRoots)
	}

	// The migrated record carries no grants member.
	payload, err := os.ReadFile(storeFileBytesPath(t, "ch-grant-legacy"))
	if err != nil {
		t.Fatalf("read migrated record: %v", err)
	}
	if strings.Contains(string(payload), "grants") {
		t.Fatalf("grant-free store serialized a grants member: %s", payload)
	}
}

// storeFileBytesPath returns the HEAD record path for a change under the
// test store root override.
func storeFileBytesPath(t *testing.T, changeName string) string {
	t.Helper()
	dir := ledgerDir(changeName)
	headData, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	return filepath.Join(dir, "record-"+head+".json")
}
