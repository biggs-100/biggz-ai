// Package sdd — Lossless Blocking Prompts: preserve user-facing choice envelopes.
//
// When a sub-agent or tool returns a blocking prompt, the orchestrator MUST
// preserve its complete user-facing choice envelope:
//   - Why input is required
//   - Every group and question in original order
//   - Every option label and description
//   - The selection mode
//   - The exact allowed-answer domain
//
// Never summarize, abbreviate, reorder, relabel, merge, or omit choices.
// Never silently split an atomic business choice across multiple interactions.
package sdd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// BlockingPromptEnvelope represents a blocking prompt from a sub-agent.
type BlockingPromptEnvelope struct {
	// Schema identifies the envelope format.
	Schema string `json:"schema"`
	// Why explains why input is required.
	Why string `json:"why"`
	// Questions contains the questions to present.
	Questions []BlockingQuestion `json:"questions"`
	// SelectionMode is "single" or "multi".
	SelectionMode string `json:"selection_mode"`
	// AllowedAnswerDomain lists valid answers.
	AllowedAnswerDomain []string `json:"allowed_answer_domain,omitempty"`
}

// BlockingQuestion represents one question in a blocking prompt.
type BlockingQuestion struct {
	// ID is a unique identifier.
	ID string `json:"id"`
	// Label is the question text.
	Label string `json:"label"`
	// Description provides additional context.
	Description string `json:"description,omitempty"`
	// Options are the choices.
	Options []BlockingOption `json:"options"`
	// Required indicates if an answer is required.
	Required bool `json:"required"`
}

// BlockingOption represents one choice in a question.
type BlockingOption struct {
	// Label is the display text.
	Label string `json:"label"`
	// Value is the internal value.
	Value string `json:"value"`
	// Description explains the choice.
	Description string `json:"description,omitempty"`
	// Effect describes what happens if selected.
	Effect string `json:"effect,omitempty"`
}

// BlockingPromptValidation is the result of validating a blocking prompt.
type BlockingPromptValidation struct {
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateBlockingPrompt checks if a blocking prompt envelope is valid.
func ValidateBlockingPrompt(env *BlockingPromptEnvelope) *BlockingPromptValidation {
	v := &BlockingPromptValidation{Valid: true}

	// Check why
	if env.Why == "" {
		v.Errors = append(v.Errors, "missing 'why' explanation")
		v.Valid = false
	}

	// Check questions
	if len(env.Questions) == 0 {
		v.Errors = append(v.Errors, "no questions provided")
		v.Valid = false
	}

	// Check selection mode
	if env.SelectionMode == "" {
		v.Warnings = append(v.Warnings, "missing selection_mode, defaulting to 'single'")
		env.SelectionMode = "single"
	} else if env.SelectionMode != "single" && env.SelectionMode != "multi" {
		v.Errors = append(v.Errors, fmt.Sprintf("invalid selection_mode: %q (must be 'single' or 'multi')", env.SelectionMode))
		v.Valid = false
	}

	// Validate each question
	for i, q := range env.Questions {
		if q.ID == "" {
			v.Errors = append(v.Errors, fmt.Sprintf("question %d: missing ID", i))
			v.Valid = false
		}
		if q.Label == "" {
			v.Errors = append(v.Errors, fmt.Sprintf("question %d (%s): missing label", i, q.ID))
			v.Valid = false
		}
		if len(q.Options) == 0 {
			v.Errors = append(v.Errors, fmt.Sprintf("question %d (%s): no options", i, q.ID))
			v.Valid = false
		}
		for j, opt := range q.Options {
			if opt.Label == "" {
				v.Errors = append(v.Errors, fmt.Sprintf("question %d (%s), option %d: missing label", i, q.ID, j))
				v.Valid = false
			}
			if opt.Value == "" {
				v.Warnings = append(v.Warnings, fmt.Sprintf("question %d (%s), option %d: missing value, using label", i, q.ID, j))
				opt.Value = opt.Label
			}
		}
	}

	return v
}

// RenderBlockingPrompt renders a blocking prompt as plain text.
// This preserves the complete envelope for lossless relay.
func RenderBlockingPrompt(env *BlockingPromptEnvelope) string {
	var sb strings.Builder

	// Header
	sb.WriteString("## Blocking Prompt\n\n")
	sb.WriteString(fmt.Sprintf("**Why:** %s\n\n", env.Why))

	// Questions
	for i, q := range env.Questions {
		sb.WriteString(fmt.Sprintf("### Question %d: %s\n", i+1, q.Label))
		if q.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n", q.Description))
		}
		sb.WriteString("\n")

		// Options
		for j, opt := range q.Options {
			sb.WriteString(fmt.Sprintf("%d. **%s**", j+1, opt.Label))
			if opt.Description != "" {
				sb.WriteString(fmt.Sprintf(" — %s", opt.Description))
			}
			if opt.Effect != "" {
				sb.WriteString(fmt.Sprintf(" (Effect: %s)", opt.Effect))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Selection mode
	sb.WriteString(fmt.Sprintf("**Selection mode:** %s\n", env.SelectionMode))

	// Allowed answers
	if len(env.AllowedAnswerDomain) > 0 {
		sb.WriteString(fmt.Sprintf("**Allowed answers:** %s\n", strings.Join(env.AllowedAnswerDomain, ", ")))
	}

	return sb.String()
}

// ParseBlockingPrompt parses a JSON string into a BlockingPromptEnvelope.
func ParseBlockingPrompt(jsonStr string) (*BlockingPromptEnvelope, error) {
	var env BlockingPromptEnvelope
	if err := json.Unmarshal([]byte(jsonStr), &env); err != nil {
		return nil, fmt.Errorf("parse blocking prompt: %w", err)
	}
	return &env, nil
}

// MarshalBlockingPrompt marshals a BlockingPromptEnvelope to JSON.
func MarshalBlockingPrompt(env *BlockingPromptEnvelope) (string, error) {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal blocking prompt: %w", err)
	}
	return string(data), nil
}

// ValidateAnswer checks if an answer is valid for the given envelope.
func ValidateAnswer(env *BlockingPromptEnvelope, questionID, answer string) bool {
	// Check if question exists
	for _, q := range env.Questions {
		if q.ID == questionID {
			// Check if answer is a valid option
			for _, opt := range q.Options {
				if opt.Value == answer || opt.Label == answer {
					return true
				}
			}
			return false // Question exists but answer is invalid
		}
	}
	return false // Question not found
}

// BlockingPromptSummary returns a one-line summary for logging.
func BlockingPromptSummary(env *BlockingPromptEnvelope) string {
	questionCount := len(env.Questions)
	optionCount := 0
	for _, q := range env.Questions {
		optionCount += len(q.Options)
	}
	return fmt.Sprintf("◆ blocking prompt · %d questions, %d options (%s)", questionCount, optionCount, env.SelectionMode)
}
