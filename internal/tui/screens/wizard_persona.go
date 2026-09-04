package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// WizardOption is one radio choice (name + one-line description).
type WizardOption struct {
	Name        string
	Description string
}

// WizardPersonaOptions uses biggz personas — never gentleman (REQ-WIZ-005:
// no gentle-ai branding; design mandates biggz personas).
var WizardPersonaOptions = []WizardOption{
	{Name: "biggz", Description: "Senior architect mentor — direct, caring, concepts first"},
	{Name: "neutral", Description: "Neutral assistant tone"},
	{Name: "custom", Description: "Define your own persona later"},
}

// RenderWizardPersona renders the persona radio. cursor is the highlighted
// row; current is the already-confirmed choice ("", none yet).
func RenderWizardPersona(cursor int, current string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Choose the agent persona"))
	b.WriteString("\n\n")
	for i, opt := range WizardPersonaOptions {
		cur := "  "
		if i == cursor {
			cur = styles.Cursor
		}
		radio := "( )"
		if opt.Name == current {
			radio = "(*)"
		}
		b.WriteString(cur + radio + " " + opt.Name + " — " + opt.Description + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · ENTER select · esc back"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
