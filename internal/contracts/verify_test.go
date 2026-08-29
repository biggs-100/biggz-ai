package contracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempContract(t *testing.T, root string, rel string, content string) string {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return abs
}

func buildLock(t *testing.T, tmp string, rels []string) string {
	t.Helper()
	m := make(map[string]string)
	for _, rel := range rels {
		abs := filepath.Join(tmp, "contracts/review-integration", rel)
		b, err := os.ReadFile(abs)
		if err != nil {
			t.Fatalf("hash %s: %v", rel, err)
		}
		h := sha256.Sum256(b)
		m["contracts/review-integration/"+rel] = hex.EncodeToString(h[:])
	}
	lockPath := filepath.Join(tmp, "contracts/review-integration/provider-contract.lock.json")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(lockPath, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return lockPath
}

func TestVerifyProviderContractExactPinsPass(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "contracts/review-integration")
	writeTempContract(t, tmp, "contracts/review-integration/v1/fixtures/a.json", `{"a":1}`)
	writeTempContract(t, tmp, "contracts/review-integration/v1/schemas/b.json", `{"b":2}`)
	lock := buildLock(t, tmp, []string{"v1/fixtures/a.json", "v1/schemas/b.json"})
	if err := VerifyProviderContract(lock, root); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
}

func TestVerifyProviderContractOneByteDriftFails(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "contracts/review-integration")
	abs := writeTempContract(t, tmp, "contracts/review-integration/v1/fixtures/a.json", `{"a":1}`)
	writeTempContract(t, tmp, "contracts/review-integration/v1/schemas/b.json", `{"b":2}`)
	lock := buildLock(t, tmp, []string{"v1/fixtures/a.json", "v1/schemas/b.json"})
	// 1-byte drift
	orig, _ := os.ReadFile(abs)
	if err := os.WriteFile(abs, append(orig, ' '), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProviderContract(lock, root); err == nil {
		t.Fatal("expected drift failure, got nil")
	}
}

func TestVerifyProviderContractOfflineNoFetch(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "contracts/review-integration")
	writeTempContract(t, tmp, "contracts/review-integration/v1/fixtures/a.json", `{"a":1}`)
	lock := buildLock(t, tmp, []string{"v1/fixtures/a.json"})
	// Offline: no network call should occur; verify succeeds without env
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "*")
	if err := VerifyProviderContract(lock, root); err != nil {
		t.Fatalf("offline verify failed: %v", err)
	}
	// Also ensure missing file is detected (manifest mismatch)
	if err := os.Remove(filepath.Join(tmp, "contracts/review-integration/v1/fixtures/a.json")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProviderContract(lock, root); err == nil {
		t.Fatal("expected missing file failure")
	}
}
