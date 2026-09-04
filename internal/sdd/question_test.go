package sdd

import (
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
