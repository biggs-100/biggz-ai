package sdd

import (
	"fmt"
	"os"

	"github.com/biggs-100/biggz-ai/internal/sddattempt"
)

// BindApprovedReview binds an approved review lineage to an SDD change.
// It updates the runtime ledger with the binding revision and lineage and
// returns the ledger scope ("clone" | "machine") so callers can surface it.
func BindApprovedReview(changeName, repoRoot, lineageID, bindingRevision string) (string, error) {
	// Load the runtime ledger to verify it exists
	status, err := sddattempt.Status(changeName, repoRoot)
	if err != nil {
		return "", fmt.Errorf("read runtime status: %w", err)
	}

	// Validate: the change must not be complete
	if status.Complete {
		return "", fmt.Errorf("change %q is already complete, cannot bind review", changeName)
	}

	// Validate: the lineage must not be empty
	if lineageID == "" {
		return "", fmt.Errorf("lineage ID is required")
	}

	// Validate: binding revision must be a valid SHA-256 hex string (64 chars)
	if len(bindingRevision) != 64 {
		return "", fmt.Errorf("binding revision must be a 64-char SHA-256 hex string, got %d chars", len(bindingRevision))
	}

	// Read existing ledger
	store, err := sddattempt.LoadStore(changeName, repoRoot)
	if err != nil {
		return "", fmt.Errorf("load runtime store: %w", err)
	}

	// Set binding info
	store.BindingRevision = bindingRevision
	store.BindingLineage = lineageID
	store.EvidenceRevision = bindingRevision

	// Save
	if err := sddattempt.SaveStore(store, repoRoot); err != nil {
		return "", fmt.Errorf("save runtime store: %w", err)
	}

	return status.Scope, nil
}

// ValidateReviewBinding checks that a change has a valid review binding.
func ValidateReviewBinding(changeName, repoRoot, expectedLineage string) error {
	store, err := sddattempt.LoadStore(changeName, repoRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no runtime ledger for change %q — has sdd-attempt begin been run?", changeName)
		}
		return fmt.Errorf("load runtime store: %w", err)
	}

	if store.BindingRevision == "" {
		return fmt.Errorf("change %q has no review binding — run `biggz review bind-sdd` first", changeName)
	}

	if expectedLineage != "" && store.BindingLineage != expectedLineage {
		return fmt.Errorf("binding lineage mismatch: expected %q, got %q", expectedLineage, store.BindingLineage)
	}

	return nil
}
