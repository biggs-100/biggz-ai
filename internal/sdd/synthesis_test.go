package sdd

import (
	"strings"
	"testing"
)

func TestSynthesis(t *testing.T) {
	t.Run("humanized JSON", func(t *testing.T) {
		j := `{"code":"sdd_task_result_malformed","phase":"sdd-apply","summary":"missing artifact","status":"blocked"}`
		out := RenderSynthesis(SubAgentResult{Failure: j})
		if !strings.Contains(out, "**Failure:**") {
			t.Fatalf("missing Failure marker")
		}
		if strings.Contains(out, `{"code"`) {
			t.Errorf("raw JSON leaked: %q", out)
		}
		if !strings.Contains(out, "missing artifact") || !strings.Contains(out, "sdd-apply") {
			t.Errorf("summary/phase missing: %q", out)
		}
	})
	t.Run("prefix BIGGZ", func(t *testing.T) {
		raw := `BIGGZ_AI_SDD_FAILURE {"schemaName":"biggz-ai.sdd-task-result-failure/v1","code":"sdd_task_result_empty","phase":"sdd-apply","summary":"no valid result","status":"blocked"}`
		out := RenderSynthesis(SubAgentResult{Failure: raw})
		if strings.Contains(out, `{"schemaName"`) {
			t.Errorf("raw JSON leaked: %q", out)
		}
		if !strings.Contains(out, "no valid result") {
			t.Errorf("summary missing: %q", out)
		}
	})
	t.Run("plain and empty", func(t *testing.T) {
		out := RenderSynthesis(SubAgentResult{Failure: "something failed"})
		if !strings.Contains(out, "something failed") {
			t.Errorf("plain failure missing: %q", out)
		}
		out2 := RenderSynthesis(SubAgentResult{Failure: "   "})
		if strings.Contains(out2, "**Failure:**") {
			t.Errorf("empty should omit Failure: %q", out2)
		}
	})
}
