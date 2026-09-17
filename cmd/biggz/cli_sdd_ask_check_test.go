package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const cliAskValidSynthesis = "## Sub-agent Result: slice 1b\n" +
	"**What was done:**\n" +
	"| Topic | Decision |\n|-------|----------|\n| a | b |\n" +
	"**Artifacts/Paths:** none\n" +
	"**Risks / Open Questions:** none\n" +
	"**Next Recommended:** none"

const (
	cliAskValidEnvelope = `{"questions":[{"question":"What next?","header":"Decision","options":[{"label":"Proceed","description":"Ship slice 1b; risk low because tests cover it."},{"label":"Stop","description":"Revert one commit; unblocks slice 2 later."}]}]}`
	cliAskThinEnvelope  = `{"questions":[{"question":"What next?","header":"Decision","options":[{"label":"Adjust","description":"Adjust two files; low risk."},{"label":"Yes, go ahead","description":"yes, go ahead"}]}]}`
	cliAskHeader17      = `{"questions":[{"question":"What next?","header":"12345678901234567","options":[{"label":"Proceed","description":"Ship slice 1b; risk low because tests cover it."},{"label":"Stop","description":"Revert one commit; unblocks slice 2 later."}]}]}`
	cliAskNonCheckpoint = `{"questions":[{"question":"Which color?","header":"Color","options":[{"label":"Red","description":"yes"},{"label":"Blue","description":"no"}]}]}`
)

func askCheckPayload(question, markdown string) string {
	return `{"question":` + quoteJSON(question) + `,"markdown":` + quoteJSON(markdown) + `}`
}

func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func runAskCheck(args []string, stdin string) (int, string, string) {
	var stdout, stderr strings.Builder
	code := runSddAskCheck(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestSddAskCheckExitTable(t *testing.T) {
	const pinnedSynthesis = "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)"

	tests := []struct {
		name     string
		stdin    string
		child    bool
		wantCode int
		wantOut  []string
		wantErr  []string
	}{
		{
			name:     "missing synthesis blocks with pinned message",
			stdin:    askCheckPayload(cliAskValidEnvelope, "no synthesis this turn"),
			wantCode: 1,
			wantOut:  []string{"blocked(synthesis_required)", pinnedSynthesis},
		},
		{
			name:     "thin option blocks naming option and question",
			stdin:    askCheckPayload(cliAskThinEnvelope, cliAskValidSynthesis),
			wantCode: 3,
			wantOut:  []string{"blocked(checkpoint_option_thin)", `"Yes, go ahead"`, "question 1"},
		},
		{
			name:     "both preconditions pass with silent stdout",
			stdin:    askCheckPayload(cliAskValidEnvelope, cliAskValidSynthesis),
			wantCode: 0,
		},
		{
			name:     "header over limit names the 16 limit",
			stdin:    askCheckPayload(cliAskHeader17, cliAskValidSynthesis),
			wantCode: 2,
			wantOut:  []string{"blocked(envelope_invalid)", "limit 16", "got 17"},
		},
		{
			name:     "subagent ownership blocks envelope",
			stdin:    askCheckPayload(cliAskValidEnvelope, cliAskValidSynthesis),
			child:    true,
			wantCode: 2,
			wantOut:  []string{"blocked(envelope_invalid)", "ownership"},
		},
		{
			name:     "non-checkpoint terse option passes",
			stdin:    askCheckPayload(cliAskNonCheckpoint, cliAskValidSynthesis),
			wantCode: 0,
		},
		{
			name:     "malformed stdin is indeterminate with usage",
			stdin:    `{not json`,
			wantCode: 4,
			wantErr:  []string{"error: cannot parse stdin payload", "Usage: biggz sdd-ask-check"},
		},
		{
			name:     "non-JSON question with valid synthesis is indeterminate on stderr",
			stdin:    askCheckPayload("Test?", cliAskValidSynthesis),
			wantCode: 4,
			wantErr:  []string{"indeterminate:", "cannot parse the question payload as JSON"},
		},
		{
			name:     "JSON question without questions or options is indeterminate",
			stdin:    askCheckPayload(`{"foo":1}`, cliAskValidSynthesis),
			wantCode: 4,
			wantErr:  []string{"indeterminate:", "nothing to validate"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.child {
				t.Setenv("PI_SUBAGENT_CHILD", "1")
			}
			code, stdout, stderr := runAskCheck(nil, tt.stdin)
			if code != tt.wantCode {
				t.Fatalf("code = %d, want %d (stdout: %q, stderr: %q)", code, tt.wantCode, stdout, stderr)
			}
			for _, want := range tt.wantOut {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout %q missing %q", stdout, want)
				}
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr %q missing %q", stderr, want)
				}
			}
			if tt.wantCode == 0 && strings.TrimSpace(stdout) != "" {
				t.Errorf("allow must be silent on stdout, got %q", stdout)
			}
			if code == 4 && strings.Contains(stdout+stderr, "blocked(") {
				t.Errorf("indeterminate output must never print a blocked( token: stdout=%q stderr=%q", stdout, stderr)
			}
			if code == 4 && strings.TrimSpace(stdout) != "" {
				t.Errorf("indeterminate must write only to stderr, got stdout %q", stdout)
			}
		})
	}
}
