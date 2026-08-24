// Package review — Pi host relay contract (port of gentle-pi.review-relay/v1).
//
// biggz pi is adapter-only until the host declares the exact relay contract.
// The relay contract is a required conjunct: it can only narrow the compiled
// boundary, never expand it. Without the exact handshake, `review start
// --agent pi` refuses before any repository work and pi never appears as a
// suggested runtime.
//
// The host-mediated immutable relay keeps Go as the sole authority for prompt
// materialization, admission, budgets, receipts and gates. The pi launcher
// (biggz-pi / gentle-pi) reads the negotiated collection input, launches a
// brand-new print-mode pi subprocess in an empty scratch directory with every
// discovery surface disabled, forwards the Go-issued opaque prompt untouched,
// and returns the raw final bytes through the existing capture path.
package review

import (
	"errors"
	"os"
)

// PiReviewRelayContract is the exact relay contract this binary admits for
// the biggz-pi host. Keep gentle-pi.review-relay/v1 as a compat alias so a
// gentle-pi installation can still collect when the host exports the gentle
// contract.
const PiReviewRelayContract = "biggz-pi.review-relay/v1"

// GentlePiReviewRelayContract is the gentle alias kept for compat with a
// gentle-pi host that still exports gentle-pi.review-relay/v1.
const GentlePiReviewRelayContract = "gentle-pi.review-relay/v1"

// PiReviewRelayContractEnv is the Biggz host relay environment variable.
const PiReviewRelayContractEnv = "BIGGZ_PI_REVIEW_RELAY_CONTRACT"

// GentlePiReviewRelayContractEnv is the gentle host relay environment variable
// kept for compat (a gentle-pi host may export it).
const GentlePiReviewRelayContractEnv = "GENTLE_PI_REVIEW_RELAY_CONTRACT"

// ErrPiRelayHandshake is the typed refusal when pi is used without the relay
// handshake. Its text deliberately names ONLY the missing variable, not its
// value, so it survives the defect-report privacy gate byte-for-byte:
// that gate rewrites any KEY=VALUE token to <redacted> in full and any
// /-rooted run to <redacted> from the slash onward. Spelling the value out
// (e.g. BIGGZ_PI_REVIEW_RELAY_CONTRACT=biggz-pi.review-relay/v1 or the bare
// biggz-pi.review-relay/v1 token) collides with both rules, so the most
// actionable guidance that actually reaches the operator is the variable name
// alone. This mirrors gentle's reviewPiRelayHandshakeGuidance constraint.
var ErrPiRelayHandshake = errors.New("the active runtime is not eligible for immutable receipt review; pi is eligible only while BIGGZ_PI_REVIEW_RELAY_CONTRACT declares the exact relay contract this binary admits, which the biggz-pi host exports on every invocation it relays; export it in this shell and re-run")

// IsPiRelayAvailable reports whether the host relay handshake is declared.
// It accepts the exact contract value under either the Biggz or the gentle
// variable, in either direction for compat, but still rejects any other value
// or an empty declaration.
func IsPiRelayAvailable() bool {
	if os.Getenv(PiReviewRelayContractEnv) == PiReviewRelayContract {
		return true
	}
	if os.Getenv(GentlePiReviewRelayContractEnv) == GentlePiReviewRelayContract {
		return true
	}
	// Compat shims: a gentle-pi host exporting the Biggz contract under its
	// legacy variable, or a biggz-pi host re-exporting the gentle contract.
	if os.Getenv(GentlePiReviewRelayContractEnv) == PiReviewRelayContract {
		return true
	}
	if os.Getenv(PiReviewRelayContractEnv) == GentlePiReviewRelayContract {
		return true
	}
	return false
}

// PiRelayHandshakeGuidance returns the actionable guidance for a handshake-
// less pi refusal. It is the same string as ErrPiRelayHandshake so the
// privacy gate check is trivially: scrub(guidance) == guidance.
func PiRelayHandshakeGuidance() string {
	return ErrPiRelayHandshake.Error()
}

// ValidatePiAgent returns ErrPiRelayHandshake when agent is "pi" and the
// host relay handshake is absent. All other agents are always allowed here
// (their eligibility is decided elsewhere).
func ValidatePiAgent(agent string) error {
	if agent == "pi" && !IsPiRelayAvailable() {
		return ErrPiRelayHandshake
	}
	return nil
}
