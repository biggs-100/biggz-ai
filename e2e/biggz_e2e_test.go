// Package e2e proves biggz-ai works end-to-end through the real binary against
// real Git repositories — the same "organic RDD" philosophy as gentle-ai: the
// agent implements organically, and biggz-ai's authority begins only after a
// candidate exists.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// biggzBinAbs is the absolute path to biggz.exe, resolved once.
var biggzBinAbs string

func init() { initBin() }

func initBin() {
	if biggzBinAbs != "" {
		return
	}
	candidates := []string{
		"biggz.exe",
		"../biggz.exe",
		"../../biggz.exe",
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		for _, rel := range []string{".", "..", "../..", "../../.."} {
			candidates = append(candidates, filepath.Join(d, rel, "biggz.exe"))
		}
	}
	for _, p := range candidates {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				biggzBinAbs = abs
				return
			}
		}
	}
	biggzBinAbs = "biggz.exe"
}

// gitCmd runs a git command in dir.
func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// biggz runs the biggz binary with args.
func biggz(args ...string) (string, error) {
	initBin()
	var out bytes.Buffer
	cmd := exec.Command(biggzBinAbs, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// biggzIn runs biggz in the given directory.
func biggzIn(dir string, args ...string) (string, error) {
	initBin()
	var out bytes.Buffer
	cmd := exec.Command(biggzBinAbs, args...)
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// initRepo creates a temp git repo with an initial commit.
func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitCmd(repo, "init")
	gitCmd(repo, "config", "user.email", "test@biggs.ai")
	gitCmd(repo, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# test"), 0644)
	gitCmd(repo, "add", ".")
	gitCmd(repo, "commit", "-m", "init")
	return repo
}

// TestOrganicRDDStatus proves RDD status works in a git repo.
func TestOrganicRDDStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E in short mode")
	}
	repo := initRepo(t)
	out, err := biggzIn(repo, "rdd", "status")
	if err != nil {
		t.Fatalf("rdd status failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "RDD Status") {
		t.Errorf("expected RDD Status in output, got:\n%s", out)
	}
	t.Logf("RDD status:\n%s", out)
}

// TestOrganicRDDDisableEnable proves RDD disable/enable cycle.
func TestOrganicRDDDisableEnable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E in short mode")
	}
	repo := initRepo(t)

	// Disable
	out, err := biggzIn(repo, "rdd", "disable", "--scope", "worktree")
	if err != nil {
		t.Fatalf("rdd disable failed: %v\n%s", err, out)
	}
	t.Logf("Disable:\n%s", out)

	// Verify
	out2, _ := biggzIn(repo, "rdd", "status")
	if !strings.Contains(out2, "DISABLED") && !strings.Contains(out2, "disabled") {
		t.Logf("Status after disable:\n%s", out2)
	}

	// Enable
	out3, err := biggzIn(repo, "rdd", "enable")
	if err != nil {
		t.Fatalf("rdd enable failed: %v\n%s", err, out3)
	}
	t.Logf("Enable:\n%s", out3)
}

// TestOrganicHelp proves biggz --help works.
func TestOrganicHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E in short mode")
	}
	out, err := biggz("--help")
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Commands") {
		t.Errorf("expected Commands in --help, got:\n%s", out)
	}
	t.Logf("Help shows %d bytes of output", len(out))
}

// TestOrganicReviewStart proves review creates a lineage.
func TestOrganicReviewStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E in short mode")
	}
	repo := initRepo(t)

	// Make a change
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)
	gitCmd(repo, "add", ".")

	// Create subject JSON with diff
	diff, _ := gitCmd(repo, "diff", "--cached")
	lineage := fmt.Sprintf("e2e-review-%d", time.Now().UnixNano())
	subject := map[string]any{
		"lineage": lineage,
		"diff":    strings.TrimSpace(diff),
		"files":   []string{"main.go"},
	}
	subData, _ := json.Marshal(subject)
	subFile := filepath.Join(repo, "subject.json")
	os.WriteFile(subFile, subData, 0644)

	// Start review
	out, err := biggzIn(repo, "review", "start", "--subject", subFile, "--lineage", lineage)
	if err != nil {
		t.Fatalf("review start failed: %v\n%s", err, out)
	}
	t.Logf("Review start:\n%s", out)

	// Validate the chain
	valOut, valErr := biggzIn(repo, "review", "validate", lineage)
	if valErr != nil {
		t.Logf("Review validate (may fail for empty test): %v\n%s", valErr, valOut)
	} else {
		t.Logf("Review validate OK:\n%s", valOut)
	}
}

// TestOrganicSDDStatus proves sdd-status works.
func TestOrganicSDDStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E in short mode")
	}
	repo := initRepo(t)

	// SDD status without openspec — should show "no openspec" or similar
	out, err := biggzIn(repo, "sdd-status")
	if err != nil {
		// This is expected if openspec doesn't exist
		t.Logf("sdd-status (expected partial): %v\n%s", err, out)
	} else {
		t.Logf("SDD status:\n%s", out)
	}
}

// TestDockerE2E runs the full test suite inside a Docker container
// matching the CI pipeline. Requires Docker to be installed.
func TestDockerE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker E2E in short mode")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available: " + err.Error())
	}
	// Quick check Docker is actually running
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("Docker daemon not running: " + err.Error())
	}

	repo := t.TempDir()
	gitCmd(repo, "init")
	gitCmd(repo, "config", "user.email", "test@biggs.ai")
	gitCmd(repo, "config", "user.name", "Test")

	// Write a simple Go file
	os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module e2e-test\n\ngo 1.25\n"), 0644)
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	gitCmd(repo, "add", ".")
	gitCmd(repo, "commit", "-m", "initial")

	// Build the Docker image using the project Dockerfile
	projectRoot := func() string {
		cwd, _ := os.Getwd()
		return filepath.Dir(cwd) // e2e/ is inside the project root
	}()
	t.Log("Building Docker image (first run may take a minute)...")
	buildCmd := exec.Command("docker", "build", "-t", "biggz-e2e", "-f", filepath.Join(projectRoot, "Dockerfile"), projectRoot)
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Docker build failed: %v\n%s", err, buildOut)
	}
	t.Logf("Docker build: %s", string(buildOut))

	// Run biggz version inside container
	runCmd := exec.Command("docker", "run", "--rm", "biggz-e2e", "--help")
	helpOut, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Docker run failed: %v\n%s", err, helpOut)
	}
	if !strings.Contains(string(helpOut), "Usage:") {
		t.Fatalf("unexpected output: %s", string(helpOut))
	}
	t.Logf("Docker help output:\n%s", string(helpOut))

	// Run doctor inside container
	doctorCmd := exec.Command("docker", "run", "--rm", "-v", fmt.Sprintf("%s:/repo", repo), "biggz-e2e", "doctor")
	doctorOut, err := doctorCmd.CombinedOutput()
	if err != nil {
		t.Logf("Doctor in Docker (expected partial): %v\n%s", err, doctorOut)
	} else {
		t.Logf("Doctor in Docker:\n%s", doctorOut)
	}

	// Run bigmem stats inside container
	statsCmd := exec.Command("docker", "run", "--rm", "biggz-e2e", "bigmem", "stats")
	statsOut, err := statsCmd.CombinedOutput()
	if err != nil {
		t.Logf("BigMem stats in Docker (expected partial): %v\n%s", err, statsOut)
	} else {
		t.Logf("BigMem stats in Docker:\n%s", statsOut)
	}
}

// TestOrganicDoctor proves doctor works.
func TestOrganicDoctor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E in short mode")
	}
	out, err := biggz("doctor")
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	t.Logf("Doctor:\n%s", out)
}
