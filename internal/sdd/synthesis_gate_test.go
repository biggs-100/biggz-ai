package sdd

import (
	"testing"
	"time"
)

func mustSynthesisMD(variant string) string {
	base := "## Sub-agent Result: test\n"
	switch variant {
	case "full-prose":
		return base + "**What was done:**\n| Topic | Decision |\n|-------|----------|\n| a | b |\n◆ phase · success · next\n**Artifacts/Paths:** a/b\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"
	case "full-table":
		return base + "| Topic | Decision |\n|-------|----------|\n| topic | decision |\n◆ phase · success · next\n**Artifacts/Paths:** internal/sdd/synthesis_gate.go\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"
	case "missing-artifacts":
		return base + "**What was done:**\n| Topic | Decision |\n|-------|----------|\n| a | b |\n◆ phase · success · next\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"
	case "missing-risks":
		return base + "**What was done:**\n| Topic | Decision |\n|-------|----------|\n| a | b |\n◆ phase · success · next\n**Artifacts/Paths:** a/b\n**Next Recommended:** verify\n"
	case "missing-next":
		return base + "**What was done:**\n| Topic | Decision |\n|-------|----------|\n| a | b |\n◆ phase · success · next\n**Artifacts/Paths:** a/b\n**Risks / Open Questions:** none\n"
	case "missing-whatdone":
		return base + "◆ phase · success · next\n**Artifacts/Paths:** a/b\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"
	default:
		return base + "**What was done:** done\n**Artifacts/Paths:** a/b\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"
	}
}

func TestHasSynthesis(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	if !HasSynthesis(mustSynthesisMD("full-prose")) {
		t.Fatalf("HasSynthesis should pass for full-prose with table")
	}
	if !HasSynthesis(mustSynthesisMD("full-table")) {
		t.Fatalf("HasSynthesis should pass for full-table alternative")
	}
	if HasSynthesis(mustSynthesisMD("missing-artifacts")) {
		t.Fatalf("HasSynthesis should fail when Artifacts missing")
	}
	if HasSynthesis(mustSynthesisMD("missing-risks")) {
		t.Fatalf("HasSynthesis should fail when Risks missing")
	}
	if HasSynthesis(mustSynthesisMD("missing-next")) {
		t.Fatalf("HasSynthesis should fail when Next missing")
	}
	if HasSynthesis(mustSynthesisMD("missing-whatdone")) {
		t.Fatalf("HasSynthesis should fail when WhatDone missing")
	}
	if HasSynthesis("## Sub-agent Result\n**Artifacts/Paths:** a\n**Risks / Open Questions:** b\n") {
		t.Fatalf("HasSynthesis should fail when Next missing (partial)")
	}
}

func TestIsCheckpointAsk(t *testing.T) {
	cases := []struct {
		q    string
		want bool
	}{
		{"proceed", true},
		{"Proceed with plan", true},
		{"adjust", true},
		{"stop", true},
		{"continue", true},
		{"correct", true},
		{"continuar", true},
		{"Continuar con el cambio", true},
		{"ajustar", true},
		{"detener", true},
		{"parar", true},
		{"cerrar", true},
		{"corregir", true},
		{"proseguir", true},
		{"how are you?", false},
		{"what is the status?", false},
		{"opción A", false},
		{"", false},
		{"PROCEED", true},
		{"CONTINUAR", true},
		{"please proceed to next phase", true},
		{"por favor continuar", true},
	}
	for _, c := range cases {
		if got := IsCheckpointAsk(c.q); got != c.want {
			t.Errorf("IsCheckpointAsk(%q)=%v want %v", c.q, got, c.want)
		}
	}
	// structured envelope JSON string contains token case-insensitive
	jsonEnvelope := `{"questions":[{"question":"Next?","options":[{"label":"Continuar (Recomendado)"},{"label":"Ajustar"}]}]}`
	if !IsCheckpointAsk(jsonEnvelope) {
		t.Fatalf("IsCheckpointAsk should detect bilingual token inside JSON envelope")
	}
}

func TestShouldBlock(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	good := mustSynthesisMD("full-prose")

	// within window allows
	SetCurrentTurnMarkdown(good)
	if ShouldBlock("proceed", good, time.Now().Add(30*time.Second)) {
		t.Fatalf("ShouldBlock should be false when synthesis within 30s window with proceed")
	}
	if ShouldBlock("continuar", good, time.Now().Add(30*time.Second)) {
		t.Fatalf("ShouldBlock should be false for Spanish token continuar within window")
	}

	// missing blocks
	SetCurrentTurnMarkdown(good)
	if !ShouldBlock("proceed", "no markers at all", time.Now()) {
		t.Fatalf("ShouldBlock should be true when synthesis missing")
	}
	if !ShouldBlock("continuar", "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be true for Spanish token when missing")
	}

	// expired blocks (121s)
	SetCurrentTurnMarkdown(good)
	if !ShouldBlock("proceed", good, time.Now().Add(121*time.Second)) {
		t.Fatalf("ShouldBlock should be true when expired 121s even with valid synthesis")
	}
	if !ShouldBlock("proceed", "no markers", time.Now().Add(121*time.Second)) {
		t.Fatalf("ShouldBlock should be true when missing and expired")
	}

	// non-checkpoint never blocks
	SetCurrentTurnMarkdown(good)
	// also test with missing synthesis but non-checkpoint
	if ShouldBlock("how are you?", "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for non-checkpoint question even without synthesis")
	}
	if ShouldBlock("how are you?", good, time.Now()) {
		t.Fatalf("ShouldBlock should be false for non-checkpoint even with synthesis")
	}
	// ensure non-checkpoint within and expired both allow
	if ShouldBlock("general question", "no markers", time.Now().Add(121*time.Second)) {
		t.Fatalf("non-checkpoint should not block even when expired")
	}
}

func TestShouldBlock_CheckpointWithoutSynthesisEvenWhenHistoryEmpty(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	// Bug regression: checkpoint without synthesis in current turn must block even when
	// there is no synthesis anywhere in history (anySynthesis empty). The old JS gate
	// confused empty history with preflight allowance and let checkpoint pass.
	// Go canonical never had that allowance — verify it blocks strictly.
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	// missing synthesis, empty history analogue (md parameter empty)
	if !ShouldBlock("proceed", "", time.Now()) {
		t.Fatalf("ShouldBlock should be true for checkpoint 'proceed' with empty md even when history empty")
	}
	if !ShouldBlock("continuar", "", time.Now()) {
		t.Fatalf("ShouldBlock should be true for checkpoint 'continuar' with empty md even when history empty")
	}
	if !ShouldBlock("proceed", "no markers at all", time.Now()) {
		t.Fatalf("ShouldBlock should be true for checkpoint with no markers even when history empty")
	}
	// Preflight non-checkpoint (Pace/Artifacts/PRs/Review) must still be allowed without synthesis
	// IsCheckpointAsk=false -> gate never blocks, regardless of md
	if ShouldBlock("Pace", "", time.Now()) {
		t.Fatalf("ShouldBlock should be false for preflight non-checkpoint 'Pace' even without synthesis")
	}
	if ShouldBlock("Artifacts", "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for preflight non-checkpoint 'Artifacts' even without synthesis")
	}
	if ShouldBlock("PRs", "", time.Now()) {
		t.Fatalf("ShouldBlock should be false for preflight non-checkpoint 'PRs' even without synthesis")
	}
	if ShouldBlock("Review", "", time.Now()) {
		t.Fatalf("ShouldBlock should be false for preflight non-checkpoint 'Review' even without synthesis")
	}
	if ShouldBlock("¿por dónde empezamos?", "", time.Now()) {
		t.Fatalf("ShouldBlock should be false for general non-checkpoint even without synthesis and empty history")
	}
}

func TestShouldBlock_SessionRecallBypass(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	recallMD := "## Session Recall\nsome previous context\n"
	SetCurrentTurnMarkdown(recallMD)
	// Even with checkpoint and no proper synthesis, Session Recall bypasses
	if ShouldBlock("proceed", recallMD, time.Now()) {
		t.Fatalf("ShouldBlock should be false when Session Recall present (bypass)")
	}
	if ShouldBlock("continuar", recallMD, time.Now().Add(10*time.Second)) {
		t.Fatalf("ShouldBlock should bypass for Spanish token with Session Recall")
	}
}

func TestShouldBlock_ChildBypass(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	// child bypass should never block even when missing
	if ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false when PI_SUBAGENT_CHILD=1 (child bypass)")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	// back to orchestrator, should block when missing
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	if !ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be true for orchestrator when missing")
	}
}

// TestShouldBlockApplyAdmission_NoBypasses pins the P0-2 hardening: admission
// to the apply phase (the phase that writes) requires a human
// checkpoint/proceed with synthesis even in auto mode and even in a child
// subagent. Unlike ShouldBlock, the write-admission gate honors neither the
// PI_SUBAGENT_CHILD bypass nor the Session Recall bypass, so auto
// back-to-back phases cannot self-validate their own entry into writing.
func TestShouldBlockApplyAdmission_NoBypasses(t *testing.T) {
	good := mustSynthesisMD("full-prose")

	t.Run("child without synthesis still blocks", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "1")
		SetCurrentTurnMarkdown(good)
		if ShouldBlock("proceed", "no markers", time.Now()) {
			t.Fatalf("precondition: ShouldBlock must bypass for the child")
		}
		if !ShouldBlockApplyAdmission("proceed", "no markers", time.Now()) {
			t.Fatalf("ShouldBlockApplyAdmission must block a child checkpoint without synthesis")
		}
	})

	t.Run("session recall without synthesis still blocks", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "0")
		recallMD := "## Session Recall\nsome previous context\n"
		SetCurrentTurnMarkdown(recallMD)
		if ShouldBlock("proceed", recallMD, time.Now()) {
			t.Fatalf("precondition: ShouldBlock must bypass for Session Recall")
		}
		if !ShouldBlockApplyAdmission("proceed", recallMD, time.Now()) {
			t.Fatalf("ShouldBlockApplyAdmission must block Session Recall without checkpoint synthesis")
		}
	})

	t.Run("human checkpoint with synthesis allows even as child", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "1")
		SetCurrentTurnMarkdown(good)
		if ShouldBlockApplyAdmission("proceed", good, time.Now().Add(30*time.Second)) {
			t.Fatalf("ShouldBlockApplyAdmission must allow a checkpoint with synthesis in window")
		}
	})

	t.Run("expired synthesis blocks", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "0")
		SetCurrentTurnMarkdown(good)
		if !ShouldBlockApplyAdmission("proceed", good, time.Now().Add(121*time.Second)) {
			t.Fatalf("ShouldBlockApplyAdmission must block when synthesis is expired")
		}
	})

	t.Run("non-checkpoint never blocks", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "0")
		SetCurrentTurnMarkdown(good)
		if ShouldBlockApplyAdmission("how are you?", "no markers", time.Now()) {
			t.Fatalf("ShouldBlockApplyAdmission must allow non-checkpoint asks")
		}
	})
}
