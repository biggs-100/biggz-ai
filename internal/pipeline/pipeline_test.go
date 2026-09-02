package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeStep records calls for pipeline orchestration tests.
type fakeStep struct {
	name       string
	prepareErr error
	applyErr   error
	rollbackErr error

	mu            sync.Mutex
	prepareCalled int
	applyCalled   int
	rollbackCalled int
	rollbackOrder *[]string

	// for burst test
	burst int
}

func (f *fakeStep) Name() string { return f.name }

func (f *fakeStep) Prepare(_ context.Context) error {
	f.mu.Lock()
	f.prepareCalled++
	f.mu.Unlock()
	return f.prepareErr
}

func (f *fakeStep) Apply(_ context.Context, ch ProgressChan) error {
	f.mu.Lock()
	f.applyCalled++
	burst := f.burst
	f.mu.Unlock()
	if burst > 0 && ch != nil {
		for i := 0; i < burst; i++ {
			ch <- ProgressEvent{Step: f.name, Percent: i * 5, Message: "burst"}
		}
	}
	return f.applyErr
}

func (f *fakeStep) Rollback(_ context.Context) error {
	f.mu.Lock()
	f.rollbackCalled++
	if f.rollbackOrder != nil {
		*f.rollbackOrder = append(*f.rollbackOrder, f.name)
	}
	f.mu.Unlock()
	return f.rollbackErr
}

// idempotentStep ensures Rollback twice is safe.
type idempotentStep struct {
	name     string
	mu       sync.Mutex
	applied  bool
	rolled   int
}

func (s *idempotentStep) Name() string { return s.name }
func (s *idempotentStep) Prepare(_ context.Context) error { return nil }
func (s *idempotentStep) Apply(_ context.Context, ch ProgressChan) error {
	s.mu.Lock()
	s.applied = true
	s.mu.Unlock()
	if ch != nil {
		ch <- ProgressEvent{Step: s.name, Percent: 100, Message: "done"}
	}
	return nil
}
func (s *idempotentStep) Rollback(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rolled++
	if s.rolled > 1 {
		// second call must be no-op success
		return nil
	}
	s.applied = false
	return nil
}

func TestOrchestrator_PrepareBlocksApply(t *testing.T) {
	ctx := context.Background()
	a := &fakeStep{name: "A"}
	b := &fakeStep{name: "B", prepareErr: errors.New("bad")}
	c := &fakeStep{name: "C"}
	plan := NewPlan(a, b, c)
	orch := &Orchestrator{Policy: NoRollback}
	res, err := orch.Run(ctx, plan)
	if err == nil {
		t.Fatalf("expected prepare error, got nil")
	}
	if !strings.Contains(err.Error(), "B:") {
		t.Fatalf("error should wrap step name 'B: %%w', got %v", err)
	}
	if a.prepareCalled != 1 || b.prepareCalled != 1 {
		t.Fatalf("prepare called counts wrong a=%d b=%d", a.prepareCalled, b.prepareCalled)
	}
	if c.prepareCalled != 0 {
		t.Fatalf("C Prepare should not be called after B failure, got %d", c.prepareCalled)
	}
	if a.applyCalled != 0 || b.applyCalled != 0 || c.applyCalled != 0 {
		t.Fatalf("Apply must not execute when Prepare fails")
	}
	if res != nil && res.Success {
		t.Fatalf("result Success should be false on prepare failure")
	}
}

func TestOrchestrator_Success(t *testing.T) {
	ctx := context.Background()
	a := &fakeStep{name: "A"}
	b := &fakeStep{name: "B"}
	c := &fakeStep{name: "C"}
	plan := NewPlan(a, b, c)
	orch := &Orchestrator{Policy: NoRollback}
	res, err := orch.Run(ctx, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("expected success true, got %+v", res)
	}
	if len(res.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(res.Steps))
	}
	for _, s := range res.Steps {
		if !s.Applied {
			t.Fatalf("step %s should be Applied", s.Step)
		}
	}
}

func TestOrchestrator_ApplyErrorWrapped(t *testing.T) {
	ctx := context.Background()
	a := &fakeStep{name: "A"}
	b := &fakeStep{name: "B", applyErr: errors.New("boom")}
	plan := NewPlan(a, b)
	orch := &Orchestrator{Policy: NoRollback}
	res, err := orch.Run(ctx, plan)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "B:") {
		t.Fatalf("error should contain 'B: ', got %v", err)
	}
	if res.Success {
		t.Fatalf("Success should be false")
	}
	if !errors.Is(err, b.applyErr) && !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should wrap original, got %v", err)
	}
}

func TestOrchestrator_RollbackOrder(t *testing.T) {
	ctx := context.Background()
	var order []string
	a := &fakeStep{name: "A", rollbackOrder: &order}
	b := &fakeStep{name: "B", rollbackOrder: &order}
	c := &fakeStep{name: "C", applyErr: errors.New("fail"), rollbackOrder: &order}
	plan := NewPlan(a, b, c)
	orch := &Orchestrator{Policy: RollbackOnFailure}
	_, err := orch.Run(ctx, plan)
	if err == nil {
		t.Fatal("expected failure")
	}
	// C never applied, so no rollback for C
	if len(order) != 2 {
		t.Fatalf("expected 2 rollbacks, got %d: %v", len(order), order)
	}
	if order[0] != "B" || order[1] != "A" {
		t.Fatalf("expected rollback B->A, got %v", order)
	}
	for _, n := range order {
		if n == "C" {
			t.Fatalf("C should not be rolled back")
		}
	}
	// check that steps still record rollbackCalled
	if a.rollbackCalled != 1 || b.rollbackCalled != 1 || c.rollbackCalled != 0 {
		t.Fatalf("rollback counts a=%d b=%d c=%d", a.rollbackCalled, b.rollbackCalled, c.rollbackCalled)
	}
}

func TestOrchestrator_RollbackErrorAggregation(t *testing.T) {
	ctx := context.Background()
	a := &fakeStep{name: "A", rollbackErr: errors.New("rollback boom")}
	b := &fakeStep{name: "B", applyErr: errors.New("apply boom")}
	plan := NewPlan(a, b)
	orch := &Orchestrator{Policy: RollbackOnFailure}
	// Need a to be applied before b fails, so order A then B
	// But our plan order is A then B, B fails, so A should be rolled back
	// Put A first, B second failing
	plan2 := NewPlan(a, b)
	_ = plan
	res, err := orch.Run(ctx, plan2)
	if err == nil {
		t.Fatal("expected error")
	}
	if res.Error == nil {
		t.Fatal("ExecutionResult.Error should contain both errors")
	}
	msg := res.Error.Error()
	if !strings.Contains(msg, "apply boom") {
		t.Fatalf("should contain original apply error, got %v", msg)
	}
	if !strings.Contains(msg, "rollback boom") {
		t.Fatalf("should contain rollback error, got %v", msg)
	}
	if !strings.Contains(msg, "rollback A") {
		t.Fatalf("should contain rollback step name, got %v", msg)
	}
}

func TestOrchestrator_DoubleRollbackIdempotent(t *testing.T) {
	ctx := context.Background()
	s := &idempotentStep{name: "X"}
	plan := NewPlan(s)
	// first run success, then manual double rollback
	orch := &Orchestrator{Policy: NoRollback}
	_, err := orch.Run(ctx, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.Rollback(ctx); err != nil {
		t.Fatalf("first rollback failed: %v", err)
	}
	if err := s.Rollback(ctx); err != nil {
		t.Fatalf("second rollback should be idempotent, got %v", err)
	}
	if s.rolled != 2 {
		t.Fatalf("expected 2 rollback calls, got %d", s.rolled)
	}
	s.mu.Lock()
	if s.applied {
		s.mu.Unlock()
		t.Fatalf("after rollback, should be in pre-Apply state (applied=false)")
	}
	s.mu.Unlock()
}

func TestOrchestrator_BurstNoDrops(t *testing.T) {
	ctx := context.Background()
	ch := make(ProgressChan, 32)
	step := &fakeStep{name: "burst", burst: 20}
	plan := NewPlan(step)
	res, err := plan.Apply(ctx, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	// drain channel (already closed by Apply)
	var got []ProgressEvent
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 20 {
		t.Fatalf("expected 20 events, got %d", len(got))
	}
}

func TestOrchestrator_ChannelClosed(t *testing.T) {
	ctx := context.Background()
	ch := make(ProgressChan, 32)
	step := &fakeStep{name: "A"}
	plan := NewPlan(step)
	_, err := plan.Apply(ctx, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// channel must be closed: ranging should finish, and ok should be false
	_, ok := <-ch
	// ch already drained? Actually previous test drains, but here channel has zero events and is closed.
	// After Apply, channel closed, so receive should return ok=false immediately if no events buffered.
	// If there were events, they were drained; for this step, zero events, so we expect closed.
	if ok {
		t.Fatalf("expected channel closed, got ok=true")
	}
	// also test that Plan.Apply via Orchestrator internal channel closes (via stored ch)
	// use fakePlan to capture ch
	captured := make(chan ProgressChan, 1)
	captureStep := &capturePlan{steps: []Step{&fakeStep{name: "Z"}}, captured: captured}
	orch := &Orchestrator{Policy: NoRollback}
	_, err = orch.Run(ctx, captureStep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case ch2 := <-captured:
		// after Run, ch2 should be closed
		_, ok2 := <-ch2
		if ok2 {
			// could have buffered events, drain then check closed
			for range ch2 {
			}
			// after drain, next read should be closed
			_, ok3 := <-ch2
			if ok3 {
				t.Fatalf("captured channel should be closed after Run")
			}
		}
	default:
		t.Fatalf("failed to capture channel")
	}
}

// capturePlan is a test helper that captures the channel passed to Apply.
type capturePlan struct {
	steps    []Step
	captured chan ProgressChan
}

func (c *capturePlan) Prepare(_ context.Context) (*PlanPreview, error) {
	names := make([]string, len(c.steps))
	for i, s := range c.steps {
		names[i] = s.Name()
	}
	return &PlanPreview{Steps: names}, nil
}
func (c *capturePlan) Apply(ctx context.Context, ch ProgressChan) (*ExecutionResult, error) {
	select {
	case c.captured <- ch:
	default:
	}
	// delegate to simple loop without closing (let orchestrator close)
	var results []StepResult
	for _, s := range c.steps {
		if err := s.Apply(ctx, ch); err != nil {
			wrapped := errors.New(s.Name() + ": " + err.Error())
			results = append(results, StepResult{Step: s.Name(), Applied: false, Error: wrapped})
			return &ExecutionResult{Success: false, Steps: results, Error: wrapped}, wrapped
		}
		results = append(results, StepResult{Step: s.Name(), Applied: true})
	}
	return &ExecutionResult{Success: true, Steps: results}, nil
}
func (c *capturePlan) Steps() []Step { return c.steps }

func TestProgress_LosslessPublishComplete(t *testing.T) {
	p := NewProgress(1)
	// publish 5 events with buffer 1 but concurrent consumer ensures lossless
	// Use concurrent drain to avoid blocking
	done := make(chan []ProgressEvent, 1)
	go func() {
		var out []ProgressEvent
		for {
			ev, ok := p.NextMessage()
			if !ok {
				break
			}
			out = append(out, ev)
		}
		done <- out
	}()
	for i := 0; i < 5; i++ {
		p.Publish(ProgressEvent{Step: "test", Percent: i * 20, Message: "m"})
	}
	p.Complete()
	out := <-done
	if len(out) != 5 {
		t.Fatalf("expected 5 events, got %d", len(out))
	}
	// publish after complete should be no-op
	if p.Publish(ProgressEvent{Step: "after", Percent: 100}) {
		t.Fatalf("publish after complete should return false")
	}
	// double complete idempotent
	p.Complete()
}

func TestPlan_PrepareListsSteps(t *testing.T) {
	ctx := context.Background()
	a := &fakeStep{name: "alpha"}
	b := &fakeStep{name: "beta"}
	plan := NewPlan(a, b)
	preview, err := plan.Prepare(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(preview.Steps) != 2 || preview.Steps[0] != "alpha" || preview.Steps[1] != "beta" {
		t.Fatalf("unexpected preview %+v", preview)
	}
}
