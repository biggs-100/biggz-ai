package pi

import (
	"fmt"
	"strings"
)

// WrapThinking wraps text at width, preserving words and existing line breaks.
// It mimics pi-tui Markdown wrapping at termWidth but for programmatic preview.
// When width <= 0 it defaults to 80. Ansi codes are not specially handled
// (caller should strip them if needed); simple rune-count wrapping is used.
func WrapThinking(text string, width int) string {
	if width <= 0 {
		width = 80
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var out []string
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		// If line already fits, keep as-is to preserve markdown structure.
		if runeLen(line) <= width {
			out = append(out, line)
			continue
		}
		wrapped := wrapLine(line, width)
		out = append(out, wrapped...)
	}
	return strings.Join(out, "\n")
}

// CollapsedPreview returns the first maxLines lines of text with a hint for
// the remaining lines, like pi-pretty's tool result collapsed state.
// When expanded is false it truncates and appends "… N more lines (Ctrl+T to expand)".
// Width controls wrapping before truncation; 0 defaults to 80.
func CollapsedPreview(text string, width, maxLines int, expanded bool) string {
	if maxLines <= 0 {
		maxLines = 3
	}
	wrapped := WrapThinking(text, width)
	lines := strings.Split(wrapped, "\n")
	if expanded || len(lines) <= maxLines {
		return wrapped
	}
	preview := strings.Join(lines[:maxLines], "\n")
	remaining := len(lines) - maxLines
	// Keep hint similar to pi-pretty's "ctrl+o to expand" but for thinking use Ctrl+T.
	hint := fmt.Sprintf("\n… %d more lines (Ctrl+T to expand)", remaining)
	return preview + hint
}

// FormatThinking combines wrapping and collapsed preview for thinking blocks.
// It mirrors formatToolResultOutput pattern but uses 80-char wrap and 3-line
// collapsed preview as requested for "wrap desplegable para ver o no".
func FormatThinking(text string, width int, expanded bool) string {
	if width <= 0 {
		width = 80
	}
	return CollapsedPreview(text, width, 3, expanded)
}

func runeLen(s string) int {
	return len([]rune(s))
}

func wrapLine(line string, width int) []string {
	var result []string
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{line}
	}
	var cur strings.Builder
	curLen := 0
	for _, w := range words {
		wLen := runeLen(w)
		// If word itself longer than width, hard-break it.
		if wLen > width {
			// Flush current line first.
			if curLen > 0 {
				result = append(result, cur.String())
				cur.Reset()
				curLen = 0
			}
			// Break long word into chunks.
			runes := []rune(w)
			for len(runes) > width {
				result = append(result, string(runes[:width]))
				runes = runes[width:]
			}
			if len(runes) > 0 {
				cur.WriteString(string(runes))
				curLen = len(runes)
			}
			continue
		}
		need := wLen
		if curLen > 0 {
			need++ // space
		}
		if curLen+need > width {
			result = append(result, cur.String())
			cur.Reset()
			cur.WriteString(w)
			curLen = wLen
		} else {
			if curLen > 0 {
				cur.WriteString(" ")
				curLen++
			}
			cur.WriteString(w)
			curLen += wLen
		}
	}
	if curLen > 0 {
		result = append(result, cur.String())
	}
	if len(result) == 0 {
		return []string{line}
	}
	return result
}
