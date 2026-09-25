package steps

import (
	"os"
	"testing"
)

// TestMain clears the ambient PI_CODING_AGENT_DIR before any test in this
// package runs.
//
// piAgentsDir and piExtensionsDir give that variable priority over the homeDir
// a caller passes. On a machine where Pi pins its agent home — for example a
// gentle-shell install exporting PI_CODING_AGENT_DIR — a test that seeds a
// stale extension with `piExtensionsDir(t.TempDir())` actually seeds the
// developer's REAL extensions directory, and the step under test then deletes
// those names from it. The step also writes pi agent files into the real
// agents directory.
//
// Tests that deliberately exercise the override keep setting it with t.Setenv,
// which runs after TestMain.
func TestMain(m *testing.M) {
	os.Unsetenv("PI_CODING_AGENT_DIR")
	os.Exit(m.Run())
}
