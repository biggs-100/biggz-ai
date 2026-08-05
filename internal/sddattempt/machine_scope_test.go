package sddattempt

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// redirectHome points HOME (and USERPROFILE on Windows, where os.UserHomeDir
// prefers it) at a temp dir, so the machine-scoped fallback ledger and the
// legacy migration source stay inside test state and never touch real user
// state.
func redirectHome(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir test home: %v", err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

// machineStoreDir returns the machine-scoped fallback ledger directory for a
// change under the redirected home (assumes redirectHome ran).
func machineStoreDir(t *testing.T, changeName string) string {
	t.Helper()
	return filepath.Join(MachineLedgerDir(), RuntimeVersion, changeName)
}

func TestStore_GitRepoResolvesCloneScope(t *testing.T) {
	redirectHome(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (%s)", err, out)
	}

	status, err := Status("ch-clone", repo)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status.Scope != ScopeClone {
		t.Fatalf("status scope = %q, want %q", status.Scope, ScopeClone)
	}

	begin, err := Begin(BeginParams{ChangeName: "ch-clone", RepoRoot: repo, WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	if begin.Scope != ScopeClone {
		t.Fatalf("begin scope = %q, want %q", begin.Scope, ScopeClone)
	}

	// The ledger lives under the git common dir, and the machine-scoped
	// fallback must not be created.
	commonOut, err := exec.Command("git", "-C", repo, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	commonDir := strings.TrimSpace(string(commonOut))
	storeDir := filepath.Join(filepath.Clean(filepath.Join(repo, commonDir)), runtimeStoreContainer, RuntimeDir, RuntimeVersion, "ch-clone")
	if _, err := os.Stat(filepath.Join(storeDir, "HEAD")); err != nil {
		t.Fatalf("clone-scoped HEAD missing: %v", err)
	}
	if _, err := os.Stat(MachineLedgerDir()); !os.IsNotExist(err) {
		t.Fatalf("machine-scoped ledger must not exist inside a git repo (stat err: %v)", err)
	}
}

func TestMachineScope_CASIntegrityAndReplay(t *testing.T) {
	redirectHome(t)
	dir := t.TempDir()

	// Idempotent replay with a request ID must work in machine scope.
	beginParams := BeginParams{ChangeName: "ch-mcas", RepoRoot: dir, WorkUnit: "w", RequestID: "req-mcas"}
	first, err := Begin(beginParams)
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	if first.Scope != ScopeMachine {
		t.Fatalf("begin scope = %q, want %q", first.Scope, ScopeMachine)
	}

	replay, err := Begin(beginParams)
	if err != nil {
		t.Fatalf("Begin(replay) error: %v", err)
	}
	if *replay != *first {
		t.Fatalf("replay result %+v != first result %+v", replay, first)
	}

	// Records are content-addressed: the canonical form hashes to the file
	// name, exactly like the clone-scoped store.
	storeDir := machineStoreDir(t, "ch-mcas")
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatalf("read machine store dir: %v", err)
	}
	recordCount := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "record-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		recordCount++
		revision := strings.TrimSuffix(strings.TrimPrefix(name, "record-"), ".json")
		data, err := os.ReadFile(filepath.Join(storeDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var store RuntimeStore
		if err := json.Unmarshal(data, &store); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if got := sha256Hex(canonicalRecordPayload(&store)); got != revision {
			t.Fatalf("record %s does not match its content address (got %s)", name, got)
		}
	}
	if recordCount != 1 {
		t.Fatalf("record count = %d, want 1 (idempotent replay must not append)", recordCount)
	}

	// Tampering fails closed, as in clone scope.
	headData, err := os.ReadFile(filepath.Join(storeDir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	recordPath := filepath.Join(storeDir, "record-"+head+".json")
	if err := os.WriteFile(recordPath, []byte("{tampered"), 0644); err != nil {
		t.Fatalf("tamper record: %v", err)
	}
	if _, err := Status("ch-mcas", dir); err == nil {
		t.Fatal("tampered machine-scoped record must fail closed")
	}
}

func TestMachineScope_MigratesLegacyLedger(t *testing.T) {
	home := redirectHome(t)
	dir := t.TempDir()

	// Pre-create a legacy home-dir ledger at the real legacy location
	// ~/.biggz/sdd-runtime/v1/<change>.json (under the redirected HOME).
	legacyRoot := filepath.Join(home, ".biggz", RuntimeDir, RuntimeVersion)
	legacy := &RuntimeStore{ObjectiveID: "obj-nogit", MaxAttempts: 3}
	legacyPath := writeLegacyLedger(t, legacyRoot, "ch-migrate-nogit", legacy)

	// First access migrates into the machine-scoped fallback ledger.
	status, err := Status("ch-migrate-nogit", dir)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !status.Migrated {
		t.Fatal("first access must report migration")
	}
	if status.Scope != ScopeMachine {
		t.Fatalf("status scope = %q, want %q", status.Scope, ScopeMachine)
	}
	if status.ChangeName != "ch-migrate-nogit" {
		t.Fatalf("migrated status = %+v, want legacy state preserved", status)
	}

	// The initial record lives under the fallback dir; the legacy file is
	// kept untouched.
	storeDir := machineStoreDir(t, "ch-migrate-nogit")
	if _, err := os.Stat(filepath.Join(storeDir, "HEAD")); err != nil {
		t.Fatalf("machine-scoped HEAD missing after migration: %v", err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy file must be kept untouched, got: %v", err)
	}

	// Second access does not migrate again.
	status2, err := Status("ch-migrate-nogit", dir)
	if err != nil {
		t.Fatalf("Status() #2 error: %v", err)
	}
	if status2.Migrated {
		t.Fatal("second access must not re-report migration")
	}
}

func TestMachineScope_GitMissingFailsLoudly(t *testing.T) {
	// A failure that is NOT the "not a git repository" class must not fall
	// back: a git binary that cannot be found fails loudly.
	redirectHome(t)
	t.Setenv("PATH", t.TempDir())
	dir := t.TempDir()
	_, err := Status("ch-nobin", dir)
	if err == nil {
		t.Fatal("missing git binary must fail loudly, not fall back to machine scope")
	}
	if strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("git-missing failure misclassified as not-a-git-repo: %v", err)
	}
}

func TestMachineScope_UnwritableFallbackFailsLoudly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only directory simulation is not portable on Windows: chmod does not gate file creation inside directories")
	}
	if os.Geteuid() == 0 {
		t.Skip("read-only directory simulation is ineffective for root")
	}
	home := redirectHome(t)
	biggzDir := filepath.Join(home, ".biggz")
	if err := os.MkdirAll(biggzDir, 0755); err != nil {
		t.Fatalf("mkdir .biggz: %v", err)
	}
	if err := os.Chmod(biggzDir, 0o555); err != nil {
		t.Fatalf("chmod .biggz read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(biggzDir, 0o755) })

	// A permission error on the fallback dir is NOT the not-a-git-repo
	// class: it must surface as a loud failure, never a silent success.
	dir := t.TempDir()
	_, err := Begin(BeginParams{ChangeName: "ch-perm", RepoRoot: dir, WorkUnit: "w"})
	if err == nil {
		t.Fatal("unwritable machine-scoped fallback dir must fail loudly")
	}
	if strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("permission failure misclassified as not-a-git-repo: %v", err)
	}
}
