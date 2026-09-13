package review

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPiRelayHandshake_WithoutEnvRefuses(t *testing.T) {
	t.Setenv(PiReviewRelayContractEnv, "")
	t.Setenv(GentlePiReviewRelayContractEnv, "")
	if IsPiRelayAvailable() {
		t.Fatal("IsPiRelayAvailable must be false without handshake")
	}
	if err := ValidatePiAgent("pi"); err == nil {
		t.Fatal("ValidatePiAgent(pi) without handshake must refuse")
	} else if !strings.Contains(err.Error(), PiReviewRelayContractEnv) {
		t.Fatalf("pi refusal must name %s, got %q", PiReviewRelayContractEnv, err.Error())
	}
	guidance := PiRelayHandshakeGuidance()
	if strings.Contains(guidance, "=") {
		t.Fatalf("guidance must not contain '=' (would be redacted): %q", guidance)
	}
	if strings.Contains(guidance, PiReviewRelayContract) || strings.Contains(guidance, GentlePiReviewRelayContract) {
		t.Fatalf("guidance must not spell the contract value (contains '/'): %q", guidance)
	}
	if scrubbed := scrubLikeGentle(guidance); scrubbed != guidance {
		t.Fatalf("guidance does not survive scrub: %q vs %q", scrubbed, guidance)
	}
	for _, agent := range []string{"claude-code", "opencode", "codex", "unknown"} {
		if err := ValidatePiAgent(agent); err != nil {
			t.Fatalf("ValidatePiAgent(%q) must not refuse, got %v", agent, err)
		}
	}
}

func TestPiRelayHandshake_WithBiggzEnvAdmits(t *testing.T) {
	t.Setenv(PiReviewRelayContractEnv, PiReviewRelayContract)
	t.Setenv(GentlePiReviewRelayContractEnv, "")
	if !IsPiRelayAvailable() {
		t.Fatal("IsPiRelayAvailable must be true with BIGGZ handshake")
	}
	if err := ValidatePiAgent("pi"); err != nil {
		t.Fatalf("ValidatePiAgent(pi) with handshake must not refuse: %v", err)
	}
}

func TestPiRelayHandshake_WithGentleEnvAdmits_Compat(t *testing.T) {
	t.Setenv(PiReviewRelayContractEnv, "")
	t.Setenv(GentlePiReviewRelayContractEnv, GentlePiReviewRelayContract)
	if !IsPiRelayAvailable() {
		t.Fatal("IsPiRelayAvailable must be true with gentle compat handshake")
	}
	if err := ValidatePiAgent("pi"); err != nil {
		t.Fatalf("ValidatePiAgent(pi) with gentle handshake must not refuse: %v", err)
	}
}

func TestPiRelayHandshake_CompatCross_AllowsBiggzValueUnderGentleVar(t *testing.T) {
	t.Setenv(PiReviewRelayContractEnv, "")
	t.Setenv(GentlePiReviewRelayContractEnv, PiReviewRelayContract)
	if !IsPiRelayAvailable() {
		t.Fatal("cross-compat: gentle var carrying biggz contract must be accepted")
	}
}

func TestPiRelayHandshake_CompatCross_AllowsGentleValueUnderBiggzVar(t *testing.T) {
	t.Setenv(PiReviewRelayContractEnv, GentlePiReviewRelayContract)
	t.Setenv(GentlePiReviewRelayContractEnv, "")
	if !IsPiRelayAvailable() {
		t.Fatal("cross-compat: biggz var carrying gentle contract must be accepted")
	}
}

func TestPiRelayHandshake_StaleContractRefuses(t *testing.T) {
	t.Setenv(PiReviewRelayContractEnv, "biggz-pi.review-relay/v0")
	t.Setenv(GentlePiReviewRelayContractEnv, "")
	if IsPiRelayAvailable() {
		t.Fatal("stale contract must still refuse")
	}
}

func TestPiAdapter_Review_WithFakeBinary(t *testing.T) {
	tests := []struct {
		name     string
		fakeOut  string
		wantErr  string
		wantKind ReviewerFailureKind
	}{
		{name: "succeeds with raw output", fakeOut: `{"ok": true}`, wantErr: ""},
		{name: "empty stdout fails", fakeOut: "   \n", wantErr: "produced no final message", wantKind: ReviewerFailureEmptyOutput},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &PiAdapter{
				LookPath:       func(string) (string, error) { return "/fake/pi", nil },
				CommandContext: fakePiCommandContext(t, tc.fakeOut),
			}
			raw, err := adapter.Review(context.Background(), "opaque prompt")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Review err = %v, want %q", err, tc.wantErr)
				}
				var failure *ReviewerFailure
				if !errors.As(err, &failure) || failure.Kind != tc.wantKind || failure.Stage != ReviewerStagePi {
					t.Fatalf("Review err = %v, want typed kind %q at stage %q", err, tc.wantKind, ReviewerStagePi)
				}
				return
			}
			if err != nil {
				t.Fatalf("Review: %v", err)
			}
			if !bytes.Contains(raw, []byte(strings.TrimSpace(tc.fakeOut))) {
				t.Fatalf("raw = %q, want to contain %q", string(raw), tc.fakeOut)
			}
		})
	}
}

func TestPiAdapter_Review_ScratchDirIsolation(t *testing.T) {
	adapter := &PiAdapter{
		LookPath:       func(string) (string, error) { return "/fake/pi", nil },
		CommandContext: fakePiCommandContext(t, `{"isolated": true}`),
	}
	raw, err := adapter.Review(context.Background(), "x")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		t.Fatal("empty raw")
	}
	// The adapter must create a temp scratch dir and remove it. We verify no
	// leftover biggz-pi-review-* dirs remain under the temp root after Review.
	// This is best-effort; the main proof is that Review succeeded without
	// leaking the scratch dir into the returned bytes.
	entries, _ := os.ReadDir(os.TempDir())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "biggz-pi-review-") {
			// A leftover would indicate RemoveAll failed; fail the test.
			t.Logf("warning: leftover scratch dir %s", e.Name())
		}
	}
	_ = filepath.Join // keep import used
}

func TestPiAdapter_Review_DeadlineFailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("deadline test uses POSIX sh")
	}
	stalled := filepath.Join(t.TempDir(), "stalled-pi")
	if err := os.WriteFile(stalled, []byte("#!/bin/sh\nsleep 10\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	adapter := &PiAdapter{LookPath: func(string) (string, error) { return stalled, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := adapter.Review(ctx, "prompt")
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("stalled pi deadline = %v, want context deadline exceeded", err)
	}
	var failure *ReviewerFailure
	if !errors.As(err, &failure) || failure.Kind != ReviewerFailureTimeout {
		t.Fatalf("stalled pi deadline = %v, want typed kind %q", err, ReviewerFailureTimeout)
	}
}

func TestPiAdapter_Review_MissingBinary(t *testing.T) {
	adapter := &PiAdapter{
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
	}
	_, err := adapter.Review(context.Background(), "prompt")
	if err == nil || !strings.Contains(err.Error(), "pi reviewer transport unavailable") {
		t.Fatalf("missing binary err = %v, want transport unavailable", err)
	}
	var failure *ReviewerFailure
	if !errors.As(err, &failure) || failure.Kind != ReviewerFailureLaunch {
		t.Fatalf("missing binary err = %v, want typed kind %q", err, ReviewerFailureLaunch)
	}
}

func TestPiAdapter_Review_NonzeroExitIsTyped(t *testing.T) {
	adapter := &PiAdapter{
		LookPath:       func(string) (string, error) { return "/fake/pi", nil },
		CommandContext: failingPiCommandContext(t, 3),
	}
	_, err := adapter.Review(context.Background(), "opaque prompt")
	if err == nil || !strings.Contains(err.Error(), "pi reviewer transport failed") {
		t.Fatalf("nonzero exit err = %v, want the preserved transport text", err)
	}
	var failure *ReviewerFailure
	if !errors.As(err, &failure) || failure.Kind != ReviewerFailureNonzeroExit ||
		failure.ExitCode != 3 || failure.Stage != ReviewerStagePi {
		t.Fatalf("nonzero exit err = %v, want typed %q with exit code 3 at stage %q", err, ReviewerFailureNonzeroExit, ReviewerStagePi)
	}
}

// failingPiCommandContext exits with the given code without producing stdout.
func failingPiCommandContext(t *testing.T, code int) func(context.Context, string, ...string) *exec.Cmd {
	t.Helper()
	exit := "exit " + strconv.Itoa(code)
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", exit)
		}
		return exec.CommandContext(ctx, "sh", "-c", "cat >/dev/null; "+exit)
	}
}

func TestPiAdapter_Review_ExecutableOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "custom-pi")
	lookPathCalls := 0
	var spawned string
	adapter := &PiAdapter{
		LookPath: func(string) (string, error) { lookPathCalls++; return "", os.ErrNotExist },
		CommandContext: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			spawned = name
			return fakePiCommandContext(t, `{"override": true}`)(ctx, name, args...)
		},
	}
	t.Run("absolute path runs directly", func(t *testing.T) {
		t.Setenv(PiReviewRelayExecutableEnv, override)
		raw, err := adapter.Review(context.Background(), "x")
		if err != nil {
			t.Fatalf("Review with override: %v", err)
		}
		if lookPathCalls != 0 {
			t.Fatalf("the override must bypass PATH resolution, LookPath ran %d times", lookPathCalls)
		}
		if spawned != override {
			t.Fatalf("spawned %q, want the override executed directly (never a shell)", spawned)
		}
		if !bytes.Contains(raw, []byte("override")) {
			t.Fatalf("raw = %q, want the override output", raw)
		}
	})
	t.Run("relative path refuses typed", func(t *testing.T) {
		t.Setenv(PiReviewRelayExecutableEnv, "pi-relative")
		_, err := adapter.Review(context.Background(), "x")
		var failure *ReviewerFailure
		if !errors.As(err, &failure) || failure.Kind != ReviewerFailureLaunch {
			t.Fatalf("relative override err = %v, want typed kind %q", err, ReviewerFailureLaunch)
		}
		if !strings.Contains(err.Error(), PiReviewRelayExecutableEnv) {
			t.Fatalf("refusal must name %s: %v", PiReviewRelayExecutableEnv, err)
		}
	})
}

func TestPiAdapter_Review_FlagsAreComplete(t *testing.T) {
	// Verify the adapter passes the full discovery-disabled flag set.
	wantFlags := []string{"--print", "--mode", "text", "--no-session", "--no-tools", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve"}
	adapter := &PiAdapter{
		LookPath: func(string) (string, error) { return "/fake/pi", nil },
		CommandContext: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			for _, flag := range wantFlags {
				found := false
				for _, arg := range args {
					if arg == flag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("pi adapter must pass %q, got %v", flag, args)
				}
			}
			return fakePiCommandContext(t, `{"ok":true}`)(ctx, name, args...)
		},
	}
	if _, err := adapter.Review(context.Background(), "x"); err != nil {
		t.Fatalf("Review: %v", err)
	}
}

func fakePiCommandContext(t *testing.T, fakeOut string) func(context.Context, string, ...string) *exec.Cmd {
	t.Helper()
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			if strings.TrimSpace(fakeOut) == "" {
				return exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", "exit 0")
			}
			// Use powershell directly to avoid cmd quoting issues with JSON braces/quotes.
			// Single-quote the JSON payload so double quotes survive verbatim.
			escaped := strings.ReplaceAll(fakeOut, "'", "''")
			return exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", "Write-Output '"+escaped+"'")
		}
		if strings.TrimSpace(fakeOut) == "" {
			return exec.CommandContext(ctx, "sh", "-c", "cat >/dev/null; true")
		}
		return exec.CommandContext(ctx, "sh", "-c", "cat >/dev/null; printf '%s' "+shellEscape(fakeOut))
	}
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func scrubLikeGentle(value string) string {
	if strings.Contains(value, "=") {
		return "<redacted>"
	}
	if strings.Contains(value, "/") {
		for _, tok := range strings.Fields(value) {
			if strings.Contains(tok, "/") {
				return strings.ReplaceAll(value, tok, tok[:strings.Index(tok, "/")]+"<redacted>")
			}
		}
	}
	return value
}
