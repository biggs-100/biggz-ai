package screens

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/agentbuilder"
	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type abView int

const (
	abEngine abView = iota
	abPrompt
	abSDD
	abSDDPhase
	abGenerating
	abPreview
	abInstalling
	abComplete
)

// abTickMsg drives the spinner frame while generating/installing.
type abTickMsg struct{}

// abGeneratedMsg is sent when the AI generation goroutine completes.
type abGeneratedMsg struct {
	agent *agentbuilder.GeneratedAgent
	err   error
}

// abInstallDoneMsg is sent when the agent installation goroutine completes.
type abInstallDoneMsg struct {
	results []agentbuilder.InstallResult
	err     error
}

// abSpinnerFrames are the unicode spinner animation frames.
var abSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerChar returns the spinner animation character for the given frame index.
func spinnerChar(frame int) string {
	return abSpinnerFrames[frame%len(abSpinnerFrames)]
}

const noAnimationEnv = "BIGGZ_NO_ANIMATION"

// tuiAnimationsDisabled reports whether spinner animation is disabled via
// BIGGZ_NO_ANIMATION=1 (with GENTLE_AI_NO_ANIMATION=1 kept for compat),
// porting gentle-ai b3dfc1ef reduced-motion fallback.
func tuiAnimationsDisabled() bool {
	return os.Getenv(noAnimationEnv) == "1" || os.Getenv("GENTLE_AI_NO_ANIMATION") == "1"
}

// renderOptions renders a vertical option list with a cursor marker.
func renderOptions(options []string, cursor int) string {
	var b strings.Builder
	for idx, option := range options {
		if idx == cursor {
			b.WriteString("▸ " + styles.MenuItemKey.Render(option) + "\n")
		} else {
			b.WriteString("  " + option + "\n")
		}
	}
	return b.String()
}

// renderRadio renders a radio option with select/focus markers.
func renderRadio(label string, selected, focused bool) string {
	marker := "( )"
	if selected {
		marker = "(*)"
	}
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	return prefix + marker + " " + label + "\n"
}

// AgentBuilderScreen walks through creating a custom agent step by step,
// ported from gentle-ai's agent_builder_* screens. State is screen-local
// (biggz TUI pattern): the screen owns the flow and reports back to the
// dashboard only on exit.
type AgentBuilderScreen struct {
	view             abView
	cursor           int
	spinner          int
	engines          []model.AgentID
	selectedEngine   model.AgentID
	textarea         textarea.Model
	sddMode          agentbuilder.SDDIntegrationMode
	sddTargetPhase   string
	generating       bool
	generationCancel context.CancelFunc
	genErr           error
	generated        *agentbuilder.GeneratedAgent
	conflictWarning  string
	installing       bool
	installErr       error
	installResults   []agentbuilder.InstallResult
	previewScroll    int
	err              string
}

// NewAgentBuilderScreen creates the agent builder with the prompt input ready.
func NewAgentBuilderScreen() AgentBuilderScreen {
	ta := textarea.New()
	ta.Placeholder = "Describe the agent you want to build..."
	ta.SetWidth(60)
	ta.SetHeight(6)
	ta.Focus()
	return AgentBuilderScreen{view: abEngine, sddMode: agentbuilder.SDDStandalone, textarea: ta}
}

func (m AgentBuilderScreen) Init() tea.Cmd { return nil }

func (m AgentBuilderScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case abTickMsg:
		if tuiAnimationsDisabled() || os.Getenv("TERM") == "dumb" || os.Getenv("BIGGZ_PRETTY") == "0" {
			return m, nil
		}
		if m.generating || m.installing {
			m.spinner = (m.spinner + 1) % len(abSpinnerFrames)
			return m, tickCmd()
		}
		return m, nil
	case abGeneratedMsg:
		if !m.generating {
			// Generation was cancelled (esc while generating): ignore the result.
			return m, nil
		}
		m.generating = false
		if msg.err != nil {
			m.genErr = msg.err
			// Stay on the generating screen to show the error.
			return m, nil
		}
		m.generated = msg.agent
		m.genErr = nil
		m.installErr = nil
		m.previewScroll = 0
		// Check for builtin conflict and set warning before showing preview.
		if msg.agent != nil && agentbuilder.HasConflictWithBuiltin(msg.agent.Name) {
			m.conflictWarning = fmt.Sprintf(
				"Warning: '%s' conflicts with a built-in skill. It will be installed as '%s-custom'.",
				msg.agent.Name, msg.agent.Name,
			)
		} else {
			m.conflictWarning = ""
		}
		m.view = abPreview
		m.cursor = 0
		return m, nil
	case abInstallDoneMsg:
		m.installing = false
		if msg.err != nil {
			m.installErr = msg.err
			m.view = abPreview
			m.cursor = 0
		} else {
			m.installResults = msg.results
			m.installErr = nil
			m.view = abComplete
			m.cursor = 0
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width - 6)
		return m, nil
	}
	return m, nil
}

// handleKey processes one keypress for the current view.
func (m AgentBuilderScreen) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.generating {
		switch key {
		case "esc":
			// Cancel generation and return to the prompt.
			if m.generationCancel != nil {
				m.generationCancel()
			}
			m.generating = false
			m.genErr = nil
			m.view = abPrompt
			m.cursor = 0
			return m, nil
		}
		if m.genErr != nil {
			switch key {
			case "j", "down":
				m.cursor = (m.cursor + 1) % 2
			case "k", "up":
				m.cursor = (m.cursor - 1 + 2) % 2
			case "enter":
				if m.cursor == 0 {
					return m.startGeneration()
				}
				// Back.
				m.genErr = nil
				m.view = abPrompt
				m.cursor = 0
				return m, nil
			}
		}
		return m, nil
	}

	switch m.view {
	case abEngine:
		switch key {
		case "j", "down":
			if m.cursor < len(m.engines) {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.cursor < len(m.engines) {
				m.selectedEngine = m.engines[m.cursor]
				m.view = abPrompt
				m.cursor = 0
			} else {
				// "Back" option.
				return m, backToDashboard()
			}
		case "esc":
			return m, backToDashboard()
		}

	case abPrompt:
		// Textarea owns most keys; tab/ctrl+enter continue.
		switch key {
		case "tab", "ctrl+enter":
			if m.textarea.Value() != "" {
				m.view = abSDD
				m.cursor = 0
			}
		case "esc":
			m.view = abEngine
			m.cursor = 0
		default:
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

	case abSDD:
		opts := m.abSDDOptions()
		switch key {
		case "j", "down":
			if m.cursor < len(opts)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			switch m.cursor {
			case 0: // Standalone — no SDD integration.
				m.sddMode = agentbuilder.SDDStandalone
				return m.startGeneration()
			case 1: // Phase Support — enhance an existing SDD phase.
				m.sddMode = agentbuilder.SDDPhaseSupport
				m.view = abSDDPhase
				m.cursor = 0
			default: // Back.
				m.view = abPrompt
				m.cursor = 0
			}
		case "esc":
			m.view = abPrompt
			m.cursor = 0
		}

	case abSDDPhase:
		phases := abSDDPhases()
		switch key {
		case "j", "down":
			if m.cursor < len(phases) {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.cursor < len(phases) {
				m.sddTargetPhase = phases[m.cursor]
				return m.startGeneration()
			}
			// "Back" option.
			m.view = abSDD
			m.cursor = 0
		case "esc":
			m.view = abSDD
			m.cursor = 0
		}

	case abPreview:
		switch key {
		case "up":
			if m.previewScroll > 0 {
				m.previewScroll--
			}
		case "down":
			m.previewScroll++
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "j":
			if m.cursor < 2 {
				m.cursor++
			}
		case "enter":
			switch m.cursor {
			case 0: // Install.
				if m.generated == nil {
					return m, nil
				}
				return m.startInstallation()
			case 1: // Regenerate.
				return m.startGeneration()
			default: // Back.
				m.view = abPrompt
				m.cursor = 0
			}
		case "esc":
			m.view = abPrompt
			m.cursor = 0
		}

	case abComplete:
		switch key {
		case "enter", "q", "esc":
			return m, backToDashboard()
		}
	}

	return m, nil
}

func backToDashboard() tea.Cmd {
	return func() tea.Msg { return NavigateMsg{Screen: 0} }
}

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
	return func() tea.Msg { return abTickMsg{} }
}

// abSDDOptions returns the SDD integration mode labels. SDDNewPhase is
// deferred in biggz (internal/agentbuilder/types.go note), so the mode is
// not offered here.
func (m AgentBuilderScreen) abSDDOptions() []string {
	return []string{
		"Standalone — no SDD integration",
		"Phase Support — enhance an existing SDD phase",
		"Back",
	}
}

// abSDDPhases returns the ordered list of SDD phase names.
func abSDDPhases() []string {
	return []string{
		"explore",
		"propose",
		"spec",
		"design",
		"tasks",
		"apply",
		"verify",
		"archive",
	}
}

// detectAgentBuilderEngines scans for supported AI agent binaries on PATH.
func (m AgentBuilderScreen) detectAgentBuilderEngines() []model.AgentID {
	candidateIDs := []model.AgentID{
		agents.AgentClaudeCode,
		agents.AgentOpenCode,
		agents.AgentGeminiCLI,
		agents.AgentCodexCLI,
	}
	var available []model.AgentID
	for _, id := range candidateIDs {
		engine := agentbuilder.NewEngine(id)
		if engine != nil && engine.Available() {
			available = append(available, id)
		}
	}
	return available
}

// homeDir returns the current user's home directory path.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return ""
}

// agentBuilderSkillsDir returns the skills directory for the given agent.
func agentBuilderSkillsDir(agentID model.AgentID) (string, bool) {
	home := homeDir()
	switch agentID {
	case agents.AgentClaudeCode:
		return filepath.Join(home, ".claude", "skills"), true
	case agents.AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "skills"), true
	case agents.AgentGeminiCLI:
		return filepath.Join(home, ".gemini", "skills"), true
	case agents.AgentCodexCLI:
		return filepath.Join(home, ".codex", "skills"), true
	default:
		return "", false
	}
}

// agentBuilderSystemPromptPath returns the system prompt file path for the
// given agent.
func agentBuilderSystemPromptPath(agentID model.AgentID) (string, bool) {
	home := homeDir()
	switch agentID {
	case agents.AgentClaudeCode:
		return filepath.Join(home, ".claude", "CLAUDE.md"), true
	case agents.AgentOpenCode:
		return filepath.Join(home, ".config", "opencode", "AGENTS.md"), true
	case agents.AgentGeminiCLI:
		return filepath.Join(home, ".gemini", "GEMINI.md"), true
	case agents.AgentCodexCLI:
		return filepath.Join(home, ".codex", "AGENTS.md"), true
	default:
		return "", false
	}
}

// buildAgentBuilderAdapters returns the AdapterInfo list for the detected
// engines. biggz defers multi-agent target plumbing beyond the generation
// engines (see deferred items): install targets are the available engines'
// skills dirs, opencode first.
func (m AgentBuilderScreen) buildAgentBuilderAdapters() []agentbuilder.AdapterInfo {
	var adapters []agentbuilder.AdapterInfo
	for _, id := range m.engines {
		if skillsDir, ok := agentBuilderSkillsDir(id); ok {
			adapters = append(adapters, agentbuilder.AdapterInfo{
				AgentID:   id,
				SkillsDir: skillsDir,
			})
		}
	}
	return adapters
}

// agentBuilderInstallTargets returns the full destination paths for preview.
func (m AgentBuilderScreen) agentBuilderInstallTargets() []string {
	adapters := m.buildAgentBuilderAdapters()
	targets := make([]string, 0, len(adapters))
	for _, a := range adapters {
		if m.generated != nil {
			targets = append(targets, filepath.Join(a.SkillsDir, m.generated.Name, "SKILL.md"))
		} else {
			targets = append(targets, a.SkillsDir)
		}
	}
	return targets
}

// startGeneration launches the AI generation goroutine and transitions to the
// generating view.
func (m AgentBuilderScreen) startGeneration() (tea.Model, tea.Cmd) {
	m.generating = true
	m.genErr = nil
	m.generated = nil
	m.view = abGenerating
	m.cursor = 0
	m.spinner = 0

	engineID := m.selectedEngine
	userInput := m.textarea.Value()

	var sddConfig *agentbuilder.SDDIntegration
	if m.sddMode != agentbuilder.SDDStandalone {
		sddConfig = &agentbuilder.SDDIntegration{
			Mode:        m.sddMode,
			TargetPhase: m.sddTargetPhase,
		}
	}

	// Capture for goroutine.
	capturedSDD := sddConfig
	adapters := m.buildAgentBuilderAdapters()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.generationCancel = cancel

	return m, tea.Batch(tickCmd(), func() tea.Msg {
		defer cancel()

		engine := agentbuilder.NewEngine(engineID)
		if engine == nil {
			return abGeneratedMsg{err: fmt.Errorf("no engine available for %s", engineID)}
		}

		installedAgents := buildInstalledAgentIDs(adapters)
		prompt := agentbuilder.ComposePrompt(userInput, capturedSDD, installedAgents)

		raw, err := engine.Generate(ctx, prompt)
		if err != nil {
			return abGeneratedMsg{err: err}
		}

		agent, err := agentbuilder.Parse(raw)
		if err != nil {
			return abGeneratedMsg{err: err}
		}

		if capturedSDD != nil {
			agent.SDDConfig = capturedSDD
		}

		return abGeneratedMsg{agent: agent}
	})
}

// startInstallation launches the agent installation goroutine.
func (m AgentBuilderScreen) startInstallation() (tea.Model, tea.Cmd) {
	m.installing = true
	m.installErr = nil
	m.view = abInstalling
	m.cursor = 0
	m.spinner = 0

	agent := m.generated
	adapters := m.buildAgentBuilderAdapters()
	engineID := m.selectedEngine

	return m, tea.Batch(tickCmd(), func() (msg tea.Msg) {
		// Recover from panics so the spinner never runs forever.
		defer func() {
			if r := recover(); r != nil {
				msg = abInstallDoneMsg{err: fmt.Errorf("install panicked: %v", r)}
			}
		}()

		// Resolve agent name, applying conflict suffix if needed.
		installAgent := agent
		if agentbuilder.HasConflictWithBuiltin(agent.Name) {
			// Shallow copy so we don't mutate the generated agent in state.
			copy := *agent
			copy.Name = agent.Name + "-custom"
			installAgent = &copy
		}

		results, err := agentbuilder.Install(installAgent, adapters, "")
		if err != nil {
			return abInstallDoneMsg{results: results, err: err}
		}

		// Persist entry to the custom-agent registry (~/.config/biggz/custom-agents.json).
		registryPath := filepath.Join(homeDir(), ".config", "biggz", "custom-agents.json")
		_ = os.MkdirAll(filepath.Dir(registryPath), 0755)
		if reg, loadErr := agentbuilder.LoadRegistry(registryPath); loadErr == nil {
			// Collect IDs of agents that were successfully installed.
			var installedIDs []model.AgentID
			for _, r := range results {
				if r.Success {
					installedIDs = append(installedIDs, r.AgentID)
				}
			}
			entry := agentbuilder.RegistryEntry{
				Name:             installAgent.Name,
				Title:            installAgent.Title,
				Description:      installAgent.Description,
				CreatedAt:        time.Now(),
				GenerationEngine: engineID,
				SDDIntegration:   installAgent.SDDConfig,
				InstalledAgents:  installedIDs,
			}
			// Update existing entry if present; otherwise append.
			if existing := reg.FindByName(installAgent.Name); existing != nil {
				existing.Title = entry.Title
				existing.Description = entry.Description
				existing.CreatedAt = entry.CreatedAt
				existing.GenerationEngine = entry.GenerationEngine
				existing.SDDIntegration = entry.SDDIntegration
				existing.InstalledAgents = entry.InstalledAgents
			} else {
				reg.Add(entry)
			}
			// Best-effort save — ignore save errors.
			_ = agentbuilder.SaveRegistry(registryPath, reg)
		}

		// Wire SDD injection: append custom-agent reference blocks to system
		// prompts. Best-effort — don't fail the whole install if it fails.
		if installAgent.SDDConfig != nil && installAgent.SDDConfig.Mode != agentbuilder.SDDStandalone {
			for _, adapter := range adapters {
				if systemPromptPath, ok := agentBuilderSystemPromptPath(adapter.AgentID); ok {
					_ = agentbuilder.InjectSDDReference(installAgent, systemPromptPath)
				}
			}
		}

		return abInstallDoneMsg{results: results, err: nil}
	})
}

// buildInstalledAgentIDs returns the list of AgentIDs from the adapter list.
func buildInstalledAgentIDs(adapters []agentbuilder.AdapterInfo) []model.AgentID {
	ids := make([]model.AgentID, 0, len(adapters))
	for _, a := range adapters {
		ids = append(ids, a.AgentID)
	}
	return ids
}

func (m AgentBuilderScreen) View() string {
	var b strings.Builder

	switch m.view {
	case abEngine:
		b.WriteString(styles.Title.Render("Choose Your AI Engine"))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("Which installed agent should help you build your sub-agent?"))
		b.WriteString("\n\n")
		if len(m.engines) == 0 {
			b.WriteString(styles.StatusDisabled.Render("No supported AI agent binaries found on PATH."))
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("Install claude, opencode, gemini, or codex and try again."))
			b.WriteString("\n\n")
			b.WriteString(renderOptions([]string{"Back"}, m.cursor))
		} else {
			opts := make([]string, 0, len(m.engines)+1)
			for _, e := range m.engines {
				opts = append(opts, string(e))
			}
			opts = append(opts, "Back")
			b.WriteString(renderOptions(opts, m.cursor))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("j/k: navigate • enter: select • esc: back"))

	case abPrompt:
		b.WriteString(styles.Title.Render("Describe Your Agent"))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("What should your custom agent do? Some ideas:"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("  • Review CSS for a11y issues"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("  • Generate API docs from code"))
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("  • Validate DB migrations"))
		b.WriteString("\n\n")
		b.WriteString(m.textarea.View())
		b.WriteString("\n\n")
		if m.textarea.Value() == "" {
			b.WriteString(styles.StatusInfo.Render("(type a description to continue)"))
		} else {
			b.WriteString(styles.StatusEnabled.Render("tab: continue (enter adds a new line)"))
		}
		b.WriteString("\n\n")
		b.WriteString(renderOptions([]string{"Back"}, -1))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("type your description • tab to continue • esc: back"))

	case abSDD:
		b.WriteString(styles.Title.Render("SDD Integration"))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("How should your agent integrate with the SDD workflow?"))
		b.WriteString("\n\n")
		opts := m.abSDDOptions()
		for idx, opt := range opts {
			focused := idx == m.cursor
			var isSelected bool
			switch idx {
			case 0:
				isSelected = m.sddMode == agentbuilder.SDDStandalone
			case 1:
				isSelected = m.sddMode == agentbuilder.SDDPhaseSupport
			}
			if idx < len(opts)-1 {
				b.WriteString(renderRadio(opt, isSelected, focused))
			} else {
				b.WriteString(renderOptions([]string{opt}, m.cursor-len(opts)+1))
			}
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("j/k: navigate • enter: select • esc: back"))

	case abSDDPhase:
		b.WriteString(styles.Title.Render("Select SDD Phase"))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("Support which phase? (your agent will enhance this phase)"))
		b.WriteString("\n\n")
		phases := abSDDPhases()
		allOpts := make([]string, 0, len(phases)+1)
		for _, phase := range phases {
			allOpts = append(allOpts, "Support phase: "+phase)
		}
		allOpts = append(allOpts, "Back")
		b.WriteString(renderOptions(allOpts, m.cursor))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("j/k: navigate • enter: select • esc: back"))

	case abGenerating:
		b.WriteString(styles.Title.Render("Generating Your Agent..."))
		b.WriteString("\n\n")
		if m.genErr != nil {
			b.WriteString(styles.StatusDisabled.Render("✗ Generation failed"))
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("  Engine: " + string(m.selectedEngine)))
			b.WriteString("\n")
			b.WriteString(styles.StatusDisabled.Render("  Error: " + m.genErr.Error()))
			b.WriteString("\n\n")
			b.WriteString(renderOptions([]string{"Retry", "Back"}, m.cursor))
			b.WriteString("\n")
			b.WriteString(styles.Help.Render("enter: select • esc: back"))
			break
		}
		b.WriteString(styles.StatusInfo.Render(spinnerChar(m.spinner) + "  Running " + string(m.selectedEngine) + "..."))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("Composing prompt and calling generation engine. This may take a moment."))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("esc: cancel"))

	case abPreview:
		agent := m.generated
		if agent == nil {
			b.WriteString(styles.StatusDisabled.Render("No agent generated yet."))
			break
		}
		if m.installErr != nil {
			b.WriteString(styles.StatusDisabled.Render("✗ Installation failed: " + m.installErr.Error()))
			b.WriteString("\n\n")
		}
		if m.conflictWarning != "" {
			b.WriteString(styles.StatusInfo.Render("⚠ " + m.conflictWarning))
			b.WriteString("\n\n")
		}
		b.WriteString(styles.Title.Render("Preview: " + agent.Title))
		b.WriteString("\n\n")
		b.WriteString(styles.Section.Render("Name:        ") + agent.Name)
		b.WriteString("\n")
		b.WriteString(styles.Section.Render("Description: ") + agent.Description)
		b.WriteString("\n")
		b.WriteString(styles.Section.Render("Trigger:     ") + agent.Trigger)
		b.WriteString("\n")
		if agent.SDDConfig != nil {
			sddInfo := string(agent.SDDConfig.Mode)
			if agent.SDDConfig.TargetPhase != "" {
				sddInfo += " → " + agent.SDDConfig.TargetPhase
			}
			b.WriteString(styles.Section.Render("SDD:         ") + sddInfo)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.Section.Render("SKILL.md content:"))
		b.WriteString("\n")
		lines := strings.Split(agent.Content, "\n")
		visibleLines := 18
		maxScroll := len(lines) - visibleLines
		if maxScroll < 0 {
			maxScroll = 0
		}
		actualScroll := m.previewScroll
		if actualScroll > maxScroll {
			actualScroll = maxScroll
		}
		if actualScroll < 0 {
			actualScroll = 0
		}
		end := actualScroll + visibleLines
		if end > len(lines) {
			end = len(lines)
		}
		for _, line := range lines[actualScroll:end] {
			b.WriteString(styles.StatusInfo.Render("  " + line))
			b.WriteString("\n")
		}
		if len(lines) > visibleLines {
			b.WriteString(styles.Help.Render(fmt.Sprintf(
				"  [lines %d-%d of %d — ↑/↓ to scroll]",
				actualScroll+1, end, len(lines),
			)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		targets := m.agentBuilderInstallTargets()
		if len(targets) > 0 {
			b.WriteString(styles.Section.Render("Will be installed to:"))
			b.WriteString("\n")
			for _, t := range targets {
				b.WriteString(styles.StatusInfo.Render("  " + t))
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		b.WriteString(renderOptions([]string{"Install", "Regenerate", "Back"}, m.cursor))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑/↓: scroll content • j/k: navigate actions • enter: select • esc: back"))

	case abInstalling:
		b.WriteString(styles.Title.Render("Installing Your Agent..."))
		b.WriteString("\n\n")
		if m.installErr != nil {
			b.WriteString(styles.StatusDisabled.Render("✗ Installation failed"))
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("  Engine: " + string(m.selectedEngine)))
			b.WriteString("\n")
			b.WriteString(styles.StatusDisabled.Render("  Error: " + m.installErr.Error()))
			b.WriteString("\n\n")
			b.WriteString(renderOptions([]string{"Back"}, 0))
			b.WriteString("\n")
			b.WriteString(styles.Help.Render("enter: select • esc: back"))
			break
		}
		b.WriteString(styles.StatusInfo.Render(spinnerChar(m.spinner) + "  Writing skill files..."))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("Installing SKILL.md to all detected agents. This should be quick."))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("please wait..."))

	case abComplete:
		b.WriteString(styles.StatusEnabled.Render("✓ Agent Created!"))
		b.WriteString("\n\n")
		if m.generated != nil {
			b.WriteString(styles.Section.Render("Agent: ") + m.generated.Title)
			b.WriteString("\n\n")
		}
		if len(m.installResults) > 0 {
			b.WriteString(styles.Section.Render("Installed to:"))
			b.WriteString("\n")
			for _, r := range m.installResults {
				if r.Success {
					b.WriteString("  " + styles.StatusEnabled.Render("✓") + "  " + string(r.AgentID))
					b.WriteString("\n")
					b.WriteString("     " + styles.StatusInfo.Render(r.Path))
					b.WriteString("\n")
				} else {
					b.WriteString("  " + styles.StatusDisabled.Render("✗") + "  " + styles.StatusDisabled.Render(string(r.AgentID)))
					b.WriteString("\n")
					if r.Err != nil {
						b.WriteString("     " + styles.StatusInfo.Render(r.Err.Error()))
						b.WriteString("\n")
					}
				}
			}
			b.WriteString("\n")
		}
		if m.generated != nil && m.generated.Trigger != "" {
			b.WriteString(styles.Section.Render("How to use:"))
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("  Your agent is active. Trigger it with:"))
			b.WriteString("\n")
			b.WriteString(styles.MenuItemKey.Render("  " + m.generated.Trigger))
			b.WriteString("\n\n")
		}
		b.WriteString(renderOptions([]string{"Done"}, 0))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("enter: return to menu • q: quit"))
	}

	return styles.AppStyle.Render(b.String())
}
