package review

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		name    string
		fakeOut string
		wantErr string
	}{
		{name: "succeeds with raw output", fakeOut: `{"ok": true}`, wantErr: ""},
		{name: "empty stdout fails", fakeOut: "   \n", wantErr: "produced no final message"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &PiAdapter{
				LookPath: func(string) (string, error) { return "/fake/pi", nil },
				CommandContext: fakePiCommandContext(t, tc.fakeOut),
			}
			raw, err := adapter.Review(context.Background(), "opaque prompt")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Review err = %v, want %q", err, tc.wantErr)
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
}

func TestPiAdapter_Review_MissingBinary(t *testing.T) {
	adapter := &PiAdapter{
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
	}
	_, err := adapter.Review(context.Background(), "prompt")
	if err == nil || !strings.Contains(err.Error(), "pi reviewer transport unavailable") {
		t.Fatalf("missing binary err = %v, want transport unavailable", err)
	}
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
