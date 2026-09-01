package screens

import (
	"hash/fnv"
	"regexp"
	"strings"
	"sync"
)

// ── repairOrphanClosingFence ───────────────────────────────────────────────
//
// Ports oh-my-pi packages/tui/src/components/markdown.ts repairOrphanClosingFence.
// Gemini can emit a bare closing fence without its opener; CommonMark then
// treats that lone fence as an opener and swallows the rest of the document.
// Repair the unambiguous rich-document shape: one unmatched bare fence after
// prose, followed by both an ATX heading and a GFM table delimiter.
// Also handles generic unclosed fences (append closing marker) for help/dashboard
// preview stability.

var (
	markdownFenceLine   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})[ \\t]*(.*)$")
	markdownHeadingLine = regexp.MustCompile("^ {0,3}#{1,6}[ \\t]+\\S")
	fencedSourceIntro   = regexp.MustCompile("(?i)\\b(?:code|example|markdown|output|snippet|source)\\s*:?\\s*$")
)

func isGfmTableDelimiter(line, headerLine string) bool {
	if headerLine == "" || !strings.Contains(line, "|") || !strings.Contains(headerLine, "|") {
		return false
	}
	trim := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "|")
		s = strings.TrimSuffix(s, "|")
		return s
	}
	delimiterCells := strings.Split(trim(line), "|")
	headerCells := strings.Split(trim(headerLine), "|")
	if len(delimiterCells) < 2 || len(headerCells) != len(delimiterCells) {
		return false
	}
	cellRe := regexp.MustCompile("^:?-{3,}:?$")
	for _, c := range delimiterCells {
		if !cellRe.MatchString(strings.TrimSpace(c)) {
			return false
		}
	}
	for _, c := range headerCells {
		if strings.TrimSpace(c) == "" {
			return false
		}
	}
	return true
}

// RepairOrphanClosingFence repairs orphan fences for stable markdown preview.
// It mirrors the TypeScript implementation: remove a lone bare fence that is
// followed by both a heading and a GFM table delimiter. Additionally, if a
// fence remains unclosed, the helper closes it so help/dashboard previews
// do not flicker.
func RepairOrphanClosingFence(text string) string {
	lines := strings.Split(text, "\n")
	type openInfo struct {
		index  int
		marker string
		info   string
	}
	var open *openInfo
	for idx, line := range lines {
		m := markdownFenceLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		marker := m[1]
		info := strings.TrimSpace(m[2])
		if open == nil {
			open = &openInfo{index: idx, marker: marker, info: info}
			continue
		}
		if len(marker) > 0 && len(open.marker) > 0 && marker[0] == open.marker[0] && len(marker) >= len(open.marker) && info == "" {
			open = nil
		}
	}
	if open == nil {
		return text
	}
	if open.info != "" {
		return text
	}
	previous := ""
	for i := open.index - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t != "" {
			previous = t
			break
		}
	}
	if previous == "" || strings.HasSuffix(previous, ":") || fencedSourceIntro.MatchString(previous) {
		return closeUnclosedFence(text, open.marker)
	}
	hasHeading := false
	hasTableDelimiter := false
	for i := open.index + 1; i < len(lines); i++ {
		line := lines[i]
		if markdownHeadingLine.MatchString(line) {
			hasHeading = true
		}
		var prev string
		if i-1 >= 0 {
			prev = lines[i-1]
		}
		if isGfmTableDelimiter(line, prev) {
			hasTableDelimiter = true
		}
		if hasHeading && hasTableDelimiter {
			newLines := append([]string{}, lines[:open.index]...)
			newLines = append(newLines, lines[open.index+1:]...)
			return strings.Join(newLines, "\n")
		}
	}
	return closeUnclosedFence(text, open.marker)
}

func closeUnclosedFence(text, marker string) string {
	if marker == "" {
		return text
	}
	trimmed := strings.TrimRight(text, "\n\t ")
	if strings.HasSuffix(trimmed, marker) {
		return text
	}
	if strings.HasSuffix(text, "\n") {
		return text + marker + "\n"
	}
	return text + "\n" + marker + "\n"
}

// CloseUnclosedFence closes an unclosed fence by appending its marker.
func CloseUnclosedFence(text string) string {
	lines := strings.Split(text, "\n")
	type openInfo struct {
		marker string
		info   string
	}
	var open *openInfo
	for _, line := range lines {
		m := markdownFenceLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		marker := m[1]
		info := strings.TrimSpace(m[2])
		if open == nil {
			open = &openInfo{marker: marker, info: info}
			continue
		}
		if len(marker) > 0 && len(open.marker) > 0 && marker[0] == open.marker[0] && len(marker) >= len(open.marker) && info == "" {
			open = nil
		}
	}
	if open == nil {
		return text
	}
	return closeUnclosedFence(text, open.marker)
}

// ── LRU cache for rendered markdown ────────────────────────────────────────

const markdownLRUCap = 200

var (
	markdownCacheMu    sync.Mutex
	markdownCacheData  = make(map[string]string, markdownLRUCap)
	markdownCacheOrder []string
)

func hashContent(content string, width int) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	sum := h.Sum64()
	sum ^= uint64(width) * 0x9e3779b97f4a7c15
	const hexDigits = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = hexDigits[sum&0xf]
		sum >>= 4
	}
	return string(buf[:])
}

func markdownCacheGet(key string) (string, bool) {
	markdownCacheMu.Lock()
	defer markdownCacheMu.Unlock()
	v, ok := markdownCacheData[key]
	if !ok {
		return "", false
	}
	for i, k := range markdownCacheOrder {
		if k == key {
			copy(markdownCacheOrder[i:], markdownCacheOrder[i+1:])
			markdownCacheOrder = markdownCacheOrder[:len(markdownCacheOrder)-1]
			markdownCacheOrder = append(markdownCacheOrder, key)
			break
		}
	}
	return v, true
}

func markdownCacheSet(key, value string) {
	markdownCacheMu.Lock()
	defer markdownCacheMu.Unlock()
	if _, exists := markdownCacheData[key]; exists {
		markdownCacheData[key] = value
		for i, k := range markdownCacheOrder {
			if k == key {
				copy(markdownCacheOrder[i:], markdownCacheOrder[i+1:])
				markdownCacheOrder = markdownCacheOrder[:len(markdownCacheOrder)-1]
				markdownCacheOrder = append(markdownCacheOrder, key)
				break
			}
		}
		return
	}
	if len(markdownCacheOrder) >= markdownLRUCap {
		oldest := markdownCacheOrder[0]
		delete(markdownCacheData, oldest)
		markdownCacheOrder = markdownCacheOrder[1:]
	}
	markdownCacheData[key] = value
	markdownCacheOrder = append(markdownCacheOrder, key)
}

// CachedBuildHelpContent wraps buildHelpContent with LRU.
func CachedBuildHelpContent(items []HelpContent, width int) string {
	key := hashContent(strings.Join(helpTitles(items), ",")+strings.Join(helpParagraphs(items), "|"), width) + ":" + hashContent("", width)
	if cached, ok := markdownCacheGet(key); ok {
		return cached
	}
	copied := make([]HelpContent, len(items))
	for i, h := range items {
		para := LatexToUnicode(RepairOrphanClosingFence(h.Paragraph))
		copied[i] = HelpContent{Title: h.Title, Keys: h.Keys, Paragraph: para}
	}
	rendered := buildHelpContent(copied, width)
	markdownCacheSet(key, rendered)
	return rendered
}

func helpTitles(items []HelpContent) []string {
	out := make([]string, len(items))
	for i, h := range items {
		out[i] = h.Title
	}
	return out
}
func helpParagraphs(items []HelpContent) []string {
	out := make([]string, len(items))
	for i, h := range items {
		out[i] = h.Paragraph
	}
	return out
}

// ClearMarkdownCache clears the LRU (for tests).
func ClearMarkdownCache() {
	markdownCacheMu.Lock()
	defer markdownCacheMu.Unlock()
	markdownCacheData = make(map[string]string, markdownLRUCap)
	markdownCacheOrder = nil
}

// MarkdownCacheLen returns current cache size (for tests).
func MarkdownCacheLen() int {
	markdownCacheMu.Lock()
	defer markdownCacheMu.Unlock()
	return len(markdownCacheData)
}

// ── latex-to-unicode ───────────────────────────────────────────────────────

var latexReplacements = []struct{ latex, unicode string }{
	{`\alpha`, "α"}, {`\beta`, "β"}, {`\gamma`, "γ"}, {`\delta`, "δ"}, {`\epsilon`, "ε"},
	{`\zeta`, "ζ"}, {`\eta`, "η"}, {`\theta`, "θ"}, {`\iota`, "ι"}, {`\kappa`, "κ"},
	{`\lambda`, "λ"}, {`\mu`, "μ"}, {`\nu`, "ν"}, {`\xi`, "ξ"}, {`\pi`, "π"},
	{`\rho`, "ρ"}, {`\sigma`, "σ"}, {`\tau`, "τ"}, {`\upsilon`, "υ"}, {`\phi`, "φ"},
	{`\chi`, "χ"}, {`\psi`, "ψ"}, {`\omega`, "ω"},
	{`\Gamma`, "Γ"}, {`\Delta`, "Δ"}, {`\Theta`, "Θ"}, {`\Lambda`, "Λ"}, {`\Xi`, "Ξ"},
	{`\Pi`, "Π"}, {`\Sigma`, "Σ"}, {`\Phi`, "Φ"}, {`\Psi`, "Ψ"}, {`\Omega`, "Ω"},
	{`\times`, "×"}, {`\div`, "÷"}, {`\pm`, "±"}, {`\mp`, "∓"}, {`\cdot`, "·"},
	{`\leq`, "≤"}, {`\le`, "≤"}, {`\geq`, "≥"}, {`\ge`, "≥"}, {`\neq`, "≠"}, {`\ne`, "≠"},
	{`\approx`, "≈"}, {`\equiv`, "≡"}, {`\sim`, "∼"}, {`\simeq`, "≃"},
	{`\infty`, "∞"}, {`\partial`, "∂"}, {`\nabla`, "∇"}, {`\sum`, "∑"}, {`\prod`, "∏"},
	{`\int`, "∫"}, {`\sqrt`, "√"}, {`\rightarrow`, "→"}, {`\to`, "→"}, {`\leftarrow`, "←"},
	{`\Rightarrow`, "⇒"}, {`\Leftarrow`, "⇐"}, {`\leftrightarrow`, "↔"}, {`\Leftrightarrow`, "⇔"},
	{`\ldots`, "…"}, {`\cdots`, "⋯"}, {`\vdots`, "⋮"}, {`\ddots`, "⋱"},
	{`\forall`, "∀"}, {`\exists`, "∃"}, {`\in`, "∈"}, {`\notin`, "∉"}, {`\subset`, "⊂"},
	{`\supset`, "⊃"}, {`\subseteq`, "⊆"}, {`\supseteq`, "⊇"}, {`\cup`, "∪"}, {`\cap`, "∩"},
	{`\emptyset`, "∅"}, {`\varnothing`, "∅"},
}

// LatexToUnicode replaces common LaTeX math commands with unicode equivalents.
func LatexToUnicode(src string) string {
	if src == "" {
		return src
	}
	if !strings.Contains(src, "\\") && !strings.Contains(src, "$") {
		return src
	}
	out := src
	out = strings.ReplaceAll(out, "$$", "")
	out = strings.ReplaceAll(out, "$", "")
	out = strings.ReplaceAll(out, "\\[", "")
	out = strings.ReplaceAll(out, "\\]", "")
	out = strings.ReplaceAll(out, "\\(", "")
	out = strings.ReplaceAll(out, "\\)", "")
	for strings.Contains(out, `\frac`) {
		idx := strings.Index(out, `\frac`)
		if idx == -1 {
			break
		}
		rest := out[idx+5:]
		rest = strings.TrimLeft(rest, " \t")
		a, rem, ok1 := extractBraced(rest)
		if !ok1 {
			break
		}
		rem = strings.TrimLeft(rem, " \t")
		b, rem2, ok2 := extractBraced(rem)
		if !ok2 {
			break
		}
		repl := "(" + a + "/" + b + ")"
		out = out[:idx] + repl + rem2
	}
	for _, r := range latexReplacements {
		if strings.Contains(out, r.latex) {
			out = strings.ReplaceAll(out, r.latex, r.unicode)
		}
	}
	out = strings.ReplaceAll(out, "^{2}", "²")
	out = strings.ReplaceAll(out, "^{3}", "³")
	out = strings.ReplaceAll(out, "^{1}", "¹")
	out = strings.ReplaceAll(out, "^{0}", "⁰")
	out = strings.ReplaceAll(out, "{", "")
	out = strings.ReplaceAll(out, "}", "")
	// Only collapse whitespace if we actually transformed latex; keep prose single-spaced.
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func extractBraced(s string) (content, rest string, ok bool) {
	if len(s) == 0 || s[0] != '{' {
		return "", s, false
	}
	depth := 0
	for i, ch := range s {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], true
			}
		}
	}
	return "", s, false
}
