package sdd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeGateFixture(t *testing.T, dir, rel string, branches int) {
	t.Helper()
	var b strings.Builder
	b.WriteString("package foo\n\nfunc Big(x int) int {\n\ty := 0\n")
	for i := 0; i < branches; i++ {
		b.WriteString("if x == " + itoaGate(i) + " { y++ }\n")
	}
	b.WriteString("return y\n}\n")
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
}

func itoaGate(i int) string {
	if i == 0 {
		return "0"
	}
	var d []byte
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	return string(d)
}

func gateDiffFor(rel string, start, count int) string {
	return "diff --git a/" + rel + " b/" + rel + "\n@@ -" +
		itoaGate(start) + ",3 +" + itoaGate(start) + "," + itoaGate(count) + " @@\n+// touched\n"
}

func TestGateDiffComplexity_BlocksNewCritical(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/big.go", 20) // cyclo ~21 > 15
	res := GateDiffComplexity(dir, gateDiffFor("internal/sdd/big.go", 1, 25))
	if res.Passed {
		t.Fatal("expected gate to block new critical offender")
	}
	if len(res.Blocking) != 1 || res.Blocking[0].Function != "Big" {
		t.Fatalf("expected Big blocking, got %+v", res.Blocking)
	}
	if res.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1", res.FilesScanned)
	}
}

func TestGateDiffComplexity_GrandfathersOutsideDiff(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/big.go", 20)
	// Pre-existing offender untouched by this diff must not block.
	res := GateDiffComplexity(dir, gateDiffFor("internal/sdd/big.go", 500, 4))
	if !res.Passed {
		t.Fatalf("expected pass (grandfathered), blocking: %+v", res.Blocking)
	}
}

func TestGateDiffComplexity_NonCriticalPassesSilent(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/foo/big.go", 20)
	res := GateDiffComplexity(dir, gateDiffFor("internal/foo/big.go", 1, 25))
	if !res.Passed {
		t.Fatalf("expected pass outside critical packages, blocking: %+v", res.Blocking)
	}
	if len(res.Blocking) != 0 {
		t.Errorf("expected no blockers, got %+v", res.Blocking)
	}
}

func TestGateDiffComplexity_TestFileWarns(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/big_test.go", 20)
	res := GateDiffComplexity(dir, gateDiffFor("internal/sdd/big_test.go", 1, 25))
	if !res.Passed {
		t.Fatalf("expected pass with warning for test file, blocking: %+v", res.Blocking)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected test-file warning")
	}
}

func TestGateDiffComplexity_EmptyDiffPasses(t *testing.T) {
	res := GateDiffComplexity(t.TempDir(), "")
	if !res.Passed || res.FilesScanned != 0 {
		t.Errorf("expected clean pass, got %+v", res)
	}
}

func TestFormatGateBlockers(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/big.go", 20)
	res := GateDiffComplexity(dir, gateDiffFor("internal/sdd/big.go", 1, 25))
	s := FormatGateBlockers(res.Blocking)
	if !strings.Contains(s, "internal/sdd/big.go") || !strings.Contains(s, "Big") || !strings.Contains(s, "cyclo=") {
		t.Errorf("blocker rendering missing parts: %q", s)
	}
}

func initGateRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init")
	run("add", "-A")
	run("commit", "-m", "base", "--no-gpg-sign")
}

func TestGateWorkingTreeComplexity_BlocksUncommitted(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/small.go", 2)
	initGateRepo(t, dir)
	writeGateFixture(t, dir, "internal/sdd/big.go", 20) // uncommitted offender
	res, err := GateWorkingTreeComplexity(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatal("expected gate to block uncommitted offender")
	}
}

func TestGateWorkingTreeComplexity_CleanPasses(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/small.go", 2)
	initGateRepo(t, dir)
	res, err := GateWorkingTreeComplexity(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass on clean tree, blocking: %+v", res.Blocking)
	}
}

func TestGateWorkingTreeComplexity_UntrackedMeasured(t *testing.T) {
	dir := t.TempDir()
	writeGateFixture(t, dir, "internal/sdd/small.go", 2)
	initGateRepo(t, dir)
	writeGateFixture(t, dir, "internal/sdd/newfile.go", 20) // untracked offender
	res, err := GateWorkingTreeComplexity(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Fatal("expected gate to block untracked offender")
	}
}

func TestGateWorkingTreeComplexity_NonRepoErrors(t *testing.T) {
	if _, err := GateWorkingTreeComplexity(t.TempDir()); err == nil {
		t.Fatal("expected error outside a git repo")
	}
}
