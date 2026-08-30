package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/backup"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/bubbles/table"
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

// BackupModel manages snapshots with table, preview and confirm modal.
type BackupModel struct {
	step           backupStep
	items          []backupEntry
	table          table.Model
	cursor         int
	width          int
	height         int
	err            string
	status         string
	preview        *backupEntry
	confirmPending bool
	backupDir      string
	restoreTarget  string
}

type backupEntry struct {
	ID        string
	Time      time.Time
	Size      string
	SizeBytes int64
	Path      string
	Paths     []string
}

// NewBackupModel creates the backup screen.
func NewBackupModel() BackupModel {
	cols := []table.Column{
		{Title: "ID", Width: 28},
		{Title: "Size", Width: 10},
		{Title: "Date", Width: 16},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(7),
	)
	s := table.DefaultStyles()
	s.Header = styles.TableHeader
	s.Selected = styles.TableSelected
	s.Cell = s.Cell.Padding(0, 1)
	t.SetStyles(s)
	return BackupModel{
		step:  backupIdle,
		table: t,
	}
}

// SetBackupDir sets custom backup dir for testing (isolated temp dir).
func (m *BackupModel) SetBackupDir(dir string) { m.backupDir = dir }

// SetRestoreTarget sets custom restore target for testing.
func (m *BackupModel) SetRestoreTarget(dir string) { m.restoreTarget = dir }

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

func (m BackupModel) listBackupsCmd() tea.Cmd {
	dir := m.backupDir
	return func() tea.Msg {
		manifests, err := backup.List(dir)
		if err != nil {
			return backupListMsg{err: err.Error()}
		}
		var items []backupEntry
		for _, b := range manifests {
			sizeStr := formatSize(b.Size)
			p := ""
			if dir != "" {
				p = filepath.Join(dir, b.ID+".tar.gz")
			} else {
				home, _ := os.UserHomeDir()
				p = filepath.Join(home, ".biggz", "backups", b.ID+".tar.gz")
			}
			items = append(items, backupEntry{
				ID:        b.ID,
				Time:      b.CreatedAt,
				Size:      sizeStr,
				SizeBytes: b.Size,
				Path:      p,
				Paths:     b.Paths,
			})
		}
		return backupListMsg{items: items}
	}
}

func formatSize(s int64) string {
	switch {
	case s > 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(s)/(1024*1024))
	case s > 1024:
		return fmt.Sprintf("%.1f KB", float64(s)/1024)
	default:
		return fmt.Sprintf("%d bytes", s)
	}
}

func (m BackupModel) createBackupCmd() tea.Cmd {
	dir := m.backupDir
	return func() tea.Msg {
		backupDir := dir
		if backupDir == "" {
			home, _ := os.UserHomeDir()
			backupDir = filepath.Join(home, ".biggz", "backups")
		}
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return backupResultMsg{err: fmt.Sprintf("mkdir: %v", err)}
		}
		home, _ := os.UserHomeDir()
		biggzDir := filepath.Join(home, ".biggz")
		paths := []string{
			filepath.Join(biggzDir, "bigmem"),
			filepath.Join(biggzDir, "rdd-mode.json"),
		}
		result, err := backup.Create(backupDir, paths)
		if err != nil {
			return backupResultMsg{err: fmt.Sprintf("backup failed: %v", err)}
		}
		status := fmt.Sprintf("Backup created: %s (%d bytes)", result.ID, result.Size)
		if len(result.Skipped) > 0 {
			status += fmt.Sprintf(", %d skipped", len(result.Skipped))
		}
		return backupResultMsg{status: status}
	}
}

func (m BackupModel) restoreBackupCmd(id string) tea.Cmd {
	dir := m.backupDir
	target := m.restoreTarget
	return func() tea.Msg {
		backupDir := dir
		if backupDir == "" {
			home, _ := os.UserHomeDir()
			backupDir = filepath.Join(home, ".biggz", "backups")
		}
		restoreDir := target
		if restoreDir == "" {
			home, _ := os.UserHomeDir()
			restoreDir = home
		}
		home, _ := os.UserHomeDir()
		biggzDir := filepath.Join(home, ".biggz")
		paths := []string{
			filepath.Join(biggzDir, "bigmem"),
			filepath.Join(biggzDir, "rdd-mode.json"),
		}
		_, _ = backup.Create(backupDir, paths)
		if err := backup.Restore(backupDir, id, restoreDir); err != nil {
			return backupResultMsg{err: fmt.Sprintf("restore failed: %v", err)}
		}
		return backupResultMsg{status: fmt.Sprintf("Restored %s to %s", id, restoreDir)}
	}
}

// Update handles input.
func (m BackupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := msg.Width - 4
		if w < 40 {
			w = 40
		}
		cols := m.table.Columns()
		if w < 60 {
			cols = []table.Column{
				{Title: "ID", Width: 20},
				{Title: "Size", Width: 8},
				{Title: "Date", Width: 12},
			}
			m.table.SetColumns(cols)
		} else {
			cols = []table.Column{
				{Title: "ID", Width: 28},
				{Title: "Size", Width: 10},
				{Title: "Date", Width: 16},
			}
			m.table.SetColumns(cols)
		}
		m.table.SetWidth(w)
		m.table.SetHeight(max(5, msg.Height-12))
		return m, nil
	case tea.KeyMsg:
		if m.step == backupRestoring && m.confirmPending {
			switch msg.String() {
			case "y", "Y":
				if len(m.items) == 0 || m.cursor < 0 || m.cursor >= len(m.items) {
					m.step = backupError
					m.err = "no selection"
					m.confirmPending = false
					return m, nil
				}
				id := m.items[m.cursor].ID
				m.confirmPending = false
				m.step = backupCreating
				return m, m.restoreBackupCmd(id)
			case "n", "esc":
				m.confirmPending = false
				m.step = backupListing
				m.status = "Restore cancelled"
				return m, nil
			}
			return m, nil
		}
		switch msg.String() {
		case "r":
			m.step = backupListing
			m.err = ""
			return m, m.listBackupsCmd()
		case "enter", " ":
			if m.step == backupIdle {
				m.step = backupListing
				return m, m.listBackupsCmd()
			}
			if m.step == backupListing && len(m.items) > 0 {
				if m.cursor < 0 {
					m.cursor = 0
				}
				if m.cursor >= len(m.items) {
					m.cursor = len(m.items) - 1
				}
				m.step = backupRestoring
				m.confirmPending = true
				if m.cursor >= 0 && m.cursor < len(m.items) {
					m.preview = &m.items[m.cursor]
				}
				return m, nil
			}
		case "c":
			if m.step == backupIdle || m.step == backupListing || m.step == backupDone || m.step == backupError {
				m.step = backupCreating
				m.err = ""
				return m, m.createBackupCmd()
			}
		case "up", "k":
			if m.step == backupListing && m.cursor > 0 {
				m.cursor--
				m.table.MoveUp(1)
				if m.cursor >= 0 && m.cursor < len(m.items) {
					m.preview = &m.items[m.cursor]
				}
			} else if m.step == backupListing {
				m.table.MoveUp(1)
				m.cursor = m.table.Cursor()
				if m.cursor >= 0 && m.cursor < len(m.items) {
					m.preview = &m.items[m.cursor]
				}
			}
			return m, nil
		case "down", "j":
			if m.step == backupListing && m.cursor < len(m.items)-1 {
				m.cursor++
				m.table.MoveDown(1)
				if m.cursor >= 0 && m.cursor < len(m.items) {
					m.preview = &m.items[m.cursor]
				}
			} else if m.step == backupListing {
				m.table.MoveDown(1)
				m.cursor = m.table.Cursor()
				if m.cursor >= 0 && m.cursor < len(m.items) {
					m.preview = &m.items[m.cursor]
				}
			}
			return m, nil
		case "esc":
			if m.step == backupRestoring {
				m.confirmPending = false
				m.step = backupListing
				m.status = "Restore cancelled"
				return m, nil
			}
			if m.step == backupError || m.step == backupDone {
				m.step = backupListing
				return m, m.listBackupsCmd()
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
		var rows []table.Row
		for _, it := range m.items {
			date := it.Time.Format("2006-01-02 15:04")
			rows = append(rows, table.Row{it.ID, it.Size, date})
		}
		m.table.SetRows(rows)
		m.table.SetCursor(0)
		m.cursor = 0
		if len(m.items) > 0 {
			m.preview = &m.items[0]
		} else {
			m.preview = nil
		}
		m.step = backupListing
		m.err = ""
		return m, nil
	case backupResultMsg:
		if msg.err != "" {
			m.err = msg.err
			m.step = backupError
			return m, nil
		}
		m.status = msg.status
		m.step = backupDone
		m.err = ""
		return m, m.listBackupsCmd()
	}
	if m.step == backupListing {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		newCursor := m.table.Cursor()
		if newCursor != m.cursor && newCursor >= 0 && newCursor < len(m.items) {
			m.cursor = newCursor
			m.preview = &m.items[m.cursor]
		}
		return m, cmd
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
			tableView := m.table.View()
			if m.width > 0 {
				lines := strings.Split(tableView, "\n")
				for i, l := range lines {
					lines[i] = TruncateToWidth(l, m.width-2)
				}
				tableView = strings.Join(lines, "\n")
			}
			b.WriteString(tableView)
			b.WriteString("\n\n")
			if m.preview != nil {
				b.WriteString(styles.PreviewPane.Render(m.renderPreview(*m.preview)))
				b.WriteString("\n")
			}
			b.WriteString(styles.StatusInfo.Render("ENTER to restore selected · [C] to create new"))
		}
	case backupCreating:
		b.WriteString(styles.Spinner.Render("Creating backup..."))
	case backupRestoring:
		if m.confirmPending && m.preview != nil {
			b.WriteString(styles.ModalOverlay.Render(m.renderConfirm(*m.preview)))
		} else {
			b.WriteString(styles.Spinner.Render("Restoring backup..."))
		}
	case backupDone:
		b.WriteString(styles.SuccessBox.Render(m.status))
		b.WriteString("\n\nPress [R] to refresh list.")
		if m.preview != nil {
			b.WriteString("\n")
			b.WriteString(styles.PreviewPane.Render(m.renderPreview(*m.preview)))
		}
	case backupError:
		b.WriteString(styles.ErrorBox.Render("Error: " + m.err))
		b.WriteString("\n\nPress [R] to retry.")
	}
	b.WriteString("\n\n")
	help := "[R] refresh · [C] create · ENTER restore/select · ESC back · ? help"
	if m.width > 0 {
		help = TruncateToWidth(help, m.width-2)
	}
	b.WriteString(styles.Help.Render(help))
	content := b.String()
	if m.width > 0 {
		innerW := m.width - 6
		if innerW < 20 {
			innerW = 20
		}
		lines := strings.Split(content, "\n")
		for i, l := range lines {
			if VisibleWidth(l) > innerW {
				lines[i] = TruncateToWidth(l, innerW)
			}
		}
		content = strings.Join(lines, "\n")
	}
	rendered := styles.AppStyle.Render(content)
	if m.width > 0 {
		lines := strings.Split(rendered, "\n")
		for i, l := range lines {
			if VisibleWidth(l) > m.width {
				lines[i] = TruncateToWidth(l, m.width)
			}
		}
		rendered = strings.Join(lines, "\n")
	}
	return rendered
}

func (m BackupModel) renderPreview(e backupEntry) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("ID: %s\n", TruncateToWidth(e.ID, 60)))
	b.WriteString(fmt.Sprintf("Size: %s\n", e.Size))
	b.WriteString(fmt.Sprintf("Date: %s\n", e.Time.Format("2006-01-02 15:04:05")))
	if len(e.Paths) > 0 {
		b.WriteString("Paths:\n")
		for _, p := range e.Paths {
			b.WriteString("  • " + TruncateToWidth(p, 50) + "\n")
		}
	} else if e.Path != "" {
		b.WriteString("Path: " + TruncateToWidth(e.Path, 50) + "\n")
	}
	return b.String()
}

func (m BackupModel) renderConfirm(e backupEntry) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Confirm Restore"))
	b.WriteString("\n\n")
	b.WriteString(m.renderPreview(e))
	b.WriteString("\n")
	b.WriteString(styles.WarningBox.Render("This will overwrite current state. A safety snapshot will be created first."))
	b.WriteString("\n\n")
	b.WriteString("Restore this backup? [y/N]  (y=confirm, n/ESC=cancel)")
	return b.String()
}
