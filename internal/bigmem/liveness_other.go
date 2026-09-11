//go:build !windows

package bigmem

import (
	"os"
	"strings"
)

// probeDBLiveness implements the non-Windows liveness signal for REQ-GW1/GW2.
// It keeps the legacy O_CREATE|O_EXCL claim probe, but that probe never opens
// bigmem.db: it can only detect a racing *probe*, never a live holder (e.g. a
// running biggz-mcp). O_EXCL success is therefore NOT proof of death and MUST
// be reported as inconclusive (holder=false, proven=false), per REQ-GW2
// (reclaim only once the probe proves the previous holder dead) and REQ-GW3
// (no live holder proven -> keep wal/shm and the recovered fallback). A failed
// probe (pre-held claim file) is inconclusive too.
//
// Recorded tradeoff (safety over aggressiveness): a genuinely dead stale
// ghost off Windows no longer reclaims; it takes the recovered fallback.
// The alternative maps O_EXCL success to proven-dead and lets
// reclaimStaleWAL's os.Remove delete a possibly-live holder's files, because
// Unix removes open files. A POSIX advisory-lock probe (fcntl on SQLite's
// lock bytes) could restore non-Windows reclaiming; out of scope here.
func probeDBLiveness(dbPath string) (holder, proven bool) {
	if strings.TrimSpace(dbPath) == "" {
		return false, false
	}
	probePath := dbPath + ".ghost_probe"
	f, err := os.OpenFile(probePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return false, false
	}
	_ = f.Close()
	_ = os.Remove(probePath)
	return false, false
}
