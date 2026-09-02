// Package tui provides a terminal UI for biggz-ai configuration and management.
//
// Disable TUI spinner animation with:
//
//	BIGGZ_NO_ANIMATION=1 biggz
//
// GENTLE_AI_NO_ANIMATION=1 is also honoured for compatibility (port b3dfc1ef).
// When either is set to "1", spinner ticks are suppressed and frames stay
// static; operations (install, sync, etc.) continue normally.
package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/biggs-100/biggz-ai/internal/tui/screens"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	syncBegin           = "\x1b[?2026h"
	syncEnd             = "\x1b[?2026l"
	bracketedPasteStart = "\x1b[200~"
	bracketedPasteEnd   = "\x1b[201~"
)

// PasteMsg is emitted when a bracketed paste sequence is received.
// Content between ESC[200~ and ESC[201~ is buffered into a single event
// so large pastes (>10 lines) do not arrive as individual keystrokes.
type PasteMsg struct {
	Text string
}

// isSyncSupported reports whether synchronized output (CSI 2026) should be
// enabled. It is gated on TERM support and animation envs plus pretty guards.
func isSyncSupported() bool {
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return false
	}
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return false
	}
	if tuiAnimationsDisabled() {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	return true
}

// syncOutput wraps frame with CSI 2026 markers when supported.
// When the terminal does not support sync or animation is disabled,
// the frame is returned unchanged without garbling. Idempotent: avoids
// double-wrapping and strips markers when guards disable sync.
func syncOutput(frame string) string {
	if !isSyncSupported() {
		// Strip any existing CSI markers to guarantee zero ESC[?2026h/l when disabled.
		if strings.Contains(frame, syncBegin) || strings.Contains(frame, syncEnd) {
			f := strings.ReplaceAll(frame, syncBegin, "")
			f = strings.ReplaceAll(f, syncEnd, "")
			return f
		}
		return frame
	}
	// Idempotent: avoid double-wrapping if frame already synced (opt-in screens).
	if strings.HasPrefix(frame, syncBegin) && strings.HasSuffix(frame, syncEnd) {
		if strings.Count(frame, syncBegin) == 1 && strings.Count(frame, syncEnd) == 1 {
			return frame
		}
	}
	// Normalize cases where markers exist but not as exact wrap (e.g., nested calls).
	if strings.Contains(frame, syncBegin) || strings.Contains(frame, syncEnd) {
		if strings.Count(frame, syncBegin) == 1 && strings.Count(frame, syncEnd) == 1 {
			return frame
		}
		f := strings.ReplaceAll(frame, syncBegin, "")
		f = strings.ReplaceAll(f, syncEnd, "")
		return syncBegin + f + syncEnd
	}
	return syncBegin + frame + syncEnd
}

// ── Throttled sync streaming (16ms trailing coalesce, CSI 2026) ────────────
//
// Lens/viewport updates arriving within 16ms are coalesced to a single flush
// at t0+16ms. Reuses isSyncSupported guards so BIGGZ_PRETTY=0,
// BIGGZ_NO_ANIMATION, TERM=dumb and PI_SUBAGENT_CHILD=1 suppress CSI.

var (
	pendingFrame string
	syncMu       sync.Mutex
	syncTimer    *time.Timer
)

// flushSyncMsg is emitted after the 16ms trailing window.
type flushSyncMsg struct{}

// scheduleSyncFlush stores the latest frame and schedules a trailing flush
// via time.AfterFunc(16ms). Rapid calls coalesce to the last frame; the
// flush wraps via syncOutput so guards and idempotency apply.
func scheduleSyncFlush(frame string) {
	syncMu.Lock()
	pendingFrame = frame
	if syncTimer != nil {
		syncTimer.Stop()
	}
	syncTimer = time.AfterFunc(16*time.Millisecond, func() {
		syncMu.Lock()
		f := pendingFrame
		pendingFrame = ""
		syncMu.Unlock()
		_ = syncOutput(f)
	})
	syncMu.Unlock()
}

// scheduleSyncFlushCmd is the bubbletea-friendly variant that returns a
// tea.Cmd emitting flushSyncMsg after 16ms. Useful for Model.Update throttling.
func scheduleSyncFlushCmd(frame string) tea.Cmd {
	syncMu.Lock()
	pendingFrame = frame
	if syncTimer != nil {
		syncTimer.Stop()
	}
	syncMu.Unlock()
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		syncMu.Lock()
		pendingFrame = ""
		syncMu.Unlock()
		return flushSyncMsg{}
	})
}

// pendingFrameForTest exposes coalesced pending frame for tests.
func pendingFrameForTest() string {
	syncMu.Lock()
	defer syncMu.Unlock()
	return pendingFrame
}

// resetSyncThrottleForTest clears throttle state (tests only).
func resetSyncThrottleForTest() {
	syncMu.Lock()
	defer syncMu.Unlock()
	pendingFrame = ""
	if syncTimer != nil {
		syncTimer.Stop()
		syncTimer = nil
	}
}

const noAnimationEnv = "BIGGZ_NO_ANIMATION"

// tuiAnimationsDisabled reports whether spinner animation is disabled.
// BIGGZ_NO_ANIMATION=1 disables animation; GENTLE_AI_NO_ANIMATION=1 is kept
// for compatibility with gentle-ai (port b3dfc1ef).
// When disabled, tickCmd returns nil and synchronized output (ESC[?2026h/l)
// is suppressed so TERM=dumb terminals are not garbled.
func tuiAnimationsDisabled() bool {
	return os.Getenv(noAnimationEnv) == "1" || os.Getenv("GENTLE_AI_NO_ANIMATION") == "1"
}

// TickMsg drives spinner animation.
type TickMsg time.Time

func tickCmd() tea.Cmd {
	if tuiAnimationsDisabled() {
		return nil
	}
	if os.Getenv("TERM") == "dumb" {
		return nil
	}
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return nil
	}
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Screen IDs
const (
	screenDashboard = iota
	screenInstall
	screenConfig
	screenStatus
	screenMemory
	screenBackup
	screenProfile
	screenUpgrade
	screenUninstall
	screenStrictTDD
	screenReview
	screenSessions
	screenWelcome
	screenMemSearch
	screenRecovery
	screenModelPickers
	screenAgentBuilder
	screenCommunity
	screenSync
	screenUpdatePrompt
	screenPluginUninstall
	screenHelp
	screenCount
)

// Exported screen IDs for CLI routing.
const (
	ScreenHelp   = screenHelp
	ScreenBackup = screenBackup
)

// Model is the top-level TUI model.
type Model struct {
	currentScreen   int
	showHelp        bool
	pasteActive     bool
	pasteBuf        string
	dashboard       screens.DashboardModel
	welcome         screens.WelcomeModel
	install         screens.InstallModel
	config          screens.ConfigModel
	status          screens.StatusModel
	memory          screens.MemoryModel
	backup          screens.BackupModel
	help            screens.HelpModel
	profile         screens.ProfileModel
	recovery        screens.RecoveryModel
	modelPicker     screens.ModelPickerScreen
	agentBuilder    screens.AgentBuilderScreen
	community       screens.CommunityScreen
	upgrade         screens.UpgradeModel
	uninstall       screens.UninstallModel
	strictTDD       screens.StrictTDDModel
	reviewScr       screens.ReviewModel
	sessions        screens.SessionsModel
	syncScr         screens.SyncModel
	updatePrompt    screens.UpdatePromptModel
	pluginUninstall screens.PluginUninstallModel
	width, height   int
	err             error
}

// New creates the initial TUI model.
func New() Model {
	return Model{
		currentScreen:   screenDashboard,
		dashboard:       screens.NewDashboardModel(),
		welcome:         screens.NewWelcomeModel(),
		install:         screens.NewInstallModel(),
		config:          screens.NewConfigModel(),
		status:          screens.NewStatusModel(),
		memory:          screens.NewMemoryModel(),
		backup:          screens.NewBackupModel(),
		help:            screens.NewHelpModel(),
		profile:         screens.NewProfileModel(),
		recovery:        screens.NewRecoveryModel(),
		modelPicker:     screens.NewModelPickerScreen(),
		agentBuilder:    screens.NewAgentBuilderScreen(),
		community:       screens.NewCommunityScreen(),
		upgrade:         screens.NewUpgradeModel(),
		uninstall:       screens.NewUninstallModel(),
		strictTDD:       screens.NewStrictTDDModel(),
		reviewScr:       screens.NewReviewModel(),
		sessions:        screens.NewSessionsModel(),
		syncScr:         screens.NewSyncModel(),
		updatePrompt:    screens.NewUpdatePromptModel(),
		pluginUninstall: screens.NewPluginUninstallModel(),
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	switch m.currentScreen {
	case screenHelp:
		return m.help.Init()
	case screenBackup:
		return m.backup.Init()
	default:
		return m.dashboard.Init()
	}
}

// feedPaste processes a raw chunk that may contain bracketed paste markers.
// It buffers incomplete sequences and returns a PasteMsg when a complete
// paste is assembled. Large pastes (>10 lines) arrive as a single event.
func (m *Model) feedPaste(chunk string) *PasteMsg {
	if !m.pasteActive {
		startIdx := strings.Index(chunk, bracketedPasteStart)
		if startIdx == -1 {
			return nil
		}
		afterStart := chunk[startIdx+len(bracketedPasteStart):]
		endIdx := strings.Index(afterStart, bracketedPasteEnd)
		if endIdx != -1 {
			text := afterStart[:endIdx]
			return &PasteMsg{Text: text}
		}
		m.pasteActive = true
		m.pasteBuf = afterStart
		return nil
	}
	endIdx := strings.Index(chunk, bracketedPasteEnd)
	if endIdx != -1 {
		beforeEnd := chunk[:endIdx]
		m.pasteBuf += beforeEnd
		text := m.pasteBuf
		m.pasteActive = false
		m.pasteBuf = ""
		return &PasteMsg{Text: text}
	}
	m.pasteBuf += chunk
	return nil
}

// flushPaste emits buffered incomplete paste content as a PasteMsg and resets
// state. Called on timeout or next non-paste input.
func (m *Model) flushPaste() *PasteMsg {
	if !m.pasteActive {
		return nil
	}
	text := m.pasteBuf
	m.pasteActive = false
	m.pasteBuf = ""
	if text == "" {
		return nil
	}
	return &PasteMsg{Text: text}
}

// Update handles messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Bracketed paste: raw string chunks containing ESC[200~/ESC[201~ are
	// buffered into a single PasteMsg. This handles fixture sequences and
	// direct string inputs used in tests without touching bubbletea internals.
	switch v := msg.(type) {
	case string:
		if m.pasteActive {
			if strings.Contains(v, bracketedPasteEnd) {
				if pm := m.feedPaste(v); pm != nil {
					return m, func() tea.Msg { return *pm }
				}
				return m, nil
			}
			// No end: incomplete sequence -> flush on next non-paste input.
			if pm := m.flushPaste(); pm != nil {
				if strings.Contains(v, bracketedPasteStart) {
					if pm2 := m.feedPaste(v); pm2 != nil {
						return m, func() tea.Msg { return *pm2 }
					}
					// Started new incomplete, keep buffered; still return flushed previous.
				}
				return m, func() tea.Msg { return *pm }
			}
			if strings.Contains(v, bracketedPasteStart) {
				if pm := m.feedPaste(v); pm != nil {
					return m, func() tea.Msg { return *pm }
				}
				return m, nil
			}
			return m, nil
		}
		if strings.Contains(v, bracketedPasteStart) {
			if pm := m.feedPaste(v); pm != nil {
				return m, func() tea.Msg { return *pm }
			}
			return m, nil
		}
		return m, nil
	case PasteMsg:
		// Paste content is inserted as text, not interpreted as keys.
		// Route to current screen if it handles PasteMsg, otherwise swallow.
		// Ensure paste does not trigger quit/navigation.
		switch m.currentScreen {
		case screenDashboard:
			u, cmd := m.dashboard.Update(v)
			m.dashboard = u.(screens.DashboardModel)
			return m, cmd
		case screenWelcome:
			u, cmd := m.welcome.Update(v)
			m.welcome = u.(screens.WelcomeModel)
			return m, cmd
		case screenInstall:
			u, cmd := m.install.Update(v)
			m.install = u.(screens.InstallModel)
			return m, cmd
		case screenConfig:
			u, cmd := m.config.Update(v)
			m.config = u.(screens.ConfigModel)
			return m, cmd
		default:
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to screens that need viewport sizing
		if m.currentScreen == screenHelp {
			u, cmd := m.help.Update(msg)
			m.help = u.(screens.HelpModel)
			return m, cmd
		}
		if m.currentScreen == screenBackup {
			u, cmd := m.backup.Update(msg)
			m.backup = u.(screens.BackupModel)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Help toggle — works on any screen, suppressed when help filter focused
		if msg.String() == "?" {
			if m.currentScreen == screenHelp && m.help.IsFocused() {
				// Let HelpModel handle '?' as filter input
			} else {
				m.showHelp = !m.showHelp
				return m, nil
			}
		}

		// When help is shown, only ESC or ? close it
		if m.showHelp {
			if msg.String() == "?" || msg.String() == "esc" {
				m.showHelp = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.currentScreen == screenDashboard {
				return m, tea.Quit
			}
			if m.currentScreen == screenWelcome {
				m.currentScreen = screenDashboard
				m.showHelp = false
				return m, nil
			}
			if m.currentScreen == screenAgentBuilder {
				// The agent builder owns ESC for its internal back navigation
				// (engine → prompt → SDD → ...) and only exits to the
				// dashboard from its first or last views. Fall through to the
				// screen routing below instead of returning to the dashboard.
				break
			}
			m.currentScreen = screenDashboard
			m.showHelp = false
			return m, nil
		}

	case flushSyncMsg:
		// 16ms trailing coalesce flush — trigger re-render via View()->syncOutput.
		return m, nil

	case screens.NavigateMsg:
		m.currentScreen = int(msg.Screen)
		m.showHelp = false
		return m, nil

	case screens.QuitMsg:
		return m, tea.Quit

	case error:
		m.err = msg
		return m, nil
	}

	// Route message to current screen
	switch m.currentScreen {
	case screenDashboard:
		u, cmd := m.dashboard.Update(msg)
		m.dashboard = u.(screens.DashboardModel)
		return m, cmd
	case screenWelcome:
		u, cmd := m.welcome.Update(msg)
		m.welcome = u.(screens.WelcomeModel)
		return m, cmd
	case screenInstall:
		u, cmd := m.install.Update(msg)
		m.install = u.(screens.InstallModel)
		return m, cmd
	case screenConfig:
		u, cmd := m.config.Update(msg)
		m.config = u.(screens.ConfigModel)
		return m, cmd
	case screenStatus:
		u, cmd := m.status.Update(msg)
		m.status = u.(screens.StatusModel)
		return m, cmd
	case screenMemory, screenMemSearch:
		u, cmd := m.memory.Update(msg)
		m.memory = u.(screens.MemoryModel)
		return m, cmd
	case screenBackup:
		u, cmd := m.backup.Update(msg)
		m.backup = u.(screens.BackupModel)
		return m, cmd
	case screenHelp:
		u, cmd := m.help.Update(msg)
		m.help = u.(screens.HelpModel)
		return m, cmd
	case screenProfile:
		u, cmd := m.profile.Update(msg)
		m.profile = u.(screens.ProfileModel)
		return m, cmd
	case screenUpgrade:
		u, cmd := m.upgrade.Update(msg)
		m.upgrade = u.(screens.UpgradeModel)
		return m, cmd
	case screenUninstall:
		u, cmd := m.uninstall.Update(msg)
		m.uninstall = u.(screens.UninstallModel)
		return m, cmd
	case screenStrictTDD:
		u, cmd := m.strictTDD.Update(msg)
		m.strictTDD = u.(screens.StrictTDDModel)
		return m, cmd
	case screenReview:
		u, cmd := m.reviewScr.Update(msg)
		m.reviewScr = u.(screens.ReviewModel)
		return m, cmd
	case screenRecovery:
		u, cmd := m.recovery.Update(msg)
		m.recovery = u.(screens.RecoveryModel)
		return m, cmd
	case screenModelPickers:
		u, cmd := m.modelPicker.Update(msg)
		m.modelPicker = u.(screens.ModelPickerScreen)
		return m, cmd
	case screenAgentBuilder:
		u, cmd := m.agentBuilder.Update(msg)
		m.agentBuilder = u.(screens.AgentBuilderScreen)
		return m, cmd
	case screenCommunity:
		u, cmd := m.community.Update(msg)
		m.community = u.(screens.CommunityScreen)
		return m, cmd
	case screenSessions:
		u, cmd := m.sessions.Update(msg)
		m.sessions = u.(screens.SessionsModel)
		return m, cmd
	case screenSync:
		u, cmd := m.syncScr.Update(msg)
		m.syncScr = u.(screens.SyncModel)
		return m, cmd
	case screenUpdatePrompt:
		u, cmd := m.updatePrompt.Update(msg)
		m.updatePrompt = u.(screens.UpdatePromptModel)
		return m, cmd
	case screenPluginUninstall:
		u, cmd := m.pluginUninstall.Update(msg)
		m.pluginUninstall = u.(screens.PluginUninstallModel)
		return m, cmd
	}

	return m, nil
}

// View renders the current screen or help overlay.
func (m Model) View() string {
	var frame string
	if m.err != nil {
		frame = styles.ErrorBox.Render(fmt.Sprintf("Error: %v\n\nPress esc to quit.", m.err))
		if os.Getenv("TERM") == "dumb" {
			return ansi.Strip(syncOutput(frame))
		}
		return syncOutput(frame)
	}

	if m.showHelp {
		frame = screens.HelpOverlay(m.currentScreen)
		if os.Getenv("TERM") == "dumb" {
			return ansi.Strip(syncOutput(frame))
		}
		return syncOutput(frame)
	}

	switch m.currentScreen {
	case screenDashboard:
		frame = m.dashboard.View()
	case screenWelcome:
		frame = m.welcome.View()
	case screenInstall:
		frame = m.install.View()
	case screenConfig:
		frame = m.config.View()
	case screenStatus:
		frame = m.status.View()
	case screenMemory, screenMemSearch:
		frame = m.memory.View()
	case screenBackup:
		frame = m.backup.View()
	case screenHelp:
		frame = m.help.View()
	case screenProfile:
		frame = m.profile.View()
	case screenUpgrade:
		frame = m.upgrade.View()
	case screenUninstall:
		frame = m.uninstall.View()
	case screenStrictTDD:
		frame = m.strictTDD.View()
	case screenReview:
		frame = m.reviewScr.View()
	case screenRecovery:
		frame = m.recovery.View()
	case screenModelPickers:
		frame = m.modelPicker.View()
	case screenAgentBuilder:
		frame = m.agentBuilder.View()
	case screenCommunity:
		frame = m.community.View()
	case screenSessions:
		frame = m.sessions.View()
	case screenSync:
		frame = m.syncScr.View()
	case screenUpdatePrompt:
		frame = m.updatePrompt.View()
	case screenPluginUninstall:
		frame = m.pluginUninstall.View()
	default:
		frame = ""
	}
	if os.Getenv("TERM") == "dumb" {
		return ansi.Strip(syncOutput(frame))
	}
	return syncOutput(frame)
}

// RunWithScreen starts the TUI at the given screen with alt screen.
// Unknown screen falls back to dashboard.
func RunWithScreen(id int) {
	m := New()
	if id < 0 || id >= screenCount {
		m.currentScreen = screenDashboard
	} else {
		m.currentScreen = id
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Run starts the TUI.
func Run() {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
