package sdd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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

func RenderSynthesis(r SubAgentResult) string {
	phase := strings.TrimSpace(r.Phase)
	if phase == "" {
		phase = "phase/agent"
	}
	arts := strings.TrimSpace(r.ArtifactsPaths)
	if arts == "" {
		arts = "None"
	}
	risks := strings.TrimSpace(r.Risks)
	if risks == "" {
		risks = "None"
	}
	next := strings.TrimSpace(r.NextRecommended)
	if next == "" {
		next = "None"
	}
	// derive lifecycle status
	status := deriveLifecycleStatus(r)
	lifecycle := renderLifecycle(phase, status, next)

	var b strings.Builder
	b.WriteString("## Sub-agent Result: " + sanitizeForWidth(phase, 80) + "\n")
	// What was done as table + checklist
	b.WriteString("**What was done:**\n")
	rows, checklist := parseWhatDoneRows(r.WhatDone)
	// Ensure at least header present
	if len(rows) == 0 {
		rows = [][]string{{"None", "None"}}
	}
	// Render table chunked
	tableMD := renderTable(rows, 80)
	b.WriteString(tableMD)
	if len(checklist) > 0 {
		for _, item := range checklist {
			// sanitize checklist item but keep prefix
			sanitized := sanitizePlain(item)
			// per-item truncate to width 80
			sanitized = truncateToWidth(sanitized, 80)
			b.WriteString(sanitized + "\n")
		}
	}
	// lifecycle one-line
	b.WriteString(lifecycle + "\n")
	b.WriteString("**Artifacts/Paths:** " + sanitizePlain(arts) + "\n")
	b.WriteString("**Risks / Open Questions:** " + sanitizePlain(risks) + "\n")
	b.WriteString("**Next Recommended:** " + sanitizePlain(next) + "\n")
	// Preview sanitized 300
	previewRaw := strings.TrimSpace(r.Preview)
	if previewRaw == "" {
		b.WriteString("**Preview:** None\n")
	} else {
		b.WriteString("**Preview:** " + formatPreview(previewRaw) + "\n")
	}
	diffRaw := strings.TrimSpace(r.Diff)
	if diffRaw == "" {
		b.WriteString("**Diff:** None\n")
	} else {
		b.WriteString("**Diff:** " + formatDiff(diffRaw) + "\n")
	}
	if v := strings.TrimSpace(r.Decisions); v != "" {
		b.WriteString("**Decisions:** " + sanitizeForWidth(v, 80) + "\n")
	}
	if v := strings.TrimSpace(r.Commands); v != "" {
		b.WriteString("**Commands:** " + sanitizeForWidth(v, 80) + "\n")
	}
	validation := strings.TrimSpace(r.Validation)
	if validation == "" {
		validation = "None"
	} else {
		validation = sanitizeForWidth(validation, 80)
	}
	b.WriteString("**Validation:** " + validation + "\n")
	if v := strings.TrimSpace(r.Failure); v != "" {
		human := humanizeFailure(v)
		if human == "" {
			human = v
		}
		b.WriteString("**Failure:** " + sanitizeForWidth(human, 80) + "\n")
	}
	return b.String()
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

// sanitizePlain replaces tabs, strips OSC/ANSI/controls and normalizes spaces
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

func parseWhatDoneRows(s string) ([][]string, []string) {
	s = strings.TrimSpace(s)
	if s == "" || s == "None" {
		return nil, nil
	}
	// First extract checklist lines
	var rows [][]string
	var checklist []string
	lines := strings.Split(s, "\n")
	// If no newline but contains semicolon, split there for rows
	if len(lines) == 1 && strings.Contains(s, ";") {
		parts := strings.Split(s, ";")
		// treat each part as potential row
		var newLines []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				newLines = append(newLines, p)
			}
		}
		if len(newLines) > 1 {
			lines = newLines
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [ ]") || strings.HasPrefix(line, "- [X]") {
			checklist = append(checklist, line)
			continue
		}
		// also handle lines that contain checklist after table delimiter? skip
		// parse topic/decision
		topic, decision := splitTopicDecision(line)
		if topic == "" && decision == "" {
			continue
		}
		// sanitize cells before truncation budget
		topic = sanitizePlain(topic)
		decision = sanitizePlain(decision)
		// per-cell truncate to conservative budget 17 for narrow 40
		// budget = (40-6)/2 =17, use 17 to guarantee VisibleWidth <= budget on narrow
		const cellBudget = 17
		topic = truncateToWidth(topic, cellBudget)
		decision = truncateToWidth(decision, cellBudget)
		rows = append(rows, []string{topic, decision})
	}
	// fallback: if no rows but original had content without delimiters, split by comma
	if len(rows) == 0 && s != "" {
		if strings.Contains(s, ",") {
			parts := strings.Split(s, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				topic, decision := splitTopicDecision(p)
				if topic == "" {
					topic = p
				}
				topic = sanitizePlain(topic)
				decision = sanitizePlain(decision)
				const cellBudget = 17
				topic = truncateToWidth(topic, cellBudget)
				decision = truncateToWidth(decision, cellBudget)
				rows = append(rows, []string{topic, decision})
			}
		} else {
			// single entry as topic
			topic := sanitizePlain(s)
			const cellBudget = 17
			topic = truncateToWidth(topic, cellBudget)
			rows = append(rows, []string{topic, ""})
		}
	}
	return rows, checklist
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

func renderTable(rows [][]string, width int) string {
	if width <= 0 {
		width = 80
	}
	// chunk
	chunks := chunkTable(rows, 7)
	var b strings.Builder
	for idx, chunk := range chunks {
		b.WriteString("| Topic | Decision |\n")
		b.WriteString("|-------|----------|\n")
		for _, r := range chunk {
			topic := r[0]
			decision := ""
			if len(r) > 1 {
				decision = r[1]
			}
			// ensure per-cell visible width <= budget already done in parse, but re-check with width-based budget
			budget := (width - 6) / 2
			if budget < 5 {
				budget = 5
			}
			// if our conservative 17 is larger than budget for narrow, need to re-truncate to budget
			if visibleWidth(topic) > budget {
				topic = truncateToWidth(topic, budget)
			}
			if visibleWidth(decision) > budget {
				decision = truncateToWidth(decision, budget)
			}
			b.WriteString("| " + topic + " | " + decision + " |\n")
		}
		if idx < len(chunks)-1 {
			remaining := len(rows) - (idx+1)*7
			if remaining > 0 {
				b.WriteString(fmt.Sprintf("… +%d more\n", remaining))
			}
		}
	}
	return b.String()
}


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
    var b strings.Builder
    cur, i, stop := 0, 0, len(s)
    for i < len(s) {
        if s[i] == '' && i+1 < len(s) && s[i+1] == '[' {
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
        if s[idx] == '' && idx+1 < len(s) && s[idx+1] == '[' {
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
