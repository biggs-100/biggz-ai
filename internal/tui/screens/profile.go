package screens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/filemerge"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type profileStep int

const (
	profileIdle profileStep = iota
	profileSaving
	profileLoading
	profileDone
	profileError
)

type profileEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// ProfileModel saves/loads named configurations.
type ProfileModel struct {
	step     profileStep
	profiles []profileEntry
	cursor   int
	name     string
	status   string
	err      string
	editing  bool
}

// NewProfileModel creates the profile screen.
func NewProfileModel() ProfileModel {
	return ProfileModel{step: profileIdle, name: "my-config"}
}

func (m ProfileModel) Init() tea.Cmd { return nil }

// profileListMsg carries the profile list.
type profileListMsg struct {
	profiles []profileEntry
	err      string
}

// profileResultMsg carries operation result.
type profileResultMsg struct {
	status string
	err    string
}

func profilesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".biggz", "profiles.json")
}

// loadProfiles reads saved profiles.
func loadProfiles() tea.Msg {
	p := profilesPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return profileListMsg{profiles: nil}
	}
	var list struct {
		Profiles []profileEntry `json:"profiles"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return profileListMsg{profiles: nil}
	}
	return profileListMsg{profiles: list.Profiles}
}

// saveProfile stores current config as a named profile.
func saveProfile(name string) tea.Msg {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")

	// Read current config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return profileResultMsg{err: fmt.Sprintf("read config: %v", err)}
	}

	// Save to profiles dir
	profDir := filepath.Join(home, ".biggz", "profiles")
	if err := os.MkdirAll(profDir, 0755); err != nil {
		return profileResultMsg{err: fmt.Sprintf("mkdir: %v", err)}
	}

	profilePath := filepath.Join(profDir, name+".json")
	if _, err := filemerge.WriteFileAtomic(profilePath, data, 0644); err != nil {
		return profileResultMsg{err: fmt.Sprintf("write profile: %v", err)}
	}

	return profileResultMsg{status: fmt.Sprintf("Profile %q saved", name)}
}

// loadProfile restores a named profile to opencode.json.
func loadProfile(name string) tea.Msg {
	home, _ := os.UserHomeDir()
	profDir := filepath.Join(home, ".biggz", "profiles")
	profilePath := filepath.Join(profDir, name+".json")

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return profileResultMsg{err: fmt.Sprintf("read profile: %v", err)}
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if _, err := filemerge.WriteFileAtomic(configPath, data, 0644); err != nil {
		return profileResultMsg{err: fmt.Sprintf("write config: %v", err)}
	}

	return profileResultMsg{status: fmt.Sprintf("Profile %q loaded", name)}
}

// Update handles input.
func (m ProfileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, loadProfiles
		case "enter", " ":
			if m.step == profileIdle {
				return m, loadProfiles
			}
			if m.step == profileLoading && len(m.profiles) > 0 {
				p := m.profiles[m.cursor]
				m.step = profileLoading
				return m, func() tea.Msg { return loadProfile(p.Name) }
			}
		case "s":
			if m.step == profileIdle || m.step == profileLoading || m.step == profileDone {
				m.step = profileSaving
				m.editing = true
				return m, nil
			}
		case "up", "k":
			if m.step == profileLoading && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.step == profileLoading && m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "backspace":
			if m.editing && len(m.name) > 0 {
				m.name = m.name[:len(m.name)-1]
			}
		default:
			if m.editing {
				if msg.String() == "enter" {
					name := m.name
					if name == "" {
						name = "my-config"
					}
					m.editing = false
					m.step = profileSaving
					return m, func() tea.Msg { return saveProfile(name) }
				}
				if len(msg.String()) == 1 {
					m.name += msg.String()
				}
			}
		}

	case profileListMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.profiles = msg.profiles
		if m.profiles == nil {
			m.profiles = []profileEntry{}
		}
		m.step = profileLoading
		m.err = ""

	case profileResultMsg:
		if msg.err != "" {
			m.err = msg.err
			m.step = profileError
			return m, nil
		}
		m.status = msg.status
		m.step = profileDone
	}

	return m, nil
}

// View renders.
func (m ProfileModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Profiles"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}

	switch m.step {
	case profileIdle:
		b.WriteString("Save and load named configurations.\n\n")
		b.WriteString(styles.StatusInfo.Render("Profiles store model assignments, delivery strategy,\n"))
		b.WriteString(styles.StatusInfo.Render("review budget, and persona settings.\n\n"))
		b.WriteString("Press ENTER to list profiles, [S] to save current config.\n")

	case profileSaving:
		if m.editing {
			b.WriteString("Enter profile name:\n\n")
			b.WriteString(fmt.Sprintf("  %s█\n", m.name))
			b.WriteString("\nPress ENTER to save, type to edit name.\n")
		} else {
			b.WriteString(styles.Spinner.Render("Saving profile..."))
		}

	case profileLoading:
		if len(m.profiles) == 0 {
			b.WriteString(styles.StatusInfo.Render("No saved profiles. Press [S] to save current config as a profile.\n"))
		} else {
			b.WriteString(styles.Section.Render(fmt.Sprintf("Saved Profiles (%d)", len(m.profiles))))
			b.WriteString("\n\n")
			for i, p := range m.profiles {
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				b.WriteString(fmt.Sprintf("%s%s\n", cur, p.Name))
			}
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("ENTER to load · [S] to save new profile"))
		}

	case profileDone:
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\nPress [R] to refresh list.")

	case profileError:
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\nPress [R] to retry.")
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · [S] save · ENTER load · ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
