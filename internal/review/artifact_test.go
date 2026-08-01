package review

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testManifestEntries(paths ...string) []ChangedPathManifestEntry {
	entries := make([]ChangedPathManifestEntry, 0, len(paths))
	for index, pathValue := range paths {
		status := "M"
		if index == 0 {
			status = "A"
		}
		entries = append(entries, ChangedPathManifestEntry{
			Path: pathValue, Status: status,
			OldMode: "100644", NewMode: "100644", Deleted: false,
		})
	}
	return entries
}

func testSubject(t *testing.T, lens string, order int, entries []ChangedPathManifestEntry) (ArtifactSubject, []string) {
	t.Helper()
	digest, err := ChangedPathManifestDigest(entries)
	if err != nil {
		t.Fatalf("ChangedPathManifestDigest: %v", err)
	}
	subject, err := NewArtifactSubject(
		"test-lineage", strings.Repeat("a", 64), strings.Repeat("b", 40),
		strings.Repeat("c", 40), strings.Repeat("d", 40), digest, lens, order)
	if err != nil {
		t.Fatalf("NewArtifactSubject: %v", err)
	}
	return subject, ManifestPaths(entries)
}

func validRequest(t *testing.T, paths []string) AdmissionRequest {
	t.Helper()
	subject, manifestPaths := testSubject(t, "risk", 0, testManifestEntries(paths...))
	return AdmissionRequest{
		ExpectedSubject:   subject,
		EchoedSubjectHash: subject.SubjectHash,
		Inspection:        ArtifactInspection{Status: ArtifactInspectionCompleted, Paths: append([]string(nil), paths...)},
		Result: ArtifactLensResult{
			Lens: "risk",
			Findings: []ArtifactFinding{{
				ID: "R1-001", Lens: "risk", Location: paths[0] + ":3",
				Severity: "CRITICAL", Claim: "simplify this branch",
				EvidenceClass: EvidenceDeterministic, CausalDisposition: CausalIntroduced,
			}},
			Evidence: []string{"go test reproduced the failing branch"},
		},
		RawPayload:    []byte(`{"subject_hash":"` + subject.SubjectHash + `","inspection":{"status":"completed","paths":["` + strings.Join(paths, `","`) + `"]},"lens":"risk","findings":[{"id":"R1-001","lens":"risk","location":"` + paths[0] + `:3","severity":"CRITICAL","claim":"simplify this branch","evidence_class":"deterministic","causal_disposition":"introduced"}],"evidence":["go test reproduced the failing branch"]}`),
		ManifestPaths: manifestPaths,
	}
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestAdmit_HappyPath(t *testing.T) {
	request := validRequest(t, []string{"a.txt", "b.txt"})
	canonical, admission, canonicalPayload, err := Admit(request)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if admission.Decision != AdmissionCompleted {
		t.Errorf("decision = %q, want completed", admission.Decision)
	}
	if admission.SubjectHash != request.ExpectedSubject.SubjectHash {
		t.Errorf("subject hash echo mismatch")
	}
	if admission.RawSHA256 == "" || admission.CanonicalSHA256 == "" || admission.ResultHash == "" {
		t.Errorf("admission hashes are empty: %+v", admission)
	}
	if canonical.ResultHash != admission.ResultHash {
		t.Errorf("result hash mismatch: %s vs %s", canonical.ResultHash, admission.ResultHash)
	}
	if len(canonicalPayload) == 0 {
		t.Error("canonical payload is empty")
	}
	// The canonical payload must hash to the recorded CanonicalSHA256.
	if payloadSHA256(canonicalPayload) != admission.CanonicalSHA256 {
		t.Error("canonical payload does not hash to admission.CanonicalSHA256")
	}
	if len(admission.CandidateCausalFindingIDs) != 1 || admission.CandidateCausalFindingIDs[0] != "R1-001" {
		t.Errorf("candidate causal ids = %v, want [R1-001]", admission.CandidateCausalFindingIDs)
	}
}

func TestAdmit_StableCanonicalPayload(t *testing.T) {
	request := validRequest(t, []string{"a.txt", "b.txt"})
	// Mirror the raw payload's non-canonical values in the decoded struct:
	// canonicalization must normalize both consistently.
	request.Result.Findings[0].Severity = "warning"
	request.Result.Findings[0].Claim = "  simplify this branch  "
	request.Result.Evidence = []string{"  go test reproduced the failing branch  "}
	// Re-admit with different whitespace/severity casing; the canonical payload
	// and hashes must be identical.
	request.RawPayload = []byte(`{
		"subject_hash": "` + request.ExpectedSubject.SubjectHash + `",
		"inspection": {"status": "completed", "paths": ["a.txt", "b.txt"]},
		"lens": "risk",
		"findings": [{"id": "R1-001", "lens": "risk", "location": "a.txt:3",
			"severity": "warning", "claim": "  simplify this branch  ",
			"evidence_class": "deterministic", "causal_disposition": "introduced"}],
		"evidence": ["  go test reproduced the failing branch  "]
	}`)
	_, _, canonicalPayload, err := Admit(request)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	var envelope struct {
		SubjectHash string `json:"subject_hash"`
		Inspection  struct {
			Paths []string `json:"paths"`
		} `json:"inspection"`
		Findings []struct {
			Severity string `json:"severity"`
			Claim    string `json:"claim"`
		} `json:"findings"`
		Evidence []string `json:"evidence"`
	}
	if err := json.Unmarshal(canonicalPayload, &envelope); err != nil {
		t.Fatalf("unmarshal canonical payload: %v", err)
	}
	if envelope.Findings[0].Severity != "WARNING" {
		t.Errorf("canonical severity = %q, want WARNING", envelope.Findings[0].Severity)
	}
	if envelope.Findings[0].Claim != "simplify this branch" {
		t.Errorf("canonical claim = %q, want trimmed claim", envelope.Findings[0].Claim)
	}
	if len(envelope.Evidence) != 1 || envelope.Evidence[0] != "go test reproduced the failing branch" {
		t.Errorf("canonical evidence = %v, want trimmed", envelope.Evidence)
	}
	// The canonical payload must be byte-stable: fixed field order and
	// canonicalized values.
	type expectedFinding struct {
		ID                string `json:"id"`
		Lens              string `json:"lens"`
		Location          string `json:"location"`
		Severity          string `json:"severity"`
		Claim             string `json:"claim"`
		EvidenceClass     string `json:"evidence_class"`
		CausalDisposition string `json:"causal_disposition"`
	}
	expected, err := json.Marshal(struct {
		SubjectHash string `json:"subject_hash"`
		Inspection  struct {
			Status string   `json:"status"`
			Paths  []string `json:"paths"`
		} `json:"inspection"`
		Lens     string            `json:"lens"`
		Findings []expectedFinding `json:"findings"`
		Evidence []string          `json:"evidence"`
	}{
		SubjectHash: request.ExpectedSubject.SubjectHash,
		Inspection: struct {
			Status string   `json:"status"`
			Paths  []string `json:"paths"`
		}{Status: "completed", Paths: []string{"a.txt", "b.txt"}},
		Lens: "risk",
		Findings: []expectedFinding{{
			ID: "R1-001", Lens: "risk", Location: "a.txt:3",
			Severity: "WARNING", Claim: "simplify this branch",
			EvidenceClass: "deterministic", CausalDisposition: "introduced",
		}},
		Evidence: []string{"go test reproduced the failing branch"},
	})
	if err != nil {
		t.Fatalf("marshal expected: %v", err)
	}
	if !bytes.Equal(canonicalPayload, expected) {
		t.Fatalf("canonical payload is not stable:\n got  %s\n want %s", canonicalPayload, expected)
	}
}

// ---------------------------------------------------------------------------
// Subject echo
// ---------------------------------------------------------------------------

func TestAdmit_RejectsWrongSubjectHash(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.EchoedSubjectHash = strings.Repeat("e", 64)
	result, admission, _, err := Admit(request)
	if err == nil {
		t.Fatal("expected error for wrong subject hash")
	}
	if admission.Decision != AdmissionBindingMismatch {
		t.Errorf("decision = %q, want binding_mismatch", admission.Decision)
	}
	if result.ResultHash != "" {
		t.Error("rejected admission must not produce a result")
	}
}

func TestAdmit_RejectsOmittedSubjectHash(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.EchoedSubjectHash = ""
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionIncomplete {
		t.Fatalf("want incomplete rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

// ---------------------------------------------------------------------------
// Inspection
// ---------------------------------------------------------------------------

func TestAdmit_RejectsIncompleteInspection(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Inspection.Status = "failed"
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionIncomplete {
		t.Fatalf("want incomplete rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsAccessFailureEvidence(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Evidence = []string{"access denied while opening the candidate tree"}
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionIncomplete {
		t.Fatalf("want incomplete rejection for access failure, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsMissingManifestPath(t *testing.T) {
	request := validRequest(t, []string{"a.txt", "b.txt"})
	request.Inspection.Paths = []string{"a.txt"}
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionIncomplete {
		t.Fatalf("want incomplete rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsPathOutOfOrder(t *testing.T) {
	request := validRequest(t, []string{"a.txt", "b.txt"})
	request.Inspection.Paths = []string{"b.txt", "a.txt"}
	// Out-of-order submission is non-canonical, which admission classifies as
	// out_of_scope (mirroring gentle-ai).
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionOutOfScope {
		t.Fatalf("want out_of_scope rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsExtraPath(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Inspection.Paths = []string{"a.txt", "outside.txt"}
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionOutOfScope {
		t.Fatalf("want out_of_scope rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsDuplicateInspectionPath(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Inspection.Paths = []string{"a.txt", "a.txt"}
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionOutOfScope {
		t.Fatalf("want out_of_scope rejection for duplicates, got err=%v decision=%q", err, admission.Decision)
	}
}

// ---------------------------------------------------------------------------
// Findings
// ---------------------------------------------------------------------------

func TestAdmit_RejectsFindingOutsideManifest(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Findings[0].Location = "other.txt:1"
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionOutOfScope {
		t.Fatalf("want out_of_scope rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsInvalidFindingLocation(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Findings[0].Location = "a.txt:zero"
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionOutOfScope {
		t.Fatalf("want out_of_scope rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsDuplicateFindingID(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Findings = append(request.Result.Findings, ArtifactFinding{
		ID: "R1-001", Lens: "risk", Location: "a.txt:5", Severity: "WARNING", Claim: "duplicate id",
	})
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionAmbiguous {
		t.Fatalf("want ambiguous rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsUnboundFindingID(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Findings[0].ID = "R2-001"
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionBindingMismatch {
		t.Fatalf("want binding_mismatch rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsSevereFindingWithoutDisposition(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Findings[0].Severity = "CRITICAL"
	request.Result.Findings[0].EvidenceClass = ""
	request.Result.Findings[0].CausalDisposition = ""
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionIncomplete {
		t.Fatalf("want incomplete rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RejectsSevereFindingUnsupportedDisposition(t *testing.T) {
	request := validRequest(t, []string{"a.txt"})
	request.Result.Findings[0].Severity = "BLOCKER"
	request.Result.Findings[0].EvidenceClass = EvidenceDeterministic
	request.Result.Findings[0].CausalDisposition = CausalDisposition("released")
	if _, admission, _, err := Admit(request); err == nil || admission.Decision != AdmissionIncomplete {
		t.Fatalf("want incomplete rejection, got err=%v decision=%q", err, admission.Decision)
	}
}

func TestAdmit_RecordsNonBlockingDispositions(t *testing.T) {
	cases := []struct {
		disposition CausalDisposition
		blocking    bool
	}{
		{CausalIntroduced, true},
		{CausalBehaviorActivated, true},
		{CausalWorsened, true},
		{CausalPreExisting, false},
		{CausalBaseOnly, false},
		{CausalUnknown, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.disposition), func(t *testing.T) {
			request := validRequest(t, []string{"a.txt"})
			request.Result.Findings[0].Severity = "CRITICAL"
			request.Result.Findings[0].EvidenceClass = EvidenceDeterministic
			request.Result.Findings[0].CausalDisposition = tc.disposition
			_, admission, _, err := Admit(request)
			if err != nil {
				t.Fatalf("Admit: %v", err)
			}
			if admission.Decision != AdmissionCompleted {
				t.Fatalf("decision = %q, want completed for non-blocking disposition", admission.Decision)
			}
			got := len(admission.CandidateCausalFindingIDs) == 1
			if got != tc.blocking {
				t.Errorf("candidate-causal membership = %v, want %v", got, tc.blocking)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Canonicalization
// ---------------------------------------------------------------------------

func TestCanonicalLensResult_DefaultsAndTrims(t *testing.T) {
	result, err := CanonicalLensResult(ArtifactLensResult{
		Lens: "risk",
		Findings: []ArtifactFinding{{
			Location: "a.txt:1", Severity: "critical", Claim: "  leak  ",
		}},
		Evidence: []string{"  reproduction: go test ./...  "},
	})
	if err != nil {
		t.Fatalf("CanonicalLensResult: %v", err)
	}
	if result.Findings[0].ID != "R1-001" {
		t.Errorf("default id = %q, want R1-001", result.Findings[0].ID)
	}
	if result.Findings[0].Lens != "risk" {
		t.Errorf("default lens = %q, want risk", result.Findings[0].Lens)
	}
	if result.Findings[0].Severity != "CRITICAL" {
		t.Errorf("severity = %q, want CRITICAL", result.Findings[0].Severity)
	}
	if result.Findings[0].Claim != "leak" {
		t.Errorf("claim = %q, want trimmed", result.Findings[0].Claim)
	}
	if result.ResultHash == "" {
		t.Error("result hash must be derived")
	}
}

func TestCanonicalLensResult_RejectsPlaceholderEvidence(t *testing.T) {
	_, err := CanonicalLensResult(ArtifactLensResult{
		Lens: "risk",
		Findings: []ArtifactFinding{{
			Location: "a.txt:1", Severity: "WARNING", Claim: "x",
		}},
		Evidence: []string{"n/a"},
	})
	if err == nil || !strings.Contains(err.Error(), "concrete") {
		t.Fatalf("want concrete-evidence rejection, got %v", err)
	}
}

func TestCanonicalLensResult_RejectsMissingEvidence(t *testing.T) {
	_, err := CanonicalLensResult(ArtifactLensResult{Lens: "risk", Findings: []ArtifactFinding{{Location: "a.txt:1", Severity: "WARNING", Claim: "x"}}})
	if err == nil {
		t.Fatal("expected rejection for missing evidence array")
	}
}

func TestCanonicalLensResult_RejectsStaleResultHash(t *testing.T) {
	_, err := CanonicalLensResult(ArtifactLensResult{
		Lens: "risk", ResultHash: strings.Repeat("f", 64),
		Findings: []ArtifactFinding{{Location: "a.txt:1", Severity: "WARNING", Claim: "x"}},
		Evidence: []string{"go test failed"},
	})
	if err == nil || !strings.Contains(err.Error(), "result_hash") {
		t.Fatalf("want result_hash rejection, got %v", err)
	}
}

func TestCanonicalLensResult_RejectsUnsupportedSeverity(t *testing.T) {
	_, err := CanonicalLensResult(ArtifactLensResult{
		Lens:     "risk",
		Findings: []ArtifactFinding{{Location: "a.txt:1", Severity: "INFO", Claim: "x"}},
		Evidence: []string{"go test failed"},
	})
	if err == nil || !strings.Contains(err.Error(), "severity") {
		t.Fatalf("want severity rejection, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Subject and manifest identity
// ---------------------------------------------------------------------------

func TestArtifactSubjectHash_Deterministic(t *testing.T) {
	left, _ := testSubject(t, "risk", 0, testManifestEntries("a.txt", "b.txt"))
	right, _ := testSubject(t, "risk", 0, testManifestEntries("a.txt", "b.txt"))
	if left.SubjectHash != right.SubjectHash {
		t.Error("subject hash must be deterministic")
	}
	other, _ := testSubject(t, "readability", 0, testManifestEntries("a.txt", "b.txt"))
	if left.SubjectHash == other.SubjectHash {
		t.Error("subject hash must change with the lens")
	}
	if err := ValidateArtifactSubject(left); err != nil {
		t.Fatalf("ValidateArtifactSubject: %v", err)
	}
}

func TestChangedPathManifestDigest_OrderSensitive(t *testing.T) {
	a, err := ChangedPathManifestDigest(testManifestEntries("a.txt", "b.txt"))
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	b, err := ChangedPathManifestDigest(testManifestEntries("b.txt", "a.txt"))
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if a == b {
		t.Error("manifest digest must be order-sensitive")
	}
	if _, err := ChangedPathManifestDigest(testManifestEntries("a.txt", "a.txt")); err == nil {
		t.Error("duplicate manifest paths must be rejected")
	}
}

func TestParseFindingLocation(t *testing.T) {
	canonical, line, err := parseFindingLocation("src/main.go:42")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if canonical != "src/main.go" || line != 42 {
		t.Errorf("got %q:%d", canonical, line)
	}
	for _, bad := range []string{"", "no-colon", "a.txt:0", "a.txt:-1", "a.txt:x", "../up.txt:1", "/abs.txt:1", "C:/win.txt:1", "a//b.txt:1"} {
		if _, _, err := parseFindingLocation(bad); err == nil {
			t.Errorf("expected rejection for %q", bad)
		}
	}
}

func TestCanonicalPaths_RejectsDuplicates(t *testing.T) {
	if _, err := canonicalPaths([]string{"a.txt", "a.txt"}); err == nil {
		t.Error("duplicate paths must be rejected")
	}
}

// ---------------------------------------------------------------------------
// Payload transport
// ---------------------------------------------------------------------------

func TestValidateReviewerResultPayload_Empty(t *testing.T) {
	err := validateReviewerResultPayload([]byte("  \n"))
	if err == nil || err.(*ReviewerResultPayloadError).Code != "empty_result" {
		t.Fatalf("want empty_result, got %v", err)
	}
}

func TestValidateReviewerResultPayload_NestedEnvelope(t *testing.T) {
	err := validateReviewerResultPayload([]byte("<task_result>not json</task_result>"))
	if err == nil || err.(*ReviewerResultPayloadError).Code != "nested_envelope" {
		t.Fatalf("want nested_envelope, got %v", err)
	}
}

func TestExtractBoundedSingleJSONObject_MultipleObjects(t *testing.T) {
	_, decision, err := ExtractBoundedSingleJSONObject([]byte(`{"a":1} {"b":2}`), 1024)
	if err == nil || decision != AdmissionAmbiguous {
		t.Fatalf("want ambiguous, got decision=%q err=%v", decision, err)
	}
}

func TestDecodeReviewerResult_RejectsUnknownFields(t *testing.T) {
	payload := []byte(`{"subject_hash":"x","inspection":{"status":"completed","paths":[]},"findings":[],"evidence":["go test"],"summary":"extra"}`)
	if _, err := decodeReviewerResult(payload); err == nil {
		t.Fatal("expected rejection of unknown top-level field")
	}
	payload = []byte(`{"subject_hash":"x","inspection":{"status":"completed","paths":[]},"findings":[{"id":"R1-001","severity":"WARNING","claim":"x","evidence":"extra"}],"evidence":["go test"]}`)
	if _, err := decodeReviewerResult(payload); err == nil {
		t.Fatal("expected rejection of unknown finding field")
	}
}

func TestDecodeReviewerResult_MultipleValues(t *testing.T) {
	payload := []byte(`{"subject_hash":"x","inspection":{"status":"completed","paths":[]},"findings":[],"evidence":["go test"]} {"second":true}`)
	if _, err := decodeReviewerResult(payload); err == nil {
		t.Fatal("expected rejection of multiple JSON values")
	}
}
