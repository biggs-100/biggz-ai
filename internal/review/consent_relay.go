// Relayed consent for `review start` — the typed, non-interactive consent
// flow of Phase C1 review-workflow parity.
//
// A start with declared lenses is a medium-risk review that needs consent
// before a lineage is created. Instead of the dead-code console prompt
// (PromptConsent), start speaks a typed JSON envelope: `--consent relay`
// prints the question and exits 0 without persisting anything; the caller
// relays it to a human and reruns with `--consent granted` or
// `--consent declined` for the exact frozen candidate. A start with zero
// declared lenses is silent structural readback: no consent is asked.
package review

import (
	"errors"
	"fmt"
	"strings"

	"github.com/biggz-ai/biggz/model"
)

// ConsentModeSchema identifies the relayed consent envelope.
const ConsentModeSchema = "biggz-ai.review-consent/v1"

// ConsentMode is the caller's --consent declaration on review start.
// Empty means undeclared.
type ConsentMode string

const (
	ConsentModeRelay    ConsentMode = "relay"
	ConsentModeGranted  ConsentMode = "granted"
	ConsentModeDeclined ConsentMode = "declined"
)

// ReviewRisk is the risk tier used by the consent gate.
type ReviewRisk string

const (
	RiskLow    ReviewRisk = "low"
	RiskMedium ReviewRisk = "medium"
)

// ResolveReviewRisk proxies the risk tier from the declared lens selection:
// zero declared lenses is silent structural readback (low, no consent); any
// declared lens is a consolidated review (medium, consent required). This is
// the Phase A2 tier proxy adapted to the lenses the start already declares.
func ResolveReviewRisk(lenses []string) ReviewRisk {
	if len(lenses) == 0 {
		return RiskLow
	}
	return RiskMedium
}

// ConsentCandidateScope identifies the exact frozen candidate the consent
// decision applies to. A relayed answer only ever authorizes this candidate;
// later candidates must receive their own question.
type ConsentCandidateScope struct {
	Repository string     `json:"repository"`
	CommitSHA  string     `json:"commit_sha"`
	Lineage    string     `json:"lineage,omitempty"`
	Risk       ReviewRisk `json:"risk"`
	Lenses     []string   `json:"lenses"`
}

// ConsentChoice is one offered answer with the effect of choosing it.
type ConsentChoice struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Effect string `json:"effect"`
}

// ConsentEnvelope is the typed consent question `biggz review start
// --consent relay` prints. It is the response of that invocation: nothing is
// persisted, and the two runnable follow-up invocations are the choices.
type ConsentEnvelope struct {
	Schema       string                `json:"schema"`
	Headline     string                `json:"headline"`
	Reason       string                `json:"reason"`
	RiskEvidence []string              `json:"risk_evidence"`
	Candidate    ConsentCandidateScope `json:"candidate"`
	Choices      []ConsentChoice       `json:"choices"`
	OffPathNote  string                `json:"off_path_note"`
}

// Single wording sources for the envelope and the decline message, so the
// relayed question and the CLI output cannot drift.
const (
	reviewConsentHeadline     = "Biggz AI can review this change before you call it done."
	reviewConsentValue        = "Reviewing takes a bit longer, and it makes the result substantially safer."
	reviewConsentReasonMedium = "this change declares review lenses, so it gets one consolidated review."
	reviewConsentOffPathCmd   = "biggz rdd disable"
	reviewConsentOffPathNote  = "To turn reviews off for good, run '" + reviewConsentOffPathCmd + "'."

	reviewConsentGrantedLabel  = "Run the review now"
	reviewConsentGrantedEffect = "Starts the review for this candidate and creates its review lineage; later candidates are asked again."
	reviewConsentDeclinedLabel = "Not now, just this once"
	reviewConsentDeclinedEffect = "Skips the review for this exact candidate only; nothing is persisted, and the next candidate is asked again."
)

const reviewConsentDeclinedMessage = "Review skipped for this candidate at your request. Nothing was persisted; the next candidate is asked again."

// errReviewConsentDeclineWithoutQuestion mirrors gentle-ai: a low-risk
// candidate asks no consent question, so there is nothing to decline.
var errReviewConsentDeclineWithoutQuestion = errors.New("this low-risk candidate asks no consent question, so there is nothing to decline; rerun biggz review start without --consent")

// ConsentDecision tells review start what the consent gate resolved to.
type ConsentDecision struct {
	// Decision is one of "proceed", "relay", or "declined".
	Decision string
	// Envelope is set when Decision is "relay": it is the typed question.
	Envelope *ConsentEnvelope
	// Message is the scoped-decline confirmation when Decision is "declined".
	Message string
}

// EvaluateStartConsent resolves the consent gate for review start. It is
// pure: it never touches the event store, so a relay or decline cannot
// create a lineage.
//
//   - low risk (zero declared lenses): silent, no consent. A declined
//     declaration is refused: there is no question to answer.
//   - declared relay: returns the typed envelope (Decision "relay").
//   - declared granted: proceed.
//   - declared declined: scoped decline, nothing persisted.
//   - undeclared with lenses: interactive callers fall back to relay; a
//     non-interactive caller gets an error — a review needing consent must
//     never start silently.
func EvaluateStartConsent(subject model.ReviewSubject, lineageID string, lenses []string, declared string, interactive bool) (ConsentDecision, error) {
	mode := ConsentMode(strings.TrimSpace(declared))
	switch mode {
	case ConsentModeRelay, ConsentModeGranted, ConsentModeDeclined, "":
	default:
		return ConsentDecision{}, fmt.Errorf("invalid --consent mode %q (use relay, granted, or declined)", declared)
	}

	if ResolveReviewRisk(lenses) == RiskLow {
		if mode == ConsentModeDeclined {
			return ConsentDecision{}, errReviewConsentDeclineWithoutQuestion
		}
		return ConsentDecision{Decision: "proceed"}, nil
	}

	switch mode {
	case ConsentModeGranted:
		// The relayed answer authorizes only this exact frozen candidate;
		// later candidates must receive their own question.
		return ConsentDecision{Decision: "proceed"}, nil
	case ConsentModeDeclined:
		return ConsentDecision{Decision: "declined", Message: reviewConsentDeclinedMessage}, nil
	case ConsentModeRelay:
		return ConsentDecision{Decision: "relay", Envelope: buildConsentEnvelope(subject, lineageID, lenses)}, nil
	default:
		if interactive {
			// Interactive omission falls back to relay: the user still sees
			// the typed question and reruns with granted or declined. Nothing
			// is persisted.
			return ConsentDecision{Decision: "relay", Envelope: buildConsentEnvelope(subject, lineageID, lenses)}, nil
		}
		return ConsentDecision{}, errors.New("review needs consent: no --consent declared and no terminal to relay the question. Run 'biggz review start' with --consent relay to print the consent envelope, then rerun with --consent granted or --consent declined")
	}
}

func buildConsentEnvelope(subject model.ReviewSubject, lineageID string, lenses []string) *ConsentEnvelope {
	evidence := []string{"declared lenses: " + strings.Join(lenses, ", ")}
	return &ConsentEnvelope{
		Schema:   ConsentModeSchema,
		Headline: reviewConsentHeadline,
		Reason:   reviewConsentReasonMedium + " " + reviewConsentValue,
		RiskEvidence: evidence,
		Candidate: ConsentCandidateScope{
			Repository: subject.Repository,
			CommitSHA:  subject.CommitSHA,
			Lineage:    lineageID,
			Risk:       RiskMedium,
			Lenses:     lenses,
		},
		Choices: []ConsentChoice{
			{ID: string(ConsentModeGranted), Label: reviewConsentGrantedLabel, Effect: reviewConsentGrantedEffect},
			{ID: string(ConsentModeDeclined), Label: reviewConsentDeclinedLabel, Effect: reviewConsentDeclinedEffect},
		},
		OffPathNote: reviewConsentOffPathNote,
	}
}


