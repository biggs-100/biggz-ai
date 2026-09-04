package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// RenderWizardDetection renders the result of detectAdapters(): one line per
// detected agent, or a no-agent notice. Pure render — detection itself stays
// in InstallModel.detectAdapters.
func RenderWizardDetection(names []string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Detecting AI coding agents"))
	b.WriteString("\n\n")
	if len(names) == 0 {
		b.WriteString(styles.StatusDisabled.Render("No AI agent detected (tried opencode, claude, qwen, cursor, windsurf)"))
		b.WriteString("\n")
	} else {
		for _, n := range names {
			b.WriteString("  ✓ " + n + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("ENTER to continue · esc back"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
