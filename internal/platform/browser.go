package platform

import (
	"os/exec"
	"runtime"
)

// OpenBrowserCmd returns an exec.Cmd that opens url in the default browser
// with Windows-safe branching: darwin → open, windows → rundll32, else
// xdg-open. The command's Dir is pinned via EnsureCommandDir so a deleted
// worktree does not crash Node-based children (uv_cwd). Caller should Start
// the command.
func OpenBrowserCmd(url string) *exec.Cmd {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	EnsureCommandDir(cmd)
	return cmd
}
