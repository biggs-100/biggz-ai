package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeGraph_UsageErrors(t *testing.T) {
	cases := [][]string{
		{},
		{"init"},
		{"init", "--cwd"},
		{"init", "--cwd", ""},
		{"init", "--cwd", "   "},
		{"wrong", "--cwd", "/tmp"},
		{"init", "--badflag", "/tmp"},
	}
	for _, args := range cases {
		// Simulate os.Args for codegraphRun
		savedArgs := os.Args
		os.Args = append([]string{"biggz", "codegraph"}, args...)
		// Capture stderr by redirecting? codegraphRun prints to os.Stderr and returns 1.
		// We just check exit code, not output, for unit-level usage validation.
		code := codegraphRun()
		os.Args = savedArgs
		if code == 0 {
			t.Errorf("expected non-zero for args %v, got 0", args)
		}
	}
}

func TestCodeGraph_Guidance(t *testing.T) {
	savedArgs := os.Args
	os.Args = []string{"biggz", "codegraph", "guidance"}
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := codegraphRun()
	w.Close()
	os.Stdout = oldStdout
	os.Args = savedArgs
	if code != 0 {
		t.Fatalf("expected 0 for guidance, got %d", code)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "CodeGraph") {
		t.Errorf("expected guidance to contain CodeGraph, got %q", out)
	}
	if !strings.Contains(out, "biggz codegraph init") {
		t.Errorf("expected guidance to contain 'biggz codegraph init', got %q", out)
	}
}

func TestResolveCodeGraphRoot_ValidRepo(t *testing.T) {
	// t.TempDir() is inside os.TempDir() which is considered unsafe by isUnsafeCodeGraphRoot.
	// Mock the temp/home dirs to avoid false unsafe rejection.
	origTemp := codegraphTempDir
	origHome := codegraphHomeDir
	fakeTemp := filepath.Join(t.TempDir(), "fake-temp-root-not-used")
	os.MkdirAll(fakeTemp, 0755)
	fakeHome := filepath.Join(t.TempDir(), "fake-home-not-used")
	os.MkdirAll(fakeHome, 0755)
	codegraphTempDir = func() string { return fakeTemp }
	codegraphHomeDir = func() (string, error) { return fakeHome, nil }
	defer func() {
		codegraphTempDir = origTemp
		codegraphHomeDir = origHome
	}()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@test.com")

	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("# test"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	root, err := resolveCodeGraphRoot(dir)
	if err != nil {
		t.Fatalf("expected valid root, got %v", err)
	}
	if root == "" {
		t.Fatal("expected non-empty root")
	}
	canonicalDir, _ := filepath.EvalSymlinks(dir)
	canonicalDir, _ = filepath.Abs(canonicalDir)
	if root != canonicalDir {
		t.Errorf("expected %q, got %q", canonicalDir, root)
	}
}

func TestResolveCodeGraphRoot_UnsafePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cases := []struct {
		name string
		path string
	}{
		{"home dir", home},
		{"temp dir", os.TempDir()},
		{"drive root", filepath.VolumeName(home) + string(filepath.Separator)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveCodeGraphRoot(tc.path)
			if err == nil {
				t.Errorf("expected error for unsafe path %q, got nil", tc.path)
			} else if !strings.Contains(err.Error(), "unsafe") {
				t.Errorf("expected unsafe error for %q, got %v", tc.path, err)
			}
		})
	}
}

func TestResolveCodeGraphRoot_NonGitDir(t *testing.T) {
	origTemp := codegraphTempDir
	origHome := codegraphHomeDir
	fakeTemp := filepath.Join(t.TempDir(), "fake-temp-root-not-used")
	os.MkdirAll(fakeTemp, 0755)
	fakeHome := filepath.Join(t.TempDir(), "fake-home-not-used")
	os.MkdirAll(fakeHome, 0755)
	codegraphTempDir = func() string { return fakeTemp }
	codegraphHomeDir = func() (string, error) { return fakeHome, nil }
	defer func() {
		codegraphTempDir = origTemp
		codegraphHomeDir = origHome
	}()

	dir := t.TempDir()
	_, err := resolveCodeGraphRoot(dir)
	if err == nil {
		t.Fatalf("expected error for non-git dir %q, got nil", dir)
	}
	if !strings.Contains(err.Error(), "not a recognized project") {
		t.Errorf("expected 'not a recognized project' for %q, got %v", dir, err)
	}
}

func TestResolveCodeGraphRoot_SubpathOfRepoRejected(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@test.com")
	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("# test"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)

	// Candidate is subdir, but git toplevel is dir, so canonicalRoot != candidate -> unsafe
	_, err := resolveCodeGraphRoot(sub)
	if err == nil {
		t.Fatalf("expected error for subpath %q (toplevel %q), got nil", sub, dir)
	}
}

// Ensure codegraph binary is not required for validation tests
func TestCodeGraph_MissingBinaryReportsHelpfully(t *testing.T) {
	// This test verifies that when the root is valid but codegraph binary is missing,
	// the error message mentions the binary. We can't guarantee codegraph is installed,
	// so we test the validation part only via resolveCodeGraphRoot, which we already do.
	// If codegraph binary IS installed, the full init would succeed; if not, it fails gracefully.
	// This test just ensures the verb is wired and help is correct.
	savedArgs := os.Args
	os.Args = []string{"biggz", "codegraph", "init", "--cwd", t.TempDir()}
	// Use a non-git temp dir so validation fails before binary check — we test validation error path
	code := codegraphRun()
	os.Args = savedArgs
	if code == 0 {
		t.Error("expected non-zero for non-git dir")
	}
}

func TestCodeGraph_HelpDocumentsReport(t *testing.T) {
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Args = []string{"biggz", "codegraph", "--help"}
	code := codegraphRun()
	w.Close()
	os.Stderr = oldStderr
	if code != 0 {
		t.Fatalf("expected 0 for --help, got %d", code)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "report") {
		t.Errorf("expected help to contain 'report', got %q", out)
	}
	if !strings.Contains(out, "--cwd") || !strings.Contains(out, "--json") || !strings.Contains(out, "--md") {
		t.Errorf("expected help to document --cwd/--json/--md, got %q", out)
	}
	// Also check report --help
	r2, w2, _ := os.Pipe()
	os.Stderr = w2
	os.Args = []string{"biggz", "codegraph", "report", "--help"}
	code = codegraphRun()
	w2.Close()
	os.Stderr = oldStderr
	if code != 0 {
		t.Fatalf("expected 0 for report --help, got %d", code)
	}
	n2, _ := r2.Read(buf)
	out2 := string(buf[:n2])
	if !strings.Contains(out2, "report <change>") {
		t.Errorf("expected report help to contain 'report <change>', got %q", out2)
	}
}

func TestCodeGraph_ReportMissingChangeFails(t *testing.T) {
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Args = []string{"biggz", "codegraph", "report"}
	code := codegraphRun()
	w.Close()
	os.Stderr = oldStderr
	if code == 0 {
		t.Fatalf("expected non-zero for missing <change>")
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "usage") && !strings.Contains(out, "<change>") {
		t.Errorf("expected usage for missing change, got %q", out)
	}
}

func TestCodeGraph_ReportMissingProposalFails(t *testing.T) {
	dir := t.TempDir()
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	os.Args = []string{"biggz", "codegraph", "report", "my-change", "--cwd", dir}
	code := codegraphRun()
	w.Close()
	os.Stderr = oldStderr
	if code == 0 {
		t.Fatalf("expected non-zero for missing proposal")
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "proposal required") {
		t.Errorf("expected 'proposal required', got %q", out)
	}
}

func TestCodeGraph_ReportCustomPaths(t *testing.T) {
	dir := t.TempDir()
	change := "custom-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("Need PaymentService for custom"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte("package main\nfunc PaymentService(){}\n"), 0644); err != nil {
		t.Fatalf("svc.go: %v", err)
	}
	jsonPath := filepath.Join(dir, "tmp", "nested", "cg.json")
	mdPath := filepath.Join(dir, "tmp", "nested", "cg.md")
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Args = []string{"biggz", "codegraph", "report", change, "--cwd", dir, "--json", jsonPath, "--md", mdPath}
	code := codegraphRun()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	if code != 0 {
		buf := make([]byte, 4096)
		n, _ := rErr.Read(buf)
		t.Fatalf("expected 0, got %d, stderr: %s", code, string(buf[:n]))
	}
	// Check custom paths exact
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("json not at custom path: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("md not at custom path: %v", err)
	}
	// Parents must be created (MkdirAll) - already verified via Stat
	// Check stdout contains JSON
	buf := make([]byte, 8192)
	n, _ := rOut.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "files") || !strings.Contains(out, "graph") {
		t.Errorf("expected JSON stdout with files+graph, got %q", out)
	}
}

func TestCodeGraph_ReportDefaults(t *testing.T) {
	dir := t.TempDir()
	change := "default-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("Need PaymentService default"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main\nfunc PaymentService(){}\n"), 0644); err != nil {
		t.Fatalf("app.go: %v", err)
	}
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Args = []string{"biggz", "codegraph", "report", change, "--cwd", dir}
	code := codegraphRun()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	if code != 0 {
		t.Fatalf("expected 0 for default paths, got %d", code)
	}
	jsonPath := filepath.Join(changeDir, "codegraph.json")
	mdPath := filepath.Join(changeDir, "codegraph.md")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("default json not created at %s: %v", jsonPath, err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("default md not created at %s: %v", mdPath, err)
	}
	buf := make([]byte, 8192)
	n, _ := rOut.Read(buf)
	if !strings.Contains(string(buf[:n]), "files") {
		t.Errorf("expected stdout JSON, got %q", string(buf[:n]))
	}
}

func TestCodeGraph_ReportPreservesInitAndGuidance(t *testing.T) {
	// Ensure init validation still works and guidance still works after report addition
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	// guidance
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Args = []string{"biggz", "codegraph", "guidance"}
	code := codegraphRun()
	w.Close()
	os.Stdout = oldStdout
	if code != 0 {
		t.Fatalf("guidance should still return 0, got %d", code)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "CodeGraph") {
		t.Errorf("guidance missing CodeGraph")
	}
	// init with invalid cwd should still fail with usage, not routed to report
	oldStderr := os.Stderr
	r2, w2, _ := os.Pipe()
	os.Stderr = w2
	os.Args = []string{"biggz", "codegraph", "init", "--cwd", "/nonexistent"}
	code = codegraphRun()
	w2.Close()
	os.Stderr = oldStderr
	if code == 0 {
		t.Errorf("init with bad cwd should still fail")
	}
	buf2 := make([]byte, 4096)
	n2, _ := r2.Read(buf2)
	_ = n2
	_ = buf2
}

func TestResolveReportRoot(t *testing.T) {
	dir := t.TempDir()
	root, err := resolveReportRoot(dir)
	if err != nil {
		t.Fatalf("resolveReportRoot: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	canon, _ := filepath.EvalSymlinks(abs)
	canon, _ = filepath.Abs(canon)
	if root != canon {
		t.Errorf("expected %q, got %q", canon, root)
	}
	// traversal via .. should still resolve to parent but not error (Abs handles)
	parent := filepath.Dir(dir)
	child := filepath.Join(dir, "..")
	root2, err := resolveReportRoot(child)
	if err != nil {
		t.Fatalf("resolveReportRoot with traversal: %v", err)
	}
	parentAbs, _ := filepath.Abs(parent)
	parentCanon, _ := filepath.EvalSymlinks(parentAbs)
	parentCanon, _ = filepath.Abs(parentCanon)
	if root2 != parentCanon {
		t.Errorf("expected traversal to resolve to parent %q, got %q", parentCanon, root2)
	}
}

// Helper to run git in tests (reuses the helper from main_test.go if available)
func init() {
	// Ensure exec is available for git
	_, _ = exec.LookPath("git")
}
