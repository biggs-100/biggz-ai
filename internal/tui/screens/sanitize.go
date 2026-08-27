// Local sanitize helpers for screens package to avoid import cycle with internal/tui.
// Logic mirrors internal/tui/sanitize.go (single authority). Keep in sync.
// See design decision: Helper location internal/tui/sanitize.go is single width authority,
// but screens cannot import parent tui due to cycle (tui -> screens). This file provides
// cycle-safe local copies for screens Renderers.

package screens

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func ReplaceTabs(s string) string { return strings.ReplaceAll(s, "\t", "    ") }
func VisibleWidth(s string) int   { return runewidth.StringWidth(ansi.Strip(s)) }

func coalesceSGR(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	last, was := "", false
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				seq := s[i : j+1]
				if was && seq == last {
					i = j + 1
					continue
				}
				b.WriteString(seq)
				last, was = seq, true
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		last, was = "", false
		i++
	}
	return b.String()
}

func TruncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if VisibleWidth(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	s = coalesceSGR(s)
	target := w - 1
	var b strings.Builder
	cur, i, stop := 0, 0, len(s)
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				b.WriteString(s[i : j+1])
				i = j + 1
				continue
			}
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && sz == 1 {
			sz = 1
		}
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if cur+rw > target {
			stop = i
			break
		}
		b.WriteString(s[i : i+sz])
		cur += rw
		i += sz
		if i >= len(s) {
			stop = len(s)
		}
	}
	b.WriteString("…")
	for idx := stop; idx < len(s); {
		if s[idx] == '\x1b' && idx+1 < len(s) && s[idx+1] == '[' {
			j := idx + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				b.WriteString(s[idx : j+1])
				idx = j + 1
				continue
			}
		}
		_, sz := utf8.DecodeRuneInString(s[idx:])
		if sz == 0 {
			sz = 1
		}
		idx += sz
	}
	return coalesceSGR(b.String())
}

func WrapTextWithAnsi(s string, w int) []string {
	if w <= 0 {
		return []string{}
	}
	if s == "" {
		return []string{""}
	}
	s = coalesceSGR(s)
	var lines []string
	var cur strings.Builder
	curW, active := 0, ""
	flush := func() {
		lines = append(lines, coalesceSGR(cur.String()))
		cur.Reset()
		curW = 0
		if active != "" {
			cur.WriteString(active)
		}
	}
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				seq := s[i : j+1]
				if seq == "\x1b[0m" {
					active = ""
				} else {
					active = seq
				}
				if !strings.HasSuffix(cur.String(), seq) {
					cur.WriteString(seq)
				}
				i = j + 1
				continue
			}
		}
		if s[i] == '\x1b' {
			j := i + 1
			end := -1
			for k := j; k < len(s); k++ {
				if s[k] == '\x07' {
					end = k
					break
				}
				if s[k] == '\x1b' && k+1 < len(s) && s[k+1] == '\\' {
					end = k + 1
					break
				}
			}
			if end != -1 {
				cur.WriteString(s[i : end+1])
				i = end + 1
				continue
			}
			cur.WriteByte(s[i])
			i++
			continue
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && sz == 1 {
			sz = 1
		}
		rw := runewidth.RuneWidth(r)
		if rw < 0 {
			rw = 0
		}
		if curW+rw > w {
			if curW == 0 {
				cur.WriteString(s[i : i+sz])
				curW += rw
				i += sz
				flush()
				continue
			}
			flush()
			continue
		}
		cur.WriteString(s[i : i+sz])
		curW += rw
		i += sz
	}
	if cur.Len() > 0 || len(lines) == 0 {
		lines = append(lines, coalesceSGR(cur.String()))
	}
	for i, l := range lines {
		lines[i] = coalesceSGR(l)
	}
	return lines
}

func ShortenPath(p string, maxWidth int) string {
	if VisibleWidth(p) <= maxWidth {
		return p
	}
	if maxWidth < 4 {
		return TruncateToWidth(p, maxWidth)
	}
	if strings.Contains(p, "/") {
		f, l := strings.Index(p, "/"), strings.LastIndex(p, "/")
		var pre, suf string
		if f != -1 {
			pre = p[:f+1]
		}
		if l != -1 && l+1 < len(p) {
			suf = p[l+1:]
		}
		if pre != "" && suf != "" {
			c := pre + "…/" + suf
			if VisibleWidth(c) <= maxWidth {
				return c
			}
		} else if pre != "" {
			c := pre + "…"
			if VisibleWidth(c) <= maxWidth {
				return c
			}
		} else if suf != "" {
			c := "…/" + suf
			if VisibleWidth(c) <= maxWidth {
				return c
			}
		}
	}
	rem := maxWidth - 1
	preW, sufW := rem/2, rem-rem/2
	pre := sliceByWidth(p, preW, true)
	suf := sliceByWidthFromEnd(p, sufW)
	cand := pre + "…" + suf
	for VisibleWidth(cand) > maxWidth && (pre != "" || suf != "") {
		if VisibleWidth(pre) >= VisibleWidth(suf) && pre != "" {
			pre = TruncateToWidth(pre, VisibleWidth(pre)-1)
			pre = sliceByWidth(pre, VisibleWidth(pre), true)
			if strings.HasSuffix(pre, "…") {
				pre = strings.TrimSuffix(pre, "…")
			}
		} else if suf != "" {
			suf = sliceByWidthFromEnd(suf, VisibleWidth(suf)-1)
		} else {
			break
		}
		cand = pre + "…" + suf
	}
	return cand
}

func sliceByWidth(s string, w int, fromStart bool) string {
	if w <= 0 {
		return ""
	}
	if VisibleWidth(s) <= w {
		return s
	}
	if !fromStart {
		return s
	}
	var b strings.Builder
	cur, i := 0, 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				b.WriteString(s[i : j+1])
				i = j + 1
				continue
			}
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		if cur+runewidth.RuneWidth(r) > w {
			break
		}
		b.WriteString(s[i : i+sz])
		cur += runewidth.RuneWidth(r)
		i += sz
	}
	return b.String()
}

func sliceByWidthFromEnd(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if VisibleWidth(s) <= w {
		return s
	}
	var runes []rune
	for _, r := range s {
		runes = append(runes, r)
	}
	wi, start := 0, len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		rw := runewidth.RuneWidth(runes[i])
		if rw < 0 {
			rw = 0
		}
		if wi+rw > w {
			break
		}
		wi += rw
		start = i
	}
	var b strings.Builder
	for i := start; i < len(runes); i++ {
		b.WriteRune(runes[i])
	}
	return b.String()
}
