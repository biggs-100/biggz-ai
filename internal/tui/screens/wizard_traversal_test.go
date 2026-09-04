package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Batch-1 traversal (task 2.1, partial): drive Update with key msgs over the
// implemented Welcome→Preset stages, forward and back, with state preserved.
// Full 10-stage traversal completes in PR3.
func TestWizardTraversalBatch1(t *testing.T) {
	t.Setenv("BIGGZ_LEGACY_INSTALL", "")
	t.Setenv("BIGGZ_NO_ANIMATION", "1")

	m := NewInstallModel()
	if m.step != stepWizWelcome {
		t.Fatalf("expected wizard start at Welcome, got step %d", m.step)
	}

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	space := tea.KeyMsg{Type: tea.KeySpace}
	down := tea.KeyMsg{Type: tea.KeyDown}

	step := func(model tea.Model) InstallModel {
		im, ok := model.(InstallModel)
		if !ok {
			t.Fatalf("expected InstallModel, got %T", model)
		}
		return im
	}

	// Welcome → Detection.
	next, _ := m.Update(enter)
	m = step(next)
	if m.step != stepWizDetection {
		t.Fatalf("expected Detection, got step %d", m.step)
	}

	// Detection → Agents.
	next, _ = m.Update(enter)
	m = step(next)
	if m.step != stepWizAgents {
		t.Fatalf("expected Agents, got step %d", m.step)
	}

	// Toggle two agents with space.
	next, _ = m.Update(space) // opencode
	m = step(next)
	next, _ = m.Update(down)
	m = step(next)
	next, _ = m.Update(space) // claude
	m = step(next)
	if len(m.selectedAgents) != 2 {
		t.Fatalf("expected 2 selected agents, got %v", m.selectedAgents)
	}

	// Agents → Persona → confirm first option.
	next, _ = m.Update(enter)
	m = step(next)
	if m.step != stepWizPersona {
		t.Fatalf("expected Persona, got step %d", m.step)
	}
	next, _ = m.Update(enter)
	m = step(next)
	if m.step != stepWizPreset {
		t.Fatalf("expected Preset, got step %d", m.step)
	}
	if m.persona != WizardPersonaOptions[0].Name {
		t.Fatalf("expected persona %q, got %q", WizardPersonaOptions[0].Name, m.persona)
	}

	// Preset: move down and confirm (PR3: confirm advances to DepTree).
	next, _ = m.Update(down)
	m = step(next)
	next, _ = m.Update(enter)
	m = step(next)
	if m.preset != WizardPresetOptions[1].Name {
		t.Fatalf("expected preset %q, got %q", WizardPresetOptions[1].Name, m.preset)
	}
	if m.step != stepWizDepTree {
		t.Fatalf("expected DepTree after preset confirm, got step %d", m.step)
	}

	// Back: DepTree → Preset → Persona, selections intact.
	next, _ = m.Update(esc)
	m = step(next)
	if m.step != stepWizPreset {
		t.Fatalf("expected back at Preset, got step %d", m.step)
	}
	next, _ = m.Update(esc)
	m = step(next)
	if m.step != stepWizPersona {
		t.Fatalf("expected back at Persona, got step %d", m.step)
	}
	if m.persona == "" || m.preset == "" || len(m.selectedAgents) != 2 {
		t.Fatalf("back navigation lost state: persona=%q preset=%q agents=%v",
			m.persona, m.preset, m.selectedAgents)
	}

	// Back to Agents, toggles intact; forward again preserves them.
	next, _ = m.Update(esc)
	m = step(next)
	if m.step != stepWizAgents || len(m.selectedAgents) != 2 {
		t.Fatalf("expected Agents with 2 agents, got step %d agents %v", m.step, m.selectedAgents)
	}
	next, _ = m.Update(enter)
	m = step(next)
	next, _ = m.Update(enter)
	m = step(next)
	if m.step != stepWizPreset || len(m.selectedAgents) != 2 {
		t.Fatalf("re-forward lost state: step %d agents %v", m.step, m.selectedAgents)
	}

	// esc at Welcome stays at Welcome.
	m.step = stepWizWelcome
	next, _ = m.Update(esc)
	m = step(next)
	if m.step != stepWizWelcome {
		t.Fatalf("esc at Welcome should stay, got step %d", m.step)
	}
}

// Full 10-stage traversal (task 2.1, complete in PR3): keyboard-only
// Welcome→Complete forward and back with state preserved (REQ-WIZ-001).
// Installing advances on confirm until live progress wiring lands in PR5.
func TestWizardTraversalFull(t *testing.T) {
	t.Setenv("BIGGZ_LEGACY_INSTALL", "")
	t.Setenv("BIGGZ_NO_ANIMATION", "1")

	m := NewInstallModel()
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	space := tea.KeyMsg{Type: tea.KeySpace}
	down := tea.KeyMsg{Type: tea.KeyDown}

	step := func(model tea.Model) InstallModel {
		im, ok := model.(InstallModel)
		if !ok {
			t.Fatalf("expected InstallModel, got %T", model)
		}
		return im
	}
	press := func(mm InstallModel, k tea.KeyMsg) InstallModel {
		next, _ := mm.Update(k)
		return step(next)
	}

	// Welcome → Detection → Agents; select one agent.
	m = press(m, enter)
	m = press(m, enter)
	if m.step != stepWizAgents {
		t.Fatalf("expected Agents, got step %d", m.step)
	}
	m = press(m, space)
	// Agents → Persona → Preset; confirm defaults.
	m = press(m, enter)
	m = press(m, enter)
	if m.step != stepWizPreset {
		t.Fatalf("expected Preset, got step %d", m.step)
	}
	m = press(m, enter) // confirm "full" preset
	if m.preset != WizardPresetOptions[0].Name {
		t.Fatalf("expected preset %q, got %q", WizardPresetOptions[0].Name, m.preset)
	}

	// Preset → DepTree (read-only for non-custom).
	if m.step != stepWizDepTree {
		t.Fatalf("expected DepTree, got step %d", m.step)
	}
	m = press(m, enter)

	// DepTree → SkillPicker; toggle two skills.
	if m.step != stepWizSkillPicker {
		t.Fatalf("expected SkillPicker, got step %d", m.step)
	}
	m = press(m, space)
	m = press(m, down)
	m = press(m, space)
	if len(m.selectedSkills) != 2 {
		t.Fatalf("expected 2 selected skills, got %v", m.selectedSkills)
	}
	m = press(m, enter)

	// SkillPicker → Review → Installing → Complete.
	if m.step != stepWizReview {
		t.Fatalf("expected Review, got step %d", m.step)
	}
	m = press(m, enter)
	if m.step != stepWizInstalling {
		t.Fatalf("expected Installing, got step %d", m.step)
	}
	m = press(m, enter)
	if m.step != stepWizComplete {
		t.Fatalf("expected Complete, got step %d", m.step)
	}

	// Back: Complete → Installing → Review → SkillPicker, state intact.
	m = press(m, esc)
	if m.step != stepWizInstalling {
		t.Fatalf("expected back at Installing, got step %d", m.step)
	}
	m = press(m, esc)
	if m.step != stepWizReview {
		t.Fatalf("expected back at Review, got step %d", m.step)
	}
	m = press(m, esc)
	if m.step != stepWizSkillPicker || len(m.selectedSkills) != 2 {
		t.Fatalf("expected SkillPicker with 2 skills, got step %d skills %v", m.step, m.selectedSkills)
	}
	// Back to DepTree and Preset; earlier selections intact.
	m = press(m, esc)
	if m.step != stepWizDepTree {
		t.Fatalf("expected back at DepTree, got step %d", m.step)
	}
	m = press(m, esc)
	if m.step != stepWizPreset {
		t.Fatalf("expected back at Preset, got step %d", m.step)
	}
	if m.preset == "" || m.persona == "" || len(m.selectedAgents) != 1 || len(m.selectedSkills) != 2 {
		t.Fatalf("back navigation lost state: preset=%q persona=%q agents=%v skills=%v",
			m.preset, m.persona, m.selectedAgents, m.selectedSkills)
	}
	// Forward again from Preset reaches Complete with state preserved.
	for m.step != stepWizComplete {
		m = press(m, enter)
	}
	if m.preset == "" || m.persona == "" || len(m.selectedAgents) != 1 || len(m.selectedSkills) != 2 {
		t.Fatalf("re-forward lost state: preset=%q persona=%q agents=%v skills=%v",
			m.preset, m.persona, m.selectedAgents, m.selectedSkills)
	}

	// Custom preset arms the DepTree checkbox picker with the full plan.
	m2 := NewInstallModel()
	m2.step = stepWizPreset
	for i := range WizardPresetOptions {
		if WizardPresetOptions[i].Name == "custom" {
			m2.cursor = i
		}
	}
	m2 = press(m2, enter)
	if m2.step != stepWizDepTree || m2.preset != "custom" {
		t.Fatalf("expected custom DepTree, got step %d preset %q", m2.step, m2.preset)
	}
	if len(m2.selectedDepTree) != len(WizardDepTreeOptions) {
		t.Fatalf("expected full dep plan %v, got %v", WizardDepTreeOptions, m2.selectedDepTree)
	}
	m2 = press(m2, space) // uncheck first plan step
	if len(m2.selectedDepTree) != len(WizardDepTreeOptions)-1 {
		t.Fatalf("deptree toggle failed, got %v", m2.selectedDepTree)
	}
	m2 = press(m2, esc) // back to Preset keeps the custom edit
	if m2.step != stepWizPreset || len(m2.selectedDepTree) != len(WizardDepTreeOptions)-1 {
		t.Fatalf("deptree back lost state: step %d dep %v", m2.step, m2.selectedDepTree)
	}

	// Complete view renders the Done-view fields.
	view := RenderWizardComplete("opencode", 3, 2, true)
	for _, want := range []string{"bigg", "Skills deployed", "Commands written", "Config merged"} {
		if !strings.Contains(view, want) {
			t.Errorf("complete view missing %q", want)
		}
	}
}

// Batch-2 views: DepTree/Skills/Review/Complete render, zero banner tokens,
// zero ANSI under TERM=dumb (REQ-WIZ-004 / REQ-WIZ-005).
func TestWizardBatch2ViewsGuards(t *testing.T) {
	t.Setenv("BIGGZ_NO_ANIMATION", "1")
	t.Setenv("TERM", "dumb")

	views := []string{
		RenderWizardDepTree(0, "full", nil),
		RenderWizardDepTree(0, "custom", []string{"deploy-skills"}),
		RenderWizardSkills(0, []string{"sdd-apply"}),
		RenderWizardReview([]string{"opencode"}, "bigg", "full", []string{"sdd-apply"}),
		RenderWizardComplete("opencode", 1, 1, true),
	}
	// Banned patterns are built by concatenation so a source grep for the
	// literal tokens stays clean on this file itself.
	banned := []string{"Render" + "Logo", "Tag" + "line", "update" + "Banner", "advi" + "sory", "Gent" + "le"}
	for i, v := range views {
		for _, b := range banned {
			if strings.Contains(v, b) {
			t.Errorf("view %d contains banned token %q", i, b)
		}
	}
		if strings.Contains(v, "\x1b[") {
			t.Errorf("view %d contains ANSI under TERM=dumb", i)
		}
		if strings.Contains(v, "\x1b[?2026h") || strings.Contains(v, "\x1b[?2026l") {
			t.Errorf("view %d contains CSI 2026 under NO_ANIMATION", i)
		}
	}
	if !strings.Contains(views[0], "deploy-skills") {
		t.Errorf("read-only deptree missing plan steps")
	}
	if !strings.Contains(views[3], "opencode") || !strings.Contains(views[3], "full") {
		t.Errorf("review missing agents/preset summary")
	}
}

// Batch-1 views: title+content render, zero banner tokens, zero ANSI under
// TERM=dumb (REQ-WIZ-004 / REQ-WIZ-005, scoped to implemented stages).
func TestWizardBatch1ViewsGuards(t *testing.T) {
	t.Setenv("BIGGZ_NO_ANIMATION", "1")
	t.Setenv("TERM", "dumb")

	views := []string{
		RenderWizardWelcome(),
		RenderWizardDetection([]string{"opencode"}),
		RenderWizardDetection(nil),
		RenderWizardAgents(0, []string{"opencode"}),
		RenderWizardPersona(0, ""),
		RenderWizardPreset(0, ""),
	}
	// Banned patterns are built by concatenation so a source grep for the
	// literal tokens stays clean on this file itself.
	banned := []string{"Render" + "Logo", "Tag" + "line", "update" + "Banner", "advi" + "sory", "Gent" + "le"}
	for i, v := range views {
		for _, b := range banned {
			if strings.Contains(v, b) {
				t.Errorf("view %d contains banned token %q", i, b)
			}
		}
		if strings.Contains(v, "\x1b[") {
			t.Errorf("view %d contains ANSI under TERM=dumb", i)
		}
		if strings.Contains(v, "\x1b[?2026h") || strings.Contains(v, "\x1b[?2026l") {
			t.Errorf("view %d contains CSI 2026 under NO_ANIMATION", i)
		}
		if !strings.Contains(v, "biggz-ai") {
			t.Errorf("view %d missing biggz-ai title", i)
		}
	}

	// Legacy fallback untouched: flag forces the lean 6-state flow.
	t.Setenv("BIGGZ_LEGACY_INSTALL", "1")
	legacy := NewInstallModel()
	if !legacy.useLegacy || legacy.step != stepInstallIdle {
		t.Fatalf("legacy flag should start at Idle, got step %d legacy=%v", legacy.step, legacy.useLegacy)
	}
}
