package screens

import (
	"fmt"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// RenderWizardComplete renders the success summary. Field layout reuses the
// Done-view fields (agent, skills deployed, commands written, config merged)
// so the wizard and legacy flows report the same result shape.
func RenderWizardComplete(agent string, skillsDeployed, commandsWritten int, configMerged bool) string {
	var b strings.Builder
	if agent == "" {
		agent = "your agent"
	}
	b.WriteString(styles.SuccessBox.Render(fmt.Sprintf(
		"✅ biggz-ai installed successfully in %s",
		agent)))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Skills deployed:  %d\n", skillsDeployed))
	b.WriteString(fmt.Sprintf("  Commands written: %d\n", commandsWritten))
	b.WriteString(fmt.Sprintf("  Config merged:    %v\n", configMerged))
	b.WriteString("\n")
	b.WriteString("Next steps:\n")
	b.WriteString("  • Run /sdd-new in OpenCode to start a change\n")
	b.WriteString("  • Run 'biggz sdd-status' to check SDD status\n")
	b.WriteString("  • Run 'biggz tdd enable' to enable Strict TDD\n")
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("esc back · ctrl+c quit"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
