package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// WizardPresetOptions mirrors the installer presets in biggz wording:
// Full / DevStack / Minimal / Custom.
var WizardPresetOptions = []WizardOption{
	{Name: "full", Description: "Complete installation — SDD, BigMem, skills, commands, persona"},
	{Name: "devstack", Description: "Dev stack — SDD + BigMem + skills + persona"},
	{Name: "minimal", Description: "Minimal — SDD orchestrator + skills only"},
	{Name: "custom", Description: "Pick components manually in the next steps"},
}

// RenderWizardPreset renders the preset radio. cursor is the highlighted
// row; current is the already-confirmed choice ("", none yet).
func RenderWizardPreset(cursor int, current string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Choose an installation preset"))
	b.WriteString("\n\n")
	for i, opt := range WizardPresetOptions {
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
