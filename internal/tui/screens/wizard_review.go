package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// RenderWizardReview renders the pre-install summary: agents, persona,
// preset, and skills. A fresh wizard_review.go is used instead of touching
// the legacy review.go (design constraint). Confirm advances to Installing;
// esc retreats to SkillPicker with selections intact.
func RenderWizardReview(agents []string, persona, preset string, skills []string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Review installation"))
	b.WriteString("\n\n")
	if len(agents) == 0 {
		b.WriteString("Agents:  (none selected)\n")
	} else {
		b.WriteString("Agents:  " + strings.Join(agents, ", ") + "\n")
	}
	if persona == "" {
		persona = "(none)"
	}
	b.WriteString("Persona: " + persona + "\n")
	if preset == "" {
		preset = "(none)"
	}
	b.WriteString("Preset:  " + preset + "\n")
	if len(skills) == 0 {
		b.WriteString("Skills:  (all)")
	} else {
		b.WriteString("Skills:  " + strings.Join(skills, ", "))
	}
	b.WriteString("\n\n")
	b.WriteString("The following will be installed:\n")
	b.WriteString("  ✅ SDD orchestrator + 18 sub-agents\n")
	b.WriteString("  ✅ SDD skills\n")
	b.WriteString("  ✅ Slash commands\n")
	b.WriteString("  ✅ BigMem MCP server\n")
	b.WriteString("  ✅ RDD kill switch\n")
	b.WriteString("  ✅ Persona + BigMem protocol in AGENTS.md\n")
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("ENTER to install · esc back"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
