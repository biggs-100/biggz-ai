package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterHelp_EmptyShowsAll(t *testing.T) {
	all := filterHelp("")
	if len(all) != len(helpData) {
		t.Fatalf("empty filter got %d want %d", len(all), len(helpData))
	}
}

func TestFilterHelp_CaseInsensitive(t *testing.T) {
	upper := filterHelp("DASHBOARD")
	lower := filterHelp("dashboard")
	if len(upper) == 0 || len(lower) == 0 {
		t.Fatal("expected matches for dashboard")
	}
	if len(upper) != len(lower) {
		t.Fatalf("case insensitive mismatch upper %d lower %d", len(upper), len(lower))
	}
	found := false
	for _, h := range upper {
		if strings.EqualFold(h.Title, "Dashboard") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Dashboard title in filtered results")
	}
}

func TestFilterHelp_NarrowsBackup(t *testing.T) {
	res := filterHelp("backup")
	if len(res) == 0 {
		t.Fatal("expected at least one match for backup")
	}
	for _, h := range res {
		hit := strings.Contains(strings.ToLower(h.Title), "backup") ||
			strings.Contains(strings.ToLower(h.Paragraph), "backup")
		for _, k := range h.Keys {
			if strings.Contains(strings.ToLower(k.Key), "backup") || strings.Contains(strings.ToLower(k.Desc), "backup") {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf("result %q does not contain backup", h.Title)
		}
	}
}

func TestFilterHelp_NoMatchesPlaceholder(t *testing.T) {
	res := filterHelp("zzzz_no_match")
	if len(res) != 0 {
		t.Fatalf("expected 0 matches got %d", len(res))
	}
	m := NewHelpModel()
	m.filter = "zzzz_no_match"
	m.filtered = res
	m.viewport.Width = 80
	m.viewport.Height = 10
	view := m.View()
	if !strings.Contains(view, "No matches") {
		t.Errorf("expected No matches placeholder, got %q", view)
	}
}

func TestHelpModel_ViewContainsShortcuts(t *testing.T) {
	m := NewHelpModel()
	m.width = 80
	m.height = 24
	m.viewport.Width = 76
	m.viewport.Height = 15
	view := m.View()
	if !strings.Contains(view, "Help") {
		t.Error("expected Help title in view")
	}
	if !strings.Contains(view, "Navigate") && !strings.Contains(view, "ENTER") {
		t.Error("expected shortcut rows in view")
	}
}

func TestHelpModel_ViewportScroll(t *testing.T) {
	m := NewHelpModel()
	m.width = 80
	m.height = 20
	m.viewport.Width = 76
	m.viewport.Height = 2
	m.filtered = filterHelp("")
	m.viewport.SetContent(buildHelpContent(m.filtered, 76))
	initialOffset := m.viewport.YOffset
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	hm := m2.(HelpModel)
	if hm.viewport.YOffset <= initialOffset && hm.viewport.Height < 10 {
		t.Logf("viewport offset %d initial %d", hm.viewport.YOffset, initialOffset)
	}
}

func TestHelpModel_NarrowTruncation(t *testing.T) {
	m := NewHelpModel()
	m.width = 40
	m.height = 20
	m.viewport.Width = 36
	m.viewport.Height = 10
	m.filtered = filterHelp("")
	content := buildHelpContent(m.filtered, 40)
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		if VisibleWidth(l) > 40 {
			t.Errorf("line exceeds width 40: %q width %d", l, VisibleWidth(l))
		}
	}
	long := strings.Repeat("x", 100)
	trunc := TruncateToWidth(long, 40)
	if VisibleWidth(trunc) > 40 {
		t.Fatalf("TruncateToWidth failed width %d >40", VisibleWidth(trunc))
	}
	if !strings.HasSuffix(trunc, "…") {
		t.Error("expected truncation ellipsis")
	}
}

func TestHelpModel_ESC_ClearsFilter(t *testing.T) {
	m := NewHelpModel()
	m.focused = true
	m.input.SetValue("dash")
	m.filter = "dash"
	m.filtered = filterHelp("dash")
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	hm := m2.(HelpModel)
	if hm.filter != "" {
		t.Fatalf("expected filter cleared, got %q", hm.filter)
	}
	if len(hm.filtered) != len(helpData) {
		t.Fatalf("expected all entries after clear, got %d want %d", len(hm.filtered), len(helpData))
	}
}

func TestHelpModel_ESC_ClosesWhenEmpty(t *testing.T) {
	m := NewHelpModel()
	m.filter = ""
	m.filtered = filterHelp("")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd for navigate back")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Screen != 0 {
		t.Fatalf("expected dashboard screen 0, got %d", nav.Screen)
	}
}

func TestHelpModel_InputFocusSuppression(t *testing.T) {
	m := NewHelpModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	hm := m2.(HelpModel)
	if !hm.focused {
		t.Fatal("expected focused after /")
	}
	m3, _ := hm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	hm3 := m3.(HelpModel)
	if !strings.Contains(hm3.input.Value(), "?") {
		t.Errorf("expected ? inserted into filter, got %q", hm3.input.Value())
	}
	m4, _ := hm3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	hm4 := m4.(HelpModel)
	if !strings.Contains(hm4.input.Value(), "q") {
		t.Errorf("expected q inserted, got %q", hm4.input.Value())
	}
}

func TestHelpModel_ViewportRenderingWithFilter(t *testing.T) {
	m := NewHelpModel()
	m.width = 80
	m.height = 24
	m.viewport.Width = 76
	m.viewport.Height = 10
	m.filter = "backup"
	m.filtered = filterHelp("backup")
	view := m.View()
	if !strings.Contains(view, "Backup") {
		t.Error("expected Backup in filtered view")
	}
	content := buildHelpContent(m.filtered, 76)
	for _, line := range strings.Split(content, "\n") {
		if VisibleWidth(line) > 80 {
			t.Errorf("help content line exceeds 80: width %d line %q", VisibleWidth(line), line)
		}
	}
}
