package sdd

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	mustErr := func(q QuestionEnvelope, substr string) {
		t.Helper()
		err := ValidateQuestionEnvelope(q)
		if err == nil {
			t.Fatalf("expected error containing %q, got nil", substr)
		}
		m := err.Error()
		if !strings.Contains(m, "isError:true") {
			t.Errorf("expected isError:true, got %q", m)
		}
		if !strings.Contains(strings.ToLower(m), strings.ToLower(substr)) {
			t.Errorf("expected %q in %q", substr, m)
		}
	}
	t.Run("header 17", func(t *testing.T) {
		mustErr(QuestionEnvelope{Questions: []Question{{Header: strings.Repeat("a", 17), Question: "Q", Options: []QuestionOption{{Label: "a"}, {Label: "b"}}}}}, "header")
	})
	t.Run("label 61", func(t *testing.T) {
		mustErr(QuestionEnvelope{Questions: []Question{{Header: "h", Question: "Q", Options: []QuestionOption{{Label: strings.Repeat("b", 61)}, {Label: "ok"}}}}}, "label")
	})
	t.Run("5 questions", func(t *testing.T) {
		qs := make([]Question, 5)
		for i := range qs {
			qs[i] = Question{Header: "h", Question: "Q", Options: []QuestionOption{{Label: "a"}, {Label: "b"}}}
		}
		mustErr(QuestionEnvelope{Questions: qs}, "question")
	})
	t.Run("1 option", func(t *testing.T) {
		mustErr(QuestionEnvelope{Questions: []Question{{Header: "h", Question: "Q", Options: []QuestionOption{{Label: "only"}}}}}, "options")
	})
	t.Run("valid 12 60 3x3", func(t *testing.T) {
		h12 := strings.Repeat("c", 12)
		l60 := strings.Repeat("d", 60)
		qs := make([]Question, 3)
		for i := range qs {
			qs[i] = Question{Header: h12, Question: "Valid?", Options: []QuestionOption{{Label: "a", Description: "first choice"}, {Label: "b", Description: "second choice"}, {Label: l60[:30], Description: "third choice"}}}
		}
		qs[2].Options[2].Label = l60
		if err := ValidateQuestionEnvelope(QuestionEnvelope{Questions: qs}); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})
	t.Run("missing description", func(t *testing.T) {
		mustErr(QuestionEnvelope{Questions: []Question{{Header: "h", Question: "Q", Options: []QuestionOption{{Label: "a", Description: "has context"}, {Label: "b"}}}}}, "description")
	})
	t.Run("missing description top-level", func(t *testing.T) {
		mustErr(QuestionEnvelope{Options: []QuestionOption{{Label: "a", Description: "has context"}, {Label: "b"}}}, "description")
	})
	t.Run("preview in fallback", func(t *testing.T) {
		q := QuestionEnvelope{Questions: []Question{{Header: "H", Question: "Q?", Options: []QuestionOption{{Label: "a", Description: "pick a", Preview: "detail line"}}}}}
		fb := FormatFallback(q)
		if !strings.Contains(fb, "pick a") || !strings.Contains(fb, "detail line") {
			t.Errorf("fallback must keep description and preview: %q", fb)
		}
	})
	t.Run("fallback order", func(t *testing.T) {
		q := QuestionEnvelope{Questions: []Question{{Header: "Hdr1", Question: "Q1?", Options: []QuestionOption{{Label: "a"}, {Label: "b"}}}, {Header: "Hdr2", Question: "Q2?", Options: []QuestionOption{{Label: "c"}, {Label: "d"}}}}}
		fb := FormatFallback(q)
		if !strings.Contains(fb, "Hdr1") || !strings.Contains(fb, "Hdr2") || strings.Index(fb, "Hdr1") > strings.Index(fb, "Hdr2") {
			t.Errorf("fallback order incorrect: %q", fb)
		}
	})
}

// TestCheckpointOptionSubstance pins the D1 boundary table: the length floor
// AND the two-distinct-classes conjunction, bilingual (EN/ES).
func TestCheckpointOptionSubstance(t *testing.T) {
	tests := []struct {
		name      string
		desc      string
		want      bool
		wantRunes int // non-zero pins the fixture's rune-length property
	}{
		{name: "reject zero classes short en", desc: "yes, go ahead", want: false},
		{name: "reject zero classes short es", desc: "Sí, adelante", want: false},
		{name: "reject long zero classes", desc: "Adopt B — clearly better; we discussed it at length earlier", want: false},
		{name: "reject one class long", desc: "Run the tests and keep going as we agreed because it matters", want: false},
		{name: "accept floor 24 runes two classes", desc: "Edit 12 files; low risk!", want: true, wantRunes: 24},
		{name: "reject below floor 23 runes", desc: "Edit 2 files; low risk.", want: false, wantRunes: 23},
		{name: "accept scope risk unlock en", desc: "Revert one commit; unblocks slice 2.", want: true},
		{name: "accept scope risk unlock es", desc: "Revirte un commit; riesgo bajo; desbloquea slice 2.", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantRunes > 0 {
				if got := len([]rune(strings.TrimSpace(tt.desc))); got != tt.wantRunes {
					t.Fatalf("fixture %q measures %d runes, want %d (the property is the contract, not the characters)", tt.desc, got, tt.wantRunes)
				}
			}
			got, classes := CheckpointOptionSubstance(tt.desc)
			if got != tt.want {
				t.Errorf("CheckpointOptionSubstance(%q) = %v (matched %q), want %v", tt.desc, got, classes, tt.want)
			}
		})
	}
}

// TestValidateCheckpointSubstance pins the rule's scope: checkpoint envelopes
// reject thin options fail-closed (error wrapping the sentinel, naming option
// and question); context-bearing and non-checkpoint envelopes pass.
func TestValidateCheckpointSubstance(t *testing.T) {
	const rich = "Reapply the patch to the two files; low risk; unblocks slice 1b"
	thin := QuestionOption{Label: "Yes, go ahead", Description: "yes, go ahead"}
	context := QuestionOption{Label: "Adjust", Description: rich}
	checkpoint := func(opts ...QuestionOption) QuestionEnvelope {
		return QuestionEnvelope{Questions: []Question{{Header: "Decisión", Question: "Proceed with slice 1b?", Options: opts}}}
	}

	t.Run("thin option rejected wrapping sentinel", func(t *testing.T) {
		err := ValidateQuestionEnvelope(checkpoint(thin, context))
		if err == nil {
			t.Fatal("expected thin checkpoint option to be rejected")
		}
		if !errors.Is(err, ErrThinCheckpointOption) {
			t.Fatalf("expected error wrapping ErrThinCheckpointOption, got %v", err)
		}
		for _, want := range []string{`"Yes, go ahead"`, "question 1"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected %q in %q", want, err.Error())
			}
		}
	})

	t.Run("context bearing envelope passes", func(t *testing.T) {
		err := ValidateQuestionEnvelope(checkpoint(
			QuestionOption{Label: "Proceed", Description: rich},
			QuestionOption{Label: "Stop", Description: "Stop here; no file changes; defer the rest"},
		))
		if err != nil {
			t.Fatalf("expected context-bearing checkpoint envelope to pass, got %v", err)
		}
	})

	t.Run("non-checkpoint envelope unaffected", func(t *testing.T) {
		plain := QuestionEnvelope{Questions: []Question{{Header: "Color", Question: "Which color?", Options: []QuestionOption{thin, {Label: "Green", Description: "yes, go ahead"}}}}}
		if err := ValidateQuestionEnvelope(plain); err != nil {
			t.Fatalf("expected non-checkpoint envelope with terse options to pass, got %v", err)
		}
	})
}
