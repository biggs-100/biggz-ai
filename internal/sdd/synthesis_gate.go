package sdd

import (
	"os"
	"strings"
	"time"
)

var synthesisMarkers = []string{
	"## Sub-agent Result:",
	"**What was done:**",
	"**Artifacts/Paths:**",
	"**Next Recommended:**",
}

var currentTurnMarkdown = ""
var currentTurnTime time.Time

func SetCurrentTurnMarkdown(md string) {
	currentTurnMarkdown = md
	currentTurnTime = time.Now()
}

func HasSynthesis(md string) bool {
	for _, m := range synthesisMarkers {
		if !strings.Contains(md, m) {
			return false
		}
	}
	return true
}

func HasSessionRecall(md string) bool {
	return strings.Contains(md, "## Session Recall")
}

func IsChildBypass() bool {
	return os.Getenv("PI_SUBAGENT_CHILD") == "1"
}

func IsCheckpointAsk(question string) bool {
	low := strings.ToLower(question)
	return strings.Contains(low, "proceed") || strings.Contains(low, "adjust") || strings.Contains(low, "stop") || strings.Contains(low, "continue") || strings.Contains(low, "correct")
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
