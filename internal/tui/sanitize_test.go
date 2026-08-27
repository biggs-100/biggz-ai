package tui

import (
	"strings"
	"testing"
)

func TestSanitize_ReplaceTabs(t *testing.T) {
	if got := ReplaceTabs("a\tb"); got != "a    b" || strings.Contains(got, "\t") {
		t.Fatalf("ReplaceTabs tab %q", got)
	}
	for _, s := range []string{"hello", "", "no-tabs"} {
		if got := ReplaceTabs(s); got != s {
			t.Fatalf("ReplaceTabs(%q)=%q", s, got)
		}
	}
	if got := ReplaceTabs("\x1b[31m\ta\x1b[0m"); strings.Contains(got, "\t") || !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("ReplaceTabs ANSI %q", got)
	}
}
func TestSanitize_VisibleWidth(t *testing.T) {
	if VisibleWidth("a中b") != 4 {
		t.Fatal("CJK width")
	}
	if VisibleWidth("\x1b[31mhello\x1b[0m") != 5 {
		t.Fatal("ANSI width")
	}
}
func TestSanitize_TruncateToWidth(t *testing.T) {
	if got := TruncateToWidth("hello", 10); got != "hello" {
		t.Fatalf("fits %q", got)
	}
	if got := TruncateToWidth("hello world", 8); !strings.HasSuffix(got, "…") || VisibleWidth(got) > 8 {
		t.Fatalf("truncate ellipsis %q", got)
	}
	if got := TruncateToWidth("a中b中c", 4); VisibleWidth(got) > 4 {
		t.Fatalf("CJK truncate %q", got)
	}
	if TruncateToWidth("hello", 0) != "" || TruncateToWidth("hello", -1) != "" {
		t.Fatal("w<=0")
	}
}
func TestSanitize_WrapTextWithAnsi(t *testing.T) {
	s := "\x1b[32mabcdefghij klmnop\x1b[0m"
	lines := WrapTextWithAnsi(s, 10)
	if len(lines) < 2 {
		t.Fatalf("wrap len %d", len(lines))
	}
	for i, l := range lines {
		if VisibleWidth(l) > 10 {
			t.Fatalf("wrap line %d %q width %d", i, l, VisibleWidth(l))
		}
	}
	if !strings.HasPrefix(lines[1], "\x1b[32m") {
		t.Fatalf("continuation missing SGR %q", lines[1])
	}
	s2 := "\x1b[32m\x1b[32mhello\x1b[0m\x1b[0m"
	j := strings.Join(WrapTextWithAnsi(s2, 10), "")
	if strings.Count(j, "\x1b[32m") > 1 || strings.Count(j, "\x1b[0m") > 1 || strings.Contains(j, "\x1b[32m\x1b[32m") {
		t.Fatalf("coalesce %q", j)
	}
}
func TestSanitize_ShortenPath(t *testing.T) {
	p := "a/b/c/d/e/f/g.txt"
	got := ShortenPath(p, 10)
	if !strings.Contains(got, "…") || !strings.HasPrefix(got, "a/") || !strings.HasSuffix(got, "g.txt") || VisibleWidth(got) > 10 {
		t.Fatalf("shorten long %q", got)
	}
	if ShortenPath("src/main.go", 20) != "src/main.go" {
		t.Fatal("short unchanged")
	}
	for _, w := range []int{3, 2, 1} {
		if g := ShortenPath(p, w); VisibleWidth(g) > w {
			t.Fatalf("shorten w=%d %q", w, g)
		}
	}
	s := "\x1b[31mhello\x1b[0m world 中文"
	_ = VisibleWidth(s)
	_ = ReplaceTabs("a\tb")
	_ = TruncateToWidth(s, 5)
	_ = WrapTextWithAnsi(s, 5)
	_ = ShortenPath("/a/very/long/path/to/file.txt", 10)
}
