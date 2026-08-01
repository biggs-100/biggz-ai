// Reviewer artifact capture — admission, subject, and canonicalization.
//
// This is biggz-ai's port of gentle-ai's reviewer artifact capture semantics
// (internal/reviewtransaction/artifact_admission.go + artifact_subject.go),
// adapted to the content-addressed event store. A reviewer result is admitted
// only when it echoes the provider-owned artifact subject, reports a completed
// inspection of every frozen manifest path, and carries well-formed findings.
package review

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Domain prefixes for content-addressed hashing. Each identity is
// SHA-256(domain + NUL + payload), prefixed with "sha256:".
const (
	ArtifactSubjectDomain       = "biggz-ai.review-artifact-subject/v1"
	ChangedPathManifestDomain   = "biggz-ai.review-changed-path-manifest/v1"
	LensResultDomain            = "biggz-ai.lens-result/v1"
	ArtifactAdmissionSchema     = "biggz-ai.review-artifact-admission/v1"
	ArtifactSubjectSchema       = "biggz-ai.review-artifact-subject/v1"
	ChangedPathManifestSchema   = "biggz-ai.review-changed-path-manifest/v1"
	ArtifactResultLimit         = 4 << 20
	ArtifactInspectionCompleted = "completed"
)

// SupportedLenses are biggz's native review lenses. Finding IDs are bound to
// their lens via a stable prefix (risk→R1, readability→R2, ...).
var lensIDPrefixes = map[string]string{
	"risk":         "R1",
	"readability":  "R2",
	"reliability":  "R3",
	"resilience":   "R4",
	"performance":  "R5",
	"dependencies": "R6",
}

func isSupportedLens(lens string) bool {
	_, ok := lensIDPrefixes[lens]
	return ok
}

func lensIDPrefix(lens string) string {
	return lensIDPrefixes[lens]
}

// EvidenceClass classifies how strongly a finding's evidence was established.
type EvidenceClass string

const (
	EvidenceDeterministic EvidenceClass = "deterministic"
	EvidenceInferential   EvidenceClass = "inferential"
	EvidenceInsufficient  EvidenceClass = "insufficient"
)

// CausalDisposition records where a finding's cause lives relative to the
// candidate. introduced/behavior-activated/worsened are the blocking classes
// (candidate-causal); pre-existing/base-only/unknown never block but are
// recorded on the admission.
type CausalDisposition string

const (
	CausalIntroduced        CausalDisposition = "introduced"
	CausalBehaviorActivated CausalDisposition = "behavior-activated"
	CausalWorsened          CausalDisposition = "worsened"
	CausalPreExisting       CausalDisposition = "pre-existing"
	CausalBaseOnly          CausalDisposition = "base-only"
	CausalUnknown           CausalDisposition = "unknown"
)

// Severities are normalized to uppercase during canonicalization. The
// supported set is BLOCKER, CRITICAL, WARNING, SUGGESTION.
func isSupportedSeverity(severity string) bool {
	switch severity {
	case "BLOCKER", "CRITICAL", "WARNING", "SUGGESTION":
		return true
	}
	return false
}

func isSevereSeverity(severity string) bool {
	return severity == "BLOCKER" || severity == "CRITICAL"
}

func isSupportedEvidenceClass(class EvidenceClass) bool {
	switch class {
	case EvidenceDeterministic, EvidenceInferential, EvidenceInsufficient:
		return true
	}
	return false
}

func isSupportedCausalDisposition(disposition CausalDisposition) bool {
	switch disposition {
	case CausalIntroduced, CausalBehaviorActivated, CausalWorsened,
		CausalPreExisting, CausalBaseOnly, CausalUnknown:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Artifact subject
// ---------------------------------------------------------------------------

// ArtifactSubject is the provider-owned identity of one reviewer execution.
// It binds the lineage, the expected authority revision (event chain head),
// the reviewed commit, its immutable base/candidate trees, the ordered path
// manifest digest, and the selected lens slot.
type ArtifactSubject struct {
	Schema                    string `json:"schema"`
	SubjectHash               string `json:"subject_hash"`
	LineageID                 string `json:"lineage_id"`
	AuthorityRevision         string `json:"authority_revision"`
	TargetIdentity            string `json:"target_identity"`
	BaseTree                  string `json:"base_tree"`
	CandidateTree             string `json:"candidate_tree"`
	ChangedPathManifestSHA256 string `json:"changed_path_manifest_sha256"`
	Lens                      string `json:"lens"`
	SelectedOrder             int    `json:"selected_order"`
}

// NewArtifactSubject derives one slot identity and its canonical self-hash.
func NewArtifactSubject(lineageID, authorityRevision, targetIdentity, baseTree, candidateTree, manifestSHA256, lens string, order int) (ArtifactSubject, error) {
	subject := ArtifactSubject{
		Schema: ArtifactSubjectSchema, LineageID: lineageID, AuthorityRevision: authorityRevision,
		TargetIdentity: targetIdentity, BaseTree: baseTree, CandidateTree: candidateTree,
		ChangedPathManifestSHA256: manifestSHA256, Lens: lens, SelectedOrder: order,
	}
	subject.SubjectHash = artifactSubjectHash(subject)
	return subject, ValidateArtifactSubject(subject)
}

// ValidateArtifactSubject verifies the canonical self-hash and every identity
// field without consulting mutable repository state.
func ValidateArtifactSubject(subject ArtifactSubject) error {
	if subject.Schema != ArtifactSubjectSchema {
		return errors.New("artifact subject schema is unsupported")
	}
	if strings.TrimSpace(subject.LineageID) == "" || strings.ContainsAny(subject.LineageID, "\x00\r\n") {
		return errors.New("artifact subject lineage identity is incomplete")
	}
	if !validSHA256Hex(subject.AuthorityRevision) || !validCommitSHA(subject.TargetIdentity) ||
		!validCommitSHA(subject.BaseTree) || !validCommitSHA(subject.CandidateTree) ||
		!validSHA256Identity(subject.ChangedPathManifestSHA256) {
		return errors.New("artifact subject identity is incomplete")
	}
	if !isSupportedLens(subject.Lens) || subject.SelectedOrder < 0 {
		return errors.New("artifact subject does not bind a selected lens slot")
	}
	wantHash := artifactSubjectHash(subject)
	if !validSHA256Identity(subject.SubjectHash) || subject.SubjectHash != wantHash {
		return errors.New("artifact subject hash does not match its binding")
	}
	return nil
}

func artifactSubjectHash(subject ArtifactSubject) string {
	preimage := struct {
		Schema                    string `json:"schema"`
		LineageID                 string `json:"lineage_id"`
		AuthorityRevision         string `json:"authority_revision"`
		TargetIdentity            string `json:"target_identity"`
		BaseTree                  string `json:"base_tree"`
		CandidateTree             string `json:"candidate_tree"`
		ChangedPathManifestSHA256 string `json:"changed_path_manifest_sha256"`
		Lens                      string `json:"lens"`
		SelectedOrder             int    `json:"selected_order"`
	}{
		Schema: subject.Schema, LineageID: subject.LineageID, AuthorityRevision: subject.AuthorityRevision,
		TargetIdentity: subject.TargetIdentity, BaseTree: subject.BaseTree, CandidateTree: subject.CandidateTree,
		ChangedPathManifestSHA256: subject.ChangedPathManifestSHA256, Lens: subject.Lens,
		SelectedOrder: subject.SelectedOrder,
	}
	payload, _ := json.Marshal(preimage)
	return domainHash(ArtifactSubjectDomain, payload)
}

// ---------------------------------------------------------------------------
// Changed-path manifest
// ---------------------------------------------------------------------------

// ChangedPathManifestEntry describes one path in a frozen candidate. Paths are
// repository-relative and entries retain the git tree-diff order.
type ChangedPathManifestEntry struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldMode string `json:"old_mode"`
	NewMode string `json:"new_mode"`
	Deleted bool   `json:"deleted"`
}

// ChangedPathManifestDigest returns the canonical provider-owned identity of
// an ordered immutable changed-path manifest.
func ChangedPathManifestDigest(entries []ChangedPathManifestEntry) (string, error) {
	if err := ValidateChangedPathManifest(entries); err != nil {
		return "", err
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return domainHash(ChangedPathManifestDomain, payload), nil
}

// ValidateChangedPathManifest verifies canonical, duplicate-free, ordered
// entries with supported statuses.
func ValidateChangedPathManifest(entries []ChangedPathManifestEntry) error {
	seen := make(map[string]struct{}, len(entries))
	for index, entry := range entries {
		canonical, err := normalizeLogicalPath(entry.Path)
		if err != nil || canonical != entry.Path {
			return fmt.Errorf("manifest entry[%d] path is not canonical: %q", index, entry.Path)
		}
		switch entry.Status {
		case "A", "D", "M", "T":
		default:
			return fmt.Errorf("manifest entry[%d] has unsupported status %q", index, entry.Status)
		}
		if entry.Deleted != (entry.Status == "D") {
			return fmt.Errorf("manifest entry[%d] deleted flag contradicts status", index)
		}
		if _, duplicate := seen[entry.Path]; duplicate {
			return fmt.Errorf("manifest contains duplicate path %q", entry.Path)
		}
		seen[entry.Path] = struct{}{}
	}
	return nil
}

// ManifestPaths returns the ordered path list of a manifest.
func ManifestPaths(entries []ChangedPathManifestEntry) []string {
	paths := make([]string, len(entries))
	for index, entry := range entries {
		paths[index] = entry.Path
	}
	return paths
}

// ---------------------------------------------------------------------------
// Reviewer result envelope
// ---------------------------------------------------------------------------

// ArtifactInspection is the reviewer's structured assertion that every path
// in the immutable manifest was actually inspected.
type ArtifactInspection struct {
	Status string   `json:"status"`
	Paths  []string `json:"paths"`
}

// ArtifactFinding is one structured observation in a reviewer result. Fields
// are canonicalized (trimmed, severity uppercased, ID/lens defaulted) during
// admission.
type ArtifactFinding struct {
	ID                string            `json:"id,omitempty"`
	Lens              string            `json:"lens,omitempty"`
	Location          string            `json:"location,omitempty"`
	Severity          string            `json:"severity,omitempty"`
	Claim             string            `json:"claim,omitempty"`
	ProofRefs         []string          `json:"proof_refs,omitempty"`
	EvidenceClass     EvidenceClass     `json:"evidence_class,omitempty"`
	CausalDisposition CausalDisposition `json:"causal_disposition,omitempty"`
}

// ArtifactLensResult is the validated canonical shape of one lens execution.
type ArtifactLensResult struct {
	Lens       string            `json:"lens"`
	Findings   []ArtifactFinding `json:"findings"`
	Evidence   []string          `json:"evidence"`
	ResultHash string            `json:"result_hash,omitempty"`
}

// ReviewerResult is the strict wire envelope a reviewer submits. Unknown
// fields are rejected during decode.
type ReviewerResult struct {
	SubjectHash string             `json:"subject_hash"`
	Inspection  ArtifactInspection `json:"inspection"`
	Lens        string             `json:"lens,omitempty"`
	Findings    []ArtifactFinding  `json:"findings"`
	Evidence    []string           `json:"evidence"`
}

// LensResultHash binds a complete canonical lens result to its lens and content.
func LensResultHash(result ArtifactLensResult) string {
	payload, _ := json.Marshal(struct {
		Lens     string            `json:"lens"`
		Findings []ArtifactFinding `json:"findings"`
		Evidence []string          `json:"evidence"`
	}{Lens: result.Lens, Findings: result.Findings, Evidence: result.Evidence})
	return domainHash(LensResultDomain, payload)
}

// CanonicalLensResult validates a lens result and derives its canonical form:
// trimmed fields, uppercased severity, defaulted finding IDs and lens binding.
func CanonicalLensResult(result ArtifactLensResult) (ArtifactLensResult, error) {
	result.Lens = strings.TrimSpace(result.Lens)
	if !isSupportedLens(result.Lens) {
		return ArtifactLensResult{}, fmt.Errorf("unknown review lens %q", result.Lens)
	}
	if result.Findings == nil || result.Evidence == nil || len(result.Evidence) == 0 {
		return ArtifactLensResult{}, errors.New("lens result requires explicit findings and concrete evidence")
	}
	idPrefix := lensIDPrefix(result.Lens)
	findings := make([]ArtifactFinding, len(result.Findings))
	for index, finding := range result.Findings {
		finding.ID = strings.TrimSpace(finding.ID)
		if finding.ID == "" {
			finding.ID = fmt.Sprintf("%s-%03d", idPrefix, index+1)
		}
		finding.Lens = strings.TrimSpace(finding.Lens)
		if finding.Lens == "" {
			finding.Lens = result.Lens
		}
		finding.Location = strings.TrimSpace(finding.Location)
		finding.Severity = strings.ToUpper(strings.TrimSpace(finding.Severity))
		finding.Claim = strings.TrimSpace(finding.Claim)
		if finding.EvidenceClass != "" && !isSupportedEvidenceClass(finding.EvidenceClass) {
			return ArtifactLensResult{}, fmt.Errorf("lens result finding[%d] has unsupported evidence class %q", index, finding.EvidenceClass)
		}
		if finding.CausalDisposition != "" && !isSupportedCausalDisposition(finding.CausalDisposition) {
			return ArtifactLensResult{}, fmt.Errorf("lens result finding[%d] has unsupported causal disposition %q", index, finding.CausalDisposition)
		}
		finding.ProofRefs = append([]string(nil), finding.ProofRefs...)
		for proofIndex := range finding.ProofRefs {
			finding.ProofRefs[proofIndex] = strings.TrimSpace(finding.ProofRefs[proofIndex])
		}
		if err := validateLensFinding(finding); err != nil {
			return ArtifactLensResult{}, fmt.Errorf("lens result finding[%d]: %w", index, err)
		}
		if finding.Lens != result.Lens {
			return ArtifactLensResult{}, fmt.Errorf("lens result finding[%d] is not bound to %q", index, result.Lens)
		}
		findings[index] = finding
	}
	evidence := make([]string, len(result.Evidence))
	for index, item := range result.Evidence {
		item = strings.TrimSpace(item)
		if !isConcreteEvidence(item) {
			return ArtifactLensResult{}, fmt.Errorf("lens result evidence[%d] must be concrete", index)
		}
		evidence[index] = item
	}
	result.Findings = findings
	result.Evidence = evidence
	derived := LensResultHash(result)
	if result.ResultHash != "" && result.ResultHash != derived {
		return ArtifactLensResult{}, errors.New("lens result_hash does not match canonical structured result content")
	}
	result.ResultHash = derived
	return result, nil
}

func validateLensFinding(finding ArtifactFinding) error {
	if !isSupportedSeverity(finding.Severity) {
		return fmt.Errorf("unsupported severity %q", finding.Severity)
	}
	if strings.TrimSpace(finding.Claim) == "" {
		return errors.New("finding claim is required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Admission
// ---------------------------------------------------------------------------

// AdmissionDecision is the stable provider-owned outcome of one admission.
type AdmissionDecision string

const (
	AdmissionCompleted       AdmissionDecision = "completed"
	AdmissionIncomplete      AdmissionDecision = "incomplete"
	AdmissionAmbiguous       AdmissionDecision = "ambiguous"
	AdmissionOutOfScope      AdmissionDecision = "out_of_scope"
	AdmissionBindingMismatch AdmissionDecision = "binding_mismatch"
)

// ArtifactAdmission records the provider's decision and exact raw/canonical
// payload identities. Only completed records are reviewer results.
type ArtifactAdmission struct {
	Schema                    string            `json:"schema"`
	Decision                  AdmissionDecision `json:"decision"`
	SubjectHash               string            `json:"subject_hash"`
	RawSHA256                 string            `json:"raw_sha256"`
	CanonicalSHA256           string            `json:"canonical_sha256"`
	ResultHash                string            `json:"result_hash,omitempty"`
	CandidateCausalFindingIDs []string          `json:"candidate_causal_finding_ids"`
	Diagnostic                string            `json:"diagnostic,omitempty"`
}

// ArtifactAdmissionError exposes the stable native decision without requiring
// callers to parse diagnostic prose.
type ArtifactAdmissionError struct {
	Admission  ArtifactAdmission
	Diagnostic string
	cause      error
}

func (err *ArtifactAdmissionError) Error() string {
	message := fmt.Sprintf("reviewer artifact admission %s: %s", err.Admission.Decision, err.Admission.Diagnostic)
	if err.Diagnostic != "" && err.Diagnostic != err.Admission.Diagnostic {
		message += "; diagnostic=" + err.Diagnostic
	}
	return message
}

func (err *ArtifactAdmissionError) Unwrap() error { return err.cause }

// AdmissionRequest carries everything the provider knows about one capture.
type AdmissionRequest struct {
	ExpectedSubject   ArtifactSubject
	EchoedSubjectHash string
	Inspection        ArtifactInspection
	Result            ArtifactLensResult
	ManifestPaths     []string
	RawPayload        []byte
}

// Admit performs the single provider-owned admission decision: subject echo,
// completed full-manifest inspection, result shape, and severe-finding
// evidence/disposition requirements. It returns the canonical lens result,
// the admission record, and the canonical payload bytes that admission hashed.
func Admit(request AdmissionRequest) (ArtifactLensResult, ArtifactAdmission, []byte, error) {
	admission := ArtifactAdmission{
		Schema:      ArtifactAdmissionSchema,
		SubjectHash: request.ExpectedSubject.SubjectHash,
		RawSHA256:   payloadSHA256(request.RawPayload),
	}
	fail := func(decision AdmissionDecision, diagnostic string) (ArtifactLensResult, ArtifactAdmission, []byte, error) {
		admission.Decision, admission.Diagnostic = decision, diagnostic
		return ArtifactLensResult{}, admission, nil, &ArtifactAdmissionError{Admission: admission}
	}
	if err := ValidateArtifactSubject(request.ExpectedSubject); err != nil {
		return fail(AdmissionBindingMismatch, err.Error())
	}
	if len(request.RawPayload) == 0 {
		return fail(AdmissionIncomplete, "raw reviewer payload is required")
	}
	if request.EchoedSubjectHash == "" {
		return fail(AdmissionIncomplete,
			"reviewer result omitted the provider-owned artifact subject hash; re-run the lens and invoke biggz review capture-result again on the same lineage with a result that echoes the binding's subject_hash")
	}
	if request.EchoedSubjectHash != request.ExpectedSubject.SubjectHash {
		return fail(AdmissionBindingMismatch,
			"reviewer result echoed a different artifact subject; re-run the lens and invoke biggz review capture-result again with a result that echoes subject_hash "+request.ExpectedSubject.SubjectHash)
	}
	if request.Inspection.Status != ArtifactInspectionCompleted {
		return fail(AdmissionIncomplete, "reviewer did not report completed candidate inspection")
	}
	inspectionPaths, err := canonicalPaths(request.Inspection.Paths)
	if err != nil || !equalStrings(inspectionPaths, request.Inspection.Paths) {
		return fail(AdmissionOutOfScope, "reviewer inspection paths are not canonical candidate paths")
	}
	allowed := make(map[string]struct{}, len(request.ManifestPaths))
	for _, logicalPath := range request.ManifestPaths {
		allowed[logicalPath] = struct{}{}
	}
	for _, logicalPath := range inspectionPaths {
		if _, ok := allowed[logicalPath]; !ok {
			return fail(AdmissionOutOfScope, "reviewer inspection includes a path outside the frozen candidate")
		}
	}
	if !equalStrings(inspectionPaths, request.ManifestPaths) {
		return fail(AdmissionIncomplete, "reviewer inspection did not cover the complete frozen path manifest")
	}
	canonical, err := CanonicalLensResult(request.Result)
	if err != nil {
		return fail(AdmissionIncomplete, err.Error())
	}
	for _, evidence := range canonical.Evidence {
		if evidenceReportsUnavailableInspection(evidence) {
			return fail(AdmissionIncomplete, "reviewer evidence reports that candidate inspection was unavailable")
		}
	}
	wantPrefix := lensIDPrefix(canonical.Lens) + "-"
	seenFindingIDs := make(map[string]struct{}, len(canonical.Findings))
	wantCandidateCausalIDs := make([]string, 0)
	for _, finding := range canonical.Findings {
		if !artifactFindingID.MatchString(finding.ID) {
			return fail(AdmissionBindingMismatch, "reviewer finding ID does not match the native ASCII schema")
		}
		if !strings.HasPrefix(finding.ID, wantPrefix) {
			return fail(AdmissionBindingMismatch, "reviewer finding ID is not bound to the selected lens")
		}
		if _, duplicate := seenFindingIDs[finding.ID]; duplicate {
			return fail(AdmissionAmbiguous, "reviewer result repeats a finding ID")
		}
		seenFindingIDs[finding.ID] = struct{}{}
		logicalPath, _, locationErr := parseFindingLocation(finding.Location)
		if locationErr != nil {
			return fail(AdmissionOutOfScope, "reviewer finding location is invalid: "+locationErr.Error())
		}
		if _, ok := allowed[logicalPath]; !ok {
			return fail(AdmissionOutOfScope, "reviewer finding location is outside the frozen candidate")
		}
		if !isSevereSeverity(finding.Severity) {
			continue
		}
		if !isSupportedEvidenceClass(finding.EvidenceClass) || !isSupportedCausalDisposition(finding.CausalDisposition) {
			return fail(AdmissionIncomplete, "severe reviewer finding requires supported evidence_class and causal_disposition")
		}
		switch finding.CausalDisposition {
		case CausalIntroduced, CausalBehaviorActivated, CausalWorsened:
			wantCandidateCausalIDs = append(wantCandidateCausalIDs, finding.ID)
		}
	}
	verifiedIDs, err := canonicalStrings(wantCandidateCausalIDs, "candidate-causal finding id")
	if err != nil {
		return fail(AdmissionIncomplete, err.Error())
	}
	canonicalPayload, err := marshalCanonicalEnvelope(request.EchoedSubjectHash,
		ArtifactInspection{Status: ArtifactInspectionCompleted, Paths: inspectionPaths}, canonical)
	if err != nil {
		return fail(AdmissionIncomplete, err.Error())
	}
	admission.Decision = AdmissionCompleted
	admission.ResultHash = canonical.ResultHash
	admission.CanonicalSHA256 = payloadSHA256(canonicalPayload)
	admission.CandidateCausalFindingIDs = verifiedIDs
	return canonical, admission, canonicalPayload, nil
}

// marshalCanonicalEnvelope renders the deterministic canonical payload: stable
// field order, canonical inspection paths, and the canonical lens result.
func marshalCanonicalEnvelope(subjectHash string, inspection ArtifactInspection, result ArtifactLensResult) ([]byte, error) {
	envelope := struct {
		SubjectHash string             `json:"subject_hash"`
		Inspection  ArtifactInspection `json:"inspection"`
		Lens        string             `json:"lens"`
		Findings    []ArtifactFinding  `json:"findings"`
		Evidence    []string           `json:"evidence"`
	}{
		SubjectHash: subjectHash, Inspection: inspection, Lens: result.Lens,
		Findings: result.Findings, Evidence: result.Evidence,
	}
	return json.Marshal(envelope)
}

var artifactFindingID = regexp.MustCompile(`^R[1-6]-[A-Za-z0-9][A-Za-z0-9._-]*$`)

// ---------------------------------------------------------------------------
// Payload transport validation
// ---------------------------------------------------------------------------

// ReviewerResultPayloadError is returned when a raw reviewer result payload is
// structurally invalid before JSON decoding is attempted.
type ReviewerResultPayloadError struct {
	Code    string
	Message string
}

func (e *ReviewerResultPayloadError) Error() string { return e.Message }

// validateReviewerResultPayload inspects the raw bytes of a reviewer result
// before JSON decoding. It rejects two structurally distinct failure modes:
//  1. empty_result: the task completed but produced no reviewer output.
//  2. nested_envelope: the reviewer output was not extracted from its XML
//     task wrapper before being passed as the strict JSON payload.
func validateReviewerResultPayload(payload []byte) error {
	if len(bytes.TrimSpace(payload)) == 0 {
		return &ReviewerResultPayloadError{
			Code:    "empty_result",
			Message: "reviewer result payload is empty or whitespace-only: the task may have completed without producing output",
		}
	}
	if bytes.Contains(payload, []byte("<task_result>")) || bytes.Contains(payload, []byte("</task_result>")) {
		if !json.Valid(payload) {
			return &ReviewerResultPayloadError{
				Code:    "nested_envelope",
				Message: "reviewer result payload contains a raw XML task envelope: extract the strict JSON reviewer output from <task_result> before capture",
			}
		}
	}
	return nil
}

// ExtractBoundedSingleJSONObject accepts transport prose around exactly one
// unambiguous JSON object. Multiple objects, an unterminated object, or a
// payload outside the caller's bound fail closed.
func ExtractBoundedSingleJSONObject(payload []byte, limit int) ([]byte, AdmissionDecision, error) {
	if limit <= 0 || len(payload) == 0 || len(payload) > limit {
		return nil, AdmissionIncomplete, errors.New("reviewer payload is empty or exceeds the native bound")
	}
	type candidate struct{ start, end int }
	candidates := []candidate{}
	start, depth := -1, 0
	inString, escaped := false, false
	for index, value := range payload {
		if depth == 0 {
			if value == '{' {
				start, depth, inString, escaped = index, 1, false, false
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch value {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var object map[string]json.RawMessage
				fragment := bytes.TrimSpace(payload[start : index+1])
				if json.Unmarshal(fragment, &object) == nil && object != nil {
					candidates = append(candidates, candidate{start: start, end: index + 1})
				}
				start = -1
			}
		}
	}
	if depth != 0 || len(candidates) == 0 {
		return nil, AdmissionIncomplete, errors.New("reviewer payload contains no complete JSON object")
	}
	if len(candidates) != 1 {
		return nil, AdmissionAmbiguous, errors.New("reviewer payload contains multiple JSON objects")
	}
	match := candidates[0]
	return append([]byte(nil), bytes.TrimSpace(payload[match.start:match.end])...), AdmissionCompleted, nil
}

// decodeReviewerResult strictly decodes a single JSON value with unknown
// fields rejected.
func decodeReviewerResult(payload []byte) (ReviewerResult, error) {
	var result ReviewerResult
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return ReviewerResult{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ReviewerResult{}, errors.New("input contains multiple JSON values")
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Canonicalization helpers
// ---------------------------------------------------------------------------

// FindingLocationErrorReason is a stable machine-readable validation reason.
type FindingLocationErrorReason string

const (
	FindingLocationExpectedPathAndLine FindingLocationErrorReason = "expected_path_and_line"
	FindingLocationLineNotInteger      FindingLocationErrorReason = "line_suffix_not_integer"
	FindingLocationLineNotPositive     FindingLocationErrorReason = "line_must_be_positive"
	FindingLocationPathNotRelative     FindingLocationErrorReason = "path_must_be_repository_relative"
	FindingLocationPathNotCanonical    FindingLocationErrorReason = "path_must_be_canonical"
)

// FindingLocationError describes why a reviewer location is invalid.
type FindingLocationError struct {
	Location string
	Reason   FindingLocationErrorReason
}

func (err *FindingLocationError) Error() string {
	return fmt.Sprintf("invalid reviewer finding location %q: %s", err.Location, err.Reason)
}

// parseFindingLocation splits "repository/path:<positive-line>" into its
// canonical logical path and line, or returns a typed reason.
func parseFindingLocation(location string) (string, int, error) {
	separator := strings.LastIndexByte(location, ':')
	if separator <= 0 || separator == len(location)-1 {
		return "", 0, &FindingLocationError{Location: location, Reason: FindingLocationExpectedPathAndLine}
	}
	lineSuffix := location[separator+1:]
	line, err := strconv.Atoi(lineSuffix)
	if err != nil {
		return "", 0, &FindingLocationError{Location: location, Reason: FindingLocationLineNotInteger}
	}
	for index := range lineSuffix {
		if lineSuffix[index] < '0' || lineSuffix[index] > '9' {
			reason := FindingLocationLineNotInteger
			if line <= 0 {
				reason = FindingLocationLineNotPositive
			}
			return "", 0, &FindingLocationError{Location: location, Reason: reason}
		}
	}
	if line <= 0 {
		return "", 0, &FindingLocationError{Location: location, Reason: FindingLocationLineNotPositive}
	}
	logicalPath := location[:separator]
	if len(logicalPath) >= 3 && logicalPath[1] == ':' && logicalPath[2] == '/' &&
		((logicalPath[0] >= 'A' && logicalPath[0] <= 'Z') || (logicalPath[0] >= 'a' && logicalPath[0] <= 'z')) {
		return "", 0, &FindingLocationError{Location: location, Reason: FindingLocationPathNotRelative}
	}
	if _, pathErr := normalizeLogicalPath(strings.ReplaceAll(logicalPath, ":", "/")); pathErr != nil {
		return "", 0, &FindingLocationError{Location: location, Reason: FindingLocationPathNotCanonical}
	}
	canonical, pathErr := normalizeLogicalPath(logicalPath)
	if pathErr != nil || canonical != logicalPath {
		return "", 0, &FindingLocationError{Location: location, Reason: FindingLocationPathNotCanonical}
	}
	return canonical, line, nil
}

// normalizeLogicalPath requires a canonical repository-relative path: no NUL,
// backslash, leading slash, dot segments, or redundant separators.
func normalizeLogicalPath(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("invalid logical path %q", value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != value {
		return "", fmt.Errorf("logical path is not canonical: %q", value)
	}
	return cleaned, nil
}

// canonicalPaths normalizes, sorts, and deduplicates a path list.
func canonicalPaths(values []string) ([]string, error) {
	normalized := make([]string, len(values))
	for index, value := range values {
		logicalPath, err := normalizeLogicalPath(value)
		if err != nil {
			return nil, err
		}
		normalized[index] = logicalPath
	}
	sort.Strings(normalized)
	for index := 1; index < len(normalized); index++ {
		if normalized[index] == normalized[index-1] {
			return nil, fmt.Errorf("duplicate candidate path %q", normalized[index])
		}
	}
	return normalized, nil
}

// canonicalStrings trims, sorts, and deduplicates a string list.
func canonicalStrings(values []string, label string) ([]string, error) {
	result := make([]string, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsRune(value, '\x00') {
			return nil, fmt.Errorf("%s must be non-empty", label)
		}
		result[index] = value
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index] == result[index-1] {
			return nil, fmt.Errorf("duplicate %s %q", label, result[index])
		}
	}
	return result, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func isConcreteEvidence(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	switch strings.ToLower(trimmed) {
	case "n/a", "na", "none", "todo", "tbd", "pass", "passed", "success", "placeholder":
		return false
	}
	return true
}

// evidenceReportsUnavailableInspection reports whether an evidence line claims
// the immutable candidate could not be inspected. Such evidence carries no
// verdict in either direction, so admission treats it as incomplete.
func evidenceReportsUnavailableInspection(value string) bool {
	value = strings.ToLower(strings.Join(strings.Fields(value), " "))
	for _, phrase := range []string{
		"inspection blocked", "inspection was blocked", "access denied", "permission denied",
		"candidate unavailable", "candidate was unavailable", "immutable candidate unavailable",
		"could not inspect", "unable to inspect", "was not inspected", "not inspected",
		"no candidate contents were available", "no candidate content was available",
	} {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Hash helpers
// ---------------------------------------------------------------------------

func domainHash(domain string, payload []byte) string {
	sum := sha256.Sum256(append([]byte(domain+"\x00"), payload...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func payloadSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// validSHA256Hex reports whether value is a lowercase 64-char SHA-256 hex.
func validSHA256Hex(value string) bool {
	return len(value) == 64 && isLowerHex(value)
}

// validSHA256Identity reports whether value is a "sha256:"-prefixed identity.
func validSHA256Identity(value string) bool {
	return strings.HasPrefix(value, "sha256:") && validSHA256Hex(strings.TrimPrefix(value, "sha256:"))
}

// validCommitSHA reports whether value is a lowercase git object SHA
// (SHA-1 40 hex or SHA-256 64 hex).
func validCommitSHA(value string) bool {
	return (len(value) == 40 || len(value) == 64) && isLowerHex(value)
}

func isLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for index := range value {
		if !(value[index] >= '0' && value[index] <= '9') &&
			!(value[index] >= 'a' && value[index] <= 'f') {
			return false
		}
	}
	return true
}
