package review

// Reviewer execute tests (tasks 1.1–1.3, 4.1–4.2): binary-owned prompt
// composition with the output contract before a byte-identical materialized
// task segment, the lens→role mapping with its typed role-unavailable refusal,
// and the executor contract — raw stdout reaches Capture unmodified and every
// failure captures nothing.

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// recordingReviewerRunner is the ReviewerRunner seam: it records the prompt it
// receives and scripts one Review outcome.
type recordingReviewerRunner struct {
	calls  int
	prompt string
	out    []byte
	err    error
	run    func(context.Context) ([]byte, error)
}

func (r *recordingReviewerRunner) Review(ctx context.Context, prompt string) ([]byte, error) {
	r.calls++
	r.prompt = prompt
	if r.run != nil {
		return r.run(ctx)
	}
	return r.out, r.err
}

// reviewerAsset reads one embedded reviewer role asset.
func reviewerAsset(t *testing.T, name string) []byte {
	t.Helper()
	asset, err := assets.FS.ReadFile("prompts/review/" + name)
	if err != nil {
		t.Fatalf("read reviewer role asset %s: %v", name, err)
	}
	return asset
}

// reviewerContract reads the binary-owned output-contract asset through the
// same assets.FS seam ComposeReviewerPrompt uses.
func reviewerContract(t *testing.T) []byte {
	t.Helper()
	contract, err := assets.FS.ReadFile(ReviewerOutputContractAsset)
	if err != nil {
		t.Fatalf("read reviewer output contract asset: %v", err)
	}
	return contract
}

// composeReviewerPromptWant renders shared ⊕ "\n" ⊕ role ⊕ contract ⊕ "\n---\n" ⊕ task.
func composeReviewerPromptWant(shared, role, contract, task []byte) []byte {
	prompt := bytes.Clone(shared)
	prompt = append(prompt, '\n')
	prompt = append(prompt, role...)
	prompt = append(prompt, contract...)
	prompt = append(prompt, "\n---\n"...)
	return append(prompt, task...)
}

// assertReviewerNoCapture proves a failure path captured nothing: the lineage
// store is byte-unchanged and the chain head still sits at the binding's
// expected revision.
func assertReviewerNoCapture(t *testing.T, store *Store, binding CaptureBinding, before map[string]string) {
	t.Helper()
	if !maps.Equal(before, materializeSnapshot(t, store.Dir)) {
		t.Fatal("failure path mutated the lineage store")
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.HeadHash != binding.ExpectedRevision {
		t.Fatalf("chain head moved %s → %s; no failure path may capture", binding.ExpectedRevision, chain.HeadHash)
	}
}

func TestComposeReviewerPrompt_SharedThenRoleThenByteIdenticalTask(t *testing.T) {
	_, binding, _ := captureFixture(t)
	for lens, roleAsset := range map[string]string{
		"risk": "r1-risk.md", "readability": "r2-readability.md",
		"reliability": "r3-reliability.md", "resilience": "r4-resilience.md",
	} {
		t.Run(lens, func(t *testing.T) {
			binding.Lens = lens
			task, err := MaterializeReviewerTask(binding)
			if err != nil {
				t.Fatalf("MaterializeReviewerTask: %v", err)
			}
			prompt, err := ComposeReviewerPrompt(binding)
			if err != nil {
				t.Fatalf("ComposeReviewerPrompt(%s): %v", lens, err)
			}
			shared := reviewerAsset(t, "shared.md")
			role := reviewerAsset(t, roleAsset)
			contract := reviewerContract(t)
			if !bytes.HasPrefix(prompt, shared) {
				t.Fatal("the shared role asset must open the prompt")
			}
			if !bytes.HasSuffix(prompt, task) {
				t.Fatal("the materialized task segment must close the prompt byte-identically")
			}
			// Segment order: shared < role < contract < separator < task.
			sharedAt := bytes.Index(prompt, shared)
			roleAt := bytes.Index(prompt, role)
			contractAt := bytes.Index(prompt, contract)
			separatorAt := bytes.Index(prompt, []byte(reviewerPromptSeparator))
			taskAt := len(prompt) - len(task)
			if !(sharedAt < roleAt && roleAt < contractAt && contractAt < separatorAt && separatorAt < taskAt) {
				t.Fatalf("segment order diverged: shared@%d role@%d contract@%d separator@%d task@%d",
					sharedAt, roleAt, contractAt, separatorAt, taskAt)
			}
			// Full byte equality leaves no channel for a caller-authored body:
			// the prompt is exactly shared ⊕ role ⊕ contract ⊕ separator ⊕ task.
			want := composeReviewerPromptWant(shared, role, contract, task)
			if !bytes.Equal(prompt, want) {
				t.Fatalf("prompt composition diverged: got %d bytes, want %d", len(prompt), len(want))
			}
		})
	}
}

// TestComposeReviewerPrompt_CarriesOutputContract (task 4.2): the
// binary-owned contract asset travels between the role text and the
// separator, states the admitted result shape in plain prose, and is never a
// Go template.
func TestComposeReviewerPrompt_CarriesOutputContract(t *testing.T) {
	contract := reviewerContract(t)
	for _, marker := range []string{
		"subject_hash", "inspection", "findings", "evidence",
		"evidence_class", "causal_disposition", "exactly one JSON object",
	} {
		if !bytes.Contains(contract, []byte(marker)) {
			t.Fatalf("output contract must state %q", marker)
		}
	}
	if bytes.Contains(contract, []byte("{{")) {
		t.Fatal("the output contract must not be a Go template")
	}
	_, binding, _ := captureFixture(t)
	prompt, err := ComposeReviewerPrompt(binding)
	if err != nil {
		t.Fatalf("ComposeReviewerPrompt: %v", err)
	}
	if !bytes.Contains(prompt, contract) {
		t.Fatal("the composed prompt must carry the output contract bytes verbatim")
	}
}

func TestExecutePiReview_RoleUnavailableRefusesWithoutLaunchOrCapture(t *testing.T) {
	store, binding, _ := captureFixture(t)
	before := materializeSnapshot(t, store.Dir)
	for _, lens := range []string{"performance", "dependencies"} {
		t.Run(lens, func(t *testing.T) {
			binding.Lens = lens
			runner := &recordingReviewerRunner{out: []byte("{}")}
			_, err := ExecutePiReview(t.Context(), binding, 0, runner)
			var failure *ReviewerFailure
			if !errors.As(err, &failure) || failure.Kind != ReviewerFailureRoleUnavailable || failure.Stage != ReviewerStageRole {
				t.Fatalf("err = %v, want typed %q at stage %q", err, ReviewerFailureRoleUnavailable, ReviewerStageRole)
			}
			if !strings.Contains(err.Error(), lens) {
				t.Fatalf("refusal must name the lens: %v", err)
			}
			if runner.calls != 0 {
				t.Fatalf("role-unavailable must launch nothing, got %d calls", runner.calls)
			}
			assertReviewerNoCapture(t, store, binding, before)
		})
	}
}

func TestExecutePiReview_RawStdoutReachesCapture(t *testing.T) {
	_, binding, _ := captureFixture(t)
	result, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	payload := captureResultJSON(t, binding, ManifestPaths(result.ChangedPathManifest), result.Subject.SubjectHash)
	wantPrompt, err := ComposeReviewerPrompt(binding)
	if err != nil {
		t.Fatalf("ComposeReviewerPrompt: %v", err)
	}
	runner := &recordingReviewerRunner{out: payload}
	outcome, err := ExecutePiReview(t.Context(), binding, 0, runner)
	if err != nil {
		t.Fatalf("ExecutePiReview: %v", err)
	}
	if runner.calls != 1 || runner.prompt != string(wantPrompt) {
		t.Fatal("the runner must receive the composed prompt byte-identically")
	}
	if outcome.Idempotent || outcome.Artifact.Revision == "" || outcome.Artifact.Revision == binding.ExpectedRevision {
		t.Fatalf("capture outcome = %+v, want a fresh advance", outcome)
	}
	// Replaying the same raw payload against the now-occupied slot is
	// idempotent only if the captured canonical bytes match it: any mutation
	// by the executor would conflict with the immutable slot.
	replay, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("replay Capture: %v", err)
	}
	if !replay.Idempotent || replay.Artifact.CanonicalSHA256 != outcome.Artifact.CanonicalSHA256 {
		t.Fatal("the executor must capture the runner's raw stdout unmodified")
	}
}

func TestExecutePiReview_TypedFailuresCaptureNothing(t *testing.T) {
	store, binding, _ := captureFixture(t)
	before := materializeSnapshot(t, store.Dir)
	tests := []struct {
		name      string
		timeout   time.Duration
		cancel    bool
		runner    *recordingReviewerRunner
		wantKind  ReviewerFailureKind
		wantStage ReviewerStage
	}{
		{name: "empty stdout", runner: &recordingReviewerRunner{out: []byte(" \n")},
			wantKind: ReviewerFailureEmptyOutput, wantStage: ReviewerStagePi},
		{name: "output over cap", runner: &recordingReviewerRunner{out: make([]byte, ArtifactResultLimit+1)},
			wantKind: ReviewerFailureOutputOverCap, wantStage: ReviewerStageOutput},
		{name: "nonzero exit", runner: &recordingReviewerRunner{err: &ReviewerFailure{
			Kind: ReviewerFailureNonzeroExit, Stage: ReviewerStagePi, ExitCode: 3}},
			wantKind: ReviewerFailureNonzeroExit, wantStage: ReviewerStagePi},
		{name: "timeout", timeout: 40 * time.Millisecond, runner: &recordingReviewerRunner{run: waitForReviewerContext},
			wantKind: ReviewerFailureTimeout, wantStage: ReviewerStagePi},
		{name: "cancelation", cancel: true, runner: &recordingReviewerRunner{run: waitForReviewerContext},
			wantKind: ReviewerFailureCanceled, wantStage: ReviewerStagePi},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				parent, cancel = context.WithCancel(parent)
				cancel()
			}
			_, err := ExecutePiReview(parent, binding, tc.timeout, tc.runner)
			var failure *ReviewerFailure
			if !errors.As(err, &failure) || failure.Kind != tc.wantKind || failure.Stage != tc.wantStage {
				t.Fatalf("err = %v, want typed kind %q at stage %q", err, tc.wantKind, tc.wantStage)
			}
			if tc.wantKind == ReviewerFailureOutputOverCap && failure.StdoutBytes != ArtifactResultLimit+1 {
				t.Fatalf("over-cap refusal must not truncate: stdout_bytes = %d", failure.StdoutBytes)
			}
			assertReviewerNoCapture(t, store, binding, before)
		})
	}
}

// waitForReviewerContext blocks until the executor's deadline or cancelation.
func waitForReviewerContext(ctx context.Context) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestExecutePiReview_BoundsAndOverMaxTimeout(t *testing.T) {
	if DefaultReviewerTimeout != 10*time.Minute || MaxReviewerTimeout != 2*time.Hour {
		t.Fatalf("timeout bounds = %s/%s, want 10m and 2h", DefaultReviewerTimeout, MaxReviewerTimeout)
	}
	store, binding, _ := captureFixture(t)
	before := materializeSnapshot(t, store.Dir)
	runner := &recordingReviewerRunner{out: []byte("{}")}
	_, err := ExecutePiReview(t.Context(), binding, MaxReviewerTimeout+time.Second, runner)
	if err == nil || !strings.Contains(err.Error(), MaxReviewerTimeout.String()) {
		t.Fatalf("over-max timeout err = %v, want a refusal naming %s", err, MaxReviewerTimeout)
	}
	if runner.calls != 0 {
		t.Fatalf("an over-max timeout must launch nothing, got %d calls", runner.calls)
	}
	assertReviewerNoCapture(t, store, binding, before)
}
