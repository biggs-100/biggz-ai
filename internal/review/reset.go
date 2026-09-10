// Store reset — `biggz review reset <lineage> --confirm --reason "..."`
// (last resort).
//
// Reset moves the ENTIRE lineage directory to
// <store-root>/trash/reset-<ts>-<lineage>/ (never deleted outright) and
// writes a reset-record.json with who/when/why. It is the bounded answer to
// a corrupted store that `doctor --fix`, `repair`, and `recover` cannot
// handle — the alternative used to be manual `rm -rf` with no audit trail
// and no recovery path.
//
// Bounded by design:
//   - reason is mandatory (empty reason refuses);
//   - the file lock is held during the move, so a live review refuses
//     instead of being ripped out from under its holder;
//   - nothing is ever deleted: the trash dir keeps every byte plus the
//     audit record, and the lineage can be restored by moving it back.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ResetReport describes the outcome of `review reset`.
type ResetReport struct {
	LineageID string `json:"lineage_id"`
	TrashedTo string `json:"trashed_to"`
	Reason    string `json:"reason"`
	Detail    string `json:"detail,omitempty"`
}

// ResetRecord is the audit record written into the trashed lineage dir.
type ResetRecord struct {
	LineageID   string `json:"lineage_id"`
	Reason      string `json:"reason"`
	ResetAt     string `json:"reset_at"`
	TrashedFrom string `json:"trashed_from"`
}

// Reset moves the whole lineage directory to trash and records why.
// An empty reason refuses: resets without documented cause are how
// evidence silently disappears.
func Reset(repo, lineageID, reason string) (ResetReport, error) {
	if lineageID == "" {
		return ResetReport{}, fmt.Errorf("reset: lineage id is required")
	}
	if reason == "" {
		return ResetReport{}, fmt.Errorf("reset: --reason is required (resets without documented cause are refused)")
	}
	store, err := Open(repo, lineageID)
	if err != nil {
		return ResetReport{}, fmt.Errorf("reset: open store: %w", err)
	}
	if _, err := os.Stat(store.Dir); err != nil {
		return ResetReport{}, fmt.Errorf("reset: no such lineage %q", lineageID)
	}
	var report ResetReport
	report.LineageID = lineageID
	report.Reason = reason
	err = WithFileLock(store.Dir, func() error {
		trashBase := filepath.Join(filepath.Dir(store.Dir), "trash")
		trashDir := filepath.Join(trashBase, "reset-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-"+lineageID)
		if err := os.MkdirAll(trashBase, 0755); err != nil {
			return fmt.Errorf("reset: create trash base: %w", err)
		}
		if err := os.Rename(store.Dir, trashDir); err != nil {
			return fmt.Errorf("reset: move lineage to trash: %w", err)
		}
		rec := ResetRecord{
			LineageID:   lineageID,
			Reason:      reason,
			ResetAt:     time.Now().UTC().Format(time.RFC3339),
			TrashedFrom: store.Dir,
		}
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return fmt.Errorf("reset: encode record: %w", err)
		}
		if err := os.WriteFile(filepath.Join(trashDir, "reset-record.json"), data, 0644); err != nil {
			return fmt.Errorf("reset: write record: %w", err)
		}
		report.TrashedTo = trashDir
		report.Detail = "lineage moved to trash (nothing deleted); restore by moving it back"
		return nil
	})
	return report, err
}
