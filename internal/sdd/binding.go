package sdd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/biggz-ai/biggz/internal/sddattempt"
)

// BindApprovedReview binds an approved review lineage to an SDD change.
// It updates the runtime ledger with the binding revision and lineage.
func BindApprovedReview(changeName, repoRoot, lineageID, bindingRevision string) error {
	// Load the runtime ledger to verify it exists
	status, err := sddattempt.Status(changeName, repoRoot)
	if err != nil {
		return fmt.Errorf("read runtime status: %w", err)
	}

	// Validate: the change must not be complete
	if status.Complete {
		return fmt.Errorf("change %q is already complete, cannot bind review", changeName)
	}

	// Validate: the lineage must not be empty
	if lineageID == "" {
		return fmt.Errorf("lineage ID is required")
	}

	// Validate: binding revision must be a valid SHA-256 hex string (64 chars)
	if len(bindingRevision) != 64 {
		return fmt.Errorf("binding revision must be a 64-char SHA-256 hex string, got %d chars", len(bindingRevision))
	}

	// Store binding info in the runtime ledger
	// We do this by creating a "finish" with binding info but no attempt change.
	// Actually, we need a dedicated binding store. Let's use the attempt ledger.
	// The easiest approach: create a small binding file alongside the runtime store.

	bindingDir := filepath.Join(repoRoot, ".biggz", "sdd-runtime")
	if err := os.MkdirAll(bindingDir, 0755); err != nil {
		return fmt.Errorf("mkdir binding dir: %w", err)
	}

	// Read existing ledger
	store, err := sddattempt.LoadStore(changeName, repoRoot)
	if err != nil {
		return fmt.Errorf("load runtime store: %w", err)
	}

	// Set binding info
	store.BindingRevision = bindingRevision
	store.BindingLineage = lineageID
	store.EvidenceRevision = bindingRevision

	// Save
	if err := sddattempt.SaveStore(store, repoRoot); err != nil {
		return fmt.Errorf("save runtime store: %w", err)
	}

	return nil
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
