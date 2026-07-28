package model

import "fmt"

// transitionMap defines all valid state transitions.
// The outer key is the current state; the inner key is the target state.
// A transition is valid iff transitionMap[current][target] is true.
// Self-transitions (current → current) are always valid no-ops.
var transitionMap = map[ReviewStatus]map[ReviewStatus]bool{
	StatusPending: {
		StatusInProgress: true,
		StatusFailed:     true,
	},
	StatusInProgress: {
		StatusCompleted: true,
		StatusFailed:    true,
	},
	StatusCompleted: {
		StatusInProgress: true, // correction cycle
		StatusArchived:   true,
	},
	StatusArchived: {},
	StatusFailed:   {},
}

// Transition checks whether moving from current to target is a valid
// FSM transition. It returns nil if the transition is allowed, or an
// error describing why it is not.
//
// Transition is a pure function with no side effects — it does not
// modify any state. Callers are responsible for applying the change
// after a nil return.
func Transition(current, target ReviewStatus) error {
	if current == target {
		return nil
	}
	transitions, ok := transitionMap[current]
	if !ok {
		return fmt.Errorf("unknown current status: %s", current)
	}
	if !transitions[target] {
		return fmt.Errorf("invalid transition from %s to %s", current, target)
	}
	return nil
}

// AllowedTransitions returns the set of valid target states from a given
// current state, including the current state itself (self-transition).
func AllowedTransitions(current ReviewStatus) []ReviewStatus {
	transitions, ok := transitionMap[current]
	if !ok {
		return []ReviewStatus{current}
	}
	allowed := []ReviewStatus{current}
	for target := range transitions {
		allowed = append(allowed, target)
	}
	return allowed
}
