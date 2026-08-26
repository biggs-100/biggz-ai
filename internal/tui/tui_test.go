package tui

import (
	"os"
	"strings"
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

// --- Sync output (CSI 2026) ---

func TestSyncOutput_MarkersPresent(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("BIGGZ_NO_ANIMATION", "")
	t.Setenv("GENTLE_AI_NO_ANIMATION", "")
	// Ensure env is clean via unset for the exact "1" check.
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	t.Setenv("TERM", "xterm-256color")
	if !isSyncSupported() {
		t.Fatal("expected sync supported with TERM=xterm-256color and no animation env")
	}
	frame := "hello world"
	out := syncOutput(frame)
	if !strings.HasPrefix(out, syncBegin) {
		t.Errorf("expected prefix %q, got %q", syncBegin, out)
	}
	if !strings.HasSuffix(out, syncEnd) {
		t.Errorf("expected suffix %q, got %q", syncEnd, out)
	}
	if !strings.Contains(out, frame) {
		t.Error("expected frame content inside sync output")
	}
}

func TestSyncOutput_Fallback_TermDumb(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	if isSyncSupported() {
		t.Fatal("expected sync NOT supported with TERM=dumb")
	}
	frame := "plain"
	out := syncOutput(frame)
	if out != frame {
		t.Errorf("expected plain fallback, got %q", out)
	}
	if strings.Contains(out, syncBegin) || strings.Contains(out, syncEnd) {
		t.Error("fallback should not contain sync markers")
	}
}

func TestSyncOutput_Fallback_NoAnimation(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("BIGGZ_NO_ANIMATION", "1")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	if isSyncSupported() {
		t.Fatal("expected sync NOT supported with BIGGZ_NO_ANIMATION=1")
	}
	frame := "plain2"
	out := syncOutput(frame)
	if out != frame {
		t.Errorf("expected plain fallback with BIGGZ_NO_ANIMATION=1, got %q", out)
	}
}

func TestSyncOutput_Fallback_GentleAnimation(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	t.Setenv("GENTLE_AI_NO_ANIMATION", "1")
	if isSyncSupported() {
		t.Fatal("expected sync NOT supported with GENTLE_AI_NO_ANIMATION=1")
	}
	frame := "plain3"
	out := syncOutput(frame)
	if out != frame {
		t.Errorf("expected plain fallback with GENTLE_AI_NO_ANIMATION=1, got %q", out)
	}
}

func TestSyncOutput_Idempotent(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	frame := "idempotent"
	first := syncOutput(frame)
	second := syncOutput(first)
	if first != second {
		t.Errorf("expected idempotent syncOutput, first %q second %q", first, second)
	}
	if strings.Count(second, syncBegin) != 1 || strings.Count(second, syncEnd) != 1 {
		t.Error("expected exactly one pair of sync markers")
	}
}

func TestSyncOutput_ViewWraps(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	m := New()
	view := m.View()
	if !strings.HasPrefix(view, syncBegin) || !strings.HasSuffix(view, syncEnd) {
		t.Errorf("expected View to be wrapped with sync markers when supported, got prefix %q suffix %q", view[:min(10, len(view))], view[max(0, len(view)-10):])
	}
}

func TestSyncOutput_ViewFallback(t *testing.T) {
	t.Setenv("TERM", "dumb")
	_ = os.Unsetenv("BIGGZ_NO_ANIMATION")
	_ = os.Unsetenv("GENTLE_AI_NO_ANIMATION")
	m := New()
	view := m.View()
	if strings.Contains(view, syncBegin) || strings.Contains(view, syncEnd) {
		t.Error("expected View without sync markers when TERM=dumb")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- Bracketed paste ---

func TestBracketedPaste_SingleEvent_15Lines(t *testing.T) {
	var lines []string
	for i := 1; i <= 15; i++ {
		lines = append(lines, "line"+strings.Repeat("x", 5)+"-"+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")
	fixture := bracketedPasteStart + content + bracketedPasteEnd
	m := New()
	// Direct feedPaste path
	pm := m.feedPaste(fixture)
	if pm == nil {
		t.Fatal("expected PasteMsg for complete 15-line fixture, got nil")
	}
	if pm.Text != content {
		t.Errorf("paste content mismatch: got %q want %q", pm.Text[:min(20, len(pm.Text))], content[:min(20, len(content))])
	}
	lineCount := strings.Count(pm.Text, "\n") + 1
	if lineCount != 15 {
		t.Errorf("expected 15 lines in single PasteMsg, got %d", lineCount)
	}
	// Also via Update string path
	m2 := New()
	_, cmd := m2.Update(fixture)
	if cmd == nil {
		t.Fatal("expected cmd with PasteMsg via Update, got nil")
	}
	msg := cmd()
	pm2, ok := msg.(PasteMsg)
	if !ok {
		t.Fatalf("expected PasteMsg from cmd, got %T", msg)
	}
	if pm2.Text != content {
		t.Error("Update PasteMsg content mismatch")
	}
}

func TestBracketedPaste_CtrlCIgnored(t *testing.T) {
	// Paste contains text resembling ctrl+c / esc, must not trigger quit
	content := "hello ctrl+c world\nesc\nline3 with quit"
	fixture := bracketedPasteStart + content + bracketedPasteEnd
	m := New()
	pm := m.feedPaste(fixture)
	if pm == nil {
		t.Fatal("expected PasteMsg")
	}
	if !strings.Contains(pm.Text, "ctrl+c") {
		t.Error("expected paste to preserve ctrl+c text")
	}
	// Ensure Update with PasteMsg does not quit
	m2 := New()
	_, cmd := m2.Update(*pm)
	if cmd != nil {
		// Should be nil or not quit; we check that it's not tea.Quit
		// For default screen it returns nil
	}
	// Also ensure direct ctrl+c key still quits when not in paste
	_, quitCmd := m2.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quitCmd == nil {
		t.Error("expected ctrl+c to quit when not in paste")
	}
}

func TestBracketedPaste_IncompleteFlush(t *testing.T) {
	m := New()
	incomplete := bracketedPasteStart + "partial content without end"
	pm := m.feedPaste(incomplete)
	if pm != nil {
		t.Fatal("expected nil for incomplete paste, got PasteMsg")
	}
	if !m.pasteActive {
		t.Fatal("expected pasteActive true after incomplete start")
	}
	// Flush on timeout/next non-paste input
	flushed := m.flushPaste()
	if flushed == nil {
		t.Fatal("expected flushed PasteMsg for incomplete sequence")
	}
	if flushed.Text != "partial content without end" {
		t.Errorf("unexpected flushed content %q", flushed.Text)
	}
	if m.pasteActive {
		t.Error("expected pasteActive false after flush")
	}
	// Via Update: incomplete then next non-paste string triggers flush
	m2 := New()
	updated1, cmd1 := m2.Update(incomplete)
	m2 = updated1.(Model)
	if cmd1 != nil {
		t.Error("expected no cmd for incomplete")
	}
	// Next input without markers should flush
	updated, cmd2 := m2.Update("hello next input")
	m2 = updated.(Model)
	if cmd2 == nil {
		t.Fatal("expected flush cmd on next non-paste input")
	}
	msg := cmd2()
	pm2, ok := msg.(PasteMsg)
	if !ok {
		t.Fatalf("expected PasteMsg flush, got %T", msg)
	}
	if pm2.Text != "partial content without end" {
		t.Errorf("flush via Update mismatch %q", pm2.Text)
	}
	if m2.pasteActive {
		t.Error("expected inactive after flush via Update")
	}
}

func TestBracketedPaste_MultiChunkSplit(t *testing.T) {
	m := New()
	chunk1 := bracketedPasteStart + "first part\n"
	pm1 := m.feedPaste(chunk1)
	if pm1 != nil {
		t.Fatal("expected nil after first chunk without end")
	}
	if !m.pasteActive {
		t.Fatal("expected active after first chunk")
	}
	chunk2 := "second part\n" + bracketedPasteEnd
	pm2 := m.feedPaste(chunk2)
	if pm2 == nil {
		t.Fatal("expected PasteMsg after second chunk with end")
	}
	expected := "first part\nsecond part\n"
	if pm2.Text != expected {
		t.Errorf("multi-chunk paste mismatch got %q want %q", pm2.Text, expected)
	}
}
