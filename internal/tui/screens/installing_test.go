package screens

import (
	"os"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/charmbracelet/x/ansi"
)

func TestInstalling_Bar_0Percent_Empty(t *testing.T) {
	m := NewInstallingModel()
	m.Percent = 0
	bar := m.BarString()
	want := strings.Repeat("░", 30)
	if bar != want {
		t.Fatalf("bar 0%% = %q, want %q (30░)", bar, want)
	}
	if got := strings.Count(bar, "█"); got != 0 {
		t.Fatalf("filled count %d, want 0", got)
	}
	if got := strings.Count(bar, "░"); got != 30 {
		t.Fatalf("empty count %d, want 30", got)
	}
	// View at width >=40 must contain bar
	t.Setenv("TERM", "xterm-256color")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	_ = os.Unsetenv("BIGGZ_PRETTY")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, want) {
		t.Fatalf("View should contain 30░ at 0%%, got %q", view)
	}
}

func TestInstalling_Bar_50Percent_Half(t *testing.T) {
	m := NewInstallingModel()
	m.Percent = 50
	bar := m.BarString()
	want := strings.Repeat("█", 15) + strings.Repeat("░", 15)
	if bar != want {
		t.Fatalf("bar 50%% = %q, want 15█+15░ %q", bar, want)
	}
	if len([]rune(bar)) != 30 {
		t.Fatalf("bar len %d, want 30", len([]rune(bar)))
	}
	t.Setenv("TERM", "xterm-256color")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, want) {
		t.Fatalf("View should contain 15█+15░ at 50%%, got %q", view)
	}
}

func TestInstalling_Bar_100Percent_Full(t *testing.T) {
	m := NewInstallingModel()
	m.Percent = 100
	bar := m.BarString()
	want := strings.Repeat("█", 30)
	if bar != want {
		t.Fatalf("bar 100%% = %q, want 30█ %q", bar, want)
	}
	m.Done = true
	t.Setenv("TERM", "xterm-256color")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, want) {
		t.Fatalf("View 100%% should contain 30█, got %q", view)
	}
	if !strings.Contains(strings.ToLower(view), "completed") && !strings.Contains(view, "✅") {
		t.Fatalf("View at 100%% Done should show completion state, got %q", view)
	}
}

func TestInstalling_StepNameDisplayed(t *testing.T) {
	m := NewInstallingModel()
	ev := pipeline.ProgressEvent{Step: "deploy-skills", Percent: 10, Message: "copying..."}
	updated, _ := m.Update(ev)
	m2 := updated.(InstallingModel)
	if m2.Step != "deploy-skills" {
		t.Fatalf("Step = %q, want deploy-skills", m2.Step)
	}
	if m2.Message != "copying..." {
		t.Fatalf("Message = %q, want copying...", m2.Message)
	}
	view := ansi.Strip(m2.View())
	if !strings.Contains(view, "deploy-skills") {
		t.Fatalf("View should contain step name deploy-skills, got %q", view)
	}
	if !strings.Contains(view, "copying...") {
		t.Fatalf("View should contain message copying..., got %q", view)
	}
}

func TestInstalling_EventsForwardedWithoutDrop(t *testing.T) {
	m := NewInstallingModel()
	// Given channel with 10 events 0..100 (10 events)
	ch := make(pipeline.ProgressChan, 32)
	go func() {
		for i := 0; i < 10; i++ {
			pct := i * 10
			if i == 9 {
				pct = 100
			}
			ch <- pipeline.ProgressEvent{Step: "step", Percent: pct, Message: "msg"}
		}
		close(ch)
	}()
	count := 0
	var last pipeline.ProgressEvent
	for {
		cmd := waitProgress(ch)
		msg := cmd()
		switch v := msg.(type) {
		case pipeline.ProgressEvent:
			count++
			last = v
			updated, _ := m.Update(v)
			m = updated.(InstallingModel)
		case progressDoneMsg:
			goto done
		}
	}
done:
	if count != 10 {
		t.Fatalf("processed %d events, want 10 lossless", count)
	}
	if m.Percent != last.Percent {
		t.Fatalf("Percent %d, want last event %d", m.Percent, last.Percent)
	}
	if m.Percent != 100 {
		t.Fatalf("final Percent %d, want 100", m.Percent)
	}
	if m.Count != 10 {
		t.Fatalf("model Count %d, want 10", m.Count)
	}
}

func TestInstalling_ChannelCloseTransitionsToDone(t *testing.T) {
	m := NewInstallingModel()
	m.Percent = 100
	ch := make(pipeline.ProgressChan, 1)
	close(ch)
	cmd := waitProgress(ch)
	msg := cmd()
	done, ok := msg.(progressDoneMsg)
	if !ok {
		t.Fatalf("expected progressDoneMsg on close, got %T", msg)
	}
	if !done.Success {
		t.Fatalf("expected Success true on close")
	}
	updated, _ := m.Update(done)
	m2 := updated.(InstallingModel)
	if !m2.Done {
		t.Fatalf("model should be Done after channel close with Percent 100")
	}
	view := ansi.Strip(m2.View())
	if !strings.Contains(strings.ToLower(view), "completed") && !strings.Contains(view, "✅") {
		t.Fatalf("Done View should show success, got %q", view)
	}
}

func TestInstalling_FailureEventShowsError(t *testing.T) {
	m := NewInstallingModel()
	// Simulate failure via progressDoneMsg with error
	updated, _ := m.Update(progressDoneMsg{Success: false, Err: errTestInstall})
	m2 := updated.(InstallingModel)
	if !m2.Failed {
		t.Fatalf("expected Failed true on error")
	}
	view := ansi.Strip(m2.View())
	if !strings.Contains(view, "failed") && !strings.Contains(view, "❌") {
		t.Fatalf("Failed View should show failed state without panic, got %q", view)
	}
	// Ensure no panic on View
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View panicked on failure: %v", r)
		}
	}()
	_ = m2.View()
}

func TestInstalling_TermDumb_ZeroCSI(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	_ = os.Unsetenv("BIGGZ_PRETTY")
	_ = os.Unsetenv("PI_SUBAGENT_CHILD")
	m := NewInstallingModel()
	m.Percent = 50
	m.Step = "deploy-skills"
	m.Message = "copying..."
	view := m.View()
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("TERM=dumb View should contain zero ANSI/CSI escapes, got %q", view)
	}
	if strings.Contains(view, "\x1b[?2026h") || strings.Contains(view, "\x1b[?2026l") {
		t.Fatalf("TERM=dumb View should not contain sync markers, got %q", view)
	}
	// Bar must be plain text 15█+15░
	plain := strings.Repeat("█", 15) + strings.Repeat("░", 15)
	if !strings.Contains(view, plain) {
		t.Fatalf("TERM=dumb bar should be plain 15█+15░, got %q", view)
	}
	// Also check installing isSyncSupported false when dumb
	if isInstallingSyncSupported() {
		t.Fatalf("isInstallingSyncSupported should be false with TERM=dumb")
	}
}

func TestInstalling_NoAnimation_DisablesCSIAndTick(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("BIGGZ_NO_ANIMATION", "1")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	_ = os.Unsetenv("BIGGZ_PRETTY")
	if isInstallingSyncSupported() {
		t.Fatalf("isInstallingSyncSupported should be false with BIGGZ_NO_ANIMATION=1")
	}
	m := NewInstallingModel()
	m.Percent = 30
	view := m.View()
	if strings.Contains(view, "\x1b[?2026h") || strings.Contains(view, "\x1b[?2026l") {
		t.Fatalf("BIGGZ_NO_ANIMATION=1 View should contain zero ESC[?2026h/l, got %q", view)
	}
	if cmd := installingTickCmd(); cmd != nil {
		t.Fatalf("tickCmd should be nil when BIGGZ_NO_ANIMATION=1, bar updates only on ProgressEvent")
	}
	// Bar should still update only on ProgressEvent, not tick
	orig := m.Percent
	updated, cmd2 := m.Update(installingTickMsg{})
	m2 := updated.(InstallingModel)
	if m2.Percent != orig {
		t.Fatalf("bar should not advance on tick when animation disabled, got %d want %d", m2.Percent, orig)
	}
	if cmd2 != nil {
		t.Fatalf("tick should not reschedule when disabled, got cmd")
	}
	// Ensure plain fallback still works: GENTLE compat
	t.Setenv("BIGGZ_NO_ANIMATION", "")
	t.Setenv("GENTLE_AI_NO_ANIMATION", "1")
	if !installingAnimationsDisabled() {
		t.Fatalf("GENTLE_AI_NO_ANIMATION=1 should disable animation")
	}
	if cmd := installingTickCmd(); cmd != nil {
		t.Fatalf("tickCmd should be nil with GENTLE compat")
	}
}

func TestInstalling_OrchestratorViaTeaCmd_NonBlocking(t *testing.T) {
	// Verify waitProgress is a tea.Cmd that can be invoked without blocking Apply
	ch := make(pipeline.ProgressChan, 32)
	// Simulate Apply emitting events in background
	go func() {
		for i := 0; i < 5; i++ {
			ch <- pipeline.ProgressEvent{Step: "test", Percent: i * 20, Message: "working"}
		}
		close(ch)
	}()
	m := NewInstallingModel()
	received := 0
	for {
		cmd := waitProgress(ch)
		if cmd == nil {
			t.Fatal("waitProgress should return non-nil tea.Cmd")
		}
		msg := cmd()
		switch v := msg.(type) {
		case pipeline.ProgressEvent:
			received++
			updated, _ := m.Update(v)
			m = updated.(InstallingModel)
		case progressDoneMsg:
			if received != 5 {
				t.Fatalf("expected 5 events streamed incrementally, got %d", received)
			}
			return
		}
	}
}
