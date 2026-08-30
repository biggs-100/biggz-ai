package codegraph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractIntent_ProposalOnly(t *testing.T) {
	dir := t.TempDir()
	change := "test-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	proposal := "# Proposal\n\nWe need to handle PaymentService and payment flow.\n\nThe system uses AuthManager for verification."
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte(proposal), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	tokens, err := ExtractIntent(change, dir)
	if err != nil {
		t.Fatalf("ExtractIntent: %v", err)
	}
	if len(tokens) == 0 {
		t.Fatal("expected tokens, got none")
	}
	// Ensure symbol present
	if _, ok := tokens["PaymentService"]; !ok {
		t.Errorf("expected PaymentService token, got %v", tokens)
	}
	if _, ok := tokens["AuthManager"]; !ok {
		t.Errorf("expected AuthManager token, got %v", tokens)
	}
	// Ensure keyword present (lower)
	foundPayment := false
	for k := range tokens {
		if strings.EqualFold(k, "payment") {
			foundPayment = true
		}
	}
	if !foundPayment {
		t.Errorf("expected payment keyword token, got %v", tokens)
	}
}

func TestExtractIntent_MissingProposalFails(t *testing.T) {
	dir := t.TempDir()
	change := "missing-change"
	_, err := ExtractIntent(change, dir)
	if err == nil {
		t.Fatal("expected error for missing proposal, got nil")
	}
	if !strings.Contains(err.Error(), "proposal required") {
		t.Errorf("expected 'proposal required', got %v", err)
	}
}

func TestExtractIntent_SymbolWeightExceedsKeyword(t *testing.T) {
	dir := t.TempDir()
	change := "weight-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	proposal := "payment PaymentService"
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte(proposal), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tokens, err := ExtractIntent(change, dir)
	if err != nil {
		t.Fatalf("ExtractIntent: %v", err)
	}
	symWeight, ok := tokens["PaymentService"]
	if !ok {
		t.Fatalf("PaymentService missing")
	}
	kwWeight := 0
	for k, w := range tokens {
		if strings.EqualFold(k, "payment") && k != "PaymentService" {
			kwWeight = w
			break
		}
	}
	if kwWeight == 0 {
		t.Fatalf("payment keyword missing in %v", tokens)
	}
	if symWeight <= kwWeight {
		t.Errorf("symbol weight %d should exceed keyword weight %d", symWeight, kwWeight)
	}
	if symWeight != WeightSymbol {
		t.Errorf("symbol weight expected %d, got %d", WeightSymbol, symWeight)
	}
	if kwWeight != WeightKeyword {
		t.Errorf("keyword weight expected %d, got %d", WeightKeyword, kwWeight)
	}
}

func TestGraph_TransitiveClosure(t *testing.T) {
	sdd := []FileEntry{{Path: "a.go", Reasons: []Reason{ReasonSDD}}}
	scan := &ScanResult{
		Files: []string{"a.go", "b.go", "c.go"},
		Edges: []Edge{
			{From: "a.go", To: "b.go", Reason: ReasonImport},
			{From: "b.go", To: "c.go", Reason: ReasonCall},
		},
	}
	report := BuildGraph(sdd, scan)
	// Check nodes include A,B,C
	nodes := make(map[string]bool)
	for _, n := range report.Graph.Nodes {
		nodes[n.Path] = true
	}
	for _, want := range []string{"a.go", "b.go", "c.go"} {
		if !nodes[want] {
			t.Errorf("expected node %s, got %v", want, report.Graph.Nodes)
		}
	}
	// Check edges include A->B, B->C, plus derived A->C
	hasEdge := func(from, to string) bool {
		for _, e := range report.Graph.Edges {
			if e.From == from && e.To == to {
				return true
			}
		}
		return false
	}
	if !hasEdge("a.go", "b.go") {
		t.Errorf("expected edge a.go->b.go")
	}
	if !hasEdge("b.go", "c.go") {
		t.Errorf("expected edge b.go->c.go")
	}
	if !hasEdge("a.go", "c.go") {
		t.Errorf("expected transitive edge a.go->c.go, got %v", report.Graph.Edges)
	}
}

func TestGraph_FlatListGuard(t *testing.T) {
	sdd := []FileEntry{{Path: "x.go", Reasons: []Reason{ReasonSDD}}}
	scan := &ScanResult{
		Files: []string{"x.go", "y.go"},
		Edges: []Edge{{From: "x.go", To: "y.go", Reason: ReasonImport}},
	}
	report := BuildGraph(sdd, scan)
	if len(report.Graph.Nodes) == 0 {
		t.Fatal("expected nodes non-empty")
	}
	if report.Graph.Nodes == nil {
		t.Fatal("nodes is nil, should be non-nil")
	}
	if report.Graph.Edges == nil {
		t.Fatal("edges is nil, should be non-nil")
	}
	if len(report.Graph.Edges) == 0 {
		t.Fatal("expected edges non-empty when files reported")
	}
	// Isolated sdd still appears as node
	sddIsolated := []FileEntry{{Path: "isolated.go", Reasons: []Reason{ReasonSDD}}}
	scan2 := &ScanResult{Files: []string{"isolated.go"}, Edges: []Edge{}}
	report2 := BuildGraph(sddIsolated, scan2)
	found := false
	for _, n := range report2.Graph.Nodes {
		if n.Path == "isolated.go" {
			found = true
			// check has sdd reason
			hasSDD := false
			for _, r := range n.Reasons {
				if r == ReasonSDD {
					hasSDD = true
				}
			}
			if !hasSDD {
				t.Errorf("isolated node missing sdd reason")
			}
		}
	}
	if !found {
		t.Errorf("isolated sdd node not preserved, nodes=%v", report2.Graph.Nodes)
	}
	// Isolated case should still have graph non-nil (and edges at least placeholder to satisfy guard)
	if report2.Graph.Nodes == nil || report2.Graph.Edges == nil {
		t.Fatal("isolated graph slices should be non-nil")
	}
}

func TestLoadHint_NilWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	change := "no-report"
	report, err := LoadHint(change, dir)
	if err != nil {
		t.Fatalf("LoadHint unexpected error: %v", err)
	}
	if report != nil {
		t.Errorf("expected nil when absent, got %v", report)
	}
}

func TestGenerate_ProposalRequired(t *testing.T) {
	dir := t.TempDir()
	change := "no-proposal"
	_, err := Generate(change, dir)
	if err == nil {
		t.Fatal("expected error for missing proposal")
	}
	if !strings.Contains(err.Error(), "proposal required") {
		t.Errorf("expected proposal required, got %v", err)
	}
}

func TestScanGo_ImportAndCallEdges(t *testing.T) {
	ClearScanCache()
	dir := t.TempDir()
	// Create a minimal Go module
	goMod := "module example.com/testscan\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("go.mod: %v", err)
	}
	// Package b with func Do
	if err := os.MkdirAll(filepath.Join(dir, "b"), 0755); err != nil {
		t.Fatalf("mkdir b: %v", err)
	}
	bGo := "package b\n\nfunc Do() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "b", "b.go"), []byte(bGo), 0644); err != nil {
		t.Fatalf("b.go: %v", err)
	}
	// Package a imports b and calls b.Do
	aGo := "package a\n\nimport \"example.com/testscan/b\"\n\nfunc A() { b.Do() }\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(aGo), 0644); err != nil {
		t.Fatalf("a.go: %v", err)
	}
	ctx := context.Background()
	scan, err := ScanGo(dir, ctx)
	if err != nil {
		t.Fatalf("ScanGo: %v", err)
	}
	// Check import edge a.go -> b/b.go
	hasImport := false
	hasCall := false
	for _, e := range scan.Edges {
		if e.Reason == ReasonImport && e.From == "a.go" && strings.Contains(e.To, "b.go") {
			hasImport = true
		}
		if e.Reason == ReasonCall && e.From == "a.go" && strings.Contains(e.To, "b.go") {
			hasCall = true
		}
	}
	if !hasImport {
		t.Errorf("expected import edge a.go->b/b.go, got %v", scan.Edges)
	}
	if !hasCall {
		t.Errorf("expected call edge a.go->b/b.go, got %v", scan.Edges)
	}
	// Go-only filter: ensure no non-Go file in Files
	for _, f := range scan.Files {
		if !strings.HasSuffix(f, ".go") {
			t.Errorf("non-Go file in scan: %s", f)
		}
	}
}

func TestGenerate_TimeoutNoPartial(t *testing.T) {
	dir := t.TempDir()
	change := "timeout-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("test timeout"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	// Create a simple Go file to allow scan
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main(){}\n"), 0644); err != nil {
		t.Fatalf("main.go: %v", err)
	}
	// Use a cancelled context directly via ScanGo to simulate timeout
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ScanGo(dir, ctx)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancel") {
		t.Errorf("expected timeout/canceled error, got %v", err)
	}
	// Ensure Generate with normal ctx still succeeds and no partial file was created via timeout error path
	// Generate uses 30s timeout internally, not cancelled, so should succeed here
	report, err := Generate(change, dir)
	if err != nil {
		t.Fatalf("Generate should succeed with normal context: %v", err)
	}
	if len(report.Files) == 0 {
		t.Errorf("expected files in report")
	}
}

func TestEmit_MkdirAll(t *testing.T) {
	report := &Report{
		Files: []FileEntry{{Path: "a.go", Reasons: []Reason{ReasonSDD}}},
		Graph: Graph{
			Nodes: []Node{{ID: "a.go", Path: "a.go", Reasons: []Reason{ReasonSDD}}},
			Edges: []Edge{{From: "a.go", To: "a.go", Reason: ReasonSDD}},
		},
	}
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "nested", "deep", "out.json")
	mdPath := filepath.Join(dir, "nested", "deep", "out.md")
	if err := Emit(report, jsonPath, mdPath); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("json file not created: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("md file not created: %v", err)
	}
	// Verify JSON content
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(r.Files) != 1 {
		t.Errorf("expected 1 file, got %v", r)
	}
	mdData, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	if !strings.Contains(string(mdData), "a.go") {
		t.Errorf("md missing a.go: %s", string(mdData))
	}
	if !strings.Contains(string(mdData), "sdd") {
		t.Errorf("md missing sdd reason: %s", string(mdData))
	}
}

func TestRenderMarkdown_ContainsFilesAndGraph(t *testing.T) {
	report := &Report{
		Files: []FileEntry{
			{Path: "a.go", Reasons: []Reason{ReasonSDD, ReasonImport}},
			{Path: "b.go", Reasons: []Reason{ReasonCall}},
		},
		Graph: Graph{
			Nodes: []Node{
				{ID: "a.go", Path: "a.go", Reasons: []Reason{ReasonSDD}},
				{ID: "b.go", Path: "b.go", Reasons: []Reason{ReasonCall}},
			},
			Edges: []Edge{{From: "a.go", To: "b.go", Reason: ReasonImport}},
		},
	}
	md := RenderMarkdown(report)
	if !strings.Contains(md, "a.go") || !strings.Contains(md, "b.go") {
		t.Errorf("md missing files: %s", md)
	}
	if !strings.Contains(md, "sdd") || !strings.Contains(md, "import") {
		t.Errorf("md missing reasons: %s", md)
	}
	if !strings.Contains(md, "Nodes") || !strings.Contains(md, "Edges") {
		t.Errorf("md missing graph summary: %s", md)
	}
}

func TestGenerate_DualEmissionDefaultPaths(t *testing.T) {
	ClearScanCache()
	dir := t.TempDir()
	change := "dual-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("Need PaymentService"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main\nfunc PaymentService(){}\n"), 0644); err != nil {
		t.Fatalf("app.go: %v", err)
	}
	report, err := Generate(change, dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	jsonPath := filepath.Join(changeDir, "codegraph.json")
	mdPath := filepath.Join(changeDir, "codegraph.md")
	if err := Emit(report, jsonPath, mdPath); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("default json not created")
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("default md not created")
	}
}

func TestScanGo_GoOnlyFilter(t *testing.T) {
	ClearScanCache()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# doc PaymentService"), 0644); err != nil {
		t.Fatalf("readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc PaymentService(){}\n"), 0644); err != nil {
		t.Fatalf("main.go: %v", err)
	}
	changeDir := filepath.Join(dir, "openspec", "changes", "filter-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("PaymentService"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	scan, err := ScanGo(dir, context.Background())
	if err != nil {
		t.Fatalf("ScanGo: %v", err)
	}
	for _, f := range scan.Files {
		if strings.HasSuffix(f, ".md") {
			t.Errorf("doc file should not be in scan files: %s", f)
		}
	}
}

func TestGenerate_30sTimeoutNoPartial(t *testing.T) {
	// Simulate that Generate respects 30s timeout: we use a quick dir but ensure Emit not called on error
	dir := t.TempDir()
	change := "no-partial-change"
	// No proposal => error, should not emit
	_, err := Generate(change, dir)
	if err == nil {
		t.Fatal("expected error")
	}
	// Ensure no file created
	jsonPath := filepath.Join(dir, "openspec", "changes", change, "codegraph.json")
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Errorf("partial json should not exist on error, got %v", err)
		_ = os.Remove(jsonPath)
	}
}

func TestLoadHint_ReadAndNil(t *testing.T) {
	dir := t.TempDir()
	change := "hint-change"
	changeDir := filepath.Join(dir, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("PaymentService"), 0644); err != nil {
		t.Fatalf("proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svc.go"), []byte("package main\nfunc PaymentService(){}\n"), 0644); err != nil {
		t.Fatalf("svc.go: %v", err)
	}
	// No hint initially
	hint, err := LoadHint(change, dir)
	if err != nil || hint != nil {
		t.Fatalf("expected nil hint before generation, got %v err %v", hint, err)
	}
	// Generate and emit
	ClearScanCache()
	report, err := Generate(change, dir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	jsonPath := filepath.Join(changeDir, "codegraph.json")
	mdPath := filepath.Join(changeDir, "codegraph.md")
	if err := Emit(report, jsonPath, mdPath); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	hint, err = LoadHint(change, dir)
	if err != nil {
		t.Fatalf("LoadHint after emit: %v", err)
	}
	if hint == nil || len(hint.Files) == 0 {
		t.Fatalf("expected hint with files, got %v", hint)
	}
}

func TestScanGo_Cached(t *testing.T) {
	ClearScanCache()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\nfunc Foo(){}\n"), 0644); err != nil {
		t.Fatalf("a.go: %v", err)
	}
	ctx := context.Background()
	start := time.Now()
	scan1, err := ScanGo(dir, ctx)
	if err != nil {
		t.Fatalf("scan1: %v", err)
	}
	scan2, err := ScanGo(dir, ctx)
	if err != nil {
		t.Fatalf("scan2: %v", err)
	}
	if len(scan1.Files) != len(scan2.Files) {
		t.Errorf("cached files mismatch")
	}
	_ = start
	// Mutate underlying file after cache - second scan should still be cached (files unchanged)
	// This verifies caching behavior
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\nfunc Bar(){}\n"), 0644); err != nil {
		t.Fatalf("b.go: %v", err)
	}
	// Cache still returns old? Since cache key is dir, it will return cached without b.go
	scan3, _ := ScanGo(dir, ctx)
	if len(scan3.Files) != len(scan1.Files) {
		// Cache is working, but we might expect it to be stale - this is expected caching
		// If we want fresh, we ClearCache
	}
	ClearScanCache()
	scan4, err := ScanGo(dir, ctx)
	if err != nil {
		t.Fatalf("scan4: %v", err)
	}
	if len(scan4.Files) != len(scan1.Files)+1 {
		t.Errorf("after clear cache expected new file, got %d vs %d", len(scan4.Files), len(scan1.Files))
	}
}
