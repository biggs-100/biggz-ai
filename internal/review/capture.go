// Reviewer result capture — binding resolution, admission orchestration, and
// durable persistence into the content-addressed lineage event store.
//
// A capture appends one `lens_result` event (content-hash-named file, same
// pattern as every other event) whose payload holds the canonicalized reviewer
// payload plus a reference to the immutable changed-path manifest, which is
// persisted content-addressed under <lineage>/manifests/.
//
// The artifact slot (lens + order + expected revision) is immutable. Re-capture
// with the same binding and the same canonical payload returns the existing
// slot without appending anything (idempotent, mirroring gentle-ai's
// publish-no-replace). Re-capture with different canonical bytes is rejected.
package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/model"
)

// Event/artifact schema and layout constants.
const (
	LensResultOperation   = "lens_result"
	LensResultEventSchema = "biggz-ai.review-lens-result-event/v1"
	ManifestsDirName      = "manifests"
	ManifestFileSchema    = "biggz-ai.review-changed-path-manifest-file/v1"
	PreflightSchema       = "biggz-ai.review-capture-preflight/v1"
	ResultArtifactSchema  = "biggz-ai.review-result-artifact/v1"
	CapturedLensStatus    = "captured"
	captureResultRole     = "Reviewer"
	emptyTreeSHA          = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
)

// CapturedLens describes one admitted reviewer result for status output.
type CapturedLens struct {
	Lens          string `json:"lens"`
	SelectedOrder int    `json:"order"`
	SubjectHash   string `json:"subject_hash"`
	Status        string `json:"status"`
	ManifestPath  string `json:"manifest_path"`
}

// RepositoryContext is the optional provider-issued binding JSON accepted by
// `biggz review capture-result --repository-context`. Every identity field it
// carries must echo the corresponding flag; "repo" (or its negotiated-contract
// alias "repository") selects the repository root, and "project" echoes the
// repository basename. Unknown keys are rejected.
type RepositoryContext struct {
	Repo             string `json:"repo,omitempty"`
	Repository       string `json:"repository,omitempty"` // alias emitted by the negotiated contract envelope
	Project          string `json:"project,omitempty"`    // informational echo of the repository basename
	LineageID        string `json:"lineage_id,omitempty"`
	TargetIdentity   string `json:"target_identity,omitempty"`
	ExpectedRevision string `json:"expected_revision,omitempty"`
	Lens             string `json:"lens,omitempty"`
	Order            *int   `json:"order,omitempty"`
	SubjectHash      string `json:"subject_hash,omitempty"`
}

// DecodeRepositoryContext strictly decodes a repository-context JSON object.
func DecodeRepositoryContext(payload []byte) (RepositoryContext, error) {
	var context RepositoryContext
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&context); err != nil {
		return RepositoryContext{}, fmt.Errorf("repository context: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return RepositoryContext{}, errors.New("repository context contains multiple JSON values")
	}
	// The negotiated contract envelope emits "repository" (with "project");
	// resolve it to the canonical "repo" pin.
	if context.Repo == "" {
		context.Repo = context.Repository
	}
	return context, nil
}

// Validate verifies every identity field in the context echoes the binding.
func (c RepositoryContext) Validate(binding CaptureBinding) error {
	if err := c.validateLineageField(binding); err != nil {
		return err
	}
	if err := c.validateTargetField(binding); err != nil {
		return err
	}
	if err := c.validateRevisionField(binding); err != nil {
		return err
	}
	if err := c.validateLensField(binding); err != nil {
		return err
	}
	if err := c.validateOrderField(binding); err != nil {
		return err
	}
	if err := c.validateSubjectHashField(binding); err != nil {
		return err
	}
	if err := c.validateProjectField(); err != nil {
		return err
	}
	return nil
}

func (c RepositoryContext) validateLineageField(binding CaptureBinding) error {
	if c.LineageID != "" && c.LineageID != binding.LineageID {
		return fmt.Errorf("repository context lineage_id %q does not match --lineage %q", c.LineageID, binding.LineageID)
	}
	return nil
}

func (c RepositoryContext) validateTargetField(binding CaptureBinding) error {
	if c.TargetIdentity != "" && c.TargetIdentity != binding.TargetIdentity {
		return fmt.Errorf("repository context target_identity %q does not match --target %q", c.TargetIdentity, binding.TargetIdentity)
	}
	return nil
}

func (c RepositoryContext) validateRevisionField(binding CaptureBinding) error {
	if c.ExpectedRevision != "" && c.ExpectedRevision != binding.ExpectedRevision {
		return fmt.Errorf("repository context expected_revision %q does not match --expected-revision %q", c.ExpectedRevision, binding.ExpectedRevision)
	}
	return nil
}

func (c RepositoryContext) validateLensField(binding CaptureBinding) error {
	if c.Lens != "" && c.Lens != binding.Lens {
		return fmt.Errorf("repository context lens %q does not match --lens %q", c.Lens, binding.Lens)
	}
	return nil
}

func (c RepositoryContext) validateOrderField(binding CaptureBinding) error {
	if c.Order != nil && *c.Order != binding.Order {
		return fmt.Errorf("repository context order %d does not match --order %d", *c.Order, binding.Order)
	}
	return nil
}

func (c RepositoryContext) validateSubjectHashField(binding CaptureBinding) error {
	if c.SubjectHash != "" && c.SubjectHash != binding.SubjectHash {
		return fmt.Errorf("repository context subject_hash %q does not match --subject-hash %q", c.SubjectHash, binding.SubjectHash)
	}
	return nil
}

func (c RepositoryContext) validateProjectField() error {
	if c.Project != "" && c.Repo != "" && repositoryProjectOf(c.Repo) != c.Project {
		return fmt.Errorf("repository context project %q does not match repository %q", c.Project, c.Repo)
	}
	return nil
}

// CaptureBinding is the exact provider-owned capture binding derived from CLI
// flags. Repo is the git repository root; empty means the working directory.
type CaptureBinding struct {
	Repo             string
	LineageID        string
	TargetIdentity   string
	Lens             string
	Order            int
	ExpectedRevision string
	SubjectHash      string
}

func (b CaptureBinding) validate() error {
	if strings.TrimSpace(b.LineageID) == "" {
		return errors.New("capture binding: lineage_id is required")
	}
	if !validCommitSHA(b.TargetIdentity) {
		return errors.New("capture binding: target_identity must be a git object SHA")
	}
	if !isSupportedLens(b.Lens) {
		return fmt.Errorf("capture binding: unsupported lens %q", b.Lens)
	}
	if b.Order < 0 {
		return errors.New("capture binding: order must be zero or greater")
	}
	if !validSHA256Hex(b.ExpectedRevision) {
		return errors.New("capture binding: expected_revision must be a SHA-256 event revision")
	}
	return nil
}

// PreflightResult is the JSON emitted by `capture-result --preflight`: the
// artifact subject (base/candidate trees + ordered manifest) without any
// persistence.
type PreflightResult struct {
	Schema                    string                     `json:"schema"`
	LineageID                 string                     `json:"lineage_id"`
	TargetIdentity            string                     `json:"target_identity"`
	Lens                      string                     `json:"lens"`
	SelectedOrder             int                        `json:"selected_order"`
	ExpectedRevision          string                     `json:"expected_revision"`
	Subject                   ArtifactSubject            `json:"subject"`
	BaseTree                  string                     `json:"base_tree"`
	CandidateTree             string                     `json:"candidate_tree"`
	ChangedPathManifestSHA256 string                     `json:"changed_path_manifest_sha256"`
	ChangedPathManifest       []ChangedPathManifestEntry `json:"changed_path_manifest"`
}

// CapturedArtifact is the JSON emitted by a successful capture.
type CapturedArtifact struct {
	Schema            string            `json:"schema"`
	LineageID         string            `json:"lineage_id"`
	TargetIdentity    string            `json:"target_identity"`
	Lens              string            `json:"lens"`
	SelectedOrder     int               `json:"selected_order"`
	SubjectHash       string            `json:"subject_hash"`
	AdmissionDecision AdmissionDecision `json:"admission_decision"`
	Revision          string            `json:"revision"`
	Path              string            `json:"path"`
	CanonicalSHA256   string            `json:"canonical_sha256"`
	ResultHash        string            `json:"result_hash"`
	ManifestPath      string            `json:"manifest_path"`
}

// CaptureOutcome wraps a capture result. Idempotent is true when the slot
// already held the same canonical payload and nothing was appended.
type CaptureOutcome struct {
	Artifact   CapturedArtifact
	Idempotent bool
}

// lensResultEventPayload is the durable `lens_result` event payload: the
// canonicalized reviewer payload plus the manifest reference.
type lensResultEventPayload struct {
	Schema                    string             `json:"schema"`
	LineageID                 string             `json:"lineage_id"`
	ExpectedRevision          string             `json:"expected_revision"`
	SubjectHash               string             `json:"subject_hash"`
	Lens                      string             `json:"lens"`
	SelectedOrder             int                `json:"selected_order"`
	AdmissionDecision         AdmissionDecision  `json:"admission_decision"`
	Inspection                ArtifactInspection `json:"inspection"`
	CanonicalPayload          json.RawMessage    `json:"canonical_payload"`
	CanonicalPayloadSHA256    string             `json:"canonical_payload_sha256"`
	ResultHash                string             `json:"result_hash"`
	CandidateCausalFindingIDs []string           `json:"candidate_causal_finding_ids"`
	ManifestSHA256            string             `json:"manifest_sha256"`
	ManifestPath              string             `json:"manifest_path"`
	Result                    ArtifactLensResult `json:"result"`
}

// manifestFile is the persisted content-addressed changed-path manifest.
type manifestFile struct {
	Schema         string                     `json:"schema"`
	LineageID      string                     `json:"lineage_id"`
	BaseTree       string                     `json:"base_tree"`
	CandidateTree  string                     `json:"candidate_tree"`
	ManifestSHA256 string                     `json:"manifest_sha256"`
	Paths          []string                   `json:"paths"`
	Entries        []ChangedPathManifestEntry `json:"entries"`
}

// Preflight verifies the capture binding against the lineage authority and
// returns the artifact subject (base/candidate trees + ordered manifest)
// without persisting anything.
func Preflight(binding CaptureBinding) (*PreflightResult, error) {
	subject, entries, chain, err := resolveCaptureBinding(binding)
	if err != nil {
		return nil, err
	}
	if chain.HeadHash != binding.ExpectedRevision {
		return nil, fmt.Errorf("capture preflight: expected revision %s does not match the current head %s", binding.ExpectedRevision, chain.HeadHash)
	}
	return &PreflightResult{
		Schema: PreflightSchema, LineageID: subject.LineageID, TargetIdentity: subject.TargetIdentity,
		Lens: subject.Lens, SelectedOrder: subject.SelectedOrder, ExpectedRevision: subject.AuthorityRevision,
		Subject: subject, BaseTree: subject.BaseTree, CandidateTree: subject.CandidateTree,
		ChangedPathManifestSHA256: subject.ChangedPathManifestSHA256, ChangedPathManifest: entries,
	}, nil
}

// captureAdmission holds the admitted result ready for persistence.
type captureAdmission struct {
	canonical        ArtifactLensResult
	admission        ArtifactAdmission
	canonicalPayload json.RawMessage
}

func captureValidatePayload(rawPayload []byte) ([]byte, AdmissionDecision, error) {
	if err := validateReviewerResultPayload(rawPayload); err != nil {
		return nil, "", err
	}
	payload, decision, err := ExtractBoundedSingleJSONObject(rawPayload, ArtifactResultLimit)
	if err != nil {
		return nil, decision, fmt.Errorf("reviewer artifact admission %s: %w", decision, err)
	}
	return payload, decision, nil
}

func captureDecodeAndCheckLens(payload []byte, binding CaptureBinding) (ReviewerResult, error) {
	decoded, err := decodeReviewerResult(payload)
	if err != nil {
		return ReviewerResult{}, fmt.Errorf("decode reviewer result: %w", err)
	}
	if decoded.Lens != "" && decoded.Lens != binding.Lens {
		return ReviewerResult{}, fmt.Errorf("reviewer result lens %q does not match the selected lens %q", decoded.Lens, binding.Lens)
	}
	return decoded, nil
}

func captureRunAdmission(subject ArtifactSubject, entries []ChangedPathManifestEntry, binding CaptureBinding, payload []byte, decoded ReviewerResult) (captureAdmission, error) {
	canonical, admission, canonicalPayload, err := Admit(AdmissionRequest{
		ExpectedSubject:   subject,
		EchoedSubjectHash: decoded.SubjectHash,
		Inspection:        decoded.Inspection,
		Result:            ArtifactLensResult{Lens: binding.Lens, Findings: decoded.Findings, Evidence: decoded.Evidence},
		RawPayload:        payload,
		ManifestPaths:     ManifestPaths(entries),
	})
	if err != nil {
		return captureAdmission{}, err
	}
	return captureAdmission{canonical: canonical, admission: admission, canonicalPayload: canonicalPayload}, nil
}

func capturePrepareAdmission(binding CaptureBinding, subject ArtifactSubject, entries []ChangedPathManifestEntry, rawPayload []byte) (captureAdmission, error) {
	payload, _, err := captureValidatePayload(rawPayload)
	if err != nil {
		return captureAdmission{}, err
	}
	decoded, err := captureDecodeAndCheckLens(payload, binding)
	if err != nil {
		return captureAdmission{}, err
	}
	return captureRunAdmission(subject, entries, binding, payload, decoded)
}

func captureHandleOccupiedSlot(occupant *Record, binding CaptureBinding, subject ArtifactSubject, adm captureAdmission, store *Store) (CapturedArtifact, bool, error) {
	var existing lensResultEventPayload
	if err := json.Unmarshal(occupant.Payload, &existing); err != nil {
		return CapturedArtifact{}, false, fmt.Errorf("capture: existing lens result is malformed: %w", err)
	}
	if existing.Lens != binding.Lens || existing.SelectedOrder != binding.Order {
		return CapturedArtifact{}, false, fmt.Errorf("capture: review slot for revision %s is occupied by lens %q order %d", binding.ExpectedRevision, existing.Lens, existing.SelectedOrder)
	}
	if existing.SubjectHash != subject.SubjectHash || !bytes.Equal(existing.CanonicalPayload, adm.canonicalPayload) {
		return CapturedArtifact{}, false, fmt.Errorf("capture: captured reviewer result already exists with different canonical bytes; the slot for lens %q order %d at revision %s is immutable", binding.Lens, binding.Order, binding.ExpectedRevision)
	}
	artifact := buildCapturedArtifact(store, subject, adm.admission, occupantRevision(occupant), existing.ManifestPath)
	return artifact, true, nil
}

func captureAppendNewResult(fresh ValidatedChain, binding CaptureBinding, subject ArtifactSubject, entries []ChangedPathManifestEntry, adm captureAdmission, store *Store) (CapturedArtifact, error) {
	if fresh.HeadHash != binding.ExpectedRevision {
		return CapturedArtifact{}, fmt.Errorf("capture: expected revision %s does not match the current head %s", binding.ExpectedRevision, fresh.HeadHash)
	}
	manifestPath, err := writeManifestLocked(store, subject, entries)
	if err != nil {
		return CapturedArtifact{}, fmt.Errorf("capture: persist manifest: %w", err)
	}
	eventPayload, err := json.Marshal(lensResultEventPayload{
		Schema: LensResultEventSchema, LineageID: subject.LineageID, ExpectedRevision: binding.ExpectedRevision,
		SubjectHash: subject.SubjectHash, Lens: subject.Lens, SelectedOrder: subject.SelectedOrder,
		AdmissionDecision: adm.admission.Decision, Inspection: canonicalInspection(entries),
		CanonicalPayload: adm.canonicalPayload, CanonicalPayloadSHA256: adm.admission.CanonicalSHA256,
		ResultHash: adm.admission.ResultHash, CandidateCausalFindingIDs: adm.admission.CandidateCausalFindingIDs,
		ManifestSHA256: subject.ChangedPathManifestSHA256, ManifestPath: manifestPath,
		Result: adm.canonical,
	})
	if err != nil {
		return CapturedArtifact{}, fmt.Errorf("capture: marshal lens result event: %w", err)
	}
	revision, err := store.appendLocked(fresh.HeadHash, Record{
		Operation: LensResultOperation,
		Role:      captureResultRole,
		Actor:     captureResultRole,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Payload:   eventPayload,
	})
	if err != nil {
		return CapturedArtifact{}, fmt.Errorf("capture: append lens result event: %w", err)
	}
	return buildCapturedArtifact(store, subject, adm.admission, revision, manifestPath), nil
}

// Capture runs admission over the raw reviewer payload and, on success,
// persists the durable artifact slot (lens_result event) and manifest into
// the lineage event store.
func Capture(binding CaptureBinding, rawPayload []byte) (CaptureOutcome, error) {
	subject, entries, _, err := resolveCaptureBinding(binding)
	if err != nil {
		return CaptureOutcome{}, err
	}
	store, err := Open(binding.Repo, binding.LineageID)
	if err != nil {
		return CaptureOutcome{}, fmt.Errorf("capture: open store: %w", err)
	}
	adm, err := capturePrepareAdmission(binding, subject, entries, rawPayload)
	if err != nil {
		return CaptureOutcome{}, err
	}
	var artifact CapturedArtifact
	idempotent := false
	err = WithFileLock(store.Dir, func() error {
		fresh, err := store.LoadChain()
		if err != nil {
			return fmt.Errorf("capture: load chain: %w", err)
		}
		if occupant := findLensResultEvent(fresh, binding.ExpectedRevision); occupant != nil {
			a, done, err := captureHandleOccupiedSlot(occupant, binding, subject, adm, store)
			if err != nil {
				return err
			}
			artifact = a
			idempotent = done
			return nil
		}
		a, err := captureAppendNewResult(fresh, binding, subject, entries, adm, store)
		if err != nil {
			return err
		}
		artifact = a
		return nil
	})
	if err != nil {
		return CaptureOutcome{}, err
	}
	return CaptureOutcome{Artifact: artifact, Idempotent: idempotent}, nil
}

// canonicalInspection reconstructs the canonical inspection recorded on the
// admission: completed status with the canonical (sorted) manifest paths.
func canonicalInspection(entries []ChangedPathManifestEntry) ArtifactInspection {
	paths := ManifestPaths(entries)
	sort.Strings(paths)
	return ArtifactInspection{Status: ArtifactInspectionCompleted, Paths: paths}
}

// resolveCaptureBinding validates the binding, lineage chain, genesis subject,
// and derives the artifact subject (trees + manifest) from the repository.
func resolveCaptureBinding(binding CaptureBinding) (ArtifactSubject, []ChangedPathManifestEntry, ValidatedChain, error) {
	if err := binding.validate(); err != nil {
		return ArtifactSubject{}, nil, ValidatedChain{}, err
	}
	store, err := Open(binding.Repo, binding.LineageID)
	if err != nil {
		return ArtifactSubject{}, nil, ValidatedChain{}, fmt.Errorf("capture binding: %w", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		return ArtifactSubject{}, nil, ValidatedChain{}, fmt.Errorf("capture binding: load chain: %w", err)
	}
	if chain.Count == 0 {
		return ArtifactSubject{}, nil, ValidatedChain{}, errors.New("capture binding: lineage has no events")
	}
	verdict := store.Validate()
	if !verdict.Valid {
		return ArtifactSubject{}, nil, ValidatedChain{}, fmt.Errorf("capture binding: chain integrity failed: %s", verdict.Reason)
	}
	genesis := chain.Records[0]
	var subjectPayload model.ReviewSubject
	if err := json.Unmarshal(genesis.Payload, &subjectPayload); err != nil || strings.TrimSpace(subjectPayload.CommitSHA) == "" {
		return ArtifactSubject{}, nil, ValidatedChain{}, errors.New("capture binding: genesis event does not carry a review subject")
	}
	if subjectPayload.CommitSHA != binding.TargetIdentity {
		return ArtifactSubject{}, nil, ValidatedChain{}, fmt.Errorf(
			"capture binding: target identity %s does not match the lineage subject commit %s",
			binding.TargetIdentity, subjectPayload.CommitSHA)
	}
	baseTree, candidateTree, entries, err := candidateManifest(binding.Repo, subjectPayload.CommitSHA)
	if err != nil {
		return ArtifactSubject{}, nil, ValidatedChain{}, err
	}
	manifestDigest, err := ChangedPathManifestDigest(entries)
	if err != nil {
		return ArtifactSubject{}, nil, ValidatedChain{}, fmt.Errorf("capture binding: manifest digest: %w", err)
	}
	subject, err := NewArtifactSubject(binding.LineageID, binding.ExpectedRevision, binding.TargetIdentity,
		baseTree, candidateTree, manifestDigest, binding.Lens, binding.Order)
	if err != nil {
		return ArtifactSubject{}, nil, ValidatedChain{}, fmt.Errorf("capture binding: derive artifact subject: %w", err)
	}
	if binding.SubjectHash != "" && binding.SubjectHash != subject.SubjectHash {
		return ArtifactSubject{}, nil, ValidatedChain{}, errors.New("capture binding: subject hash does not match the provider-owned artifact subject")
	}
	return subject, entries, chain, nil
}

// candidateManifest derives the immutable base/candidate trees and the ordered
// changed-path manifest for a reviewed commit. The base tree is the parent
// tree; a root commit's base is git's empty tree.
func candidateManifest(repo, commitSHA string) (string, string, []ChangedPathManifestEntry, error) {
	repoArgs := func(args ...string) []string {
		if repo != "" {
			return append([]string{"-C", repo}, args...)
		}
		return args
	}
	candidate, err := gitOutput(exec.Command("git", repoArgs("rev-parse", commitSHA+"^{tree}")...))
	if err != nil {
		return "", "", nil, fmt.Errorf("capture binding: resolve candidate tree for %s: %w", commitSHA, err)
	}
	base, err := gitOutput(exec.Command("git", repoArgs("rev-parse", commitSHA+"^^{tree}")...))
	if err != nil {
		base = emptyTreeSHA
	}
	raw, err := gitOutput(exec.Command("git", repoArgs("diff", "--raw", "-z", "--no-renames",
		"--no-ext-diff", "--no-textconv", "--ignore-submodules=none", base, candidate, "--")...))
	if err != nil {
		return "", "", nil, fmt.Errorf("capture binding: render candidate manifest: %w", err)
	}
	entries, err := parseRawManifest([]byte(raw))
	if err != nil {
		return "", "", nil, fmt.Errorf("capture binding: %w", err)
	}
	return base, candidate, entries, nil
}

// parseRawManifest parses `git diff --raw -z` output into ordered manifest
// entries: ":<old-mode> <new-mode> <old-sha> <new-sha> <status>\0<path>\0".
func parseRawManifest(raw []byte) ([]ChangedPathManifestEntry, error) {
	entries := make([]ChangedPathManifestEntry, 0)
	fields := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	for index := 0; index+1 < len(fields); index += 2 {
		header := strings.SplitN(fields[index], " ", 5)
		if len(header) != 5 {
			return nil, fmt.Errorf("malformed raw diff header %q", fields[index])
		}
		pathValue := fields[index+1]
		if _, err := normalizeLogicalPath(pathValue); err != nil {
			return nil, fmt.Errorf("raw diff path %q is not canonical: %w", pathValue, err)
		}
		status := header[4]
		entries = append(entries, ChangedPathManifestEntry{
			Path: pathValue, Status: status,
			OldMode: strings.TrimPrefix(header[0], ":"), NewMode: header[1],
			Deleted: status == "D",
		})
	}
	if err := ValidateChangedPathManifest(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func gitOutput(cmd *exec.Cmd) (string, error) {
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// writeManifestLocked persists the content-addressed changed-path manifest
// under <lineage>/manifests/<digest-hex>.json. The caller holds the lineage
// file lock. Same content is a no-op; different content is an error.
func writeManifestLocked(store *Store, subject ArtifactSubject, entries []ChangedPathManifestEntry) (string, error) {
	file := manifestFile{
		Schema: ManifestFileSchema, LineageID: subject.LineageID,
		BaseTree: subject.BaseTree, CandidateTree: subject.CandidateTree,
		ManifestSHA256: subject.ChangedPathManifestSHA256,
		Paths:          ManifestPaths(entries), Entries: entries,
	}
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", err
	}
	digest := strings.TrimPrefix(subject.ChangedPathManifestSHA256, "sha256:")
	path := filepath.Join(store.Dir, ManifestsDirName, digest+".json")
	if err := publishNoReplace(path, payload); err != nil {
		return "", err
	}
	return filepath.Join(ManifestsDirName, digest+".json"), nil
}

// publishNoReplace writes a file atomically; an existing file with identical
// content is a no-op, differing content is an error.
func publishNoReplace(path string, payload []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(existing, payload) {
			return fmt.Errorf("publish no-replace: %s exists with different content", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// findLensResultEvent returns the lens_result event appended onto the given
// expected revision, if any. The append-only chain makes at most one event
// share any PrevRevision.
func findLensResultEvent(chain ValidatedChain, expectedRevision string) *Record {
	for index := range chain.Records {
		rec := &chain.Records[index]
		if rec.Operation == LensResultOperation && rec.PrevRevision == expectedRevision {
			return rec
		}
	}
	return nil
}

// occupantRevision recomputes an event's content hash — its file name — from
// its canonical record bytes.
func occupantRevision(rec *Record) string {
	payload, err := json.Marshal(rec)
	if err != nil {
		return ""
	}
	return sha256Hex(payload)
}

func buildCapturedArtifact(store *Store, subject ArtifactSubject, admission ArtifactAdmission, revision, manifestPath string) CapturedArtifact {
	// Canonical v1 layout: <store.Dir>/v1/events/<sha256>. Dual-read fallback
	// for legacy flat files is preserved in store.readRecord / Validate,
	// but the artifact Path must point to the canonical location so that
	// os.Stat succeeds when the event was published via store.publishImmutable.
	return CapturedArtifact{
		Schema: ResultArtifactSchema, LineageID: subject.LineageID, TargetIdentity: subject.TargetIdentity,
		Lens: subject.Lens, SelectedOrder: subject.SelectedOrder, SubjectHash: subject.SubjectHash,
		AdmissionDecision: admission.Decision, Revision: revision,
		Path: filepath.Join(store.Dir, "v1", "events", revision), CanonicalSHA256: admission.CanonicalSHA256,
		ResultHash: admission.ResultHash, ManifestPath: manifestPath,
	}
}

// CapturedLenses scans a chain for lens_result events and returns the captured
// lens summaries in selected-order. Captures superseded by a later
// dispose/reopen are discarded evidence and are not surfaced.
func CapturedLenses(chain ValidatedChain) []CapturedLens {
	lenses := make([]CapturedLens, 0)
	for index := range chain.Records {
		rec := &chain.Records[index]
		if rec.Operation != LensResultOperation {
			continue
		}
		var payload lensResultEventPayload
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			continue
		}
		if payload.AdmissionDecision != AdmissionCompleted {
			continue
		}
		if isSlotSuperseded(chain, index, payload.Lens, payload.SelectedOrder) {
			continue
		}
		lenses = append(lenses, CapturedLens{
			Lens: payload.Lens, SelectedOrder: payload.SelectedOrder,
			SubjectHash: payload.SubjectHash, Status: CapturedLensStatus,
			ManifestPath: payload.ManifestPath,
		})
	}
	sort.SliceStable(lenses, func(i, j int) bool {
		if lenses[i].SelectedOrder != lenses[j].SelectedOrder {
			return lenses[i].SelectedOrder < lenses[j].SelectedOrder
		}
		return lenses[i].Lens < lenses[j].Lens
	})
	return lenses
}
