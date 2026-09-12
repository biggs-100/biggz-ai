package review

// Reviewer task materializer — composes the complete provider-owned reviewer
// task for a collect transition from the frozen trees: binding, preflight
// context, name-status, numstat, and per-path frozen patch bytes with
// explicit delimiters.
//
// READ-ONLY: it verifies the capture binding against the lineage authority
// (the same resolution `Preflight` performs), reads frozen-tree bytes through
// the isolated inspector, and returns the composed bytes. It never writes to
// the lineage store, never appends events, and never captures — so
// `biggz review capture-result --materialize` can print the task without
// mutating anything.
//
// Determinism: every section derives from the frozen base/candidate trees, so
// the same lineage produces byte-identical output across runs and from any
// working directory.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Reviewer-task delimiters. Hosts forward these bytes verbatim to the
// reviewer; the binding and context lines stay one-line JSON so the host-side
// binding literal remains recognizable.
const (
	MaterializeBindingMarker    = "GENTLE_AI_REVIEW_BINDING"
	MaterializeContextMarker    = "GENTLE_AI_REVIEW_CONTEXT"
	MaterializeNameStatusMarker = "GENTLE_AI_REVIEW_NAME_STATUS"
	MaterializeNumStatMarker    = "GENTLE_AI_REVIEW_NUMSTAT"
	MaterializePatchMarker      = "GENTLE_AI_REVIEW_PATCH"
	// MaterializeVacuousCode refuses materialized evidence that would let an
	// all-clear be recorded over nothing (D3 non-vacuity).
	MaterializeVacuousCode = "materialize_vacuous"
	// MaterializeTreeMismatchCode refuses a frozen-view resolution that
	// disagrees with the preflight-derived artifact subject.
	MaterializeTreeMismatchCode = "materialize_tree_mismatch"
)

// MaterializeRefusal is the typed refusal for a materialization that cannot
// produce complete evidence: a vacuous task (no changed paths, an empty patch
// for a content-changing path, or missing diff facts) or an inconsistent
// frozen view.
type MaterializeRefusal struct {
	Code   string
	Detail string
}

// Error implements error.
func (r *MaterializeRefusal) Error() string {
	return fmt.Sprintf("review materialize: %s: %s", r.Code, r.Detail)
}

// MaterializeCapRefusal is the typed refusal emitted when the composed
// reviewer task exceeds ArtifactResultLimit. A refused materialization emits
// no bytes at all: this is never a silent truncation. Actual is a lower bound
// on the true size (composition stops at the first overflow).
type MaterializeCapRefusal struct {
	Cap    int
	Actual int
}

// Error implements error.
func (r *MaterializeCapRefusal) Error() string {
	return fmt.Sprintf("review materialize: the composed reviewer task is at least %d bytes, exceeding the %d-byte whole-task cap (refused whole; never truncated)",
		r.Actual, r.Cap)
}

// MaterializeReviewerTask composes the complete reviewer task for one capture
// binding: binding JSON, preflight context JSON, name-status, numstat, then
// one delimited section per changed path with its frozen patch bytes.
func MaterializeReviewerTask(b CaptureBinding) ([]byte, error) {
	result, err := Preflight(b)
	if err != nil {
		return nil, err
	}
	inspector, err := OpenFrozenInspector(b.Repo, result.TargetIdentity, result.BaseTree)
	if err != nil {
		return nil, err
	}
	defer func() { _ = inspector.Close() }()
	if inspector.BaseTree() != result.BaseTree || inspector.CandidateTree() != result.CandidateTree {
		return nil, &MaterializeRefusal{Code: MaterializeTreeMismatchCode, Detail: fmt.Sprintf(
			"the isolated view resolved trees %s..%s but preflight resolved %s..%s",
			inspector.BaseTree(), inspector.CandidateTree(), result.BaseTree, result.CandidateTree)}
	}
	return composeReviewerTask(result, inspector)
}

// materializeFacts is the read-only frozen evidence composition needs.
// *FrozenInspector implements it; tests seam controlled facts.
type materializeFacts interface {
	Paths() []string
	NameStatus() ([]byte, error)
	NumStat() ([]byte, error)
	PatchForPath(path string) ([]byte, error)
}

// materializeBinding is the one-line provider-owned binding JSON the reviewer
// task opens with. Keys mirror the host binding literal.
type materializeBinding struct {
	Lineage     string `json:"lineage"`
	Target      string `json:"target"`
	Lens        string `json:"lens"`
	Order       int    `json:"order"`
	Revision    string `json:"revision"`
	SubjectHash string `json:"subject_hash"`
}

// composeReviewerTask renders the deterministic task bytes and enforces the
// whole-task cap and the non-vacuity guards.
func composeReviewerTask(result *PreflightResult, facts materializeFacts) ([]byte, error) {
	paths := facts.Paths()
	if len(paths) == 0 {
		return nil, &MaterializeRefusal{Code: MaterializeVacuousCode, Detail: "the frozen inspection path set is empty"}
	}
	header, err := materializeHeader(result)
	if err != nil {
		return nil, err
	}
	diffFacts, err := materializeDiffFacts(facts)
	if err != nil {
		return nil, err
	}
	writer := &materializeWriter{cap: ArtifactResultLimit}
	if err := writer.write(header); err != nil {
		return nil, err
	}
	if err := writer.write(diffFacts); err != nil {
		return nil, err
	}
	if err := writeMaterializePatches(writer, facts, paths); err != nil {
		return nil, err
	}
	return writer.buffer.Bytes(), nil
}

// materializeHeader renders the binding and preflight-context lines.
func materializeHeader(result *PreflightResult) ([]byte, error) {
	bindingJSON, err := json.Marshal(materializeBinding{
		Lineage: result.LineageID, Target: result.TargetIdentity, Lens: result.Lens,
		Order: result.SelectedOrder, Revision: result.ExpectedRevision,
		SubjectHash: result.Subject.SubjectHash,
	})
	if err != nil {
		return nil, fmt.Errorf("review materialize: encode binding: %w", err)
	}
	contextJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("review materialize: encode context: %w", err)
	}
	var header bytes.Buffer
	header.WriteString(MaterializeBindingMarker + " ")
	header.Write(bindingJSON)
	header.WriteString("\n" + MaterializeContextMarker + " ")
	header.Write(contextJSON)
	header.WriteString("\n")
	return header.Bytes(), nil
}

// materializeDiffFacts renders the name-status and numstat sections, refusing
// typed when a non-empty manifest carries no diff facts.
func materializeDiffFacts(facts materializeFacts) ([]byte, error) {
	nameStatus, err := facts.NameStatus()
	if err != nil {
		return nil, err
	}
	numStat, err := facts.NumStat()
	if err != nil {
		return nil, err
	}
	if len(nameStatus) == 0 || len(numStat) == 0 {
		return nil, &MaterializeRefusal{Code: MaterializeVacuousCode, Detail: "the frozen manifest is non-empty but its name-status/numstat evidence is empty"}
	}
	var payload bytes.Buffer
	payload.WriteString(MaterializeNameStatusMarker + "\n")
	payload.Write(nameStatus)
	payload.WriteString(MaterializeNumStatMarker + "\n")
	payload.Write(numStat)
	return payload.Bytes(), nil
}

// writeMaterializePatches appends one delimited patch section per manifest
// path, refusing typed when a content-changing path yields no bytes.
func writeMaterializePatches(writer *materializeWriter, facts materializeFacts, paths []string) error {
	for _, path := range paths {
		if err := writer.writeString(MaterializePatchMarker + " " + path + "\n"); err != nil {
			return err
		}
		patch, err := facts.PatchForPath(path)
		if err != nil {
			return err
		}
		if len(patch) == 0 {
			return &MaterializeRefusal{Code: MaterializeVacuousCode,
				Detail: fmt.Sprintf("path %q is content-changing but its frozen patch is empty", path)}
		}
		if err := writer.write(patch); err != nil {
			return err
		}
	}
	return nil
}

// materializeWriter accumulates the composed task under the whole-task cap.
// A write that would exceed the cap refuses typed instead of truncating.
type materializeWriter struct {
	buffer bytes.Buffer
	cap    int
}

// write appends one payload or refuses typed.
func (w *materializeWriter) write(payload []byte) error {
	if w.buffer.Len()+len(payload) > w.cap {
		return &MaterializeCapRefusal{Cap: w.cap, Actual: w.buffer.Len() + len(payload)}
	}
	w.buffer.Write(payload)
	return nil
}

// writeString appends one string payload or refuses typed.
func (w *materializeWriter) writeString(value string) error {
	return w.write([]byte(value))
}
