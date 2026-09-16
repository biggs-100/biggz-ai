package sdd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	// Preview carries side-by-side detail (mockup, snippet, diff, config)
	// for options that need richer comparison than a description allows.
	// Rendered by the questionnaire UI when present; persisted verbatim
	// by FormatFallback.
	Preview string `json:"preview,omitempty"`
}
type Question struct {
	Question string           `json:"question"`
	Header   string           `json:"header"`
	Options  []QuestionOption `json:"options"`
}
type QuestionEnvelope struct {
	Questions []Question       `json:"questions"`
	Options   []QuestionOption `json:"options,omitempty"`
}

// Envelope limits: keep maxHeaderLen=16 (do not raise to 24 without migrating pendingMaxStoredBytes).
// Pretty Spanish headers must stay short: use "Decisión" (8) not "Decisión del checkpoint" (23),
// otherwise FormatFallback inherits the same limits and breaks biggz-ai.pending-question/v1 contract.
// If you raise the limit, update pending.go truncate bounds and tests together.
const (
	maxHeaderLen = 16 // header ≤16 runes, e.g. "Decisión" (8); not "Decisión del checkpoint" (23)
	maxLabelLen  = 60 // label ≤60 runes, e.g. "Continuar (Recomendado)" is safe
	maxQuestions = 4
	minOptions   = 2
	maxOptions   = 4
)

func IsCheckpointEnvelope(q QuestionEnvelope) bool {
	toks := []string{"proceed", "adjust", "stop", "continue", "correct", "continuar", "ajustar", "detener", "parar", "cerrar", "corregir", "proseguir"}
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
		if err := validateTopLevelOptions(q.Options); err != nil {
			return err
		}
	} else {
		for i, qu := range q.Questions {
			if err := validateSingleQuestion(i, qu); err != nil {
				return err
			}
		}
	}
	// Substance is scoped to checkpoint asks: non-checkpoint questions keep the
	// old behavior (spec: "non-checkpoint asks MUST remain unaffected").
	if IsCheckpointEnvelope(q) {
		return ValidateCheckpointSubstance(q)
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
		if err := validateOptionDescription(o); err != nil {
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
		if err := validateOptionDescription(o); err != nil {
			return fmt.Errorf("%w (question %d)", err, index)
		}
	}
	return nil
}

// Checkpoint option substance: a description must clear a length floor AND
// match at least two distinct decision-context classes. Each conjunct kills a
// different vacuous ask — long filler without signals, or a single buzzword
// without scope/effort/risk/unlock/deferral coverage. Bilingual (EN/ES),
// case-insensitive, word-boundary tokens.
const (
	minCheckpointOptionRunes    = 24
	minCheckpointContextClasses = 2
)

// ErrThinCheckpointOption marks a checkpoint option whose description carries
// no decision context; callers map it to the checkpoint_option_thin block.
var ErrThinCheckpointOption = errors.New("checkpoint_option_thin")

var checkpointContextClasses = []struct {
	name   string
	tokens *regexp.Regexp
}{
	{"scope", regexp.MustCompile(`(?i)\b(?:files?|paths?|commits?|archivos?|rutas?|repos?)\b`)},
	{"effort", regexp.MustCompile(`(?i)\b(?:mins?|minutes?|hours?|minutos?|horas?|tests?|pruebas?)\b`)},
	{"risk", regexp.MustCompile(`(?i)\b(?:risks?|revert\w*|rollbacks?|riesgos?|revirte)\b`)},
	{"unlock", regexp.MustCompile(`(?i)\b(?:unlocks?|unlocking|unblocks?|unblocking|ships?|shipping|desbloquea\w*|desbloquear)\b`)},
	{"deferral", regexp.MustCompile(`(?i)\b(?:defer\w*|later|blocked by|aplaza\w*|aplaz\w*)\b`)},
}

// CheckpointOptionSubstance reports whether a checkpoint option description
// carries decision context, and which classes matched.
func CheckpointOptionSubstance(description string) (bool, string) {
	if len([]rune(strings.TrimSpace(description))) < minCheckpointOptionRunes {
		return false, ""
	}
	var matched []string
	for _, c := range checkpointContextClasses {
		if c.tokens.MatchString(description) {
			matched = append(matched, c.name)
		}
	}
	return len(matched) >= minCheckpointContextClasses, strings.Join(matched, "+")
}

// ValidateCheckpointSubstance rejects, fail-closed, every checkpoint option
// whose description carries no decision context. Errors wrap
// ErrThinCheckpointOption and name the offending option and question.
func ValidateCheckpointSubstance(q QuestionEnvelope) error {
	for i, qu := range q.Questions {
		for _, o := range qu.Options {
			if ok, _ := CheckpointOptionSubstance(o.Description); !ok {
				return thinCheckpointOptionError(o, fmt.Sprintf("question %d", i+1))
			}
		}
	}
	for _, o := range q.Options {
		if ok, _ := CheckpointOptionSubstance(o.Description); !ok {
			return thinCheckpointOptionError(o, "top-level options")
		}
	}
	return nil
}

func thinCheckpointOptionError(o QuestionOption, where string) error {
	return fmt.Errorf("%w: option %q (%s) carries no decision context (needs ≥24 chars and ≥2 of scope/effort/risk/unlock/deferral signals); add scope, effort, risk, unlocks, deferral cost — or do more research before asking", ErrThinCheckpointOption, o.Label, where)
}

func validateOptionLabel(label string) error {
	if len([]rune(label)) > maxLabelLen {
		return fmt.Errorf("isError:true label exceeds limit %d: got %d", maxLabelLen, len([]rune(label)))
	}
	return nil
}

// validateOptionDescription enforces visible decision context: every choice
// must explain itself. A question whose options carry bare labels forces the
// human to decide blind (the questionnaire modal shows only the envelope),
// so context-free options are rejected fail-closed.
func validateOptionDescription(o QuestionOption) error {
	if strings.TrimSpace(o.Description) == "" {
		return fmt.Errorf("isError:true option %q missing description (every choice must carry visible decision context)", o.Label)
	}
	return nil
}

// writeFallbackOption renders one option with its description and, when
// present, its preview as indented lines so persisted fallbacks keep the
// full decision context the human saw.
func writeFallbackOption(b *strings.Builder, o QuestionOption) {
	b.WriteString("- " + o.Label)
	if strings.TrimSpace(o.Description) != "" {
		b.WriteString(": " + strings.TrimSpace(o.Description))
	}
	b.WriteString("\n")
	if strings.TrimSpace(o.Preview) != "" {
		for _, line := range strings.Split(o.Preview, "\n") {
			b.WriteString("  > " + line + "\n")
		}
	}
}

func FormatFallback(q QuestionEnvelope) string {
	var b strings.Builder
	if len(q.Questions) == 0 && len(q.Options) > 0 {
		b.WriteString("## Questions\n\n")
		for _, o := range q.Options {
			writeFallbackOption(&b, o)
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
			writeFallbackOption(&b, o)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String()) + "\n"
}
