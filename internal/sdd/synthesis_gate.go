// synthesis_gate — advise library for Post-Delegation Human Checkpoint.
//
// ENFORCEMENT RETIRED (2026-09-04): blocking proved unfulfillable from the
// agent side (same-turn side-channel + body-text false positives) and is now
// removed. Context-before-question is governed by the explicit agent
// contract in docs (Ask contract, no blocking gate), not by code.
//
// This file keeps only pure advise helpers: HasSynthesis, IsCheckpointAsk,
// HasOptions, HasSessionRecall, IsChildBypass, plus renderLifecycle used by
// RenderSynthesis. No blocking, no turn state, no envelopes.
// See internal/assets/pi/biggz-synthesis-gate.js for JS counterpart (advise-only).

package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func HasSynthesis(md string) bool {
	required := []string{
		"## Sub-agent Result",
		"**Artifacts/Paths:**",
		"**Risks / Open Questions:**",
		"**Next Recommended:**",
	}
	for _, m := range required {
		if !strings.Contains(md, m) {
			return false
		}
	}
	// WhatDone can be prose marker or table header - table replaces prose per spec
	if !strings.Contains(md, "**What was done:**") && !strings.Contains(md, "| Topic | Decision |") {
		return false
	}
	return true
}

func HasSessionRecall(md string) bool {
	return strings.Contains(md, "## Session Recall")
}

func IsChildBypass() bool {
	return os.Getenv("PI_SUBAGENT_CHILD") == "1"
}

// checkpointTokens bilingual: English + Spanish. Detection scans ONLY option
// labels/values (questions[].options[] + top-level options[], parity with
// biggz-synthesis-gate.js isCheckpointAsk). Question BODY text is never a
// checkpoint signal: verbs like "continuar" in "¿...para continuar con X?"
// are content, not choices. Raw non-envelope strings keep legacy
// whole-string scan for backward compat.
var checkpointTokens = []string{
	"proceed", "adjust", "stop", "continue", "correct",
	"continuar", "ajustar", "detener", "parar", "cerrar", "corregir", "proseguir", "proceder", "procede",
}

// HasOptions detects option-bearing ask envelopes via case-insensitive substring heuristic.
// True when JSON envelope contains "options" (e.g. ask_user_question/question with options array).
// Advise-only: HasOptions alone never blocks (REQ-DG-1).
func HasOptions(question string) bool {
	return strings.Contains(strings.ToLower(question), "\"options\"")
}

func IsCheckpointAsk(question string) bool {
	if isQuestionEnvelope(question) {
		return envelopeHasCheckpointLabel(question)
	}
	return containsCheckpointToken(question)
}

func containsCheckpointToken(s string) bool {
	low := strings.ToLower(s)
	for _, tok := range checkpointTokens {
		if strings.Contains(low, tok) {
			return true
		}
	}
	return false
}

// checkpointOption mirrors one option object in the question-tool envelope.
type checkpointOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Name  string `json:"name"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

// checkpointEnvelope mirrors the structured ask envelope (questions with
// options, plus top-level options fallback).
type checkpointEnvelope struct {
	Questions []struct {
		Options []json.RawMessage `json:"options"`
	} `json:"questions"`
	Options []json.RawMessage `json:"options"`
}

func isQuestionEnvelope(s string) bool {
	var env checkpointEnvelope
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		return false
	}
	return len(env.Questions) > 0 || len(env.Options) > 0
}

func optionHasCheckpointToken(raw json.RawMessage) bool {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return containsCheckpointToken(asString)
	}
	var opt checkpointOption
	if err := json.Unmarshal(raw, &opt); err != nil {
		return false
	}
	for _, field := range []string{opt.Label, opt.Value, opt.Name, opt.ID, opt.Title} {
		if field != "" && containsCheckpointToken(field) {
			return true
		}
	}
	return false
}

func envelopeHasCheckpointLabel(s string) bool {
	var env checkpointEnvelope
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		return containsCheckpointToken(s)
	}
	for _, q := range env.Questions {
		for _, raw := range q.Options {
			if optionHasCheckpointToken(raw) {
				return true
			}
		}
	}
	for _, raw := range env.Options {
		if optionHasCheckpointToken(raw) {
			return true
		}
	}
	return false
}

// renderLifecycle renders one-line lifecycle ◆ Phase · Status · Next with color.
// Colors: success/éxito=green, warning/atención=yellow, error=red.
// Keeps 4-marker invariant and is used by RenderSynthesis. Single line, no empty dim trailer.
func renderLifecycle(phase, status, next string) string {
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = "phase"
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "success"
	}
	next = strings.TrimSpace(next)
	if next == "" {
		next = "none"
	}
	var color string
	low := strings.ToLower(status)
	switch low {
	case "success", "pass", "done", "ok", "éxito", "exito":
		color = "[32m"
	case "warning", "warn", "pending", "partial", "atención", "atencion":
		color = "[33m"
	case "error", "fail", "failed", "blocked", "missing":
		color = "[31m"
	default:
		// default to success green for unknown but non-error
		color = "[32m"
	}
	reset := "[0m"
	line := fmt.Sprintf("◆ %s · %s · %s", phase, status, next)
	return color + line + reset
}
