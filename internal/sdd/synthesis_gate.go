// synthesis_gate — enforces Post-Delegation Human Checkpoint (checkpoint + option-bearing).
// Ensures orchestrator emits synthesis markdown with ## Sub-agent Result + Artifacts/Paths + Risks + Next
// BEFORE calling ask_user_choice (Pi closed single-select, 2-4 ordered options) / ask_user_question (Pi open/free-text) / question (OpenCode).
// IsCheckpointAsk + HasOptions (2-4 options or options-bearing envelope) both require synthesis.
// In Pi, strictly closed single-select (proceed/adjust/stop, continue/correct) must use ask_user_choice; open/free-text uses ask_user_question.
// Gate is tool-agnostic: ShouldBlock checks HasSynthesis + 120s window + IsCheckpointAsk/HasOptions, not tool name.
// Free-text without options is allowed; option-bearing (2-4 or HasOptions) and checkpoint asks require synthesis;
// synthesis after EVERY sub-agent (SDD or non-SDD) is enforced via orchestrator prompt (gentle-pi parity).
// See internal/assets/pi/biggz-synthesis-gate.js for JS counterpart that wraps ask_user_choice/ask_user_question/question.

package sdd

import (
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

// checkpointTokens bilingual: English + Spanish. Gate scans whole envelope JSON string case-insensitive
// so localized labels like "Continuar (Recomendado)" are detected. Keep English for backward compat.
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
	low := strings.ToLower(question)
	for _, tok := range checkpointTokens {
		if strings.Contains(low, tok) {
			return true
		}
	}
	return false
}

// ShouldBlock blocks checkpoint and option-bearing asks without synthesis.
// Free-text asks without options (IsCheckpointAsk==false && HasOptions==false) are allowed
// by the gate; synthesis after EVERY delegated sub-agent is still required via orchestrator prompt.
// When HasOptions || IsCheckpointAsk, requires HasSynthesis in current turn within 120s window.
// Strict same-turn 120s: missing or expired MUST block. Go is canonical for JS mirror.
func ShouldBlock(question string, md string, now time.Time) bool {
	if IsChildBypass() {
		return false
	}
	if HasSessionRecall(md) {
		return false
	}
	if !IsCheckpointAsk(question) && !HasOptions(question) {
		return false
	}
	if !HasSynthesis(md) {
		return true
	}
	if now.Sub(currentTurnTime) > 120*time.Second {
		return true
	}
	return false
}

func CheckSynthesisPrecondition(question string, md string) (bool, string) {
	if ShouldBlock(question, md, time.Now()) {
		return false, "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)"
	}
	return true, ""
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
