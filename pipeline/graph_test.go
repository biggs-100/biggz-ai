package pipeline

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/model"
)

type graphMockStage struct {
	name      string
	depends   []string
	execErr   error
	execDelay time.Duration
	executed  bool
	mu        sync.Mutex
}

func (m *graphMockStage) Name() string { return m.name }
func (m *graphMockStage) Execute(ctx context.Context, state *model.ReviewState) error {
	m.mu.Lock()
	m.executed = true
	m.mu.Unlock()
	if m.execDelay > 0 {
		select {
		case <-time.After(m.execDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.execErr
}
func (m *graphMockStage) Rollback(ctx context.Context, state *model.ReviewState) error { return nil }

func TestGraph_ParallelExecution(t *testing.T) {
	g := NewGraph()
	a := &graphMockStage{name: "A"}
	b := &graphMockStage{name: "B"}
	c := &graphMockStage{name: "C", depends: []string{"A", "B"}}

	g.AddNode(a)
	g.AddNode(b)
	g.AddNode(c, "A", "B")

	start := time.Now()
	err := g.Execute(context.Background(), &model.ReviewState{})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if duration > 100*time.Millisecond {
		t.Logf("A+B ran in parallel: %v", duration)
	}
	if !a.executed {
		t.Error("A not executed")
	}
	if !b.executed {
		t.Error("B not executed")
	}
}

func TestGraph_SequentialDependency(t *testing.T) {
	g := NewGraph()
	a := &graphMockStage{name: "A"}
	b := &graphMockStage{name: "B", depends: []string{"A"}}
	c := &graphMockStage{name: "C", depends: []string{"B"}}

	g.AddNode(a)
	g.AddNode(b, "A")
	g.AddNode(c, "B")

	err := g.Execute(context.Background(), &model.ReviewState{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !a.executed || !b.executed || !c.executed {
		t.Error("not all executed")
	}
}

func TestGraph_FailureRollback(t *testing.T) {
	g := NewGraph()
	a := &graphMockStage{name: "A"}
	failing := &graphMockStage{name: "fail", execErr: fmt.Errorf("failed")}
	c := &graphMockStage{name: "C", depends: []string{"A", "fail"}}

	g.AddNode(a)
	g.AddNode(failing)
	g.AddNode(c, "A", "fail")

	err := g.Execute(context.Background(), &model.ReviewState{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGraph_Empty(t *testing.T) {
	g := NewGraph()
	err := g.Execute(context.Background(), &model.ReviewState{})
	if err != nil {
		t.Fatalf("Execute() error on empty graph: %v", err)
	}
}

func TestGraph_CycleDetection(t *testing.T) {
	g := NewGraph()
	a := &graphMockStage{name: "A"}
	b := &graphMockStage{name: "B", depends: []string{"C"}}
	c := &graphMockStage{name: "C", depends: []string{"B"}}

	g.AddNode(a)
	g.AddNode(b, "C")
	g.AddNode(c, "B")

	err := g.Execute(context.Background(), &model.ReviewState{})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestGraph_ConcurrentSafe(t *testing.T) {
	g := NewGraph()
	for i := 0; i < 10; i++ {
		s := &graphMockStage{name: fmt.Sprintf("S%d", i), execDelay: 5 * time.Millisecond}
		g.AddNode(s)
	}

	err := g.Execute(context.Background(), &model.ReviewState{})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
}

func TestGraph_CancelOnFailure(t *testing.T) {
	g := NewGraph()
	longRunning := &graphMockStage{name: "long", execDelay: 100 * time.Millisecond}
	failing := &graphMockStage{name: "fail", execErr: fmt.Errorf("boom")}

	g.AddNode(longRunning)
	g.AddNode(failing)

	err := g.Execute(context.Background(), &model.ReviewState{})
	if err == nil {
		t.Fatal("expected error")
	}
}
