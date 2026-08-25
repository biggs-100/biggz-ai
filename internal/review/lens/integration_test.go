package lens

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/pipeline"
)

func fixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	gitRun(t, repo, "init")
	writeFile(t, filepath.Join(repo, "base.txt"), "base\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "base")
	writeFile(t, filepath.Join(repo, "docs", "guide.md"), "line1\nline2\nline3\n")
	writeFile(t, filepath.Join(repo, "internal", "auth", "token.go"), "package auth\nfunc Issue(){}\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "candidate")
	head := gitRun(t, repo, "rev-parse", "HEAD")
	return repo, head
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type integrationStubLens struct{ id string }

func (s *integrationStubLens) ID() string { return s.id }
func (s *integrationStubLens) Analyze(_ context.Context, input LensInput) (LensResult, error) {
	return LensResult{LensID: s.id, Findings: nil, Evidence: []string{"ok for " + s.id}, Truncated: input.Truncated}, nil
}

type failingLens struct{ id string }

func (f *failingLens) ID() string { return f.id }
func (f *failingLens) Analyze(_ context.Context, _ LensInput) (LensResult, error) {
	return LensResult{}, os.ErrInvalid
}

func TestLens_SingleDerivation_NoDuplicateDiff(t *testing.T) {
	repo, head := fixtureRepo(t)
	input, err := review.DeriveRiskInput(repo, head, "")
	if err != nil {
		t.Fatalf("DeriveRiskInput: %v", err)
	}
	hunks := map[string][]byte{
		"pkg/foo.go": []byte("package foo\nfunc Foo(){}\n"),
	}
	lensInput := NewLensInput(input, hunks, false, repo)
	if len(lensInput.Paths) != len(input.Paths) {
		t.Errorf("paths mismatch: got %d, want %d", len(lensInput.Paths), len(input.Paths))
	}
	if lensInput.BaseTree != input.BaseTree {
		t.Error("BaseTree mismatch")
	}
	for _, l := range []Lens{&integrationStubLens{id: "readability"}, &integrationStubLens{id: "reliability"}, &integrationStubLens{id: "resilience"}} {
		result, err := l.Analyze(context.Background(), lensInput)
		if err != nil {
			t.Errorf("lens %s Analyze: %v", l.ID(), err)
		}
		if result.LensID != l.ID() {
			t.Errorf("LensID = %q, want %q", result.LensID, l.ID())
		}
		if result.Truncated != lensInput.Truncated {
			t.Errorf("Truncated not propagated for %s", l.ID())
		}
	}
}

func TestLens_HunkCap_8MiB(t *testing.T) {
	repo, head := fixtureRepo(t)
	input, err := review.DeriveRiskInput(repo, head, "")
	if err != nil {
		t.Fatalf("DeriveRiskInput: %v", err)
	}
	large := make([]byte, HunkCapBytes+1024)
	for i := range large {
		large[i] = 'a'
	}
	copy(large, []byte("package pkg\nvar c = http.Client{}\n"))
	hunks := map[string][]byte{"big.go": large}
	lensInput := NewLensInput(input, hunks, false, repo)
	if !lensInput.Truncated {
		t.Error("hunks >8MiB should set Truncated true")
	}
	total := 0
	for _, b := range lensInput.Hunks {
		total += len(b)
	}
	if total > HunkCapBytes {
		t.Errorf("capped total %d exceeds %d", total, HunkCapBytes)
	}
	// Stub lens should propagate truncated.
	l := &integrationStubLens{id: "resilience"}
	result, err := l.Analyze(context.Background(), lensInput)
	if err != nil {
		t.Fatalf("stub Analyze: %v", err)
	}
	if !result.Truncated {
		t.Error("result should propagate Truncated")
	}
	smallHunks := map[string][]byte{"small.go": []byte("package small\nfunc Foo(){}\n")}
	lensInput2 := NewLensInput(input, smallHunks, false, repo)
	if lensInput2.Truncated {
		t.Error("small hunks should not be truncated")
	}
}

func TestLens_Rollback_SequentialNoDAG(t *testing.T) {
	repo, head := fixtureRepo(t)
	input, err := review.DeriveRiskInput(repo, head, "")
	if err != nil {
		t.Fatalf("DeriveRiskInput: %v", err)
	}
	hunks := map[string][]byte{"pkg/foo.go": []byte("package foo\nfunc Foo(){}\n")}
	lensInput := NewLensInput(input, hunks, false, repo)
	ordered := Ordered(review.PlanLenses(review.RiskHigh, nil))
	_ = ordered
	l1 := &integrationStubLens{id: "resilience"}
	l2 := &integrationStubLens{id: "readability"}
	fail := &failingLens{id: "reliability"}
	s1 := NewLensStage(l1, lensInput)
	s2 := NewLensStage(fail, lensInput)
	s3 := NewLensStage(l2, lensInput)
	p := pipeline.New(s1, s2, s3)
	err = p.Execute(context.Background(), &model.ReviewState{})
	if err == nil {
		t.Fatal("pipeline should fail")
	}
	if s1.Result() == nil {
		t.Error("s1 should have executed")
	}
	if s2.Result() != nil {
		t.Error("failing stage should not cache result")
	}
	if s3.Result() != nil {
		t.Error("s3 must not execute after failure")
	}
	if got := review.PlanLenses(review.RiskHigh, nil); !equalStrings(got, []string{"risk", "resilience", "readability", "reliability"}) {
		t.Errorf("PlanLenses order not frozen: %v", got)
	}
}

func TestLens_OrderFreeze(t *testing.T) {
	want := []string{"risk", "resilience", "readability", "reliability"}
	if got := review.PlanLenses(review.RiskHigh, nil); !equalStrings(got, want) {
		t.Errorf("PlanLenses(RiskHigh) = %v, want %v", got, want)
	}
	declared := []string{"readability"}
	if got := review.PlanLenses(review.RiskHigh, declared); !equalStrings(got, declared) {
		t.Errorf("declared wins: got %v, want %v", got, declared)
	}
	ResetRegistry()
	RegisterLens(&integrationStubLens{id: "resilience"})
	RegisterLens(&integrationStubLens{id: "readability"})
	RegisterLens(&integrationStubLens{id: "reliability"})
	ordered := Ordered([]string{"risk", "resilience", "readability", "reliability", "unknown"})
	found := []string{}
	for _, l := range ordered {
		found = append(found, l.ID())
	}
	if len(found) != 3 {
		t.Errorf("Ordered len = %d, want 3", len(found))
	}
}

func TestLens_NoDAG(t *testing.T) {
	if _, err := os.Stat(filepath.Join(".", "graph.go")); !os.IsNotExist(err) {
		t.Error("graph.go must not exist (no DAG)")
	}
	if _, err := os.Stat(filepath.Join("internal", "review", "lens", "graph.go")); !os.IsNotExist(err) {
		t.Error("internal/review/lens/graph.go must not exist")
	}
}

func TestLens_TruncatedFlagPropagation(t *testing.T) {
	repo, head := fixtureRepo(t)
	input, _ := review.DeriveRiskInput(repo, head, "")
	hunks := map[string][]byte{"a.go": []byte("package a\n")}
	li := NewLensInput(input, hunks, true, repo)
	if !li.Truncated {
		t.Error("Truncated true should propagate")
	}
	l := &integrationStubLens{id: "readability"}
	result, _ := l.Analyze(context.Background(), li)
	if !result.Truncated {
		t.Error("result should propagate Truncated")
	}
	_ = strings.Contains
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
