package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/biggz-ai/biggz/internal/backup"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type backupStep int

const (
	backupIdle backupStep = iota
	backupListing
	backupCreating
	backupRestoring
	backupDone
	backupError
)

// BackupModel manages snapshots.
type BackupModel struct {
	step    backupStep
	items   []backupEntry
	cursor  int
	err     string
	status  string
}

type backupEntry struct {
	Name string
	Time time.Time
	Size string
	Path string
}

// NewBackupModel creates the backup screen.
func NewBackupModel() BackupModel {
	return BackupModel{step: backupIdle}
}

func (m BackupModel) Init() tea.Cmd { return nil }

// backupListMsg carries the backup list.
type backupListMsg struct {
	items []backupEntry
	err   string
}

// backupResultMsg carries the result of a backup operation.
type backupResultMsg struct {
	status string
	err    string
}

// listBackups scans ~/.biggz/backups/.
func listBackups() tea.Msg {
	home, _ := os.UserHomeDir()
	backupDir := filepath.Join(home, ".biggz", "backups")

	// Use the backup package to list snapshots
	manifests, err := backup.List(backupDir)
	if err != nil {
		return backupListMsg{items: nil}
	}

	var items []backupEntry
	for _, m := range manifests {
		s := m.Size
		var size string
		switch {
		case s > 1024*1024:
			size = fmt.Sprintf("%.1f MB", float642(s)/(1024*1024))
		case s > 1024:
			size = fmt.Sprintf("%.1f KB", float642(s)/1024)
		default:
			size = fmt.Sprintf("%d bytes", s)
		}
		items = append(items, backupEntry{
			Name: m.ID,
			Time: m.CreatedAt,
			Size: size,
			Path: filepath.Join(backupDir, m.ID+".tar.gz"),
		})
	}
	return backupListMsg{items: items}
}

func float642(n int64) float64 { return float64(n) }

// createBackup creates a new snapshot.
func createBackup() tea.Msg {
	home, _ := os.UserHomeDir()
	backupDir := filepath.Join(home, ".biggz", "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return backupResultMsg{err: fmt.Sprintf("mkdir: %v", err)}
	}

	// Snapshot key biggz paths
	biggzDir := filepath.Join(home, ".biggz")
	patherns := []string{
		filepath.Join(biggzDir, "bigmem"),
		filepath.Join(biggzDir, "rdd-mode.json"),
	}
	result, err := backup.Create(backupDir, patherns)
	if err != nil {
		return backupResultMsg{err: fmt.Sprintf("backup failed: %v", err)}
	}

	return backupResultMsg{status: fmt.Sprintf("Backup created: %s (%d bytes)", result.ID, result.Size)}
}

// Update handles input.
func (m BackupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.step = backupListing
			return m, listBackups
		case "enter", " ":
			if m.step == backupIdle {
				m.step = backupListing
				return m, listBackups
			}
			if m.step == backupListing && len(m.items) > 0 {
				// Restore selected backup (would need confirmation in real app)
				m.step = backupRestoring
				return m, nil
			}
		case "c":
			if m.step == backupIdle || m.step == backupListing || m.step == backupDone || m.step == backupError {
				m.step = backupCreating
				return m, createBackup
			}
		case "up", "k":
			if m.step == backupListing && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.step == backupListing && m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}

	case backupListMsg:
		if msg.err != "" {
			m.err = msg.err
			m.step = backupError
			return m, nil
		}
		m.items = msg.items
		if m.items == nil {
			m.items = []backupEntry{}
		}
		m.step = backupListing
		m.err = ""

	case backupResultMsg:
		if msg.err != "" {
			m.err = msg.err
			m.step = backupError
			return m, nil
		}
		m.status = msg.status
		m.step = backupDone
	}

	return m, nil
}

// View renders the backup screen.
func (m BackupModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Backup & Restore"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}

	switch m.step {
	case backupIdle:
		b.WriteString("Manage biggz-ai snapshots.\n\n")
		b.WriteString(styles.StatusInfo.Render("What gets backed up:\n"))
		b.WriteString("  • BigMem persistent memory (SQLite)\n")
		b.WriteString("  • RDD configuration\n")
		b.WriteString("  • Installed state\n\n")
		b.WriteString("Press ENTER to list existing backups, or [C] to create a new one.\n")

	case backupListing:
		if len(m.items) == 0 {
			b.WriteString(styles.StatusInfo.Render("No backups found. Press [C] to create one.\n"))
		} else {
			b.WriteString(styles.Section.Render(fmt.Sprintf("Existing Backups (%d)", len(m.items))))
			b.WriteString("\n\n")
			for i, item := range m.items {
				cur := "  "
				if i == m.cursor {
					cur = "▸ "
				}
				timeStr := item.Time.Format("Jan 2, 2006 15:04")
				b.WriteString(fmt.Sprintf("%s%s  %s  %s\n", cur, item.Name, timeStr, item.Size))
			}
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("ENTER to restore selected · [C] to create new"))
		}

	case backupCreating:
		b.WriteString(styles.Spinner.Render("Creating backup..."))

	case backupRestoring:
		b.WriteString(styles.Spinner.Render("Restoring backup..."))

	case backupDone:
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\nPress [R] to refresh list.")

	case backupError:
		b.WriteString(styles.ErrorBox.Render("Restore cancelled or failed."))
		b.WriteString("\n\nPress [R] to retry.")
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · [C] create · ENTER restore/select · ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
