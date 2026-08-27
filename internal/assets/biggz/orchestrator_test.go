package biggz_test

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func readOrchestrator(t *testing.T) string {
	t.Helper()
	data, err := assets.FS.ReadFile("biggz/biggz-orchestrator.md")
	if err != nil {
		t.Fatalf("Read(biggz-orchestrator.md) error = %v", err)
	}
	return string(data)
}

func TestOrchestratorSynthesisTemplateInvariant(t *testing.T) {
	md := readOrchestrator(t)

	t.Run("contains copy-paste block with 4 markers", func(t *testing.T) {
		markers := []string{
			"## Sub-agent Result",
			"**Artifacts/Paths:**",
			"**Risks / Open Questions:**",
			"**Next Recommended:**",
		}
		for _, m := range markers {
			if !strings.Contains(md, m) {
				t.Errorf("biggz-orchestrator.md missing marker %q", m)
			}
		}
		if !strings.Contains(md, "## Sub-agent Result: {phase/agent}") {
			t.Errorf("biggz-orchestrator.md missing canonical example header %q", "## Sub-agent Result: {phase/agent}")
		}
	})

	t.Run("contains INVALID and will be blocked rule", func(t *testing.T) {
		if !strings.Contains(md, "INVALID and will be blocked") {
			t.Errorf("biggz-orchestrator.md missing INVALID and will be blocked rule")
		}
		// must mention that synthesis is separate markdown before tool call
		if !strings.Contains(md, "synthesis markdown is separate") {
			t.Errorf("biggz-orchestrator.md missing synthesis separation invariant")
		}
	})

	t.Run("contains 12x REMINDER convergence", func(t *testing.T) {
		const needle = "REMINDER: synthesis markdown is separate"
		n := strings.Count(md, needle)
		if n < 12 {
			t.Errorf("biggz-orchestrator.md expected >=12 REMINDER occurrences, got %d", n)
		}
	})

	t.Run("synthesis separate from tool param", func(t *testing.T) {
		// The checkpoint text must explicitly state markdown is NOT the tool param
		// and is emitted FIRST adjacent same turn.
		if !strings.Contains(md, "separate chat markdown emitted FIRST") {
			t.Errorf("biggz-orchestrator.md missing FIRST adjacent emission rule")
		}
		if !strings.Contains(md, "Do NOT put synthesis inside the tool's question param") {
			t.Errorf("biggz-orchestrator.md missing tool-param exclusion rule")
		}
	})
}

func TestOrchestratorSynthesisTemplateGuardsDrift(t *testing.T) {
	md := readOrchestrator(t)

	// Simulate drift detection: if any core marker is removed the test fails.
	// We verify count expectations so future edits that silently drop a marker are caught.
	if c := strings.Count(md, "## Sub-agent Result"); c < 2 {
		t.Errorf("expected at least 2 example blocks with ## Sub-agent Result, got %d", c)
	}
	if c := strings.Count(md, "**Artifacts/Paths:**"); c < 2 {
		t.Errorf("expected at least 2 Artifacts/Paths markers, got %d", c)
	}
	if !strings.Contains(md, "**Risks / Open Questions:**") {
		t.Errorf("missing Risks marker — drift not allowed")
	}
	if !strings.Contains(md, "**Next Recommended:**") {
		t.Errorf("missing Next Recommended marker — drift not allowed")
	}
}

func TestOrchestratorSessionRecallGateInvariant(t *testing.T) {
	md := readOrchestrator(t)
	t.Run("contains Session Recall hard gate", func(t *testing.T) {
		if !strings.Contains(md, "## Session Recall") {
			t.Errorf("biggz-orchestrator.md missing ## Session Recall hard gate marker")
		}
		if !strings.Contains(md, "Session Boot Recall") {
			t.Errorf("biggz-orchestrator.md missing Session Boot Recall gate title")
		}
		if !strings.Contains(md, "HARD GATE") {
			t.Errorf("biggz-orchestrator.md missing HARD GATE designation for Session Recall")
		}
	})
	t.Run("recall requires biggz_mem_context and biggz_mem_search", func(t *testing.T) {
		if !strings.Contains(md, "biggz_mem_context") {
			t.Errorf("biggz-orchestrator.md Session Recall must require biggz_mem_context")
		}
		if !strings.Contains(md, "biggz_mem_search") {
			t.Errorf("biggz-orchestrator.md Session Recall must require biggz_mem_search")
		}
		if !strings.Contains(md, "session_summary") {
			t.Errorf("biggz-orchestrator.md Session Recall must mention session_summary search")
		}
		if !strings.Contains(md, "sdd {project}") && !strings.Contains(md, "sdd ") {
			t.Errorf("biggz-orchestrator.md Session Recall must mention sdd {project} search")
		}
	})
	t.Run("recall fallback to sdd-status", func(t *testing.T) {
		if !strings.Contains(md, "sdd-status") {
			t.Errorf("biggz-orchestrator.md Session Recall must document fallback to sdd-status")
		}
		if !strings.Contains(md, "--json --instructions") {
			t.Errorf("biggz-orchestrator.md Session Recall fallback must be sdd-status --json --instructions")
		}
	})
	t.Run("recall REMINDER separation", func(t *testing.T) {
		if !strings.Contains(md, "Session Recall markdown is separate") {
			t.Errorf("biggz-orchestrator.md Session Recall must have REMINDER about separate markdown")
		}
		if strings.Count(md, "## Session Recall") < 1 {
			t.Errorf("expected at least 1 ## Session Recall block")
		}
	})
}
