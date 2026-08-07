package screens

import (
	"errors"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/agentbuilder"
	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/model"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	errTestGenerate = errors.New("test generation failed")
	errTestInstall  = errors.New("test install failed")
)

// keyMsg returns a KeyMsg for the given key string.
func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func updateScreen(m AgentBuilderScreen, msg tea.Msg) AgentBuilderScreen {
	updated, _ := m.Update(msg)
	return updated.(AgentBuilderScreen)
}

func TestAgentBuilder_InitialView_IsEngineSelection(t *testing.T) {
	m := NewAgentBuilderScreen()
	if m.view != abEngine {
		t.Errorf("initial view = %v, want abEngine", m.view)
	}
}

func TestAgentBuilder_NoEngines_EnterGoesBackToDashboard(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.engines = []model.AgentID(nil)

	updated, cmd := m.Update(keyMsg("enter"))
	m2 := updated.(AgentBuilderScreen)
	if m2.view != abEngine {
		t.Fatalf("view = %v, want abEngine (no engines to select)", m2.view)
	}
	if cmd == nil {
		t.Fatal("expected a navigation command for the Back option")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Screen != 0 {
		t.Errorf("navigate screen = %d, want 0 (dashboard)", nav.Screen)
	}
}

func TestAgentBuilder_EngineSelect_ProceedsToPrompt(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.engines = []model.AgentID{agents.AgentOpenCode}
	m.textarea.SetValue("")

	m = updateScreen(m, keyMsg("enter"))
	if m.view != abPrompt {
		t.Errorf("view = %v, want abPrompt after engine select", m.view)
	}
	if m.selectedEngine != agents.AgentOpenCode {
		t.Errorf("selectedEngine = %q, want opencode", m.selectedEngine)
	}
}

func TestAgentBuilder_Prompt_TabContinuesOnlyWithText(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.view = abPrompt

	// Empty textarea: tab does nothing.
	m = updateScreen(m, keyMsg("tab"))
	if m.view != abPrompt {
		t.Errorf("view = %v, want abPrompt (empty prompt blocks continue)", m.view)
	}

	// With text: tab proceeds to SDD selection.
	m.textarea.SetValue("build a css linter")
	m = updateScreen(m, keyMsg("tab"))
	if m.view != abSDD {
		t.Errorf("view = %v, want abSDD after tab with text", m.view)
	}
}

func TestAgentBuilder_SDD_PhaseSupport_GoesToPhaseSelection(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.view = abSDD

	m.cursor = 1 // Phase Support
	m = updateScreen(m, keyMsg("enter"))
	if m.view != abSDDPhase {
		t.Errorf("view = %v, want abSDDPhase", m.view)
	}
	if m.sddMode != agentbuilder.SDDPhaseSupport {
		t.Errorf("sddMode = %q, want phase-support", m.sddMode)
	}
}

func TestAgentBuilder_SDD_Standalone_StartsGeneration(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.view = abSDD

	m.cursor = 0 // Standalone
	updated, cmd := m.Update(keyMsg("enter"))
	m2 := updated.(AgentBuilderScreen)
	if m2.view != abGenerating {
		t.Errorf("view = %v, want abGenerating", m2.view)
	}
	if !m2.generating {
		t.Error("generating = false, want true")
	}
	if cmd == nil {
		t.Fatal("expected a command batch starting generation")
	}
}

func TestAgentBuilder_GeneratedMsg_TransitionsToPreview(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.generating = true
	agent := &agentbuilder.GeneratedAgent{
		Name:        "css-linter",
		Title:       "CSS Linter",
		Description: "Lints CSS",
		Trigger:     "When the user asks to lint CSS",
		Content:     "# CSS Linter\n\n## Description\nLints CSS.\n",
	}

	m = updateScreen(m, abGeneratedMsg{agent: agent})
	if m.view != abPreview {
		t.Errorf("view = %v, want abPreview", m.view)
	}
	if m.generating {
		t.Error("generating = true, want false after result")
	}
	if m.generated != agent {
		t.Errorf("generated = %v, want the produced agent", m.generated)
	}
}

func TestAgentBuilder_GeneratedErr_StaysOnGeneratingWithError(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.generating = true
	m.view = abGenerating

	m = updateScreen(m, abGeneratedMsg{err: errTestGenerate})
	if m.view != abGenerating {
		t.Errorf("view = %v, want abGenerating (error shown in place)", m.view)
	}
	if m.genErr != errTestGenerate {
		t.Errorf("genErr = %v, want the generation error", m.genErr)
	}
	if m.generating {
		t.Error("generating = true, want false after error result")
	}
}

func TestAgentBuilder_InstallDoneMsg_SuccessTransitionsToComplete(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.generating = false
	m.installing = true
	m.view = abInstalling

	m = updateScreen(m, abInstallDoneMsg{
		results: []agentbuilder.InstallResult{
			{AgentID: agents.AgentOpenCode, Path: "/skills/css-linter/SKILL.md", Success: true},
		},
	})
	if m.view != abComplete {
		t.Errorf("view = %v, want abComplete", m.view)
	}
	if m.installing {
		t.Error("installing = true, want false after result")
	}
	if len(m.installResults) != 1 {
		t.Errorf("installResults len = %d, want 1", len(m.installResults))
	}
}

func TestAgentBuilder_InstallDoneErr_ReturnsToPreview(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.generating = false
	m.installing = true
	m.view = abInstalling
	m.generated = &agentbuilder.GeneratedAgent{Name: "css-linter", Title: "CSS Linter"}

	m = updateScreen(m, abInstallDoneMsg{err: errTestInstall})
	if m.view != abPreview {
		t.Errorf("view = %v, want abPreview after install failure", m.view)
	}
	if m.installErr != errTestInstall {
		t.Errorf("installErr = %v, want the install error", m.installErr)
	}
}

func TestAgentBuilder_EscDuringGeneration_CancelsBackToPrompt(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.generating = true
	m.view = abGenerating

	m = updateScreen(m, keyMsg("esc"))
	if m.view != abPrompt {
		t.Errorf("view = %v, want abPrompt after cancel", m.view)
	}
	if m.generating {
		t.Error("generating = true, want false after cancel")
	}
}

func TestAgentBuilder_ConflictWarning_OnGeneratedResult(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.generating = true
	// "auto-fix" is a built-in catalog skill in biggz.
	agent := &agentbuilder.GeneratedAgent{
		Name:    "auto-fix",
		Title:   "Auto Fix",
		Content: "# Auto Fix\n",
	}

	m = updateScreen(m, abGeneratedMsg{agent: agent})
	if m.conflictWarning == "" {
		t.Error("conflictWarning = empty, want warning for builtin collision")
	}
}

func TestAgentBuilder_Complete_EnterReturnsToDashboard(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.view = abComplete

	updated, cmd := m.Update(keyMsg("enter"))
	m2 := updated.(AgentBuilderScreen)
	if m2.view != abComplete {
		t.Errorf("view = %v, want abComplete", m2.view)
	}
	if cmd == nil {
		t.Fatal("expected navigation command from completion screen")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Screen != 0 {
		t.Errorf("navigate screen = %d, want 0 (dashboard)", nav.Screen)
	}
}

func TestAgentBuilder_PreviewScroll_Clamped(t *testing.T) {
	m := NewAgentBuilderScreen()
	m.view = abPreview
	m.previewScroll = 0

	m = updateScreen(m, keyMsg("up"))
	if m.previewScroll != 0 {
		t.Errorf("previewScroll = %d, want 0 (clamped at top)", m.previewScroll)
	}

	m = updateScreen(m, keyMsg("down"))
	if m.previewScroll != 1 {
		t.Errorf("previewScroll = %d, want 1", m.previewScroll)
	}
}

func TestAgentBuilder_ABSDDPhases_Ordered(t *testing.T) {
	got := abSDDPhases()
	want := []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"}
	if len(got) != len(want) {
		t.Fatalf("phases len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phases[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
