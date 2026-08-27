package model

import "fmt"

// ---------------------------------------------------------------------------
// 13-State FSM for Review Lifecycle
// ---------------------------------------------------------------------------
//
// The FSM Guard Table (from core-review spec):
//
// | Transition                      | Role Guard               | Precondition                    | Budget Check           |
// |---------------------------------|--------------------------|---------------------------------|------------------------|
// | Unreviewed     → InReview       | Reviewer, Lead           | Evidence non-empty              | None                   |
// | InReview       → NeedsChanges   | Reviewer, Lead           | None                            | None                   |
// | NeedsChanges   → ChangesSubmitted| Author                  | None                            | FixRounds < max        |
// | ChangesSubmitted → ReReview     | Reviewer, Lead           | None                            | ScopedValidations < max|
// | InReview       → Approved       | Reviewer, Lead, Admin    | All policies pass               | None                   |
// | InReview       → Escalated      | Lead, Admin              | Escalation reason provided      | None                   |
// | *              → Invalidated    | Admin                    | Scope change detected           | None                   |
// | *              → Blocked        | Lead, Admin              | Policy violation                | None                   |
// | *              → Withdrawn      | Author                   | None                            | None                   |
// | Approved       → Superseded     | Lead, Admin              | Superseding review exists       | None                   |
// | *              → Completed      | Lead, Admin              | All policies pass, receipt valid| None                   |
// | Completed      → Archived       | Lead, Admin              | 30-day minimum since Complete   | None                   |
//
// "Any state" (*) transitions are handled via wildcard entries (From == "").
// Preconditions are the caller's responsibility — the FSM validates role
// guards, transition legality, and budget counter limits.

// New 13-state review statuses (extension of the existing 5-state set).
const (
	StatusUnreviewed       ReviewStatus = "unreviewed"
	StatusInReview         ReviewStatus = "in_review"
	StatusNeedsChanges     ReviewStatus = "needs_changes"
	StatusChangesSubmitted ReviewStatus = "changes_submitted"
	StatusReReview         ReviewStatus = "re_review"
	StatusApproved         ReviewStatus = "approved"
	StatusEscalated        ReviewStatus = "escalated"
	StatusInvalidated      ReviewStatus = "invalidated"
	StatusBlocked          ReviewStatus = "blocked"
	StatusWithdrawn        ReviewStatus = "withdrawn"
	StatusSuperseded       ReviewStatus = "superseded"
	// StatusCompleted and StatusArchived are already defined in the 5-state set.
)

// GuardEntry represents a single row in the FSM guard table.
// From is the current state (empty string means "any state").
// To is the target state being transitioned to.
// Roles lists the actor roles permitted to perform this transition.
// Precondition is a human-readable description of the precondition;
// it is not enforced by the FSM (caller's responsibility).
// BudgetCheck identifies which budget counter to validate:
//
//	""              — no budget check
//	"fix-rounds"    — checks FixRounds < MaxFixRounds
//	"scoped-validations" — checks ScopedValidations < MaxScopedValidations
type GuardEntry struct {
	From         ReviewStatus `json:"from"`
	To           ReviewStatus `json:"to"`
	Roles        []Role       `json:"roles"`
	Precondition string       `json:"precondition"`
	BudgetCheck  string       `json:"budget_check,omitempty"`
}

// guardTable is the complete set of valid transitions for the
// 13-state FSM. See the doc comment at the top of this file for
// the formatted table.
var guardTable = []GuardEntry{
	// Unreviewed → InReview: Reviewer, Lead
	{From: StatusUnreviewed, To: StatusInReview, Roles: []Role{RoleReviewer, RoleLead}, Precondition: "evidence-non-empty"},
	// InReview → NeedsChanges: Reviewer, Lead
	{From: StatusInReview, To: StatusNeedsChanges, Roles: []Role{RoleReviewer, RoleLead}},
	// NeedsChanges → ChangesSubmitted: Author
	{From: StatusNeedsChanges, To: StatusChangesSubmitted, Roles: []Role{RoleAuthor}, BudgetCheck: "fix-rounds"},
	// ChangesSubmitted → ReReview: Reviewer, Lead
	{From: StatusChangesSubmitted, To: StatusReReview, Roles: []Role{RoleReviewer, RoleLead}, BudgetCheck: "scoped-validations"},
	// InReview → Approved: Reviewer, Lead, Admin
	{From: StatusInReview, To: StatusApproved, Roles: []Role{RoleReviewer, RoleLead, RoleAdmin}, Precondition: "all-policies-pass"},
	// InReview → Escalated: Lead, Admin
	{From: StatusInReview, To: StatusEscalated, Roles: []Role{RoleLead, RoleAdmin}, Precondition: "escalation-reason-provided"},
	// Any → Invalidated: Admin
	{From: "", To: StatusInvalidated, Roles: []Role{RoleAdmin}, Precondition: "scope-change-detected"},
	// Any → Blocked: Lead, Admin
	{From: "", To: StatusBlocked, Roles: []Role{RoleLead, RoleAdmin}, Precondition: "policy-violation"},
	// Any → Withdrawn: Author
	{From: "", To: StatusWithdrawn, Roles: []Role{RoleAuthor}},
	// Approved → Superseded: Lead, Admin
	{From: StatusApproved, To: StatusSuperseded, Roles: []Role{RoleLead, RoleAdmin}, Precondition: "superseding-review-exists"},
	// Any → Completed: Lead, Admin
	{From: "", To: StatusCompleted, Roles: []Role{RoleLead, RoleAdmin}, Precondition: "all-policies-pass-receipt-valid"},
	// Completed → Archived: Lead, Admin
	{From: StatusCompleted, To: StatusArchived, Roles: []Role{RoleLead, RoleAdmin}, Precondition: "30-day-minimum"},
}

// ---------------------------------------------------------------------------
// FSM — 13-state with role guards and budget counters
// ---------------------------------------------------------------------------

// FSM is a stateless validator for the 13-state review lifecycle.
// It validates transition legality, role guards, and budget counters.
// Precondition evaluation (policy checks, evidence requirements, etc.)
// is the caller's responsibility.
type FSM struct{}

// Transition validates a state transition. It returns nil if the
// transition is allowed given the current state, target state, actor
// role, and current budget counter values.
//
// Self-transitions (current == target) are always valid.
func (FSM) Transition(current, target ReviewStatus, role Role, counters BudgetCounters) error {
	if current == target {
		return nil
	}

	entry := findGuardEntry(current, target)
	if entry == nil {
		return fmt.Errorf("invalid transition from %s to %s", current, target)
	}

	if !containsRole(entry.Roles, role) {
		return fmt.Errorf("role %s not permitted for transition %s → %s", role, current, target)
	}

	if err := checkBudget(entry.BudgetCheck, counters); err != nil {
		return err
	}

	return nil
}

// GuardTable returns the complete guard table for inspection, debugging,
// and documentation purposes.
func (FSM) GuardTable() []GuardEntry {
	result := make([]GuardEntry, len(guardTable))
	copy(result, guardTable)
	return result
}

// findGuardEntry looks up a guard entry by (from, to). It first searches
// for an exact match, then falls back to wildcard entries (from == "").
func findGuardEntry(from, to ReviewStatus) *GuardEntry {
	for i := range guardTable {
		if guardTable[i].From == from && guardTable[i].To == to {
			return &guardTable[i]
		}
	}
	for i := range guardTable {
		if guardTable[i].From == "" && guardTable[i].To == to {
			return &guardTable[i]
		}
	}
	return nil
}

// containsRole checks whether the given role is in the allowed list.
func containsRole(roles []Role, role Role) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// checkBudget returns an error if the budget check named by check is
// exceeded, or nil if no check is required or the budget is not exceeded.
func checkBudget(check string, counters BudgetCounters) error {
	switch check {
	case "fix-rounds":
		if counters.FixRounds >= MaxFixRounds {
			return fmt.Errorf("budget exceeded: fix rounds exhausted (%d/%d)",
				counters.FixRounds, MaxFixRounds)
		}
	case "scoped-validations":
		if counters.ScopedValidations >= MaxScopedValidations {
			return fmt.Errorf("budget exceeded: scoped validations exhausted (%d/%d)",
				counters.ScopedValidations, MaxScopedValidations)
		}
	}
	return nil
}
