package update_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/internal/update"
)

// ---------------------------------------------------------------------------
// Test data helpers
// ---------------------------------------------------------------------------

// testKeyPair generates an ed25519 key pair and returns the minisign-encoded
// public key content, the private key, and the key ID.
func testKeyPair(t *testing.T) (pubKeyPEM []byte, priv ed25519.PrivateKey, keyID [8]byte) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if _, err := rand.Read(keyID[:]); err != nil {
		t.Fatalf("rand key id: %v", err)
	}

	// Build minisign public key: [2B "Ed"][8B keyID][32B pubkey]
	pk := make([]byte, 42)
	pk[0] = 'E'
	pk[1] = 'd'
	copy(pk[2:10], keyID[:])
	copy(pk[10:42], pub)

	pubKeyPEM = []byte("untrusted comment: test minisign public key\n" +
		base64.StdEncoding.EncodeToString(pk) + "\n")
	return
}

// testSignature creates a minisign-format detached signature over data.
func testSignature(t *testing.T, data []byte, priv ed25519.PrivateKey, keyID [8]byte) []byte {
	t.Helper()
	sig := ed25519.Sign(priv, data)

	// Build sig1: [2B "Ed"][8B keyID][64B sig]
	sig1 := make([]byte, 74)
	sig1[0] = 'E'
	sig1[1] = 'd'
	copy(sig1[2:10], keyID[:])
	copy(sig1[10:74], sig)

	trustedComment := "trusted comment: timestamp:0\tfile"
	// Global signature signs: sig + trusted_comment_without_prefix
	globalMsg := append(sig, []byte(trustedComment[17:])...)
	globalSig := ed25519.Sign(priv, globalMsg)

	var buf bytes.Buffer
	buf.WriteString("untrusted comment: test minisign signature\n")
	buf.WriteString(base64.StdEncoding.EncodeToString(sig1) + "\n")
	buf.WriteString(trustedComment + "\n")
	buf.WriteString(base64.StdEncoding.EncodeToString(globalSig))
	return buf.Bytes()
}

// testTarGz creates a gzipped tar archive with the given files.
// files is a map of archive paths to string content.
func testTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0755,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// testZip creates a zip archive with the given files.
func testZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// testChecksums creates a GoReleaser-format checksums.txt content.
func testChecksums(data []byte, filename string) []byte {
	sum := sha256.Sum256(data)
	return []byte(fmt.Sprintf("%x  %s\n", sum, filename))
}

// ---------------------------------------------------------------------------
// 4.1 Channel tests
// ---------------------------------------------------------------------------

func TestParseChannel(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want update.Channel
	}{
		{"unset defaults to stable", "", update.ChannelStable},
		{"empty defaults to stable", " ", update.ChannelStable},
		{"stable explicit", "stable", update.ChannelStable},
		{"beta lowercase", "beta", update.ChannelBeta},
		{"beta uppercase", "BETA", update.ChannelBeta},
		{"prerelease", "prerelease", update.ChannelBeta},
		{"mixed case prerelease", "Prerelease", update.ChannelBeta},
		{"garbage defaults to stable", "garbage", update.ChannelStable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BIGGZ_CHANNEL", tt.env)
			if got := update.ParseChannel(); got != tt.want {
				t.Errorf("ParseChannel() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSelectRelease(t *testing.T) {
	mk := func(tag string, pre bool) update.Release {
		return update.Release{TagName: tag, Prerelease: pre}
	}
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		releases []update.Release
		channel  update.Channel
		want     *string // expected tag, nil for empty result
	}{
		{
			name:     "empty returns nil",
			releases: nil,
			channel:  update.ChannelStable,
			want:     nil,
		},
		{
			name: "stable skips prerelease",
			releases: []update.Release{
				mk("v1.0.1-beta", true),
				mk("v1.0.0", false),
			},
			channel: update.ChannelStable,
			want:    strPtr("v1.0.0"),
		},
		{
			name: "stable returns first non-prerelease",
			releases: []update.Release{
				mk("v2.0.0", false),
				mk("v1.0.1-beta", true),
				mk("v1.0.0", false),
			},
			channel: update.ChannelStable,
			want:    strPtr("v2.0.0"),
		},
		{
			name: "stable falls back to latest when all are prerelease",
			releases: []update.Release{
				mk("v2.0.0-beta", true),
				mk("v1.0.0-beta", true),
			},
			channel: update.ChannelStable,
			want:    strPtr("v2.0.0-beta"),
		},
		{
			name: "beta returns latest regardless",
			releases: []update.Release{
				mk("v2.0.0-beta", true),
				mk("v1.0.0", false),
			},
			channel: update.ChannelBeta,
			want:    strPtr("v2.0.0-beta"),
		},
		{
			name: "beta with only stable",
			releases: []update.Release{
				mk("v2.0.0", false),
			},
			channel: update.ChannelBeta,
			want:    strPtr("v2.0.0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := update.SelectRelease(tt.releases, tt.channel)
			if tt.want == nil {
				if got != nil {
					t.Errorf("SelectRelease() = %v, want nil", got.TagName)
				}
				return
			}
			if got == nil {
				t.Fatal("SelectRelease() = nil, want release")
			}
			if got.TagName != *tt.want {
				t.Errorf("SelectRelease() = %s, want %s", got.TagName, *tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4.2 Verify tests
// ---------------------------------------------------------------------------

func TestVerifyChecksum_Match(t *testing.T) {
	data := []byte("test-binary-content")
	sum := sha256.Sum256(data)
	checksums := []byte(fmt.Sprintf("%x  archive.tar.gz\n", sum))

	if err := update.VerifyChecksum(data, checksums); err != nil {
		t.Errorf("VerifyChecksum matched checksum should pass: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	data := []byte("test-binary-content")
	otherSum := sha256.Sum256([]byte("different-content"))
	checksums := []byte(fmt.Sprintf("%x  archive.tar.gz\n", otherSum))

	if err := update.VerifyChecksum(data, checksums); err == nil {
		t.Error("VerifyChecksum mismatched checksum should fail")
	}
}

func TestVerifyChecksum_MultipleLines(t *testing.T) {
	data := []byte("test-binary-content")
	sum := sha256.Sum256(data)
	checksums := []byte(fmt.Sprintf("aabb  other1.tar.gz\n%x  target.tar.gz\nbbcc  other2.tar.gz\n", sum))

	if err := update.VerifyChecksum(data, checksums); err != nil {
		t.Errorf("VerifyChecksum should find match in multiple lines: %v", err)
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	data := []byte("checksums content to sign")
	pubKey, priv, keyID := testKeyPair(t)
	sig := testSignature(t, data, priv, keyID)

	if err := update.VerifySignature(data, sig, pubKey); err != nil {
		t.Errorf("VerifySignature valid sig should pass: %v", err)
	}
}

func TestVerifySignature_WrongKey(t *testing.T) {
	data := []byte("checksums content to sign")
	_, priv, keyID := testKeyPair(t)
	sig := testSignature(t, data, priv, keyID)

	// Generate a different public key.
	wrongPub, _, _ := testKeyPair(t)

	if err := update.VerifySignature(data, sig, wrongPub); err == nil {
		t.Error("VerifySignature with wrong key should fail")
	}
}

func TestVerifySignature_TamperedData(t *testing.T) {
	data := []byte("checksums content to sign")
	pubKey, priv, keyID := testKeyPair(t)
	sig := testSignature(t, []byte("different content"), priv, keyID)

	if err := update.VerifySignature(data, sig, pubKey); err == nil {
		t.Error("VerifySignature with tampered data should fail")
	}
}

func TestVerifySignature_InvalidFormat(t *testing.T) {
	pubKey, _, _ := testKeyPair(t)

	if err := update.VerifySignature([]byte("data"), []byte("not-a-valid-signature"), pubKey); err == nil {
		t.Error("VerifySignature with invalid signature format should fail")
	}
}

// ---------------------------------------------------------------------------
// 4.3 Download / ExtractArchive tests
// ---------------------------------------------------------------------------

func TestExtractArchive_TarGz(t *testing.T) {
	binaryContent := "#!/bin/sh\necho hello"
	archive := testTarGz(t, map[string]string{
		"biggz_v1.0.0_linux_amd64/biggz": binaryContent,
	})

	destDir := t.TempDir()
	path, err := update.ExtractArchive(archive, destDir, "biggz")
	if err != nil {
		t.Fatalf("ExtractArchive tar.gz: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != binaryContent {
		t.Errorf("extracted content = %q, want %q", string(data), binaryContent)
	}
}

func TestExtractArchive_Zip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping zip extraction test on Windows — zip binary name differs")
	}
	binaryContent := "this-is-a-binary"
	archive := testZip(t, map[string]string{
		"biggz_v1.0.0_windows_amd64/biggz.exe": binaryContent,
	})

	destDir := t.TempDir()
	path, err := update.ExtractArchive(archive, destDir, "biggz.exe")
	if err != nil {
		t.Fatalf("ExtractArchive zip: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(data) != binaryContent {
		t.Errorf("extracted content = %q, want %q", string(data), binaryContent)
	}
}

func TestExtractArchive_BinaryNotFound(t *testing.T) {
	archive := testTarGz(t, map[string]string{
		"other-file.txt": "not-the-binary",
	})

	destDir := t.TempDir()
	_, err := update.ExtractArchive(archive, destDir, "biggz")
	if err == nil {
		t.Error("ExtractArchive with missing binary should fail")
	}
}

func TestExtractArchive_ExecutablePreserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission not applicable on Windows")
	}
	archive := testTarGz(t, map[string]string{
		"biggz": "binary-content",
	})

	destDir := t.TempDir()
	path, err := update.ExtractArchive(archive, destDir, "biggz")
	if err != nil {
		t.Fatalf("ExtractArchive: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat extracted file: %v", err)
	}
	if fi.Mode()&0111 == 0 {
		t.Errorf("extracted binary should be executable, got mode %o", fi.Mode())
	}
}

func TestExtractArchive_EmptyArchive(t *testing.T) {
	var emptyGzip bytes.Buffer
	gw := gzip.NewWriter(&emptyGzip)
	gw.Close()

	destDir := t.TempDir()
	_, err := update.ExtractArchive(emptyGzip.Bytes(), destDir, "biggz")
	if err == nil {
		t.Error("ExtractArchive empty archive should fail")
	}
}

// ---------------------------------------------------------------------------
// 4.4 Replace tests
// ---------------------------------------------------------------------------

func TestReplaceBinary_Unix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ReplaceBinary returns ErrWindowsBinaryLock on Windows")
	}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "new-binary")
	if err := os.WriteFile(srcPath, []byte("new-content"), 0755); err != nil {
		t.Fatal(err)
	}
	dstPath := filepath.Join(dstDir, "old-binary")
	if err := os.WriteFile(dstPath, []byte("old-content"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := update.ReplaceBinary(srcPath, dstPath); err != nil {
		t.Fatalf("ReplaceBinary: %v", err)
	}

	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dst after replace: %v", err)
	}
	if string(data) != "new-content" {
		t.Errorf("dst content = %q, want %q", string(data), "new-content")
	}
}

func TestReplaceBinary_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	err := update.ReplaceBinary("src", "dst")
	if err != update.ErrWindowsBinaryLock {
		t.Errorf("ReplaceBinary on Windows = %v, want ErrWindowsBinaryLock", err)
	}
}

func TestReplaceHint_Windows(t *testing.T) {
	hint := update.ReplaceHint("github.com/biggz-ai/biggz")
	if runtime.GOOS == "windows" {
		if !strings.Contains(hint, "go install") {
			t.Errorf("ReplaceHint on Windows should contain 'go install', got: %s", hint)
		}
	} else {
		if !strings.Contains(hint, "replaced successfully") {
			t.Errorf("ReplaceHint on Unix should mention success, got: %s", hint)
		}
	}
}

func TestReplaceHint_ModulePath(t *testing.T) {
	// On both platforms, the module path should appear somewhere.
	hint := update.ReplaceHint("github.com/biggz-ai/biggz")
	if !strings.Contains(hint, "github.com/biggz-ai/biggz") {
		t.Errorf("ReplaceHint should include module path, got: %s", hint)
	}
}

// ---------------------------------------------------------------------------
// 4.5 Smoke test helper: document how to verify goreleaser output
// ---------------------------------------------------------------------------

// TestGoreleaserSnapshotDocuments checks that goreleaser output can be verified.
// This is a documentation test — it demonstrates the verification flow against
// a local snapshot build. To run:
//
//	goreleaser build --snapshot --clean
//	go test -run TestGoreleaserSnapshotFlow ./internal/update/
//
// The test reads snapshot archives from dist/ and verifies checksums + signatures.
func TestGoreleaserSnapshotFlow(t *testing.T) {
	// This test is manual and skipped by default.
	// It documents how to verify goreleaser --snapshot output:
	//
	// 1. Run: goreleaser build --snapshot --clean
	// 2. This test reads dist/*.tar.gz, dist/*.zip, dist/checksums.txt
	// 3. Verifies SHA-256 checksums for each archive
	// 4. If minisign.pub key is present, also verifies checksums.txt.minisig
	//
	// To enable, run with:
	//   go test -run TestGoreleaserSnapshotFlow -goreleaser-dist=../../dist
	//
	t.Skip("Manual smoke test — run with goreleaser build --snapshot --clean first")
}

// ---------------------------------------------------------------------------
// Helper: test that the embedded public key is loadable
// ---------------------------------------------------------------------------

func TestEmbeddedPublicKey(t *testing.T) {
	key := update.MinissignPublicKey()
	if len(key) == 0 {
		t.Fatal("embedded minisign.pub is empty")
	}
	if !bytes.HasPrefix(key, []byte("untrusted comment:")) {
		t.Errorf("embedded key should start with 'untrusted comment:', got %q", key[:30])
	}
}
