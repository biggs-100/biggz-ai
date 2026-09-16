package sdd

import (
	"encoding/json"
	"errors"
	"strings"
)

// Exit codes of `biggz sdd-ask-check` (design D2/D5). 0 allows; 1-3 are
// decided blocks and are the only codes whose message carries a blocked(
// token; 4 (indeterminate) is CLI-level: usage/input errors must never print
// a blocked( token, so a caller can degrade-open with a visible notice.
const (
	AskCheckExitAllow             = 0
	AskCheckExitSynthesisRequired = 1
	AskCheckExitEnvelopeInvalid   = 2
	AskCheckExitThinOption        = 3
)

// AskCheckRequest is the stdin payload of `biggz sdd-ask-check`: the raw ask
// params/envelope plus the caller-attested current-turn markdown (design D3).
type AskCheckRequest struct {
	Question string `json:"question"`
	Markdown string `json:"markdown"`
}

// AskCheckResult maps 1:1 to the process exit code. Message is the
// agent-facing line for stdout; empty when the ask is allowed.
type AskCheckResult struct {
	Code    int
	Message string
}

// CheckCheckpointAsk runs the ordered decision pipeline of the checkpoint-ask
// contract: synthesis precondition (1) -> envelope validation incl. substance
// (2|3) -> allow (0). It seeds the one-shot current-turn window from the
// request: the caller attests Markdown is the live turn (design D3).
func CheckCheckpointAsk(req AskCheckRequest) AskCheckResult {
	SetCurrentTurnMarkdown(req.Markdown)
	if ok, reason := CheckSynthesisPrecondition(req.Question, req.Markdown); !ok {
		return AskCheckResult{
			Code:    AskCheckExitSynthesisRequired,
			Message: "blocked(synthesis_required): " + reason + "; emit the block first, adjacent, same turn, then retry",
		}
	}
	var env QuestionEnvelope
	if err := json.Unmarshal([]byte(req.Question), &env); err != nil {
		// Legacy raw non-envelope checkpoint strings carry no options to validate.
		return AskCheckResult{Code: AskCheckExitAllow}
	}
	if err := ValidateQuestionEnvelope(env); err != nil {
		return askEnvelopeFailure(err)
	}
	// Second substance call site (design D5): value/id/name/title-signaled
	// checkpoints are not label-signaled, so ValidateQuestionEnvelope skipped
	// the substance pass; run it here for them.
	if IsCheckpointAsk(req.Question) && !IsCheckpointEnvelope(env) {
		if err := ValidateCheckpointSubstance(env); err != nil {
			return askEnvelopeFailure(err)
		}
	}
	return AskCheckResult{Code: AskCheckExitAllow}
}

// askEnvelopeFailure renders envelope errors as actionable blocked( lines,
// stripping the internal isError:true prefix and mapping the thin-option
// sentinel to exit 3 (design D5).
func askEnvelopeFailure(err error) AskCheckResult {
	msg := strings.TrimPrefix(err.Error(), "isError:true ")
	if errors.Is(err, ErrThinCheckpointOption) {
		msg = strings.TrimPrefix(msg, ErrThinCheckpointOption.Error()+": ")
		return AskCheckResult{Code: AskCheckExitThinOption, Message: "blocked(checkpoint_option_thin): " + msg}
	}
	return AskCheckResult{Code: AskCheckExitEnvelopeInvalid, Message: "blocked(envelope_invalid): " + msg}
}
