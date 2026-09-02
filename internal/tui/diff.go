package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/biggs-100/biggz-ai/internal/tui/styles"
)

const diffCap = 1 << 20

func isDiffPretty() bool {
	if os.Getenv("BIGGZ_PRETTY") == "0" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return styles.IsPrettyEnabled()
}
func hlA(s string) string {
	if s == "" || !isDiffPretty() {
		return s
	}
	return "\x1b[32m\x1b[1m" + s + "\x1b[0m"
}
func hlR(s string) string {
	if s == "" || !isDiffPretty() {
		return s
	}
	return "\x1b[31m\x1b[1m" + s + "\x1b[0m"
}
func RenderDiff(oldText, newText string, width int) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = diffFallback(oldText, newText, width)
		}
	}()
	if width <= 0 {
		width = 80
	}
	if len(oldText)+len(newText) > diffCap {
		return diffFallback(oldText, newText, width)
	}
	if oldText == newText {
		if oldText == "" {
			return ""
		}
		var res []string
		for _, l := range strings.Split(oldText, "\n") {
			l = ReplaceTabs(l)
			if VisibleWidth(l) > width {
				res = append(res, WrapTextWithAnsi(l, width)...)
			} else {
				res = append(res, l)
			}
		}
		s := strings.Join(res, "\n")
		if !isDiffPretty() {
			return ansi.Strip(s)
		}
		return s
	}
	var diffs []diffmatchpatch.Diff
	func() {
		defer func() { _ = recover() }()
		dmp := diffmatchpatch.New()
		diffs = dmp.DiffMain(oldText, newText, false)
		dmp.DiffCleanupSemantic(diffs)
	}()
	if diffs == nil {
		return diffFallback(oldText, newText, width)
	}
	if width > 100 {
		return diffSplit(diffs, width)
	}
	return diffUnified(diffs, width)
}
func diffSplit(diffs []diffmatchpatch.Diff, width int) string {
	sep := " │ "
	if !isDiffPretty() {
		sep = " | "
	}
	cw := (width - VisibleWidth(sep)) / 2
	if cw < 10 {
		cw = 10
	}
	var lb, rb strings.Builder
	for _, d := range diffs {
		parts := strings.Split(d.Text, "\n")
		for i, p := range parts {
			p = ReplaceTabs(p)
			var r string
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				r = p
			case diffmatchpatch.DiffDelete:
				r = hlR(p)
			case diffmatchpatch.DiffInsert:
				r = hlA(p)
			}
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				lb.WriteString(r)
				rb.WriteString(r)
			case diffmatchpatch.DiffDelete:
				lb.WriteString(r)
			case diffmatchpatch.DiffInsert:
				rb.WriteString(r)
			}
			if i < len(parts)-1 {
				switch d.Type {
				case diffmatchpatch.DiffEqual:
					lb.WriteString("\n")
					rb.WriteString("\n")
				case diffmatchpatch.DiffDelete:
					lb.WriteString("\n")
				case diffmatchpatch.DiffInsert:
					rb.WriteString("\n")
				}
			}
		}
	}
	ll := strings.Split(lb.String(), "\n")
	rl := strings.Split(rb.String(), "\n")
	m := len(ll)
	if len(rl) > m {
		m = len(rl)
	}
	var out []string
	for i := 0; i < m; i++ {
		l, r := "", ""
		if i < len(ll) {
			l = ll[i]
		}
		if i < len(rl) {
			r = rl[i]
		}
		if VisibleWidth(l) > cw {
			l = TruncateToWidth(l, cw)
		}
		if VisibleWidth(r) > cw {
			r = TruncateToWidth(r, cw)
		}
		lp := l + strings.Repeat(" ", maxInt(0, cw-VisibleWidth(l)))
		line := lp + sep + r
		if VisibleWidth(line) > width {
			line = TruncateToWidth(line, width)
		}
		out = append(out, line)
	}
	s := strings.Join(out, "\n")
	if !isDiffPretty() {
		return ansi.Strip(s)
	}
	return s
}
func diffUnified(diffs []diffmatchpatch.Diff, width int) string {
	var b strings.Builder
	for _, d := range diffs {
		parts := strings.Split(d.Text, "\n")
		for i, p := range parts {
			p = ReplaceTabs(p)
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				b.WriteString(p)
			case diffmatchpatch.DiffDelete:
				b.WriteString(hlR(p))
			case diffmatchpatch.DiffInsert:
				b.WriteString(hlA(p))
			}
			if i < len(parts)-1 {
				b.WriteString("\n")
			}
		}
	}
	var out []string
	for _, l := range strings.Split(b.String(), "\n") {
		if VisibleWidth(l) > width {
			out = append(out, WrapTextWithAnsi(l, width)...)
		} else {
			out = append(out, l)
		}
	}
	s := strings.Join(out, "\n")
	if !isDiffPretty() {
		return ansi.Strip(s)
	}
	return s
}
func diffFallback(oldText, newText string, width int) string {
	pretty := isDiffPretty()
	note := "[diff truncated: >1MB, showing line-level]"
	ol := strings.Split(oldText, "\n")
	nl := strings.Split(newText, "\n")
	if len(ol) > 30 {
		ol = ol[:30]
	}
	if len(nl) > 30 {
		nl = nl[:30]
	}
	var out []string
	out = append(out, note)
	for _, l := range ol {
		l = ReplaceTabs(l)
		if VisibleWidth(l) > width-2 {
			l = TruncateToWidth(l, width-2)
		}
		if pretty && l != "" {
			l = "\x1b[31m\x1b[1m" + l + "\x1b[0m"
		}
		out = append(out, "- "+l)
	}
	for _, l := range nl {
		l = ReplaceTabs(l)
		if VisibleWidth(l) > width-2 {
			l = TruncateToWidth(l, width-2)
		}
		if pretty && l != "" {
			l = "\x1b[32m\x1b[1m" + l + "\x1b[0m"
		}
		out = append(out, "+ "+l)
	}
	s := strings.Join(out, "\n")
	if !pretty {
		return ansi.Strip(s)
	}
	return s
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
