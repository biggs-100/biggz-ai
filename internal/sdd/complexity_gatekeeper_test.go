package sdd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gatekeeperGitRepo(t *testing.T) (repoRoot, openspecRoot string) {
	t.Helper()
	repoRoot = t.TempDir()
	openspecRoot = filepath.Join(repoRoot, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init")
	run("add", "-A")
	run("commit", "-m", "base", "--no-gpg-sign")
	return repoRoot, openspecRoot
}

func gatekeeperVerifyResult() *PhaseResult {
	return &PhaseResult{
		Status:           "success",
		ExecutiveSummary: "verify done",
		Artifacts:        []ArtifactRef{{Path: "verify-report.md", Type: "verify"}},
		NextRecommended:  "archive",
	}
}

func findCheck(gk *GatekeeperResult, name string) *GatekeeperCheck {
	for i := range gk.Details {
		if gk.Details[i].Name == name {
			return &gk.Details[i]
		}
	}
	return nil
}

func TestGatekeeper_ComplexityBlocksVerify(t *testing.T) {
	repoRoot, openspecRoot := gatekeeperGitRepo(t)
	writeGateFixture(t, repoRoot, "internal/sdd/big.go", 20) // uncommitted offender
	if err := os.WriteFile(filepath.Join(openspecRoot, "changes", "test-change", "verify-report.md"), []byte("# V\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gk := Gatekeeper(openspecRoot, "test-change", "verify", gatekeeperVerifyResult())
	c := findCheck(gk, "complexity_gate")
	if c == nil {
		t.Fatal("expected complexity_gate detail")
	}
	if c.Passed || c.Skipped {
		t.Errorf("expected failing complexity_gate, got %+v", c)
	}
	if gk.Passed {
		t.Error("expected overall gatekeeper FAIL with new critical offender")
	}
}

func TestGatekeeper_ComplexityPassesCleanVerify(t *testing.T) {
	_, openspecRoot := gatekeeperGitRepo(t) // clean tree
	if err := os.WriteFile(filepath.Join(openspecRoot, "changes", "test-change", "verify-report.md"), []byte("# V\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gk := Gatekeeper(openspecRoot, "test-change", "verify", gatekeeperVerifyResult())
	c := findCheck(gk, "complexity_gate")
	if c == nil {
		t.Fatal("expected complexity_gate detail")
	}
	if !c.Passed || c.Skipped {
		t.Errorf("expected passing complexity_gate, got %+v", c)
	}
}

func TestGatekeeper_ComplexitySkipsNonVerify(t *testing.T) {
	tmpDir := t.TempDir()
	openspecRoot := filepath.Join(tmpDir, "openspec")
	gk := Gatekeeper(openspecRoot, "test-change", "spec", &PhaseResult{
		Status: "success", ExecutiveSummary: "s",
		Artifacts:       []ArtifactRef{{Path: "spec.md"}},
		NextRecommended: "design",
	})
	c := findCheck(gk, "complexity_gate")
	if c == nil {
		t.Fatal("expected complexity_gate detail")
	}
	if !c.Skipped {
		t.Errorf("expected skip outside verify, got %+v", c)
	}
}

func TestGatekeeper_ComplexitySkipsNonRepo(t *testing.T) {
	tmpDir := t.TempDir() // no .git
	openspecRoot := filepath.Join(tmpDir, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "verify-report.md"), []byte("# V\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gk := Gatekeeper(openspecRoot, "test-change", "verify", gatekeeperVerifyResult())
	c := findCheck(gk, "complexity_gate")
	if c == nil {
		t.Fatal("expected complexity_gate detail")
	}
	if !c.Skipped {
		t.Errorf("expected skip without git repo, got %+v", c)
	}
}
