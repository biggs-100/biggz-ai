package reliability

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
	if got := l.ID(); got != "reliability" {
		t.Fatalf("ID() = %q, want reliability", got)
	}
	var _ lens.Lens = (*Lens)(nil)
}

func TestLens_ImplementsLens(t *testing.T) {
	var l Lens
	result, err := l.Analyze(context.Background(), inputWith(nil, nil, nil, false, ""))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.LensID != "reliability" {
		t.Errorf("LensID = %q, want reliability", result.LensID)
	}
}

func TestLens_MissingTest_Inferential(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	in := inputWith([]string{"internal/foo/bar.go"}, map[string]int{"internal/foo/bar.go": 10}, map[string][]byte{"internal/foo/bar.go": []byte("package foo\nfunc Bar(){}\n")}, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-missing-test-") && f.File == "internal/foo/bar.go" {
			found = true
			if f.Class != review.EvidenceInferential {
				t.Errorf("Class = %q, want inferential", f.Class)
			}
			if f.LensID != "reliability" {
				t.Errorf("LensID = %q, want reliability", f.LensID)
			}
		}
	}
	if !found {
		t.Fatalf("expected missing-test inferential finding, got %+v", result.Findings)
	}
}

func TestLens_MissingTest_ProofRefFileLine(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	in := inputWith([]string{"a/b.go"}, map[string]int{"a/b.go": 5}, map[string][]byte{"a/b.go": []byte("package b\n")}, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	if len(result.Findings) == 0 {
		t.Fatal("expected missing-test finding")
	}
	var target *lens.LensFinding
	for i, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-missing-test-") {
			target = &result.Findings[i]
			break
		}
	}
	if target == nil {
		t.Fatal("missing-test finding not found")
	}
	if len(target.ProofRefs) == 0 || target.ProofRefs[0] != "a/b.go:1" {
		t.Errorf("ProofRefs = %v, want [a/b.go:1]", target.ProofRefs)
	}
	if target.Line != 1 {
		t.Errorf("Line = %d, want 1", target.Line)
	}
	re := regexp.MustCompile(`^a/b\.go:\d+$`)
	if !re.MatchString(target.ProofRefs[0]) {
		t.Errorf("ProofRef %q must match file:line", target.ProofRefs[0])
	}
}

func TestLens_WithSiblingTest_NoFinding(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	// Changed files include both bar.go and bar_test.go
	in := inputWith([]string{"internal/foo/bar.go", "internal/foo/bar_test.go"}, map[string]int{"internal/foo/bar.go": 10, "internal/foo/bar_test.go": 5}, map[string][]byte{"internal/foo/bar.go": []byte("package foo\n")}, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-missing-test-") && f.File == "internal/foo/bar.go" {
			t.Errorf("unexpected missing-test finding when sibling test exists: %+v", f)
		}
	}
}

func TestLens_WithTestOnDisk_NoFinding(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	// Create sibling test file on disk
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write foo.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo_test.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write foo_test.go: %v", err)
	}
	in := inputWith([]string{"pkg/foo.go"}, map[string]int{"pkg/foo.go": 10}, map[string][]byte{"pkg/foo.go": []byte("package pkg\n")}, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-missing-test-") && f.File == "pkg/foo.go" {
			t.Errorf("unexpected missing-test when test exists on disk: %+v", f)
		}
	}
}

func TestLens_NonGo_NoMissingTest(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	in := inputWith([]string{"README.md", "config.yaml"}, map[string]int{"README.md": 10, "config.yaml": 5}, nil, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-missing-test-") {
			t.Errorf("non-Go file should not trigger missing-test: %+v", f)
		}
	}
}

func TestLens_ErrorToken_Inferential(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	hunks := map[string][]byte{"pkg/foo.go": []byte("package foo\nfunc Bar(){ panic(\"oops\") }\n")}
	in := inputWith([]string{"pkg/foo.go"}, map[string]int{"pkg/foo.go": 10}, hunks, false, dir)
	// Create sibling test to avoid missing-test noise
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "foo_test.go"), []byte("package foo\n"), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-error-token-") {
			found = true
			if f.Class != review.EvidenceInferential {
				t.Errorf("error token Class = %q, want inferential", f.Class)
			}
		}
	}
	if !found {
		t.Fatalf("expected error-token inferential finding for panic, got %+v", result.Findings)
	}
}

func TestLens_ErrorToken_ProofRefFileLine(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	// panic on line 3
	hunks := map[string][]byte{"a/b.go": []byte("package b\nfunc Foo(){\n panic(\"x\")\n}\n")}
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b_test.go"), []byte("package b\n"), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	in := inputWith([]string{"a/b.go"}, map[string]int{"a/b.go": 5}, hunks, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	var target *lens.LensFinding
	for i, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-error-token-") {
			target = &result.Findings[i]
			break
		}
	}
	if target == nil {
		t.Fatal("expected error-token finding")
	}
	if len(target.ProofRefs) == 0 {
		t.Fatal("missing ProofRefs")
	}
	re := regexp.MustCompile(`^a/b\.go:\d+$`)
	if !re.MatchString(target.ProofRefs[0]) {
		t.Errorf("ProofRef %q must be file:line", target.ProofRefs[0])
	}
	if target.Line <= 0 {
		t.Errorf("Line = %d, want >0", target.Line)
	}
	if target.File != "a/b.go" {
		t.Errorf("File = %q, want a/b.go", target.File)
	}
}

func TestLens_ErrorToken_HunkBound(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	// Hunk with valid error token, but repo file also has panic — hunk should be used
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Repo file valid without panic
	if err := os.WriteFile(filepath.Join(dir, "pkg", "h.go"), []byte("package pkg\nfunc Foo(){}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "h_test.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	hunks := map[string][]byte{"pkg/h.go": []byte("package pkg\nfunc Bar(){ panic(\"hunk\") }\n")}
	in := inputWith([]string{"pkg/h.go"}, map[string]int{"pkg/h.go": 5}, hunks, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-error-token-") {
			found = true
		}
	}
	if !found {
		t.Error("error token should use hunk bytes (invalid) even when repo file is valid")
	}
	// Reverse: hunk empty but repo has panic — should NOT flag because hunk-bound and no fallback
	dir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir2, "pkg"), 0755); err != nil {
		t.Fatalf("mkdir2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "pkg", "f.go"), []byte("package pkg\nfunc Foo(){ panic(\"repo\") }\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "pkg", "f_test.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	in2 := inputWith([]string{"pkg/f.go"}, map[string]int{"pkg/f.go": 5}, map[string][]byte{}, false, dir2)
	result2, _ := l.Analyze(context.Background(), in2)
	for _, f := range result2.Findings {
		if strings.HasPrefix(f.ID, "R3-error-token-") {
			t.Errorf("error token should not use repo fallback, got finding %v", f)
		}
	}
}

func TestLens_NoVolumeFindings(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	// Create sibling test to silence missing-test
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "big_test.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	// Large DiffSummary should NOT trigger R3 volume finding (only R2 does)
	in := inputWith([]string{"pkg/big.go"}, map[string]int{"pkg/big.go": 1000}, map[string][]byte{"pkg/big.go": []byte("package pkg\nfunc Foo(){}\n")}, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if strings.Contains(strings.ToLower(f.Message), "400") || strings.Contains(strings.ToLower(f.Message), "threshold") || strings.Contains(strings.ToLower(f.Message), "volume") {
			t.Errorf("R3 must not emit volume findings, got %q", f.Message)
		}
		if strings.Contains(f.ID, "threshold") || strings.Contains(f.ID, "volume") {
			t.Errorf("R3 ID must not contain volume/threshold, got %q", f.ID)
		}
	}
}

func TestLens_TruncatedPropagated(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{"a.go": []byte("package a\n")}, true, dir)
	if err := os.MkdirAll(filepath.Join(dir, "tmp"), 0755); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package a\n"), 0644)
	}
	result, _ := l.Analyze(context.Background(), in)
	if !result.Truncated {
		t.Error("Truncated true should propagate")
	}
	in2 := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, map[string][]byte{"a.go": []byte("package a\n")}, false, dir)
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
	if result.LensID != "reliability" {
		t.Errorf("LensID = %q", result.LensID)
	}
}

func TestLens_MultipleFiles_Mixed(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "foo"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "foo", "with_test.go"), []byte("package foo\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "foo", "with_test_test.go"), []byte("package foo\n"), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}
	hunks := map[string][]byte{
		"internal/foo/with_test.go": []byte("package foo\nfunc A(){}\n"),
		"internal/foo/missing.go":   []byte("package foo\nfunc B(){ panic(\"x\") }\n"),
	}
	summary := map[string]int{"internal/foo/with_test.go": 10, "internal/foo/missing.go": 10}
	in := inputWith([]string{"internal/foo/with_test.go", "internal/foo/missing.go"}, summary, hunks, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	// with_test.go has test, so no missing-test; missing.go missing test + error token
	missing := 0
	errorTok := 0
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R3-missing-test-") {
			missing++
			if f.File != "internal/foo/missing.go" {
				t.Errorf("missing-test file = %q, want missing.go", f.File)
			}
		}
		if strings.HasPrefix(f.ID, "R3-error-token-") {
			errorTok++
		}
	}
	if missing != 1 {
		t.Errorf("expected 1 missing-test (missing.go), got %d: %+v", missing, result.Findings)
	}
	if errorTok != 1 {
		t.Errorf("expected 1 error-token (panic in no_test.go), got %d: %+v", errorTok, result.Findings)
	}
}

func TestLens_EvidenceAndProofRefsPresent(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	hunks := map[string][]byte{"x.go": []byte("package x\nfunc F(){ panic(\"x\") }\n")}
	in := inputWith([]string{"x.go"}, map[string]int{"x.go": 10}, hunks, false, dir)
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
		if f.Class != review.EvidenceInferential {
			t.Errorf("finding %q Class = %q, want inferential", f.ID, f.Class)
		}
		if f.LensID != "reliability" {
			t.Errorf("LensID = %q, want reliability", f.LensID)
		}
		if !strings.HasPrefix(f.ID, "R3-") {
			t.Errorf("ID %q must have R3 prefix", f.ID)
		}
	}
}

func TestLens_InferentialOnly(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	hunks := map[string][]byte{"a.go": []byte("package a\nfunc F(){ panic(\"x\") }\n")}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if f.Class != review.EvidenceInferential {
			t.Errorf("R3 must be inferential only, got %q for %q", f.Class, f.ID)
		}
		if f.Class == review.EvidenceDeterministic {
			t.Errorf("R3 must not have deterministic findings, got %q", f.ID)
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
					t.Errorf("%s imports %s — forbidden for R3", fname, path)
				}
				if strings.Contains(path, "internal/planner") {
					t.Errorf("%s imports %s — forbidden for R3 (no DAG)", fname, path)
				}
			}
		}
	}
}

func TestLens_MissingTest_Table(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		withTest  bool
		wantFound bool
	}{
		{"no test", "pkg/foo.go", false, true},
		{"with test", "pkg/foo.go", true, false},
		{"nested no test", "internal/auth/token.go", false, true},
		{"non-go no check", "README.md", false, false},
		{"test file itself", "pkg/foo_test.go", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			summary := map[string]int{}
			paths := []string{tc.path}
			if tc.withTest {
				testPath := strings.TrimSuffix(tc.path, ".go") + "_test.go"
				paths = append(paths, testPath)
				summary[testPath] = 5
			}
			summary[tc.path] = 10
			hunks := map[string][]byte{tc.path: []byte("package foo\n")}
			// For test file itself, no missing-test should occur regardless
			if strings.HasSuffix(tc.path, "_test.go") {
				// Ensure we don't count test file as Go file needing test
				hunks = map[string][]byte{tc.path: []byte("package foo\n")}
			}
			in := inputWith(paths, summary, hunks, false, dir)
			// Ensure on-disk test does not interfere
			if tc.withTest {
				// Create on disk as well to double-check
				base := strings.TrimSuffix(tc.path, ".go")
				dirPath := filepath.Join(dir, filepath.Dir(tc.path))
				if err := os.MkdirAll(dirPath, 0755); err == nil {
					_ = os.WriteFile(filepath.Join(dir, base+"_test.go"), []byte("package foo\n"), 0644)
				}
			}
			l := &Lens{}
			result, _ := l.Analyze(context.Background(), in)
			found := false
			for _, f := range result.Findings {
				if strings.HasPrefix(f.ID, "R3-missing-test-") && f.File == tc.path {
					found = true
				}
			}
			if found != tc.wantFound {
				t.Errorf("path %q withTest=%v: found=%v want %v, findings %+v", tc.path, tc.withTest, found, tc.wantFound, result.Findings)
			}
		})
	}
}

func TestLens_ErrorTokens_Table(t *testing.T) {
	cases := []struct {
		name      string
		hunk      string
		wantFound bool
	}{
		{"panic", "package foo\nfunc F(){ panic(\"x\") }", true},
		{"log.Fatal", "package foo\nfunc F(){ log.Fatal(err) }", true},
		{"errors.New", "package foo\nvar e = errors.New(\"x\")", true},
		{"fmt.Errorf", "package foo\nfunc F(){ return fmt.Errorf(\"x\") }", true},
		{"no token", "package foo\nfunc F(){ return nil }", false},
		{"err nil check", "package foo\nfunc F(){ if err != nil { return err } }", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			hunks := map[string][]byte{"a.go": []byte(tc.hunk)}
			// Create sibling test to silence missing-test
			if err := os.MkdirAll(filepath.Join(dir, "a"), 0755); err == nil {
				_ = os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package a\n"), 0644)
			}
			// Use repo dir that contains a_test.go? Our input paths use a.go, repo dir is tempDir, need a/b.go?
			// Use pkg prefix
			hunks2 := map[string][]byte{"pkg/a.go": []byte(tc.hunk)}
			if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0755); err == nil {
				_ = os.WriteFile(filepath.Join(dir, "pkg", "a_test.go"), []byte("package pkg\n"), 0644)
			}
			in := inputWith([]string{"pkg/a.go"}, map[string]int{"pkg/a.go": 10}, hunks2, false, dir)
			_ = hunks
			l := &Lens{}
			result, _ := l.Analyze(context.Background(), in)
			found := false
			for _, f := range result.Findings {
				if strings.HasPrefix(f.ID, "R3-error-token-") {
					found = true
				}
			}
			if found != tc.wantFound {
				t.Errorf("hunk %q: found=%v want %v, findings %+v", tc.hunk, found, tc.wantFound, result.Findings)
			}
		})
	}
}

func TestLens_ID_Prefix(t *testing.T) {
	l := &Lens{}
	dir := t.TempDir()
	hunks := map[string][]byte{"a.go": []byte("package a\nfunc F(){ panic(\"x\") }\n")}
	in := inputWith([]string{"a.go"}, map[string]int{"a.go": 10}, hunks, false, dir)
	result, _ := l.Analyze(context.Background(), in)
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "R3-") {
			t.Errorf("ID %q must have R3 prefix", f.ID)
		}
	}
}
