//go:build windows

package bigmem

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows"
)

// probeDBLiveness implements the Windows liveness signal for REQ-GW1/GW2:
// opening the primary DB with share mode 0 succeeds only when no other handle
// (e.g. a running biggz-mcp) holds the file open. Success therefore proves the
// previous holder is dead; ERROR_SHARING_VIOLATION / ERROR_LOCK_VIOLATION prove
// a live holder. A missing file also proves no holder. Any other failure is
// inconclusive, which keeps the recovered fallback intact (REQ-GW3).
func probeDBLiveness(dbPath string) (holder, proven bool) {
	if strings.TrimSpace(dbPath) == "" {
		return false, false
	}
	p, err := windows.UTF16PtrFromString(dbPath)
	if err != nil {
		return false, false
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		_ = windows.CloseHandle(h)
		return false, true
	}
	switch {
	case errors.Is(err, windows.ERROR_SHARING_VIOLATION), errors.Is(err, windows.ERROR_LOCK_VIOLATION):
		return true, true
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND), errors.Is(err, windows.ERROR_PATH_NOT_FOUND):
		return false, true
	default:
		return false, false
	}
}
