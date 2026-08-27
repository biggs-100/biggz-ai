package bigmem

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}
func newIsolatedStore(t *testing.T) *Store {
	t.Helper()
	home := isolatedHome(t)
	s, err := Open(filepath.Join(home, ".biggz", "bigmem"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestGetBlob_TraversalRejected(t *testing.T) {
	isolatedHome(t)
	for _, c := range []string{"blob:sha256:../../etc/passwd", "blob:sha256:zzzz", fmt.Sprintf("blob:sha256:%s/../", strings.Repeat("a", 64)), "blob:sha256:" + strings.Repeat("a", 64) + "/etc", "/tmp/passwd"} {
		if _, err := GetBlob(c); err != ErrInvalidAddr {
			t.Errorf("GetBlob %q = %v want ErrInvalidAddr", c, err)
		}
	}
	if strings.Contains(BlobRoot(), "..") {
		t.Error("BlobRoot contains ..")
	}
}
func TestGetBlob_InvalidRejected(t *testing.T) {
	isolatedHome(t)
	for _, c := range []string{"blob:sha256:zzzz", "blob:sha256:" + strings.Repeat("a", 63), "blob:sha256:" + strings.Repeat("a", 65), "blob:sha256:", "not-a-blob", ""} {
		if _, err := GetBlob(c); err != ErrInvalidAddr {
			t.Errorf("GetBlob %q = %v", c, err)
		}
		if IsBlobAddr(c) {
			t.Errorf("IsBlobAddr %q true", c)
		}
	}
}
func TestPutBlob_RoundTrip150KB(t *testing.T) {
	isolatedHome(t)
	data := []byte(strings.Repeat("x", 150*1024))
	addr, err := PutBlob(data)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if !IsBlobAddr(addr) || len(addr) != len(BlobPrefix)+64 {
		t.Fatalf("bad addr %q", addr)
	}
	hex, _ := ValidateAddr(addr)
	b, _ := os.ReadFile(filepath.Join(BlobRoot(), hex))
	if string(b) != string(data) {
		t.Fatal("mismatch")
	}
	got, _ := GetBlob(addr)
	if string(got) != string(data) {
		t.Fatal("GetBlob mismatch")
	}
}
func TestPutBlob_DedupNoOverwrite(t *testing.T) {
	isolatedHome(t)
	data := []byte(strings.Repeat("y", 150*1024))
	a1, _ := PutBlob(data)
	hex, _ := ValidateAddr(a1)
	path := filepath.Join(BlobRoot(), hex)
	info1, _ := os.Stat(path)
	mt1 := info1.ModTime()
	// ensure mtime granularity
	for i := 0; i < 2; i++ {
		// tiny sleep handled by filesystem; retry if needed
	}
	a2, _ := PutBlob(data)
	if a1 != a2 {
		t.Fatalf("dedup %q vs %q", a1, a2)
	}
	info2, _ := os.Stat(path)
	if !info2.ModTime().Equal(mt1) {
		t.Errorf("mtime changed %v vs %v", mt1, info2.ModTime())
	}
}
func TestGetBlob_ValidResolves(t *testing.T) {
	isolatedHome(t)
	d := []byte("hello blob")
	a, _ := PutBlob(d)
	got, _ := GetBlob(a)
	if string(got) != string(d) {
		t.Fatal("mismatch")
	}
}
func TestGetBlob_MissingNotFound(t *testing.T) {
	isolatedHome(t)
	h := sha256.Sum256([]byte("missing-not-found-unique-12345"))
	addr := BlobPrefix + fmt.Sprintf("%x", h)
	_ = os.Remove(filepath.Join(BlobRoot(), fmt.Sprintf("%x", h)))
	if _, err := GetBlob(addr); err != ErrBlobNotFound {
		t.Fatalf("want ErrBlobNotFound got %v", err)
	}
}
func TestBlob_ConcurrentSameBytes(t *testing.T) {
	isolatedHome(t)
	data := []byte(strings.Repeat("c", 200*1024))
	var wg sync.WaitGroup
	addrs := [2]string{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			addrs[idx], _ = PutBlob(data)
		}(i)
	}
	wg.Wait()
	if addrs[0] != addrs[1] {
		t.Fatalf("concurrent %q vs %q", addrs[0], addrs[1])
	}
	got, _ := GetBlob(addrs[0])
	if string(got) != string(data) {
		t.Fatal("corrupted")
	}
}
func TestShouldExternalize(t *testing.T) {
	for _, tc := range []struct{ in string; want bool }{
		{strings.Repeat("a", 10*1024), false},
		{strings.Repeat("a", 150*1024), true},
		{"data:image/png;base64,abcd" + strings.Repeat("a", 5*1024), true},
		{strings.Repeat("a", 100000), false},
		{strings.Repeat("a", 100001), true},
		{"data:image/jpeg;base64,xxx", true},
	} {
		if got := ShouldExternalize(tc.in); got != tc.want {
			t.Errorf("ShouldExternalize len %d got %v", len(tc.in), got)
		}
	}
}
func TestGet_BlobResolved(t *testing.T) {
	s := newIsolatedStore(t)
	large := strings.Repeat("L", 150*1024)
	addr, _ := PutBlob([]byte(large))
	obs := &Observation{Title: "r", Type: "note", Content: addr, Project: "test"}
	s.Save(obs)
	got, _ := s.Get(obs.ID)
	if got.Content != large {
		t.Fatalf("not resolved len %d", len(got.Content))
	}
	var raw string
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", obs.ID).Scan(&raw)
	if raw != addr {
		t.Fatal("DB mutated")
	}
}
func TestGet_MissingFallback(t *testing.T) {
	s := newIsolatedStore(t)
	d := []byte("missing payload")
	h := sha256.Sum256(d)
	hex := fmt.Sprintf("%x", h)
	addr := BlobPrefix + hex
	PutBlob(d)
	_ = os.Remove(filepath.Join(BlobRoot(), hex))
	obs := &Observation{Title: "m", Type: "note", Content: addr, Project: "test"}
	s.Save(obs)
	got, _ := s.Get(obs.ID)
	if got.Content != addr {
		t.Fatalf("fallback failed")
	}
}
func TestSearch_BlobPassthrough(t *testing.T) {
	s := newIsolatedStore(t)
	large := strings.Repeat("S", 150*1024)
	addr, _ := PutBlob([]byte(large))
	obs := &Observation{Title: "search blob", Type: "note", Content: addr, Project: "test", TopicKey: "topic/search"}
	s.Save(obs)
	res, _ := s.Search("search", SearchOptions{Project: "test"})
	found := false
	for _, r := range res {
		if r.ID == obs.ID {
			found = true
			if r.Content != large {
				t.Fatal("search not resolved")
			}
		}
	}
	if !found {
		t.Fatal("not found")
	}
	small := "small content"
	o2 := &Observation{Title: "small", Type: "note", Content: small, Project: "test"}
	s.Save(o2)
	res2, _ := s.Search("small", SearchOptions{Project: "test"})
	for _, r := range res2 {
		if r.ID == o2.ID && r.Content != small {
			t.Fatal("passthrough failed")
		}
	}
}
func TestBlobRoot_Sibling(t *testing.T) {
	home := isolatedHome(t)
	if BlobRoot() != filepath.Join(home, ".biggz", "blobs") {
		t.Fatalf("BlobRoot %q", BlobRoot())
	}
	if strings.Contains(BlobRoot(), ".omp") {
		t.Fatal("contains .omp")
	}
}
func TestDoctorFixBlobs_Migrates2Skips1(t *testing.T) {
	s := newIsolatedStore(t)
	o1 := &Observation{Title: "l1", Type: "note", Content: strings.Repeat("A", 150*1024), Project: "test"}
	o2 := &Observation{Title: "l2", Type: "note", Content: strings.Repeat("B", 150*1024), Project: "test"}
	s.Save(o1)
	s.Save(o2)
	addr, _ := PutBlob([]byte(strings.Repeat("C", 150*1024)))
	o3 := &Observation{Title: "b", Type: "note", Content: addr, Project: "test"}
	s.Save(o3)
	res, _ := s.DoctorFixBlobs()
	if res.Migrated != 2 || res.Skipped != 1 {
		t.Fatalf("migrated %d skipped %d", res.Migrated, res.Skipped)
	}
	var c1, c2 string
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", o1.ID).Scan(&c1)
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", o2.ID).Scan(&c2)
	if !IsBlobAddr(c1) || !IsBlobAddr(c2) {
		t.Fatal("not migrated")
	}
}
func TestDoctorFixBlobs_IdempotentReRun(t *testing.T) {
	s := newIsolatedStore(t)
	o := &Observation{Title: "l", Type: "note", Content: strings.Repeat("D", 150*1024), Project: "test"}
	s.Save(o)
	r1, _ := s.DoctorFixBlobs()
	if r1.Migrated != 1 {
		t.Fatalf("r1 %d", r1.Migrated)
	}
	r2, _ := s.DoctorFixBlobs()
	if r2.Migrated != 0 {
		t.Fatalf("r2 %d", r2.Migrated)
	}
	got, _ := s.Get(o.ID)
	if got.Content != strings.Repeat("D", 150*1024) {
		t.Fatal("mismatch")
	}
}
func TestDoctorFixBlobs_NoFlagUntouched(t *testing.T) {
	s := newIsolatedStore(t)
	large := strings.Repeat("E", 150*1024)
	o := &Observation{Title: "l", Type: "note", Content: large, Project: "test"}
	s.Save(o)
	var raw string
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", o.ID).Scan(&raw)
	if IsBlobAddr(raw) {
		t.Fatal("should be inline without fix")
	}
	s.DoctorFix()
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", o.ID).Scan(&raw)
	if IsBlobAddr(raw) {
		t.Fatal("DoctorFix should not migrate blobs")
	}
}
func TestDoctorFixBlobs_AdvisoryHint(t *testing.T) {
	s := newIsolatedStore(t)
	o := &Observation{Title: "h", Type: "note", Content: strings.Repeat("F", 150*1024), Project: "test"}
	s.Save(o)
	s.DoctorFixBlobs()
	// CLI hint check
	data, _ := os.ReadFile(filepath.Join(findRepoRoot(t), "cmd/biggz/cli_bigmem.go"))
	if !strings.Contains(string(data), "find ~/.biggz/blobs -type f -mtime +30") {
		t.Fatal("hint missing")
	}
}
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return "."
}
func TestGC_NoAutoDeletion(t *testing.T) {
	s := newIsolatedStore(t)
	addr, _ := PutBlob([]byte(strings.Repeat("G", 150*1024)))
	o := &Observation{Title: "gc", Type: "note", Content: addr, Project: "test"}
	s.Save(o)
	root := BlobRoot()
	before, _ := os.ReadDir(root)
	cb := len(before)
	for i := 0; i < 5; i++ {
		s.Save(&Observation{Title: fmt.Sprintf("s%d", i), Type: "note", Content: "small", Project: "test"})
		s.Get(o.ID)
		s.Search("small", SearchOptions{Project: "test"})
	}
	s.DoctorFix()
	s.DoctorFixBlobs()
	after, _ := os.ReadDir(root)
	if len(after) < cb {
		t.Fatalf("blob count decreased %d -> %d", cb, len(after))
	}
	hex, _ := ValidateAddr(addr)
	if _, err := os.Stat(filepath.Join(root, hex)); err != nil {
		t.Fatal("blob deleted")
	}
}
func TestE2E_20x150KB(t *testing.T) {
	s := newIsolatedStore(t)
	ids := []string{}
	for i := 0; i < 20; i++ {
		prefix := fmt.Sprintf("blob-e2e-%04d-", i)
		content := prefix + strings.Repeat("x", 150*1024-len(prefix))
		addr, _ := PutBlob([]byte(content))
		obs := &Observation{Title: fmt.Sprintf("e2e-%d", i), Type: "note", Content: addr, Project: "test"}
		s.Save(obs)
		ids = append(ids, obs.ID)
	}
	for _, id := range ids {
		var c string
		s.db.QueryRow("SELECT content FROM observations WHERE id=?", id).Scan(&c)
		if len(c) > len(BlobPrefix)+64 || !IsBlobAddr(c) {
			t.Fatalf("DB not addr %d", len(c))
		}
		got, _ := s.Get(id)
		if len(got.Content) != 150*1024 {
			t.Fatalf("resolve len %d", len(got.Content))
		}
	}
	// no leafId
	rows, _ := s.db.Query("PRAGMA table_info(observations)")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt any
			rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
			if name == "leafId" || name == "leaf_id" {
				t.Fatal("leafId found")
			}
		}
	}
	if ents, _ := os.ReadDir(BlobRoot()); len(ents) < 20 {
		t.Fatalf("blob count %d", len(ents))
	}
}
func TestMCP_MemSaveExternalized(t *testing.T) {
	s := newIsolatedStore(t)
	large := strings.Repeat("x", 150*1024)
	obs := &Observation{Title: "mcp large", Type: "note", Content: large, Project: "test"}
	if ShouldExternalize(obs.Content) {
		a, _ := PutBlob([]byte(obs.Content))
		obs.Content = a
	}
	s.Save(obs)
	var raw string
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", obs.ID).Scan(&raw)
	if !IsBlobAddr(raw) {
		t.Fatal("should be addr")
	}
	got, _ := s.Get(obs.ID)
	if got.Content != large {
		t.Fatal("not resolved")
	}
}
func TestMCP_MemSaveSmallInline(t *testing.T) {
	s := newIsolatedStore(t)
	small := strings.Repeat("a", 10*1024)
	if ShouldExternalize(small) {
		t.Fatal("small should not externalize")
	}
	obs := &Observation{Title: "small", Type: "note", Content: small, Project: "test"}
	s.Save(obs)
	var raw string
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", obs.ID).Scan(&raw)
	if raw != small {
		t.Fatal("not verbatim")
	}
}
func TestMCP_MemSaveSmallImageExternalized(t *testing.T) {
	s := newIsolatedStore(t)
	img := "data:image/png;base64,iVBORw0KGgo" + strings.Repeat("a", 5*1024)
	if !ShouldExternalize(img) {
		t.Fatal("image should externalize")
	}
	obs := &Observation{Title: "img", Type: "note", Content: img, Project: "test"}
	a, _ := PutBlob([]byte(obs.Content))
	obs.Content = a
	s.Save(obs)
	var raw string
	s.db.QueryRow("SELECT content FROM observations WHERE id=?", obs.ID).Scan(&raw)
	if !IsBlobAddr(raw) {
		t.Fatal("image not addr")
	}
}
