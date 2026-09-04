package screens

import (
	"os"
	"strconv"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
)

// Shared helpers for batch-1 wizard screens (Welcome → Preset).
//
// Wizard views are pure Render* funcs with state in InstallModel. They reuse
// internal/tui/styles tokens only and honor reduced-motion / dumb-terminal
// guards: CSI 2026 wrappers are stripped when sync is unsupported and all
// ANSI is stripped when TERM=dumb or BIGGZ_PRETTY=0.

// wizardSyncSupported mirrors installing.go:isInstallingSyncSupported —
// BIGGZ_PRETTY=0, PI_SUBAGENT_CHILD=1, animation disabled, TERM empty/dumb
// means no CSI 2026 sync wrappers.
func wizardSyncSupported() bool {
	if os.Getenv("BIGGZ_PRETTY") == "0" {
		return false
	}
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return false
	}
	if os.Getenv("BIGGZ_NO_ANIMATION") == "1" || os.Getenv("GENTLE_AI_NO_ANIMATION") == "1" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	return true
}

// wizardGuardView strips CSI 2026 when sync is unsupported and all ANSI when
// TERM=dumb or BIGGZ_PRETTY=0 (REQ-WIZ-004).
func wizardGuardView(out string) string {
	if !wizardSyncSupported() {
		out = strings.ReplaceAll(out, "\x1b[?2026h", "")
		out = strings.ReplaceAll(out, "\x1b[?2026l", "")
	}
	if os.Getenv("TERM") == "dumb" || os.Getenv("BIGGZ_PRETTY") == "0" {
		out = ansi.Strip(out)
	}
	return out
}

// wizardStepsList is the full 10-stage order shown on the welcome screen.
var wizardStepsList = []string{
	"Welcome",
	"Detection",
	"Agents",
	"Persona",
	"Preset",
	"Dependency tree",
	"Skills",
	"Review",
	"Installing",
	"Complete",
}

// RenderWizardWelcome renders the wizard entry screen: title + step list.
// Branding banners are excluded by design (REQ-WIZ-005).
func RenderWizardWelcome() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString("This guided wizard will walk you through:\n\n")
	for i, s := range wizardStepsList {
		b.WriteString("  " + strconv.Itoa(i+1) + ". " + s + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("ENTER to begin · esc quit"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
