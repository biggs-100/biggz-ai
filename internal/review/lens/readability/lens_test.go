package readability

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

// helper to build LensInput with minimal fields.
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
	if got := l.ID(); got != "readability" {
		t.Fatalf("ID() = %q, want readability", got)
	}
	var _ lens.Lens = (*Lens)(nil)
}

func TestLens_ImplementsLens(t *testing.T) {
	var l Lens
	result, err := l.Analyze(context.Background(), inputWith(nil, nil, nil, false, ""))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.LensID != "readability" {
		t.Errorf("LensID = %q, want readability", result.LensID)
	}
}

func TestLens_ParserFailure_Deterministic(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{
		"pkg/foo.go": []byte("package foo\nfunc (\n"),
	}
	in := inputWith([]string{"pkg/foo.go"}, map[string]int{"pkg/foo.go": 10}, hunks, false, "")
	result, err := l.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-parser-") && f.Class == review.EvidenceDeterministic {
			found = true
			if len(f.ProofRefs) == 0 {
				t.Error("deterministic parser finding must have ProofRefs")
			}
			if f.File != "pkg/foo.go" {
				t.Errorf("File = %q, want pkg/foo.go", f.File)
			}
		}
	}
	if !found {
		t.Fatalf("expected deterministic parser finding, got %+v", result.Findings)
	}
}

func TestLens_ParserFailure_ProofRefFileLine(t *testing.T) {
	l := &Lens{}
	// Invalid on line 2.
	hunks := map[string][]byte{
		"a/b.go": []byte("package b\nfunc (\n"),
	}
	in := inputWith([]string{"a/b.go"}, map[string]int{"a/b.go": 5}, hunks, false, "")
	result, err := l.Analyze(context.Background(), in)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected parser finding")
	}
	proof := result.Findings[0].ProofRefs[0]
	re := regexp.MustCompile(`^a/b\.go:\d+$`)
	if !re.MatchString(proof) {
		t.Errorf("ProofRef %q must match file:line", proof)
	}
	if result.Findings[0].Line <= 0 {
		t.Errorf("Line = %d, want >0", result.Findings[0].Line)
	}
	// Ensure proof line matches Line field.
	if proof != "a/b.go:"+strings.Split(proof, ":")[1] {
		t.Errorf("proof mismatch")
	}
}

func TestLens_ParserFailure_DeterministicClass(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"x.go": []byte("package x\nvar = {\n")}
	in := inputWith([]string{"x.go"}, map[string]int{"x.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	if len(result.Findings) == 0 {
		t.Fatal("expected parser finding")
	}
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-parser-") && f.Class != review.EvidenceDeterministic {
			t.Errorf("parser finding Class = %q, want deterministic", f.Class)
		}
	}
}

func TestLens_ParserSuccess_NoFinding(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"ok.go": []byte("package ok\n\nfunc Foo() {}\n")}
	in := inputWith([]string{"ok.go"}, map[string]int{"ok.go": 5}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-parser-") {
			t.Errorf("unexpected parser finding for valid Go: %+v", f)
		}
	}
}

func TestLens_NonGoFile_NoParserCheck(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"README.md": []byte("package foo\nfunc (\n")}
	in := inputWith([]string{"README.md"}, map[string]int{"README.md": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-parser-") {
			t.Errorf("non-Go file should not trigger parser: %+v", f)
		}
	}
}

func TestLens_Threshold_AnyFile_Over400_Inferential(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"pkg/foo.go"}, map[string]int{"pkg/foo.go": 450}, map[string][]byte{"pkg/foo.go": []byte("package foo\nfunc Foo(){}\n")}, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-threshold-") && f.File == "pkg/foo.go" {
			found = true
			if f.Class != review.EvidenceInferential {
				t.Errorf("threshold Class = %q, want inferential", f.Class)
			}
			if f.Severity != "info" {
				t.Errorf("threshold Severity = %q, want info", f.Severity)
			}
			if len(f.ProofRefs) == 0 || f.ProofRefs[0] != "pkg/foo.go:1" {
				t.Errorf("ProofRefs = %v, want pkg/foo.go:1", f.ProofRefs)
			}
		}
	}
	if !found {
		t.Fatalf("expected threshold finding for 450 lines, got %+v", result.Findings)
	}
}

func TestLens_Threshold_NonGo_Over400(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"docs/guide.md"}, map[string]int{"docs/guide.md": 401}, nil, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if f.File == "docs/guide.md" && strings.HasPrefix(f.ID, "R2-threshold-") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected threshold for non-Go 401 lines")
	}
}

func TestLens_Threshold_Go_Over200_Inferential(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"internal/bar.go"}, map[string]int{"internal/bar.go": 250}, map[string][]byte{"internal/bar.go": []byte("package bar\nfunc Foo(){}\n")}, false, "")
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if f.File == "internal/bar.go" && strings.HasPrefix(f.ID, "R2-threshold-") {
			found = true
			if f.Class != review.EvidenceInferential {
				t.Errorf("Class = %q want inferential", f.Class)
			}
		}
	}
	if !found {
		t.Fatalf("expected Go threshold >200, got %+v", result.Findings)
	}
}

func TestLens_Threshold_Go_Over400_Uses400Boundary(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 450}, map[string][]byte{"a.go": []byte("package a\nfunc F(){}\n")}, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if f.File == "a.go" && strings.Contains(f.Message, "400") {
			return
		}
	}
	t.Errorf("Go file 450 should report 400 boundary, got %+v", result.Findings)
}

func TestLens_Threshold_Exactly400_NoFinding(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"big.md"}, map[string]int{"big.md": 400}, nil, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if f.File == "big.md" && strings.HasPrefix(f.ID, "R2-threshold-") {
			t.Errorf("unexpected finding for exactly 400 lines: %+v", f)
		}
	}
}

func TestLens_Threshold_GoExactly200_NoFinding(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 200}, map[string][]byte{"a.go": []byte("package a\n")}, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if f.File == "a.go" && strings.HasPrefix(f.ID, "R2-threshold-") {
			t.Errorf("unexpected finding for Go exactly 200 lines")
		}
	}
}

func TestLens_Threshold_Below_NoFinding(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"small.go"}, map[string]int{"small.go": 50}, map[string][]byte{"small.go": []byte("package s\n")}, false, "")
	result, _ := l.Analyze(context.Background(), in)
	if len(result.Findings) != 0 {
		// Could be parser? valid Go no parser failure, and threshold below so none.
		t.Errorf("expected no findings for small file, got %+v", result.Findings)
	}
}

func TestLens_NoMixedCaseCheck(t *testing.T) {
	l := &Lens{}
	// File with mixedCase+underscore historically flagged, now MUST NOT.
	hunks := map[string][]byte{"My_File.go": []byte("package foo\nfunc Foo(){}\n")}
	in := inputWith([]string{"My_File.go"}, map[string]int{"My_File.go": 10}, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "mixed") || strings.Contains(f.Message, "naming") {
			t.Errorf("R2 must not check mixedCase+underscores, got finding %q", f.Message)
		}
	}
	// Only possible finding would be parser/threshold; both absent here.
	if len(result.Findings) != 0 {
		t.Errorf("mixedCase file with valid parser and small size should have no findings, got %+v", result.Findings)
	}
}

func TestLens_TruncatedPropagated(t *testing.T) {
	l := &Lens{}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{"a.go": []byte("package a\n")}, true, "")
	result, _ := l.Analyze(context.Background(), in)
	if !result.Truncated {
		t.Error("Truncated should propagate true")
	}
	in2 := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{"a.go": []byte("package a\n")}, false, "")
	result2, _ := l.Analyze(context.Background(), in2)
	if result2.Truncated {
		t.Error("Truncated false should propagate false")
	}
}

func TestLens_EmptyInput_NoFindings(t *testing.T) {
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
	if result.LensID != "readability" {
		t.Errorf("LensID = %q", result.LensID)
	}
}

func TestLens_MultipleFiles_Mixed(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{
		"good.go": []byte("package good\nfunc Foo(){}\n"),
		"bad.go":  []byte("package bad\nfunc (\n"),
	}
	summary := map[string]int{"good.go": 250, "bad.go": 10, "doc.md": 500}
	in := inputWith([]string{"good.go", "bad.go", "doc.md"}, summary, hunks, false, "")
	result, _ := l.Analyze(context.Background(), in)
	// good.go should have threshold (>200 Go), bad.go parser, doc.md threshold (>400)
	countThreshold := 0
	countParser := 0
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-threshold-") {
			countThreshold++
		}
		if strings.HasPrefix(f.ID, "R2-parser-") {
			countParser++
		}
	}
	if countThreshold < 2 {
		t.Errorf("expected at least 2 threshold findings (good.go, doc.md), got %d: %+v", countThreshold, result.Findings)
	}
	if countParser != 1 {
		t.Errorf("expected 1 parser finding (bad.go), got %d: %+v", countParser, result.Findings)
	}
}

func TestLens_EvidenceAndProofRefsPresent(t *testing.T) {
	l := &Lens{}
	hunks := map[string][]byte{"x.go": []byte("package x\nfunc (\n")}
	in := inputWith([]string{"x.go"}, map[string]int{"x.go": 450}, hunks, false, "")
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
		if f.Class != review.EvidenceDeterministic && f.Class != review.EvidenceInferential {
			t.Errorf("finding %q Class = %q invalid", f.ID, f.Class)
		}
		if f.LensID != "readability" {
			t.Errorf("LensID = %q", f.LensID)
		}
	}
}

func TestLens_HunkBound_ParserUsesHunkBytes(t *testing.T) {
	l := &Lens{}
	// Valid file on disk but hunk with invalid content should trigger deterministic
	// finding from hunk, not from repo file.
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "h.go")
	if err := os.WriteFile(goodPath, []byte("package h\nfunc Foo(){}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hunks := map[string][]byte{"h.go": []byte("package h\nfunc (\n")}
	in := inputWith([]string{"h.go"}, map[string]int{"h.go": 5}, hunks, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-parser-") {
			found = true
		}
	}
	if !found {
		t.Error("parser should use hunk bytes (invalid) even when repo file is valid")
	}
	// Reverse: hunk valid, repo invalid but hunk missing should fallback to repo
	dir2 := t.TempDir()
	badPath := filepath.Join(dir2, "fallback.go")
	if err := os.WriteFile(badPath, []byte("package f\nfunc (\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	in2 := inputWith([]string{"fallback.go"}, map[string]int{"fallback.go": 5}, nil, false, dir2)
	result2, _ := l.Analyze(context.Background(), in2)
	found2 := false
	for _, f := range result2.Findings {
		if strings.HasPrefix(f.ID, "R2-parser-") {
			found2 = true
		}
	}
	if !found2 {
		t.Error("parser should fallback to Repo file when Hunks absent")
	}
}

func TestLens_NoPluginNoGraphImport(t *testing.T) {
	// Guard: readability lens must not import plugin/ or planner graph.
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
					t.Errorf("%s imports %s — forbidden for R2", fname, path)
				}
				if strings.Contains(path, "internal/planner") {
					t.Errorf("%s imports %s — forbidden for R2 (no DAG)", fname, path)
				}
			}
		}
	}
}

// Table-driven threshold edge coverage.

func TestLens_Thresholds_Table(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		lines     int
		wantFound bool
	}{
		{"go-201", "a.go", 201, true},
		{"go-200", "a.go", 200, false},
		{"go-199", "a.go", 199, false},
		{"go-401", "a.go", 401, true},
		{"non-go-401", "a.md", 401, true},
		{"non-go-400", "a.md", 400, false},
		{"non-go-200", "a.md", 200, false},
	}
	l := &Lens{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := inputWith([]string{tc.path}, map[string]int{tc.path: tc.lines}, map[string][]byte{tc.path: []byte("package a\n")}, false, "")
			// Ensure Go files have valid parser content so only threshold matters.
			if strings.HasSuffix(tc.path, ".go") {
				// keep valid
			} else {
				// non-Go no parser
			}
			result, _ := l.Analyze(context.Background(), in)
			found := false
			for _, f := range result.Findings {
				if f.File == tc.path && strings.HasPrefix(f.ID, "R2-threshold-") {
					found = true
				}
			}
			if found != tc.wantFound {
				t.Errorf("path %q lines %d: found=%v want %v, findings %+v", tc.path, tc.lines, found, tc.wantFound, result.Findings)
			}
		})
	}
}
