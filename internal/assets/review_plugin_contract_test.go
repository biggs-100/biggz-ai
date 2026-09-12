// Package assets_test verifies the embedded OpenCode plugin files keep the
// ported contract shapes from gentle-ai while preserving biggz's deliberate
// divergences (quarantine-to-file persistence).
package assets_test

import (
	"encoding/json"
	"maps"
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

// TestReviewResultArtifactsMaterializeTransportContract locks the S2 transport:
// the before hook replaces the caller-authored task body with the bytes
// `capture-result --materialize` prints, forwards them unchanged (the
// transport leg never trims or re-encodes), proves they carry the preflighted
// artifact subject, and keeps the capture leg with the completed binding.
func TestReviewResultArtifactsMaterializeTransportContract(t *testing.T) {
	source := readReviewPlugin(t)

	for _, want := range []string{
		// The materialize route replaces the binding/context injection.
		`"--materialize"`,
		`function materializeReviewerTask(`,
		`captureArgs(binding, "materialize")`,
		`output.args.prompt = materialized.task`,
		`function assertMaterializedSubject(`,
		`review materialize returned a different artifact subject`,
		// Verbatim transport: the materialize leg resolves the raw stdout
		// Buffer and never trims it.
		`function runNativeBytes(`,
		`resolve(Buffer.concat(stdout))`,
		`const task = raw.toString("utf8")`,
		// The before-hook binding survives to the capture leg.
		`reviewBindings`,
		`captureArgs(binding, "input")`,
		`"--input", "-"`,
		`"--preflight"`,
		// The negotiated collect binding always carries the provider-issued
		// repository context, so the sorted-key shapes must accept it (the
		// original strings listed revision before repository_context and
		// rejected every negotiated binding).
		`"lens,lineage,order,repository_context,revision,target"`,
		`"lens,lineage,order,repository_context,revision,subject_hash,target"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("review-result-artifacts.ts missing the S2 transport marker %q", want)
		}
	}

	// The caller prompt is discarded: the superseded injection route and the
	// hand-built GENTLE_AI_REVIEW_CONTEXT line must not survive.
	if strings.Contains(source, "injectReviewerContext") {
		t.Fatal("review-result-artifacts.ts must not keep the superseded binding/context injection route")
	}
	if strings.Contains(source, "JSON.stringify(preflight)") {
		t.Fatal("review-result-artifacts.ts must not hand-build GENTLE_AI_REVIEW_CONTEXT; the materialized bytes are the reviewer prompt")
	}
	if strings.Contains(source, `materialized.toString("utf8").trim()`) {
		t.Fatal("review-result-artifacts.ts must forward the materialized task verbatim (no trim)")
	}
}

// TestReviewerAgentsRunToolLess proves the review step ships tool-less
// reviewers on both overlay variants: every lens agent denies all tools, so
// the transported frozen bytes are the reviewer's only evidence (review
// spec: "Verbatim Transport and Tool-Less Reviewer").
func TestReviewerAgentsRunToolLess(t *testing.T) {
	reviewers := []string{"review-risk", "review-readability", "review-reliability", "review-resilience"}
	firstSeen := map[string]map[string]bool{}
	for _, overlay := range []string{
		"opencode/sdd-overlay-single.json",
		"opencode/sdd-overlay-multi.json",
	} {
		agents := readOverlayAgentTools(t, overlay)
		for _, name := range reviewers {
			tools, ok := agents[name]
			if !ok {
				t.Fatalf("%s must define reviewer agent %q", overlay, name)
			}
			if len(tools) != 1 || tools["*"] {
				t.Fatalf("%s agent %q tools = %v, want exactly {\"*\": false} (tool-less reviewer)", overlay, name, tools)
			}
			if previous, ok := firstSeen[name]; ok && !maps.Equal(previous, tools) {
				t.Fatalf("reviewer agent %q tools differ across overlays: %v vs %v", name, previous, tools)
			}
			firstSeen[name] = tools
		}
	}
}

// readOverlayAgentTools returns each agent's tools map from one embedded
// overlay.
func readOverlayAgentTools(t *testing.T, overlay string) map[string]map[string]bool {
	t.Helper()
	data, err := assets.FS.ReadFile(overlay)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", overlay, err)
	}
	var root struct {
		Agent map[string]struct {
			Tools map[string]bool `json:"tools"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("Unmarshal %s error = %v", overlay, err)
	}
	agents := make(map[string]map[string]bool, len(root.Agent))
	for name, agent := range root.Agent {
		agents[name] = agent.Tools
	}
	return agents
}
