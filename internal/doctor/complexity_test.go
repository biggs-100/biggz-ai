package doctor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComplexity_WarnTable(t *testing.T) {
	// Create temp dir with a critical package file that has high complexity
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "review")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package review
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
`
	if err := os.WriteFile(filepath.Join(pkg, "high.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Need to run check with custom roots pointing to temp dir
	// But our ComplexityCheck currently uses roots relative to repo, not temp dir with same package structure.
	// Instead we test via creating file under actual repo path? Simpler: directly test scan logic via custom roots.
	// Use temp dir as root but need to change criticalRoots handling: we'll use NewComplexityCheckWithCustom
	c := NewComplexityCheckWithCustom([]string{filepath.Join(dir, "internal", "review")}, 5*time.Second)
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Errorf("expected warn for high complexity, got %v msg=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "Foo") {
		t.Errorf("message should contain Foo, got %q", res.Message)
	}
	if res.Severity != SeverityWarning {
		t.Errorf("severity should be WARNING, got %q", res.Severity)
	}
}

func TestComplexity_PassZero(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "sdd")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	// Simple low complexity file
	content := `package sdd
func Low(){ x:=1; y:=2; _=x+y }
`
	if err := os.WriteFile(filepath.Join(pkg, "low.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewComplexityCheckWithCustom([]string{pkg}, 5*time.Second)
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Errorf("expected pass for low complexity, got %v msg=%q err=%q", res.Status, res.Message, res.Error)
	}
	if !strings.Contains(res.Message, "0 violations") {
		t.Errorf("message should contain 0 violations, got %q", res.Message)
	}
	if res.Severity != SeverityInfo {
		t.Errorf("severity should be INFO, got %q", res.Severity)
	}
}

func TestComplexity_TestIsolation(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "verification")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	// Only test file has high complexity, should be informational not blocking -> pass
	content := `package verification
func TestFoo(a int) {
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
	if err := os.WriteFile(filepath.Join(pkg, "foo_test.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewComplexityCheckWithCustom([]string{pkg}, 5*time.Second)
	res := c.Run(context.Background())
	// Should be pass (test violations informational only)
	if res.Status != StatusPass {
		t.Errorf("expected pass for test-only violation, got %v msg=%q", res.Status, res.Message)
	}
	details, ok := res.Details.(ComplexityDetails)
	if !ok {
		t.Fatalf("details not ComplexityDetails: %T", res.Details)
	}
	if len(details.Offenders) != 0 {
		t.Errorf("offenders should be 0 for test-only, got %d", len(details.Offenders))
	}
	if len(details.TestOffenders) == 0 {
		t.Errorf("testOffenders should contain informational violation, got %d", len(details.TestOffenders))
	}
}

func TestComplexity_JSONOffenders(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "review")
	if err := os.MkdirAll(pkg, 0755); err != nil {
		t.Fatal(err)
	}
	content := `package review
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
	if err := os.WriteFile(filepath.Join(pkg, "high.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewComplexityCheckWithCustom([]string{pkg}, 5*time.Second)
	res := c.Run(context.Background())
	// Build report with this single result to simulate doctor --json
	report := &Report{Warning: []*Result{res}}
	if res.Status == StatusPass {
		report = &Report{Info: []*Result{res}}
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	// Check that JSON contains complexity entry with status and details.offenders
	jsonStr := string(data)
	if !strings.Contains(jsonStr, "complexity") {
		t.Errorf("JSON should contain complexity ID, got %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "offenders") {
		t.Errorf("JSON should contain offenders, got %s", jsonStr)
	}
	// Verify Details contains function records
	details, ok := res.Details.(ComplexityDetails)
	if !ok {
		t.Fatalf("details type %T", res.Details)
	}
	if len(details.Offenders) == 0 {
		t.Error("expected offenders in details")
	} else {
		o := details.Offenders[0]
		if o.File == "" || o.Function == "" || o.Line == 0 {
			t.Errorf("offender missing fields: %+v", o)
		}
		if o.Cyclomatic <= 15 && o.Cognitive <= 20 {
			t.Errorf("offender should exceed threshold, got %+v", o)
		}
	}
}

func TestComplexity_PanicIsolation(t *testing.T) {
	// Simulate panic in ComplexityCheck via custom that panics; ensure Runner isolates
	// Use a testCheck that panics as second check, but we also test ComplexityCheck's own panic handling.
	// For ComplexityCheck, we test internal recover by causing panic via nil roots? Instead test Runner with panic check.
	runner := &Runner{
		Checks: []Check{
			NewComplexityCheckWithCustom([]string{t.TempDir()}, 2*time.Second),
			&testCheck{id: "panic-check", panic: true},
			NewComplexityCheckWithCustom([]string{t.TempDir()}, 2*time.Second),
		},
	}
	report := runner.RunAll(context.Background())
	// Should have 3 results, panic one in Critical, others in Info/Warning
	if len(report.All()) != 3 {
		t.Errorf("expected 3 results, got %d", len(report.All()))
	}
	// Complexity checks should still be present
	foundComplexity := 0
	for _, r := range report.All() {
		if r.ID == ComplexityCheckID {
			foundComplexity++
		}
	}
	if foundComplexity != 2 {
		t.Errorf("expected 2 complexity results, got %d", foundComplexity)
	}
	// Panicked check should be recorded
	if len(report.Critical) == 0 {
		t.Errorf("expected critical for panicked check")
	}
}

func TestComplexity_TimeoutWarn(t *testing.T) {
	// Very short timeout should cause warn, not fail
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "review")
	os.MkdirAll(pkg, 0755)
	// Create many files to slow scan? Instead use timeout 1ns to force timeout
	c := NewComplexityCheckWithCustom([]string{pkg}, 1*time.Nanosecond)
	// Add a file so scan has work
	os.WriteFile(filepath.Join(pkg, "a.go"), []byte("package p\nfunc Foo(){}\n"), 0644)
	// Give scheduler a chance to timeout
	time.Sleep(2 * time.Millisecond)
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		// Could be pass if scan completed before timeout (race). Allow either pass or warn but never Critical fail
		if res.Status == StatusFail && res.Severity == SeverityCritical {
			t.Errorf("timeout should yield warn, not critical fail, got %v %q", res.Status, res.Severity)
		}
	}
	if res.Severity == SeverityCritical {
		t.Errorf("timeout should not be CRITICAL, got %q", res.Severity)
	}
}
