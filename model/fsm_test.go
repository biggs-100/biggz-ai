package model

import "testing"

// allStatuses lists every ReviewStatus for use in rapid generators and property checks.
var allStatuses = []ReviewStatus{
	StatusPending,
	StatusInProgress,
	StatusCompleted,
	StatusArchived,
	StatusFailed,
}

// ---------------------------------------------------------------------------
// 13-State FSM Tests
// ---------------------------------------------------------------------------

// newStatuses returns all new 13-state values for table-driven tests.
var newStatuses = []ReviewStatus{
	StatusUnreviewed,
	StatusInReview,
	StatusNeedsChanges,
	StatusChangesSubmitted,
	StatusReReview,
	StatusApproved,
	StatusEscalated,
	StatusInvalidated,
	StatusBlocked,
	StatusWithdrawn,
	StatusSuperseded,
	StatusCompleted,
	StatusArchived,
}

func TestFSM_ValidTransition_HappyPath(t *testing.T) {
	fsm := FSM{}

	cases := []struct {
		name     string
		from     ReviewStatus
		to       ReviewStatus
		role     Role
		counters BudgetCounters
	}{
		{"Unreviewed→InReview as Reviewer", StatusUnreviewed, StatusInReview, RoleReviewer, BudgetCounters{}},
		{"Unreviewed→InReview as Lead", StatusUnreviewed, StatusInReview, RoleLead, BudgetCounters{}},
		{"InReview→NeedsChanges", StatusInReview, StatusNeedsChanges, RoleReviewer, BudgetCounters{}},
		{"NeedsChanges→ChangesSubmitted", StatusNeedsChanges, StatusChangesSubmitted, RoleAuthor, BudgetCounters{0, 0}},
		{"ChangesSubmitted→ReReview", StatusChangesSubmitted, StatusReReview, RoleReviewer, BudgetCounters{0, 0}},
		{"InReview→Approved", StatusInReview, StatusApproved, RoleAdmin, BudgetCounters{}},
		{"InReview→Escalated as Lead", StatusInReview, StatusEscalated, RoleLead, BudgetCounters{}},
		{"InReview→Escalated as Admin", StatusInReview, StatusEscalated, RoleAdmin, BudgetCounters{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fsm.Transition(tc.from, tc.to, tc.role, tc.counters)
			if err != nil {
				t.Errorf("expected valid %s → %s for role %s, got: %v",
					tc.from, tc.to, tc.role, err)
			}
		})
	}
}

func TestFSM_RoleGuardRejects(t *testing.T) {
	fsm := FSM{}

	cases := []struct {
		name string
		from ReviewStatus
		to   ReviewStatus
		role Role
	}{
		{"Author cannot Unreviewed→InReview", StatusUnreviewed, StatusInReview, RoleAuthor},
		{"Admin cannot Unreviewed→InReview", StatusUnreviewed, StatusInReview, RoleAdmin},
		{"Author cannot InReview→Escalated", StatusInReview, StatusEscalated, RoleAuthor},
		{"Reviewer cannot InReview→Escalated", StatusInReview, StatusEscalated, RoleReviewer},
		{"Admin cannot NeedsChanges→ChangesSubmitted", StatusNeedsChanges, StatusChangesSubmitted, RoleAdmin},
		{"Reviewer cannot Any→Withdrawn", StatusInReview, StatusWithdrawn, RoleReviewer},
		{"Lead cannot Any→Withdrawn", StatusApproved, StatusWithdrawn, RoleLead},
		{"Author cannot Any→Blocked", StatusInReview, StatusBlocked, RoleAuthor},
		{"Author cannot Any→Invalidated", StatusInReview, StatusInvalidated, RoleAuthor},
		{"Author cannot Completed→Archived", StatusCompleted, StatusArchived, RoleAuthor},
		{"Reviewer cannot Completed→Archived", StatusCompleted, StatusArchived, RoleReviewer},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fsm.Transition(tc.from, tc.to, tc.role, BudgetCounters{})
			if err == nil {
				t.Errorf("expected role-not-permitted for %s → %s with role %s, got nil",
					tc.from, tc.to, tc.role)
			}
		})
	}
}

func TestFSM_BudgetCheck_FixRoundsExhausted(t *testing.T) {
	fsm := FSM{}

	// At max fix rounds (3), NeedsChanges → ChangesSubmitted should be rejected.
	err := fsm.Transition(StatusNeedsChanges, StatusChangesSubmitted, RoleAuthor, BudgetCounters{FixRounds: MaxFixRounds})
	if err == nil {
		t.Errorf("expected budget exceeded error at max fix rounds")
	}
}

func TestFSM_BudgetCheck_FixRoundsOK(t *testing.T) {
	fsm := FSM{}

	// Below max fix rounds, transition should succeed.
	err := fsm.Transition(StatusNeedsChanges, StatusChangesSubmitted, RoleAuthor, BudgetCounters{FixRounds: MaxFixRounds - 1})
	if err != nil {
		t.Errorf("expected valid transition below max fix rounds, got: %v", err)
	}
}

func TestFSM_BudgetCheck_ScopedValidationsExhausted(t *testing.T) {
	fsm := FSM{}

	err := fsm.Transition(StatusChangesSubmitted, StatusReReview, RoleReviewer, BudgetCounters{ScopedValidations: MaxScopedValidations})
	if err == nil {
		t.Errorf("expected budget exceeded error at max scoped validations")
	}
}

func TestFSM_BudgetCheck_ScopedValidationsOK(t *testing.T) {
	fsm := FSM{}

	err := fsm.Transition(StatusChangesSubmitted, StatusReReview, RoleReviewer, BudgetCounters{ScopedValidations: MaxScopedValidations - 1})
	if err != nil {
		t.Errorf("expected valid transition below max scoped validations, got: %v", err)
	}
}

func TestFSM_InvalidTransition(t *testing.T) {
	fsm := FSM{}

	cases := []struct {
		name string
		from ReviewStatus
		to   ReviewStatus
		role Role
	}{
		{"Unreviewed→Approved (no wildcard)", StatusUnreviewed, StatusApproved, RoleAdmin},
		{"Approved→InReview (no reverse)", StatusApproved, StatusInReview, RoleAdmin},
		{"Withdrawn→InReview (terminal)", StatusWithdrawn, StatusInReview, RoleAdmin},
		{"Blocked→Approved (terminal)", StatusBlocked, StatusApproved, RoleAdmin},
		{"Invalidated→Unreviewed (terminal)", StatusInvalidated, StatusUnreviewed, RoleAdmin},
		// Wildcard "Any →" transitions exist for: Invalidated, Blocked, Withdrawn, Completed.
		// Unreviewed→Completed is VALID via wildcard (Any→Completed with Admin).
		// Archived→Completed is VALID via wildcard (Any→Completed with Admin).
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fsm.Transition(tc.from, tc.to, tc.role, BudgetCounters{})
			if err == nil {
				t.Errorf("expected invalid transition %s → %s for role %s, got nil",
					tc.from, tc.to, tc.role)
			}
		})
	}
}

func TestFSM_New_SelfTransition(t *testing.T) {
	fsm := FSM{}

	for _, s := range newStatuses {
		t.Run(string(s), func(t *testing.T) {
			if err := fsm.Transition(s, s, RoleAuthor, BudgetCounters{}); err != nil {
				t.Errorf("self-transition for %s should be valid, got: %v", s, err)
			}
		})
	}
}

func TestFSM_GuardTable_Completeness(t *testing.T) {
	fsm := FSM{}
	entries := fsm.GuardTable()

	// We expect 12 guard entries (one per transition in the spec table).
	if len(entries) != 12 {
		t.Errorf("expected 12 guard entries, got %d", len(entries))
	}

	// Verify every entry has at least one role.
	for _, e := range entries {
		if len(e.Roles) == 0 {
			t.Errorf("guard entry %s → %s has no roles", e.From, e.To)
		}
	}

	// Verify wildcard entries have correct targets.
	anyTargets := make(map[ReviewStatus]bool)
	for _, e := range entries {
		if e.From == "" {
			anyTargets[e.To] = true
		}
	}
	for _, expected := range []ReviewStatus{StatusInvalidated, StatusBlocked, StatusWithdrawn, StatusCompleted} {
		if !anyTargets[expected] {
			t.Errorf("missing wildcard entry for → %s", expected)
		}
	}
}

func TestFSM_EveryStatusExists(t *testing.T) {
	// Verify all 13 new statuses are distinct and different from old ones.
	oldStatuses := allStatuses
	for _, old := range oldStatuses {
		for _, new := range newStatuses {
			if old == new && old != StatusCompleted && old != StatusArchived {
				t.Errorf("new status %s collides with old status %s", new, old)
			}
		}
	}
}
