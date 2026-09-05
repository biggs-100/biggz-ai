package sdd

import (
	"testing"
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

func TestHasOptionsAdviseOnly(t *testing.T) {
	// REQ-DG-1: HasOptions alone never blocks — advise-only helper.
	// Free-text ask (no options, no checkpoint token) is neither.
	freeText := `{"question":"How are you doing today?"}`
	if IsCheckpointAsk(freeText) || HasOptions(freeText) {
		t.Fatalf("precondition: free-text fixture must be neither checkpoint nor option-bearing")
	}

	// Session Preflight option-ask bears options but no checkpoint token.
	preflightOptions := `{"questions":[{"question":"Pick pace","options":[{"label":"Relaxed"},{"label":"Fast"}]}]}`
	if !HasOptions(preflightOptions) {
		t.Fatalf("precondition: preflight fixture must bear options")
	}
	if IsCheckpointAsk(preflightOptions) {
		t.Fatalf("precondition: preflight fixture must not be a checkpoint ask")
	}

	// Checkpoint WITH options carries both signals (advise, never block).
	checkpointOptions := `{"questions":[{"question":"Next?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`
	if !HasOptions(checkpointOptions) || !IsCheckpointAsk(checkpointOptions) {
		t.Fatalf("precondition: checkpoint fixture must bear options AND checkpoint token")
	}
}
