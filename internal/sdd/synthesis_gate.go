// synthesis_gate — enforces Post-Delegation Human Checkpoint (checkpoint + option-bearing).
// Ensures orchestrator emits synthesis markdown with ## Sub-agent Result + Artifacts/Paths + Risks + Next
// BEFORE calling ask_user_choice (Pi closed single-select, 2-4 ordered options) / ask_user_question (Pi open/free-text) / question (OpenCode).
// IsCheckpointAsk alone requires synthesis (REQ-DG-1); HasOptions alone never blocks.
// In Pi, strictly closed single-select (proceed/adjust/stop, continue/correct) must use ask_user_choice; open/free-text uses ask_user_question.
// Gate is tool-agnostic: ShouldBlock checks HasSynthesis + 120s window + IsCheckpointAsk/HasOptions, not tool name.
// ENFORCEMENT RETIRED (2026-09-04): blocking proved unfulfillable from the
// agent side (same-turn side-channel + body-text false positives) and is now
// a passthrough. Context-before-question is governed by the explicit agent
// contract in docs, not by code. Pure helpers below (HasSynthesis,
// IsCheckpointAsk, HasOptions, FormatFallback) stay as living documentation.
// Free-text without options is allowed; option-bearing (2-4 or HasOptions) and checkpoint asks require synthesis;
// synthesis after EVERY sub-agent (SDD or non-SDD) is enforced via orchestrator prompt (gentle-pi parity).
// See internal/assets/pi/biggz-synthesis-gate.js for JS counterpart that wraps ask_user_choice/ask_user_question/question.

package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

var synthesisMarkers = []string{
	"## Sub-agent Result",
	"**What was done:**",
	"**Artifacts/Paths:**",
	"**Risks / Open Questions:**",
	"**Next Recommended:**",
}

var currentTurnMarkdown = ""
var currentTurnTime time.Time

func SetCurrentTurnMarkdown(md string) {
	currentTurnMarkdown = md
	currentTurnTime = time.Now()
}

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

// ShouldBlock is RETIRED: always false (passthrough). Kept for signature
// compat; the agent contract (context as plain chat before the ask) replaces
// enforcement. See package note above.
// Only IsCheckpointAsk gates: HasOptions alone NEVER blocks, so free-text
// asks and Session Preflight option-asks always pass the gate.
// When IsCheckpointAsk, requires HasSynthesis in current turn within 120s window.
// Strict same-turn 120s: missing or expired MUST block. Go is canonical for JS mirror.
func ShouldBlock(question string, md string, now time.Time) bool {
	return false
}

func CheckSynthesisPrecondition(question string, md string) (bool, string) {
	return true, ""
}

// BlockedFallbackEnvelope is the REQ-DG-2 same-turn plain-chat payload.
// On block it carries the attempted context plus the full question text
// (via FormatFallback) so nothing is swallowed. Go is canonical for the
// JS mirror (blockedEnvelope in biggz-synthesis-gate.js).
type BlockedFallbackEnvelope struct {
	Block    bool
	Reason   string
	Context  string
	Fallback string
}

// BuildBlockedEnvelope is RETIRED: always Block:false. Kept for signature
// compat (callers treat the ask as allowed and proceed).
// When ShouldBlock is true it returns Block:true with Reason (same text
// as CheckSynthesisPrecondition), Context (attempted ask summary carrying
// the full question string), and Fallback (FormatFallback of env, prompt
// and options verbatim). ShouldBlock itself is unchanged (REQ-DG-1).
func BuildBlockedEnvelope(question string, md string, now time.Time, env QuestionEnvelope) BlockedFallbackEnvelope {
	return BlockedFallbackEnvelope{Block: false}
}

// ShouldBlockApplyAdmission is RETIRED: always false (passthrough). Write
// admission is governed by the human checkpoint in the workflow, not by code.
func ShouldBlockApplyAdmission(question string, md string, now time.Time) bool {
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
