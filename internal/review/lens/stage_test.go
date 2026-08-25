package lens

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/pipeline"
)

type okLens struct {
	id string
}

func (o *okLens) ID() string { return o.id }
func (o *okLens) Analyze(_ context.Context, _ LensInput) (LensResult, error) {
	return LensResult{
		LensID:   o.id,
		Findings: nil,
		Evidence: []string{"evidence for " + o.id},
	}, nil
}

type failLens struct {
	id string
}

func (f *failLens) ID() string { return f.id }
func (f *failLens) Analyze(_ context.Context, _ LensInput) (LensResult, error) {
	return LensResult{}, errors.New("injected failure")
}

func TestLensStage_Name(t *testing.T) {
	s := NewLensStage(&okLens{id: "resilience"}, LensInput{})
	if got := s.Name(); got != "resilience" {
		t.Errorf("Name() = %q, want resilience", got)
	}
	empty := &LensStage{lens: nil}
	if got := empty.Name(); got != "lens:unknown" {
		t.Errorf("Name() nil lens = %q, want lens:unknown", got)
	}
}

func TestLensStage_ExecuteSuccess(t *testing.T) {
	s := NewLensStage(&okLens{id: "readability"}, LensInput{Repo: "repo-root"})
	if err := s.Execute(context.Background(), &model.ReviewState{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result := s.Result()
	if result == nil {
		t.Fatal("Result() is nil after success")
	}
	if result.LensID != "readability" {
		t.Errorf("LensID = %q, want readability", result.LensID)
	}
	if result.ResultHash == "" {
		t.Error("ResultHash should be populated after Execute")
	}
	if !strings.HasPrefix(result.ResultHash, "sha256:") {
		t.Errorf("ResultHash = %q, want sha256: prefix", result.ResultHash)
	}
}

func TestLensStage_ExecuteFailure(t *testing.T) {
	s := NewLensStage(&failLens{id: "readability"}, LensInput{})
	err := s.Execute(context.Background(), &model.ReviewState{})
	if err == nil {
		t.Fatal("Execute should fail for failing lens")
	}
	if !strings.Contains(err.Error(), "readability") {
		t.Errorf("error %q should mention lens id", err.Error())
	}
	if s.Result() != nil {
		t.Error("Result() should stay nil after failure")
	}
}

func TestLensStage_SequentialRollback(t *testing.T) {
	// Verify pipeline executes LensStages sequentially and stops at failure
	// with reverse rollback (later stages not run).
	s1 := NewLensStage(&okLens{id: "risk"}, LensInput{})
	s2 := NewLensStage(&failLens{id: "resilience"}, LensInput{})
	s3 := NewLensStage(&okLens{id: "readability"}, LensInput{})
	p := pipeline.New(s1, s2, s3)
	err := p.Execute(context.Background(), &model.ReviewState{})
	if err == nil {
		t.Fatal("pipeline should fail on s2")
	}
	if s1.Result() == nil {
		t.Error("s1 should have executed before failure")
	}
	if s2.Result() != nil {
		t.Error("failing stage should not cache result")
	}
	if s3.Result() != nil {
		t.Error("s3 must not execute after s2 failure")
	}
	// Rollback is no-op but must not error; pipeline contract is reverse rollback.
	if err := s1.Rollback(context.Background(), &model.ReviewState{}); err != nil {
		t.Errorf("Rollback: %v", err)
	}
}

func TestLensStage_NoDAG(t *testing.T) {
	// Guard: lens package must not import planner/graph.go or DAG scheduler.
	// Use Go parser to inspect actual imports, not string literals in tests.
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, "\"")
				if strings.Contains(path, "internal/planner") {
					t.Errorf("%s imports %s — forbidden for S1 sequential pipeline", fname, path)
				}
			}
		}
	}
}

func TestLensStage_ImplementsPipelineStage(t *testing.T) {
	var _ pipeline.Stage = (*LensStage)(nil)
}
