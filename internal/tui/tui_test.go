package tui

import (
	"testing"

	"github.com/biggs-100/biggz-ai/internal/tui/screens"
	tea "github.com/charmbracelet/bubbletea"
)

// TestNewModel verifies the TUI initialises correctly.
func TestNewModel(t *testing.T) {
	m := New()
	if m.currentScreen != screenDashboard {
		t.Errorf("expected screen %d (dashboard), got %d", screenDashboard, m.currentScreen)
	}
	if m.showHelp {
		t.Error("expected help to be hidden initially")
	}
}

// TestNavigate verifies screen navigation.
func TestNavigate(t *testing.T) {
	m := New()

	// Navigate to install
	updated, _ := m.Update(screens.NavigateMsg{Screen: screenInstall})
	m2 := updated.(Model)
	if m2.currentScreen != screenInstall {
		t.Errorf("expected screen %d (install), got %d", screenInstall, m2.currentScreen)
	}
	if m2.showHelp {
		t.Error("expected help to be hidden after navigation")
	}
}

// TestHelpToggle verifies help overlay toggling.
func TestHelpToggle(t *testing.T) {
	m := New()

	// Press ? to show help
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(Model)
	if !m2.showHelp {
		t.Error("expected help to be shown after pressing ?")
	}

	// Press ? again to hide
	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m3 := updated.(Model)
	if m3.showHelp {
		t.Error("expected help to be hidden after pressing ? again")
	}
}

// TestQuit verifies quit from dashboard.
func TestQuit(t *testing.T) {
	m := New()

	// q from dashboard should quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command after q key")
	}
}

// TestHelpContent verifies all screens have help content.
func TestHelpContent(t *testing.T) {
	for screenID := 0; screenID < screenCount; screenID++ {
		h := screens.GetHelp(screenID)
		if h.Title == "" {
			t.Errorf("screen %d has no help title", screenID)
		}
		if len(h.Keys) == 0 {
			t.Errorf("screen %d has no help keys", screenID)
		}
	}
}

// TestHelpOverlay verifies the help overlay renders without error.
func TestHelpOverlay(t *testing.T) {
	for screenID := 0; screenID < screenCount; screenID++ {
		result := screens.HelpOverlay(screenID)
		if result == "" {
			t.Errorf("help overlay for screen %d returned empty", screenID)
		}
	}
}
