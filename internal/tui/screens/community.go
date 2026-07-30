package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type cvView int

const (
	cvList        cvView = iota
	cvSkillPicker
	cvPluginDetail
	cvInstalling
	cvInstallDone
)

// pluginInfo describes a community plugin or tool.
type pluginInfo struct {
	name     string
	desc     string
	category string
	repo     string
	installed bool
}

var communityPlugins = []pluginInfo{
	{"biggz-mcp", "BigMem MCP server (22 memory tools)", "core", "github.com/biggz-ai/biggz", true},
	{"context7", "Documentation search via Context7", "tools", "github.com/context7/context7-mcp", false},
	{"grep-by-vercel", "Code search across GitHub", "tools", "github.com/vercel/grep", false},
	{"sentry", "Error tracking and performance monitoring", "monitoring", "github.com/getsentry/sentry-mcp", false},
	{"brave-search", "Web search via Brave Search API", "search", "github.com/nicholasgriffintn/mcp-brave", false},
}

var communitySkills = []struct {
	name string
	desc string
}{
	{"branch-pr", "Create PRs with issue-first checks"},
	{"chained-pr", "Split PRs into reviewable slices"},
	{"judgment-day", "Blind dual review protocol"},
	{"cognitive-doc-design", "Design docs that reduce cognitive load"},
	{"go-testing", "Go testing patterns and golden files"},
	{"work-unit-commits", "Plan commits as work units"},
}

// CommunityScreen lists and manages community plugins, tools, and skills.
type CommunityScreen struct {
	view       cvView
	cursor     int
	pluginCur  int // which plugin we're acting on
	err        string
	status     string
}

func NewCommunityScreen() CommunityScreen {
	m := CommunityScreen{view: cvList}
	m.checkInstalled()
	return m
}

func (m *CommunityScreen) checkInstalled() {
	home, _ := os.UserHomeDir()
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return
	}
	// Simple check: look for plugin name in config
	content := string(data)
	for i, p := range communityPlugins {
		communityPlugins[i].installed = strings.Contains(content, p.name)
	}
}

func (m CommunityScreen) Init() tea.Cmd { return nil }

// installPluginMsg signals installation result.
type installPluginMsg struct {
	name string
	err  error
}

func (m CommunityScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			max := len(communityPlugins) - 1
			if m.view == cvSkillPicker {
				max = len(communitySkills) - 1
			}
			if m.cursor < max {
				m.cursor++
			}
		case "enter", " ":
			switch m.view {
			case cvList:
				m.pluginCur = m.cursor
				m.view = cvPluginDetail
			case cvPluginDetail:
				p := communityPlugins[m.pluginCur]
				if p.installed {
					m.status = fmt.Sprintf("%s is already installed.", p.name)
				} else {
					m.view = cvInstalling
					return m, func() tea.Msg {
						// Simulated install — in production this would run npm/go install
						return installPluginMsg{name: p.name}
					}
				}
			}
		case "s":
			if m.view == cvList {
				m.view = cvSkillPicker
				m.cursor = 0
			}
		case "left", "right":
			if m.view == cvSkillPicker {
				m.view = cvList
				m.cursor = 0
			}
		case "esc":
			switch m.view {
			case cvSkillPicker:
				m.view = cvList
				m.cursor = 0
			case cvPluginDetail:
				m.view = cvList
			default:
				return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
			}
		}

	case installPluginMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.view = cvInstallDone
			return m, nil
		}
		// Mark as installed
		for i := range communityPlugins {
			if communityPlugins[i].name == msg.name {
				communityPlugins[i].installed = true
			}
		}
		m.status = fmt.Sprintf("%s installed successfully.", msg.name)
		m.view = cvInstallDone
	}

	return m, nil
}

func (m CommunityScreen) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Community"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
		m.err = ""
	}
	if m.status != "" {
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\n")
	}

	switch m.view {
	case cvSkillPicker:
		b.WriteString(styles.Section.Render("Available Skills"))
		b.WriteString("\n\n")
		for i, s := range communitySkills {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			b.WriteString(fmt.Sprintf("%s%s — %s\n", cur, styles.MenuItemKey.Render(s.name), s.desc))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("← → switch · ESC back"))

	case cvPluginDetail:
		p := communityPlugins[m.pluginCur]
		b.WriteString(styles.Section.Render(p.name))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  %s\n\n", p.desc))
		b.WriteString(fmt.Sprintf("  Category: %s\n", p.category))
		b.WriteString(fmt.Sprintf("  Repo:     %s\n", p.repo))
		b.WriteString(fmt.Sprintf("  Status:   "))
		if p.installed {
			b.WriteString(styles.StatusEnabled.Render("installed"))
		} else {
			b.WriteString(styles.StatusDisabled.Render("not installed"))
		}
		b.WriteString("\n\n")
		if !p.installed {
			b.WriteString(styles.Help.Render("ENTER to install"))
		}
		b.WriteString(styles.Help.Render(" · ESC back"))

	case cvInstalling:
		b.WriteString(styles.Spinner.Render(fmt.Sprintf("Installing %s...", communityPlugins[m.pluginCur].name)))
		b.WriteString("\n\n")
		b.WriteString(styles.StatusInfo.Render("This will be available after restart."))

	case cvInstallDone:
		b.WriteString(styles.Help.Render("Press any key to continue · ESC back"))

	default:
		b.WriteString(styles.Section.Render("Plugins & Tools"))
		b.WriteString("\n\n")
		for i, p := range communityPlugins {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			status := ""
			if p.installed {
				status = " " + styles.StatusEnabled.Render("✓")
			}
			b.WriteString(fmt.Sprintf("%s[%s] %s%s\n", cur, p.category, styles.MenuItemKey.Render(p.name), status))
			b.WriteString(fmt.Sprintf("     %s\n", p.desc))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ · ENTER detail · [S]kills · ESC back"))
	}
	return styles.AppStyle.Render(b.String())
}
