package sdd

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/pathquote"
	"github.com/biggs-100/biggz-ai/internal/review"
)

func TestEvaluateGate_PostApply(t *testing.T) {
	req := &GateRequest{
		Gate:             GatePostApply,
		ChangeName:       "test-change",
		WorkspaceRoot:    "/tmp/workspace",
		EvidenceRevision: "abc123",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateAllow {
		t.Errorf("expected ALLOW, got %v", resp.Result)
	}
	if resp.Gate != GatePostApply {
		t.Errorf("expected post-apply gate, got %v", resp.Gate)
	}
}

func TestEvaluateGate_PostApply_MissingEvidence(t *testing.T) {
	req := &GateRequest{
		Gate:       GatePostApply,
		ChangeName: "test-change",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateInvalidated {
		t.Errorf("expected INVALIDATED, got %v", resp.Result)
	}
}

func TestEvaluateGate_PreCommit(t *testing.T) {
	req := &GateRequest{
		Gate:        GatePreCommit,
		ChangeName:  "test-change",
		ReceiptHash: "def456",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateAllow {
		t.Errorf("expected ALLOW, got %v", resp.Result)
	}
}

func TestEvaluateGate_PreCommit_MissingReceipt(t *testing.T) {
	req := &GateRequest{
		Gate:       GatePreCommit,
		ChangeName: "test-change",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateEscalated {
		t.Errorf("expected ESCALATED, got %v", resp.Result)
	}
}

func TestEvaluateGate_PrePush(t *testing.T) {
	req := &GateRequest{
		Gate:             GatePrePush,
		ChangeName:       "test-change",
		EvidenceRevision: "abc123",
		ReceiptHash:      "def456",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateAllow {
		t.Errorf("expected ALLOW, got %v", resp.Result)
	}
}

func TestEvaluateGate_PrePush_MissingEvidence(t *testing.T) {
	req := &GateRequest{
		Gate:        GatePrePush,
		ChangeName:  "test-change",
		ReceiptHash: "def456",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateEscalated {
		t.Errorf("expected ESCALATED, got %v", resp.Result)
	}
}

func TestEvaluateGate_PrePR(t *testing.T) {
	req := &GateRequest{
		Gate:             GatePrePR,
		ChangeName:       "test-change",
		EvidenceRevision: "abc123",
		ReceiptHash:      "def456",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateAllow {
		t.Errorf("expected ALLOW, got %v", resp.Result)
	}
}

func TestEvaluateGate_PrePR_MissingEvidence(t *testing.T) {
	req := &GateRequest{
		Gate:        GatePrePR,
		ChangeName:  "test-change",
		ReceiptHash: "def456",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateEscalated {
		t.Errorf("expected ESCALATED, got %v", resp.Result)
	}
}

func TestEvaluateGate_Release(t *testing.T) {
	req := &GateRequest{
		Gate:             GateRelease,
		ChangeName:       "test-change",
		EvidenceRevision: "abc123",
		ReceiptHash:      "def456",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateAllow {
		t.Errorf("expected ALLOW, got %v", resp.Result)
	}
}

func TestEvaluateGate_Release_MissingReceipt(t *testing.T) {
	req := &GateRequest{
		Gate:             GateRelease,
		ChangeName:       "test-change",
		EvidenceRevision: "abc123",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateEscalated {
		t.Errorf("expected ESCALATED, got %v", resp.Result)
	}
}

func TestEvaluateGate_UnknownGate(t *testing.T) {
	req := &GateRequest{
		Gate:       "unknown",
		ChangeName: "test-change",
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateInvalidated {
		t.Errorf("expected INVALIDATED for unknown gate, got %v", resp.Result)
	}
}

func TestEvaluateGate_MissingChangeName(t *testing.T) {
	req := &GateRequest{
		Gate: GatePostApply,
	}

	resp, err := EvaluateGate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Result != GateInvalidated {
		t.Errorf("expected INVALIDATED for missing change_name, got %v", resp.Result)
	}
}

func TestParseGateRequest(t *testing.T) {
	json := `{"gate":"post-apply","change_name":"test","workspace_root":"/tmp","evidence_revision":"abc"}`
	req, err := ParseGateRequest(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Gate != GatePostApply {
		t.Errorf("expected post-apply, got %v", req.Gate)
	}
	if req.ChangeName != "test" {
		t.Errorf("expected test, got %v", req.ChangeName)
	}
}

func TestParseGateRequest_Invalid(t *testing.T) {
	_, err := ParseGateRequest("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGateResultSummary(t *testing.T) {
	tests := []struct {
		name     string
		resp     *GateResponse
		contains string
	}{
		{
			name:     "allow",
			resp:     &GateResponse{Result: GateAllow, Gate: GatePostApply, Reason: "passed"},
			contains: "ALLOW",
		},
		{
			name:     "invalidated",
			resp:     &GateResponse{Result: GateInvalidated, Gate: GatePreCommit, Reason: "failed"},
			contains: "INVALIDATED",
		},
		{
			name:     "escalated",
			resp:     &GateResponse{Result: GateEscalated, Gate: GatePrePush, Reason: "needs authority"},
			contains: "ESCALATED",
		},
		{
			name:     "scope_changed",
			resp:     &GateResponse{Result: GateScopeChanged, Gate: GatePrePR, Reason: "scope changed"},
			contains: "SCOPE_CHANGED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := GateResultSummary(tt.resp)
			if summary == "" {
				t.Error("expected non-empty summary")
			}
		})
	}
}

func TestGateResponse_Time(t *testing.T) {
	req := &GateRequest{
		Gate:       GatePostApply,
		ChangeName: "test-change",
	}

	before := time.Now()
	resp, err := EvaluateGate(req)
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Time.Before(before) || resp.Time.After(after) {
		t.Errorf("expected time between %v and %v, got %v", before, after, resp.Time)
	}
}

// ---------------------------------------------------------------------------
// RDD surfacing parity (tasks 5.3-5.5, design D5)
// ---------------------------------------------------------------------------

// TestRDDParitySurfacing proves the pre-publication review obligation rides
// the existing blockedReasons mechanism with the exact producer command from
// the review manifest, surfaces in the apply/verify phase instructions,
// clears when a valid receipt exists or RDD is disabled, and never adds a new
// status key or changes nextRecommended routing.
func TestRDDParitySurfacing(t *testing.T) {
	t.Run("ObligationNamesProducerAndSurfacesOffer", func(t *testing.T) {
		rddResolveIsolateHome(t)
		ws := t.TempDir()
		change := "parity-obligation"
		seedDeriveChange(t, ws, change, map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
			"verify-report.md":   passingReport("1/1", "1/1"),
		})
		cs, err := readChange(filepath.Join(ws, "openspec", "changes", change), change, false, ws, true)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}

		// Offer: quoted subject file, no lineage id (design D1/D5).
		if cs.ReviewOffer == nil || !cs.ReviewOffer.Available {
			t.Fatalf("expected the review offer, got %+v", cs.ReviewOffer)
		}
		subjectPath := filepath.Join(ws, "openspec", "changes", change, "review-subject.json")
		if !strings.Contains(cs.ReviewOffer.Invocation, pathquote.Quote(subjectPath)) {
			t.Fatalf("offer must carry the quoted subject file %s: %q", subjectPath, cs.ReviewOffer.Invocation)
		}
		if strings.Contains(cs.ReviewOffer.Invocation, "--lineage") {
			t.Fatalf("offer must not embed a lineage id: %q", cs.ReviewOffer.Invocation)
		}

		// Obligation: blockedReasons carries the typed reason and the exact
		// producer command resolved from the review manifest.
		resolution := review.ResolveProducer(review.ProducerSurfaceSDDVerify, review.CurrentProducerHost(), subjectPath)
		if !resolution.Producible {
			t.Fatalf("the sdd-verify producer must be registered for host %q", review.CurrentProducerHost())
		}
		obligation := ""
		for _, reason := range cs.BlockedReasons {
			if strings.HasPrefix(reason, "rdd_receipt_missing") {
				obligation = reason
				break
			}
		}
		if obligation == "" {
			t.Fatalf("blockedReasons must carry the RDD obligation, got %v", cs.BlockedReasons)
		}
		if !strings.Contains(obligation, resolution.Command) {
			t.Fatalf("obligation must name the exact producer command %q, got %q", resolution.Command, obligation)
		}

		// Phase instructions surface it pre-publication — apply and verify.
		if cs.PhaseInstructions == nil {
			t.Fatal("expected phase instructions")
		}
		assertObligation := func(label string, lines []string) {
			t.Helper()
			for _, line := range lines {
				if strings.Contains(line, "Pre-publication review obligation") && strings.Contains(line, resolution.Command) {
					return
				}
			}
			t.Fatalf("%s instructions must surface the obligation with the producer command, got %v", label, lines)
		}
		assertObligation("apply", cs.PhaseInstructions.Apply)
		assertObligation("verify", cs.PhaseInstructions.Verify)

		// nextRecommended stays the existing blocked-verify routing; no new key.
		if cs.NextRecommended != "resolve-blockers" {
			t.Fatalf("nextRecommended = %q, want resolve-blockers (unchanged routing)", cs.NextRecommended)
		}
	})

	t.Run("ObligationClearsWithValidReceipt", func(t *testing.T) {
		rddResolveIsolateHome(t)
		repo, head := rddResolveRepo(t)
		change := "parity-receipt"
		seedDeriveChange(t, repo, change, map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
			"verify-report.md":   passingReport("1/1", "1/1"),
		})

		lineageID, err := review.DeriveLineageID(repo, head)
		if err != nil {
			t.Fatalf("DeriveLineageID: %v", err)
		}
		store := rddResolveStart(t, repo, head, lineageID)
		rddResolveCapture(t, store, repo, head, lineageID)
		rddResolveFinalize(t, repo, lineageID)

		// Offer, gate and receipt resolve to one lineage: the offer carries no
		// id, while the gate resolves HEAD to the derived identity whose store
		// holds the captured receipt.
		resolved, err := review.ResolveCandidateLineage(repo, "HEAD^{commit}")
		if err != nil {
			t.Fatalf("ResolveCandidateLineage: %v", err)
		}
		if resolved != lineageID {
			t.Fatalf("gate resolved %q, want the derived lineage %q (offer ≡ gate ≡ receipt)", resolved, lineageID)
		}

		cs, err := readChange(filepath.Join(repo, "openspec", "changes", change), change, false, repo, true)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}
		for _, reason := range cs.BlockedReasons {
			if isReviewObligationReason(reason) {
				t.Fatalf("a valid receipt must clear the obligation, got %q", reason)
			}
		}
		if cs.PhaseInstructions != nil {
			for _, lines := range [][]string{cs.PhaseInstructions.Apply, cs.PhaseInstructions.Verify} {
				for _, line := range lines {
					if strings.Contains(line, "Pre-publication review obligation") {
						t.Fatalf("a valid receipt must clear the obligation from instructions, got %q", line)
					}
				}
			}
		}
	})

	t.Run("ObligationClearsWhenRDDDisabled", func(t *testing.T) {
		rddResolveIsolateHome(t)
		if _, err := review.RDDDisable("", "", "global"); err != nil {
			t.Fatalf("RDDDisable: %v", err)
		}
		ws := t.TempDir()
		change := "parity-disabled"
		seedDeriveChange(t, ws, change, map[string]string{
			"proposal.md":        "# Proposal\n",
			"specs/core/spec.md": specFixture,
			"design.md":          "# Design\n",
			"tasks.md":           "- [x] T1\n",
			"verify-report.md":   passingReport("1/1", "1/1"),
		})
		cs, err := readChange(filepath.Join(ws, "openspec", "changes", change), change, false, ws, true)
		if err != nil {
			t.Fatalf("readChange: %v", err)
		}
		for _, reason := range cs.BlockedReasons {
			if isReviewObligationReason(reason) {
				t.Fatalf("disabled RDD must surface no obligation, got %q", reason)
			}
		}
		if cs.ReviewOffer != nil {
			t.Fatalf("disabled RDD must not offer a review, got %+v", cs.ReviewOffer)
		}
	})

	t.Run("UnproducibleRefusalIsTyped", func(t *testing.T) {
		unproducible := producerRefusal(review.ProducerResolution{Surface: review.ProducerSurfaceSDDVerify, Host: "unknown-host"}, errors.New("no resolvable lineage"))
		if !strings.Contains(unproducible.Error(), "rdd_unproducible") {
			t.Fatalf("missing producer must report rdd_unproducible, got %v", unproducible)
		}
		if strings.Contains(unproducible.Error(), "rdd_receipt_missing") {
			t.Fatalf("unproducible must not masquerade as rdd_receipt_missing, got %v", unproducible)
		}
		producible := producerRefusal(review.ResolveProducer(review.ProducerSurfaceSDDVerify, "opencode", filepath.Join("ws", "review-subject.json")), errors.New("no resolvable lineage"))
		if !strings.Contains(producible.Error(), "rdd_receipt_missing") || !strings.Contains(producible.Error(), "biggz review start --subject") {
			t.Fatalf("missing receipt must name the exact producer command, got %v", producible)
		}
	})
}
