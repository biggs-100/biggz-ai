package sdd

import (
	"testing"
)

func TestValidateBlockingPrompt_Valid(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Need user input to proceed",
		Questions: []BlockingQuestion{
			{
				ID:    "q1",
				Label: "Choose approach",
				Options: []BlockingOption{
					{Label: "Option A", Value: "a"},
					{Label: "Option B", Value: "b"},
				},
			},
		},
		SelectionMode: "single",
	}

	v := ValidateBlockingPrompt(env)
	if !v.Valid {
		t.Errorf("expected valid, got errors: %v", v.Errors)
	}
}

func TestValidateBlockingPrompt_MissingWhy(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Questions: []BlockingQuestion{
			{
				ID:    "q1",
				Label: "Choose",
				Options: []BlockingOption{
					{Label: "A", Value: "a"},
				},
			},
		},
	}

	v := ValidateBlockingPrompt(env)
	if v.Valid {
		t.Error("expected invalid for missing why")
	}
}

func TestValidateBlockingPrompt_NoQuestions(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Need input",
	}

	v := ValidateBlockingPrompt(env)
	if v.Valid {
		t.Error("expected invalid for no questions")
	}
}

func TestValidateBlockingPrompt_InvalidSelectionMode(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Need input",
		Questions: []BlockingQuestion{
			{
				ID:    "q1",
				Label: "Choose",
				Options: []BlockingOption{
					{Label: "A", Value: "a"},
				},
			},
		},
		SelectionMode: "invalid",
	}

	v := ValidateBlockingPrompt(env)
	if v.Valid {
		t.Error("expected invalid for invalid selection mode")
	}
}

func TestValidateBlockingPrompt_QuestionMissingID(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Need input",
		Questions: []BlockingQuestion{
			{
				Label: "Choose",
				Options: []BlockingOption{
					{Label: "A", Value: "a"},
				},
			},
		},
		SelectionMode: "single",
	}

	v := ValidateBlockingPrompt(env)
	if v.Valid {
		t.Error("expected invalid for question missing ID")
	}
}

func TestValidateBlockingPrompt_QuestionNoOptions(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Need input",
		Questions: []BlockingQuestion{
			{
				ID:    "q1",
				Label: "Choose",
			},
		},
		SelectionMode: "single",
	}

	v := ValidateBlockingPrompt(env)
	if v.Valid {
		t.Error("expected invalid for question with no options")
	}
}

func TestRenderBlockingPrompt(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Need user decision",
		Questions: []BlockingQuestion{
			{
				ID:    "approach",
				Label: "Which approach?",
				Options: []BlockingOption{
					{Label: "Fast", Value: "fast", Description: "Quick but risky"},
					{Label: "Safe", Value: "safe", Description: "Slow but reliable"},
				},
			},
		},
		SelectionMode: "single",
	}

	rendered := RenderBlockingPrompt(env)
	if rendered == "" {
		t.Error("expected non-empty rendered output")
	}
	if !contains(rendered, "Blocking Prompt") {
		t.Error("expected 'Blocking Prompt' in output")
	}
	if !contains(rendered, "Fast") {
		t.Error("expected 'Fast' option in output")
	}
}

func TestParseBlockingPrompt(t *testing.T) {
	jsonStr := `{
		"why": "Need input",
		"questions": [{
			"id": "q1",
			"label": "Choose",
			"options": [{"label": "A", "value": "a"}]
		}],
		"selection_mode": "single"
	}`

	env, err := ParseBlockingPrompt(jsonStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Why != "Need input" {
		t.Errorf("expected 'Need input', got %q", env.Why)
	}
}

func TestParseBlockingPrompt_Invalid(t *testing.T) {
	_, err := ParseBlockingPrompt("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMarshalBlockingPrompt(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Why: "Test",
		Questions: []BlockingQuestion{
			{
				ID:    "q1",
				Label: "Choose",
				Options: []BlockingOption{
					{Label: "A", Value: "a"},
				},
			},
		},
		SelectionMode: "single",
	}

	jsonStr, err := MarshalBlockingPrompt(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jsonStr == "" {
		t.Error("expected non-empty JSON")
	}
}

func TestValidateAnswer(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Questions: []BlockingQuestion{
			{
				ID:    "q1",
				Label: "Choose",
				Options: []BlockingOption{
					{Label: "Option A", Value: "a"},
					{Label: "Option B", Value: "b"},
				},
			},
		},
	}

	tests := []struct {
		questionID string
		answer     string
		valid      bool
	}{
		{"q1", "a", true},
		{"q1", "Option A", true},
		{"q1", "c", false},
		{"q2", "a", false},
	}

	for _, tt := range tests {
		if got := ValidateAnswer(env, tt.questionID, tt.answer); got != tt.valid {
			t.Errorf("ValidateAnswer(%s, %s) = %v, want %v", tt.questionID, tt.answer, got, tt.valid)
		}
	}
}

func TestBlockingPromptSummary(t *testing.T) {
	env := &BlockingPromptEnvelope{
		Questions: []BlockingQuestion{
			{
				ID:      "q1",
				Label:   "Choose",
				Options: []BlockingOption{{Label: "A"}, {Label: "B"}},
			},
			{
				ID:      "q2",
				Label:   "Confirm",
				Options: []BlockingOption{{Label: "Yes"}},
			},
		},
		SelectionMode: "single",
	}

	summary := BlockingPromptSummary(env)
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !contains(summary, "2 questions") {
		t.Error("expected '2 questions' in summary")
	}
	if !contains(summary, "3 options") {
		t.Error("expected '3 options' in summary")
	}
}
