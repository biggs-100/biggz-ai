package screens

import (
	"fmt"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// HelpContent describes the help for a screen.
type HelpContent struct {
	Title     string
	Keys      []HelpKey
	Paragraph string
}

// HelpKey describes a keyboard shortcut.
type HelpKey struct {
	Key  string
	Desc string
}

// helpData maps screen IDs to help content.
var helpData = map[int]HelpContent{
	0: {
		Title: "Dashboard",
		Keys: []HelpKey{
			{"↑↓", "Navigate actions"},
			{"ENTER", "Select action"},
			{"s", "Search memories"},
			{"r", "Recent observations"},
			{"e", "Browse sessions"},
			{"i", "Install agent plugin"},
			{"?", "Toggle this help"},
			{"q / ESC", "Quit"},
			{"CTRL+C", "Force quit"},
		},
		Paragraph: "biggs-ai is a Review-Driven Development harness for AI coding agents. Dashboard shows memory stats, projects, and quick actions.",
	},
	12: {
		Title: "Welcome / System Menu",
		Keys: []HelpKey{
			{"↑↓", "Navigate menu items"},
			{"ENTER", "Select highlighted item"},
			{"d", "Return to dashboard"},
			{"?", "Toggle this help"},
			{"ESC", "Back to dashboard"},
			{"CTRL+C", "Force quit"},
		},
		Paragraph: "Full system management menu. Configure, backup, update, uninstall, and manage biggz-ai settings.",
	},
	1: {
		Title: "Install",
		Keys: []HelpKey{
			{"ENTER", "Start installation"},
			{"↑↓", "Select agent (if multiple detected)"},
			{"R", "Retry after error / reinstall"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Installs biggz-ai in your AI coding agent. Detects OpenCode, Claude Code, or Qwen Code. Deploys skills, commands, MCP, persona, and agent configuration.",
	},
	2: {
		Title: "Configuration",
		Keys: []HelpKey{
			{"← →", "Switch between tabs (Models / Persona / Delivery / Save)"},
			{"↑↓", "Navigate items in current tab"},
			{"ENTER", "Select / toggle / save"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Configure model assignments for each SDD phase, choose the agent persona, and set delivery strategy and review budget. All changes are persisted to opencode.json.",
	},
	3: {
		Title: "Status",
		Keys: []HelpKey{
			{"R", "Refresh all status data"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Shows real-time status of RDD (Review-Driven Development), active SDD changes, and system health. Data is fetched from the live system on refresh.",
	},
	4: {
		Title: "BigMem Memory",
		Keys: []HelpKey{
			{"R", "Load memories from BigMem"},
			{"↑↓", "Navigate memory list"},
			{"ENTER", "View memory details"},
			{"ESC", "Back to list / main menu"},
		},
		Paragraph: "Browse persistent memories stored in BigMem. Each entry is a decision, bug fix, discovery, or convention saved automatically by the orchestrator or sub-agents.",
	},
	5: {
		Title: "Backup & Restore",
		Keys: []HelpKey{
			{"ENTER", "Create or restore a backup"},
			{"↑↓", "Select backup entry"},
			{"R", "Refresh backup list"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Create snapshots of biggz-ai state and restore them later. Backups include skills, configuration, and persistent memory.",
	},
	6: {
		Title: "Profiles",
		Keys: []HelpKey{
			{"R", "Refresh profile list"},
			{"S", "Save current config as a named profile"},
			{"ENTER", "Load selected profile"},
			{"↑↓", "Navigate profile list"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Save and load named configurations. Profiles store model assignments, delivery strategy, review budget, and persona settings as a complete snapshot of opencode.json.",
	},
	7: {
		Title: "Update",
		Keys: []HelpKey{
			{"ENTER", "Check for updates"},
			{"R", "Refresh check"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Check for new versions of biggz-ai on GitHub. When an update is available, you can download and install it directly from this screen.",
	},
	8: {
		Title: "Uninstall",
		Keys: []HelpKey{
			{"ENTER", "Start uninstall"},
			{"Y", "Confirm uninstall"},
			{"N", "Cancel uninstall"},
			{"R", "Retry after error"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Remove biggz-ai from your system. Deletes ~/.biggz/, removes the biggz-orchestrator agent from opencode.json, and strips the persona from AGENTS.md. gentle-ai data is NOT affected.",
	},
	9: {
		Title: "Strict TDD",
		Keys: []HelpKey{
			{"ENTER", "Load state / toggle mode"},
			{"↑↓", "Toggle enable/disable"},
			{"R", "Refresh"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Strict TDD mode requires tests to pass before code can be committed. When enabled, sdd-apply will enforce test passing before accepting changes.",
	},
	10: {
		Title: "Review Timeline",
		Keys: []HelpKey{
			{"ENTER", "Load lineages / view detail"},
			{"R", "Refresh lineage list"},
			{"↑↓", "Navigate lineages"},
			{"ESC", "Back to list / main menu"},
		},
		Paragraph: "Browse review lineages stored in the review transaction store. Each lineage represents a complete review lifecycle with events, findings, and corrections.",
	},
	18: {
		Title: "Sync Configuration",
		Keys: []HelpKey{
			{"ENTER", "Start sync"},
			{"C", "Toggle combined upgrade"},
			{"ESC", "Back"},
		},
		Paragraph: "Sync biggz-ai configuration: re-deploy skills, re-apply overlays, re-inject persona. Optionally combined with upgrade check.",
	},
	19: {
		Title: "Update Available",
		Keys: []HelpKey{
			{"ENTER / ESC", "Dismiss"},
		},
		Paragraph: "A new version of biggz-ai is available. Dismiss to continue, or run 'biggz update' from the terminal.",
	},
	20: {
		Title: "Plugin Uninstall",
		Keys: []HelpKey{
			{"↑↓", "Navigate"},
			{"ENTER", "Uninstall"},
			{"ESC", "Back"},
		},
		Paragraph: "Manage installed OpenCode community plugins. Select a plugin to uninstall with confirmation.",
	},
	14: {
		Title: "Recovery Trace",
		Keys: []HelpKey{
			{"ENTER", "Load ledgers / view detail"},
			{"R", "Refresh list"},
			{"↑↓", "Navigate / scroll rows"},
			{"ESC", "Back to list / dashboard"},
		},
		Paragraph: "Browse recovery trace ledgers. Each ledger documents file disposition decisions across releases — KEEP, TRANSPLANT, REWRITE, DELETE, REGENERATE, or DEFER — with contributor credit and invariant proof.",
	},
	11: {
		Title: "Sessions",
		Keys: []HelpKey{
			{"ENTER", "Load session list"},
			{"R", "Refresh"},
			{"↑↓", "Navigate sessions"},
			{"ESC", "Back to main menu"},
		},
		Paragraph: "Browse BigMem sessions. Each session groups observations from a single coding conversation.",
	},
	16: {
		Title: "Agent Builder",
		Keys: []HelpKey{
			{"↑↓ / j k", "Navigate options"},
			{"ENTER", "Select option"},
			{"TAB", "Continue from prompt (when text is entered)"},
			{"↑↓", "Scroll SKILL.md preview content"},
			{"ESC", "Back one step"},
		},
		Paragraph: "Create custom sub-agent skills with AI: choose a generation engine, describe the agent, optionally attach it to an SDD phase, preview the generated SKILL.md, then install it to your agents.",
	},
	15: {
		Title: "Model Picker",
		Keys: []HelpKey{
			{"↑↓ / j k", "Navigate agents, providers, models, efforts"},
			{"ENTER", "Select / assign model"},
			{"ESC", "Back one step / main menu"},
		},
		Paragraph: "Assign AI models to every configurable agent: the orchestrator, SDD phases, Judgment Day agents, and review agents. Driven by the OpenCode model cache; effort levels come from the biggz model-variants plugin cache. Changes are persisted to opencode.json.",
	},
}

// GetHelp returns the help content for a screen ID.
func GetHelp(screenID int) HelpContent {
	if h, ok := helpData[screenID]; ok {
		return h
	}
	return HelpContent{Title: "Unknown Screen", Keys: []HelpKey{{"ESC", "Go back"}}}
}

// HelpModel provides searchable help with textinput + viewport.
type HelpModel struct {
	input    textinput.Model
	viewport viewport.Model
	filter   string
	filtered []HelpContent
	focused  bool
	width    int
	height   int
}

// NewHelpModel creates the help screen.
func NewHelpModel() HelpModel {
	ti := textinput.New()
	ti.Placeholder = "Filter help..."
	ti.CharLimit = 64
	ti.Width = 30
	vp := viewport.New(60, 10)
	vp.SetContent(buildHelpContentCached(filterHelp(""), 80))
	return HelpModel{
		input:    ti,
		viewport: vp,
		filtered: filterHelp(""),
		width:    80,
		height:   24,
	}
}

// Init initializes the help model.
func (m HelpModel) Init() tea.Cmd { return textinput.Blink }

// IsFocused reports whether filter input is focused (for tui help-toggle suppression).
func (m HelpModel) IsFocused() bool { return m.focused }

// Filter returns current filter (for tests).
func (m HelpModel) Filter() string { return m.filter }

// Filtered returns filtered contents (for tests).
func (m HelpModel) Filtered() []HelpContent { return m.filtered }

// filterHelp filters helpData case-insensitively across Title/Keys/Paragraph.
func filterHelp(q string) []HelpContent {
	if q == "" {
		out := make([]HelpContent, 0, len(helpData))
		for _, v := range helpData {
			out = append(out, v)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
		return out
	}
	ql := strings.ToLower(q)
	var out []HelpContent
	for _, v := range helpData {
		if strings.Contains(strings.ToLower(v.Title), ql) || strings.Contains(strings.ToLower(v.Paragraph), ql) {
			out = append(out, v)
			continue
		}
		matched := false
		for _, k := range v.Keys {
			if strings.Contains(strings.ToLower(k.Key), ql) || strings.Contains(strings.ToLower(k.Desc), ql) {
				matched = true
				break
			}
		}
		if matched {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func buildHelpContent(items []HelpContent, width int) string {
	var b strings.Builder
	if len(items) == 0 {
		b.WriteString(styles.StatusInfo.Render("No matches"))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("No help entries match your filter."))
		return b.String()
	}
	for _, h := range items {
		title := TruncateToWidth(h.Title, width)
		b.WriteString(styles.Section.Render(title))
		b.WriteString("\n")
		if h.Paragraph != "" {
			// Stable markdown preview: repair orphan fences and normalize latex before wrapping.
			para := LatexToUnicode(RepairOrphanClosingFence(h.Paragraph))
			para = TruncateToWidth(para, width)
			lines := WrapTextWithAnsi(para, width)
			for _, l := range lines {
				b.WriteString(l)
				b.WriteString("\n")
			}
		}
		for _, k := range h.Keys {
			line := fmt.Sprintf("  %s  — %s", k.Key, k.Desc)
			b.WriteString(TruncateToWidth(line, width))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// buildHelpContentCached is the LRU-cached path (key: content hash + width, cap 200).
func buildHelpContentCached(items []HelpContent, width int) string {
	return CachedBuildHelpContent(items, width)
}

// Update handles input.
func (m HelpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(0, msg.Width-4)
		m.viewport.Height = max(0, msg.Height-8)
		m.input.Width = max(10, msg.Width-20)
		m.viewport.SetContent(buildHelpContentCached(m.filtered, max(20, m.viewport.Width)))
		return m, nil
	case tea.KeyMsg:
		if m.focused {
			switch msg.String() {
			case "esc":
				if m.filter != "" {
					m.filter = ""
					m.input.SetValue("")
					m.input.Blur()
					m.focused = false
					m.filtered = filterHelp("")
					m.viewport.SetContent(buildHelpContentCached(m.filtered, max(20, m.viewport.Width)))
					return m, nil
				}
				m.input.Blur()
				m.focused = false
				return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
			case "enter":
				m.input.Blur()
				m.focused = false
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			newFilter := m.input.Value()
			if newFilter != m.filter {
				m.filter = newFilter
				m.filtered = filterHelp(m.filter)
				m.viewport.SetContent(buildHelpContentCached(m.filtered, max(20, m.viewport.Width)))
				m.viewport.GotoTop()
			}
			return m, cmd
		}
		switch msg.String() {
		case "/":
			m.focused = true
			m.input.Focus()
			return m, textinput.Blink
		case "esc":
			if m.filter != "" {
				m.filter = ""
				m.input.SetValue("")
				m.filtered = filterHelp("")
				m.viewport.SetContent(buildHelpContentCached(m.filtered, max(20, m.viewport.Width)))
				m.viewport.GotoTop()
				return m, nil
			}
			return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
		case "down", "j":
			m.viewport.LineDown(1)
			return m, nil
		case "up", "k":
			m.viewport.LineUp(1)
			return m, nil
		case "q":
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the help screen.
func (m HelpModel) View() string {
	contentWidth := max(20, m.viewport.Width)
	if contentWidth == 20 && m.width > 4 {
		contentWidth = max(20, m.width-4)
	}
	m.viewport.SetContent(buildHelpContentCached(m.filtered, contentWidth))
	var b strings.Builder
	b.WriteString(styles.Title.Render("Help"))
	b.WriteString("\n\n")
	if m.focused {
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
	} else {
		hint := "/ filter"
		if m.filter != "" {
			hint = fmt.Sprintf("Filter: %s  (ESC clear, / edit)", m.filter)
		}
		b.WriteString(styles.Help.Render(TruncateToWidth(hint, max(20, m.width-4))))
		b.WriteString("\n\n")
	}
	b.WriteString(m.viewport.View())
	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("↑↓/j k scroll · / filter · ESC clear/back · ? help"))
	return styles.AppStyle.Render(b.String())
}

// HelpOverlay renders the help as a styled string.
func HelpOverlay(screenID int) string {
	return HelpOverlayWidth(screenID, 80)
}

// HelpOverlayWidth renders help overlay constrained to width. Used for
// deterministic gallery generation at 80/100c matching live View() wrapping.
func HelpOverlayWidth(screenID int, width int) string {
	if width <= 0 {
		width = 80
	}
	h := GetHelp(screenID)
	h.Paragraph = LatexToUnicode(RepairOrphanClosingFence(h.Paragraph))
	var b strings.Builder
	// Title truncated to width.
	title := TruncateToWidth("Help: "+h.Title, width)
	b.WriteString(styles.Title.Render(title))
	b.WriteString("\n\n")
	if h.Paragraph != "" {
		// Wrap paragraph to width before styling to match View() wrapping.
		lines := WrapTextWithAnsi(h.Paragraph, width)
		for _, l := range lines {
			b.WriteString(styles.StatusInfo.Render(l))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(styles.Section.Render(TruncateToWidth("Keyboard Shortcuts", width)))
	b.WriteString("\n\n")
	for _, k := range h.Keys {
		line := fmt.Sprintf("  %s  — %s", k.Key, k.Desc)
		line = TruncateToWidth(line, width)
		// Render with key style but ensure VisibleWidth stays within budget.
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.Help.Render(TruncateToWidth("Press ? or ESC to close help", width)))
	rendered := styles.AppStyle.Render(b.String())
	// Ensure final ANSI lines respect width (gallery determinism via VisibleWidth).
	var out []string
	for _, l := range strings.Split(rendered, "\n") {
		if VisibleWidth(l) > width {
			l = TruncateToWidth(l, width)
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
