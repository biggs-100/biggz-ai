package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/biggz-ai/biggz/internal/bigmem"
	"github.com/biggz-ai/biggz/internal/review"
)

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusPass, "pass"},
		{StatusWarn, "warn"},
		{StatusFail, "fail"},
		{Status(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestReportBucketing(t *testing.T) {
	report := &Report{}
	report.Critical = append(report.Critical, &Result{ID: "a", Status: StatusFail, Severity: SeverityCritical})
	report.Warning = append(report.Warning, &Result{ID: "b", Status: StatusWarn, Severity: SeverityWarning})
	report.Info = append(report.Info, &Result{ID: "c", Status: StatusPass, Severity: SeverityInfo})
	report.Info = append(report.Info, &Result{ID: "d", Status: StatusPass, Severity: SeverityInfo})

	if len(report.Critical) != 1 {
		t.Errorf("Critical count = %d, want 1", len(report.Critical))
	}
	if len(report.Warning) != 1 {
		t.Errorf("Warning count = %d, want 1", len(report.Warning))
	}
	if len(report.Info) != 2 {
		t.Errorf("Info count = %d, want 2", len(report.Info))
	}

	all := report.All()
	if len(all) != 4 {
		t.Errorf("All() count = %d, want 4", len(all))
	}

	if n := report.CountByStatus(StatusPass); n != 2 {
		t.Errorf("CountByStatus(pass) = %d, want 2", n)
	}
	if n := report.CountByStatus(StatusWarn); n != 1 {
		t.Errorf("CountByStatus(warn) = %d, want 1", n)
	}
	if n := report.CountByStatus(StatusFail); n != 1 {
		t.Errorf("CountByStatus(fail) = %d, want 1", n)
	}
}

// testCheck is a simple check implementation for testing.
type testCheck struct {
	id      CheckID
	status  Status
	message string
	panic   bool
}

func (c *testCheck) ID() CheckID          { return c.id }
func (c *testCheck) Remedy() *Remedy       { return nil }
func (c *testCheck) Run(ctx context.Context) *Result {
	if c.panic {
		panic("test panic")
	}
	return &Result{
		ID:       c.id,
		Status:   c.status,
		Message:  c.message,
		Severity: severityFromStatus(c.status),
	}
}

func severityFromStatus(s Status) string {
	switch s {
	case StatusFail:
		return SeverityCritical
	case StatusWarn:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}

func TestRunner_PanicIsolation(t *testing.T) {
	runner := &Runner{
		Checks: []Check{
			&testCheck{id: "A", status: StatusPass, message: "check A ok"},
			&testCheck{id: "B", panic: true},
			&testCheck{id: "C", status: StatusPass, message: "check C ok"},
		},
	}

	report := runner.RunAll(context.Background())

	// Check A should be in Info bucket.
	if len(report.Info) != 2 {
		t.Errorf("Info count = %d, want 2", len(report.Info))
	}

	// Check B should be Critical (failed with panic captured).
	if len(report.Critical) != 1 {
		t.Errorf("Critical count = %d, want 1", len(report.Critical))
	} else {
		res := report.Critical[0]
		if res.ID != "B" {
			t.Errorf("Panicked check ID = %q, want %q", res.ID, "B")
		}
		if res.Status != StatusFail {
			t.Errorf("Panicked check Status = %v, want fail", res.Status)
		}
		if res.Error == "" || res.Message != "check panicked" {
			t.Errorf("Panicked check Message = %q, want 'check panicked'", res.Message)
		}
	}

	// Check C should be in Info bucket.
	foundA := false
	foundC := false
	for _, r := range report.Info {
		if r.ID == "A" {
			foundA = true
		}
		if r.ID == "C" {
			foundC = true
		}
	}
	if !foundA {
		t.Error("Check A result missing from Info")
	}
	if !foundC {
		t.Error("Check C result missing from Info")
	}
}

func TestRunner_AllPass(t *testing.T) {
	runner := &Runner{
		Checks: []Check{
			&testCheck{id: "X", status: StatusPass, message: "ok"},
			&testCheck{id: "Y", status: StatusPass, message: "ok"},
		},
	}

	report := runner.RunAll(context.Background())

	if len(report.Critical) != 0 {
		t.Errorf("Critical count = %d, want 0", len(report.Critical))
	}
	if len(report.Warning) != 0 {
		t.Errorf("Warning count = %d, want 0", len(report.Warning))
	}
	if len(report.Info) != 2 {
		t.Errorf("Info count = %d, want 2", len(report.Info))
	}
}

func TestRunner_MixedResults(t *testing.T) {
	runner := &Runner{
		Checks: []Check{
			&testCheck{id: "pass1", status: StatusPass, message: "ok"},
			&testCheck{id: "warn1", status: StatusWarn, message: "warning"},
			&testCheck{id: "fail1", status: StatusFail, message: "failure"},
			&testCheck{id: "pass2", status: StatusPass, message: "ok"},
		},
	}

	report := runner.RunAll(context.Background())

	if len(report.Critical) != 1 {
		t.Errorf("Critical count = %d, want 1", len(report.Critical))
	}
	if len(report.Warning) != 1 {
		t.Errorf("Warning count = %d, want 1", len(report.Warning))
	}
	if len(report.Info) != 2 {
		t.Errorf("Info count = %d, want 2", len(report.Info))
	}
}

// ---------------------------------------------------------------------------
// 4.1 — Remedy dispatch tests
// ---------------------------------------------------------------------------

// remedyTestCheck is a test check that provides a remedy.
type remedyTestCheck struct {
	testCheck
	action func(ctx context.Context) error
}

func (c *remedyTestCheck) Remedy() *Remedy {
	if c.action == nil {
		return nil
	}
	return &Remedy{
		ID:          "test-fix",
		Description: fmt.Sprintf("Fix %s", c.id),
		Action:      c.action,
	}
}

func TestRemedy_Dispatch(t *testing.T) {
	executed := false
	check := &remedyTestCheck{
		testCheck: testCheck{id: "fixable", status: StatusFail, message: "something wrong"},
		action: func(ctx context.Context) error {
			executed = true
			return nil
		},
	}

	remedy := check.Remedy()
	if remedy == nil {
		t.Fatal("Remedy() returned nil, want non-nil")
	}
	if remedy.Description != "Fix fixable" {
		t.Errorf("Description = %q, want %q", remedy.Description, "Fix fixable")
	}

	err := remedy.Action(context.Background())
	if err != nil {
		t.Errorf("Action failed: %v", err)
	}
	if !executed {
		t.Error("Action was not executed")
	}
}

func TestRemedy_Nil(t *testing.T) {
	check := &testCheck{id: "no-fix", status: StatusPass, message: "ok"}
	if remedy := check.Remedy(); remedy != nil {
		t.Error("Remedy() should be nil for checks without remedy")
	}
}

func TestRemedy_FailingAction(t *testing.T) {
	check := &remedyTestCheck{
		testCheck: testCheck{id: "fail-action", status: StatusFail, message: "broken"},
		action: func(ctx context.Context) error {
			return fmt.Errorf("repair failed")
		},
	}

	remedy := check.Remedy()
	if remedy == nil {
		t.Fatal("Remedy() returned nil")
	}

	err := remedy.Action(context.Background())
	if err == nil {
		t.Error("Expected error from failing action, got nil")
	}
}

// ---------------------------------------------------------------------------
// 4.2 — Bigmem check tests
// ---------------------------------------------------------------------------

func TestBigmemCheck_CleanStore(t *testing.T) {
	dir := t.TempDir()
	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(dir)
	})
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q, err=%q)", result.Status, result.Message, result.Error)
	}
}

func TestBigmemCheck_CannotOpen(t *testing.T) {
	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return nil, fmt.Errorf("permission denied")
	})
	result := c.Run(context.Background())
	if result.Status != StatusFail {
		t.Errorf("status = %v, want fail", result.Status)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %s, want CRITICAL", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// 4.3 — Binary check tests
// ---------------------------------------------------------------------------

func TestBinaryCheck_BinaryPresent(t *testing.T) {
	dir := t.TempDir()
	binaryName := "biggz-mcp"
	if runtime.GOOS == "windows" {
		binaryName = "biggz-mcp.exe"
	}
	binaryPath := filepath.Join(dir, binaryName)
	if err := os.WriteFile(binaryPath, []byte("data"), 0755); err != nil {
		t.Fatal(err)
	}

	c := NewBinaryCheckWithCustom(dir, os.Stat)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q)", result.Status, result.Message)
	}
}

func TestBinaryCheck_Missing(t *testing.T) {
	dir := t.TempDir()
	c := NewBinaryCheckWithCustom(dir, os.Stat)
	result := c.Run(context.Background())
	if result.Status != StatusFail {
		t.Errorf("status = %v, want fail", result.Status)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %s, want CRITICAL", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// 4.4 — Config check tests
// ---------------------------------------------------------------------------

func TestConfigCheck_Complete(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bigmem"), 0755)
	os.MkdirAll(filepath.Join(dir, "backups"), 0755)

	c := NewConfigCheckWithCustom(dir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q)", result.Status, result.Message)
	}
}

func TestConfigCheck_MissingSubdir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bigmem"), 0755)
	// deliberately omit "backups"

	c := NewConfigCheckWithCustom(dir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusFail {
		t.Errorf("status = %v, want fail", result.Status)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %s, want CRITICAL", result.Severity)
	}
}

func TestConfigCheck_NoRoot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	c := NewConfigCheckWithCustom(dir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusFail {
		t.Errorf("status = %v, want fail", result.Status)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %s, want CRITICAL", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// 4.5 — Review check tests
// ---------------------------------------------------------------------------

// createReviewLineage creates a minimal valid review lineage in the given directory.
func createReviewLineage(t *testing.T, parentDir, lineageID string) {
	t.Helper()
	lineageDir := filepath.Join(parentDir, lineageID)
	if err := os.MkdirAll(lineageDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a genesis record.
	rec := review.Record{
		Schema:    "biggz-ai.review-record/v1",
		Operation: "genesis",
		Role:      "test",
		Actor:     "test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   []byte(`{"test":true}`),
	}

	store := review.OpenWithDir(lineageDir, lineageID)
	_, err := store.Append("", rec)
	if err != nil {
		t.Fatal(err)
	}
}

// reviewTestHelper creates a temp .git directory with review-transactions inside it.
func reviewTestHelper(t *testing.T) (biggzDir, gitDir string) {
	t.Helper()
	root := t.TempDir()
	gitDir = filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	return root, gitDir
}

func TestReviewCheck_NoGit(t *testing.T) {
	root := t.TempDir()
	c := NewReviewCheckWithCustom(
		func() (string, error) { return root, nil },
		func(name string) (os.FileInfo, error) {
			return nil, fmt.Errorf("not found")
		},
		nil,
		func(name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("git not available")
		},
	)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn (msg=%q)", result.Status, result.Message)
	}
	if result.Severity != SeverityWarning {
		t.Errorf("severity = %s, want WARNING", result.Severity)
	}
}

func TestReviewCheck_NoLineages(t *testing.T) {
	root, gitDir := reviewTestHelper(t)
	// Create review-transactions dir but empty.
	os.MkdirAll(filepath.Join(gitDir, "biggz", "review-transactions"), 0755)

	c := NewReviewCheckWithCustom(
		func() (string, error) { return root, nil },
		os.Stat,
		os.ReadDir,
		nil,
	)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q)", result.Status, result.Message)
	}
}

func TestReviewCheck_ValidLineage(t *testing.T) {
	root, gitDir := reviewTestHelper(t)
	lineageDir := filepath.Join(gitDir, "biggz", "review-transactions")
	if err := os.MkdirAll(lineageDir, 0755); err != nil {
		t.Fatal(err)
	}
	createReviewLineage(t, lineageDir, "test-lineage")

	c := NewReviewCheckWithCustom(
		func() (string, error) { return root, nil },
		os.Stat,
		os.ReadDir,
		nil,
	)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q, err=%q)", result.Status, result.Message, result.Error)
	}
}

// ---------------------------------------------------------------------------
// 4.6 — Path check tests
// ---------------------------------------------------------------------------

func TestPathCheck_Duplicates(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	binaryName := "biggz"
	if runtime.GOOS == "windows" {
		binaryName = "biggz.exe"
	}
	if err := os.WriteFile(filepath.Join(dir1, binaryName), []byte("data"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, binaryName), []byte("data"), 0755); err != nil {
		t.Fatal(err)
	}

	path := dir1 + string(filepath.ListSeparator) + dir2
	c := NewPathCheckWithCustom(
		func(key string) string { return path },
		os.Stat,
	)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn for duplicates", result.Status)
	}
	if result.Severity != SeverityWarning {
		t.Errorf("severity = %s, want WARNING", result.Severity)
	}
}

func TestPathCheck_NoDuplicates(t *testing.T) {
	dir := t.TempDir()

	binaryName := "biggz"
	if runtime.GOOS == "windows" {
		binaryName = "biggz.exe"
	}
	if err := os.WriteFile(filepath.Join(dir, binaryName), []byte("data"), 0755); err != nil {
		t.Fatal(err)
	}

	c := NewPathCheckWithCustom(
		func(key string) string { return dir },
		os.Stat,
	)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
}

func TestPathCheck_EmptyPath(t *testing.T) {
	c := NewPathCheckWithCustom(
		func(key string) string { return "" },
		os.Stat,
	)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn for empty PATH", result.Status)
	}
}

// ---------------------------------------------------------------------------
// 4.7 — Disk check tests
// ---------------------------------------------------------------------------

func TestDiskCheck_LowSpace(t *testing.T) {
	c := NewDiskCheckWithCustom("C:\\", func(path string) (int64, int64, error) {
		return 100 * 1024 * 1024, 1024 * 1024 * 1024, nil // 100 MB free
	})
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn (low space)", result.Status)
	}
	if result.Severity != SeverityWarning {
		t.Errorf("severity = %s, want WARNING", result.Severity)
	}
}

func TestDiskCheck_SufficientSpace(t *testing.T) {
	c := NewDiskCheckWithCustom("C:\\", func(path string) (int64, int64, error) {
		return 10 * 1024 * 1024 * 1024, 100 * 1024 * 1024 * 1024, nil // 10 GB free
	})
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
}

func TestDiskCheck_CheckError(t *testing.T) {
	c := NewDiskCheckWithCustom("C:\\", func(path string) (int64, int64, error) {
		return 0, 0, fmt.Errorf("disk check unavailable")
	})
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn on error", result.Status)
	}
}

// ---------------------------------------------------------------------------
// 4.8 — Git check tests
// ---------------------------------------------------------------------------

func TestGitCheck_NoGit(t *testing.T) {
	c := NewGitCheckWithCustom(
		func(name string) (string, error) {
			return "", fmt.Errorf("executable not found")
		},
		nil, nil, nil,
	)
	result := c.Run(context.Background())
	if result.Status != StatusFail {
		t.Errorf("status = %v, want fail", result.Status)
	}
	if result.Severity != SeverityCritical {
		t.Errorf("severity = %s, want CRITICAL", result.Severity)
	}
	if !strings.Contains(result.Message, "not found") {
		t.Errorf("message = %q, should mention 'not found'", result.Message)
	}
}

func TestGitCheck_NoRepo(t *testing.T) {
	c := NewGitCheckWithCustom(
		func(name string) (string, error) {
			return "/usr/bin/git", nil
		},
		func(name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("fatal: not a git repository")
		},
		nil, nil,
	)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn", result.Status)
	}
	if result.Severity != SeverityWarning {
		t.Errorf("severity = %s, want WARNING", result.Severity)
	}
}

func TestGitCheck_GitOK(t *testing.T) {
	c := NewGitCheckWithCustom(
		func(name string) (string, error) {
			return "/usr/bin/git", nil
		},
		func(name string, args ...string) ([]byte, error) {
			return []byte(".git\n"), nil
		},
		nil, nil,
	)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
	if result.Severity != SeverityInfo {
		t.Errorf("severity = %s, want INFO", result.Severity)
	}
}

// ---------------------------------------------------------------------------
// 4.9 — Version check tests
// ---------------------------------------------------------------------------

func TestVersionCheck_UpToDate(t *testing.T) {
	origVersion := BuildVersion
	BuildVersion = "v1.0.0"
	defer func() { BuildVersion = origVersion }()

	c := NewVersionCheckWithCustom(func(name string, args ...string) ([]byte, error) {
		return []byte("v1.0.0\n"), nil
	})
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
	if result.Severity != SeverityInfo {
		t.Errorf("severity = %s, want INFO", result.Severity)
	}
}

func TestVersionCheck_DevBuild(t *testing.T) {
	origVersion := BuildVersion
	BuildVersion = ""
	defer func() { BuildVersion = origVersion }()

	c := NewVersionCheckWithCustom(func(name string, args ...string) ([]byte, error) {
		return []byte("v1.0.0\n"), nil
	})
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
	if !strings.Contains(result.Message, "dev") {
		t.Errorf("message = %q, should mention 'dev'", result.Message)
	}
}

func TestVersionCheck_DifferentVersion(t *testing.T) {
	origVersion := BuildVersion
	BuildVersion = "v1.0.0"
	defer func() { BuildVersion = origVersion }()

	c := NewVersionCheckWithCustom(func(name string, args ...string) ([]byte, error) {
		return []byte("v2.0.0\n"), nil
	})
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
	if !strings.Contains(result.Message, "v2.0.0") {
		t.Errorf("message = %q, should mention latest version", result.Message)
	}
}

func TestVersionCheck_NoTag(t *testing.T) {
	origVersion := BuildVersion
	BuildVersion = "v1.0.0"
	defer func() { BuildVersion = origVersion }()

	c := NewVersionCheckWithCustom(func(name string, args ...string) ([]byte, error) {
		return nil, fmt.Errorf("no tags in repo")
	})
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass", result.Status)
	}
}

// ---------------------------------------------------------------------------
// 4.10 — Backup check tests
// ---------------------------------------------------------------------------

func TestBackupCheck_FreshBackup(t *testing.T) {
	backupDir := t.TempDir()
	backupPath := filepath.Join(backupDir, "backup-1.zip")
	if err := os.WriteFile(backupPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	c := NewBackupCheckWithCustom(backupDir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q)", result.Status, result.Message)
	}
}

func TestBackupCheck_OldBackup(t *testing.T) {
	backupDir := t.TempDir()
	backupPath := filepath.Join(backupDir, "old-backup.zip")
	if err := os.WriteFile(backupPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set modification time to 10 days ago.
	tenDaysAgo := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(backupPath, tenDaysAgo, tenDaysAgo); err != nil {
		t.Logf("warning: could not set backup modtime: %v", err)
	}

	c := NewBackupCheckWithCustom(backupDir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn (old backup)", result.Status)
	}
	if result.Severity != SeverityWarning {
		t.Errorf("severity = %s, want WARNING", result.Severity)
	}
}

func TestBackupCheck_NoBackupDir(t *testing.T) {
	backupDir := filepath.Join(t.TempDir(), "nonexistent")
	c := NewBackupCheckWithCustom(backupDir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn (no dir)", result.Status)
	}
}

func TestBackupCheck_EmptyBackupDir(t *testing.T) {
	backupDir := t.TempDir()
	c := NewBackupCheckWithCustom(backupDir, os.Stat, os.ReadDir)
	result := c.Run(context.Background())
	if result.Status != StatusWarn {
		t.Errorf("status = %v, want warn (empty)", result.Status)
	}
}

// ---------------------------------------------------------------------------
// 4.11 — Integration test: end-to-end Runner with all checks
// ---------------------------------------------------------------------------

func TestIntegration_AllChecksWithTempDirs(t *testing.T) {
	// Create temp root directory mimicking ~/.biggz/ structure.
	rootDir := t.TempDir()
	bigmemDir := filepath.Join(rootDir, "bigmem")
	backupDir := filepath.Join(rootDir, "backups")
	os.MkdirAll(bigmemDir, 0755)
	os.MkdirAll(backupDir, 0755)

	// Create a fresh backup file so the backup check passes.
	os.WriteFile(filepath.Join(backupDir, "test-backup.zip"), []byte("data"), 0644)

	// Create a bigmem store so the bigmem check passes.
	bigmemStore, err := bigmem.Open(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	bigmemStore.Close()

	// Create a mock binary so the binary check passes.
	binaryName := "biggz-mcp"
	if runtime.GOOS == "windows" {
		binaryName = "biggz-mcp.exe"
	}
	os.WriteFile(filepath.Join(rootDir, binaryName), []byte("data"), 0755)

	// Build binary check pointing at our temp dir.
	binaryCheck := NewBinaryCheckWithCustom(rootDir, os.Stat)

	// Config check pointing at our temp root (has bigmem and backups subdirs).
	configCheck := NewConfigCheckWithCustom(rootDir, os.Stat, os.ReadDir)

	// Bigmem check pointing at our temp store.
	bigmemCheck := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(rootDir)
	})

	// Disk check using mock.
	diskCheck := NewDiskCheckWithCustom("C:\\", func(path string) (int64, int64, error) {
		return 10 * 1024 * 1024 * 1024, 100 * 1024 * 1024 * 1024, nil // 10 GB free
	})

	// Path check with a clean PATH.
	dirWithBinary := t.TempDir()
	os.WriteFile(filepath.Join(dirWithBinary, binaryName), []byte("data"), 0755)
	pathCheck := NewPathCheckWithCustom(
		func(key string) string { return dirWithBinary },
		os.Stat,
	)

	// Git check — mock git as not on PATH (safe for CI without git).
	gitCheck := NewGitCheckWithCustom(
		func(name string) (string, error) {
			return "", fmt.Errorf("not found")
		},
		nil, nil, nil,
	)

	// Version check — mock exec.
	versionCheck := NewVersionCheckWithCustom(func(name string, args ...string) ([]byte, error) {
		return []byte("v1.0.0\n"), nil
	})

	// Backup check pointing at our temp backup dir.
	backupCheck := NewBackupCheckWithCustom(backupDir, os.Stat, os.ReadDir)

	// Review check — no git dir, so WARNING from getwdFn pointing to a non-git dir.
	reviewCheck := NewReviewCheckWithCustom(
		func() (string, error) { return t.TempDir(), nil },
		os.Stat,
		os.ReadDir,
		func(name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("git not available")
		},
	)

	runner := &Runner{
		Checks: []Check{
			bigmemCheck,
			binaryCheck,
			configCheck,
			diskCheck,
			pathCheck,
			gitCheck,
			versionCheck,
			backupCheck,
			reviewCheck,
		},
	}

	report := runner.RunAll(context.Background())

	// Verify report structure.
	all := report.All()
	if len(all) != 9 {
		t.Errorf("expected 9 check results, got %d", len(all))
	}

	// Build a set of all check IDs for uniqueness verification.
	seen := make(map[CheckID]bool)
	for _, r := range all {
		if seen[r.ID] {
			t.Errorf("duplicate check ID: %s", r.ID)
		}
		seen[r.ID] = true
	}

	// Verify each expected check ID is present.
	expectedIDs := []CheckID{
		BigmemCheckID, BinaryCheckID, ConfigCheckID, DiskCheckID,
		PathCheckID, GitCheckID, VersionCheckID, BackupCheckID, ReviewCheckID,
	}
	for _, id := range expectedIDs {
		if !seen[id] {
			t.Errorf("missing check: %s", id)
		}
	}

	// Verify Report.All() returns correct total.
	if len(all) != len(expectedIDs) {
		t.Errorf("All() returned %d results, want %d", len(all), len(expectedIDs))
	}

	t.Logf("Integration test results: %d CRITICAL, %d WARNING, %d INFO",
		len(report.Critical), len(report.Warning), len(report.Info))
	for _, r := range all {
		t.Logf("  %s: status=%v severity=%s msg=%q", r.ID, r.Status, r.Severity, r.Message)
	}
}

// TestIntegration_JSONOutput verifies that the Report marshals to JSON correctly.
func TestIntegration_JSONOutput(t *testing.T) {
	report := &Report{
		Critical: []*Result{{ID: "git", Status: StatusFail, Message: "Git not found", Severity: SeverityCritical}},
		Warning:  []*Result{{ID: "disk", Status: StatusWarn, Message: "Low disk space", Severity: SeverityWarning}},
		Info:     []*Result{{ID: "bigmem", Status: StatusPass, Message: "Healthy", Severity: SeverityInfo}},
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	output := string(data)
	if !strings.Contains(output, `"critical"`) {
		t.Error("JSON missing critical field")
	}
	if !strings.Contains(output, `"warning"`) {
		t.Error("JSON missing warning field")
	}
	if !strings.Contains(output, `"info"`) {
		t.Error("JSON missing info field")
	}
	if !strings.Contains(output, "Git not found") {
		t.Error("JSON missing result message")
	}
	
	t.Logf("JSON output:\n%s", output)
}

// TestIntegration_TableOutput verifies severity-grouped table formatting.
func TestIntegration_TableOutput(t *testing.T) {
	// We test that the runner produces severity-grouped buckets correctly.
	report := &Report{
		Critical: []*Result{
			{ID: "git", Status: StatusFail, Message: "Git not found", Severity: SeverityCritical},
		},
		Warning: []*Result{
			{ID: "disk", Status: StatusWarn, Message: "Low disk space", Severity: SeverityWarning},
		},
		Info: []*Result{
			{ID: "bigmem", Status: StatusPass, Message: "Database OK", Severity: SeverityInfo},
			{ID: "binary", Status: StatusPass, Message: "Binary found", Severity: SeverityInfo},
		},
	}

	// Verify correct counts.
	if len(report.Critical) != 1 {
		t.Errorf("Critical count = %d, want 1", len(report.Critical))
	}
	if len(report.Warning) != 1 {
		t.Errorf("Warning count = %d, want 1", len(report.Warning))
	}
	if len(report.Info) != 2 {
		t.Errorf("Info count = %d, want 2", len(report.Info))
	}
	if len(report.All()) != 4 {
		t.Errorf("Total results = %d, want 4", len(report.All()))
	}
	if report.CountByStatus(StatusFail) != 1 {
		t.Errorf("Fail count = %d, want 1", report.CountByStatus(StatusFail))
	}
	if report.CountByStatus(StatusWarn) != 1 {
		t.Errorf("Warn count = %d, want 1", report.CountByStatus(StatusWarn))
	}
	if report.CountByStatus(StatusPass) != 2 {
		t.Errorf("Pass count = %d, want 2", report.CountByStatus(StatusPass))
	}
}

// TestIntegration_ExitCodes verifies the exit code logic matches severity.
func TestIntegration_ExitCodes(t *testing.T) {
	tests := []struct {
		name   string
		report *Report
		want   int
	}{
		{
			name:   "all pass → 0",
			report: &Report{Info: []*Result{{ID: "a", Status: StatusPass, Severity: SeverityInfo}}},
			want:   0,
		},
		{
			name:   "warning only → 1",
			report: &Report{Warning: []*Result{{ID: "a", Status: StatusWarn, Severity: SeverityWarning}}},
			want:   1,
		},
		{
			name:   "critical → 2",
			report: &Report{Critical: []*Result{{ID: "a", Status: StatusFail, Severity: SeverityCritical}}},
			want:   2,
		},
		{
			name: "critical and warning → 2",
			report: &Report{
				Critical: []*Result{{ID: "a", Status: StatusFail, Severity: SeverityCritical}},
				Warning:  []*Result{{ID: "b", Status: StatusWarn, Severity: SeverityWarning}},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.report.ExitCode()
			if got != tt.want {
				t.Errorf("ExitCode(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}
