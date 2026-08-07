// Package assets_test verifies the embedded OpenCode plugin files keep the
// ported contract shapes from gentle-ai while preserving biggz's deliberate
// divergences (quarantine-to-file persistence).
package assets_test

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func readReviewPlugin(t *testing.T) string {
	t.Helper()
	data, err := assets.FS.ReadFile("opencode/plugins/review-result-artifacts.ts")
	if err != nil {
		t.Fatalf("Read(review-result-artifacts.ts) error = %v", err)
	}
	return string(data)
}

// TestReviewResultArtifactsPluginContract mirrors gentle-ai's
// TestReviewResultArtifactsPluginContract for the ported SDD-phase block and
// the privacy gate, adapted to biggz names (biggz binary, biggz-ai schema).
// The quarantine-to-file divergence is asserted PRESENT (gentle's contract
// forbids native persistence; biggz deliberately quarantines).
func TestReviewResultArtifactsPluginContract(t *testing.T) {
	source := readReviewPlugin(t)

	for _, want := range []string{
		`spawn("biggz"`,
		`const SDD_PHASES`,
		`const SDD_TASK_FAILURE_PREFIX`,
		`"biggz-ai.sdd-task-result-failure/v1"`,
		`"sdd_task_result_empty"`,
		`"sdd_task_result_malformed"`,
		`failedSDDSessions`,
		`extractionClass(cause, "sddClass")`,
		`function taskResult(output: unknown, subject: string, classification?: string)`,
		`biggz sdd-status --cwd ${shellQuote(cwd)} --json`,
		// Privacy gate ported from gentle-ai: env/email/abs-path regexes,
		// first-line-only, 512-char cap.
		`const REDACTION_MARKER = "<redacted>"`,
		`const ENV_ASSIGNMENT`,
		`const EMAIL_ADDRESS`,
		`const ABSOLUTE_PATH`,
		`const CAUSE_LIMIT = 512`,
		`function scrubText(value: string): string`,
		`function scrubbedCause(cause: unknown): string`,
		// SDD hooks on both tool boundaries.
		`if (isSDDPhase(subagent))`,
		`"tool.execute.before"`,
		`"tool.execute.after"`,
		`export default ReviewResultArtifactsPlugin`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("review-result-artifacts.ts missing %q", want)
		}
	}

	// SDD phase task result validation must attach a machine-readable class
	// under the sddClass property so sddTaskFailure can type the code.
	if !strings.Contains(source, `taskResult(output.output, "SDD phase", "sddClass")`) {
		t.Fatal("review-result-artifacts.ts must validate SDD task results with sddClass classification")
	}
	// The stored handoff is rethrown on any later launch of the same phase in
	// the same session.
	if !strings.Contains(source, `throw new Error(failure.handoff)`) {
		t.Fatal("review-result-artifacts.ts must rethrow the stored GENTLE_AI_SDD_FAILURE handoff")
	}

	// Biggz divergence (deliberate): raw payloads are quarantined to a durable
	// file under .git/biggz/preserved-results/ — never lost, never forwarded.
	for _, want := range []string{
		`"preserved-results"`,
		`writeFileSync(join(dir, fileName), raw, { flag: "wx" })`,
		`function preservedCaptureFailure(`,
		`function writePreserved(`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("review-result-artifacts.ts must keep the biggz quarantine divergence %q", want)
		}
	}

	// The scrubbed gate must be used on the preflight failure path instead of
	// verbatim forwarding of native text.
	if strings.Contains(source, "forward it verbatim") {
		t.Fatal("review-result-artifacts.ts still forwards native preflight text verbatim")
	}
	if !strings.Contains(source, "`${scrubbedCause(cause)}. `") {
		t.Fatal("review-result-artifacts.ts preflight failure path must forward scrubbedCause")
	}
}
