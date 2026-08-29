package sdd

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/pathquote"
	"github.com/biggs-100/biggz-ai/internal/review"
)

// The status layer owns the change-instance identity that the ledger made
// the containment boundary: the ledger keys its chain by change name alone
// and archive never touches it, so the identity must live somewhere that IS
// the change instance — the change's own directory. The marker file travels
// with the directory into archive/ and a recreated change starts with a
// fresh directory, so the token is stable across one instance's life and
// distinct across recreations. Deriving the token from artifact content
// instead was rejected: a recreated change with byte-identical artifacts
// would inherit the archived change's authority, which is the resurrection
// hazard the instance boundary exists to close.
const changeInstanceMarkerFile = ".biggz-instance"

const (
	// EditAuthorityConsentSchema identifies the SDD edit-authority consent
	// envelope.
	EditAuthorityConsentSchema = "biggz-ai.edit-authority-consent/v1"

	// EditAuthorityContractV1 names the sdd-integration contract the
	// envelope belongs to.
	EditAuthorityContractV1 = "biggz-ai.sdd-integration/v1"

	sddConsentOperation      = "sdd-attempt.grant"
	sddConsentActionRequired = "consent_required"

	// sddConsentAnswerGranted and sddConsentAnswerDeclined are the machine
	// answer tokens, identical to the review family so orchestrators relay
	// one vocabulary.
	sddConsentAnswerGranted  = "granted"
	sddConsentAnswerDeclined = "declined"

	// sddConsentGrantActor and sddConsentGrantReason are the audit fields the
	// envelope's grant invocation carries. The consent conversation itself is
	// the authorization evidence; the ledger records who ran it and when.
	sddConsentGrantActor  = "maintainer"
	sddConsentGrantReason = "edit-authority-consent-granted"

	// sddConsentGrantInvocationPrefix fronts the real `biggz sdd-attempt
	// grant` verb: `biggz sdd-attempt grant <change>
	// [--expected-revision <rev>] --root <path>... --actor <actor>
	// --reason <reason> --request-id <id> --change-instance <token>`.
	sddConsentGrantInvocationPrefix = "biggz sdd-attempt grant "

	// sddConsentStatusInvocationPrefix is the decline and off-path re-entry:
	// declining persists nothing, so the runnable follow-up is native SDD
	// status for the same change.
	sddConsentStatusInvocationPrefix = "biggz sdd-status "

	// sddConsentGrantRequestDomain names the deterministic request-id
	// derivation domain.
	sddConsentGrantRequestDomain = "biggz-ai.sdd-consent-grant-request/v1"
)

// readChangeInstanceMarker loads the persisted change-instance token, or ""
// when none has been minted. It never mints: an ordinary status on a
// single-repository change must leave zero filesystem footprint.
func readChangeInstanceMarker(changeRoot string) (string, error) {
	payload, err := os.ReadFile(filepath.Join(changeRoot, changeInstanceMarkerFile))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read change-instance marker: %w", err)
	}
	return strings.TrimSpace(string(payload)), nil
}

// ensureChangeInstanceMarker returns the existing token or mints and
// persists a fresh opaque one. Minting happens only when the consent
// envelope needs a token to embed, so the marker exists exactly for changes
// that ever raised the edit-authority question.
func ensureChangeInstanceMarker(changeRoot string) (string, error) {
	existing, err := readChangeInstanceMarker(changeRoot)
	if err != nil || existing != "" {
		return existing, err
	}
	seed := make([]byte, 16)
	if _, err := rand.Read(seed); err != nil {
		return "", fmt.Errorf("mint change-instance identity: %w", err)
	}
	token := "sdd-" + hex.EncodeToString(seed)
	if err := os.WriteFile(filepath.Join(changeRoot, changeInstanceMarkerFile), []byte(token+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("persist change-instance marker: %w", err)
	}
	return token, nil
}

// sddConsentGrantRequestID derives the grant invocation's request-id from
// the exact inputs the grant would bind. Deterministic on purpose:
// re-rendering the same blocked status names the same request-id, so an
// accidentally repeated execution replays idempotently instead of
// double-granting, while a widening (different roots) or a moved ledger head
// (different expected revision) derives a fresh id.
func sddConsentGrantRequestID(change, instance, expectedRevision string, roots []string) string {
	hash := sha256.New()
	for _, part := range append([]string{sddConsentGrantRequestDomain, change, instance, expectedRevision}, roots...) {
		hash.Write([]byte(part))
		hash.Write([]byte{0})
	}
	return "grant-" + hex.EncodeToString(hash.Sum(nil))[:16]
}

// quotePath renders a filesystem path inside user-facing text and suggested
// invocations. It delegates to pathquote.Quote to preserve bytes exactly.
func quotePath(path string) string {
	return pathquote.Quote(path)
}

// EditAuthorityConsentResult is the typed blocking consent question a
// multi-repository SDD change raises when its task plan targets repository
// roots outside the authorized edit roots. It is a Lossless Blocking Prompt:
// WHY input is required, the COMPLETE choice set, and the EXACT runnable way
// to answer, scoped to one change. The identity block is the change name
// plus the missing roots. Granting is per-change and audited; declining
// persists nothing and the change stays blocked.
type EditAuthorityConsentResult struct {
	Schema    string `json:"schema"`
	Contract  string `json:"contract"`
	Operation string `json:"operation"`
	Action    string `json:"action"`
	// Blocking marks this envelope as a decision the caller must relay before
	// apply can proceed; nothing has been persisted.
	Blocking bool `json:"blocking"`
	// Change and MissingRoots are the identity block: which change is blocked
	// and which resolved repository roots no edit authority covers.
	Change       string   `json:"change"`
	MissingRoots []string `json:"missing_roots"`
	Headline     string   `json:"headline"`
	Reason       string   `json:"reason"`
	Value        string   `json:"value"`
	Evidence     []string `json:"evidence"`
	// Choices carries exactly the granted and declined choices in that order;
	// the granted invocation is the real grant verb, the declined invocation
	// is the status re-entry that shows the block again.
	Choices []review.ConsentChoice `json:"choices"`
	// OffPathNote/OffPathCommand document the deliberate alternative outside
	// the choice set: keep the change single-repository by editing its
	// tasks.md, then re-enter through native status.
	OffPathNote    string `json:"off_path_note"`
	OffPathCommand string `json:"off_path_command"`
}

// Validate enforces the envelope's SDD identity and the completeness half of
// the consent-envelope discipline: the envelope states why input is
// required, offers exactly the grant and decline choices in that order, every
// choice is answerable and runnable, the off path is documented, and the
// granted invocation carries every missing root it asks to authorize.
func (result EditAuthorityConsentResult) Validate() error {
	if err := validateConsentIdentity(result); err != nil {
		return err
	}
	if err := validateConsentChange(result); err != nil {
		return err
	}
	if err := validateConsentMissingRoots(result); err != nil {
		return err
	}
	if err := validateConsentContent(result); err != nil {
		return err
	}
	if err := validateConsentChoices(result); err != nil {
		return err
	}
	if err := validateConsentEvidence(result); err != nil {
		return err
	}
	if err := validateConsentInvocations(result); err != nil {
		return err
	}
	return nil
}

func validateConsentIdentity(result EditAuthorityConsentResult) error {
	if result.Schema != EditAuthorityConsentSchema || result.Contract != EditAuthorityContractV1 ||
		result.Operation != sddConsentOperation || result.Action != sddConsentActionRequired || !result.Blocking {
		return errors.New("invalid SDD consent question identity")
	}
	return nil
}

func validateConsentChange(result EditAuthorityConsentResult) error {
	if strings.TrimSpace(result.Change) == "" {
		return errors.New("SDD consent question requires the blocked change name")
	}
	return nil
}

func validateConsentMissingRoots(result EditAuthorityConsentResult) error {
	if len(result.MissingRoots) == 0 {
		return errors.New("SDD consent question requires the missing repository roots")
	}
	for _, root := range result.MissingRoots {
		if strings.TrimSpace(root) == "" {
			return errors.New("SDD consent question names a blank missing root")
		}
	}
	return nil
}

func validateConsentContent(result EditAuthorityConsentResult) error {
	if result.Headline == "" || result.Reason == "" || result.Value == "" || result.Evidence == nil {
		return errors.New("consent question must state why input is required")
	}
	if result.OffPathNote == "" || result.OffPathCommand == "" {
		return errors.New("consent question must document the deliberate off path")
	}
	return nil
}

func validateConsentChoices(result EditAuthorityConsentResult) error {
	if len(result.Choices) != 2 ||
		result.Choices[0].ID != sddConsentAnswerGranted ||
		result.Choices[1].ID != sddConsentAnswerDeclined {
		return errors.New("consent question requires exactly the granted and declined choices in that order")
	}
	for _, choice := range result.Choices {
		if choice.Label == "" || choice.Effect == "" {
			return fmt.Errorf("consent choice %q is incomplete", choice.ID)
		}
		if choice.Invocation == "" {
			return fmt.Errorf("consent choice %q does not name a runnable invocation", choice.ID)
		}
	}
	return nil
}

func validateConsentEvidence(result EditAuthorityConsentResult) error {
	for _, root := range result.MissingRoots {
		if !sddConsentEvidenceNames(result.Evidence, root) {
			return fmt.Errorf("SDD consent evidence does not name missing root %q", root)
		}
	}
	return nil
}

func validateConsentInvocations(result EditAuthorityConsentResult) error {
	granted := result.Choices[0]
	if !strings.HasPrefix(granted.Invocation, sddConsentGrantInvocationPrefix+result.Change+" ") ||
		!strings.Contains(granted.Invocation, " --root ") ||
		!strings.Contains(granted.Invocation, " --change-instance ") ||
		!strings.Contains(granted.Invocation, " --request-id ") ||
		!strings.Contains(granted.Invocation, " --actor ") ||
		!strings.Contains(granted.Invocation, " --reason ") {
		return errors.New("SDD consent grant choice does not name the runnable grant invocation")
	}
	for _, root := range result.MissingRoots {
		if !strings.Contains(granted.Invocation, " --root "+quotePath(root)) {
			return fmt.Errorf("SDD consent grant invocation does not carry missing root %q", root)
		}
	}
	declined := result.Choices[1]
	if !strings.HasPrefix(declined.Invocation, sddConsentStatusInvocationPrefix) ||
		!strings.Contains(declined.Invocation, result.Change) {
		return errors.New("SDD consent decline choice does not name the status re-entry for the blocked change")
	}
	if !strings.HasPrefix(result.OffPathCommand, sddConsentStatusInvocationPrefix) {
		return errors.New("SDD consent off path must re-enter through native status")
	}
	return nil
}

func sddConsentEvidenceNames(evidence []string, root string) bool {
	for _, entry := range evidence {
		if strings.Contains(entry, root) {
			return true
		}
	}
	return false
}

// newEditAuthorityConsent builds the typed blocking consent question for one
// blocked(edit_authority_missing) status: the missing roots are the
// evidence, the granted choice names the exact runnable grant invocation
// (including the persisted change-instance token and, when the ledger
// already has a head, the compare-and-swap revision a widening grant must
// chain on), and the declined choice re-enters through native status. The
// envelope satisfies Validate by construction.
func newEditAuthorityConsent(change, workspaceRoot string, missingRoots []string, instance, expectedRevision string) *EditAuthorityConsentResult {
	statusInvocation := sddConsentStatusInvocationPrefix + change
	evidence := make([]string, 0, len(missingRoots))
	for _, root := range missingRoots {
		evidence = append(evidence, fmt.Sprintf("%s is a Git repository root outside the authorized edit roots", root))
	}
	var grant strings.Builder
	fmt.Fprintf(&grant, "%s%s", sddConsentGrantInvocationPrefix, change)
	if expectedRevision != "" {
		fmt.Fprintf(&grant, " --expected-revision %s", expectedRevision)
	}
	for _, root := range missingRoots {
		fmt.Fprintf(&grant, " --root %s", quotePath(root))
	}
	fmt.Fprintf(&grant, " --actor %s --reason %s --request-id %s --change-instance %s",
		sddConsentGrantActor, sddConsentGrantReason,
		sddConsentGrantRequestID(change, instance, expectedRevision, missingRoots), instance)
	return &EditAuthorityConsentResult{
		Schema:       EditAuthorityConsentSchema,
		Contract:     EditAuthorityContractV1,
		Operation:    sddConsentOperation,
		Action:       sddConsentActionRequired,
		Blocking:     true,
		Change:       change,
		MissingRoots: append([]string{}, missingRoots...),
		Headline:     "This change plans work outside its authorized edit roots.",
		Reason:       "the task plan targets repository roots that no edit authority covers, so apply stays blocked until a human decides.",
		Value:        "Granting scopes edit authority to this change alone: the grant is recorded in the change's ledger, auditable, and dies with archive.",
		Evidence:     evidence,
		Choices: []review.ConsentChoice{
			{
				ID:         sddConsentAnswerGranted,
				Label:      "Grant this change edit authority over the named roots",
				Effect:     "This change's apply actor may edit the named repositories. The grant is per-change, audited (who, when, which roots), and dies with archive; nothing is granted to any other change.",
				Invocation: grant.String(),
			},
			{
				ID:         sddConsentAnswerDeclined,
				Label:      "Keep the change blocked",
				Effect:     "The change stays blocked(edit_authority_missing) and nothing is persisted. Both exits stay open: edit the change's tasks.md so no work unit targets an unauthorized root, or grant authority with the named grant invocation.",
				Invocation: statusInvocation,
			},
		},
		OffPathNote:    fmt.Sprintf("To keep this change single-repository instead, edit its tasks.md so no work unit targets an unauthorized root, then re-enter through '%s'.", statusInvocation),
		OffPathCommand: statusInvocation,
	}
}
