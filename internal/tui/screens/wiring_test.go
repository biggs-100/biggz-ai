package screens

import (
	"context"
	"errors"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/pipeline"
	tea "github.com/charmbracelet/bubbletea"
)

// wizardFakePlan emits a fixed 0→100 event sequence on the provided
// ProgressChan(32) via Orchestrator.RunWithChan. When failAfter >= 0 it
// returns an error after that many events (rollback asserted separately).
type wizardFakePlan struct {
	failAfter  int
	rollbacked []string
}

func (p *wizardFakePlan) Prepare(_ context.Context) (*pipeline.PlanPreview, error) {
	return &pipeline.PlanPreview{Steps: []string{"fake"}}, nil
}

func (p *wizardFakePlan) Apply(_ context.Context, ch pipeline.ProgressChan) (*pipeline.ExecutionResult, error) {
	for i := 0; i < 10; i++ {
		if p.failAfter >= 0 && i == p.failAfter {
			return &pipeline.ExecutionResult{Success: false}, errors.New("fake apply failure")
		}
		ch <- pipeline.ProgressEvent{Step: "fake", Percent: i * 100 / 9, Message: "working"}
	}
	return &pipeline.ExecutionResult{Success: true}, nil
}

// drainProgress consumes ch exactly like the wizard Installing stage does:
// waitProgress → InstallingModel.Update until close yields progressDoneMsg.
func drainProgress(t *testing.T, ch pipeline.ProgressChan) InstallingModel {
	t.Helper()
	m := NewInstallingModel()
	count := 0
	for {
		msg := waitProgress(ch)()
		switch v := msg.(type) {
		case pipeline.ProgressEvent:
			um, _ := m.Update(v)
			m = um.(InstallingModel)
			count++
			if count > 64 {
				t.Fatalf("progress stream did not close (consumed %d events)", count)
			}
		case progressDoneMsg:
			um, _ := m.Update(v)
			m = um.(InstallingModel)
			if count != 10 {
				t.Fatalf("expected 10 lossless events, got %d", count)
			}
			return m
		default:
			t.Fatalf("unexpected message %T", msg)
		}
	}
}

// Task 3.3 RED: fake plan via RunWithChan(32) — 10 events 0→100 lossless,
// close→Complete, fail→error. No pipeline API change.
func TestWizardProgressWiring(t *testing.T) {
	ctx := context.Background()

	// Lossless feed: 10 events 0→100 through a cap-32 channel.
	t.Run("lossless 0→100", func(t *testing.T) {
		ch := make(pipeline.ProgressChan, 32)
		if cap(ch) != 32 {
			t.Fatalf("expected ProgressChan cap 32, got %d", cap(ch))
		}
		orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
		plan := &wizardFakePlan{failAfter: -1}
		type outcome struct {
			res *pipeline.ExecutionResult
			err error
		}
		done := make(chan outcome, 1)
		go func() {
			res, err := orch.RunWithChan(ctx, plan, ch)
			done <- outcome{res, err}
		}()
		m := drainProgress(t, ch)
		out := <-done
		if out.err != nil {
			t.Fatalf("RunWithChan failed: %v", out.err)
		}
		if !out.res.Success {
			t.Fatalf("expected success result")
		}
		if m.Count != 10 {
			t.Fatalf("expected Count 10, got %d", m.Count)
		}
		if m.Percent != 100 {
			t.Fatalf("expected final Percent 100, got %d", m.Percent)
		}
		if !m.Done || m.Failed {
			t.Fatalf("expected Done without Failed, got Done=%v Failed=%v", m.Done, m.Failed)
		}
	})

	// Close transitions Installing → Complete through updateWizard.
	t.Run("close→Complete", func(t *testing.T) {
		m := NewInstallModel()
		m.step = stepWizInstalling
		next, _ := m.Update(progressDoneMsg{Success: true})
		im := next.(InstallModel)
		if im.step != stepWizComplete {
			t.Fatalf("expected Complete after close, got step %d", im.step)
		}
	})

	// Fail surfaces the error view in place; RollbackOnFailure preserved.
	t.Run("fail→error", func(t *testing.T) {
		ch := make(pipeline.ProgressChan, 32)
		orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
		if orch.Policy != pipeline.RollbackOnFailure {
			t.Fatalf("orchestrator must keep RollbackOnFailure")
		}
		plan := &wizardFakePlan{failAfter: 3}
		type outcome struct {
			res *pipeline.ExecutionResult
			err error
		}
		done := make(chan outcome, 1)
		go func() {
			res, err := orch.RunWithChan(ctx, plan, ch)
			done <- outcome{res, err}
		}()
		// Consume the 3 pre-failure events, then the close.
		m := NewInstallingModel()
		events := 0
		closed := false
		for !closed {
			msg := waitProgress(ch)()
			switch v := msg.(type) {
			case pipeline.ProgressEvent:
				um, _ := m.Update(v)
				m = um.(InstallingModel)
				events++
			case progressDoneMsg:
				closed = true
			}
		}
		out := <-done
		if out.err == nil {
			t.Fatalf("expected apply failure to surface")
		}
		if events != 3 {
			t.Fatalf("expected 3 pre-failure events, got %d", events)
		}
		// Wizard surfaces the orchestrator error via installResultMsg
		// and stays on Installing with the Failed view.
		wm := NewInstallModel()
		wm.step = stepWizInstalling
		next, _ := wm.Update(installResultMsg{result: nil, err: out.err})
		im := next.(InstallModel)
		if im.step != stepWizInstalling {
			t.Fatalf("failure must stay on Installing, got step %d", im.step)
		}
		if !im.installing.Failed {
			t.Fatalf("installing model must show Failed")
		}
	})
}

// Task 3.4: Review confirm arms Installing + ProgressChan(32) and streams
// ProgressEvent → InstallingModel → Complete via waitProgress.
func TestWizardReviewConfirmWiring(t *testing.T) {
	m := NewInstallModel()
	m.step = stepWizReview
	m.selectedAgents = []string{"opencode"}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	im := next.(InstallModel)
	if im.step != stepWizInstalling {
		t.Fatalf("expected Installing after Review confirm, got step %d", im.step)
	}
	if im.progressCh == nil || cap(im.progressCh) != 32 {
		t.Fatalf("expected ProgressChan(32), got cap %d", cap(im.progressCh))
	}
	if cmd == nil {
		t.Fatalf("expected orchestrator/wait command batch")
	}

	// ProgressEvent forwards into InstallingModel and re-arms waitProgress.
	next, cmd = im.Update(pipeline.ProgressEvent{Step: "deploy", Percent: 40, Message: "half"})
	im = next.(InstallModel)
	if im.step != stepWizInstalling {
		t.Fatalf("progress must stay on Installing, got step %d", im.step)
	}
	if im.installing.Percent != 40 || im.installing.Count != 1 {
		t.Fatalf("expected Percent 40 Count 1, got %d/%d", im.installing.Percent, im.installing.Count)
	}
	if cmd == nil {
		t.Fatalf("expected re-armed waitProgress command")
	}

	// Success result completes to the Complete stage.
	next, _ = im.Update(installResultMsg{result: nil, err: nil})
	im = next.(InstallModel)
	if im.step != stepWizComplete {
		t.Fatalf("expected Complete after success, got step %d", im.step)
	}
	if im.installing.Percent != 100 || !im.installing.Done {
		t.Fatalf("expected Done at 100%%, got %d Done=%v", im.installing.Percent, im.installing.Done)
	}
}
