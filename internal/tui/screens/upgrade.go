package screens

import (
	"context"
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/biggs-100/biggz-ai/internal/update"
	tea "github.com/charmbracelet/bubbletea"
)

type upgradeStep int

const (
	upgradeIdle upgradeStep = iota
	upgradeChecking
	upgradeReady
	upgradeDownloading
	upgradeDone
	upgradeError
)

// UpgradeModel checks for and applies updates.
type UpgradeModel struct {
	step     upgradeStep
	status   string
	err      string
	release  *update.Release
	currentV string
	latestV  string
}

// NewUpgradeModel creates the upgrade screen.
func NewUpgradeModel() UpgradeModel {
	v := doctor.BuildVersion
	if v == "" {
		v = "dev"
	}
	return UpgradeModel{
		step:     upgradeIdle,
		currentV: v,
	}
}

func (m UpgradeModel) Init() tea.Cmd { return nil }

// upgradeCheckMsg carries the check result.
type upgradeCheckMsg struct {
	release *update.Release
	err     string
}

// upgradeResultMsg carries the download/install result.
type upgradeResultMsg struct {
	status string
	err    string
}

// checkForUpdate calls GitHub releases API.
func checkForUpdate() tea.Msg {
	ctx := context.Background()
	releases, err := update.ListReleases(ctx, "biggz-ai", "biggz")
	if err != nil {
		return upgradeCheckMsg{err: fmt.Sprintf("check failed: %v", err)}
	}
	if len(releases) == 0 {
		return upgradeCheckMsg{err: "no releases found"}
	}
	ch := update.ParseChannel()
	rel := update.SelectRelease(releases, ch)
	if rel == nil {
		return upgradeCheckMsg{err: "no matching release"}
	}
	return upgradeCheckMsg{release: rel}
}

// Update handles input.
func (m UpgradeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.step = upgradeChecking
			m.err = ""
			return m, checkForUpdate
		case "enter", " ":
			if m.step == upgradeIdle {
				m.step = upgradeChecking
				return m, checkForUpdate
			}
			if m.step == upgradeReady {
				m.step = upgradeDownloading
				m.status = fmt.Sprintf("Downloading %s...", m.release.TagName)
				return m, nil
			}
		}

	case upgradeCheckMsg:
		if msg.err != "" {
			m.step = upgradeError
			m.err = msg.err
			return m, nil
		}
		m.release = msg.release
		m.latestV = strings.TrimPrefix(msg.release.TagName, "v")

		if doctor.BuildVersion != "" && doctor.BuildVersion != "dev" {
			cur := strings.TrimPrefix(doctor.BuildVersion, "v")
			if cur == m.latestV {
				m.step = upgradeDone
				m.status = fmt.Sprintf("Already up to date (%s)", msg.release.TagName)
				return m, nil
			}
		}
		m.step = upgradeReady
	}

	return m, nil
}

// View renders.
func (m UpgradeModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Update"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Current version: %s\n", styles.StatusInfo.Render(m.currentV)))
	b.WriteString("\n")

	switch m.step {
	case upgradeIdle:
		b.WriteString("Press ENTER to check for updates, or [R] to refresh.\n")

	case upgradeChecking:
		b.WriteString(styles.Spinner.Render("Checking for updates..."))

	case upgradeReady:
		b.WriteString(styles.Section.Render("Update Available"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Latest:  %s\n", m.release.TagName))
		b.WriteString(fmt.Sprintf("  Current: %s\n", m.currentV))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("Press ENTER to download and install"))

	case upgradeDownloading:
		b.WriteString(styles.Spinner.Render(m.status))

	case upgradeDone:
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\nPress [R] to check again.")

	case upgradeError:
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\nPress [R] to retry.")
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · ENTER check/install · ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
