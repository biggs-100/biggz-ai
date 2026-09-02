package screens

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
)

func strip(s string) string { return ansi.Strip(s) }

func TestPillCollapse(t *testing.T) {
	t.Setenv("BIGGZ_PRETTY", ""); t.Setenv("PI_SUBAGENT_CHILD", ""); t.Setenv("TERM", "xterm-256color"); t.Setenv("BIGGZ_NO_ANIMATION", ""); t.Setenv("GENTLE_AI_NO_ANIMATION", "")
	pills := []Pill{{Label: "a"}, {Label: "b"}, {Label: "c"}, {Label: "d"}, {Label: "e"}}
	vis, suffix := CollapsePills(pills, 3)
	if len(vis) != 3 || vis[0].Label != "a" || vis[2].Label != "c" || suffix != "… +2 hidden" {
		t.Fatalf("collapse got %v %q", vis, suffix)
	}
	rendered := RenderPills(pills)
	plain := strip(rendered)
	if !strings.Contains(plain, "a") || !strings.Contains(plain, "b") || !strings.Contains(plain, "c") {
		t.Fatalf("missing a/b/c %q", plain)
	}
	if !strings.Contains(plain, "… +2 hidden") {
		t.Fatalf("missing suffix %q", plain)
	}
	if strings.Contains(plain, " d ") || strings.Contains(plain, " e ") {
		t.Fatalf("hidden leaked %q", plain)
	}
	// order at any width
	for _, w := range []int{80, 40, 120} {
		plain = strip(RenderPills(pills))
		if !(strings.Index(plain, "a") < strings.Index(plain, "b") && strings.Index(plain, "b") < strings.Index(plain, "c")) {
			t.Fatalf("w %d order %q", w, plain)
		}
	}
}

func TestPillSpinnerStatic(t *testing.T) {
	t.Setenv("BIGGZ_PRETTY", ""); t.Setenv("PI_SUBAGENT_CHILD", ""); t.Setenv("TERM", "xterm-256color"); t.Setenv("GENTLE_AI_NO_ANIMATION", ""); t.Setenv("BIGGZ_NO_ANIMATION", "1")
	if GetSpinnerFrame() != "·" || IsSpinnerPrettyEnabled() {
		t.Fatalf("NO_ANIMATION should be static")
	}
	AdvanceSpinnerFrame()
	if GetSpinnerFrame() != "·" {
		t.Fatalf("after advance still static")
	}
	plain := strip(RenderPills([]Pill{{Label: "READ", State: "running"}}))
	if !strings.Contains(plain, "·") || strings.Contains(plain, "⠋") {
		t.Fatalf("running pill static failed %q", plain)
	}
	t.Setenv("BIGGZ_NO_ANIMATION", ""); t.Setenv("GENTLE_AI_NO_ANIMATION", "1")
	if GetSpinnerFrame() != "·" {
		t.Fatalf("gentle compat should freeze")
	}
	t.Setenv("GENTLE_AI_NO_ANIMATION", "")
	if GetSpinnerFrame() == "·" {
		t.Fatalf("should be animated after clear")
	}
}

func TestPillPrettyAndDumb(t *testing.T) {
	t.Setenv("BIGGZ_PRETTY", "0"); t.Setenv("PI_SUBAGENT_CHILD", ""); t.Setenv("TERM", "xterm-256color"); t.Setenv("BIGGZ_NO_ANIMATION", "")
	pills := []Pill{{Label: "READ", State: "running"}, {Label: "WRITE", State: "complete"}}
	rendered := RenderPills(pills)
	if strings.Contains(rendered, "\x1b[") {
		t.Fatalf("pretty off should be plain %q", rendered)
	}
	if !strings.Contains(rendered, "READ") || !strings.Contains(strip(RenderPills(pills)), "WRITE") {
		t.Fatalf("plain missing labels %q", rendered)
	}
	if strings.Contains(styles.PillStyle("running").Render("x"), "\x1b[") {
		t.Fatalf("PillStyle pretty off should be plain")
	}
	t.Setenv("BIGGZ_PRETTY", ""); t.Setenv("TERM", "dumb")
	rendered = RenderPills(pills)
	if strings.Contains(rendered, "\x1b[") || strings.Contains(rendered, "⠋") || strings.Contains(rendered, "✓") {
		t.Fatalf("dumb should strip %q", rendered)
	}
	// tokens exist
	for _, st := range []string{"running", "queued", "complete", "failed"} {
		if styles.PillStyle(st).Render("X") == "" {
			t.Fatalf("PillStyle %q empty", st)
		}
		if styles.PillIcon(st) == "" {
			t.Fatalf("PillIcon %q empty", st)
		}
	}
	_ = styles.PillRunning; _ = styles.PillQueued; _ = styles.PillComplete; _ = styles.PillFailed
}
