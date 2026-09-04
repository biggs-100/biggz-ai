package main

import (
	"strings"
	"testing"
)

// TestSDDAttemptUnknownFlagFailClosed pins the fail-closed flag loop in
// sddAttemptRun (mirroring sddStatusRun): unknown flags such as --cwd or
// --change are refused with exit 1 instead of being silently ignored.
// <change> is positional and the workspace root is always os.Getwd().
// RED: pre-fix the loop had no default, so these flags were swallowed.
func TestSDDAttemptUnknownFlagFailClosed(t *testing.T) {
	for _, flag := range []string{"--cwd", "--change", "--bogus"} {
		code, _, stderr := runSDDAttemptCLI(t, "status", "thin", flag, "x")
		if code != 1 {
			t.Fatalf("flag %s: exit code = %d, want 1", flag, code)
		}
		if !strings.Contains(stderr, "unknown flag "+flag) {
			t.Fatalf("flag %s: stderr = %q, want it to contain %q", flag, stderr, "unknown flag "+flag)
		}
	}
}
