// Negotiated review-integration contract — the provider-owned, typed
// next-transition envelope (Phase D2 review-contract parity).
//
// `biggz review status <lineage> --contract biggz-ai.review-integration/v1
// --next-transition` prints ONLY this envelope. The orchestrator routes from
// it EXCLUSIVELY: no prose, no raw status fields, no eligibility. The envelope
// is derived from the same persisted bytes as the status `next_transition`
// (deriveNextTransition) and adds no machine surface: it is a presentation
// layer over the existing engine.
//
// The envelope names exactly one transition:
//
//   - collect  — one named capture input with its exact binding; subject_hash
//     is intentionally omitted and derived via `capture-result --preflight`.
//   - execute  — the exact operation with ordered argument tokens.
//   - stop     — halt and surface reason_code; gates are lifecycle operations
//     the orchestrator runs when the lifecycle demands.
package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/biggz-ai/biggz/model"
)

// ContractSchema identifies the negotiated review-integration envelope.
// biggz's FIRST contract, so it starts at v1 (gentle-ai's v2 is its second).
const ContractSchema = "biggz-ai.review-integration/v1"

// SupportedReviewContracts lists every contract schema the review CLI accepts.
var SupportedReviewContracts = []string{ContractSchema}

// ContractEnvelope is the ONLY routing authority of the negotiated flow. It
// deliberately carries no raw status field: there is nothing to interpret.
type ContractEnvelope struct {
	Schema         string                `json:"schema"`
	Lineage        string                `json:"lineage"`
	NextTransition NextTransitionEnvelope `json:"next_transition"`
}

// NextTransitionEnvelope is the typed routing decision. Exactly one of
// Operation (execute), Inputs (collect), or ReasonCode (stop) is set.
type NextTransitionEnvelope struct {
	Type       string `json:"type"` // collect | execute | stop
	Operation  string `json:"operation,omitempty"`
	Arguments  []string `json:"arguments,omitempty"`
	ReasonCode string `json:"reason_code,omitempty"`
	Inputs     map[string]ContractCaptureInput `json:"inputs,omitempty"`
}

// ContractCaptureInput is the exact provider-owned capture binding for one
// collect transition. subject_hash is intentionally omitted: the caller
// derives it with `capture-result --preflight` before the real capture.
type ContractCaptureInput struct {
	Lineage           string                     `json:"lineage"`
	Target            string                     `json:"target"`
	Lens              string                     `json:"lens"`
	Order             int                        `json:"order"`
	ExpectedRevision  string                     `json:"expected_revision"`
	RepositoryContext *ContractRepositoryContext `json:"repository_context,omitempty"`
}

// ContractRepositoryContext pins the repository for the capture invocation.
// Repository is the review subject's recorded repository (repo root or origin
// URL); Project is its basename echo.
type ContractRepositoryContext struct {
	Repository string `json:"repository"`
	Project    string `json:"project"`
}

// BuildNextTransition derives the negotiated envelope for a lineage from its
// persisted chain, the integrity verdict, and the RDD kill switch. A lineage
// with no events has nothing to route and errors: the contract mode never
// emits a transitionless envelope.
func (a *Authority) BuildNextTransition(lineageID string) (*ContractEnvelope, error) {
	store, err := a.Open(lineageID)
	if err != nil {
		return nil, fmt.Errorf("contract envelope: %w", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		return nil, fmt.Errorf("contract envelope: load chain: %w", err)
	}
	derived := deriveNextTransition(store, a.repo, chain, store.Validate())
	if derived == nil {
		return nil, errors.New("contract envelope: no next transition to route (the lineage has no review events)")
	}

	env := ContractEnvelope{Schema: ContractSchema, Lineage: lineageID}
	switch derived.Action {
	case "collect":
		if derived.Lens == "" || derived.Order == nil {
			return nil, errors.New("contract envelope: collect transition lacks lens or order")
		}
		var subject model.ReviewSubject
		if err := json.Unmarshal(chain.Records[0].Payload, &subject); err != nil || strings.TrimSpace(subject.CommitSHA) == "" {
			return nil, errors.New("contract envelope: collect requires a review subject genesis")
		}
		env.NextTransition = NextTransitionEnvelope{
			Type: "collect",
			Inputs: map[string]ContractCaptureInput{
				"capture": {
					Lineage: lineageID, Target: subject.CommitSHA, Lens: derived.Lens,
					Order: *derived.Order, ExpectedRevision: chain.HeadHash,
					RepositoryContext: contractRepositoryContext(a.repo, subject.Repository),
				},
			},
		}
	case "finalize":
		env.NextTransition = NextTransitionEnvelope{
			Type: "execute", Operation: "finalize", Arguments: []string{lineageID},
		}
	case "correction":
		env.NextTransition = NextTransitionEnvelope{
			Type: "execute", Operation: "resume",
			Arguments: []string{lineageID, "--correction-lines", strconv.Itoa(derived.BudgetRemaining)},
		}
	case "gate":
		// Gates are lifecycle operations the orchestrator runs when the
		// lifecycle demands; the finalized receipt is what they validate.
		env.NextTransition = NextTransitionEnvelope{Type: "stop", ReasonCode: "ready_for_gates"}
	case "stop":
		env.NextTransition = NextTransitionEnvelope{Type: "stop", ReasonCode: derived.Reason}
	default:
		return nil, fmt.Errorf("contract envelope: unsupported derived action %q", derived.Action)
	}
	return &env, nil
}

// contractRepositoryContext pins the repository for a collect input: the
// review subject's recorded repository (repo root or origin URL), falling
// back to the detected repository root. Both missing means no pin.
func contractRepositoryContext(storeRepo, subjectRepository string) *ContractRepositoryContext {
	repository := strings.TrimSpace(subjectRepository)
	if repository == "" {
		if root, err := gitIn(storeRepo, "rev-parse", "--show-toplevel"); err == nil && strings.TrimSpace(root) != "" {
			repository = strings.TrimSpace(root)
		}
	}
	if repository == "" {
		return nil
	}
	return &ContractRepositoryContext{Repository: repository, Project: repositoryProjectOf(repository)}
}

// repositoryProjectOf derives the repository basename, tolerant of both path
// separators so a local path and an origin URL name the same project.
func repositoryProjectOf(repository string) string {
	trimmed := strings.TrimRight(repository, "/\\")
	if idx := strings.LastIndexAny(trimmed, "/\\"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

// WithFollowUpInvocations extends a relayed consent envelope with the exact
// follow-up invocation for each choice: the orchestrator relays the envelope,
// gets the human's answer, and runs EXACTLY the one named invocation. Without
// a base the choices stay invocation-free, keeping the non-contract relay
// output byte-for-byte unchanged.
func (e *ConsentEnvelope) WithFollowUpInvocations(base string) *ConsentEnvelope {
	if e == nil || strings.TrimSpace(base) == "" {
		return e
	}
	for i := range e.Choices {
		e.Choices[i].Invocation = base + " --consent " + e.Choices[i].ID
	}
	return e
}
