package sdd

import (
	"strings"
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
	case "thin":
		return base + "**What was done:**\n| Topic | Decision |\n|-------|----------|\n| a | b |\n◆ phase · success · next\n**Artifacts/Paths:** a\n**Risks / Open Questions:** none\n**Next Recommended:** verify\n"
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
		t.Fatalf("full-prose should pass")
	}
	if !HasSynthesis(mustSynthesisMD("full-table")) {
		t.Fatalf("full-table should pass")
	}
	if HasSynthesis(mustSynthesisMD("missing-artifacts")) {
		t.Fatalf("missing artifacts should fail")
	}
	if HasSynthesis(mustSynthesisMD("missing-risks")) {
		t.Fatalf("missing risks should fail")
	}
	if HasSynthesis(mustSynthesisMD("missing-next")) {
		t.Fatalf("missing next should fail")
	}
	if HasSynthesis(mustSynthesisMD("missing-whatdone")) {
		t.Fatalf("missing whatdone should fail")
	}
	if HasSynthesis("## Sub-agent Result\n**Artifacts/Paths:** a\n**Risks / Open Questions:** b\n") {
		t.Fatalf("partial should fail")
	}
}

func TestIsCheckpointAsk(t *testing.T) {
	cases := []struct {
		q    string
		want bool
	}{
		{"proceed", true}, {"Proceed with plan", true}, {"adjust", true}, {"stop", true}, {"continue", true}, {"correct", true},
		{"continuar", true}, {"Continuar con el cambio", true}, {"ajustar", true}, {"detener", true}, {"parar", true}, {"cerrar", true}, {"corregir", true}, {"proseguir", true},
		{"how are you?", false}, {"what is the status?", false}, {"opción A", false}, {"", false},
		{"PROCEED", true}, {"CONTINUAR", true}, {"please proceed to next phase", true}, {"por favor continuar", true},
	}
	for _, c := range cases {
		if got := IsCheckpointAsk(c.q); got != c.want {
			t.Errorf("IsCheckpointAsk(%q)=%v want %v", c.q, got, c.want)
		}
	}
	jsonEnvelope := `{"questions":[{"question":"Next?","options":[{"label":"Continuar (Recomendado)"},{"label":"Ajustar"}]}]}`
	if !IsCheckpointAsk(jsonEnvelope) {
		t.Fatalf("bilingual token in JSON envelope should be detected")
	}
}

func TestIsCheckpointAskEnvelopeLabelsOnly(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	bodyOnly := `{"questions":[{"header":"Ritmo","question":"¿Qué ritmo usamos para continuar con el change?","options":[{"label":"Interactivo"},{"label":"Automático"}]}]}`
	if IsCheckpointAsk(bodyOnly) {
		t.Fatalf("body-only token with neutral labels must not be checkpoint")
	}
	labelHit := `{"questions":[{"header":"Checkpoint","question":"¿Seguimos?","options":[{"label":"Continuar"},{"label":"Ajustar"}]}]}`
	if !IsCheckpointAsk(labelHit) {
		t.Fatalf("label hit must be checkpoint")
	}
	valueHit := `{"questions":[{"question":"Next?","options":[{"label":"Go","value":"proceed"}]}]}`
	if !IsCheckpointAsk(valueHit) {
		t.Fatalf("value hit must be checkpoint")
	}
	stringHit := `{"questions":[{"question":"Next?","options":["proceed","stop"]}]}`
	if !IsCheckpointAsk(stringHit) {
		t.Fatalf("plain-string checkpoint must be true")
	}
	topNeutral := `{"question":"¿Cómo continuar?","options":[{"label":"A"},{"label":"B"}]}`
	if IsCheckpointAsk(topNeutral) {
		t.Fatalf("neutral top-level must not be checkpoint")
	}
	if !IsCheckpointAsk("Continuar con el cambio") {
		t.Fatalf("legacy raw-string must stay true")
	}
}

func TestHasOptionsAdviseOnly(t *testing.T) {
	freeText := `{"question":"How are you doing today?"}`
	if IsCheckpointAsk(freeText) || HasOptions(freeText) {
		t.Fatalf("free-text must be neither checkpoint nor option-bearing")
	}
	preflightOptions := `{"questions":[{"question":"Pick pace","options":[{"label":"Relaxed"},{"label":"Fast"}]}]}`
	if !HasOptions(preflightOptions) || IsCheckpointAsk(preflightOptions) {
		t.Fatalf("preflight must bear options but not checkpoint")
	}
	checkpointOptions := `{"questions":[{"question":"Next?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`
	if !HasOptions(checkpointOptions) || !IsCheckpointAsk(checkpointOptions) {
		t.Fatalf("checkpoint must bear options and checkpoint token")
	}
}

func TestShouldBlock(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	good := mustSynthesisMD("full-prose")
	SetCurrentTurnMarkdown(good)
	if ShouldBlock("proceed", good, time.Now().Add(30*time.Second)) {
		t.Fatalf("within window should allow")
	}
	if ShouldBlock("continuar", good, time.Now().Add(30*time.Second)) {
		t.Fatalf("Spanish token within window should allow")
	}
	SetCurrentTurnMarkdown(good)
	if !ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("missing should block")
	}
	SetCurrentTurnMarkdown(good)
	if !ShouldBlock("proceed", good, time.Now().Add(121*time.Second)) {
		t.Fatalf("expired 121s should block")
	}
	if ShouldBlock("how are you?", "no markers", time.Now()) {
		t.Fatalf("non-checkpoint must not block even without synthesis")
	}
	if ShouldBlock("how are you?", good, time.Now()) {
		t.Fatalf("non-checkpoint must not block even with synthesis")
	}
	// history/empty md still blocks for checkpoint
	if !ShouldBlock("proceed", "", time.Now()) {
		t.Fatalf("empty md should block for checkpoint")
	}
	if ShouldBlock("Pace", "", time.Now()) {
		t.Fatalf("preflight Pace must not block")
	}
}

func TestShouldBlock_OptionBearingAndBypass(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	freeText := `{"question":"How are you doing today?"}`
	if ShouldBlock(freeText, "no markers", time.Now()) {
		t.Fatalf("free-text must not block")
	}
	preflightOptions := `{"questions":[{"question":"Pick pace","options":[{"label":"Relaxed"},{"label":"Fast"}]}]}`
	if ShouldBlock(preflightOptions, "no markers", time.Now()) {
		t.Fatalf("preflight option-ask must not block REQ-DG-1")
	}
	good := mustSynthesisMD("full-prose")
	SetCurrentTurnMarkdown(good)
	if ShouldBlock(preflightOptions, good, time.Now().Add(121*time.Second)) {
		t.Fatalf("preflight must not block even when expired")
	}
	checkpointOptions := `{"questions":[{"question":"Next?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`
	if !ShouldBlock(checkpointOptions, "no markers", time.Now()) {
		t.Fatalf("option-bearing checkpoint without synthesis must block")
	}
	SetCurrentTurnMarkdown(good)
	if ShouldBlock(checkpointOptions, good, time.Now().Add(30*time.Second)) {
		t.Fatalf("option-bearing checkpoint with synthesis should allow")
	}
	// child bypass unchanged
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	if ShouldBlock(checkpointOptions, "no markers", time.Now()) {
		t.Fatalf("child bypass must allow")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	recallMD := "## Session Recall\nsome previous context\n"
	SetCurrentTurnMarkdown(recallMD)
	if ShouldBlock(checkpointOptions, recallMD, time.Now()) {
		t.Fatalf("recall bypass must allow")
	}
}

func TestShouldBlock_SessionRecallAndChild(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	recallMD := "## Session Recall\nsome previous context\n"
	SetCurrentTurnMarkdown(recallMD)
	if ShouldBlock("proceed", recallMD, time.Now()) {
		t.Fatalf("recall should bypass")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	if ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("child should bypass")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	if !ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("orchestrator missing should block")
	}
}

func TestShouldBlock_TurnResetAndBodyThin(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	good := mustSynthesisMD("full-prose")
	SetCurrentTurnMarkdown(good)
	if ShouldBlock("proceed", good, time.Now().Add(10*time.Second)) {
		t.Fatalf("precondition fresh synthesis should allow")
	}
	// turn_start reset
	currentTurnMarkdown = ""
	currentTurnTime = time.Time{}
	if !ShouldBlock("proceed", "", time.Now()) {
		t.Fatalf("after turn reset should block")
	}
	if !ShouldBlock("proceed", good, time.Now()) {
		t.Fatalf("reset time should block even if md has synthesis")
	}
	SetCurrentTurnMarkdown(good)
	if ShouldBlock("proceed", good, time.Now().Add(10*time.Second)) {
		t.Fatalf("after new synthesis should allow")
	}
	// body-token never blocks
	bodyOnly := `{"questions":[{"header":"Ritmo","question":"¿Qué ritmo usamos para continuar con el change?","options":[{"label":"Interactivo"},{"label":"Automático"}]}]}`
	if ShouldBlock(bodyOnly, "no markers", time.Now()) {
		t.Fatalf("body-only must not block")
	}
	labelHit := `{"questions":[{"header":"Checkpoint","question":"¿Seguimos?","options":[{"label":"Continuar"},{"label":"Ajustar"}]}]}`
	if !ShouldBlock(labelHit, "no markers", time.Now()) {
		t.Fatalf("label hit without synthesis must block")
	}
	// thin allow
	thin := mustSynthesisMD("thin")
	SetCurrentTurnMarkdown(thin)
	if ShouldBlock("proceed", thin, time.Now().Add(10*time.Second)) {
		t.Fatalf("thin synthesis should still allow")
	}
	t.Setenv("BIGGZ_ADVISE", "1")
	SetCurrentTurnMarkdown(thin)
	if ShouldBlock("proceed", thin, time.Now().Add(10*time.Second)) {
		t.Fatalf("thin with advise should still allow")
	}
}

func TestShouldBlockApplyAdmission_NoBypasses(t *testing.T) {
	good := mustSynthesisMD("full-prose")
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	SetCurrentTurnMarkdown(good)
	if ShouldBlock("proceed", "no markers", time.Now()) {
		t.Fatalf("precondition child bypass")
	}
	if !ShouldBlockApplyAdmission("proceed", "no markers", time.Now()) {
		t.Fatalf("admission must block child without synthesis")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	recallMD := "## Session Recall\nsome previous context\n"
	SetCurrentTurnMarkdown(recallMD)
	if ShouldBlock("proceed", recallMD, time.Now()) {
		t.Fatalf("precondition recall bypass")
	}
	if !ShouldBlockApplyAdmission("proceed", recallMD, time.Now()) {
		t.Fatalf("admission must block recall without synthesis")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	SetCurrentTurnMarkdown(good)
	if ShouldBlockApplyAdmission("proceed", good, time.Now().Add(30*time.Second)) {
		t.Fatalf("admission should allow checkpoint with synthesis even as child")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	SetCurrentTurnMarkdown(good)
	if !ShouldBlockApplyAdmission("proceed", good, time.Now().Add(121*time.Second)) {
		t.Fatalf("admission must block expired")
	}
	if ShouldBlockApplyAdmission("how are you?", "no markers", time.Now()) {
		t.Fatalf("admission must allow non-checkpoint")
	}
}

func TestBlockedEnvelope_ReqDG2_FallbackVerbatim(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	good := mustSynthesisMD("full-prose")
	SetCurrentTurnMarkdown(good)
	checkpointEnv := QuestionEnvelope{Questions: []Question{{Header: "Decisión", Question: "Proceed with plan?", Options: []QuestionOption{{Label: "Proceed", Description: "go"}, {Label: "Adjust", Description: "tweak"}}}}}
	checkpointQ := `{"questions":[{"question":"Proceed with plan?","options":[{"label":"Proceed"},{"label":"Adjust"}]}]}`
	env := BuildBlockedEnvelope(checkpointQ, "no markers", time.Now(), checkpointEnv)
	if !env.Block || !strings.Contains(env.Context, checkpointQ) {
		t.Fatalf("blocked envelope must have block and context, got %+v", env)
	}
	for _, want := range []string{"Proceed with plan?", "Proceed", "Adjust"} {
		if !strings.Contains(env.Fallback, want) {
			t.Fatalf("fallback must contain %q, got %q", want, env.Fallback)
		}
	}
	freeTextQ := `{"question":"How are you doing today?"}`
	if got := BuildBlockedEnvelope(freeTextQ, "no markers", time.Now(), QuestionEnvelope{}); got.Block {
		t.Fatalf("free-text must not block")
	}
	preflightQ := `{"questions":[{"question":"Pick pace","options":[{"label":"Relaxed"},{"label":"Fast"}]}]}`
	preflightEnv := QuestionEnvelope{Questions: []Question{{Question: "Pick pace", Options: []QuestionOption{{Label: "Relaxed", Description: "slow"}, {Label: "Fast", Description: "quick"}}}}}
	if got := BuildBlockedEnvelope(preflightQ, "no markers", time.Now(), preflightEnv); got.Block {
		t.Fatalf("preflight must not block")
	}
	SetCurrentTurnMarkdown(good)
	if got := BuildBlockedEnvelope(checkpointQ, good, time.Now().Add(30*time.Second), checkpointEnv); got.Block {
		t.Fatalf("valid synthesis should not block envelope")
	}
}

func TestCheckSynthesisPrecondition_Message(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	ok, msg := CheckSynthesisPrecondition("proceed", "no markers")
	if ok || msg != "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)" {
		t.Fatalf("blocked precondition message mismatch ok=%v msg=%q", ok, msg)
	}
	SetCurrentTurnMarkdown(mustSynthesisMD("full-prose"))
	ok, msg = CheckSynthesisPrecondition("proceed", mustSynthesisMD("full-prose"))
	if !ok || msg != "" {
		t.Fatalf("allow precondition should be ok true empty msg, got %v %q", ok, msg)
	}
}
