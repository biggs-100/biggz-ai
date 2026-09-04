package sdd

import (
	"testing"
	"time"
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
		name   string
		resp   *GateResponse
		contains string
	}{
		{
			name: "allow",
			resp: &GateResponse{Result: GateAllow, Gate: GatePostApply, Reason: "passed"},
			contains: "ALLOW",
		},
		{
			name: "invalidated",
			resp: &GateResponse{Result: GateInvalidated, Gate: GatePreCommit, Reason: "failed"},
			contains: "INVALIDATED",
		},
		{
			name: "escalated",
			resp: &GateResponse{Result: GateEscalated, Gate: GatePrePush, Reason: "needs authority"},
			contains: "ESCALATED",
		},
		{
			name: "scope_changed",
			resp: &GateResponse{Result: GateScopeChanged, Gate: GatePrePR, Reason: "scope changed"},
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
