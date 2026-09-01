package sdd

import (
	"strings"
	"testing"
	"time"
)

func TestDetectLanguage(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"en que nos quedamos?", "es"},
		{"ok, continue", "en"},
		{"ok, continua", "es"},
		{"ok", "en"},
		{"hi", "en"},
		{"hello", "en"},
		{"¿qué?", "es"},
		{"dale", "en"},
		{"go", "en"},
		{"", "en"},
		{"   ", "en"},
		{"continua", "es"},
		{"continue", "en"},
		{"por favor continua con el spec", "es"},
		{"please continue with the spec", "en"},
		{"En Que Nos Quedamos?", "es"},
		{"ok, continua con el spec", "es"},
		{"á", "es"},
		{"ñ", "es"},
		{"¿¡", "es"},
	}
	for _, c := range cases {
		if got := DetectLanguage(c.input); got != c.want {
			t.Errorf("DetectLanguage(%q)=%q want %q", c.input, got, c.want)
		}
	}
}

func TestDetectLanguage_Red(t *testing.T) {
	if got := DetectLanguage("ok"); got != "en" {
		t.Fatalf("RED wrong-detection failed: DetectLanguage(ok)=%q want en (before fix would fail)", got)
	}
	if got := DetectLanguage("¿qué?"); got != "es" {
		t.Fatalf("RED wrong-detection failed: DetectLanguage(¿qué?)=%q want es", got)
	}
}

func TestRender_OverTranslation(t *testing.T) {
	r := SubAgentResult{
		Phase:           "sdd-apply",
		WhatDone:        "fix: done",
		ArtifactsPaths:  "internal/sdd/synthesis.go, sdd/polish-synthesis-human-language/proposal, ORDER BY updated_at DESC",
		Risks:           "None",
		NextRecommended: "verify",
	}
	out := RenderSynthesisLocalized(r, "es")
	for _, want := range []string{"internal/sdd/synthesis.go", "sdd/polish-synthesis-human-language/proposal", "ORDER BY"} {
		if !strings.Contains(out, want) {
			t.Errorf("over-translation: es render must keep %q verbatim, got %q", want, out)
		}
	}
	// Also ensure es render still has English markers
	if !HasSynthesis(out) {
		t.Errorf("over-translation: es render broke HasSynthesis markers")
	}
}

func TestRender_MarkerInvariant(t *testing.T) {
	// ShouldBlock is bypassed when PI_SUBAGENT_CHILD=1, so force orchestrator mode for this test.
	t.Setenv("PI_SUBAGENT_CHILD", "0")
	r := SubAgentResult{
		Phase:           "sdd-apply",
		WhatDone:        "contenido en español: corregido bug",
		ArtifactsPaths:  "internal/sdd/synthesis.go",
		Risks:           "ninguno",
		NextRecommended: "verificar",
	}
	outES := RenderSynthesisLocalized(r, "es")
	if !HasSynthesis(outES) {
		t.Fatalf("marker invariant: es content+en markers should pass HasSynthesis, got %q", outES)
	}
	SetCurrentTurnMarkdown(outES)
	if ShouldBlock("proceed", outES, time.Now()) {
		t.Errorf("marker invariant: es content+en markers should NOT block")
	}
	if ShouldBlock("continuar", outES, time.Now()) {
		t.Errorf("marker invariant: es content+en markers with Spanish token continuar should NOT block")
	}
	// Translated markers should fail
	translated := strings.ReplaceAll(outES, "**Artifacts/Paths:**", "**Artefactos/Rutas:**")
	if HasSynthesis(translated) {
		t.Errorf("marker invariant: translated Artifacts/Paths should fail HasSynthesis")
	}
	SetCurrentTurnMarkdown(translated)
	if !ShouldBlock("proceed", translated, time.Now()) {
		t.Errorf("marker invariant: missing Artifacts marker should block")
	}
	translated2 := strings.ReplaceAll(outES, "## Sub-agent Result", "## Resultado del Sub-agente")
	if HasSynthesis(translated2) {
		t.Errorf("marker invariant: translated header should fail HasSynthesis")
	}
}

func TestRenderSynthesisLocalized(t *testing.T) {
	t.Run("es vs en markers English", func(t *testing.T) {
		r := SubAgentResult{Phase: "sdd-apply", WhatDone: "done", ArtifactsPaths: "internal/sdd/synthesis.go", Risks: "None", NextRecommended: "verify"}
		for _, lang := range []string{"es", "en"} {
			out := RenderSynthesisLocalized(r, lang)
			if !strings.Contains(out, "## Sub-agent Result") {
				t.Errorf("lang %s missing header", lang)
			}
			if !strings.Contains(out, "**Artifacts/Paths:**") {
				t.Errorf("lang %s missing Artifacts marker", lang)
			}
			if !strings.Contains(out, "**Risks / Open Questions:**") {
				t.Errorf("lang %s missing Risks marker", lang)
			}
			if !HasSynthesis(out) {
				t.Errorf("lang %s failed HasSynthesis", lang)
			}
		}
	})
	t.Run("fallback empty lang defaults en", func(t *testing.T) {
		r := SubAgentResult{Phase: "sdd-apply", WhatDone: "done", ArtifactsPaths: "a/b", Risks: "x", NextRecommended: "y"}
		out := RenderSynthesisLocalized(r, "")
		if !HasSynthesis(out) {
			t.Errorf("empty lang fallback failed")
		}
	})
	t.Run("empty artifacts -> None", func(t *testing.T) {
		r := SubAgentResult{Phase: "phase", WhatDone: "", ArtifactsPaths: "", Risks: "", NextRecommended: ""}
		out := RenderSynthesisLocalized(r, "es")
		if !strings.Contains(out, "None") {
			t.Errorf("empty should render None, got %q", out)
		}
	})
	t.Run("hi -> en via DetectLanguage", func(t *testing.T) {
		if DetectLanguage("hi") != "en" {
			t.Errorf("hi should be en")
		}
	})
	t.Run("5-section order", func(t *testing.T) {
		r := SubAgentResult{Phase: "sdd-spec", WhatDone: "topic: decision", ArtifactsPaths: "a/b", Risks: "risk", NextRecommended: "verify", Preview: "preview", Diff: "diff"}
		out := RenderSynthesisLocalized(r, "es")
		idxWhat := strings.Index(out, "**What was done:**")
		if idxWhat == -1 {
			idxWhat = strings.Index(out, "| Topic | Decision |")
		}
		idxArts := strings.Index(out, "**Artifacts/Paths:**")
		idxRisks := strings.Index(out, "**Risks / Open Questions:**")
		idxNext := strings.Index(out, "**Next Recommended:**")
		if idxWhat == -1 || idxArts == -1 || idxRisks == -1 || idxNext == -1 {
			t.Fatalf("missing markers for order check")
		}
		if !(idxWhat < idxArts && idxArts < idxRisks && idxRisks < idxNext) {
			t.Errorf("5-section order broken: what=%d arts=%d risks=%d next=%d", idxWhat, idxArts, idxRisks, idxNext)
		}
	})
	t.Run("mixed last-turn-wins via DetectLanguage", func(t *testing.T) {
		if DetectLanguage("ok, continua con el spec") != "es" {
			t.Errorf("mixed last-turn should be es")
		}
		if DetectLanguage("ok, continue with spec") != "en" {
			t.Errorf("mixed last-turn should be en")
		}
	})
	t.Run("whitelist", func(t *testing.T) {
		r := SubAgentResult{Phase: "x", WhatDone: "done", ArtifactsPaths: "sdd/polish-synthesis-human-language/proposal", Risks: "None", NextRecommended: "verify"}
		out := RenderSynthesisLocalized(r, "es")
		if !strings.Contains(out, "sdd/polish-synthesis-human-language/proposal") {
			t.Errorf("whitelist sdd/ missing")
		}
	})
}

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
