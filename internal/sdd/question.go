package sdd

import (
	"fmt"
	"os"
	"strings"
)

type QuestionOption struct{ Label string `json:"label"`; Description string `json:"description,omitempty"` }
type Question struct{ Question string `json:"question"`; Header string `json:"header"`; Options []QuestionOption `json:"options"` }
type QuestionEnvelope struct{ Questions []Question `json:"questions"`; Options []QuestionOption `json:"options,omitempty"` }

const (
	maxHeaderLen = 16
	maxLabelLen  = 60
	maxQuestions = 4
	minOptions   = 2
	maxOptions   = 4
)

func IsCheckpointEnvelope(q QuestionEnvelope) bool {
	toks := []string{"proceed", "adjust", "stop", "continue", "correct"}
	hasTok := func(s string) bool {
		l := strings.ToLower(strings.TrimSpace(s))
		for _, t := range toks {
			if l == t || strings.Contains(l, t) {
				return true
			}
		}
		return false
	}
	for _, qu := range q.Questions {
		for _, o := range qu.Options {
			if hasTok(o.Label) {
				return true
			}
		}
	}
	for _, o := range q.Options {
		if hasTok(o.Label) {
			return true
		}
	}
	return false
}

func IsSubAgent() bool { return os.Getenv("PI_SUBAGENT_CHILD") == "1" }

func ValidateQuestionEnvelope(q QuestionEnvelope) error {
	if IsSubAgent() && IsCheckpointEnvelope(q) {
		return fmt.Errorf("isError:true checkpoint asks may only be emitted by orchestrator, not sub-agent (ownership)")
	}
	if len(q.Questions) > maxQuestions {
		return fmt.Errorf("isError:true questions exceed limit %d: got %d", maxQuestions, len(q.Questions))
	}
	if len(q.Questions) == 0 && len(q.Options) > 0 {
		return validateTopLevelOptions(q.Options)
	}
	for i, qu := range q.Questions {
		if err := validateSingleQuestion(i, qu); err != nil {
			return err
		}
	}
	return nil
}

func validateTopLevelOptions(options []QuestionOption) error {
	if len(options) < minOptions || len(options) > maxOptions {
		return fmt.Errorf("isError:true options out of range %d-%d: got %d", minOptions, maxOptions, len(options))
	}
	for _, o := range options {
		if err := validateOptionLabel(o.Label); err != nil {
			return err
		}
	}
	return nil
}

func validateSingleQuestion(index int, qu Question) error {
	if len([]rune(qu.Header)) > maxHeaderLen {
		return fmt.Errorf("isError:true header exceeds limit %d: got %d for question %d", maxHeaderLen, len([]rune(qu.Header)), index)
	}
	if len(qu.Options) < minOptions || len(qu.Options) > maxOptions {
		return fmt.Errorf("isError:true options out of range %d-%d: got %d for question %d", minOptions, maxOptions, len(qu.Options), index)
	}
	for _, o := range qu.Options {
		if err := validateOptionLabel(o.Label); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionLabel(label string) error {
	if len([]rune(label)) > maxLabelLen {
		return fmt.Errorf("isError:true label exceeds limit %d: got %d", maxLabelLen, len([]rune(label)))
	}
	return nil
}

func FormatFallback(q QuestionEnvelope) string {
	var b strings.Builder
	if len(q.Questions) == 0 && len(q.Options) > 0 {
		b.WriteString("## Questions\n\n")
		for _, o := range q.Options {
			b.WriteString("- " + o.Label)
			if strings.TrimSpace(o.Description) != "" {
				b.WriteString(": " + strings.TrimSpace(o.Description))
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	for i, qu := range q.Questions {
		h := strings.TrimSpace(qu.Header)
		qs := strings.TrimSpace(qu.Question)
		if h != "" {
			b.WriteString(fmt.Sprintf("### %s: Question %d\n", h, i+1))
		} else {
			b.WriteString(fmt.Sprintf("### Question %d\n", i+1))
		}
		if qs != "" {
			b.WriteString(qs + "\n")
		}
		for _, o := range qu.Options {
			b.WriteString("- " + o.Label)
			if strings.TrimSpace(o.Description) != "" {
				b.WriteString(": " + strings.TrimSpace(o.Description))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}
