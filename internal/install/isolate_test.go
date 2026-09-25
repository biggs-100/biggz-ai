package install

import (
	"os"
	"testing"
)

// TestMain clears the ambient PI_CODING_AGENT_DIR before any test in this
// package binary runs.
//
// AgentConfigPath gives that variable priority over the HomeDir the caller
// passes, so on a machine where Pi pins its agent home — for example a
// gentle-shell install exporting PI_CODING_AGENT_DIR — every deploy helper in
// this package resolves to the developer's REAL pi agent home instead of the
// test's t.TempDir(). The failure mode is silent in one direction (assertions
// read the temp path and report "file not created") and destructive in the
// other (assets and settings.json land in the live agent home).
//
// Tests that exercise the override on purpose still set it explicitly with
// t.Setenv, which runs after TestMain; t.Setenv then restores the cleared
// value when the test ends.
func TestMain(m *testing.M) {
	os.Unsetenv("PI_CODING_AGENT_DIR")
	os.Exit(m.Run())
}
