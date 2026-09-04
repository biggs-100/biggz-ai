package sdd

import (
	"context"
	"testing"
)

func TestReviewOfferForVerify_RDDDisabled(t *testing.T) {
	ResetReviewEntryHookCallCount()
	
	ctx := context.Background()
	offer, err := ReviewOfferForVerify(ctx, "/tmp/workspace", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offer.Available {
		t.Error("expected offer to be unavailable when RDD is disabled")
	}
	if offer.Reason != "RDD disabled" {
		t.Errorf("expected reason 'RDD disabled', got %q", offer.Reason)
	}
	if ReviewEntryHookCallCount() != 1 {
		t.Errorf("expected hook to be called once, got %d", ReviewEntryHookCallCount())
	}
}

func TestReviewOfferForVerify_RDDEnabled(t *testing.T) {
	ResetReviewEntryHookCallCount()
	
	ctx := context.Background()
	offer, err := ReviewOfferForVerify(ctx, "/tmp/workspace", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !offer.Available {
		t.Error("expected offer to be available when RDD is enabled")
	}
	if offer.Reason == "" {
		t.Error("expected a reason")
	}
	if ReviewEntryHookCallCount() != 1 {
		t.Errorf("expected hook to be called once, got %d", ReviewEntryHookCallCount())
	}
}

func TestReviewOfferForVerify_ContextCancelled(t *testing.T) {
	ResetReviewEntryHookCallCount()
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	_, err := ReviewOfferForVerify(ctx, "/tmp/workspace", true)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if ReviewEntryHookCallCount() != 1 {
		t.Errorf("expected hook to be called once, got %d", ReviewEntryHookCallCount())
	}
}

func TestReviewOfferForVerify_HookFires(t *testing.T) {
	ResetReviewEntryHookCallCount()
	hookCalled := false
	SetReviewEntryHook(func() {
		hookCalled = true
	})
	defer ResetReviewEntryHook()
	
	ctx := context.Background()
	ReviewOfferForVerify(ctx, "/tmp/workspace", true)
	
	if !hookCalled {
		t.Error("expected hook to be called")
	}
}

func TestReviewOfferForVerify_MultipleCalls(t *testing.T) {
	ResetReviewEntryHookCallCount()
	
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ReviewOfferForVerify(ctx, "/tmp/workspace", true)
	}
	
	if ReviewEntryHookCallCount() != 5 {
		t.Errorf("expected hook to be called 5 times, got %d", ReviewEntryHookCallCount())
	}
}

func TestResetReviewEntryHookCallCount(t *testing.T) {
	ResetReviewEntryHookCallCount()
	
	ctx := context.Background()
	ReviewOfferForVerify(ctx, "/tmp/workspace", true)
	
	if ReviewEntryHookCallCount() != 1 {
		t.Errorf("expected 1, got %d", ReviewEntryHookCallCount())
	}
	
	ResetReviewEntryHookCallCount()
	
	if ReviewEntryHookCallCount() != 0 {
		t.Errorf("expected 0 after reset, got %d", ReviewEntryHookCallCount())
	}
}
