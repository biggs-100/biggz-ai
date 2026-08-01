// Orphaned artifact reclamation — `biggz review reclaim <lineage>` (Debt D3).
//
// Reclaim garbage-collects orphaned artifacts: files under <lineage>/manifests/
// and <lineage>/receipts/ that NO event in the chain references are moved to
// <lineage>/trash/<timestamp>/ (never deleted outright), reporting the count
// and the moved paths. Files referenced by any event — the chain's lens_result
// manifest references and complete_review receipt references — are untouched,
// and chain events themselves are never reclaimed.
//
// Reclaim fails closed when the chain cannot be loaded: without the chain the
// referenced set is unknowable, and an unclassifiable artifact must never be
// moved. Run `biggz review repair`/`recover` first.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReclaimReport describes the outcome of `review reclaim`.
type ReclaimReport struct {
	LineageID string   `json:"lineage_id"`
	TrashDir  string   `json:"trash_dir"`
	Reclaimed int      `json:"reclaimed"`
	Paths     []string `json:"paths"`
	Detail    string   `json:"detail,omitempty"`
}

// Reclaim moves artifacts under manifests/ and receipts/ that no chain event
// references into <lineage>/trash/<ts>/ and reports them. Referenced files
// and every chain event stay untouched.
func Reclaim(repo, lineageID string) (ReclaimReport, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return ReclaimReport{}, fmt.Errorf("reclaim: open store: %w", err)
	}
	var report ReclaimReport
	report.LineageID = lineageID
	err = WithFileLock(store.Dir, func() error {
		chain, err := store.LoadChain()
		if err != nil {
			return fmt.Errorf("reclaim: load chain: %w", err)
		}
		referenced, err := referencedArtifacts(chain)
		if err != nil {
			return fmt.Errorf("reclaim: %w", err)
		}
		orphans := make([]string, 0)
		for _, dirName := range []string{ManifestsDirName, ReceiptsDirName} {
			dir := filepath.Join(store.Dir, dirName)
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("reclaim: read %s: %w", dirName, err)
			}
			for _, entry := range entries {
				if entry.IsDir() || strings.HasSuffix(entry.Name(), ".tmp") {
					continue
				}
				rel := dirName + "/" + entry.Name()
				if _, ok := referenced[rel]; ok {
					continue
				}
				orphans = append(orphans, rel)
			}
		}
		if len(orphans) == 0 {
			report.Detail = "no orphaned artifacts"
			return nil
		}
		trashDir := filepath.Join(store.Dir, "trash", time.Now().UTC().Format("20060102T150405.000000000Z"))
		if err := os.MkdirAll(trashDir, 0755); err != nil {
			return fmt.Errorf("reclaim: create trash dir: %w", err)
		}
		moved := make([]string, 0, len(orphans))
		for _, rel := range orphans {
			src := filepath.Join(store.Dir, filepath.FromSlash(rel))
			dst := filepath.Join(trashDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return fmt.Errorf("reclaim: create trash subdir: %w", err)
			}
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("reclaim: move %s to trash: %w", rel, err)
			}
			moved = append(moved, rel)
		}
		report.TrashDir = filepath.ToSlash(filepath.Base(filepath.Dir(trashDir)) + "/" + filepath.Base(trashDir))
		report.Reclaimed = len(moved)
		report.Paths = moved
		report.Detail = fmt.Sprintf("%d artifact(s) moved to trash (referenced artifacts and chain events untouched)", len(moved))
		return nil
	})
	return report, err
}

// referencedArtifacts collects every artifacts/ and receipts/ relative path
// that any chain event references: lens_result events reference their manifest
// path, complete_review events reference their receipt path. Paths are
// normalized to forward slashes (event payloads store OS-native paths).
func referencedArtifacts(chain ValidatedChain) (map[string]struct{}, error) {
	referenced := make(map[string]struct{})
	for index := range chain.Records {
		rec := &chain.Records[index]
		switch rec.Operation {
		case LensResultOperation:
			var payload lensResultEventPayload
			if err := json.Unmarshal(rec.Payload, &payload); err != nil {
				return nil, fmt.Errorf("chain lens_result event %d is malformed: %w", index, err)
			}
			if payload.ManifestPath != "" {
				referenced[filepath.ToSlash(payload.ManifestPath)] = struct{}{}
			}
		case CompleteReviewOperation:
			var evt completeEventPayload
			if err := json.Unmarshal(rec.Payload, &evt); err != nil {
				return nil, fmt.Errorf("chain complete_review event %d is malformed: %w", index, err)
			}
			if evt.ReceiptPath != "" {
				referenced[filepath.ToSlash(evt.ReceiptPath)] = struct{}{}
			}
		}
	}
	return referenced, nil
}
