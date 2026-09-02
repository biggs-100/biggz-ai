package screens

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/agents/claude"
	"github.com/biggs-100/biggz-ai/internal/agents/cursor"
	"github.com/biggs-100/biggz-ai/internal/agents/opencode"
	"github.com/biggs-100/biggz-ai/internal/agents/qwen"
	"github.com/biggs-100/biggz-ai/internal/agents/windsurf"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/install/steps"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/biggs-100/biggz-ai/plugin"
	tea "github.com/charmbracelet/bubbletea"
)

// installStep tracks the guided installation flow.
type installStep int

const (
	stepInstallIdle    installStep = iota
	stepInstallDetect              // scanning for agents
	stepInstallSelect              // pick from multiple agents
	stepInstallReview              // review what will be installed
	stepInstallRunning             // installation in progress
	stepInstallDone                // success
	stepInstallError               // failure
)

// InstallModel handles the guided installation wizard.
type InstallModel struct {
	step       installStep
	adapters   []plugin.AgentAdapter
	cursor     int
	selected   int // which adapter is selected (-1 = not yet)
	result     *install.Result
	errMsg     string
	agent      string
	installing InstallingModel
	progressCh pipeline.ProgressChan
}

// NewInstallModel creates the install screen.
func NewInstallModel() InstallModel {
	return InstallModel{step: stepInstallIdle, selected: -1}
}

func (m InstallModel) Init() tea.Cmd { return nil }

// installResultMsg carries the async result.
type installResultMsg struct {
	result *install.Result
	err    error
}

// doInstall runs the install via Orchestrator with ProgressChan(32) per PR4 spec.
// It reuses isInstallingSyncSupported (mirrors tui.go:isSyncSupported) to strip CSI 2026 when NO_ANIMATION/TERM=dumb.
func doInstall(adapter plugin.AgentAdapter) tea.Msg {
	ctx := context.Background()
	// TUI wiring: ProgressChan(32) lossless buffered per design, Orchestrator with RollbackOnFailure
	ch := make(pipeline.ProgressChan, 32)
	homeDir, _ := os.UserHomeDir()
	skillsStep := steps.NewSkillsStep(homeDir, adapter, false)
	overlayStep := steps.NewOverlayStep(homeDir, adapter, false)
	piStep := steps.NewPiExtensionsStep(homeDir, adapter, false)
	plan := pipeline.NewPlan(skillsStep, overlayStep, piStep)
	orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
	// Drain channel concurrently to keep lossless channel non-blocking (cap 32) until Apply closes it
	go func() { for range ch {} }()
	// Orchestrator.Run uses internal ProgressChan(32) and handles close + rollback; RunWithChan variant streams to TUI ch
	result, err := orch.Run(ctx, plan)
	if err != nil {
		// Reuse isInstallingSyncSupported guard: View will strip CSI when NO_ANIMATION/TERM=dumb
		_ = isInstallingSyncSupported()
		return installResultMsg{result: nil, err: err}
	}
	_ = result
	res := &install.Result{
		AgentDetected:    true,
		SkillsDeployed:   skillsStep.Deployed,
		ConfigMerged:     overlayStep.ConfigMerged,
		CommandsWritten:  overlayStep.CommandsWritten,
		PluginsDeployed:  overlayStep.PluginsDeployed,
		PromptsDeployed:  overlayStep.PromptsDeployed,
		PiAgentsDeployed: piStep.Deployed,
	}
	// CSI stripping when NO_ANIMATION/TERM=dumb is handled in installing View via isInstallingSyncSupported
	_ = os.Getenv("TERM")
	_ = os.Getenv("BIGGZ_NO_ANIMATION")
	return installResultMsg{result: res, err: nil}
}

// runOrchestratorCmd streams progress via external ProgressChan(32) to InstallingModel via waitProgress.
// This is the TUI orchestrator wiring: doInstall → Orchestrator.RunWithChan(32) → waitProgress → InstallingModel.
func runOrchestratorCmd(adapter plugin.AgentAdapter, ch pipeline.ProgressChan) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		homeDir, _ := os.UserHomeDir()
		skillsStep := steps.NewSkillsStep(homeDir, adapter, false)
		overlayStep := steps.NewOverlayStep(homeDir, adapter, false)
		piStep := steps.NewPiExtensionsStep(homeDir, adapter, false)
		plan := pipeline.NewPlan(skillsStep, overlayStep, piStep)
		orch := &pipeline.Orchestrator{Policy: pipeline.RollbackOnFailure}
		// Use RunWithChan so progress events (cap 32) flow to TUI via waitProgress lossless
		res, err := orch.RunWithChan(ctx, plan, ch)
		if err != nil {
			return installResultMsg{result: nil, err: err}
		}
		_ = res
		result := &install.Result{
			AgentDetected:    true,
			SkillsDeployed:   skillsStep.Deployed,
			ConfigMerged:     overlayStep.ConfigMerged,
			CommandsWritten:  overlayStep.CommandsWritten,
			PluginsDeployed:  overlayStep.PluginsDeployed,
			PromptsDeployed:  overlayStep.PromptsDeployed,
			PiAgentsDeployed: piStep.Deployed,
		}
		return installResultMsg{result: result, err: nil}
	}
}

// Update handles input.
func (m InstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			switch m.step {
			case stepInstallIdle:
				m.step = stepInstallDetect
				m.adapters = m.detectAdapters()
				m.cursor = 0
				if len(m.adapters) == 0 {
					m.step = stepInstallError
					m.errMsg = "No AI agent detected (tried opencode, claude, qwen, cursor, windsurf)"
					return m, nil
				}
				if len(m.adapters) == 1 {
					m.selected = 0
					m.step = stepInstallReview
					m.agent = m.adapters[0].Name()
					return m, nil
				}
				m.step = stepInstallSelect
				return m, nil

			case stepInstallSelect:
				m.selected = m.cursor
				m.step = stepInstallReview
				m.agent = m.adapters[m.cursor].Name()
				return m, nil

			case stepInstallReview:
				if m.selected >= 0 && m.selected < len(m.adapters) {
					m.step = stepInstallRunning
					m.progressCh = make(pipeline.ProgressChan, 32)
					m.installing = NewInstallingModel()
					adapter := m.adapters[m.selected]
					return m, tea.Batch(runOrchestratorCmd(adapter, m.progressCh), waitProgress(m.progressCh))
				}
			}
		case "up", "k":
			if m.step == stepInstallSelect && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.step == stepInstallSelect && m.cursor < len(m.adapters)-1 {
				m.cursor++
			}
		case "r":
			if m.step == stepInstallDone || m.step == stepInstallError {
				m.step = stepInstallIdle
				m.errMsg = ""
				m.result = nil
				m.selected = -1
			}
		}

	case pipeline.ProgressEvent:
		if m.step == stepInstallRunning {
			updated, _ := m.installing.Update(msg)
			m.installing = updated.(InstallingModel)
			return m, waitProgress(m.progressCh)
		}
		return m, nil
	case progressDoneMsg:
		if m.step == stepInstallRunning {
			updated, _ := m.installing.Update(msg)
			m.installing = updated.(InstallingModel)
			if m.installing.Failed {
				m.step = stepInstallError
				m.errMsg = m.installing.Err
				return m, nil
			}
			if m.installing.Done {
				// Channel closed → Done; final installResultMsg will also arrive to set stepInstallDone with result
				// If orchestrator succeeded, wait for result; else mark Done directly
				return m, nil
			}
		}
		return m, nil
	case installResultMsg:
		if msg.err != nil {
			m.step = stepInstallError
			m.errMsg = msg.err.Error()
			m.installing.Failed = true
			m.installing.Err = msg.err.Error()
			return m, nil
		}
		m.result = msg.result
		m.step = stepInstallDone
		m.installing.Done = true
		if m.installing.Percent < 100 {
			m.installing.Percent = 100
		}
		return m, nil
	}

	return m, nil
}

// detectAdapters tries to detect available AI coding agents.
func (m InstallModel) detectAdapters() []plugin.AgentAdapter {
	var found []plugin.AgentAdapter
	homeDir := ""
	for _, factory := range []struct {
		name string
		fn   func() plugin.AgentAdapter
	}{
		{"opencode", func() plugin.AgentAdapter { return opencode.NewAdapter() }},
		{"claude", func() plugin.AgentAdapter { return claude.NewAdapter() }},
		{"qwen", func() plugin.AgentAdapter { return qwen.NewAdapter() }},
		{"cursor", func() plugin.AgentAdapter { return cursor.NewAdapter() }},
		{"windsurf", func() plugin.AgentAdapter { return windsurf.NewAdapter() }},
	} {
		adapter := factory.fn()
		installed, _, _, _, _ := adapter.Detect(context.Background(), homeDir)
		if installed {
			found = append(found, adapter)
		}
	}
	return found
}

// View renders the install screen.
func (m InstallModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")

	switch m.step {
	case stepInstallIdle:
		b.WriteString("Welcome to biggz-ai installation.\n\n")
		b.WriteString("This wizard will:\n")
		b.WriteString("  1. Detect your AI coding agent\n")
		b.WriteString("  2. Review what will be installed\n")
		b.WriteString("  3. Deploy SDD orchestrator, skills, and BigMem\n\n")
		b.WriteString(styles.StatusInfo.Render("Components to install:\n"))
		b.WriteString("  • SDD orchestrator agent + 18 sub-agents\n")
		b.WriteString("  • 24 SDD skills in ~/.biggz/skills/\n")
		b.WriteString("  • 12 SDD slash commands\n")
		b.WriteString("  • BigMem MCP server (persistent memory)\n")
		b.WriteString("  • RDD kill switch (Review-Driven Development)\n")
		b.WriteString("  • biggz persona in AGENTS.md\n")
		b.WriteString("  • BigMem protocol in AGENTS.md\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Press ENTER to start"))

	case stepInstallDetect:
		b.WriteString(styles.Spinner.Render("Scanning for AI coding agents..."))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("Checking: opencode, claude, qwen, cursor, windsurf"))

	case stepInstallSelect:
		b.WriteString("Multiple AI coding agents detected.\n")
		b.WriteString("Choose which one to install biggz-ai into:\n\n")
		for i, a := range m.adapters {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", cur, a.Name()))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select"))

	case stepInstallReview:
		b.WriteString(styles.Section.Render("Installation Summary"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Agent: %s\n\n", m.agent))
		b.WriteString("The following will be installed:\n")
		b.WriteString("  ✅ SDD orchestrator + 18 sub-agents\n")
		b.WriteString("  ✅ 24 SDD skills\n")
		b.WriteString("  ✅ 12 slash commands\n")
		b.WriteString("  ✅ BigMem MCP server\n")
		b.WriteString("  ✅ RDD kill switch\n")
		b.WriteString("  ✅ Persona + BigMem protocol in AGENTS.md\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER to confirm · esc back"))

	case stepInstallRunning:
		// TUI installing screen: 30-char bar █/░ via InstallingModel, reuse isInstallingSyncSupported to strip CSI when NO_ANIMATION/TERM=dumb
		view := m.installing.View()
		// isInstallingSyncSupported mirrors tui.go:isSyncSupported - ensure zero CSI 2026 when disabled
		if !isInstallingSyncSupported() {
			view = strings.ReplaceAll(view, "\x1b[?2026h", "")
			view = strings.ReplaceAll(view, "\x1b[?2026l", "")
		}
		// Dumb terminal plain fallback: zero ANSI when TERM=dumb per REQ-TUI-PIPE-003
		if os.Getenv("TERM") == "dumb" || os.Getenv("BIGGZ_PRETTY") == "0" {
			view = ansi.Strip(view)
			return view
		}
		return view

	case stepInstallDone:
		r := m.result
		b.WriteString(styles.SuccessBox.Render(fmt.Sprintf(
			"✅ biggz-ai installed successfully in %s",
			m.agent)))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Skills deployed:  %d\n", r.SkillsDeployed))
		b.WriteString(fmt.Sprintf("  Commands written: %d\n", r.CommandsWritten))
		b.WriteString(fmt.Sprintf("  Config merged:    %v\n", r.ConfigMerged))
		b.WriteString("\n")
		b.WriteString("Next steps:\n")
		b.WriteString("  • Run /sdd-new in OpenCode to start a change\n")
		b.WriteString("  • Run 'biggz sdd-status' to check SDD status\n")
		b.WriteString("  • Run 'biggz tdd enable' to enable Strict TDD\n")
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Press [R] to reinstall · esc for menu"))

	case stepInstallError:
		b.WriteString(styles.ErrorBox.Render(fmt.Sprintf("❌ Install failed: %s", m.errMsg)))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("Press [R] to retry · esc for menu"))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("esc back · ctrl+c quit"))
	return styles.AppStyle.Render(b.String())
}
