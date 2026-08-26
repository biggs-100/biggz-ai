package review

import (
	"fmt"
	"os"
	"time"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/model"
)

// Correction represents a single fix applied during a review cycle.
// This is intentionally minimal — budget, retry limits, and policy rules
// are evaluated externally via the FSM guard table and budget counters.
type Correction struct {
	ID           string    `json:"id"`
	Files        []string  `json:"files"`
	LinesChanged int       `json:"lines_changed"`
	Reason       string    `json:"reason"`
	BeforeHash   string    `json:"before_hash"` // SHA of file range (via filemerge.ComputeHash) OR evidence chain before correction
	AfterHash    string    `json:"after_hash"`   // SHA of evidence chain after correction
	CreatedAt    time.Time `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Hashline-guarded file helpers (Phase 3: hashline)
// ---------------------------------------------------------------------------
// These helpers implement the read-compute-store / write-validate cycle
// required by the hashline spec. The file's exact content hash is computed
// at read time and stored in Correction.BeforeHash; at write time it is
// validated via filemerge.ApplyWithHash (exact-range SHA-256 hex,
// warn-and-stop with needs_attention + freshHash, force bypasses).

// ComputeFileHash reads path and returns its exact-range SHA-256 hex via
// filemerge.ComputeHash. A missing file yields the hash of empty content.
func ComputeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return filemerge.ComputeHash(nil), nil
		}
		return "", err
	}
	return filemerge.ComputeHash(data), nil
}

// ReadFileWithHash reads path and returns (content, hash, error). The hash
// is the exact content hash suitable for storing in Correction.BeforeHash.
func ReadFileWithHash(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, filemerge.ComputeHash(nil), nil
		}
		return nil, "", err
	}
	return data, filemerge.ComputeHash(data), nil
}

// PrepareCorrection reads path, computes its hash, and returns a Correction
// with BeforeHash populated. The caller can then modify content and call
// ApplyCorrection to validate at write time.
func PrepareCorrection(path, reason string) (Correction, []byte, error) {
	data, hash, err := ReadFileWithHash(path)
	if err != nil {
		return Correction{}, nil, err
	}
	c := Correction{
		BeforeHash: hash,
		Reason:     reason,
		Files:      []string{path},
		CreatedAt:  time.Now(),
	}
	return c, data, nil
}

// ApplyCorrection validates the on-disk hash against correction.BeforeHash
// and atomically writes newContent when it matches (or when force is true).
// On mismatch it returns (freshHash, *filemerge.HashMismatchError) with
// Code "needs_attention" and FreshHash set to the current on-disk hash;
// the file remains unchanged and the batch must not abort. When the write
// succeeds it returns (newHash, nil) where newHash is the hash of
// newContent.
func ApplyCorrection(correction Correction, path string, newContent []byte, force bool) (string, error) {
	return filemerge.ApplyWithHash(path, correction.BeforeHash, newContent, force)
}

// WriteFileWithHash is a lower-level helper that validates expectedHash
// before writing. Prefer ApplyCorrection when a Correction is available.
func WriteFileWithHash(path, expectedHash string, newContent []byte, force bool) (string, error) {
	return filemerge.ApplyWithHash(path, expectedHash, newContent, force)
}

// ---------------------------------------------------------------------------
// Correction budget enforcement
// ---------------------------------------------------------------------------
//
// The budget counters (FixRounds, ScopedValidations) are tracked on the
// ReviewState and enforced by the FSM guard table. These functions provide
// direct budget validation for use in lifecycle methods.

// ValidateCorrectionBudget checks whether the fix-rounds budget allows
// a new correction cycle. Returns nil if the budget is not exhausted.
func ValidateCorrectionBudget(counters model.BudgetCounters) error {
	if counters.FixRounds >= model.MaxFixRounds {
		return fmt.Errorf("correction budget exceeded: fix rounds exhausted (%d/%d)",
			counters.FixRounds, model.MaxFixRounds)
	}
	return nil
}

// ValidateReReviewBudget checks whether the scoped-validations budget
// allows a re-review. Returns nil if the budget is not exhausted.
func ValidateReReviewBudget(counters model.BudgetCounters) error {
	if counters.ScopedValidations >= model.MaxScopedValidations {
		return fmt.Errorf("re-review budget exceeded: scoped validations exhausted (%d/%d)",
			counters.ScopedValidations, model.MaxScopedValidations)
	}
	return nil
}

// IncrementFixRound returns updated counters with FixRounds incremented.
func IncrementFixRound(counters model.BudgetCounters) model.BudgetCounters {
	return model.BudgetCounters{
		FixRounds:         counters.FixRounds + 1,
		ScopedValidations: counters.ScopedValidations,
	}
}

// IncrementScopedValidation returns updated counters with ScopedValidations
// incremented by one.
func IncrementScopedValidation(counters model.BudgetCounters) model.BudgetCounters {
	return model.BudgetCounters{
		FixRounds:         counters.FixRounds,
		ScopedValidations: counters.ScopedValidations + 1,
	}
}
