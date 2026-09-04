package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// WizardAgentOptions is the fixed multi-select list for the Agents stage.
var WizardAgentOptions = []string{"opencode", "claude", "qwen", "cursor", "windsurf"}

// wizardAgentSelected reports whether name is in the selection.
func wizardAgentSelected(selected []string, name string) bool {
	for _, s := range selected {
		if s == name {
			return true
		}
	}
	return false
}

// toggleWizardAgent adds name when absent, removes it when present.
func toggleWizardAgent(selected []string, name string) []string {
	for i, s := range selected {
		if s == name {
			return append(selected[:i], selected[i+1:]...)
		}
	}
	return append(selected, name)
}

// RenderWizardAgents renders the multi-select checklist. cursor is the
// highlighted row; selected holds the chosen agent names.
func RenderWizardAgents(cursor int, selected []string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Select agents to install into"))
	b.WriteString("\n\n")
	for i, name := range WizardAgentOptions {
		cur := "  "
		if i == cursor {
			cur = styles.Cursor
		}
		box := "[ ]"
		if wizardAgentSelected(selected, name) {
			box = "[x]"
		}
		b.WriteString(cur + box + " " + name + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · SPACE toggle · ENTER continue · esc back"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
