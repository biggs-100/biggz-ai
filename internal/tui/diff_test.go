package tui

import (
	"strings"
	"testing"
)

func TestRenderDiff_SplitAt120(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("BIGGZ_PRETTY", "")
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("BIGGZ_NO_ANIMATION", "")
	t.Setenv("GENTLE_AI_NO_ANIMATION", "")
	o, n := "hello world\nfoo bar", "hello brave world\nfoo baz"
	out := RenderDiff(o, n, 120)
	if !strings.Contains(out, "│") && !strings.Contains(out, "|") {
		t.Fatalf("split sep missing %q", out)
	}
	if !strings.Contains(out, "brave") {
		t.Error("brave missing")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("ANSI missing at 120 %q", out)
	}
}
func TestRenderDiff_UnifiedAt80(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("BIGGZ_PRETTY", "")
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("BIGGZ_NO_ANIMATION", "")
	t.Setenv("GENTLE_AI_NO_ANIMATION", "")
	o, n := "hello world\nfoo bar", "hello brave world\nfoo baz"
	out := RenderDiff(o, n, 80)
	if strings.Contains(out, " │ ") && strings.Count(out, " │ ") > 1 {
		t.Errorf("unified should not have sep %q", out)
	}
	if !strings.Contains(out, "brave") {
		t.Error("brave missing")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("ANSI missing at 80 %q", out)
	}
}
func TestRenderDiff_CapFallback(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("BIGGZ_PRETTY", "")
	t.Setenv("PI_SUBAGENT_CHILD", "")
	o, n := strings.Repeat("a", 700000), strings.Repeat("b", 700000)
	out := RenderDiff(o, n, 120)
	if !strings.Contains(strings.ToLower(out), "truncated") && !strings.Contains(out, "1MB") {
		t.Fatalf("truncated missing %q", out[:200])
	}
	if !strings.Contains(out, "-") && !strings.Contains(out, "│") && !strings.Contains(out, "|") {
		t.Error("fallback markers missing")
	}
}
func TestRenderDiff_MalformedNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic %v", r)
		}
	}()
	o := string([]byte{0xff, 0xfe, 0xfd}) + "hello\x00world"
	n := "\x00\x01\x02 malformed\xff"
	_ = RenderDiff(o, n, 80)
	_ = RenderDiff(o, n, 120)
	_ = RenderDiff("", "", 80)
	_ = RenderDiff("same", "same", 80)
}
func TestRenderDiff_PrettyOffAndDumb(t *testing.T) {
	o, n := "old line", "new line"
	t.Setenv("BIGGZ_PRETTY", "0")
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("TERM", "xterm-256color")
	if strings.Contains(RenderDiff(o, n, 80), "\x1b[") {
		t.Fatal("BIGGZ_PRETTY=0 should have 0 ANSI")
	}
	t.Setenv("BIGGZ_PRETTY", "")
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("TERM", "dumb")
	out2 := RenderDiff(o, n, 120)
	if strings.Contains(out2, "\x1b[") {
		t.Fatalf("TERM=dumb 0 ANSI %q", out2)
	}
	if strings.Contains(out2, "│") {
		t.Fatal("dumb should be ASCII sep")
	}
}
