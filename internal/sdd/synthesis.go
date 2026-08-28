package sdd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type SubAgentResult struct {
	Phase          string
	WhatDone       string
	ArtifactsPaths string
	Risks          string
	NextRecommended string
	Preview        string
	Diff           string
	Decisions      string
	Commands       string
	Validation     string
	Failure        string
}

func RenderSynthesis(r SubAgentResult) string {
	phase := strings.TrimSpace(r.Phase)
	if phase == "" {
		phase = "phase/agent"
	}
	what := strings.TrimSpace(r.WhatDone)
	if what == "" {
		what = "None"
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
	var b strings.Builder
	b.WriteString("## Sub-agent Result: " + phase + "\n")
	b.WriteString("**What was done:** " + what + "\n")
	b.WriteString("**Artifacts/Paths:** " + arts + "\n")
	b.WriteString("**Risks / Open Questions:** " + risks + "\n")
	b.WriteString("**Next Recommended:** " + next + "\n")
	if v := strings.TrimSpace(r.Preview); v != "" {
		b.WriteString("**Preview:** " + v + "\n")
	}
	if v := strings.TrimSpace(r.Diff); v != "" {
		b.WriteString("**Diff:** " + v + "\n")
	}
	if v := strings.TrimSpace(r.Decisions); v != "" {
		b.WriteString("**Decisions:** " + v + "\n")
	}
	if v := strings.TrimSpace(r.Commands); v != "" {
		b.WriteString("**Commands:** " + v + "\n")
	}
	if v := strings.TrimSpace(r.Validation); v != "" {
		b.WriteString("**Validation:** " + v + "\n")
	}
	if v := strings.TrimSpace(r.Failure); v != "" {
		human := humanizeFailure(v)
		if human == "" {
			human = v
		}
		b.WriteString("**Failure:** " + human + "\n")
	}
	return b.String()
}

func humanizeFailure(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.Index(s, "{"); i > 0 && strings.Contains(strings.TrimSpace(s[:i]), "FAILURE") {
		s = strings.TrimSpace(s[i:])
	}
	if !strings.HasPrefix(s, "{") {
		s = strings.ReplaceAll(s, "\n", " ")
		return strings.Join(strings.Fields(s), " ")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		s = strings.ReplaceAll(s, "\n", " ")
		if len(s) > 200 {
			s = s[:200] + "…"
		}
		return "malformed failure payload: " + strings.Join(strings.Fields(s), " ")
	}
	get := func(k string) string {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
		return ""
	}
	sum := get("summary")
	if sum == "" { sum = get("message") }
	if sum == "" { sum = get("error") }
	if sum == "" { sum = get("diagnosis") }
	code, phase, status := get("code"), get("phase"), get("status")
	if sum != "" {
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
	return "failure"
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
