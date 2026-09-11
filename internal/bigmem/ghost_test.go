package bigmem

import (
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ─── Ghost-WAL liveness (REQ-GW1..GW3, defect #4) ────────────────────────────
//
// RED-first evidence: the live-holder and inconclusive cases below fail against
// the shape-only + O_EXCL probe implementation because a live holder (e.g. a
// running biggz-mcp) is misread as a stale ghost and Open falls back to
// bigmem_recovered. The share-0 probe (Windows) / O_EXCL probe (other OSes)
// must classify live holders, proven-dead holders, and inconclusive probes
// distinctly before any reclaim or fallback happens.

// holdOpen keeps paths open to simulate a live SQLite-style holder such as a
// running biggz-mcp. Go's os.Open does not share delete access, so both
// os.Remove and a share-0 CreateFile observe the holder exactly like a real
// process holding the store.
func holdOpen(t *testing.T, paths ...string) []*os.File {
	t.Helper()
	holders := make([]*os.File, 0, len(paths))
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			t.Fatalf("hold %s: %v", p, err)
		}
		t.Cleanup(func() { _ = f.Close() })
		holders = append(holders, f)
	}
	return holders
}

// captureStderr redirects os.Stderr while fn runs and returns what was written.
// Tests using it must stay sequential (no t.Parallel).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stderr = old
	out := <-done
	_ = r.Close()
	return out
}

// forceProbeSeam pins the package-level probe seam for the duration of the
// test so classifyGhostWAL outcomes are deterministic on every platform.
// Tests using it must stay sequential (no t.Parallel).
func forceProbeSeam(t *testing.T, holder, proven bool) {
	t.Helper()
	orig := ghostProbeDBLiveness
	ghostProbeDBLiveness = func(string) (bool, bool) { return holder, proven }
	t.Cleanup(func() { ghostProbeDBLiveness = orig })
}

// TestGhostWAL_LiveHolder_UsesPrimary reproduces the live defect: a running
// holder has bigmem.db open while ghost-shaped stale wal/shm sit next to it
// (exactly the biggz-mcp + CLI coexistence). The primary MUST stay in use:
// wal/shm untouched, no recovered fallback, no ghost warning, no data loss.
func TestGhostWAL_LiveHolder_UsesPrimary(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("live-holder detection is Windows-only (share-0 probe)")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")

	// Seed a real row so the case proves no data loss, then close so the fake
	// ghost shape is not clobbered by a live SQLite handle.
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	obs := &Observation{Title: "live-holder marker", Type: "note", Content: "holder-marker-content", Project: "test"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("seed close: %v", err)
	}
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
	holders := holdOpen(t, dbPath, dbPath+"-wal", dbPath+"-shm")

	var resolveErr error
	var calls []string
	stderr := captureStderr(t, func() {
		for i := 0; i < 2 && resolveErr == nil; i++ {
			var got string
			got, resolveErr = ResolveDBPath(dir)
			calls = append(calls, got)
		}
	})
	if resolveErr != nil {
		t.Fatalf("ResolveDBPath: %v", resolveErr)
	}
	for i, got := range calls {
		if got != dbPath {
			t.Errorf("ResolveDBPath call #%d = %q, want primary %q (recovered fallback must not trigger)", i+1, got, dbPath)
		}
	}
	if info, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Errorf("wal must stay untouched while holder is live: %v", err)
	} else if info.Size() != 0 {
		t.Errorf("wal size changed while holder is live: %d, want 0", info.Size())
	}
	if info, err := os.Stat(dbPath + "-shm"); err != nil {
		t.Errorf("shm must stay untouched while holder is live: %v", err)
	} else if info.Size() != 32768 {
		t.Errorf("shm must not be truncated/rewritten while holder is live: size %d, want 32768", info.Size())
	}
	if _, err := os.Stat(recoveredDBPathForRoot(dir)); !os.IsNotExist(err) {
		t.Errorf("recovered fallback must not be created: %v", err)
	}
	if strings.Contains(stderr, "ghost WAL/SHM detected") {
		t.Errorf("ghost warning MUST NOT be emitted with a live holder; stderr:\n%s", stderr)
	}

	// Release the simulated holder; the seeded row must survive the episode.
	for _, f := range holders {
		_ = f.Close()
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	results, err := s2.Search("holder-marker-content", SearchOptions{Project: "test", Limit: 5})
	if err != nil {
		t.Fatalf("search after holder episode: %v", err)
	}
	if len(results) == 0 {
		t.Error("seeded row MUST remain readable (no data loss)")
	}
}

// TestGhostWAL_NonWindows_LiveHolderKeepsFiles is the W1 regression guard
// (REQ-GW2/GW3): off Windows the probe cannot observe a live holder, so the
// only safe outcome is inconclusive -> recovered fallback with wal/shm kept.
// Mapping O_EXCL success to proven-dead would let os.Remove delete a live
// holder's files (Unix removes open files) and violate REQ-GW2's "live holder
// blocks reclaim".
func TestGhostWAL_NonWindows_LiveHolderKeepsFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-Windows O_EXCL probe semantics (share-0 is exact on Windows)")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
	holdOpen(t, dbPath, dbPath+"-wal", dbPath+"-shm")

	resolved, err := ResolveDBPath(dir)
	if err != nil {
		t.Fatalf("ResolveDBPath: %v", err)
	}
	if resolved != recoveredDBPathForRoot(dir) {
		t.Errorf("ResolveDBPath = %q, want recovered fallback %q (inconclusive MUST NOT claim the primary)", resolved, recoveredDBPathForRoot(dir))
	}
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Errorf("wal MUST be preserved while a live holder may own it: %v", err)
	}
	if _, err := os.Stat(dbPath + "-shm"); err != nil {
		t.Errorf("shm MUST be preserved while a live holder may own it: %v", err)
	}
}

// TestGhostWAL_Inconclusive_PreservesFiles covers REQ-GW3: when the liveness
// probe cannot prove either way, wal/shm MUST remain and Open MUST still
// succeed (recovered fallback stays intact).
func TestGhostWAL_Inconclusive_PreservesFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")

	if runtime.GOOS == "windows" {
		// A directory at dbPath makes the share-0 probe fail with
		// ERROR_ACCESS_DENIED: neither holder nor death can be proven.
		if err := os.MkdirAll(dbPath, 0o755); err != nil {
			t.Fatalf("mkdir db: %v", err)
		}
	} else {
		if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		// A pre-held claim file makes O_CREATE|O_EXCL fail: inconclusive.
		if err := os.WriteFile(dbPath+".ghost_probe", []byte("held"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))

	if _, err := ResolveDBPath(dir); err != nil {
		t.Fatalf("ResolveDBPath must not fail on inconclusive probe: %v", err)
	}
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Errorf("inconclusive probe must NOT remove wal: %v", err)
	}
	if _, err := os.Stat(dbPath + "-shm"); err != nil {
		t.Errorf("inconclusive probe must NOT remove shm: %v", err)
	}
}

// TestGhostWAL_Inconclusive_RecoveredFallback closes verify finding W2: with
// the probe forced inconclusive via the seam, the REQ-GW3 fallback half MUST
// hold end-to-end: the ghost warning is emitted, the recovered path is
// returned, and wal/shm are left untouched. The recovered DB is built here
// the same way a previous inconclusive resolve builds it (safeCopyDB), so no
// pre-existing-file assumption is baked into the assertions.
func TestGhostWAL_Inconclusive_RecoveredFallback(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	obs := &Observation{Title: "fallback marker", Type: "note", Content: "fallback-marker-content", Project: "test"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("seed close: %v", err)
	}
	recoveredPath := recoveredDBPathForRoot(dir)
	if err := safeCopyDB(dbPath, recoveredPath); err != nil {
		t.Fatalf("seed recovered fallback: %v", err)
	}
	// Switch the primary to rollback-journal mode so SQLite's own WAL hygiene
	// (mergeDB reads the primary on the warning branch) cannot remove the
	// forged files and be mistaken for the code under test. reclaimStaleWAL
	// would still delete them explicitly (os.Remove), which the planted probe
	// marker also flags.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	if _, err := raw.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		t.Fatalf("journal_mode=DELETE: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw close: %v", err)
	}

	forceProbeSeam(t, false, false)
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
	// A planted probe marker identifies the branch: reclaimStaleWAL removes it,
	// the fallback path never does.
	if err := os.WriteFile(dbPath+".ghost_probe", []byte("sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}

	var resolved string
	var resolveErr error
	stderr := captureStderr(t, func() { resolved, resolveErr = ResolveDBPath(dir) })
	if resolveErr != nil {
		t.Fatalf("ResolveDBPath: %v", resolveErr)
	}
	if resolved != recoveredPath {
		t.Errorf("ResolveDBPath = %q, want recovered %q", resolved, recoveredPath)
	}
	if !strings.Contains(stderr, "ghost WAL/SHM detected") {
		t.Errorf("REQ-GW3 ghost warning MUST be emitted on the inconclusive fallback; stderr:\n%s", stderr)
	}
	for _, suffix := range []string{"-wal", "-shm", ".ghost_probe"} {
		if _, err := os.Stat(dbPath + suffix); err != nil {
			t.Errorf("inconclusive probe MUST leave %s untouched: %v", suffix, err)
		}
	}

	// The fallback store must carry the seeded row.
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("open from recovered fallback: %v", err)
	}
	defer s2.Close()
	results, err := s2.Search("fallback-marker-content", SearchOptions{Project: "test", Limit: 5})
	if err != nil {
		t.Fatalf("search on recovered fallback: %v", err)
	}
	if len(results) == 0 {
		t.Error("seeded row MUST be readable through the recovered fallback")
	}
}

// TestGhostWAL_LeftoverProbe_ReclaimWhenDead: a crashed claim file from the
// legacy O_EXCL probe MUST NOT pin the store to bigmem_recovered once the
// previous holder is provably dead.
func TestGhostWAL_LeftoverProbe_ReclaimWhenDead(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("reclaim-after-dead-holder is validated on the Windows share-0 probe path")
	}
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
	if err := os.WriteFile(dbPath+".ghost_probe", []byte("crashed"), 0o644); err != nil {
		t.Fatal(err)
	}

	var resolved string
	var resolveErr error
	stderr := captureStderr(t, func() { resolved, resolveErr = ResolveDBPath(dir) })
	if resolveErr != nil {
		t.Fatalf("ResolveDBPath: %v", resolveErr)
	}
	if resolved != dbPath {
		t.Errorf("ResolveDBPath = %q, want primary %q", resolved, dbPath)
	}
	if _, err := os.Stat(dbPath + "-wal"); !os.IsNotExist(err) {
		t.Errorf("wal MUST be reclaimed when holder is provably dead: %v", err)
	}
	if _, err := os.Stat(dbPath + "-shm"); !os.IsNotExist(err) {
		t.Errorf("shm MUST be reclaimed when holder is provably dead: %v", err)
	}
	if _, err := os.Stat(recoveredDBPathForRoot(dir)); !os.IsNotExist(err) {
		t.Errorf("recovered fallback must not trigger: %v", err)
	}
	if strings.Contains(stderr, "ghost WAL/SHM detected") {
		t.Errorf("no ghost warning on reclaim; stderr:\n%s", stderr)
	}
}

// ─── Classification / probe matrix (REQ-GW1..GW3) ────────────────────────────

// TestProbeDBLiveness_Matrix covers the platform probe contract: no holder
// proves death, a live handle proves a holder (Windows share-0), and a probe
// that cannot observe either stays inconclusive.
func TestProbeDBLiveness_Matrix(t *testing.T) {
	t.Run("no holder", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "bigmem.db")
		if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		holder, proven := probeDBLiveness(dbPath)
		if runtime.GOOS == "windows" {
			// share-0 success proves no holder -> proven dead (REQ-GW2).
			if holder || !proven {
				t.Errorf("probeDBLiveness = (%v, %v), want (false, true)", holder, proven)
			}
			return
		}
		// W1: off Windows O_EXCL success only proves no competing probe; it
		// cannot observe a live holder, so it MUST stay inconclusive.
		if holder || proven {
			t.Errorf("probeDBLiveness = (%v, %v), want (false, false) off Windows (REQ-GW2/GW3)", holder, proven)
		}
	})
	t.Run("live holder", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("holder detection is Windows-only (share-0 probe)")
		}
		dbPath := filepath.Join(t.TempDir(), "bigmem.db")
		if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
		holdOpen(t, dbPath)
		if holder, proven := probeDBLiveness(dbPath); !holder || !proven {
			t.Errorf("probeDBLiveness = (%v, %v), want (true, true)", holder, proven)
		}
	})
	t.Run("inconclusive", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "bigmem.db")
		if runtime.GOOS == "windows" {
			if err := os.MkdirAll(dbPath, 0o755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(dbPath+".ghost_probe", []byte("held"), 0o644); err != nil {
			t.Fatal(err)
		}
		if holder, proven := probeDBLiveness(dbPath); holder || proven {
			t.Errorf("probeDBLiveness = (%v, %v), want (false, false)", holder, proven)
		}
	})
}

// TestClassifyGhostWAL_Matrix is the REQ-GW1 classification matrix: fresh and
// non-ghost shapes (ghostNone), stale-idle with a dead holder (ghostStale),
// live holder (ghostLiveHolder, Windows) and inconclusive probe.
func TestClassifyGhostWAL_Matrix(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(t *testing.T, dbPath string)
		want        ghostClass
		wantOther   ghostClass // non-Windows expectation (when it differs)
		windowsOnly bool
	}{
		{
			name: "fresh ghost-shape is normal",
			setup: func(t *testing.T, dbPath string) {
				createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-30*time.Second))
			},
			want: ghostNone,
		},
		{
			name: "wal nonzero is normal",
			setup: func(t *testing.T, dbPath string) {
				createGhostFiles(t, dbPath, 100, 32768, time.Now().Add(-6*time.Minute))
			},
			want: ghostNone,
		},
		{
			name: "shm zero is normal",
			setup: func(t *testing.T, dbPath string) {
				createGhostFiles(t, dbPath, 0, 0, time.Now().Add(-6*time.Minute))
			},
			want: ghostNone,
		},
		{
			name: "stale idle is reclaimable",
			setup: func(t *testing.T, dbPath string) {
				createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
			},
			want: ghostStale,
			// W1 fix: off Windows O_EXCL success cannot prove death, so the
			// same stale shape MUST stay inconclusive (recovered fallback).
			wantOther: ghostInconclusive,
		},
		{
			name: "live holder is not stale",
			setup: func(t *testing.T, dbPath string) {
				if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
					t.Fatal(err)
				}
				createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
				holdOpen(t, dbPath)
			},
			want:        ghostLiveHolder,
			windowsOnly: true,
		},
		{
			name: "inconclusive probe",
			setup: func(t *testing.T, dbPath string) {
				createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))
				if runtime.GOOS == "windows" {
					if err := os.MkdirAll(dbPath, 0o755); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(dbPath+".ghost_probe", []byte("held"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want:      ghostInconclusive,
			wantOther: ghostInconclusive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.windowsOnly && runtime.GOOS != "windows" {
				t.Skip("holder detection is Windows-only (share-0 probe)")
			}
			dbPath := filepath.Join(t.TempDir(), "bigmem.db")
			tc.setup(t, dbPath)
			want := tc.want
			if runtime.GOOS != "windows" {
				want = tc.wantOther
			}
			if got := classifyGhostWAL(dbPath); got != want {
				t.Errorf("classifyGhostWAL = %v, want %v", got, want)
			}
		})
	}
}

// TestGhostWAL_ReclaimKeepsData: the REQ-GW2 reclaim of a dead holder's ghost
// files MUST keep the primary DB and its rows intact.
func TestGhostWAL_ReclaimKeepsData(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	obs := &Observation{Title: "reclaim marker", Type: "note", Content: "reclaim-marker-content", Project: "test"}
	if err := s.Save(obs); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// No holder + proven death is forced via the seam: off Windows the O_EXCL
	// probe can no longer prove death (W1 fix), so the seam keeps this reclaim
	// assertion meaningful on every platform.
	forceProbeSeam(t, false, true)
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))

	resolved, err := ResolveDBPath(dir)
	if err != nil {
		t.Fatalf("ResolveDBPath: %v", err)
	}
	if resolved != dbPath {
		t.Errorf("ResolveDBPath = %q, want primary %q", resolved, dbPath)
	}
	if _, err := os.Stat(dbPath + "-wal"); !os.IsNotExist(err) {
		t.Errorf("wal must be reclaimed: %v", err)
	}
	if _, err := os.Stat(dbPath + "-shm"); !os.IsNotExist(err) {
		t.Errorf("shm must be reclaimed: %v", err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	results, err := s2.Search("reclaim-marker-content", SearchOptions{Project: "test", Limit: 5})
	if err != nil {
		t.Fatalf("search after reclaim: %v", err)
	}
	if len(results) == 0 {
		t.Error("seeded row MUST survive the reclaim (no data loss)")
	}
}

// TestReclaimCheckpointErrorNeverFailsOpen covers REQ-GW2 best-effort
// checkpoint: a failing wal_checkpoint(TRUNCATE) MUST NOT fail Open, and the
// stale reclaim MUST still remove the zombie wal/shm and use the primary.
func TestReclaimCheckpointErrorNeverFailsOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "bigmem.db")
	if err := os.WriteFile(dbPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	// Proven death is forced via the probe seam (see TestGhostWAL_ReclaimKeepsData).
	forceProbeSeam(t, false, true)
	createGhostFiles(t, dbPath, 0, 32768, time.Now().Add(-6*time.Minute))

	orig := ghostReclaimCheckpoint
	calls := 0
	ghostReclaimCheckpoint = func(string) error {
		calls++
		return errors.New("checkpoint failed")
	}
	t.Cleanup(func() { ghostReclaimCheckpoint = orig })

	resolved, err := ResolveDBPath(dir)
	if err != nil {
		t.Fatalf("checkpoint error MUST NOT fail ResolveDBPath: %v", err)
	}
	if resolved != dbPath {
		t.Errorf("ResolveDBPath = %q, want primary %q", resolved, dbPath)
	}
	if calls != 1 {
		t.Errorf("checkpoint calls = %d, want 1", calls)
	}
	if _, err := os.Stat(dbPath + "-wal"); !os.IsNotExist(err) {
		t.Errorf("wal must still be reclaimed despite checkpoint error: %v", err)
	}
	if _, err := os.Stat(dbPath + "-shm"); !os.IsNotExist(err) {
		t.Errorf("shm must still be reclaimed despite checkpoint error: %v", err)
	}
	if _, err := os.Stat(recoveredDBPathForRoot(dir)); !os.IsNotExist(err) {
		t.Errorf("recovered fallback must not trigger: %v", err)
	}
}
