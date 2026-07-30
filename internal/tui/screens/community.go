package screens

import (
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type cvView int

const (
	cvList cvView = iota
	cvSkillPicker
)

// CommunityScreen lists available community plugins and tools.
type CommunityScreen struct {
	view   cvView
	cursor int
	err    string
}

var communityPlugins = []struct {
	name        string
	desc        string
	category    string
}{
	{"biggz-mcp", "BigMem MCP server (22 memory tools)", "core"},
	{"context7", "Documentation search via Context7", "tools"},
	{"grep-by-vercel", "Code search across GitHub", "tools"},
	{"sentry", "Error tracking and performance monitoring", "monitoring"},
	{"brave-search", "Web search via Brave", "search"},
}

var communitySkills = []struct {
	name        string
	desc        string
}{
	{"branch-pr", "Create PRs with issue-first checks"},
	{"chained-pr", "Split PRs into reviewable slices"},
	{"judgment-day", "Blind dual review protocol"},
	{"cognitive-doc-design", "Design docs that reduce cognitive load"},
	{"go-testing", "Go testing patterns and golden files"},
	{"work-unit-commits", "Plan commits as work units"},
}

func NewCommunityScreen() CommunityScreen {
	return CommunityScreen{view: cvList}
}

func (m CommunityScreen) Init() tea.Cmd { return nil }

func (m CommunityScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 { m.cursor-- }
		case "down", "j":
			max := len(communityPlugins) - 1
			if m.view == cvSkillPicker {
				max = len(communitySkills) - 1
			}
			if m.cursor < max { m.cursor++ }
		case "s":
			if m.view == cvList { m.view = cvSkillPicker; m.cursor = 0 }
		case "left", "right":
			if m.view == cvSkillPicker { m.view = cvList; m.cursor = 0 }
		case "esc":
			if m.view == cvSkillPicker { m.view = cvList; m.cursor = 0; return m, nil }
			return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
		}
	}
	return m, nil
}

func (m CommunityScreen) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Community"))
	b.WriteString("\n\n")

	switch m.view {
	case cvSkillPicker:
		b.WriteString(styles.Section.Render("Available Skills"))
		b.WriteString("\n\n")
		for i, s := range communitySkills {
			cur := "  "
			if i == m.cursor { cur = "▸ " }
			b.WriteString(fmt.Sprintf("%s%s — %s\n", cur, styles.MenuItemKey.Render(s.name), s.desc))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("← → switch tab · ESC back"))

	default:
		b.WriteString(styles.Section.Render("Plugins & Tools"))
		b.WriteString("\n\n")
		for i, p := range communityPlugins {
			cur := "  "
			if i == m.cursor { cur = "▸ " }
			b.WriteString(fmt.Sprintf("%s[%s] %s\n", cur, p.category, styles.MenuItemKey.Render(p.name)))
			b.WriteString(fmt.Sprintf("     %s\n", p.desc))
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · [S]kills tab · ESC back"))
	}
	return styles.AppStyle.Render(b.String())
}
