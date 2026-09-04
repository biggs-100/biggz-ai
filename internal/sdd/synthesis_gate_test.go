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

func TestIsCheckpointAskEnvelopeLabelsOnly(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	// Token in question BODY with neutral labels is content, not a choice → false.
	bodyOnly := `{"questions":[{"header":"Ritmo","question":"¿Qué ritmo usamos para continuar con el change?","options":[{"label":"Interactivo"},{"label":"Automático"}]}]}`
	if IsCheckpointAsk(bodyOnly) {
		t.Fatalf("IsCheckpointAsk must be false when token is only in question body, labels neutral")
	}
	// Token in a label is a checkpoint choice → true.
	labelHit := `{"questions":[{"header":"Checkpoint","question":"¿Seguimos?","options":[{"label":"Continuar"},{"label":"Ajustar"}]}]}`
	if !IsCheckpointAsk(labelHit) {
		t.Fatalf("IsCheckpointAsk must be true when token is in an option label")
	}
	// Token in option value counts as a choice → true.
	valueHit := `{"questions":[{"question":"Next?","options":[{"label":"Go","value":"proceed"}]}]}`
	if !IsCheckpointAsk(valueHit) {
		t.Fatalf("IsCheckpointAsk must be true when token is in an option value")
	}
	// Plain-string options are labels → true when token present.
	stringHit := `{"questions":[{"question":"Next?","options":["proceed","stop"]}]}`
	if !IsCheckpointAsk(stringHit) {
		t.Fatalf("IsCheckpointAsk must be true for plain-string checkpoint options")
	}
	// Top-level options with neutral labels → false even with token in body.
	topNeutral := `{"question":"¿Cómo continuar?","options":[{"label":"A"},{"label":"B"}]}`
	if IsCheckpointAsk(topNeutral) {
		t.Fatalf("IsCheckpointAsk must be false for neutral top-level options")
	}
	// Raw non-envelope strings keep legacy whole-string behavior.
	if !IsCheckpointAsk("Continuar con el cambio") {
		t.Fatalf("legacy raw-string scan must stay true for backward compat")
	}
}

func TestShouldBlock(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	good := mustSynthesisMD("full-prose")

	// Enforcement RETIRED (2026-09-04): ShouldBlock is a passthrough.
	// Context-before-question is governed by the explicit agent contract
	// in docs, not by code. Every case below must allow.
	SetCurrentTurnMarkdown(good)
	for _, q := range []string{"proceed", "continuar", "how are you?", "general question"} {
		for _, md := range []string{good, "no markers at all", ""} {
			if ShouldBlock(q, md, time.Now()) {
				t.Fatalf("ShouldBlock(%q) must be false (passthrough retired)", q)
			}
			if ShouldBlock(q, md, time.Now().Add(121*time.Second)) {
				t.Fatalf("ShouldBlock(%q) must be false even when expired (passthrough retired)", q)
			}
	}
	}
}

func TestShouldBlock_PassthroughRetired(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	// Enforcement RETIRED: checkpoint without synthesis must NOT block even
	// with empty history. The agent contract governs context-before-question.
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	for _, q := range []string{"proceed", "continuar", "Pace", "Artifacts", "PRs", "Review", "¿por dónde empezamos?"} {
		for _, md := range []string{"", "no markers at all"} {
			if ShouldBlock(q, md, time.Now()) {
				t.Fatalf("ShouldBlock(%q) must be false (passthrough retired)", q)
			}
	}
	}
}

func TestShouldBlock_ReqDG1_OptionBearingNonCheckpoint(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	// REQ-DG-1: HasOptions alone MUST NOT block. Only IsCheckpointAsk gates.
	// Free-text ask (no options, no checkpoint token) never blocks.
	freeText := `{"question":"How are you doing today?"}`
	if IsCheckpointAsk(freeText) || HasOptions(freeText) {
		t.Fatalf("precondition: free-text fixture must be neither checkpoint nor option-bearing")
	}
	if ShouldBlock(freeText, "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for free-text ask even without synthesis")
	}
	if ShouldBlock(freeText, "", time.Now()) {
		t.Fatalf("ShouldBlock should be false for free-text ask with empty md")
	}

	// Session Preflight option-ask (bears options, no checkpoint token) never blocks.
	// Labels avoid checkpoint tokens (proceed/adjust/stop/continue/correct + ES variants).
	preflightOptions := `{"questions":[{"question":"Pick pace","options":[{"label":"Relaxed"},{"label":"Fast"}]}]}`
	if !HasOptions(preflightOptions) {
		t.Fatalf("precondition: preflight fixture must bear options")
	}
	if IsCheckpointAsk(preflightOptions) {
		t.Fatalf("precondition: preflight fixture must not be a checkpoint ask")
	}
	if ShouldBlock(preflightOptions, "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for preflight option-ask even without synthesis (REQ-DG-1)")
	}
	if ShouldBlock(preflightOptions, "", time.Now()) {
		t.Fatalf("ShouldBlock should be false for preflight option-ask with empty md (REQ-DG-1)")
	}
	good := mustSynthesisMD("full-prose")
	SetCurrentTurnMarkdown(good)
	if ShouldBlock(preflightOptions, good, time.Now().Add(121*time.Second)) {
		t.Fatalf("ShouldBlock should be false for preflight option-ask even when window expired")
	}

	// Checkpoint WITH options and no synthesis NO LONGER blocks (retired).
	checkpointOptions := `{"questions":[{"question":"Next?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`
	if !HasOptions(checkpointOptions) || !IsCheckpointAsk(checkpointOptions) {
		t.Fatalf("precondition: checkpoint fixture must bear options AND checkpoint token")
	}
	SetCurrentTurnMarkdown(good)
	if ShouldBlock(checkpointOptions, "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for option-bearing checkpoint without synthesis (retired)")
	}
	// Valid synthesis in window passes.
	SetCurrentTurnMarkdown(good)
	if ShouldBlock(checkpointOptions, good, time.Now().Add(30*time.Second)) {
		t.Fatalf("ShouldBlock should be false for option-bearing checkpoint with valid synthesis")
	}
}

func TestShouldBlock_ReqDG1_BypassUnchanged(t *testing.T) {
	// Threat-matrix row: PI_SUBAGENT_CHILD + Session Recall bypasses keep prior behavior.
	checkpointOptions := `{"questions":[{"question":"Next?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`

	t.Setenv("PI_SUBAGENT_CHILD", "1")
	if ShouldBlock(checkpointOptions, "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for option-bearing checkpoint in child (bypass unchanged)")
	}

	t.Setenv("PI_SUBAGENT_CHILD", "0")
	recallMD := "## Session Recall\nsome previous context\n"
	SetCurrentTurnMarkdown(recallMD)
	if ShouldBlock(checkpointOptions, recallMD, time.Now()) {
		t.Fatalf("ShouldBlock should be false for option-bearing checkpoint with Session Recall (bypass unchanged)")
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
	// back to orchestrator, passthrough retired: never blocks when missing
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	if ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("ShouldBlock should be false for orchestrator when missing (retired)")
	}
}

// TestShouldBlockApplyAdmission_NoBypasses documents the RETIRED write-
// admission gate: it is now a passthrough like ShouldBlock. Human checkpoints
// in the workflow govern write admission, not code.
func TestShouldBlockApplyAdmission_NoBypasses(t *testing.T) {
	good := mustSynthesisMD("full-prose")

	t.Run("child without synthesis allows (retired)", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "1")
		SetCurrentTurnMarkdown(good)
		if ShouldBlock("proceed", "no markers", time.Now()) {
			t.Fatalf("precondition: ShouldBlock must bypass for the child")
		}
		if ShouldBlockApplyAdmission("proceed", "no markers", time.Now()) {
			t.Fatalf("ShouldBlockApplyAdmission must allow a child checkpoint without synthesis (retired)")
		}
	})

	t.Run("session recall without synthesis allows (retired)", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "0")
		recallMD := "## Session Recall\nsome previous context\n"
		SetCurrentTurnMarkdown(recallMD)
		if ShouldBlock("proceed", recallMD, time.Now()) {
			t.Fatalf("precondition: ShouldBlock must bypass for Session Recall")
		}
		if ShouldBlockApplyAdmission("proceed", recallMD, time.Now()) {
			t.Fatalf("ShouldBlockApplyAdmission must allow Session Recall without checkpoint synthesis (retired)")
		}
	})

	t.Run("human checkpoint with synthesis allows even as child", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "1")
		SetCurrentTurnMarkdown(good)
		if ShouldBlockApplyAdmission("proceed", good, time.Now().Add(30*time.Second)) {
			t.Fatalf("ShouldBlockApplyAdmission must allow a checkpoint with synthesis in window")
		}
	})

	t.Run("expired synthesis allows (retired)", func(t *testing.T) {
		t.Setenv("PI_SUBAGENT_CHILD", "0")
		SetCurrentTurnMarkdown(good)
		if ShouldBlockApplyAdmission("proceed", good, time.Now().Add(121*time.Second)) {
			t.Fatalf("ShouldBlockApplyAdmission must allow when synthesis is expired (retired)")
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

// TestBlockedEnvelope_ReqDG2_FallbackVerbatim pins REQ-DG-2: a blocked
// checkpoint emits context + fallback with the full question verbatim
// (via FormatFallback); free-text and preflight option-asks never block.
func TestBlockedEnvelope_ReqDG2_FallbackVerbatim(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	good := mustSynthesisMD("full-prose")
	SetCurrentTurnMarkdown(good)

	checkpointEnv := QuestionEnvelope{Questions: []Question{{Header: "Decisi\u00f3n", Question: "Proceed with plan?", Options: []QuestionOption{{Label: "Proceed"}, {Label: "Adjust"}}}}}
	checkpointQ := `{"questions":[{"question":"Proceed with plan?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`
	env := BuildBlockedEnvelope(checkpointQ, "no markers", time.Now(), checkpointEnv)
	if env.Block {
		t.Fatalf("BuildBlockedEnvelope must not block checkpoint without synthesis (retired)")
	}
	// Context/fallback payload retired with blocking: envelope carries no block.
	if env.Context != "" || env.Fallback != "" {
		t.Fatalf("retired blocked envelope must be empty, got context %q fallback %q", env.Context, env.Fallback)
	}

	freeTextQ := `{"question":"How are you doing today?"}`
	freeEnv := QuestionEnvelope{}
	if got := BuildBlockedEnvelope(freeTextQ, "no markers", time.Now(), freeEnv); got.Block {
		t.Fatalf("BuildBlockedEnvelope must not block free-text ask")
	}

	preflightQ := `{"questions":[{"question":"Pick pace","options":[{"label":"Relaxed"},{"label":"Fast"}]}]}`
	preflightEnv := QuestionEnvelope{Questions: []Question{{Question: "Pick pace", Options: []QuestionOption{{Label: "Relaxed"}, {Label: "Fast"}}}}}
	if got := BuildBlockedEnvelope(preflightQ, "no markers", time.Now(), preflightEnv); got.Block {
		t.Fatalf("BuildBlockedEnvelope must not block preflight option-ask (REQ-DG-1)")
	}

	SetCurrentTurnMarkdown(good)
	if got := BuildBlockedEnvelope(checkpointQ, good, time.Now().Add(30*time.Second), checkpointEnv); got.Block {
		t.Fatalf("BuildBlockedEnvelope must not block checkpoint with valid synthesis")
	}
}
