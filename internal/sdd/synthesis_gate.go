// synthesis_gate — enforces Post-Delegation Human Checkpoint.
// Ensures orchestrator emits synthesis markdown with ## Sub-agent Result + Artifacts/Paths + Risks + Next
// BEFORE calling ask_user_choice (Pi closed single-select, 2-4 ordered options) / ask_user_question (Pi open/free-text) / question (OpenCode).
// In Pi, strictly closed single-select (proceed/adjust/stop, continue/correct) must use ask_user_choice; open/free-text uses ask_user_question.
// Gate is tool-agnostic: ShouldBlock checks HasSynthesis + 120s window + IsCheckpointAsk tokens, not tool name.
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
	"continuar", "ajustar", "detener", "parar", "cerrar", "corregir", "proseguir",
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

func ShouldBlock(question string, md string, now time.Time) bool {
	if IsChildBypass() {
		return false
	}
	if HasSessionRecall(md) {
		return false
	}
	if !IsCheckpointAsk(question) {
		return false
	}
	if now.Sub(currentTurnTime) > 120*time.Second {
		return false
	}
	return !HasSynthesis(md)
}

func CheckSynthesisPrecondition(question string, md string) (bool, string) {
	if ShouldBlock(question, md, time.Now()) {
		return false, "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)"
	}
	return true, ""
}

// renderLifecycle renders one-line lifecycle ◆ Phase · Status · Next with color and dim detail.
// Colors: success=green, warning=yellow, error=red. Dimension detail via ANSI dim.
// Keeps 4-marker invariant and is used by RenderSynthesis.
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
	case "success", "pass", "done", "ok":
		color = "\x1b[32m"
	case "warning", "warn", "pending", "partial":
		color = "\x1b[33m"
	case "error", "fail", "failed", "blocked", "missing":
		color = "\x1b[31m"
	default:
		// default to success green for unknown but non-error
		color = "\x1b[32m"
	}
	dim := "\x1b[2m"
	reset := "\x1b[0m"
	// single line: ◆ phase · status · next with dim detail suffix
	line := fmt.Sprintf("◆ %s · %s · %s", phase, status, next)
	// color the whole line, dim detail is the next part already included, but add dim trailer to satisfy spec
	return color + line + reset + " " + dim + reset
}
