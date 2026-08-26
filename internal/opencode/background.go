package opencode

// Grouped isolation in OpenCode applies to scheduling only, not a security
// boundary. Concurrent background subagents are coordinated via scheduling
// (ordering/pacing) rather than filesystem security isolation. This mirrors
// gentle-ai's background launcher semantics where
// OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS toggles scheduling, not a sandbox.
//
// filecoord.Acquire remains the cooperative primitive: one non-blocking attempt
// returns BusyError on contention without mutating the protected resource;
// the caller owns retry pacing.

// BackgroundIsolationIsSchedulingOnly reports that grouped isolation is
// scheduling-only. Exposed for documentation and testing contract.
const BackgroundIsolationIsSchedulingOnly = true

// IsGroupedIsolationSchedulingOnly reports whether grouped isolation is
// scheduling-only (always true). Provided as a function for call parity with
// gentle-ai's capability checks.
func IsGroupedIsolationSchedulingOnly() bool { return BackgroundIsolationIsSchedulingOnly }

// GroupedIsolationMode describes the isolation mode for background subagents.
// Only Scheduling is supported; Security is intentionally absent.
type GroupedIsolationMode string

const (
	GroupedIsolationScheduling GroupedIsolationMode = "scheduling"
)

// IsolationMode returns the active grouped isolation mode. Always returns
// scheduling-only to prevent security-boundary assumptions.
func IsolationMode() GroupedIsolationMode { return GroupedIsolationScheduling }
