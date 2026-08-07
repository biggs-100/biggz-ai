package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

// errStageFail is the sentinel error returned by mock stages that are
// configured to fail.
var errStageFail = errors.New("stage failed")

// mockStage records execution and rollback calls for testing pipeline
// orchestration. It uses a shared callLog to capture interleaved call
// order across stages, and per-stage counters for quick assertions.
type mockStage struct {
	name     string
	execErr  error
	callLog  *[]string
	executed int
	rolled   int
}

func (m *mockStage) Name() string { return m.name }

func (m *mockStage) Execute(_ context.Context, _ *model.ReviewState) error {
	m.executed++
	if m.callLog != nil {
		*m.callLog = append(*m.callLog, "exec:"+m.name)
	}
	return m.execErr
}

func (m *mockStage) Rollback(_ context.Context, _ *model.ReviewState) error {
	m.rolled++
	if m.callLog != nil {
		*m.callLog = append(*m.callLog, "roll:"+m.name)
	}
	return nil
}

func TestPipeline_EmptyStages(t *testing.T) {
	p := New()
	state := model.NewReviewState(model.ReviewSubject{})
	err := p.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error for empty pipeline, got: %v", err)
	}
}

func TestPipeline_AllSucceed(t *testing.T) {
	var log []string
	a := &mockStage{name: "A", callLog: &log}
	b := &mockStage{name: "B", callLog: &log}
	c := &mockStage{name: "C", callLog: &log}

	p := New(a, b, c)
	state := model.NewReviewState(model.ReviewSubject{})
	err := p.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// All stages should have executed exactly once.
	if a.executed != 1 {
		t.Errorf("expected A executed once, got %d", a.executed)
	}
	if b.executed != 1 {
		t.Errorf("expected B executed once, got %d", b.executed)
	}
	if c.executed != 1 {
		t.Errorf("expected C executed once, got %d", c.executed)
	}

	// No rollback should be called when all succeed.
	if a.rolled != 0 {
		t.Errorf("expected no rollback for A, got %d", a.rolled)
	}
	if b.rolled != 0 {
		t.Errorf("expected no rollback for B, got %d", b.rolled)
	}
	if c.rolled != 0 {
		t.Errorf("expected no rollback for C, got %d", c.rolled)
	}

	// Verify execution order: A, B, C
	expectedOrder := []string{"exec:A", "exec:B", "exec:C"}
	for i, want := range expectedOrder {
		if i >= len(log) {
			t.Fatalf("expected call at index %d, got none", i)
		}
		if log[i] != want {
			t.Errorf("call at index %d: expected %q, got %q", i, want, log[i])
		}
	}
}

func TestPipeline_MiddleStageFails(t *testing.T) {
	var log []string
	a := &mockStage{name: "A", callLog: &log}
	b := &mockStage{name: "B", execErr: errStageFail, callLog: &log}
	c := &mockStage{name: "C", callLog: &log}

	p := New(a, b, c)
	state := model.NewReviewState(model.ReviewSubject{})
	err := p.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	// A and B should have executed; C should not.
	if a.executed != 1 {
		t.Errorf("expected A executed once, got %d", a.executed)
	}
	if b.executed != 1 {
		t.Errorf("expected B executed once, got %d", b.executed)
	}
	if c.executed != 0 {
		t.Errorf("expected C not executed, got %d", c.executed)
	}

	// Rollback should be called on B (the failed stage) and A (prior).
	// C should NOT be rolled back — it never executed.
	if b.rolled != 1 {
		t.Errorf("expected B rollback called once, got %d", b.rolled)
	}
	if a.rolled != 1 {
		t.Errorf("expected A rollback called once, got %d", a.rolled)
	}
	if c.rolled != 0 {
		t.Errorf("expected C rollback not called, got %d", c.rolled)
	}

	// Error should mention the failing stage.
	if !errors.Is(err, errStageFail) {
		t.Errorf("expected error to wrap errStageFail, got: %v", err)
	}
}

func TestPipeline_RollbackOrder_ReverseCompletion(t *testing.T) {
	var log []string
	a := &mockStage{name: "A", callLog: &log}
	b := &mockStage{name: "B", execErr: errStageFail, callLog: &log}
	c := &mockStage{name: "C", callLog: &log}

	p := New(a, b, c)
	state := model.NewReviewState(model.ReviewSubject{})
	_ = p.Execute(context.Background(), state)

	// Execution order: A, B (fails).
	// Rollback order must be: B (failed stage first), then A (reverse completion).
	// C is never executed, so never rolled back.
	expectedOrder := []string{"exec:A", "exec:B", "roll:B", "roll:A"}
	if len(log) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d: %v", len(expectedOrder), len(log), log)
	}
	for i, want := range expectedOrder {
		if log[i] != want {
			t.Errorf("call at index %d: expected %q, got %q", i, want, log[i])
		}
	}
}

func TestPipeline_FirstStageFails(t *testing.T) {
	var log []string
	a := &mockStage{name: "A", execErr: errStageFail, callLog: &log}
	b := &mockStage{name: "B", callLog: &log}
	c := &mockStage{name: "C", callLog: &log}

	p := New(a, b, c)
	state := model.NewReviewState(model.ReviewSubject{})
	err := p.Execute(context.Background(), state)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	// Only A should have executed; B and C did not run.
	if a.executed != 1 {
		t.Errorf("expected A executed once, got %d", a.executed)
	}
	if b.executed != 0 {
		t.Errorf("expected B not executed, got %d", b.executed)
	}
	if c.executed != 0 {
		t.Errorf("expected C not executed, got %d", c.executed)
	}

	// A should still be rolled back (failed stage is rolled back).
	if a.rolled != 1 {
		t.Errorf("expected A rollback called once, got %d", a.rolled)
	}
	if b.rolled != 0 {
		t.Errorf("expected B rollback not called, got %d", b.rolled)
	}
	if c.rolled != 0 {
		t.Errorf("expected C rollback not called, got %d", c.rolled)
	}

	// Only the failed stage's rollback should appear.
	expectedLog := []string{"exec:A", "roll:A"}
	if len(log) != len(expectedLog) {
		t.Fatalf("expected %d calls, got %d: %v", len(expectedLog), len(log), log)
	}
	for i, want := range expectedLog {
		if log[i] != want {
			t.Errorf("call at index %d: expected %q, got %q", i, want, log[i])
		}
	}
}

func TestPipeline_SingleStage(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		a := &mockStage{name: "A"}
		p := New(a)
		state := model.NewReviewState(model.ReviewSubject{})
		err := p.Execute(context.Background(), state)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if a.executed != 1 {
			t.Errorf("expected A executed once, got %d", a.executed)
		}
		if a.rolled != 0 {
			t.Errorf("expected no rollback for A, got %d", a.rolled)
		}
	})

	t.Run("fails and rolls back", func(t *testing.T) {
		a := &mockStage{name: "A", execErr: errStageFail}
		p := New(a)
		state := model.NewReviewState(model.ReviewSubject{})
		err := p.Execute(context.Background(), state)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if a.executed != 1 {
			t.Errorf("expected A executed once, got %d", a.executed)
		}
		if a.rolled != 1 {
			t.Errorf("expected A rollback called once, got %d", a.rolled)
		}
	})
}
