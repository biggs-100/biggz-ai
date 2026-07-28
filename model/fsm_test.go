package model

import (
	"testing"

	"pgregory.net/rapid"
)

// allStatuses lists every ReviewStatus for use in rapid generators and property checks.
var allStatuses = []ReviewStatus{
	StatusPending,
	StatusInProgress,
	StatusCompleted,
	StatusArchived,
	StatusFailed,
}

// validTransitions returns true if the FSM allows moving from current to target.
func validTransition(current, target ReviewStatus) bool {
	if current == target {
		return true
	}
	transitions, ok := transitionMap[current]
	if !ok {
		return false
	}
	return transitions[target]
}

// --- Property-based tests using rapid ---

// TestFSM_ArbitraryTransitions generates random (current, target) pairs and verifies
// that Transition matches the expected validity from the transition map.
func TestFSM_ArbitraryTransitions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		current := rapid.SampledFrom(allStatuses).Draw(t, "current")
		target := rapid.SampledFrom(allStatuses).Draw(t, "target")

		err := Transition(current, target)
		expectedValid := validTransition(current, target)

		if expectedValid && err != nil {
			t.Errorf("expected valid transition %s → %s, got error: %v", current, target, err)
		}
		if !expectedValid && err == nil {
			t.Errorf("expected invalid transition %s → %s, got nil", current, target)
		}
	})
}

// TestFSM_ValidSequenceChain generates a chain of valid transitions starting from
// StatusPending and verifies that every step succeeds.
func TestFSM_ValidSequenceChain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Pick a random valid sequence from all possible valid transitions
		// starting from Pending. We generate a random path through the state graph.
		current := StatusPending
		steps := rapid.IntRange(0, 10).Draw(t, "steps")

		for i := 0; i < steps; i++ {
			allowed := AllowedTransitions(current)
			next := rapid.SampledFrom(allowed).Draw(t, "next")
			if err := Transition(current, next); err != nil {
				t.Fatalf("valid transition %s → %s failed: %v", current, next, err)
			}
			current = next
		}
	})
}

// TestFSM_RejectsInvalidTransitions verifies that every (current, target) pair that
// is NOT in the transition map returns an error.
func TestFSM_RejectsInvalidTransitions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		current := rapid.SampledFrom(allStatuses).Draw(t, "current")
		target := rapid.SampledFrom(allStatuses).Draw(t, "target")

		err := Transition(current, target)

		if !validTransition(current, target) && err == nil {
			t.Errorf("Transition(%q, %q) = nil, expected error", current, target)
		}
	})
}

// TestFSM_SelfTransition verifies that any state can transition to itself.
func TestFSM_SelfTransition(t *testing.T) {
	for _, s := range allStatuses {
		if err := Transition(s, s); err != nil {
			t.Errorf("self-transition for %s should be valid, got: %v", s, err)
		}
	}
}

// --- Table-driven tests for known valid/invalid transitions ---

func TestKnownValidTransitions(t *testing.T) {
	tests := []struct {
		current ReviewStatus
		target  ReviewStatus
	}{
		{StatusPending, StatusInProgress},
		{StatusPending, StatusFailed},
		{StatusInProgress, StatusCompleted},
		{StatusInProgress, StatusFailed},
		{StatusCompleted, StatusArchived},
	}

	for _, tc := range tests {
		t.Run(string(tc.current)+"_to_"+string(tc.target), func(t *testing.T) {
			if err := Transition(tc.current, tc.target); err != nil {
				t.Errorf("expected valid transition %s → %s, got error: %v", tc.current, tc.target, err)
			}
		})
	}
}

func TestKnownInvalidTransitions(t *testing.T) {
	tests := []struct {
		current ReviewStatus
		target  ReviewStatus
	}{
		{StatusPending, StatusCompleted},
		{StatusPending, StatusArchived},
		{StatusInProgress, StatusPending},
		{StatusInProgress, StatusArchived},
		{StatusCompleted, StatusPending},
		{StatusCompleted, StatusInProgress},
		{StatusCompleted, StatusFailed},
		{StatusArchived, StatusPending},
		{StatusArchived, StatusInProgress},
		{StatusArchived, StatusCompleted},
		{StatusArchived, StatusFailed},
		{StatusFailed, StatusPending},
		{StatusFailed, StatusInProgress},
		{StatusFailed, StatusCompleted},
		{StatusFailed, StatusArchived},
	}

	for _, tc := range tests {
		t.Run(string(tc.current)+"_to_"+string(tc.target), func(t *testing.T) {
			if err := Transition(tc.current, tc.target); err == nil {
				t.Errorf("expected invalid transition %s → %s, got nil", tc.current, tc.target)
			}
		})
	}
}
