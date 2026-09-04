package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// WizardSkillOptions is the multi-select skill list for the SkillPicker stage,
// mirroring the skill directories shipped under internal/assets/skills.
var WizardSkillOptions = []string{
	"sdd-apply",
	"sdd-archive",
	"sdd-design",
	"sdd-explore",
	"sdd-ff",
	"sdd-init",
	"sdd-new",
	"sdd-onboard",
	"sdd-propose",
	"sdd-research",
	"sdd-spec",
	"sdd-sync",
	"sdd-tasks",
	"sdd-verify",
	"branch-pr",
	"chained-pr",
	"work-unit-commits",
	"cognitive-doc-design",
	"comment-writer",
	"go-testing",
	"use-modern-go",
	"skill-creator",
	"skill-improver",
	"skill-registry",
	"systemic-issue-triage",
	"issue-creation",
	"issue-root-resolution",
	"judgment-day",
	"rdd-defect-workflow",
	"hermes-ephemeral-delegation",
}

// wizardSkillSelected reports whether name is in the selection.
func wizardSkillSelected(selected []string, name string) bool {
	for _, s := range selected {
		if s == name {
			return true
		}
	}
	return false
}

// toggleWizardSkill adds name when absent, removes it when present.
func toggleWizardSkill(selected []string, name string) []string {
	for i, s := range selected {
		if s == name {
			return append(selected[:i], selected[i+1:]...)
		}
	}
	return append(selected, name)
}

// RenderWizardSkills renders the skill multi-select checklist. cursor is the
// highlighted row; selected holds the chosen skill names.
func RenderWizardSkills(cursor int, selected []string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Select skills to deploy"))
	b.WriteString("\n\n")
	for i, name := range WizardSkillOptions {
		cur := "  "
		if i == cursor {
			cur = styles.Cursor
		}
		box := "[ ]"
		if wizardSkillSelected(selected, name) {
			box = "[x]"
		}
		b.WriteString(cur + box + " " + name + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.Help.Render("↑↓ navigate · SPACE toggle · ENTER continue · esc back"))
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
