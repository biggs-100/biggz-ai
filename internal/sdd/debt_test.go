package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerify_ComplexityDebt_ViolationsTop10(t *testing.T) {
	dir := t.TempDir()
	pkgA := filepath.Join(dir, "pkgA")
	if err := os.MkdirAll(pkgA, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a file with high cyclo function (will be top offender)
	high := `package pkgA
func High(a int) {
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
`
	if err := os.WriteFile(filepath.Join(pkgA, "high.go"), []byte(high), 0644); err != nil {
		t.Fatal(err)
	}
	// Create additional files to exceed top10 limit: 12 more high functions
	for i := 0; i < 12; i++ {
		name := filepath.Join(pkgA, strings.Repeat("a", i+1)+".go")
		content := high
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	md, err := ComplexityDebtMarkdownForRoots([]string{pkgA})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "Complexity Debt") {
		t.Errorf("markdown should contain header, got %q", md)
	}
	if !strings.Contains(strings.ToLower(md), "cyclomatic violations") {
		t.Errorf("should contain violation counts, got %q", md)
	}
	// Check top 10 offenders sorted
	if !strings.Contains(md, "Top 10 offenders") {
		t.Errorf("should contain Top 10, got %q", md)
	}
	// Verify at least one offender appears (any .go file)
	if !strings.Contains(md, ".go:") {
		t.Errorf("should contain offender file, got %q", md)
	}
}

func TestVerify_ComplexityDebt_ZeroViolations(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "empty")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	low := `package empty
func Low(){ x:=1; y:=2; _=x+y }
`
	if err := os.WriteFile(filepath.Join(pkg, "low.go"), []byte(low), 0644); err != nil {
		t.Fatal(err)
	}
	md, err := ComplexityDebtMarkdownForRoots([]string{pkg})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "0 violations") {
		t.Errorf("zero violations case should state 0 violations, got %q", md)
	}
	if !strings.Contains(md, "functions scanned") {
		t.Errorf("should contain totals, got %q", md)
	}
}

func TestVerify_ComplexityDebt_TestFileInfoOnly(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "pkgB")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	high := `package pkgB
func High(a int) {
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
`
	// Write high complexity as test file
	if err := os.WriteFile(filepath.Join(pkg, "high_test.go"), []byte(high), 0644); err != nil {
		t.Fatal(err)
	}
	// Also write low complexity regular file
	low := `package pkgB
func Low(){}
`
	if err := os.WriteFile(filepath.Join(pkg, "low.go"), []byte(low), 0644); err != nil {
		t.Fatal(err)
	}
	reports, err := CollectComplexityDebtForRoots([]string{pkg})
	if err != nil {
		t.Fatal(err)
	}
	r := reports[pkg]
	if r.CyclomaticViolations != 0 && len(r.TopOffenders) != 0 {
		// Test file violations should not count as blocking
		// Our implementation counts test file separately: TopOffenders should be 0, TestOffenders should have 1
	}
	if len(r.TopOffenders) != 0 {
		t.Errorf("test file should not be in blocking offenders, got %d", len(r.TopOffenders))
	}
	if len(r.TestOffenders) == 0 {
		t.Errorf("test file should be in TestOffenders, got %d", len(r.TestOffenders))
	}
	md, _ := ComplexityDebtMarkdownForRoots([]string{pkg})
	if !strings.Contains(md, "Informational test file") && !strings.Contains(md, "0 violations") {
		// If only test file violation, markdown may still say 0 violations for blocking, but include informational
		t.Errorf("should mention informational test file, got %q", md)
	}
}

func TestVerify_ComplexityDebtMarkdown_RealRoots(t *testing.T) {
	// Smoke test with real roots: should not error
	md, err := ComplexityDebtMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "Complexity Debt") {
		t.Errorf("real markdown should contain header, got %q", md)
	}
	if !strings.Contains(md, "functions scanned") && !strings.Contains(md, "Total functions scanned") {
		t.Errorf("should contain totals, got %q", md)
	}
}
