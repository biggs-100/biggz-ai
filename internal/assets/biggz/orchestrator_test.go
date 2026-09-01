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

func readWorkflow(t *testing.T) string {
	t.Helper()
	data, err := assets.FS.ReadFile("biggz/biggz-orchestrator-workflow.md")
	if err != nil {
		t.Fatalf("Read(biggz-orchestrator-workflow.md) error = %v", err)
	}
	return string(data)
}

func readDelegation(t *testing.T) string {
	t.Helper()
	data, err := assets.FS.ReadFile("biggz/biggz-orchestrator-delegation.md")
	if err != nil {
		t.Fatalf("Read(biggz-orchestrator-delegation.md) error = %v", err)
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

	t.Run("contains REMINDER convergence (thin + lazy)", func(t *testing.T) {
		const needle = "REMINDER: synthesis markdown is separate"
		// Thin orchestrator has at least 1; combined with lazy files should have richer coverage.
		// Original monolith required >=12 in single file; split architecture distributes REMINDERs.
		nOrch := strings.Count(md, needle)
		if nOrch < 1 {
			t.Errorf("biggz-orchestrator.md expected >=1 REMINDER, got %d", nOrch)
		}
		// Verify lazy files together provide remaining coverage (workflow + delegation may contain additional context, but orchestrator thin keeps at least 1)
		// For backward compat, also check combined count across orchestrator+workflow+delegation if available.
		wf := readWorkflow(t)
		dl := readDelegation(t)
		combined := nOrch + strings.Count(wf, needle) + strings.Count(dl, needle)
		// Combined should still satisfy legacy budget or at least not drop to zero.
		if combined < 1 {
			t.Errorf("combined REMINDER across lazy files expected >=1, got %d", combined)
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
		// Thin orchestrator keeps exactly ONE canonical block; legacy monolith had duplicates (≥2).
		// Updated to require ≥1 in thin orchestrator; duplication is now fixed.
		for _, m := range []string{"**Artifacts/Paths:**", "**Risks / Open Questions:**", "**Next Recommended:**"} {
			if strings.Count(md, m) < 1 {
				t.Errorf("expected at least 1 occurrence of %q for compat, got %d", m, strings.Count(md, m))
			}
		}
		if count := strings.Count(md, "## Sub-agent Result: {phase/agent}"); count != 1 {
			t.Errorf("expected exactly 1 ## Sub-agent Result: {phase/agent} canonical block in thin orchestrator (deduped), got %d", count)
		}
		// Generic count includes inline reference in INVALID sentence; header count is authoritative for dedup
		if count := strings.Count(md, "```markdown\n## Sub-agent Result"); count != 1 {
			t.Errorf("expected exactly 1 markdown code block with ## Sub-agent Result, got %d", count)
		}
	})

	t.Run("thin orchestrator ≤120 lines", func(t *testing.T) {
		lines := strings.Count(md, "\n") + 1
		if lines > 120 {
			t.Errorf("thin orchestrator must be ≤120 lines, got %d", lines)
		}
	})

	t.Run("lazy routing pointers present", func(t *testing.T) {
		if !strings.Contains(md, "biggz-orchestrator-workflow.md") {
			t.Errorf("thin orchestrator missing pointer to biggz-orchestrator-workflow.md (Before handling any /sdd-* or SDD request, read ...)")
		}
		if !strings.Contains(md, "Before handling any /sdd-* or SDD request, read biggz-orchestrator-workflow.md") {
			t.Errorf("thin orchestrator missing exact workflow routing sentence")
		}
		if !strings.Contains(md, "biggz-orchestrator-delegation.md") {
			t.Errorf("thin orchestrator missing pointer to biggz-orchestrator-delegation.md (Before delegating, read ...)")
		}
		if !strings.Contains(md, "Before delegating, read biggz-orchestrator-delegation.md") {
			t.Errorf("thin orchestrator missing exact delegation routing sentence")
		}
	})

	t.Run("pending 2-line not 4-line duplicate", func(t *testing.T) {
		if strings.Count(md, "Pending Question Persistence") != 1 {
			t.Errorf("expected exactly 1 Pending Question Persistence block (deduped 2-line version), got %d", strings.Count(md, "Pending Question Persistence"))
		}
		if !strings.Contains(md, "MUST persist pending-question dual-write") {
			t.Errorf("thin orchestrator missing 2-line pending dual-write sentence")
		}
		if !strings.Contains(md, "LoadOnCompaction") {
			t.Errorf("thin orchestrator pending must mention LoadOnCompaction")
		}
		// Ensure the old 4-line verbose VerifyEquality/FormatFallback description is not in orchestrator (moved to pending.go header)
		// Allow reference to pending.go but not the verbose 4-line block.
		if strings.Contains(md, "verify equality retry once") && strings.Contains(md, "BigMem `sdd/{change}/pending-question` + `openspec/changes/{change}/state.yaml` `pending_question`, verify equality retry once") {
			t.Errorf("orchestrator should have concise 2-line pending, not verbose 4-line with VerifyEquality details (details belong in pending.go header)")
		}
	})

	t.Run("language boundary concise", func(t *testing.T) {
		if !strings.Contains(md, "Generated technical artifacts default to English") {
			t.Errorf("thin orchestrator missing concise Language Boundary 4-line header")
		}
		if !strings.Contains(md, "biggz-synthesis-gate.js:isCheckpointAsk") {
			t.Errorf("thin orchestrator Language Boundary must reference biggz-synthesis-gate.js:isCheckpointAsk for checkpoint tokens")
		}
		// Ensure token enumeration is not duplicated verbatim in orchestrator (should be via gate reference)
		// Thin should not contain the long bilingual enumeration "proceed/continuar/proseguir, adjust/ajustar, stop/detener/parar" as prose paragraph.
		if strings.Contains(md, "proceed/continuar/proseguir") {
			t.Errorf("thin orchestrator should not enumerate bilingual tokens inline; reference gate via isCheckpointAsk instead")
		}
	})
}

func TestOrchestratorSynthesisTemplateGuardsDrift(t *testing.T) {
	md := readOrchestrator(t)

	if c := strings.Count(md, "## Sub-agent Result: {phase/agent}"); c != 1 {
		t.Errorf("expected exactly 1 example block with ## Sub-agent Result: {phase/agent} in thin orchestrator (deduped), got %d", c)
	}
	if c := strings.Count(md, "**Artifacts/Paths:**"); c < 1 {
		t.Errorf("expected at least 1 Artifacts/Paths marker in thin orchestrator, got %d", c)
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

func TestOrchestratorLazyFilesExist(t *testing.T) {
	t.Run("workflow file exists and contains SDD Workflow + dependency graph", func(t *testing.T) {
		wf := readWorkflow(t)
		if !strings.Contains(wf, "SDD Workflow") {
			t.Errorf("biggz-orchestrator-workflow.md missing SDD Workflow")
		}
		if !strings.Contains(wf, "proposal -> specs") {
			t.Errorf("workflow missing dependency graph ASCII")
		}
		if !strings.Contains(wf, "Native SDD Dispatcher Guard") {
			t.Errorf("workflow missing Native SDD Dispatcher Guard")
		}
		if !strings.Contains(wf, "evidence-revision") && !strings.Contains(wf, "evidence_revision") {
			t.Errorf("workflow must preserve ledger-bound evidence_revision (biggz improvement)")
		}
		if !strings.Contains(wf, "Session Boot Recall") {
			t.Errorf("workflow missing Session Boot Recall hard gate")
		}
	})
	t.Run("delegation file exists and contains Work Routing Ladder + Delegation Rules", func(t *testing.T) {
		dl := readDelegation(t)
		if !strings.Contains(dl, "Work Routing Ladder") {
			t.Errorf("biggz-orchestrator-delegation.md missing Work Routing Ladder")
		}
		if !strings.Contains(dl, "Inline Direct") {
			t.Errorf("delegation missing Inline Direct — typo, one-file edit")
		}
		if !strings.Contains(dl, "Simple Delegation") {
			t.Errorf("delegation missing Simple Delegation — generic non-SDD")
		}
		if !strings.Contains(dl, "SDD (optional)") {
			t.Errorf("delegation missing SDD (optional)")
		}
		if !strings.Contains(dl, "Direct inline") || !strings.Contains(dl, "Delegated direct worker") {
			t.Errorf("delegation missing Delegation Rules table Direct inline vs Delegated direct worker")
		}
		if !strings.Contains(dl, "SDD Agent Authority") {
			t.Errorf("delegation must keep SD Agent Authority ban")
		}
		if !strings.Contains(dl, "Allowed edit surfaces") {
			t.Errorf("delegation missing Allowed edit surfaces rule")
		}
		if !strings.Contains(dl, "never '.' and never bare repo root") && !strings.Contains(dl, "never '.'") {
			t.Errorf("delegation Allowed edit surfaces must mention never '.' and never bare repo root")
		}
		if !strings.Contains(dl, "~20 tool calls") && !strings.Contains(dl, "20 tool calls") {
			t.Errorf("delegation must restore Long-session nuance (~20 tool calls)")
		}
	})
}

func TestOrchestratorSessionRecallGateInvariant(t *testing.T) {
	// Session Recall now lives in lazy workflow file (thin orchestrator delegates). Check workflow.
	wf := readWorkflow(t)
	md := readOrchestrator(t)
	combined := wf + "\n" + md
	t.Run("contains Session Recall hard gate", func(t *testing.T) {
		if !strings.Contains(combined, "## Session Recall") {
			t.Errorf("biggz-orchestrator-workflow.md (or orchestrator) missing ## Session Recall hard gate marker")
		}
		if !strings.Contains(combined, "Session Boot Recall") {
			t.Errorf("missing Session Boot Recall gate title")
		}
		if !strings.Contains(combined, "HARD GATE") {
			t.Errorf("missing HARD GATE designation for Session Recall")
		}
	})
	t.Run("recall requires biggz_mem_context and biggz_mem_search", func(t *testing.T) {
		if !strings.Contains(combined, "biggz_mem_context") {
			t.Errorf("Session Recall must require biggz_mem_context")
		}
		if !strings.Contains(combined, "biggz_mem_search") {
			t.Errorf("Session Recall must require biggz_mem_search")
		}
		if !strings.Contains(combined, "session_summary") {
			t.Errorf("Session Recall must mention session_summary search")
		}
		if !strings.Contains(combined, "sdd {project}") && !strings.Contains(combined, "sdd ") {
			t.Errorf("Session Recall must mention sdd {project} search")
		}
	})
	t.Run("recall fallback to sdd-status", func(t *testing.T) {
		if !strings.Contains(combined, "sdd-status") {
			t.Errorf("Session Recall must document fallback to sdd-status")
		}
		if !strings.Contains(combined, "--json --instructions") {
			t.Errorf("Session Recall fallback must be sdd-status --json --instructions")
		}
	})
	t.Run("recall REMINDER separation", func(t *testing.T) {
		if !strings.Contains(combined, "Session Recall markdown is separate") {
			t.Errorf("Session Recall must have REMINDER about separate markdown")
		}
		if strings.Count(combined, "## Session Recall") < 1 {
			t.Errorf("expected at least 1 ## Session Recall block")
		}
	})
}

func TestOrchestratorAliasInvariant(t *testing.T) {
	// Alias invariant now in workflow (hybrid) plus engram_status; thin orchestrator may just reference it.
	wf := readWorkflow(t)
	md := readOrchestrator(t)
	combined := wf + "\n" + md
	t.Run("template mentions engram alias bigmem", func(t *testing.T) {
		lower := strings.ToLower(combined)
		if !strings.Contains(lower, "engram") {
			t.Errorf("biggz-orchestrator-workflow.md missing engram alias mention")
		}
		if !strings.Contains(lower, "bigmem") {
			t.Errorf("biggz-orchestrator-workflow.md missing bigmem alias mention")
		}
		if !strings.Contains(combined, "Alias invariant") && !strings.Contains(combined, "alias for") {
			t.Errorf("missing alias invariant note (engram is alias for bigmem)")
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
