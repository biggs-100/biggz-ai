package sdd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

type SubAgentResult struct {
	Phase           string
	WhatDone        string
	ArtifactsPaths  string
	Risks           string
	NextRecommended string
	Preview         string
	Diff            string
	Decisions       string
	Commands        string
	Validation      string
	Failure         string
}

// compactK formats tokens: 4100→4.1k, 2250→2.2k, hide window if ==spent or <1k.
func compactK(n int) string {
	if n < 0 {
		n = 0
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}
func formatFleetTokens(window, spent int) string {
	if window == spent || window < 1000 {
		return compactK(spent)
	}
	return compactK(window) + "›" + compactK(spent)
}

// FormatFleetTokens exported for tests.
func FormatFleetTokens(window, spent int) string { return formatFleetTokens(window, spent) }
func CompactK(n int) string                      { return compactK(n) }

// WaitRun holds minimal run info for headline.
type WaitRun struct {
	Name  string
	State string
}

// FormatWaitHeadline builds 1-line headline per POLISH-ORCH-02: Wait 23s · 2 runs (sdd-apply running, sdd-verify queued) — open Fleet for detail.
// Returns solid headline and optional dim hint (≤2 lines total). Never dumps full formatAsyncRunList.
func FormatWaitHeadline(elapsedSec int, runs []WaitRun) string {
	if len(runs) == 0 {
		return fmt.Sprintf("Wait %ds \u00b7 0 runs \u2014 open Fleet for detail", elapsedSec)
	}
	var summaries []string
	for _, r := range runs {
		n := strings.TrimSpace(r.Name)
		if n == "" {
			n = "run"
		}
		st := strings.TrimSpace(r.State)
		if st == "" {
			st = "waiting"
		}
		summaries = append(summaries, fmt.Sprintf("%s %s", n, st))
	}
	inner := strings.Join(summaries, ", ")
	headline := fmt.Sprintf("Wait %ds \u00b7 %d runs (%s) \u2014 open Fleet for detail", elapsedSec, len(runs), inner)
	return headline
}

// FormatWaitHeadlineLines returns headline split into ≤2 lines: first solid, optional dim hint.
func FormatWaitHeadlineLines(elapsedSec int, runs []WaitRun) []string {
	head := FormatWaitHeadline(elapsedSec, runs)
	// ensure single line headline, no extra dump
	head = strings.Join(strings.Fields(head), " ")
	if visibleWidth(head) <= 120 {
		return []string{head}
	}
	// if excessively long, truncate headline to 120 and add dim hint as second line (never full list)
	head = truncateToWidth(head, 120)
	return []string{head, "\x1b[2m\u2014 open Fleet for detail\x1b[0m"}
}

// RightAlign pads left to width w visible.
func RightAlign(s string, w int) string {
	vw := visibleWidth(s)
	if vw >= w {
		return s
	}
	return strings.Repeat(" ", w-vw) + s
}

// TableCellBudget mirrors tui.TableCellBudget: (width-6)/2 min 5.
func TableCellBudget(width int) int {
	b := (width - 6) / 2
	if b < 5 {
		b = 5
	}
	return b
}

// HeaderGroups renders collapsed header 2 groups per POLISH-TUI-03.
func HeaderGroups(running, queued, capU, capL int, paneWarn bool, elapsed, tok string) string {
	g1 := fmt.Sprintf("%d running\u00b7%d queued\u00b7cap %d/%d", running, queued, capU, capL)
	if paneWarn {
		g1 += "\u00b7pane \u26a0"
	}
	g2 := fmt.Sprintf("%s\u00b7%s", strings.TrimSpace(elapsed), strings.TrimSpace(tok))
	return g1 + " \u00b7 " + g2
}

// VisibleWorkflowRows implements tail visibility per POLISH-TUI-05.
func VisibleWorkflowRows[T any](rows []T, limit int) ([]T, int) {
	if limit <= 0 || len(rows) <= limit {
		return rows, 0
	}
	hidden := len(rows) - limit
	return rows[:limit], hidden
}

// VisibleWorkflowRowsStrings helper for string slices with hidden tail formatting.
func VisibleWorkflowRowsStrings(rows []string, limit int) ([]string, string) {
	vis, hidden := VisibleWorkflowRows(rows, limit)
	if hidden == 0 {
		return vis, ""
	}
	return vis, fmt.Sprintf("\u2026 +%d hidden", hidden)
}

func localizeStatus(status, lang string) string {
	if strings.ToLower(strings.TrimSpace(lang)) != "es" {
		return status
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "pass", "done", "ok":
		return "éxito"
	case "warning", "warn", "pending", "partial":
		return "atención"
	case "error", "fail", "failed", "blocked", "missing":
		return "error"
	default:
		return status
	}
}

func noneLabel(lang string) string {
	if strings.ToLower(strings.TrimSpace(lang)) == "es" {
		return "Ninguno"
	}
	return "None"
}

func renderSynthesisWithLang(r SubAgentResult, lang string) string {
	return RenderSynthesisWithWidth(r, lang, 80)
}

func renderSynthesisWithWidth(r SubAgentResult, lang string, width int) string {
	if width <= 0 {
		width = 80
	}
	budget := cellBudget(width)
	phase := strings.TrimSpace(r.Phase)
	if phase == "" {
		phase = "phase/agent"
	}
	none := noneLabel(lang)
	arts := strings.TrimSpace(r.ArtifactsPaths)
	if arts == "" {
		arts = none
	}
	risks := strings.TrimSpace(r.Risks)
	if risks == "" {
		risks = none
	}
	next := strings.TrimSpace(r.NextRecommended)
	if next == "" {
		next = none
	}
	// derive lifecycle status
	status := deriveLifecycleStatus(r)
	status = localizeStatus(status, lang)
	lifecycle := renderLifecycle(phase, status, next)

	var b strings.Builder
	b.WriteString("## Sub-agent Result: " + sanitizeForWidth(phase, width) + "\n")
	// What was done as table + checklist
	b.WriteString("**What was done:**\n")
	rows, checklist := parseWhatDoneRows(r.WhatDone, budget)
	// Ensure at least header present
	if len(rows) == 0 {
		rows = [][]string{{none, none}}
	}
	// Render table chunked — width-aware budget (cellBudget(width)) fixes 34 vs 37 mismatch
	tableMD := renderTable(rows, width)
	b.WriteString(tableMD)
	if len(checklist) > 0 {
		for _, item := range checklist {
			// sanitize checklist item but keep prefix
			sanitized := sanitizePlain(item)
			// per-item truncate to width
			sanitized = truncateToWidth(sanitized, width)
			b.WriteString(sanitized + "\n")
		}
	}
	// lifecycle one-line
	b.WriteString(lifecycle + "\n")
	b.WriteString("**Artifacts/Paths:** " + sanitizePlain(arts) + "\n")
	b.WriteString("**Risks / Open Questions:** " + sanitizePlain(risks) + "\n")
	b.WriteString("**Next Recommended:** " + sanitizePlain(next) + "\n")
	// Preview sanitized 300 (width-aware cell budget only; preview stays 300)
	previewRaw := strings.TrimSpace(r.Preview)
	if previewRaw == "" {
		b.WriteString("**Preview:** " + none + "\n")
	} else {
		b.WriteString("**Preview:** " + formatPreview(previewRaw) + "\n")
	}
	diffRaw := strings.TrimSpace(r.Diff)
	if diffRaw == "" {
		b.WriteString("**Diff:** " + none + "\n")
	} else {
		b.WriteString("**Diff:** " + formatDiff(diffRaw) + "\n")
	}
	if v := strings.TrimSpace(r.Decisions); v != "" {
		b.WriteString("**Decisions:** " + sanitizeForWidth(v, width) + "\n")
	}
	if v := strings.TrimSpace(r.Commands); v != "" {
		b.WriteString("**Commands:** " + sanitizeForWidth(v, width) + "\n")
	}
	validation := strings.TrimSpace(r.Validation)
	if validation == "" {
		validation = none
	} else {
		validation = sanitizeForWidth(validation, width)
	}
	b.WriteString("**Validation:** " + validation + "\n")
	if v := strings.TrimSpace(r.Failure); v != "" {
		human := humanizeFailure(v)
		if human == "" {
			human = v
		}
		b.WriteString("**Failure:** " + sanitizeForWidth(human, width) + "\n")
	}
	return b.String()
}

func RenderSynthesis(r SubAgentResult) string {
	return RenderSynthesisLocalized(r, "en")
}

// RenderSynthesisWithWidth renders with explicit width. Width <=0 defaults to 80.
// Reversible hotfix: width controls table cell budget (cellBudget(width)) and per-field truncation.
// Preview remains 300 visible width; diff 80 is still truncated via formatDiff but table adapts.
func RenderSynthesisWithWidth(r SubAgentResult, lang string, width int) string {
	if width <= 0 {
		width = 80
	}
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if normalized == "" {
		normalized = "en"
	}
	// handle numeric lang passed accidentally as "80"/"100" — treat as width hint if width is default
	if num, err := strconv.Atoi(normalized); err == nil && num > 0 && width == 80 {
		// lang was numeric width string; fallback lang to en and use that numeric width if no explicit override
		// caller used RenderSynthesisWithWidth with lang="80"; honor numeric width
		width = num
		normalized = "en"
	}
	if normalized != "es" && normalized != "en" {
		if num, err := strconv.Atoi(normalized); err == nil && num > 0 {
			normalized = "en"
		} else {
			detected := DetectLanguage(lang)
			if detected == "es" || detected == "en" {
				normalized = detected
			} else {
				normalized = "en"
			}
		}
	}
	return renderSynthesisWithWidth(r, normalized, width)
}

// RenderPrettySynthesis returns a pretty-decorated synthesis that remains valid for HasSynthesis.
// It wraps RenderSynthesis with external separators and emoji outside the markers,
// never wrapping markers in a code fence. The markers remain verbatim so
// synthesis_gate.go HasSynthesis still passes, and visibleWidth handles emoji width 2 via go-runewidth.
func RenderPrettySynthesis(r SubAgentResult) string {
	raw := RenderSynthesis(r)
	var b strings.Builder
	// External pretty decoration: separator + emoji header outside markers
	b.WriteString("---\n")
	b.WriteString("✨ Sub-agent Result — pretty ✨\n\n")
	b.WriteString(raw)
	if !strings.HasSuffix(raw, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n---\n")
	return b.String()
}

// DetectLanguage heuristically detects human language: "es" for Spanish, "en" otherwise.
// Heuristic: diacritics áéíóúñ¿¡ -> es, Spanish keywords que/en/por/con/para/continua/dale/procede etc.,
// short ambiguous hi/ok/go/dale -> en default, tie or unknown -> en.
func DetectLanguage(text string) string {
	if text == "" {
		return "en"
	}
	if strings.ContainsAny(text, "áéíóúÁÉÍÓÚñÑ¿¡") {
		return "es"
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "en"
	}
	words := extractWordsLower(trimmed)
	if len(words) == 0 {
		return "en"
	}
	if len(words) == 1 && isAmbiguousShort(words[0]) {
		return "en"
	}
	// Very short single token without diacritic defaults to en unless it's a strong Spanish keyword longer than 2.
	// This handles "ok", "hi", "dale", "go" as en, but "que" alone should be es.
	// So check if single token is Spanish strong and not ambiguous -> es.
	if len(words) == 1 {
		if _, ok := spanishKeywords[words[0]]; ok && !isAmbiguousShort(words[0]) {
			// except single "en"? "en" is length 2 and Spanish, treat as es
			return "es"
		}
		if _, ok := englishKeywords[words[0]]; ok {
			return "en"
		}
		// unknown single short word defaults en
		if len(words[0]) <= 3 {
			return "en"
		}
	}
	spanishCount := 0
	englishCount := 0
	for _, w := range words {
		if isAmbiguousShort(w) {
			continue
		}
		if _, ok := spanishKeywords[w]; ok {
			spanishCount++
		}
		if _, ok := englishKeywords[w]; ok {
			englishCount++
		}
	}
	if spanishCount > englishCount {
		return "es"
	}
	if englishCount > spanishCount {
		return "en"
	}
	// tie: if any Spanish keyword present, prefer es when text contains Spanish diacritic-like pattern already handled;
	// otherwise default en (spec PS5 short ambiguous -> en)
	return "en"
}

func extractWordsLower(s string) []string {
	lower := strings.ToLower(s)
	var words []string
	var cur strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				words = append(words, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

func isAmbiguousShort(w string) bool {
	_, ok := ambiguousShort[w]
	return ok
}

var ambiguousShort = map[string]struct{}{
	"ok": {}, "okay": {}, "hi": {}, "hello": {}, "hey": {}, "go": {}, "dale": {},
}

var spanishKeywords = map[string]struct{}{
	"que": {}, "qué": {}, "en": {}, "por": {}, "con": {}, "para": {}, "sin": {}, "sobre": {}, "entre": {}, "hasta": {}, "desde": {},
	"nos": {}, "nosotros": {}, "vamos": {}, "quedamos": {}, "continua": {}, "continúa": {}, "continuar": {}, "continuamos": {}, "procede": {}, "procedamos": {}, "dale": {},
	"donde": {}, "dónde": {}, "quien": {}, "quién": {}, "como": {}, "cómo": {}, "cuando": {}, "cuándo": {}, "porque": {}, "porqué": {},
	"hola": {}, "gracias": {}, "si": {}, "sí": {}, "listo": {}, "perfecto": {}, "ajusta": {}, "ajustar": {}, "detener": {}, "parar": {}, "cerrar": {}, "corregir": {}, "proseguir": {},
	"esta": {}, "está": {}, "este": {}, "esto": {}, "estamos": {}, "estoy": {}, "estas": {}, "estás": {}, "tiene": {}, "tienen": {}, "tenemos": {}, "tengo": {}, "eres": {}, "es": {}, "son": {}, "mucho": {}, "muy": {}, "bien": {},
	"siguiente": {}, "pendiente": {}, "sigamos": {}, "prosigue": {},
}

var englishKeywords = map[string]struct{}{
	"hello": {}, "continue": {}, "continuing": {}, "proceed": {}, "proceeding": {}, "adjust": {}, "adjusting": {}, "stop": {}, "stopping": {}, "correct": {}, "correcting": {}, "close": {}, "closing": {},
	"please": {}, "thanks": {}, "thank": {}, "lets": {}, "let": {},
}

// RenderSynthesisLocalized renders synthesis with language hint. Markers and technical identifiers stay English
// (## Sub-agent Result:, **Artifacts/Paths:**, **Risks / Open Questions:**, **Next Recommended:**, | Topic | Decision |,
// paths with "/" or "sdd/", code like "ORDER BY"). If lang is empty, falls back to "en" (or DetectLanguage).
// Keeps RenderSynthesis compatible; localized content is provided by sub-agent via hint, wrapper preserves whitelist.
func RenderSynthesisLocalized(r SubAgentResult, lang string) string {
	normalized := strings.ToLower(strings.TrimSpace(lang))
	// width-optional: si lang es "80"/"100" numérico, tratar como width (compat hotfix)
	if num, err := strconv.Atoi(normalized); err == nil && num > 0 {
		return RenderSynthesisWithWidth(r, "en", num)
	}
	if normalized == "" {
		normalized = "en"
	}
	if normalized != "es" && normalized != "en" {
		// Try to detect if lang is actually a human message rather than code
		detected := DetectLanguage(lang)
		if detected == "es" || detected == "en" {
			normalized = detected
		} else {
			normalized = "en"
		}
	}
	return RenderSynthesisWithWidth(r, normalized, 80)
}

func deriveLifecycleStatus(r SubAgentResult) string {
	failure := strings.TrimSpace(r.Failure)
	if failure != "" {
		return "error"
	}
	validation := strings.ToLower(strings.TrimSpace(r.Validation))
	if strings.Contains(validation, "pass") {
		return "success"
	}
	next := strings.ToLower(strings.TrimSpace(r.NextRecommended))
	if next == "" || next == "none" {
		return "warning"
	}
	// default success when next exists and no failure
	if strings.Contains(validation, "fail") || strings.Contains(validation, "error") {
		return "error"
	}
	return "success"
}

// sanitizePlain replaces tabs, strips OSC/ANSI/controls and normalizes spaces.
// It preserves emoji and wide runes; visibleWidth/truncateToWidth use go-runewidth
// so emoji like ✨ count as width 2, and Preview 300 / Diff 80 truncation remains safe.
func sanitizePlain(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "	", "    ")
	s = stripOsc(s)
	s = ansi.Strip(s)
	s = stripControls(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func sanitizeForWidth(s string, w int) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "	", "    ")
	s = stripOsc(s)
	s = ansi.Strip(s)
	s = stripControls(s)
	// collapse whitespace but keep single spaces for prose
	s = strings.Join(strings.Fields(s), " ")
	if w <= 0 {
		w = 80
	}
	s = truncateToWidth(s, w)
	return s
}

func stripOsc(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == ']' {
			// OSC sequence until BEL (\x07) or ESC+\
			end := -1
			for k := i + 2; k < len(s); k++ {
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
				i = end + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripControls(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\r' || r == '\n' {
			b.WriteRune(' ')
			continue
		}
		if r < 32 || r == 127 {
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func formatPreview(s string) string {
	s = strings.ReplaceAll(s, "	", "    ")
	s = stripOsc(s)
	s = ansi.Strip(s)
	s = stripControls(s)
	s = strings.Join(strings.Fields(s), " ")
	// truncate to 300 visible width
	if visibleWidth(s) > 300 {
		s = truncateToWidth(s, 300)
	}
	return s
}

func formatDiff(s string) string {
	s = strings.ReplaceAll(s, "	", "    ")
	s = stripOsc(s)
	s = ansi.Strip(s)
	s = stripControls(s)
	s = strings.Join(strings.Fields(s), " ")
	// keep as summary, truncate to 80 if too long
	if visibleWidth(s) > 80 {
		s = truncateToWidth(s, 80)
	}
	return s
}

func parseWhatDoneRows(s string, budget int) ([][]string, []string) {
	if budget <= 0 {
		budget = cellBudget(80)
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "None" {
		return nil, nil
	}
	lines := splitWhatDoneLines(s)
	rows, checklist := classifyWhatDoneLines(lines, budget)
	if len(rows) == 0 {
		rows = fallbackWhatDoneRows(s, budget)
	}
	return rows, checklist
}

// parseWhatDoneRowsCompat keeps old signature for tests that call without budget (defaults 37)
func parseWhatDoneRowsCompat(s string) ([][]string, []string) {
	return parseWhatDoneRows(s, cellBudget(80))
}

func splitWhatDoneLines(s string) []string {
	lines := strings.Split(s, "\n")
	if len(lines) != 1 || !strings.Contains(s, ";") {
		return lines
	}
	parts := strings.Split(s, ";")
	var filtered []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) > 1 {
		return filtered
	}
	return lines
}

func isChecklistLine(line string) bool {
	if strings.HasPrefix(line, "- [x]") {
		return true
	}
	if strings.HasPrefix(line, "- [ ]") {
		return true
	}
	return strings.HasPrefix(line, "- [X]")
}

func sanitizeCell(cell string, budget int) string {
	cell = sanitizePlain(cell)
	if budget <= 0 {
		budget = cellBudget(80)
	}
	return truncateToWidth(cell, budget)
}

func tryParseRow(line string, budget int) (string, string, bool) {
	topic, decision := splitTopicDecision(line)
	if topic == "" && decision == "" {
		return "", "", false
	}
	topic = sanitizeCell(topic, budget)
	decision = sanitizeCell(decision, budget)
	return topic, decision, true
}

func classifyWhatDoneLines(lines []string, budget int) ([][]string, []string) {
	if budget <= 0 {
		budget = cellBudget(80)
	}
	var rows [][]string
	var checklist []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if isChecklistLine(line) {
			checklist = append(checklist, line)
			continue
		}
		topic, decision, ok := tryParseRow(line, budget)
		if !ok {
			continue
		}
		rows = append(rows, []string{topic, decision})
	}
	return rows, checklist
}

func fallbackWhatDoneRows(s string, budget int) [][]string {
	if budget <= 0 {
		budget = cellBudget(80)
	}
	if strings.Contains(s, ",") {
		return splitCommaFallback(s, budget)
	}
	return singleFallback(s, budget)
}

func splitCommaFallback(s string, budget int) [][]string {
	if budget <= 0 {
		budget = cellBudget(80)
	}
	parts := strings.Split(s, ",")
	var rows [][]string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		topic, decision := splitTopicDecision(p)
		if topic == "" {
			topic = p
		}
		topic = sanitizeCell(topic, budget)
		decision = sanitizeCell(decision, budget)
		rows = append(rows, []string{topic, decision})
	}
	return rows
}

func singleFallback(s string, budget int) [][]string {
	if budget <= 0 {
		budget = cellBudget(80)
	}
	topic := sanitizeCell(s, budget)
	return [][]string{{topic, ""}}
}

func splitTopicDecision(line string) (string, string) {
	// try delimiters in order
	delims := []string{":", "→", "—", "|", " - ", " – ", "="}
	for _, d := range delims {
		if idx := strings.Index(line, d); idx != -1 {
			topic := strings.TrimSpace(line[:idx])
			decision := strings.TrimSpace(line[idx+len(d):])
			if topic != "" || decision != "" {
				return topic, decision
			}
		}
	}
	// no delimiter, try comma split already handled outside
	return line, ""
}

func chunkTable(rows [][]string, max int) [][][]string {
	if max <= 0 {
		max = 7
	}
	if len(rows) <= max {
		return [][][]string{rows}
	}
	var chunks [][][]string
	for i := 0; i < len(rows); i += max {
		end := i + max
		if end > len(rows) {
			end = len(rows)
		}
		chunks = append(chunks, rows[i:end])
	}
	return chunks
}

func cellBudget(width int) int {
	b := (width - 6) / 2
	if b < 5 {
		b = 5
	}
	return b
}

func truncateCell(s string, budget int) string {
	if visibleWidth(s) > budget {
		return truncateToWidth(s, budget)
	}
	return s
}

func renderCell(s string, budget int) string {
	return truncateCell(s, budget)
}

func renderRow(cells []string, budget int) string {
	topic := ""
	decision := ""
	if len(cells) > 0 {
		topic = cells[0]
	}
	if len(cells) > 1 {
		decision = cells[1]
	}
	topic = renderCell(topic, budget)
	decision = renderCell(decision, budget)
	return "| " + topic + " | " + decision + " |\n"
}

func renderChunk(chunk [][]string, budget int) string {
	var b strings.Builder
	b.WriteString("| Topic | Decision |\n")
	b.WriteString("|-------|----------|\n")
	for _, r := range chunk {
		b.WriteString(renderRow(r, budget))
	}
	return b.String()
}

func chunkRemaining(rows [][]string, idx int) string {
	remaining := len(rows) - (idx+1)*7
	if remaining > 0 {
		return fmt.Sprintf("… +%d more\n", remaining)
	}
	return ""
}

func renderTable(rows [][]string, width int) string {
	if width <= 0 {
		width = 80
	}
	chunks := chunkTable(rows, 7)
	budget := cellBudget(width)
	var b strings.Builder
	for idx, chunk := range chunks {
		b.WriteString(renderChunk(chunk, budget))
		if idx < len(chunks)-1 {
			if more := chunkRemaining(rows, idx); more != "" {
				b.WriteString(more)
			}
		}
	}
	return b.String()
}

// visibleWidth returns visible column width using go-runewidth (emoji width 2).
func visibleWidth(s string) int { return runewidth.StringWidth(ansi.Strip(s)) }

func coalesceSGR(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	last, was := "", false
	i := 0
	for i < len(s) {
		if s[i] == '' && i+1 < len(s) && s[i+1] == '[' {
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

func sgrEnd(s string, i int) int {
	if i >= len(s) {
		return -1
	}
	if s[i] != '\x1b' {
		return -1
	}
	if i+1 >= len(s) || s[i+1] != '[' {
		return -1
	}
	j := i + 2
	for j < len(s) && s[j] != 'm' {
		j++
	}
	if j < len(s) {
		return j
	}
	return -1
}

func runeWidthSafe(r rune) int {
	w := runewidth.RuneWidth(r)
	if w < 0 {
		return 0
	}
	return w
}

func buildTruncatedPrefix(s string, target int) (string, int) {
	var b strings.Builder
	cur := 0
	i := 0
	stop := len(s)
	for i < len(s) {
		if end := sgrEnd(s, i); end != -1 {
			b.WriteString(s[i : end+1])
			i = end + 1
			continue
		}
		r, sz := utf8.DecodeRuneInString(s[i:])
		rw := runeWidthSafe(r)
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
	return b.String(), stop
}

func trailingSGR(s string, stop int) string {
	var b strings.Builder
	for idx := stop; idx < len(s); {
		if end := sgrEnd(s, idx); end != -1 {
			b.WriteString(s[idx : end+1])
			idx = end + 1
			continue
		}
		_, sz := utf8.DecodeRuneInString(s[idx:])
		if sz == 0 {
			sz = 1
		}
		idx += sz
	}
	return b.String()
}

// truncateToWidth truncates to visible width w using go-runewidth (emoji width 2), preserving ANSI SGR.
func truncateToWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if visibleWidth(s) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	s = coalesceSGR(s)
	target := w - 1
	prefix, stop := buildTruncatedPrefix(s, target)
	suffix := trailingSGR(s, stop)
	return coalesceSGR(prefix + "…" + suffix)
}

func humanizeFailure(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = stripFailurePrefix(s)
	if !strings.HasPrefix(s, "{") {
		return normalizeSpaces(s)
	}
	m, errMsg := parseFailureMap(s)
	if errMsg != "" {
		return errMsg
	}
	get := failureFieldGetter(m)
	sum := extractSummary(get)
	code, phase, status := get("code"), get("phase"), get("status")
	if sum != "" {
		return formatWithSummary(sum, code, phase, status)
	}
	if formatted := formatWithoutSummary(code, phase, status, get); formatted != "" {
		return formatted
	}
	return "failure"
}

func stripFailurePrefix(s string) string {
	if i := strings.Index(s, "{"); i > 0 && strings.Contains(strings.TrimSpace(s[:i]), "FAILURE") {
		return strings.TrimSpace(s[i:])
	}
	return s
}

func normalizeSpaces(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func parseFailureMap(s string) (map[string]any, string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		cleaned := normalizeSpaces(s)
		if len(cleaned) > 200 {
			cleaned = cleaned[:200] + "…"
		}
		return nil, "malformed failure payload: " + cleaned
	}
	return m, ""
}

func failureFieldGetter(m map[string]any) func(string) string {
	return func(k string) string {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
}

func extractSummary(get func(string) string) string {
	for _, k := range []string{"summary", "message", "error", "diagnosis"} {
		if v := get(k); v != "" {
			return v
		}
	}
	return ""
}

func formatWithSummary(sum, code, phase, status string) string {
	if phase != "" && code != "" {
		if status != "" {
			return fmt.Sprintf("%s %s (%s): %s", phase, status, code, sum)
		}
		return fmt.Sprintf("%s (%s): %s", phase, code, sum)
	}
	if phase != "" {
		return fmt.Sprintf("%s: %s", phase, sum)
	}
	if code != "" {
		return fmt.Sprintf("%s: %s", code, sum)
	}
	return sum
}

func formatWithoutSummary(code, phase, status string, get func(string) string) string {
	if code != "" && phase != "" {
		if status != "" {
			return fmt.Sprintf("%s %s (%s)", phase, status, code)
		}
		return fmt.Sprintf("%s (%s)", phase, code)
	}
	if code != "" {
		return code
	}
	if phase != "" {
		return phase + " failed"
	}
	for _, k := range []string{"schemaName", "type", "reason"} {
		if v := get(k); v != "" {
			return v
		}
	}
	return ""
}

func ReadLoop(path string, capBytes int) (string, error) {
	if capBytes <= 0 {
		capBytes = 50 * 1024
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	size := info.Size()
	if size <= int64(capBytes) {
		d, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(d), nil
	}
	res, err := readPaginated(path, capBytes, size)
	if err != nil {
		return "", err
	}
	if int64(len(res)) != size {
		retry, err2 := readPaginated(path, capBytes, size)
		if err2 != nil {
			return "", err2
		}
		if int64(len(retry)) != size {
			return "", fmt.Errorf("verify failed: expected %d got %d retry %d", size, len(res), len(retry))
		}
		return retry, nil
	}
	return res, nil
}

func readPaginated(path string, capBytes int, size int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var b strings.Builder
	b.Grow(int(size))
	buf := make([]byte, capBytes)
	off := int64(0)
	for off < size {
		n, err := f.ReadAt(buf, off)
		if n > 0 {
			b.Write(buf[:n])
			off += int64(n)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("read at %d: %w", off, err)
		}
		if n == 0 {
			break
		}
	}
	return b.String(), nil
}

// PersistPendingForCheckpoint saves synthesis + envelope for compaction recovery.
func PersistPendingForCheckpoint(change, synthesisMD string, envelope QuestionEnvelope) error {
	return SavePendingDualWrite(change, PendingQuestion{Schema: PendingSchema, Change: change, Envelope: envelope, SynthesisMD: synthesisMD})
}

// LoadPendingFallback loads pending and returns fallback markdown (FormatFallback).
func LoadPendingFallback(change string) (string, error) {
	pq, err := LoadOnCompaction(change)
	if err != nil {
		return "", err
	}
	md := PendingFallbackMD(pq)
	if md == "" {
		md = pq.SynthesisMD
	}
	return md, nil
}

func ReadLoopWithFunc(readFn func(offset, limit int) (string, error), expectedLen int) (string, error) {
	if readFn == nil {
		return "", fmt.Errorf("read function is nil")
	}
	if expectedLen <= 0 {
		expectedLen = 50 * 1024
	}
	const capBytes = 50 * 1024
	limit := capBytes
	if expectedLen < capBytes {
		limit = expectedLen
	}
	readOnce := func() (string, error) {
		var b strings.Builder
		b.Grow(expectedLen)
		off := 0
		for off < expectedLen {
			rem := expectedLen - off
			cur := limit
			if rem < cur {
				cur = rem
			}
			chunk, err := readFn(off, cur)
			if err != nil {
				return "", err
			}
			if chunk == "" {
				break
			}
			b.WriteString(chunk)
			off += len(chunk)
		}
		return b.String(), nil
	}
	res, err := readOnce()
	if err != nil {
		return "", err
	}
	if len(res) != expectedLen {
		retry, err2 := readOnce()
		if err2 != nil {
			return "", err2
		}
		if len(retry) != expectedLen {
			return "", fmt.Errorf("verify failed: expected %d got %d retry %d", expectedLen, len(res), len(retry))
		}
		return retry, nil
	}
	return res, nil
}
