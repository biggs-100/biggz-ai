package screens

import (
	"strings"
	"testing"
)

// Task 4.1 RED: reduced-motion compliance (REQ-WIZ-004). New wizard views
// MUST honor BIGGZ_NO_ANIMATION=1 / GENTLE_AI_NO_ANIMATION=1 / TERM=dumb:
// tick command nil, no spinner frames, no CSI 2026 wrappers, zero ANSI
// under dumb. Reuses the installing.go motion helpers; no new style tokens.
func TestWizardReducedMotion(t *testing.T) {
	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", "⠁", "⠂", "⠄", "⠐", "⠠", "⡀"}

	t.Run("tick nil under NO_ANIMATION", func(t *testing.T) {
		t.Setenv("BIGGZ_NO_ANIMATION", "1")
		if installingTickCmd() != nil {
			t.Fatalf("installingTickCmd must be nil under BIGGZ_NO_ANIMATION=1")
		}
		if installingAnimationsDisabled() != true {
			t.Fatalf("installingAnimationsDisabled must be true")
		}
		if isInstallingSyncSupported() {
			t.Fatalf("sync must be unsupported under NO_ANIMATION (CSI 2026 stripped)")
		}
	})

	t.Run("tick nil under gentle flag", func(t *testing.T) {
		t.Setenv("GENTLE_AI_NO_ANIMATION", "1")
		if installingAnimationsDisabled() != true {
			t.Fatalf("installingAnimationsDisabled must honor GENTLE_AI_NO_ANIMATION=1")
		}
		if isInstallingSyncSupported() {
			t.Fatalf("sync must be unsupported under gentle no-animation flag")
		}
	})

	t.Run("no spinner or CSI2026 under NO_ANIMATION", func(t *testing.T) {
		t.Setenv("BIGGZ_NO_ANIMATION", "1")
		views := []string{
			RenderWizardWelcome(),
			RenderWizardDetection([]string{"opencode"}),
			RenderWizardAgents(0, []string{"opencode"}),
			RenderWizardPersona(0, ""),
			RenderWizardPreset(0, ""),
			RenderWizardDepTree(0, "full", nil),
			RenderWizardDepTree(0, "custom", []string{"deploy-skills"}),
			RenderWizardSkills(0, []string{"sdd-apply"}),
			RenderWizardReview([]string{"opencode"}, "bigg", "full", []string{"sdd-apply"}),
			RenderWizardComplete("opencode", 1, 1, true),
			NewInstallingModel().View(),
		}
		for i, v := range views {
			for _, f := range spinnerFrames {
				if strings.Contains(v, f) {
					t.Errorf("view %d contains spinner frame %q under NO_ANIMATION", i, f)
				}
			}
			if strings.Contains(v, "\x1b[?2026h") || strings.Contains(v, "\x1b[?2026l") {
				t.Errorf("view %d contains CSI 2026 under NO_ANIMATION", i)
			}
		}
	})

	t.Run("zero ANSI under TERM=dumb", func(t *testing.T) {
		t.Setenv("TERM", "dumb")
		views := []string{
			RenderWizardWelcome(),
			RenderWizardDetection(nil),
			RenderWizardAgents(0, nil),
			RenderWizardPersona(0, ""),
			RenderWizardPreset(0, ""),
			RenderWizardDepTree(0, "full", nil),
			RenderWizardSkills(0, nil),
			RenderWizardReview(nil, "", "", nil),
			RenderWizardComplete("", 0, 0, false),
			NewInstallingModel().View(),
		}
		for i, v := range views {
			if strings.Contains(v, "\x1b[") || strings.Contains(v, "\x1b(") {
				t.Errorf("view %d contains ANSI under TERM=dumb", i)
			}
		}
		if isInstallingPretty() {
			t.Errorf("isInstallingPretty must be false under TERM=dumb")
		}
	})
}
