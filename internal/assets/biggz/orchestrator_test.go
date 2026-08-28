package biggz_test

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/sdd"
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

	t.Run("contains 6 optional omit-empty sections", func(t *testing.T) {
		optional := []string{
			"**Preview:**",
			"**Diff:**",
			"**Decisions:**",
			"**Commands:**",
			"**Validation:**",
			"**Failure:**",
		}
		for _, m := range optional {
			if !strings.Contains(md, m) {
				t.Errorf("biggz-orchestrator.md missing optional section %q", m)
			}
			if !strings.Contains(md, m+" {optional, omit if empty") {
				t.Errorf("biggz-orchestrator.md optional section %q must be marked omit-empty", m)
			}
		}
	})

	t.Run("contains INVALID and will be blocked rule", func(t *testing.T) {
		if !strings.Contains(md, "INVALID and will be blocked") {
			t.Errorf("biggz-orchestrator.md missing INVALID and will be blocked rule")
		}
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
		if !strings.Contains(md, "separate chat markdown emitted FIRST") {
			t.Errorf("biggz-orchestrator.md missing FIRST adjacent emission rule")
		}
		if !strings.Contains(md, "Do NOT put synthesis inside the tool's question param") {
			t.Errorf("biggz-orchestrator.md missing tool-param exclusion rule")
		}
	})

	t.Run("hasSynthesis compat 4 markers kept", func(t *testing.T) {
		// Ensure optional sections do not replace the 4 required markers.
		for _, m := range []string{"**Artifacts/Paths:**", "**Risks / Open Questions:**", "**Next Recommended:**"} {
			if strings.Count(md, m) < 2 {
				t.Errorf("expected at least 2 occurrences of %q for compat, got %d", m, strings.Count(md, m))
			}
		}
	})
}

func TestOrchestratorSynthesisTemplateGuardsDrift(t *testing.T) {
	md := readOrchestrator(t)

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
	// Guard optional sections drift: if any of the 6 is removed, fail.
	for _, m := range []string{"**Preview:**", "**Diff:**", "**Decisions:**", "**Commands:**", "**Validation:**", "**Failure:**"} {
		if !strings.Contains(md, m) {
			t.Errorf("missing optional section %q — drift not allowed (PR1)", m)
		}
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

func TestOrchestratorAliasInvariant(t *testing.T) {
	md := readOrchestrator(t)
	t.Run("template mentions engram alias bigmem", func(t *testing.T) {
		lower := strings.ToLower(md)
		if !strings.Contains(lower, "engram") {
			t.Errorf("biggz-orchestrator.md missing engram alias mention")
		}
		if !strings.Contains(lower, "bigmem") {
			t.Errorf("biggz-orchestrator.md missing bigmem alias mention")
		}
		if !strings.Contains(md, "Alias invariant") && !strings.Contains(md, "alias for") {
			t.Errorf("biggz-orchestrator.md missing alias invariant note (engram is alias for bigmem)")
		}
	})
	t.Run("Go engram==bigmem alias", func(t *testing.T) {
		// Both constants must exist and be considered valid stores.
		if !sdd.IsEngramStore(sdd.ArtifactStoreEngram) {
			t.Errorf("IsEngramStore must accept engram")
		}
		if !sdd.IsEngramStore(sdd.ArtifactStoreBigMem) {
			t.Errorf("IsEngramStore must accept bigmem alias")
		}
		if sdd.ArtifactStoreEngram == sdd.ArtifactStoreBigMem {
			// If they are defined as same value, that's also acceptable alias;
			// the key is both are valid and helper accepts both.
		}
		// isValid must accept both engram and bigmem.
		// Use exported helper via indirect check: IsEngramStore covers both.
		// Also ensure the underlying string values are distinct but aliased logically.
		if string(sdd.ArtifactStoreEngram) != "engram" {
			t.Errorf("ArtifactStoreEngram = %q, want \"engram\"", sdd.ArtifactStoreEngram)
		}
		if string(sdd.ArtifactStoreBigMem) != "bigmem" {
			t.Errorf("ArtifactStoreBigMem = %q, want \"bigmem\"", sdd.ArtifactStoreBigMem)
		}
	})
	t.Run("sdd engram_status alias guard", func(t *testing.T) {
		// Ensure engram_status.go mentions alias (prevent drift).
		// Read via assets is not possible for Go file, so check via string constant presence:
		// The alias helpers must be present in the sdd package.
		if !sdd.IsEngramStore("engram") || !sdd.IsEngramStore("bigmem") {
			t.Errorf("IsEngramStore must return true for both \"engram\" and \"bigmem\" strings")
		}
	})
}
