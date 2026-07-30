package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/recoverytrace"
	"github.com/biggz-ai/biggz/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

type recView int

const (
	recList recView = iota
	recDetail
)

// RecoveryModel browses recovery trace ledgers.
type RecoveryModel struct {
	store      *recoverytrace.Store
	ledgers    []recoverytrace.LedgerSummary
	cursor     int
	view       recView
	detail     *recoverytrace.Ledgers
	detailName string
	detailID   string
	scroll     int
	err        string
}

// NewRecoveryModel creates the recovery screen.
func NewRecoveryModel() RecoveryModel {
	return RecoveryModel{view: recList}
}

func (m RecoveryModel) Init() tea.Cmd { return nil }

type recListMsg struct {
	ledgers []recoverytrace.LedgerSummary
	err     string
}
type recDetailMsg struct {
	ledgers *recoverytrace.Ledgers
	name    string
	err     string
}

func loadRecList() tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "recovery")
	s, err := recoverytrace.Open(dbPath)
	if err != nil {
		return recListMsg{err: err.Error()}
	}
	defer s.Close()
	ledgers, err := s.ListLedgers("")
	if err != nil {
		return recListMsg{err: err.Error()}
	}
	if ledgers == nil {
		ledgers = []recoverytrace.LedgerSummary{}
	}
	return recListMsg{ledgers: ledgers}
}

func loadRecDetail(id string) tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".biggz", "recovery")
	s, err := recoverytrace.Open(dbPath)
	if err != nil {
		return recDetailMsg{err: err.Error()}
	}
	defer s.Close()
	ledgers, name, _, err := s.GetLedger(id)
	if err != nil {
		return recDetailMsg{err: err.Error()}
	}
	return recDetailMsg{ledgers: ledgers, name: name}
}

func (m RecoveryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.view = recList
			return m, loadRecList
		case "enter", " ":
			if m.view == recList && len(m.ledgers) > 0 {
				l := m.ledgers[m.cursor]
				m.detailID = l.ID
				m.view = recDetail
				m.scroll = 0
				return m, func() tea.Msg { return loadRecDetail(l.ID) }
			}
		case "up", "k":
			if m.view == recList && m.cursor > 0 {
				m.cursor--
			}
			if m.view == recDetail && m.scroll > 0 {
				m.scroll--
			}
		case "down", "j":
			if m.view == recList && m.cursor < len(m.ledgers)-1 {
				m.cursor++
			}
			if m.view == recDetail {
				m.scroll++
			}
		case "esc":
			if m.view == recDetail {
				m.view = recList
				m.detail = nil
				return m, nil
			}
		}

	case recListMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.ledgers = msg.ledgers
		m.err = ""

	case recDetailMsg:
		if msg.err != "" { m.err = msg.err; return m, nil }
		m.detail = msg.ledgers
		m.detailName = msg.name
		m.err = ""
	}

	return m, nil
}

func (m RecoveryModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Recovery Trace"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}

	switch m.view {
	case recDetail:
		if m.detail == nil {
			b.WriteString("Loading...\n")
		} else {
			b.WriteString(styles.Section.Render(fmt.Sprintf("Ledger: %s", m.detailName)))
			b.WriteString("\n\n")
			rec := m.detail.Reconciliation
			b.WriteString(fmt.Sprintf("  Issues:         %d\n", rec.Issues))
			b.WriteString(fmt.Sprintf("  Pull Requests:  %d\n", rec.PullRequests))
			b.WriteString(fmt.Sprintf("  Collision PRs:  %d\n", rec.CollisionPRs))
			b.WriteString(fmt.Sprintf("  Overlaps:       %d\n", rec.Overlaps))
			b.WriteString(fmt.Sprintf("  Decompositions: %d\n\n", rec.Decompositions))

			rows := m.detail.Rows
			if len(rows) > 0 {
				b.WriteString(styles.StatusEnabled.Render(fmt.Sprintf("Recovery Rows (%d):", len(rows))))
				b.WriteString("\n")
				start := m.scroll
				end := start + 10
				if start > len(rows) { start = len(rows) }
				if end > len(rows) { end = len(rows) }
				for _, row := range rows[start:end] {
					path := row.Path
					if len(path) > 45 { path = "..." + path[len(path)-42:] }
					b.WriteString(fmt.Sprintf("  %-12s %s\n", row.Disposition, path))
					if row.Invariant != "" {
						b.WriteString(fmt.Sprintf("              %s\n", row.Invariant))
					}
				}
				if len(rows) > 10 {
					b.WriteString(fmt.Sprintf("  (%d more... scroll with ↑↓)\n", len(rows)-10))
				}
			} else {
				b.WriteString(styles.StatusInfo.Render("No recovery rows in this ledger."))
			}
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ scroll · ESC back"))

	default:
		if len(m.ledgers) == 0 {
			b.WriteString("Press ENTER or [R] to load ledgers.\n\n")
			b.WriteString(styles.StatusInfo.Render("Recovery ledgers track file dispositions across releases."))
		} else {
			b.WriteString(styles.Section.Render(fmt.Sprintf("Recovery Ledgers (%d)", len(m.ledgers))))
			b.WriteString("\n\n")
			for i, l := range m.ledgers {
				cur := "  "
				if i == m.cursor { cur = "▸ " }
				b.WriteString(fmt.Sprintf("%s%s  %-20s  (%d rows)\n", cur, l.ID[:min(24, len(l.ID))], l.Name, l.RowCount))
				b.WriteString(fmt.Sprintf("   %s\n", l.CreatedAt[:10]))
			}
			b.WriteString("\n")
			b.WriteString(styles.StatusInfo.Render("ENTER view details"))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · ENTER view · ESC back"))
	return styles.AppStyle.Render(b.String())
}
