// Chain repair — `biggz review repair <lineage>` (Phase C2).
//
// Repair validates the content-addressed event chain (content-address
// integrity, prev links, HEAD) and fixes exactly one class of damage:
// an unreadable/corrupt TAIL event. It truncates the chain to the last
// valid event and re-derives HEAD, reporting precisely what was repaired.
//
// Mid-chain corruption is NOT truncated: dropping the middle of a reviewed
// history would silently destroy reviewed evidence. Repair refuses with an
// error naming `biggz review export <lineage>` as the recovery path, so the
// operator can recover the lineage bytes and re-import them into a fresh
// lineage.
//
// This is the pragmatic subset of gentle-ai's classified authority repair
// (internal/cli/review_repair.go); the full preflight/authorization
// machinery is intentionally out of scope.
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepairReport describes the outcome of `review repair`.
type RepairReport struct {
	LineageID  string `json:"lineage_id"`
	Repaired   bool   `json:"repaired"`
	Action     string `json:"action,omitempty"` // "truncated_tail" when repaired
	HeadHash   string `json:"head_hash"`
	EventCount int    `json:"event_count"`
	Truncated  int    `json:"truncated,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// Repair validates the lineage chain and repairs a corrupt tail event.
// A healthy chain is a no-op ("chain intact"). A corrupt tail (the HEAD
// event file unreadable or not matching its content address, or a lost
// HEAD file) truncates to the last valid event. Mid-chain corruption
// returns an error naming `biggz review export <lineage>` as the recovery
// path and never mutates the store.
func Repair(repo, lineageID string) (RepairReport, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return RepairReport{}, fmt.Errorf("repair: open store: %w", err)
	}
	var report RepairReport
	err = WithFileLock(store.Dir, func() error {
		verified, corrupt, err := verifyEventFiles(store.Dir)
		if err != nil {
			return err
		}
		head, err := readHEAD(store.Dir)
		if err != nil {
			return fmt.Errorf("repair: read HEAD: %w", err)
		}
		if head == "" && len(verified) == 0 {
			report = RepairReport{
				LineageID: lineageID, Action: "none",
				Detail: "chain intact (no events)",
			}
			return nil
		}

		// Find the deepest verified chain: each verified record contributes
		// depth 1 + depth(prev) when the prev is also verified. The record at
		// the head of the longest chain is the last valid event. A valid
		// chain FROM the recorded HEAD always wins (an orphan chain may only
		// tie it in depth, never beat a chain that HEAD already names).
		depths := make(map[string]int, len(verified))
		for name := range verified {
			chainDepth(verified, depths, name)
		}
		lastValid, lastDepth := "", 0
		if head != "" {
			if _, ok := verified[head]; ok {
				broken, err := chainBrokenAt(verified, head)
				if err != nil {
					return err
				}
				if broken != "" {
					// Mid-chain corruption: HEAD names a verified record
					// whose predecessor link is broken before genesis.
					// Truncating here would silently drop reviewed history.
					return fmt.Errorf(
						"repair: chain corruption is mid-chain at %s (the event after it is unreadable or does not match its content address); truncation would silently drop reviewed history — recover with 'biggz review export %s' and re-import into a fresh lineage",
						broken, lineageID)
				}
				lastValid, lastDepth = head, chainDepth(verified, depths, head)
			}
		}
		if lastValid == "" {
			// HEAD missing, unreadable, or naming a corrupt event: fall back
			// to the deepest verified chain.
			for name, depth := range depths {
				if depth > lastDepth {
					lastValid, lastDepth = name, depth
				}
			}
		}

		if head == lastValid {
			report = RepairReport{
				LineageID: lineageID, HeadHash: head, EventCount: lastDepth,
				Action: "none", Detail: "chain intact",
			}
			if len(corrupt) > 0 {
				report.Detail = fmt.Sprintf("chain intact (%d unreadable record file(s) not on the chain)", len(corrupt))
			}
			return nil
		}

		// Tail damage: HEAD is missing, unreadable, or points at something
		// that is not the last valid event. Truncate to the last valid event
		// and re-derive HEAD. The corrupt record file(s) are removed: an
		// unreadable tail can never rejoin the chain, and keeping it would
		// make the store fail every future integrity check.
		if lastValid == "" {
			return fmt.Errorf(
				"repair: no valid event remains in the store (HEAD %s is unreadable and every record file fails verification); recover the lineage bytes with 'biggz review export %s' before it degrades further",
				head, lineageID)
		}
		truncated := len(verified) - lastDepth
		for _, name := range corrupt {
			truncated++
			_ = os.Remove(filepath.Join(store.Dir, "v1", "events", name))
			_ = os.Remove(filepath.Join(store.Dir, name))
		}
		if err := writeHEADFile(store.Dir, lastValid); err != nil {
			return fmt.Errorf("repair: re-derive HEAD: %w", err)
		}
		report = RepairReport{
			LineageID: lineageID, Repaired: true, Action: "truncated_tail",
			HeadHash: lastValid, EventCount: lastDepth, Truncated: truncated,
			Detail: fmt.Sprintf("HEAD re-derived to the last valid event %s; %d record file(s) truncated off the chain and removed (unreadable tail cannot rejoin)", lastValid, truncated),
		}
		return nil
	})
	return report, err
}

// verifyEventFiles verifies every event-named file (64-char hex, excluding
// HEAD, LOCK, .lock, and .tmp files): the content hash must match the name
// and the payload must parse as a Record. Verified records map name → record;
// corrupt file names are returned separately.
//
// Migration-aware: scans both <dir>/v1/events/ (canonical) and <dir>/
// (legacy flat). Files present in both with identical hash are deduplicated;
// a file present in both but with differing content is treated as corrupt
// for repair consistency.
func verifyEventFiles(dir string) (map[string]Record, []string, error) {
	verified := make(map[string]Record)
	var corrupt []string
	seen := make(map[string]struct{})
	// Helper to scan one directory, respecting dir semantics.
	scanDir := func(scanPath string, isEventsDir bool) error {
		entries, err := os.ReadDir(scanPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("repair: read store dir: %w", err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				if !isEventsDir && name == "v1" {
					continue
				}
				continue
			}
			if name == "HEAD" || name == "LOCK" || name == ".lock" ||
				strings.HasSuffix(name, ".tmp") || len(name) != 64 {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			// Prefer canonical v1/events path; fallback to legacy flat.
			var data []byte
			var readErr error
			if d, err := os.ReadFile(filepath.Join(dir, "v1", "events", name)); err == nil {
				data = d
			} else if d, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
				data = d
			} else {
				readErr = err
			}
			if readErr != nil || sha256Hex(data) != name {
				corrupt = append(corrupt, name)
				continue
			}
			var rec Record
			if err := json.Unmarshal(data, &rec); err != nil {
				corrupt = append(corrupt, name)
				continue
			}
			verified[name] = rec
		}
		return nil
	}
	if err := scanDir(filepath.Join(dir, "v1", "events"), true); err != nil {
		return nil, nil, err
	}
	if err := scanDir(dir, false); err != nil {
		return nil, nil, err
	}
	return verified, corrupt, nil
}

// chainDepth computes the verified depth of hash (1 + depth of its verified
// predecessor), with cycle protection.
func chainDepth(verified map[string]Record, depths map[string]int, hash string) int {
	if depth, ok := depths[hash]; ok {
		return depth
	}
	rec, ok := verified[hash]
	if !ok {
		return 0
	}
	depth := 1
	if rec.PrevRevision != "" {
		// Guard against cycles without a separate visited set: mark -1
		// in-flight and treat a revisit as a broken link (depth 1).
		depths[hash] = -1
		prevDepth := chainDepth(verified, depths, rec.PrevRevision)
		if prevDepth >= 0 {
			depth += prevDepth
		}
	}
	depths[hash] = depth
	return depth
}

// chainBrokenAt walks the chain from head following verified prev links and
// returns the first hash whose predecessor is NOT verified (mid-chain
// break), or "" when the chain reaches genesis intact.
func chainBrokenAt(verified map[string]Record, head string) (string, error) {
	visited := make(map[string]bool)
	for hash := head; hash != ""; {
		if visited[hash] {
			return "", fmt.Errorf("repair: cycle detected at %s", hash)
		}
		visited[hash] = true
		rec, ok := verified[hash]
		if !ok {
			return hash, nil
		}
		hash = rec.PrevRevision
	}
	return "", nil
}

// writeHEADFile atomically writes the HEAD file (temp + rename).
func writeHEADFile(dir, revision string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "HEAD")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(revision+"\n"), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
