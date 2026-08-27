package screens

import (
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// ReviewModel shows review lineage history.
type ReviewModel struct {
	lineages []review.LineageInfo
	cursor   int
	detail   *review.LineageStatus
	loaded   bool
	err      string
}

// NewReviewModel creates the review screen.
func NewReviewModel() ReviewModel {
	return ReviewModel{}
}

func (m ReviewModel) Init() tea.Cmd { return nil }

// reviewListMsg carries the lineage list.
type reviewListMsg struct {
	lineages []review.LineageInfo
	err      string
}

// reviewDetailMsg carries a single lineage detail.
type reviewDetailMsg struct {
	status *review.LineageStatus
	err    string
}

// loadLineages lists all review lineages.
func loadLineages() tea.Msg {
	repoDir, err := os.Getwd()
	if err != nil {
		return reviewListMsg{err: err.Error()}
	}
	auth := review.NewAuthority(repoDir)
	info, err := auth.Inventory()
	if err != nil {
		return reviewListMsg{err: fmt.Sprintf("inventory: %v", err)}
	}
	return reviewListMsg{lineages: info}
}

// loadLineageDetail loads a specific lineage status.
func loadLineageDetail(lineageID string) tea.Msg {
	repoDir, err := os.Getwd()
	if err != nil {
		return reviewDetailMsg{err: err.Error()}
	}
	auth := review.NewAuthority(repoDir)
	status, err := auth.Status(lineageID)
	if err != nil {
		return reviewDetailMsg{err: fmt.Sprintf("status: %v", err)}
	}
	return reviewDetailMsg{status: status}
}

// Update handles input.
func (m ReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loaded = true
			return m, loadLineages
		case "enter", " ":
			if !m.loaded {
				m.loaded = true
				return m, loadLineages
			}
			if len(m.lineages) > 0 {
				return m, func() tea.Msg { return loadLineageDetail(m.lineages[m.cursor].LineageID) }
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.lineages)-1 {
				m.cursor++
			}
		}

	case reviewListMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.lineages = msg.lineages
		m.cursor = 0
		m.err = ""

	case reviewDetailMsg:
		if msg.err != "" {
			m.err = msg.err
			return m, nil
		}
		m.detail = msg.status
	}

	return m, nil
}

// View renders the review timeline.
func (m ReviewModel) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Review Timeline"))
	b.WriteString("\n\n")

	if m.err != "" {
		b.WriteString(styles.ErrorBox.Render(m.err))
		b.WriteString("\n\n")
	}

	if m.detail != nil {
		// Show detail
		lineage := TruncateToWidth(ReplaceTabs(m.detail.LineageID), 60)
		b.WriteString(styles.Section.Render(fmt.Sprintf("Lineage: %s", lineage)))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("  Events:   %d\n", m.detail.EventCount))
		head := m.detail.HeadHash
		if len(head) > 16 {
			head = head[:16]
		}
		b.WriteString(fmt.Sprintf("  Head:     %s\n", head))
		b.WriteString(fmt.Sprintf("  Valid:    %v\n", m.detail.ChainValid))
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ESC to go back to list"))
		return styles.AppStyle.Render(b.String())
	}

	if !m.loaded {
		b.WriteString("Press ENTER or [R] to load review lineages.\n\n")
		b.WriteString(styles.StatusInfo.Render("Shows all review transactions for this repository."))
	} else if len(m.lineages) == 0 {
		b.WriteString(styles.StatusInfo.Render("No reviews yet. Start a review with 'biggz review start'."))
	} else {
		b.WriteString(styles.Section.Render(fmt.Sprintf("Lineages (%d)", len(m.lineages))))
		b.WriteString("\n\n")
		for i, li := range m.lineages {
			cur := "  "
			if i == m.cursor {
				cur = "▸ "
			}
			id := TruncateToWidth(ReplaceTabs(li.LineageID), 50)
			line := fmt.Sprintf("%s%s  [%s]", cur, id, li.State)
			line = TruncateToWidth(line, 80)
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.StatusInfo.Render("ENTER to view details"))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Help.Render("[R] refresh · ↑↓ navigate · ENTER detail · ESC back · ? help"))
	return styles.AppStyle.Render(b.String())
}
