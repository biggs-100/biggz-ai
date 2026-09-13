package main

// Tasks 2.1–2.3: the `capture-result --agent pi --execute` CLI surface. The
// usage matrix is validated before the handshake and RDD gates; the pipeline
// runs start → execute (through a real `pi` shim on PATH) → finalize → gate
// with the shim's raw stdout captured byte-for-byte; every refusal captures
// nothing and leaves no partial slot.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// reviewExecuteEnv isolates HOME and the global RDD state for one test and
// declares (or clears) the pi relay handshake.
func reviewExecuteEnv(t *testing.T, handshake bool) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	contract := ""
	if handshake {
		contract = review.PiReviewRelayContract
	}
	t.Setenv(review.PiReviewRelayContractEnv, contract)
	t.Setenv(review.GentlePiReviewRelayContractEnv, "")
}

// reviewExecuteShim installs a real `pi` shim on PATH: it drains its stdin
// into a dump file, prints payload verbatim to stdout, and exits with
// exitCode. Windows gets a .cmd so LookPath exercises PATHEXT; elsewhere a
// POSIX shell script. It returns the stdin dump path.
func reviewExecuteShim(t *testing.T, payload []byte, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	golden := filepath.Join(dir, "golden.bin")
	if err := os.WriteFile(golden, payload, 0644); err != nil {
		t.Fatalf("write shim golden: %v", err)
	}
	dump := filepath.Join(dir, "stdin.bin")
	t.Setenv("BIGGZ_TEST_SHIM_GOLDEN", golden)
	t.Setenv("BIGGZ_TEST_SHIM_DUMP", dump)
	name, script := "pi", fmt.Sprintf("#!/bin/sh\ncat > \"$BIGGZ_TEST_SHIM_DUMP\"\ncat \"$BIGGZ_TEST_SHIM_GOLDEN\"\nexit %d\n", exitCode)
	if runtime.GOOS == "windows" {
		name = "pi.cmd"
		script = fmt.Sprintf("@echo off\r\npowershell -NoProfile -ExecutionPolicy Bypass -Command \"$s=[Console]::OpenStandardInput();$f=[IO.File]::Create($env:BIGGZ_TEST_SHIM_DUMP);$s.CopyTo($f);$f.Close();$o=[Console]::OpenStandardOutput();$b=[IO.File]::ReadAllBytes($env:BIGGZ_TEST_SHIM_GOLDEN);$o.Write($b,0,$b.Length);$o.Flush()\"\r\nexit /b %d\r\n", exitCode)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatalf("write pi shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dump
}

// reviewExecuteMarkerShim installs a `pi` shim that records its launch in the
// returned marker path; a refusal that preserves the preconditions must leave
// it absent.
func reviewExecuteMarkerShim(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "launched.marker")
	t.Setenv("BIGGZ_TEST_SHIM_MARKER", marker)
	name, script := "pi", "#!/bin/sh\necho launched > \"$BIGGZ_TEST_SHIM_MARKER\"\nexit 0\n"
	if runtime.GOOS == "windows" {
		name = "pi.cmd"
		script = "@echo off\r\necho launched > \"%BIGGZ_TEST_SHIM_MARKER%\"\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatalf("write marker shim: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

// TestReviewExecuteUsageMatrix: --execute is accepted only with --agent pi and
// is exclusive with --input/--preflight/--materialize; --timeout requires
// --execute with an integer in 1..7200. Every rejection is a pure usage error
// raised before the handshake and RDD gates, so it never depends on the
// environment.
func TestReviewExecuteUsageMatrix(t *testing.T) {
	base := []string{"biggz", "review", "capture-result",
		"--lineage", "lineage-x", "--target", strings.Repeat("a", 40),
		"--lens", "risk", "--order", "0", "--expected-revision", strings.Repeat("b", 64)}
	tests := []struct {
		name      string
		args      []string
		want      string // rejected usage wording; empty = the value must pass the gate
		handshake bool
	}{
		{"execute without the pi agent", []string{"--execute"}, "requires --agent pi", false},
		{"execute with another agent", []string{"--execute", "--agent", "claude-code"}, "requires --agent pi", false},
		{"execute with input", []string{"--execute", "--agent", "pi", "--input", "-"}, "mutually exclusive", false},
		{"execute with preflight", []string{"--execute", "--agent", "pi", "--preflight"}, "mutually exclusive", false},
		{"execute with materialize", []string{"--execute", "--agent", "pi", "--materialize"}, "mutually exclusive", false},
		{"timeout without execute", []string{"--timeout", "60"}, "requires --execute", false},
		{"timeout below the floor", []string{"--execute", "--agent", "pi", "--timeout", "0"}, "between 1 and 7200", false},
		{"timeout not an integer", []string{"--execute", "--agent", "pi", "--timeout", "soon"}, "between 1 and 7200", false},
		{"timeout above the ceiling", []string{"--execute", "--agent", "pi", "--timeout", "7201"}, "between 1 and 7200", false},
		{"timeout at the ceiling passes the gate", []string{"--execute", "--agent", "pi", "--timeout", "7200"}, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reviewExecuteEnv(t, tc.handshake)
			chdir(t, t.TempDir())
			code, _, stderr := runReviewCapture(t, append(append([]string{}, base...), tc.args...), "")
			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr)
			}
			if tc.want == "" {
				for _, forbidden := range []string{"between 1 and 7200", "--timeout", "unknown flag"} {
					if strings.Contains(stderr, forbidden) {
						t.Fatalf("7200 must pass the --timeout gate, got: %s", stderr)
					}
				}
				return
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr should name %q, got: %s", tc.want, stderr)
			}
			if strings.Contains(stderr, review.PiReviewRelayContractEnv) {
				t.Fatalf("usage errors must precede the handshake gate: %s", stderr)
			}
		})
	}
}

// TestReviewExecuteShimPipeline: a `pi` shim on PATH replays a strict-JSON
// golden keyed by the --preflight-derived subject_hash. start → execute →
// finalize → gate advances the lineage, the composed prompt reaches the
// reviewer byte-identically, and the captured bytes equal the shim's stdout.
func TestReviewExecuteShimPipeline(t *testing.T) {
	origBurn := review.BurnEnabled
	review.BurnEnabled = false
	defer func() { review.BurnEnabled = origBurn }()
	reviewExecuteEnv(t, true)
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	lineageID, targetSHA, headHash := materializeStartedLineage(t, repoDir)
	binding := review.CaptureBinding{Repo: repoDir, LineageID: lineageID, TargetIdentity: targetSHA,
		Lens: "risk", Order: 0, ExpectedRevision: headHash}
	taskBytes, err := review.MaterializeReviewerTask(binding)
	if err != nil {
		t.Fatalf("MaterializeReviewerTask: %v", err)
	}

	code, stdout, stderr := runReviewCapture(t, []string{"biggz", "review", "capture-result",
		"--lineage", lineageID, "--target", targetSHA, "--lens", "risk", "--order", "0",
		"--expected-revision", headHash, "--preflight"}, "")
	if code != 0 {
		t.Fatalf("preflight: exit code = %d (stderr: %s)", code, stderr)
	}
	var preflight review.PreflightResult
	if err := json.Unmarshal([]byte(stdout), &preflight); err != nil {
		t.Fatalf("preflight JSON: %v\n%s", err, stdout)
	}
	goldenPayload, err := json.Marshal(map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": review.ManifestPaths(preflight.ChangedPathManifest)},
		"lens":         "risk",
		"findings":     []any{},
		"evidence":     []any{"candidate inspection completed"},
	})
	if err != nil {
		t.Fatalf("marshal golden: %v", err)
	}
	dumpPath := reviewExecuteShim(t, goldenPayload, 0)

	code, stdout, stderr = runReviewCapture(t, []string{"biggz", "review", "capture-result",
		"--lineage", lineageID, "--target", targetSHA, "--lens", "risk", "--order", "0",
		"--expected-revision", headHash, "--agent", "pi", "--execute", "--timeout", "120"}, "")
	if code != 0 {
		t.Fatalf("execute: exit code = %d (stderr: %s)", code, stderr)
	}
	var artifact review.CapturedArtifact
	if err := json.Unmarshal([]byte(stdout), &artifact); err != nil {
		t.Fatalf("capture artifact JSON: %v\n%s", err, stdout)
	}
	if artifact.AdmissionDecision != review.AdmissionCompleted {
		t.Fatalf("admission decision = %q, want completed", artifact.AdmissionDecision)
	}
	if artifact.Revision == "" || artifact.Revision == headHash {
		t.Fatalf("execute must append a new revision, got %q", artifact.Revision)
	}

	// Raw stdout reached Capture unmodified: replaying the shim's exact bytes
	// against the now-occupied slot is idempotent only when the canonical
	// bytes match, so any transport mutation would conflict.
	replay, err := review.Capture(binding, goldenPayload)
	if err != nil {
		t.Fatalf("replay Capture: %v", err)
	}
	if !replay.Idempotent || replay.Artifact.CanonicalSHA256 != artifact.CanonicalSHA256 {
		t.Fatal("the captured bytes must equal the shim's stdout byte-for-byte")
	}

	// The composed prompt reached the reviewer byte-identically: the shim's
	// stdin dump closes with the materialized task segment.
	dumped, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatalf("read shim stdin dump: %v", err)
	}
	if !bytes.HasSuffix(dumped, taskBytes) {
		t.Fatalf("the shim stdin must close with the byte-identical materialized task (%d dump bytes, %d task bytes)", len(dumped), len(taskBytes))
	}

	// The lineage advanced as --input would, and the review still finalizes
	// and passes its publication gate.
	status := statusJSON(t, lineageID)
	var newHead string
	if err := json.Unmarshal(status["head_hash"], &newHead); err != nil {
		t.Fatalf("status head_hash: %v", err)
	}
	if newHead != artifact.Revision {
		t.Fatalf("status head = %s, want the capture revision %s", newHead, artifact.Revision)
	}
	code, stdout, stderr = runReviewFinalize(t, []string{"biggz", "review", "finalize", lineageID})
	if code != 0 || !strings.Contains(stdout, "Review finalized") {
		t.Fatalf("finalize: exit code = %d (stdout: %s, stderr: %s)", code, stdout, stderr)
	}
	code, _, stderr = runReviewGate(t, []string{"biggz", "review", "gate", "post-apply", lineageID, "--json"})
	if code != 0 {
		t.Fatalf("gate: exit code = %d (stderr: %s)", code, stderr)
	}
}

// TestReviewExecuteRefusals: transport failures and admission rejections are
// typed refusals naming the stage, exit 1, with no appended event and no
// partial slot; a missing handshake or disabled RDD refuses before
// materialization with nothing launched.
func TestReviewExecuteRefusals(t *testing.T) {
	base := func(lineageID, targetSHA, headHash string) []string {
		return []string{"biggz", "review", "capture-result",
			"--lineage", lineageID, "--target", targetSHA, "--lens", "risk", "--order", "0",
			"--expected-revision", headHash, "--agent", "pi", "--execute"}
	}

	t.Run("transport and admission refusals capture nothing", func(t *testing.T) {
		reviewExecuteEnv(t, true)
		repoDir := gitRepoWithAuthCommit(t)
		chdir(t, repoDir)
		lineageID, targetSHA, headHash := materializeStartedLineage(t, repoDir)
		before := materializeCLISnapshot(t, lineageStoreDir(repoDir))
		rejected, err := json.Marshal(map[string]any{
			"subject_hash": strings.Repeat("0", 64),
			"inspection":   map[string]any{"status": "completed", "paths": []string{}},
			"lens":         "risk", "findings": []any{}, "evidence": []any{"candidate inspection completed"},
		})
		if err != nil {
			t.Fatalf("marshal rejected payload: %v", err)
		}
		cases := []struct {
			name     string
			payload  []byte
			exitCode int
			want     []string
		}{
			{"nonzero exit is typed", []byte(`{"unused":true}`), 7, []string{"kind=nonzero-exit", "stage=pi", "exit_code=7", "no capture was performed"}},
			{"empty stdout is typed", nil, 0, []string{"kind=empty-output", "stage=pi", "no capture was performed"}},
			{"over-cap output is typed", bytes.Repeat([]byte("x"), review.ArtifactResultLimit+1), 0, []string{"kind=output-over-cap", "stage=output", "no capture was performed"}},
			{"admission rejection refuses like --input", rejected, 0, []string{"admission", "binding_mismatch"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				reviewExecuteShim(t, tc.payload, tc.exitCode)
				code, _, stderr := runReviewCapture(t, base(lineageID, targetSHA, headHash), "")
				if code != 1 {
					t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr)
				}
				for _, want := range tc.want {
					if !strings.Contains(stderr, want) {
						t.Fatalf("stderr must name %q, got: %s", want, stderr)
					}
				}
				if after := materializeCLISnapshot(t, lineageStoreDir(repoDir)); !maps.Equal(before, after) {
					t.Fatalf("refusal mutated the lineage store:\nbefore=%v\nafter=%v", before, after)
				}
			})
		}
	})

	t.Run("missing handshake refuses before materialization", func(t *testing.T) {
		reviewExecuteEnv(t, false)
		repoDir := gitRepoWithAuthCommit(t)
		chdir(t, repoDir)
		lineageID, targetSHA, headHash := materializeStartedLineage(t, repoDir)
		before := materializeCLISnapshot(t, lineageStoreDir(repoDir))
		marker := reviewExecuteMarkerShim(t)
		code, _, stderr := runReviewCapture(t, base(lineageID, targetSHA, headHash), "")
		if code != 1 || !strings.Contains(stderr, review.PiReviewRelayContractEnv) {
			t.Fatalf("missing handshake: exit code = %d, stderr = %s", code, stderr)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("missing handshake must launch nothing")
		}
		if after := materializeCLISnapshot(t, lineageStoreDir(repoDir)); !maps.Equal(before, after) {
			t.Fatal("missing handshake must not mutate the lineage store")
		}
	})

	t.Run("disabled RDD refuses before materialization", func(t *testing.T) {
		reviewExecuteEnv(t, true)
		repoDir := gitRepoWithAuthCommit(t)
		chdir(t, repoDir)
		lineageID, targetSHA, headHash := materializeStartedLineage(t, repoDir)
		before := materializeCLISnapshot(t, lineageStoreDir(repoDir))
		marker := reviewExecuteMarkerShim(t)
		if _, err := review.RDDDisable("", "", "global"); err != nil {
			t.Fatalf("RDDDisable: %v", err)
		}
		code, _, stderr := runReviewCapture(t, base(lineageID, targetSHA, headHash), "")
		if code != 1 || !strings.Contains(stderr, "blocked by RDD") {
			t.Fatalf("disabled RDD: exit code = %d, stderr = %s", code, stderr)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("disabled RDD must launch nothing")
		}
		if after := materializeCLISnapshot(t, lineageStoreDir(repoDir)); !maps.Equal(before, after) {
			t.Fatal("disabled RDD must not mutate the lineage store")
		}
	})
}
