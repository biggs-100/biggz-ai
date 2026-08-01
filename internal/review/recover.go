// Chain authority recovery — `biggz review recover <lineage>` (Debt D3).
//
// Recover restores a LOST HEAD. When the HEAD file is missing but valid
// event files remain, it finds the deepest fully-verified chain and writes
// HEAD atomically, reporting the recovered head and the events kept. It is
// the read-only twin of repair:
//
//   - repair fixes a corrupt TAIL (truncates to the last valid event);
//   - recover restores a missing HEAD (never truncates, never removes).
//
// Recover never guesses: if HEAD exists the chain is validated from it and a
// healthy chain is a no-op ("authority intact"); mid-chain corruption is an
// error naming `biggz review export` as the recovery path, exactly like
// repair. A HEAD that names an unreadable/corrupt event is a corrupt tail —
// the repair verb's job — so recover refuses and names repair instead.
package review

import (
	"fmt"
)

// RecoverReport describes the outcome of `review recover`.
type RecoverReport struct {
	LineageID  string `json:"lineage_id"`
	Recovered  bool   `json:"recovered"`
	Action     string `json:"action,omitempty"` // "head_restored" when recovered
	HeadHash   string `json:"head_hash"`
	EventCount int    `json:"event_count"`
	Detail     string `json:"detail,omitempty"`
}

// Recover validates the lineage chain and restores a lost HEAD from the
// deepest fully-verified chain. A HEAD that exists with an intact chain is a
// no-op; mid-chain corruption is an error (recovery never guesses); a HEAD
// naming a corrupt event is a corrupt tail, which belongs to repair.
func Recover(repo, lineageID string) (RecoverReport, error) {
	store, err := Open(repo, lineageID)
	if err != nil {
		return RecoverReport{}, fmt.Errorf("recover: open store: %w", err)
	}
	var report RecoverReport
	err = WithFileLock(store.Dir, func() error {
		verified, corrupt, err := verifyEventFiles(store.Dir)
		if err != nil {
			return err
		}
		head, err := readHEAD(store.Dir)
		if err != nil {
			return fmt.Errorf("recover: read HEAD: %w", err)
		}

		// A present HEAD is authoritative: validate the chain it names and
		// report; recovery never rewrites or truncates an existing HEAD.
		if head != "" {
			if _, ok := verified[head]; !ok {
				// The event HEAD names is unreadable or fails verification:
				// a corrupt TAIL, which is exactly what repair truncates.
				return fmt.Errorf(
					"recover: HEAD names event %s which fails verification — that is a corrupt tail, not a lost head; run 'biggz review repair %s' to truncate it (recover only restores a missing HEAD)",
					head, lineageID)
			}
			broken, err := chainBrokenAt(verified, head)
			if err != nil {
				return err
			}
			if broken != "" {
				// Mid-chain corruption: recovery never guesses by dropping
				// reviewed history. Same rule as repair.
				return fmt.Errorf(
					"recover: chain corruption is mid-chain at %s (the event after it is unreadable or does not match its content address); recovery never guesses — recover with 'biggz review export %s' and re-import into a fresh lineage",
					broken, lineageID)
			}
			depths := make(map[string]int, len(verified))
			depth := chainDepth(verified, depths, head)
			report = RecoverReport{
				LineageID: lineageID, HeadHash: head, EventCount: depth,
				Action: "none", Detail: "authority intact",
			}
			if len(corrupt) > 0 {
				report.Detail = fmt.Sprintf("authority intact (%d unreadable record file(s) not on the chain)", len(corrupt))
			}
			return nil
		}

		// HEAD is missing: restore it from the deepest fully-verified chain.
		// The chain is re-derived from the verified record files alone; the
		// deepest chain wins exactly like repair's fallback, and corrupt
		// files are reported but never removed (recover truncates nothing).
		if len(verified) == 0 {
			report = RecoverReport{
				LineageID: lineageID, Action: "none",
				Detail: "no events to recover (empty lineage)",
			}
			return nil
		}
		depths := make(map[string]int, len(verified))
		lastValid, lastDepth := "", 0
		for name := range verified {
			depth := chainDepth(verified, depths, name)
			if depth > lastDepth {
				lastValid, lastDepth = name, depth
			}
		}
		if lastValid == "" {
			return fmt.Errorf(
				"recover: no valid event remains in the store; recover the lineage bytes with 'biggz review export %s' before it degrades further",
				lineageID)
		}
		if err := writeHEADFile(store.Dir, lastValid); err != nil {
			return fmt.Errorf("recover: restore HEAD: %w", err)
		}
		report = RecoverReport{
			LineageID: lineageID, Recovered: true, Action: "head_restored",
			HeadHash: lastValid, EventCount: lastDepth,
			Detail: fmt.Sprintf("HEAD restored to the deepest fully-verified chain head %s; %d event(s) kept", lastValid, lastDepth),
		}
		if len(corrupt) > 0 {
			report.Detail += fmt.Sprintf("; %d unreadable record file(s) left in place (recover never removes; 'biggz review repair %s' can truncate a corrupt tail)", len(corrupt), lineageID)
		}
		return nil
	})
	return report, err
}
