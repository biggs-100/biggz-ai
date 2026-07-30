package recoverytrace

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnprovenDeletion      = errors.New("deletion is not proven by a destination or an explicit no-retained-invariant declaration")
	ErrOrphanedInvariant     = errors.New("retained invariant has no owning context or exact proof reference")
	ErrMissingCredit         = errors.New("row does not credit a contributor")
	ErrCountMismatch         = errors.New("reconciliation counts do not match the recovered backlog")
	ErrUnauthorizedDeviation = errors.New("early removal deviation is not authorized by publication evidence")
	ErrUnknownDisposition    = errors.New("row carries an unknown disposition")
	ErrDuplicatePath         = errors.New("path carries more than one disposition")
	ErrBacklogMismatch       = errors.New("reconciliation counts do not match the backlog items behind them")
	ErrUnknownReleaseClass   = errors.New("backlog item carries a release classification outside the five")
)

// ValidateLedgers validates the entire ledger structure.
func ValidateLedgers(ledgers Ledgers, expected Reconciliation) error {
	if err := validateReconciliation(ledgers.Reconciliation, expected); err != nil {
		return err
	}
	if err := validateBacklog(ledgers.Reconciliation, ledgers.Backlog); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(ledgers.Rows))
	for _, row := range ledgers.Rows {
		if err := validateRow(row); err != nil {
			return err
		}
		if _, duplicate := seen[row.Path]; duplicate {
			return fmt.Errorf("%w: %s", ErrDuplicatePath, row.Path)
		}
		seen[row.Path] = struct{}{}
	}
	return nil
}

func validateReconciliation(rec, expected Reconciliation) error {
	if rec.Issues != expected.Issues ||
		rec.PullRequests != expected.PullRequests ||
		rec.CollisionPRs != expected.CollisionPRs ||
		rec.Overlaps != expected.Overlaps ||
		rec.Decompositions != expected.Decompositions {
		return fmt.Errorf("%w: got %+v, expected %+v", ErrCountMismatch, rec, expected)
	}
	return nil
}

func validateBacklog(rec Reconciliation, backlog []BacklogEntry) error {
	var issues, prs int
	for _, item := range backlog {
		if item.Release != "" && !item.Release.IsValid() {
			return fmt.Errorf("%w: %s", ErrUnknownReleaseClass, item.Release)
		}
		switch item.Kind {
		case "issue":
			issues++
		case "pull_request":
			prs++
		}
	}
	if issues != rec.Issues {
		return fmt.Errorf("%w: counted %d issues, reconciliation declares %d", ErrBacklogMismatch, issues, rec.Issues)
	}
	if prs != rec.PullRequests {
		return fmt.Errorf("%w: counted %d PRs, reconciliation declares %d", ErrBacklogMismatch, prs, rec.PullRequests)
	}
	return nil
}

func validateRow(row Row) error {
	if !row.Disposition.IsValid() {
		return fmt.Errorf("%w: %s", ErrUnknownDisposition, row.Disposition)
	}
	if row.Contributor == "" {
		return fmt.Errorf("%w: path %s", ErrMissingCredit, row.Path)
	}
	if row.Disposition == DispositionDelete &&
		row.DestinationPath == "" &&
		!row.NoRetainedInvariant {
		return fmt.Errorf("%w: %s", ErrUnprovenDeletion, row.Path)
	}
	if row.NoRetainedInvariant && row.DestinationPath != "" {
		return fmt.Errorf("contradiction: path %s has both a destination and a no-retained-invariant declaration", row.Path)
	}
	if row.Invariant != "" && row.Context == "" {
		return fmt.Errorf("%w: path %s has invariant %q but no context", ErrOrphanedInvariant, row.Path, row.Invariant)
	}
	if row.EarlyDeviation && len(row.Publication) == 0 {
		return fmt.Errorf("%w: path %s", ErrUnauthorizedDeviation, row.Path)
	}
	return nil
}

// NormalizeProject detects the project name from a path.
func NormalizeProject(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/\\"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}
