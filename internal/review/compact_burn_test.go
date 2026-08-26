package review

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func approvedCompactFixture(t *testing.T, repo, lineage string) (string, CompactStoreFile) {
	t.Helper()
	base, _, err := reviewAuthorityRoot(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "v2", lineage)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	rev := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	data := []byte(`{"revision":"` + rev + `","state":"approved","lineage_id":"` + lineage + `"}`)
	if err := os.WriteFile(filepath.Join(dir, "record.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(base, "effect-markers", "v1", lineage), 0755)
	_ = os.WriteFile(filepath.Join(base, "effect-markers", "v1", lineage, "marker.json"), []byte("marker"), 0644)
	_ = os.MkdirAll(filepath.Join(base, "incidents", lineage), 0755)
	_ = os.WriteFile(filepath.Join(base, "incidents", lineage, "capture.json"), []byte("capture"), 0644)
	store := CompactStoreFile{Dir: dir, lineageID: lineage}
	return rev, store
}

func initSnapshotRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBurnApprovedCompactAuthorityTwiceNotFound(t *testing.T) {
	repo := initSnapshotRepo(t)
	lineage := "burn-twice-notfound"
	rev, store := approvedCompactFixture(t, repo, lineage)
	base, _, _ := reviewAuthorityRoot(context.Background(), repo)

	if err := BurnApprovedCompactAuthority(context.Background(), repo, lineage, rev); err != nil {
		t.Fatalf("first burn: %v", err)
	}
	if _, err := os.Stat(store.Dir); !os.IsNotExist(err) {
		t.Fatalf("first burn left authority: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "effect-markers", "v1", lineage)); !os.IsNotExist(err) {
		t.Fatalf("burn left effect-marker")
	}
	if _, err := os.Stat(filepath.Join(base, "incidents", lineage)); !os.IsNotExist(err) {
		t.Fatalf("burn left incidents")
	}
	err := BurnApprovedCompactAuthority(context.Background(), repo, lineage, rev)
	if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "not found") && !strings.Contains(err.Error(), "no such")) {
		t.Fatalf("second burn error = %v, want lineage-not-found", err)
	}
}

func TestBurnApprovedCompactAuthorityConcurrentTimeout(t *testing.T) {
	repo := initSnapshotRepo(t)
	lineage := "burn-concurrent-timeout"
	rev, _ := approvedCompactFixture(t, repo, lineage)

	base, _, _ := reviewAuthorityRoot(context.Background(), repo)
	lease, err := storeResetAcquireLease(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	vlock, err := acquireLocalStoreLock(filepath.Join(base, "v2", "LOCK"))
	if err != nil {
		t.Fatal(err)
	}
	defer vlock.release()

	var wg sync.WaitGroup
	wg.Add(1)
	var concurrentErr error
	go func() {
		defer wg.Done()
		concurrentErr = BurnApprovedCompactAuthority(context.Background(), repo, lineage, rev)
	}()
	wg.Wait()
	if concurrentErr == nil || (!errors.Is(concurrentErr, ErrAuthorityLockTimeout) && !strings.Contains(strings.ToLower(concurrentErr.Error()), "timeout")) {
		t.Fatalf("concurrent burn error = %v, want timeout", concurrentErr)
	}
}

func TestBurnApprovedCompactAuthorityResidueIncomplete(t *testing.T) {
	repo := initSnapshotRepo(t)
	lineage := "burn-residue-incomplete"
	rev, store := approvedCompactFixture(t, repo, lineage)
	base, _, _ := reviewAuthorityRoot(context.Background(), repo)

	original := storeResetRemoveTreeFn
	storeResetRemoveTreeFn = func(path string) error {
		if path == filepath.Join(base, "effect-markers", "v1", lineage) {
			return errors.New("injected companion cleanup failure")
		}
		return os.RemoveAll(path)
	}
	defer func() { storeResetRemoveTreeFn = original }()

	err := BurnApprovedCompactAuthority(context.Background(), repo, lineage, rev)
	var incomplete *ReviewAuthorityBurnIncompleteError
	if !errors.As(err, &incomplete) {
		t.Fatalf("residue burn error = %v, want incomplete", err)
	}
	if _, statErr := os.Stat(store.Dir); statErr != nil {
		t.Fatalf("incomplete burn removed authority: %v", statErr)
	}
}
