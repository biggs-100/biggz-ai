package screens

import (
	"strings"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

// WizardDepTreeOptions is the static install plan shown on the DepTree stage.
// It mirrors the pipeline plan wired in install.go (deploy-skills, overlay,
// pi-extensions) in biggz wording.
var WizardDepTreeOptions = []WizardOption{
	{Name: "deploy-skills", Description: "Deploy SDD skills to ~/.biggz/skills and the agent skills dir"},
	{Name: "deploy-overlay", Description: "Merge config, slash commands, plugins, and prompts"},
	{Name: "deploy-pi-extensions", Description: "Deploy Pi extensions (footer, agents, skills bridge)"},
}

// wizardDepTreeSelected reports whether name is in the selection.
func wizardDepTreeSelected(selected []string, name string) bool {
	for _, s := range selected {
		if s == name {
			return true
		}
	}
	return false
}

// toggleWizardDepTree adds name when absent, removes it when present.
func toggleWizardDepTree(selected []string, name string) []string {
	for i, s := range selected {
		if s == name {
			return append(selected[:i], selected[i+1:]...)
		}
	}
	return append(selected, name)
}

// defaultWizardDepTree returns every plan step — the non-custom install set.
func defaultWizardDepTree() []string {
	out := make([]string, 0, len(WizardDepTreeOptions))
	for _, o := range WizardDepTreeOptions {
		out = append(out, o.Name)
	}
	return out
}

// RenderWizardDepTree renders the dependency-tree plan. For non-custom
// presets the list is read-only; for the custom preset it is a checkbox
// picker. cursor is the highlighted row; selected holds the chosen steps.
func RenderWizardDepTree(cursor int, preset string, selected []string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Install biggz-ai"))
	b.WriteString("\n\n")
	b.WriteString(styles.Section.Render("Dependency tree"))
	b.WriteString("\n\n")
	if preset == "custom" {
		for i, opt := range WizardDepTreeOptions {
			cur := "  "
			if i == cursor {
				cur = styles.Cursor
			}
			box := "[ ]"
			if wizardDepTreeSelected(selected, opt.Name) {
				box = "[x]"
			}
			b.WriteString(cur + box + " " + opt.Name + " — " + opt.Description + "\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("↑↓ navigate · SPACE toggle · ENTER continue · esc back"))
	} else {
		for _, opt := range WizardDepTreeOptions {
			b.WriteString("  ✓ " + opt.Name + " — " + opt.Description + "\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.Help.Render("ENTER to continue · esc back"))
	}
	return wizardGuardView(styles.AppStyle.Render(b.String()))
}
