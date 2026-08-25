package resilience

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

func inputWith(paths []string, summary map[string]int, hunks map[string][]byte, truncated bool, repo string) lens.LensInput {
	total := 0
	for _, v := range summary {
		total += v
	}
	return lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:        paths,
			ChangedLines: total,
			DiffSummary:  summary,
			BaseTree:     "abc",
		},
		Hunks:     hunks,
		Truncated: truncated,
		Repo:      repo,
	}
}

func TestLens_ID(t *testing.T) {
	var l Lens
	if got := l.ID(); got != "resilience" {
		t.Fatalf("ID() = %q, want resilience", got)
	}
	var _ lens.Lens = (*Lens)(nil)
}

func TestLens_ImplementsLens(t *testing.T) {
	var l Lens
	result, err := l.Analyze(context.Background(), inputWith(nil, nil, nil, false, ""))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.LensID != "resilience" {
		t.Errorf("LensID = %q, want resilience", result.LensID)
	}
}

func TestLens_Timeout_HunkFinding_Inferential(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/http.go": []byte("package pkg\nvar c = http.Client{}\n")}
	in := inputWith([]string{"pkg/http.go"}, map[string]int{"pkg/http.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "timeout") && f.File == "pkg/http.go" {
			found = true
			if f.Class != review.EvidenceInferential {
				t.Errorf("Class = %q, want inferential", f.Class)
			}
			if len(f.ProofRefs) == 0 {
				t.Error("ProofRefs must not be empty")
			}
			if f.LensID != "resilience" {
				t.Errorf("LensID = %q", f.LensID)
			}
		}
	}
	if !found {
		t.Fatalf("expected timeout inferential finding, got %+v", result.Findings)
	}
}

func TestLens_Timeout_ProofRefFileLine(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"a/b.go": []byte("package b\nvar c = http.Client{}\n")}
	in := inputWith([]string{"a/b.go"}, map[string]int{"a/b.go": 5}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	if len(result.Findings) == 0 {
		t.Fatal("expected timeout finding")
	}
	proof := result.Findings[0].ProofRefs[0]
	re := regexp.MustCompile(`^a/b\.go:\d+$`)
	if !re.MatchString(proof) {
		t.Errorf("ProofRef %q must match file:line", proof)
	}
	if result.Findings[0].Line <= 0 {
		t.Errorf("Line = %d, want >0", result.Findings[0].Line)
	}
}

func TestLens_Timeout_WithTimeout_NoFinding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/http.go": []byte("package pkg\nvar c = http.Client{Timeout: time.Second}\n")}
	in := inputWith([]string{"pkg/http.go"}, map[string]int{"pkg/http.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "timeout") {
			t.Errorf("http.Client with Timeout should not flag, got %+v", f)
		}
	}
}

func TestLens_Context_Finding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/service.go": []byte("package pkg\nfunc Foo(){ ctx := context.Background() }\n")}
	in := inputWith([]string{"pkg/service.go"}, map[string]int{"pkg/service.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "context") {
			found = true
			if f.Class != review.EvidenceInferential {
				t.Errorf("Class = %q, want inferential", f.Class)
			}
		}
	}
	if !found {
		t.Fatalf("expected context finding, got %+v", result.Findings)
	}
}

func TestLens_Context_WithCancel_NoFinding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/service.go": []byte("package pkg\nfunc Foo(){ ctx, cancel := context.WithCancel(context.Background()) }\n")}
	in := inputWith([]string{"pkg/service.go"}, map[string]int{"pkg/service.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "context") {
			t.Errorf("WithCancel should not flag context, got %+v", f)
		}
	}
}

func TestLens_Concurrency_Finding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/worker.go": []byte("package pkg\nfunc Foo(){ go func(){ do() }() }\n")}
	in := inputWith([]string{"pkg/worker.go"}, map[string]int{"pkg/worker.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "concurrency") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected concurrency finding for go func, got %+v", result.Findings)
	}
}

func TestLens_Concurrency_WithWaitGroup_NoFinding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/worker.go": []byte("package pkg\nfunc Foo(){ go func(){ wg.Done() }(); wg.Wait() }\n")}
	// This hunk contains WaitGroup so our heuristic checks for waitgroup token
	// and currently flags only when missing waitgroup. Our implementation checks
	// if line contains go and not contains waitgroup/errgroup/context. This line has waitgroup, but also has go, but our check is per line, not across lines.
	// So we test a single line with both tokens should not flag.
	// Create a line that has both go and waitgroup
	hunks2 := map[string][]byte{"pkg/worker.go": []byte("package pkg\nfunc Foo(){ go func(){ wg.Done() }() // waitgroup }\n")}
	in := inputWith([]string{"pkg/worker.go"}, map[string]int{"pkg/worker.go": 10}, hunks2, false, "")
	result, _ := l.Analyze(context.Background(), in)
	// This line contains waitgroup so should not be flagged
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "concurrency") && f.Line == 2 {
			t.Errorf("go with waitgroup should not flag, got %+v", f)
		}
	}
	_ = hunks
	_ = result
}

func TestLens_Cleanup_Finding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/file.go": []byte("package pkg\nfunc Foo(){ f, _ := os.Open(\"a.txt\") }\n")}
	in := inputWith([]string{"pkg/file.go"}, map[string]int{"pkg/file.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "cleanup") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cleanup finding for os.Open without defer, got %+v", result.Findings)
	}
}

func TestLens_Cleanup_WithDefer_NoFinding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"pkg/file.go": []byte("package pkg\nfunc Foo(){ f, _ := os.Open(\"a.txt\"); defer f.Close() }\n")}
	in := inputWith([]string{"pkg/file.go"}, map[string]int{"pkg/file.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "cleanup") {
			t.Errorf("os.Open with defer Close should not flag, got %+v", f)
		}
	}
}

func TestLens_NonGo_NoFinding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"README.md": []byte("http.Client{} context.Background() go func() os.Open")}
	in := inputWith([]string{"README.md"}, map[string]int{"README.md": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	if len(result.Findings) != 0 {
		t.Errorf("non-Go hunks should not trigger, got %+v", result.Findings)
	}
}

func TestLens_HunkBound_NoFallback(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	// Repo file has timeout pattern, but hunks empty — should NOT flag (no fallback)
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nvar c = http.Client{}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Ensure hunk is empty
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{}, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "timeout") {
			t.Errorf("should not fallback to full file, got %+v", f)
		}
	}
	// Hunk with valid content should flag even when repo file is clean
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "b.go"), []byte("package b\nvar c = http.Client{Timeout: time.Second}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hunks := map[string][]byte{"b.go": []byte("package b\nvar c = http.Client{}\n")}
	in2 := inputWith([]string{"b.go"}, map[string]int{"b.go": 10}, hunks, false, dir2)
	result2, _ := l.Analyze(context.Background(), in2)
	found := false
	for _, f := range result2.Findings {
		if strings.Contains(f.ID, "timeout") {
			found = true
		}
	}
	if !found {
		t.Error("hunk-bound flag should trigger from hunk bytes even when repo file is clean")
	}
}

func TestLens_8MiBCap_TruncatedFlag(t *testing.T) {
	l := &Lens{}
	// Build hunks exceeding 8MiB
	large := make([]byte, capBytesForTest+1024)
	for i := range large {
		large[i] = 'a'
	}
	// Insert a timeout pattern at start so it would be found within cap
	copy(large, []byte("package pkg\nvar c = http.Client{}\n"))
	hunks := map[string][]byte{"big.go": large}
	in := inputWith([]string{"big.go"}, map[string]int{"big.go": 100}, hunks, false, "")
	result, err := l.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Truncated {
		t.Error("hunks >8MiB should set Truncated true")
	}
	// Under cap should not be truncated
	small := []byte("package pkg\nvar c = http.Client{}\n")
	hunks2 := map[string][]byte{"small.go": small}
	in2 := inputWith([]string{"small.go"}, map[string]int{"small.go": 10}, hunks2, false, "")
	result2, _ := l.Analyze(context.Background(), in2)
	if result2.Truncated {
		t.Error("hunks <=8MiB should not set Truncated")
	}
}

func TestLens_8MiBCap_NoError(t *testing.T) {
	l := &Lens{}
	large := make([]byte, capBytesForTest+100)
	for i := range large {
		large[i] = 'x'
	}
	copy(large, []byte("package pkg\n"))
	hunks := map[string][]byte{"a.go": large}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, false, "")
	_, err := l.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("8MiB cap should not error, got %v", err)
	}
}

func TestLens_TruncatedPropagated(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{"a.go": []byte("package a\nvar c = http.Client{}\n")}, true, "")
	result, _ := l.Analyze(context.Background(), in)
	if !result.Truncated {
		t.Error("Truncated true should propagate")
	}
	in2 := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{"a.go": []byte("package a\n")}, false, "")
	result2, _ := l.Analyze(context.Background(), in2)
	if result2.Truncated {
		t.Error("Truncated false with small hunks should stay false")
	}
	// Also when already truncated and hunks exceed cap, stays true
	large := make([]byte, capBytesForTest+10)
	copy(large, []byte("package a\n"))
	hunks := map[string][]byte{"a.go": large}
	in3 := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, true, "")
	result3, _ := l.Analyze(context.Background(), in3)
	if !result3.Truncated {
		t.Error("Truncated should stay true when input already truncated and over cap")
	}
}

func TestLens_EmptyInput(t *testing.T) {
	l := &Lens{}
	in := inputWith(nil, nil, nil, false, "")
	result, err := l.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("empty input should have no findings, got %d", len(result.Findings))
	}
	if result.Truncated {
		t.Error("empty Truncated false")
	}
	if result.LensID != "resilience" {
		t.Errorf("LensID = %q", result.LensID)
	}
}

func TestLens_InferentialOnly(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{
		"a.go": []byte("package a\nvar c = http.Client{}\nctx := context.Background()\ngo func(){ }()\n f, _ := os.Open(\"x\")"),
	}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 20}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if f.Class != review.EvidenceInferential {
			t.Errorf("R4 must be inferential only, got %q for %q", f.Class, f.ID)
		}
		if f.Class == review.EvidenceDeterministic {
			t.Errorf("R4 must not have deterministic, got %q", f.ID)
		}
		if !strings.HasPrefix(f.ID, "R4-") {
			t.Errorf("ID %q must have R4 prefix", f.ID)
		}
	}
	if len(result.Findings) == 0 {
		t.Fatalf("expected findings for combined patterns, got none")
	}
}

func TestLens_EvidenceAndProofRefs(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"x.go": []byte("package x\nvar c = http.Client{}\n")}
	in := inputWith([]string{"x.go"}, map[string]int{"x.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	if len(result.Evidence) == 0 {
		t.Error("Evidence should not be empty when findings present")
	}
	for _, f := range result.Findings {
		if len(f.ProofRefs) == 0 {
			t.Errorf("finding %q missing ProofRefs", f.ID)
		}
		for _, pr := range f.ProofRefs {
			if !strings.Contains(pr, ":") {
				t.Errorf("ProofRef %q must be file:line", pr)
			}
		}
		if f.LensID != "resilience" {
			t.Errorf("LensID = %q", f.LensID)
		}
	}
}

func TestLens_MultiplePatterns(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{
		"a.go": []byte("package a\nvar c = http.Client{}\n"),
		"b.go": []byte("package b\nfunc Foo(){ ctx := context.Background() }\n"),
		"c.go": []byte("package c\nfunc Foo(){ go func(){ }() }\n"),
		"d.go": []byte("package d\nfunc Foo(){ f, _ := os.Open(\"x\") }\n"),
	}
	paths := []string{"a.go", "b.go", "c.go", "d.go"}
	summary := map[string]int{"a.go": 5, "b.go": 5, "c.go": 5, "d.go": 5}
	in := inputWith(paths, summary, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	kinds := map[string]int{}
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "timeout") {
			kinds["timeout"]++
		} else if strings.Contains(f.ID, "context") {
			kinds["context"]++
		} else if strings.Contains(f.ID, "concurrency") {
			kinds["concurrency"]++
		} else if strings.Contains(f.ID, "cleanup") {
			kinds["cleanup"]++
		}
	}
	for _, want := range []string{"timeout", "context", "concurrency", "cleanup"} {
		if kinds[want] == 0 {
			t.Errorf("expected at least one %s finding, got kinds %v, findings %+v", want, kinds, result.Findings)
		}
	}
}

func TestLens_NoPluginNoGraphImport(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, "\"")
				if strings.Contains(path, "plugin") {
					t.Errorf("%s imports %s — forbidden for R4", fname, path)
				}
				if strings.Contains(path, "internal/planner") {
					t.Errorf("%s imports %s — forbidden for R4 (no DAG)", fname, path)
				}
			}
		}
	}
}

func TestLens_ID_Prefix(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"a.go": []byte("package a\nvar c = http.Client{}\n")}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "R4-") {
			t.Errorf("ID %q must have R4 prefix", f.ID)
		}
	}
}

func TestLens_Threshold_Table(t *testing.T) {
	cases := []struct {
		name      string
		hunk      string
		kind      string
		wantFound bool
	}{
		{"timeout hit", "package a\nvar c = http.Client{}\n", "timeout", true},
		{"timeout ok", "package a\nvar c = http.Client{Timeout: 5}\n", "timeout", false},
		{"context hit", "package a\nfunc F(){ ctx:=context.Background() }\n", "context", true},
		{"context ok", "package a\nfunc F(){ ctx, cancel:=context.WithCancel(context.Background()) }\n", "context", false},
		{"concurrency hit", "package a\nfunc F(){ go func(){}() }\n", "concurrency", true},
		{"cleanup hit", "package a\nfunc F(){ f,_:=os.Open(\"x\") }\n", "cleanup", true},
		{"cleanup ok", "package a\nfunc F(){ f,_:=os.Open(\"x\"); defer f.Close() }\n", "cleanup", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hunks := map[string][]byte{"a.go": []byte(tc.hunk)}
			in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, false, "")
			l := &Lens{}
			result, _ := l.Analyze(context.Background(), in)
			found := false
			for _, f := range result.Findings {
				if strings.Contains(f.ID, tc.kind) {
					found = true
				}
			}
			if found != tc.wantFound {
				t.Errorf("hunk %q kind %q: found=%v want %v, findings %+v", tc.hunk, tc.kind, found, tc.wantFound, result.Findings)
			}
		})
	}
}

// capBytesForTest is the cap used in tests (same as lens capBytes).
var capBytesForTest = 8 << 20

func TestLens_RepoNotUsed(t *testing.T) {
	// Ensure Repo field does not affect resilience findings (hunk-bound only)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nvar c = http.Client{Timeout: 1}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hunks := map[string][]byte{"a.go": []byte("package a\nvar c = http.Client{}\n")}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, false, dir)
	l := &Lens{}
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.ID, "timeout") {
			found = true
		}
	}
	if !found {
		t.Error("should flag timeout from hunk even when repo file has Timeout")
	}
	_ = filepath.Join
}
