//go:build windows

package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/pathquote"
)

// ErrWindowsBinaryLock is returned only when the async staging also fails.
var ErrWindowsBinaryLock = errors.New("cannot replace running binary on Windows; use ReplaceHint")

// ReplaceBinary attempts to replace the running Windows binary.
//
// Strategy:
//  1. Try a direct os.Rename(src, dst) — succeeds in tests and when the
//     binary is not locked.
//  2. On failure (access denied / lock), move src to dst+".new" and spawn a
//     detached cmd that waits ~1s and moves it into place. The caller treats
//     the staged rename as success; the actual replacement happens asynchronously
//     after this process exits.
func ReplaceBinary(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	newPath := dst + ".new"
	// Ensure the temp src is moved to the staged path.
	if err := os.Rename(src, newPath); err != nil {
		return fmt.Errorf("stage Windows binary %q to %q: %w", src, newPath, err)
	}
	// Write a transient .bat that waits ~1s then moves the staged binary.
	batPath := filepath.Join(os.TempDir(), fmt.Sprintf("biggz-upgrade-%d.bat", os.Getpid()))
	// Windows-safe quoting preserves backslashes; %q would double them.
	batContent := fmt.Sprintf("@echo off\r\nping -n 2 127.0.0.1 >nul\r\nmove /y %s %s >nul 2>&1\r\ndel %%~f0\r\n", pathquote.Quote(newPath), pathquote.Quote(dst))
	if err := os.WriteFile(batPath, []byte(batContent), 0644); err != nil {
		// Staging succeeded; the user can restart manually.
		return nil
	}
	// Spawn detached mover: cmd /c start /b cmd /c "batPath"
	_ = exec.Command("cmd", "/c", "start", "/b", "cmd", "/c", batPath).Start()
	return nil
}

// ReplaceHint returns a Windows-appropriate message explaining that the
// binary was staged and will be replaced on restart.
func ReplaceHint(modulePath string) string {
	return fmt.Sprintf("Binary staged for replacement on next restart (restart to complete). Alternatively, run:\n  go install %s@latest", modulePath)
}
