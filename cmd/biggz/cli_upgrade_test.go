package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/update"
)

// fakeReleasesServer creates an httptest server that serves the given releases
// for ListReleases and GetRelease endpoints. It sets BIGGZ_GITHUB_API_BASE
// via env for subprocess tests and also overrides update.GitHubAPIBase for
// in-process tests. Returns the server and a cleanup func.
func fakeReleasesServer(t *testing.T, releases []update.Release) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ListReleases: GET /repos/{owner}/{repo}/releases
		if strings.HasSuffix(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/tags/") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(releases)
			return
		}
		// GetRelease: GET /repos/{owner}/{repo}/releases/tags/{tag}
		if strings.Contains(r.URL.Path, "/releases/tags/") {
			parts := strings.Split(r.URL.Path, "/")
			tag := parts[len(parts)-1]
			for _, rel := range releases {
				if rel.TagName == tag {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(rel)
					return
				}
			}
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(func() { srv.Close() })
	return srv
}

func TestUpdate_HelpPrintsCheckOnly(t *testing.T) {
	cmd := goRunBiggz(t, "update", "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("update --help: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "Usage: biggz update") {
		t.Errorf("update help should contain 'Usage: biggz update', got %q", out)
	}
	if !strings.Contains(out, "Check for available updates") {
		t.Errorf("update help should mention check, got %q", out)
	}
	if strings.Contains(out, "--no-reconcile") {
		t.Errorf("update check-only help should not mention --no-reconcile, got %q", out)
	}
	if !strings.Contains(out, "biggz upgrade") {
		t.Errorf("update help should hint at 'biggz upgrade', got %q", out)
	}
}

func TestUpgrade_HelpPrintsUsage(t *testing.T) {
	cmd := goRunBiggz(t, "upgrade", "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("upgrade --help: %v", err)
	}
	out := stderr.String()
	if !strings.Contains(out, "Usage: biggz upgrade") {
		t.Errorf("upgrade help should contain 'Usage: biggz upgrade', got %q", out)
	}
	for _, flag := range []string{"--dry-run", "--version", "--no-reconcile", "--no-backup"} {
		if !strings.Contains(out, flag) {
			t.Errorf("upgrade help should mention %q, got %q", flag, out)
		}
	}
}

func TestUpdate_CheckOnlyDoesNotCreateBackup(t *testing.T) {
	tmpHome := t.TempDir()
	// Set both HOME and USERPROFILE for cross-platform isolation.
	cmd := goRunBiggz(t, "update", "--help")
	cmd.Env = append(os.Environ(),
		"HOME="+tmpHome,
		"USERPROFILE="+tmpHome,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("update --help: %v", err)
	}
	// Check no backup was created (check-only must never mutate).
	backupDir := filepath.Join(tmpHome, ".biggz", "backups")
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("update --help should not create backup, found %d entries in %s", len(entries), backupDir)
	}
	// Also verify that running plain update (which will error 404) does not create backup.
	cmd2 := goRunBiggz(t, "update")
	cmd2.Env = append(os.Environ(),
		"HOME="+tmpHome,
		"USERPROFILE="+tmpHome,
	)
	var stdout2, stderr2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	_ = cmd2.Run() // may error due to 404, ignore exit
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("biggz update (no flags) should not create backup, found %d entries", len(entries))
	}
	combined := stdout2.String() + stderr2.String()
	if !strings.Contains(combined, "Update available") && !strings.Contains(combined, "Already up to date") && !strings.Contains(combined, "error:") && !strings.Contains(combined, "no releases") {
		t.Errorf("expected check output (Update available / Already up to date / error), got %q", combined)
	}
}

func TestUpgrade_DryRunDoesNotMutate(t *testing.T) {
	tmpHome := t.TempDir()
	// Run upgrade --help to verify flag is recognized without mutation.
	cmd := goRunBiggz(t, "upgrade", "--help")
	cmd.Env = append(os.Environ(),
		"HOME="+tmpHome,
		"USERPROFILE="+tmpHome,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("upgrade --help: %v", err)
	}
	backupDir := filepath.Join(tmpHome, ".biggz", "backups")
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("upgrade --help should not create backup, found %d entries", len(entries))
	}
	// Run upgrade --dry-run (will 404 due to no real releases) — still should not create backup.
	cmd2 := goRunBiggz(t, "upgrade", "--dry-run")
	cmd2.Env = append(os.Environ(),
		"HOME="+tmpHome,
		"USERPROFILE="+tmpHome,
	)
	var stdout2, stderr2 bytes.Buffer
	cmd2.Stdout = &stdout2
	cmd2.Stderr = &stderr2
	_ = cmd2.Run()
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("upgrade --dry-run should not create backup on check failure, found %d entries", len(entries))
	}
}

func TestUpgrade_DryRunPrintsPendingWhenUpdateAvailable(t *testing.T) {
	// Use httptest to provide a fake release newer than current version.
	// This test exercises the dry-run early return path (discoverRelease + up-to-date check + pending hint)
	// without downloading. It runs the CLI via env var injection to mock GitHub.
	releases := []update.Release{
		{TagName: "v9.9.9", Prerelease: false, Assets: []update.Asset{
			{Name: "checksums.txt", URL: "http://example.com/checksums.txt"},
			{Name: "checksums.txt.minisig", URL: "http://example.com/checksums.txt.minisig"},
			{Name: "biggz_v9.9.9_linux_amd64.tar.gz", URL: "http://example.com/archive.tar.gz"},
		}},
	}
	srv := fakeReleasesServer(t, releases)
	tmpHome := t.TempDir()
	cmd := goRunBiggz(t, "upgrade", "--dry-run")
	cmd.Env = append(os.Environ(),
		"HOME="+tmpHome,
		"USERPROFILE="+tmpHome,
		"BIGGZ_GITHUB_API_BASE="+srv.URL,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("upgrade --dry-run with fake release failed: %v stderr=%s stdout=%s", err, stderr.String(), stdout.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "Update available") {
		t.Errorf("expected 'Update available' in dry-run output, got %q", combined)
	}
	if !strings.Contains(combined, "Upgrade (dry-run)") {
		t.Errorf("expected 'Upgrade (dry-run)' hint, got %q", combined)
	}
	if !strings.Contains(combined, "1 upgrade(s) pending") {
		t.Errorf("expected pending count, got %q", combined)
	}
	// Ensure no backup was created on dry-run (dry-run must not mutate).
	backupDir := filepath.Join(tmpHome, ".biggz", "backups")
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("upgrade --dry-run should not create backup, found %d entries", len(entries))
	}
	// Ensure no download occurred (no snapshot message either — dry-run skips snapshot)
	if strings.Contains(combined, "Snapshot created") {
		t.Errorf("dry-run should not create snapshot, got %q", combined)
	}
}

func TestUpdate_CheckPrintsAvailableWithFakeRelease(t *testing.T) {
	releases := []update.Release{
		{TagName: "v9.9.9", Prerelease: false, Assets: []update.Asset{}},
	}
	srv := fakeReleasesServer(t, releases)
	tmpHome := t.TempDir()
	cmd := goRunBiggz(t, "update")
	cmd.Env = append(os.Environ(),
		"HOME="+tmpHome,
		"USERPROFILE="+tmpHome,
		"BIGGZ_GITHUB_API_BASE="+srv.URL,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("update with fake release failed: %v stderr=%s stdout=%s", err, stderr.String(), stdout.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "Update available") {
		t.Errorf("expected 'Update available', got %q", combined)
	}
	if !strings.Contains(combined, "Run 'biggz upgrade'") {
		t.Errorf("expected upgrade hint, got %q", combined)
	}
	// Ensure update did not create backup.
	backupDir := filepath.Join(tmpHome, ".biggz", "backups")
	if entries, err := os.ReadDir(backupDir); err == nil && len(entries) > 0 {
		t.Errorf("update should not create backup, found %d entries", len(entries))
	}
}
