package screens

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
	tea "github.com/charmbracelet/bubbletea"
)

const installingBarWidth = 30

// InstallingModel renders a 30-char progress bar (█/░) streaming ProgressEvent via tea.Msg.
// Bar fill is Percent*30/100 exactly per spec REQ-TUI-PIPE-001.
type InstallingModel struct {
	Percent int
	Step    string
	Message string
	Done    bool
	Failed  bool
	Err     string
	Count   int
}

// NewInstallingModel creates an empty installing model.
func NewInstallingModel() InstallingModel { return InstallingModel{} }

func (m InstallingModel) Init() tea.Cmd { return nil }

// progressDoneMsg signals channel close / orchestrator completion.
// Reuses pipeline ExecutionResult semantics for Done/Failed transition.
type progressDoneMsg struct {
	Success bool
	Err     error
}

// waitProgress returns a tea.Cmd that forwards the next ProgressEvent without drop.
// It is lossless: buffer cap 32, monotonic, and on close returns progressDoneMsg.
// The channel is closed by Apply (StagePlan) and the model transitions to Done.
func waitProgress(ch pipeline.ProgressChan) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return progressDoneMsg{Success: true}
		}
		return ev
	}
}

// installingAnimationsDisabled mirrors tui/tui.go tuiAnimationsDisabled and screens/agentbuilder.go.
// Reuses guard: BIGGZ_NO_ANIMATION=1 or GENTLE_AI_NO_ANIMATION=1 disables animation/spinner.
func installingAnimationsDisabled() bool {
	return os.Getenv("BIGGZ_NO_ANIMATION") == "1" || os.Getenv("GENTLE_AI_NO_ANIMATION") == "1"
}

// isInstallingSyncSupported mirrors tui/tui.go isSyncSupported (CSI 2026) logic:
// BIGGZ_PRETTY=0, PI_SUBAGENT_CHILD=1, animation disabled, TERM empty/dumb → false.
// Reused to strip CSI when NO_ANIMATION/TERM=dumb per PR4 task.
func isInstallingSyncSupported() bool {
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return false
	}
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return false
	}
	if installingAnimationsDisabled() {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	return true
}

// isInstallingPretty reports whether styled output is allowed.
// TERM=dumb or BIGGZ_PRETTY=0 forces plain fallback (zero ANSI/CSI).
func isInstallingPretty() bool {
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return false
	}
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

// installingTickMsg drives bar/spinner tick for installing screen.
type installingTickMsg time.Time

// installingTickCmd mirrors tui.go tickCmd for installing screen.
// When BIGGZ_NO_ANIMATION or TERM=dumb or PRETTY=0 it returns nil (frozen spinner),
// otherwise returns 100ms tick - bar updates only on ProgressEvent when disabled.
func installingTickCmd() tea.Cmd {
	if installingAnimationsDisabled() {
		return nil
	}
	if os.Getenv("TERM") == "dumb" {
		return nil
	}
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return nil
	}
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return installingTickMsg(t) })
}

// BarString returns plain 30-char bar █/░ for Percent*30/100 (no ANSI).
// Used by tests to verify 0%=30░, 50%=15█+15░, 100%=30█.
func (m InstallingModel) BarString() string {
	filled := m.Percent * installingBarWidth / 100
	if filled < 0 {
		filled = 0
	}
	if filled > installingBarWidth {
		filled = installingBarWidth
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", installingBarWidth-filled)
}

// Update handles ProgressEvent (lossless), progressDoneMsg (close→Done), and installingTickMsg (frozen when NO_ANIMATION).
func (m InstallingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case pipeline.ProgressEvent:
		m.Count++
		m.Step = v.Step
		m.Message = v.Message
		m.Percent = v.Percent
		if m.Percent < 0 {
			m.Percent = 0
		}
		if m.Percent > 100 {
			m.Percent = 100
		}
		return m, nil
	case progressDoneMsg:
		if v.Err != nil || !v.Success {
			m.Failed = true
			if v.Err != nil {
				m.Err = v.Err.Error()
			}
		} else {
			m.Done = true
			if m.Percent < 100 {
				// keep current percent unless 100; view shows completion state at 100% in success case
				// If progress finished without 100, mark 100 for success visual per spec
				if m.Count > 0 && m.Percent == 0 {
					m.Percent = 100
				}
			}
		}
		return m, nil
	case installingTickMsg:
		if installingAnimationsDisabled() {
			return m, nil
		}
		return m, installingTickCmd()
	}
	return m, nil
}

// View renders bar + step/message + status.
// Uses lipgloss tokens ProgressFilled/ProgressEmpty when pretty, plain fallback when TERM=dumb.
// Strips CSI 2026 when isInstallingSyncSupported()==false (NO_ANIMATION/TERM=dumb) and zero ANSI when TERM=dumb.
func (m InstallingModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Installing biggz-ai"))
	b.WriteString("\n\n")

	// Build bar: styled when pretty, plain when dumb
	var bar string
	filled := m.Percent * installingBarWidth / 100
	if filled < 0 {
		filled = 0
	}
	if filled > installingBarWidth {
		filled = installingBarWidth
	}
	plain := strings.Repeat("█", filled) + strings.Repeat("░", installingBarWidth-filled)
	if isInstallingPretty() {
		bar = styles.ProgressFilled.Render(strings.Repeat("█", filled)) + styles.ProgressEmpty.Render(strings.Repeat("░", installingBarWidth-filled))
	} else {
		bar = plain
	}
	b.WriteString(fmt.Sprintf("%s %3d%%\n", bar, m.Percent))
	if m.Step != "" || m.Message != "" {
		b.WriteString(fmt.Sprintf("%s %s\n", m.Step, m.Message))
	} else {
		b.WriteString(styles.StatusInfo.Render("Preparing...") + "\n")
	}
	if m.Done {
		b.WriteString("\n")
		b.WriteString(styles.SuccessBox.Render("✅ Install completed"))
	} else if m.Failed {
		b.WriteString("\n")
		b.WriteString(styles.ErrorBox.Render(fmt.Sprintf("❌ Install failed: %s", m.Err)))
	} else {
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("Installing..."))
	}
	out := b.String()
	// Reuse isSyncSupported: strip CSI 2026 when NO_ANIMATION/TERM=dumb/BIGGZ_PRETTY=0
	if !isInstallingSyncSupported() {
		out = strings.ReplaceAll(out, "\x1b[?2026h", "")
		out = strings.ReplaceAll(out, "\x1b[?2026l", "")
	}
	// Dumb terminal plain fallback: zero ANSI/CSI escapes, bar plain text
	if os.Getenv("TERM") == "dumb" || os.Getenv("BIGGZ_PRETTY") == "0" {
		out = ansi.Strip(out)
	}
	return out
}
