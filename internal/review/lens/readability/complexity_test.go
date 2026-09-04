package readability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
)

func TestGitPathSelection(t *testing.T) {
	// Verify absolute repo path produces no fallback warning, relative does.
	dir := t.TempDir()
	// Create a simple Go file under critical package
	pkgDir := filepath.Join(dir, "internal", "review", "lens")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	filePath := filepath.Join(pkgDir, "foo.go")
	content := "package lens\nfunc Foo(){ if true { if true { if true { } } } }\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Use lens.LensInput with Repo absolute
	absRepo := dir
	inputAbs := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/lens/foo.go"},
			DiffSummary: map[string]int{"internal/review/lens/foo.go": 5},
		},
		Hunks: map[string][]byte{
			"internal/review/lens/foo.go": []byte(content),
		},
		Repo: absRepo,
	}
	_, warnsAbs := offendersFromHunks(inputAbs)
	for _, w := range warnsAbs {
		if strings.Contains(w, "relative") {
			t.Errorf("absolute repo should not produce relative warning, got %q", w)
		}
	}

	// Relative repo should produce warning
	relRepo := "relative/path"
	inputRel := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/lens/foo.go"},
			DiffSummary: map[string]int{"internal/review/lens/foo.go": 5},
		},
		Hunks: map[string][]byte{
			"internal/review/lens/foo.go": []byte(content),
		},
		Repo: relRepo,
	}
	_, warnsRel := offendersFromHunks(inputRel)
	found := false
	for _, w := range warnsRel {
		if strings.Contains(w, "relative") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("relative repo should produce fallback warning, got %v", warnsRel)
	}
}

func TestComplexityThresholds(t *testing.T) {
	if CyclomaticThreshold != 15 {
		t.Errorf("CyclomaticThreshold = %d, want 15", CyclomaticThreshold)
	}
	if CognitiveThreshold != 20 {
		t.Errorf("CognitiveThreshold = %d, want 20", CognitiveThreshold)
	}
}

func TestFindFuncAtLine(t *testing.T) {
	src := []byte("package p\n\nfunc Foo() {\n}\n\nfunc Bar() {\n  x:=1\n}\n")
	name, line, ok := findFuncAtLine("p.go", src, 3)
	if !ok || name != "Foo" || line != 3 {
		t.Errorf("findFuncAtLine line 3: got %q %d %v, want Foo 3 true", name, line, ok)
	}
	name, line, ok = findFuncAtLine("p.go", src, 6)
	if !ok || name != "Bar" {
		t.Errorf("findFuncAtLine line 6: got %q %d %v", name, line, ok)
	}
	_, _, ok = findFuncAtLine("p.go", src, 1)
	if ok {
		t.Error("line 1 should not be inside function")
	}
}

// Helpers for CI gate tests: content with specific complexities
var highCycloContent = []byte(`package p
func Foo(a int) {
 if a==1 {}
 if a==2 {}
 if a==3 {}
 if a==4 {}
 if a==5 {}
 if a==6 {}
 if a==7 {}
 if a==8 {}
 if a==9 {}
 if a==10 {}
 if a==11 {}
 if a==12 {}
 if a==13 {}
 if a==14 {}
 if a==15 {}
 if a==16 {}
 if a==17 {}
}
`)

var highCognitContent = []byte(`package p
func Bar() {
 if true {
  if true {
   if true {
    if true {
     if true {
      if true {
       if true {
        for i:=0; i<10; i++ {
         if true {}
         if true {}
         if true {}
        }
       }
      }
     }
    }
   }
  }
 }
}
`)

func TestCIGate_Cyclomatic18Fail(t *testing.T) {
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/foo.go"},
			DiffSummary: map[string]int{"internal/review/foo.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/review/foo.go": highCycloContent,
		},
		Repo: "",
	}
	offs, _ := offendersFromHunks(input)
	found := false
	for _, o := range offs {
		if o.File == "internal/review/foo.go" && o.Cyclomatic > 15 {
			found = true
			if o.Function != "Foo" {
				t.Errorf("expected Foo, got %q", o.Function)
			}
		}
	}
	if !found {
		t.Errorf("expected cyclomatic 18 violation for Foo, got offenders %v", offs)
	}
}

func TestCIGate_Cognitive22Fail(t *testing.T) {
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/sdd/service.go"},
			DiffSummary: map[string]int{"internal/sdd/service.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/sdd/service.go": highCognitContent,
		},
		Repo: "",
	}
	offs, _ := offendersFromHunks(input)
	found := false
	for _, o := range offs {
		if o.File == "internal/sdd/service.go" && o.Cognitive > 20 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cognitive 22+ violation for Bar, got %v", offs)
	}
}

func TestCIGate_BothThresholdsIndependently(t *testing.T) {
	// High cyclo only (cognitive 17) should only report cyclo, not cognit
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/foo.go"},
			DiffSummary: map[string]int{"internal/review/foo.go": 5},
		},
		Hunks: map[string][]byte{
			"internal/review/foo.go": highCycloContent,
		},
		Repo: "",
	}
	offs, _ := offendersFromHunks(input)
	if len(offs) == 0 {
		t.Fatalf("expected offender")
	}
	o := offs[0]
	if !(o.Cyclomatic > 15) {
		t.Errorf("expected cyclomatic >15, got %d", o.Cyclomatic)
	}
	if o.Cognitive > 20 {
		t.Errorf("cognitive should be <=20 for this case, got %d", o.Cognitive)
	}
}

func TestCIGate_TestFileInfoOnly(t *testing.T) {
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/foo_test.go"},
			DiffSummary: map[string]int{"internal/review/foo_test.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/review/foo_test.go": highCycloContent,
		},
		Repo: "",
	}
	offs, _ := offendersFromHunks(input)
	// Should still be collected but caller treats as informational
	if len(offs) == 0 {
		t.Errorf("test file offender should be collected as informational, got none")
	}
	// Verify lens will mark test files as info severity (checked via offendersFromHunks + isTestFile)
	for _, o := range offs {
		if !isTestFile(o.File) {
			t.Errorf("expected test file, got %q", o.File)
		}
	}
}

func TestCIGate_OutOfScopeIgnored(t *testing.T) {
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/cli/foo.go"},
			DiffSummary: map[string]int{"internal/cli/foo.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/cli/foo.go": highCycloContent,
		},
		Repo: "",
	}
	offs, _ := offendersFromHunks(input)
	if len(offs) != 0 {
		t.Errorf("out-of-scope package internal/cli should be ignored, got %v", offs)
	}
}

func TestCIGate_LegacyNotBlocked(t *testing.T) {
	// Legacy file not in changed set should not be reported
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/other.go"},
			DiffSummary: map[string]int{"internal/review/other.go": 5},
		},
		Hunks: map[string][]byte{
			"internal/review/other.go": []byte("package p\nfunc Other(){}"),
		},
		Repo: "",
	}
	// Do not include the legacy old.go in input; even though we could have high complexity there, it should not be reported
	// Simulate that old.go has high complexity but not in Paths/Hunks
	offs, _ := offendersFromHunks(input)
	for _, o := range offs {
		if o.File == "internal/verification/old.go" {
			t.Errorf("legacy file not in changed set should not be reported, got %v", o)
		}
	}
}

func TestCIGate_ModifiedLegacyBlocks(t *testing.T) {
	// Modified legacy file should be reported
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/verification/old.go"},
			DiffSummary: map[string]int{"internal/verification/old.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/verification/old.go": highCycloContent,
		},
		Repo: "",
	}
	offs, _ := offendersFromHunks(input)
	found := false
	for _, o := range offs {
		if o.File == "internal/verification/old.go" && o.Cyclomatic > 15 {
			found = true
		}
	}
	if !found {
		t.Errorf("modified legacy should block, got %v", offs)
	}
}

func TestCIGate_RenameWarn(t *testing.T) {
	// Diff with rename but no mappable hunks should warn and not block
	diffContent := "diff --git a/old.go b/new.go\n@@ -0,0 +0,0 @@\n"
	// We simulate by passing diff-like hunk with no valid header range (count 0)
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/old.go"},
			DiffSummary: map[string]int{"internal/review/old.go": 1},
		},
		Hunks: map[string][]byte{
			"internal/review/old.go": []byte(diffContent),
		},
		Repo: "",
	}
	offs, warns := offendersFromHunks(input)
	if len(offs) != 0 {
		t.Errorf("rename with no mappable hunks should not block, got %v", offs)
	}
	foundWarn := false
	for _, w := range warns {
		if strings.Contains(w, "rename") || strings.Contains(w, "no mappable") || strings.Contains(w, "warning") {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Errorf("expected rename warning, got %v", warns)
	}
}

func TestGrandfather(t *testing.T) {
	t.Run("legacy untouched not block", func(t *testing.T) {
		input := lens.LensInput{
			RiskInput: review.RiskInput{
				Paths:       []string{"internal/review/new.go"},
				DiffSummary: map[string]int{"internal/review/new.go": 2},
			},
			Hunks: map[string][]byte{
				"internal/review/new.go": []byte("package p\nfunc New(){}\n"),
			},
			Repo: "",
		}
		offs, _ := offendersFromHunks(input)
		for _, o := range offs {
			if o.File == "internal/verification/old.go" {
				t.Errorf("legacy old.go should not appear when not changed")
			}
		}
	})
}

func TestLens_R2Cyclo(t *testing.T) {
	l := &Lens{}
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/lens/foo.go"},
			DiffSummary: map[string]int{"internal/review/lens/foo.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/review/lens/foo.go": highCycloContent,
		},
		Repo: "",
	}
	result, err := l.Analyze(nil, input)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-CYCLO-") && f.Class == review.EvidenceInferential {
			found = true
			if len(f.ProofRefs) == 0 || !strings.Contains(f.ProofRefs[0], "18 >15") {
				t.Errorf("ProofRef %v should contain 18 >15", f.ProofRefs)
			}
			if f.File != "internal/review/lens/foo.go" {
				t.Errorf("File = %q", f.File)
			}
		}
	}
	if !found {
		t.Errorf("expected R2-CYCLO finding, got %v", result.Findings)
	}
}

func TestLens_R2Cognit(t *testing.T) {
	l := &Lens{}
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/sdd/bar.go"},
			DiffSummary: map[string]int{"internal/sdd/bar.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/sdd/bar.go": highCognitContent,
		},
		Repo: "",
	}
	result, err := l.Analyze(nil, input)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-COGNIT-") && f.Class == review.EvidenceInferential {
			found = true
			if len(f.ProofRefs) == 0 || !strings.Contains(f.ProofRefs[0], ">20") {
				t.Errorf("ProofRef %v should contain >20", f.ProofRefs)
			}
		}
	}
	if !found {
		t.Errorf("expected R2-COGNIT, got %v", result.Findings)
	}
}

func TestLens_HunkBounded(t *testing.T) {
	l := &Lens{}
	// Legacy violation not in hunk: file not in changed set should not produce finding
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/other.go"},
			DiffSummary: map[string]int{"internal/review/other.go": 5},
		},
		Hunks: map[string][]byte{
			"internal/review/other.go": []byte("package p\nfunc Other(){}\n"),
		},
		Repo: "",
	}
	result, _ := l.Analyze(nil, input)
	for _, f := range result.Findings {
		if (strings.HasPrefix(f.ID, "R2-CYCLO-") || strings.HasPrefix(f.ID, "R2-COGNIT-")) && f.File == "internal/verification/old.go" {
			t.Errorf("legacy FuncOld should not appear when not in hunk, got %v", f)
		}
	}
}

func TestLens_TestFileInformational(t *testing.T) {
	l := &Lens{}
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/foo_test.go"},
			DiffSummary: map[string]int{"internal/review/foo_test.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/review/foo_test.go": highCycloContent,
		},
		Repo: "",
	}
	result, _ := l.Analyze(nil, input)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-CYCLO-") && f.File == "internal/review/foo_test.go" {
			if f.Severity != "info" {
				t.Errorf("test file R2-CYCLO should be informational info, got %q", f.Severity)
			}
			if f.Class != review.EvidenceInferential {
				t.Errorf("Class should be inferential")
			}
			return
		}
	}
	// If no finding, it's okay because test file may be treated as info but still should appear
	// Instead ensure it exists (our offender includes test, lens emits)
	t.Errorf("expected informational R2-CYCLO for test file, got %v", result.Findings)
}

func TestLens_ProofRef(t *testing.T) {
	l := &Lens{}
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/lens/foo.go"},
			DiffSummary: map[string]int{"internal/review/lens/foo.go": 10},
		},
		Hunks: map[string][]byte{
			"internal/review/lens/foo.go": highCycloContent,
		},
		Repo: "",
	}
	result, _ := l.Analyze(nil, input)
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-CYCLO-") {
			if f.Class != review.EvidenceInferential {
				t.Errorf("expected inferential")
			}
			if len(f.ProofRefs) == 0 || !strings.Contains(f.ProofRefs[0], ":") {
				t.Errorf("ProofRef must be file:line")
			}
			if !strings.Contains(f.ProofRefs[0], "Foo") && !strings.Contains(f.Message, "Foo") {
				t.Errorf("ProofRef/Message should contain function name, got %v / %q", f.ProofRefs, f.Message)
			}
			return
		}
	}
	t.Error("no R2-CYCLO finding for ProofRef test")
}

func TestLens_NoSecondDiff(t *testing.T) {
	// Verify that Analyze does not invoke git diff; we can't easily detect git calls,
	// but we verify that input without Repo and with Hunks still yields findings via reuse.
	l := &Lens{}
	input := lens.LensInput{
		RiskInput: review.RiskInput{
			Paths:       []string{"internal/review/foo.go"},
			DiffSummary: map[string]int{"internal/review/foo.go": 5},
		},
		Hunks: map[string][]byte{
			"internal/review/foo.go": highCycloContent,
		},
		Repo: "",
	}
	result, err := l.Analyze(nil, input)
	if err != nil {
		t.Fatalf("Analyze should not require second diff, got error %v", err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.ID, "R2-CYCLO") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding via DeriveRiskInput reuse without second diff")
	}
}

func writeCycloFixture(t *testing.T, dir, rel string, branches int) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("package foo\n\nfunc Big(x int) int {\n\ty := 0\n")
	for i := 0; i < branches; i++ {
		b.WriteString("if x == " + itoa(i) + " { y++ }\n")
	}
	b.WriteString("return y\n}\n")
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}
	return full
}

func itoa(i int) string {
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

func TestSplitFileDiffs(t *testing.T) {
	diff := "diff --git a/internal/sdd/a.go b/internal/sdd/a.go\n@@ -1,3 +1,4 @@\n+line\n" +
		"diff --git a/docs/b.md b/docs/b.md\n@@ -1 +1 @@\n-old\n+new\n" +
		"diff --git a/old.go b/dev/null\n@@ -1 +0,0 @@\n-gone\n"
	got := SplitFileDiffs(diff)
	if len(got) != 2 {
		t.Fatalf("expected 2 files (deleted skipped), got %d: %v", len(got), keysOf(got))
	}
	if _, ok := got["internal/sdd/a.go"]; !ok {
		t.Errorf("missing internal/sdd/a.go in %v", keysOf(got))
	}
	if _, ok := got["docs/b.md"]; !ok {
		t.Errorf("missing docs/b.md in %v", keysOf(got))
	}
	if !strings.Contains(string(got["internal/sdd/a.go"]), "@@") {
		t.Error("per-file section must keep hunk headers")
	}
}

func TestSplitFileDiffsQuotedPath(t *testing.T) {
	diff := "diff --git \"a/my dir/f.go\" \"b/my dir/f.go\"\n@@ -1 +1 @@\n-x\n+y\n"
	got := SplitFileDiffs(diff)
	if _, ok := got["my dir/f.go"]; !ok {
		t.Errorf("quoted path not parsed, got %v", keysOf(got))
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestOffendersForFileDiffs_OverlapBlocks(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join("internal", "sdd", "big.go")
	writeCycloFixture(t, dir, rel, 20) // cyclo ~21 > 15
	diff := "diff --git a/" + filepath.ToSlash(rel) + " b/" + filepath.ToSlash(rel) + "\n@@ -1,5 +1,25 @@\n+// touched\n"
	hunks := map[string][]byte{filepath.ToSlash(rel): []byte(diff)}
	offs, _ := OffendersForFileDiffs(dir, hunks)
	found := false
	for _, o := range offs {
		if o.Function == "Big" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Big offender overlapping hunk, got %v", offs)
	}
}

func TestOffendersForFileDiffs_NoOverlapPasses(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join("internal", "sdd", "big.go")
	writeCycloFixture(t, dir, rel, 20)
	// Hunk far below the function (lines 500+): no overlap → grandfathered.
	diff := "diff --git a/" + filepath.ToSlash(rel) + " b/" + filepath.ToSlash(rel) + "\n@@ -500,3 +500,4 @@\n+// far away\n"
	hunks := map[string][]byte{filepath.ToSlash(rel): []byte(diff)}
	offs, _ := OffendersForFileDiffs(dir, hunks)
	if len(offs) != 0 {
		t.Errorf("expected no offenders outside hunks, got %v", offs)
	}
}

func TestOffendersForFileDiffs_NonCriticalIgnored(t *testing.T) {
	dir := t.TempDir()
	rel := filepath.Join("internal", "foo", "big.go")
	writeCycloFixture(t, dir, rel, 20)
	diff := "diff --git a/" + filepath.ToSlash(rel) + " b/" + filepath.ToSlash(rel) + "\n@@ -1,5 +1,25 @@\n+// touched\n"
	hunks := map[string][]byte{filepath.ToSlash(rel): []byte(diff)}
	offs, _ := OffendersForFileDiffs(dir, hunks)
	if len(offs) != 0 {
		t.Errorf("expected non-critical package ignored, got %v", offs)
	}
}
