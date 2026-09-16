package sdd

import (
	"strings"
	"testing"
)

const askCheckValidSynthesis = "## Sub-agent Result: slice 1b\n" +
	"**What was done:**\n" +
	"| Topic | Decision |\n|-------|----------|\n| a | b |\n" +
	"**Artifacts/Paths:** none\n" +
	"**Risks / Open Questions:** none\n" +
	"**Next Recommended:** none"

const (
	askCheckValidEnvelope = `{"questions":[{"question":"What next?","header":"Decision","options":[{"label":"Proceed","description":"Ship slice 1b; risk low because tests cover it."},{"label":"Stop","description":"Revert one commit; unblocks slice 2 later."}]}]}`
	askCheckThinEnvelope  = `{"questions":[{"question":"What next?","header":"Decision","options":[{"label":"Adjust","description":"Adjust two files; low risk."},{"label":"Yes, go ahead","description":"yes, go ahead"}]}]}`
	askCheckHeader17      = `{"questions":[{"question":"What next?","header":"12345678901234567","options":[{"label":"Proceed","description":"Ship slice 1b; risk low because tests cover it."},{"label":"Stop","description":"Revert one commit; unblocks slice 2 later."}]}]}`
	askCheckValueSignal   = `{"questions":[{"question":"What next?","header":"Decision","options":[{"label":"Option A","value":"proceed","description":"yes, go ahead"},{"label":"Option B","value":"stop","description":"do it later"}]}]}`
	askCheckNonCheckpoint = `{"questions":[{"question":"Which color?","header":"Color","options":[{"label":"Red","description":"yes"},{"label":"Blue","description":"no"}]}]}`
)

func TestCheckCheckpointAsk(t *testing.T) {
	const pinnedSynthesis = "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)"
	const noSynthesis = "no synthesis in this turn"

	tests := []struct {
		name     string
		req      AskCheckRequest
		child    bool
		wantCode int
		wantMsg  []string
		notMsg   []string
	}{
		{
			name:     "missing synthesis blocks with pinned message",
			req:      AskCheckRequest{Question: askCheckValidEnvelope, Markdown: noSynthesis},
			wantCode: AskCheckExitSynthesisRequired,
			wantMsg:  []string{"blocked(synthesis_required)", pinnedSynthesis},
		},
		{
			name:     "synthesis precedes substance",
			req:      AskCheckRequest{Question: askCheckThinEnvelope, Markdown: noSynthesis},
			wantCode: AskCheckExitSynthesisRequired,
			wantMsg:  []string{pinnedSynthesis},
			notMsg:   []string{"checkpoint_option_thin"},
		},
		{
			name:     "thin option blocks naming option and question",
			req:      AskCheckRequest{Question: askCheckThinEnvelope, Markdown: askCheckValidSynthesis},
			wantCode: AskCheckExitThinOption,
			wantMsg:  []string{"blocked(checkpoint_option_thin)", `"Yes, go ahead"`, "question 1", "decision context"},
		},
		{
			name:     "value-signaled checkpoint gets substance check",
			req:      AskCheckRequest{Question: askCheckValueSignal, Markdown: askCheckValidSynthesis},
			wantCode: AskCheckExitThinOption,
			wantMsg:  []string{"blocked(checkpoint_option_thin)", `"Option A"`},
		},
		{
			name:     "both preconditions pass silently",
			req:      AskCheckRequest{Question: askCheckValidEnvelope, Markdown: askCheckValidSynthesis},
			wantCode: AskCheckExitAllow,
		},
		{
			name:     "header over limit blocks envelope",
			req:      AskCheckRequest{Question: askCheckHeader17, Markdown: askCheckValidSynthesis},
			wantCode: AskCheckExitEnvelopeInvalid,
			wantMsg:  []string{"blocked(envelope_invalid)", "limit 16", "got 17"},
		},
		{
			name:     "subagent ownership blocks envelope",
			req:      AskCheckRequest{Question: askCheckValidEnvelope, Markdown: askCheckValidSynthesis},
			child:    true,
			wantCode: AskCheckExitEnvelopeInvalid,
			wantMsg:  []string{"blocked(envelope_invalid)", "ownership"},
		},
		{
			name:     "non-checkpoint terse option passes",
			req:      AskCheckRequest{Question: askCheckNonCheckpoint, Markdown: askCheckValidSynthesis},
			wantCode: AskCheckExitAllow,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.child {
				t.Setenv("PI_SUBAGENT_CHILD", "1")
			}
			res := CheckCheckpointAsk(tt.req)
			if res.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d (message: %q)", res.Code, tt.wantCode, res.Message)
			}
			for _, want := range tt.wantMsg {
				if !strings.Contains(res.Message, want) {
					t.Errorf("message %q missing %q", res.Message, want)
				}
			}
			for _, bad := range tt.notMsg {
				if strings.Contains(res.Message, bad) {
					t.Errorf("message %q must not contain %q", res.Message, bad)
				}
			}
			if tt.wantCode == AskCheckExitAllow && res.Message != "" {
				t.Errorf("allow must be silent, got message %q", res.Message)
			}
			if tt.wantCode != AskCheckExitAllow && !strings.HasPrefix(res.Message, "blocked(") {
				t.Errorf("message %q must start with a blocked( token", res.Message)
			}
		})
	}
}
