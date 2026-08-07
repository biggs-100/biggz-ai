package screens

import (
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/opencodeplugin"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type pluginStep int

const (
	pluginList     pluginStep = iota
	pluginConfirm
	pluginRunning
	pluginDone
	pluginError
)

// PluginUninstallModel handles OpenCode plugin management from TUI.
type PluginUninstallModel struct {
	step     pluginStep
	cursor   int
	plugins  []opencodeplugin.Plugin
	err      string
	status   string
}

func NewPluginUninstallModel() PluginUninstallModel {
	return PluginUninstallModel{step: pluginList}
}

func (m PluginUninstallModel) Init() tea.Cmd { return func() tea.Msg { return m.loadPlugins() } }

func (m PluginUninstallModel) loadPlugins() tea.Msg {
	installed, err := opencodeplugin.ListInstalled()
	if err != nil {
		return pluginLoadMsg{err: err.Error()}
	}
	var plugins []opencodeplugin.Plugin
	for _, p := range opencodeplugin.KnownPlugins() {
		for _, i := range installed {
			if strings.Contains(i, p.NpmPackage) || strings.Contains(p.NpmPackage, i) {
				plugins = append(plugins, p)
				break
			}
		}
	}
	return pluginLoadMsg{plugins: plugins}
}

type pluginLoadMsg struct {
	plugins []opencodeplugin.Plugin
	err     string
}

type pluginActionResultMsg struct {
	name string
	err  error
}

func (m PluginUninstallModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pluginLoadMsg:
		if msg.err != "" {
			m.err = msg.err
		}
		m.plugins = msg.plugins

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.plugins)-1 {
				m.cursor++
			}
		case "enter", " ":
			switch m.step {
			case pluginList:
				if len(m.plugins) == 0 {
					return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
				}
				m.step = pluginConfirm
			case pluginConfirm:
				m.step = pluginRunning
				plugin := m.plugins[m.cursor]
				return m, func() tea.Msg {
					err := opencodeplugin.Uninstall(plugin.Name)
					return pluginActionResultMsg{name: plugin.Name, err: err}
				}
			}
		case "esc":
			switch m.step {
			case pluginConfirm:
				m.step = pluginList
			default:
				return m, func() tea.Msg { return NavigateMsg{Screen: 0} }
			}
		}

	case pluginActionResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.status = fmt.Sprintf("Plugin %q uninstalled.", msg.name)
		}
		m.step = pluginDone
	}

	return m, nil
}

func (m PluginUninstallModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("OpenCode Plugins"))
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

	switch m.step {
	case pluginList:
		if len(m.plugins) == 0 {
			b.WriteString("No plugins installed.\n\n")
		} else {
			b.WriteString("Installed plugins:\n\n")
			for i, p := range m.plugins {
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				b.WriteString(fmt.Sprintf("%s%s — %s\n", cur, styles.MenuItemKey.Render(p.Name), p.Description))
			}
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ · ENTER uninstall · ESC back"))

	case pluginConfirm:
		p := m.plugins[m.cursor]
		b.WriteString(styles.WarningBox.Render(fmt.Sprintf(
			"Uninstall %s?\n\nThis will remove the plugin and its configuration.", p.Name)))
		b.WriteString("\n\n")
		b.WriteString(styles.Help.Render("ENTER to confirm · ESC back"))

	case pluginRunning:
		b.WriteString(styles.Spinner.Render("Uninstalling plugin..."))

	case pluginDone:
		b.WriteString(styles.Help.Render("Press ESC to return"))
	}

	return styles.AppStyle.Render(b.String())
}
