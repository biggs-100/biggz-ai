package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sdd"
)

// runSessionCloseCLIArgs invokes sessionCloseRun in-process with explicit
// flags, capturing stdout and stderr through temp files.
func runSessionCloseCLIArgs(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs := os.Args
	os.Args = append([]string{"biggz", "session-close"}, args...)
	defer func() { os.Args = oldArgs }()

	outFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("create stdout capture: %v", err)
	}
	errFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("create stderr capture: %v", err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	code = sessionCloseRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

// stubSessionCloseSeams replaces verify/save delegates for a test.
func stubSessionCloseSeams(
	t *testing.T,
	verify func(ctx context.Context, workspaceRoot, proj string) (bool, error),
	save func(ctx context.Context, workspaceRoot, change, proj, sessionID, content string, hasMCP bool) (string, error),
) {
	t.Helper()
	oldVerify, oldSave := sessionCloseVerify, sessionCloseSave
	sessionCloseVerify, sessionCloseSave = verify, save
	t.Cleanup(func() { sessionCloseVerify, sessionCloseSave = oldVerify, oldSave })
}

// clearProjectEnv isolates project detection from the developer machine.
func clearProjectEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BIGMEM_PROJECT", "")
	t.Setenv("BIGGZ_PROJECT", "")
	t.Setenv("ENGRAM_PROJECT", "")
}

func TestSessionCloseConflictingModeFlagsFail(t *testing.T) {
	clearProjectEnv(t)
	code, _, stderr := runSessionCloseCLIArgs(t, "--check-only", "--save", "x")
	if code != 2 {
		t.Fatalf("conflicting flags: got exit %d, want 2", code)
	}
	if !strings.Contains(strings.ToLower(stderr), "usage") {
		t.Fatalf("conflicting flags: stderr must contain usage, got %q", stderr)
	}
}

func TestSessionCloseForeignProjectAllows(t *testing.T) {
	clearProjectEnv(t)
	dir := t.TempDir()
	// Temp dirs resolve to a foreign project (basename), never biggz-ai.
	code, _, _ := runSessionCloseCLIArgs(t, "--check-only", "--cwd", dir)
	if code != 0 {
		t.Fatalf("foreign project: got exit %d, want 0", code)
	}
	// No fallback file must be written for foreign projects.
	matches, _ := filepath.Glob(filepath.Join(dir, "openspec", "changes", "*", "session-fallback.md"))
	if len(matches) > 0 {
		t.Fatalf("foreign project: unexpected fallback files %v", matches)
	}
}

func TestSessionCloseJSONShape(t *testing.T) {
	clearProjectEnv(t)
	t.Setenv("BIGMEM_PROJECT", "biggz-ai")
	dir := t.TempDir()
	stubSessionCloseSeams(t,
		func(ctx context.Context, workspaceRoot, proj string) (bool, error) { return false, nil },
		nil,
	)
	code, stdout, _ := runSessionCloseCLIArgs(t, "--check-only", "--json", "--cwd", dir)
	if code != 1 {
		t.Fatalf("json shape: got exit %d, want 1", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json shape: stdout is not valid JSON: %v (%q)", err, stdout)
	}
	for _, key := range []string{"verified", "reason", "fallback"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("json shape: missing key %q in %v", key, payload)
		}
	}
	if _, ok := payload["verified"].(bool); !ok {
		t.Fatalf("json shape: verified must be bool in %v", payload)
	}
}

func TestSessionCloseFallbackNeverVerifies(t *testing.T) {
	clearProjectEnv(t)
	t.Setenv("BIGMEM_PROJECT", "biggz-ai")
	dir := t.TempDir()
	// Degraded evidence exists, but only a persisted session_summary clears
	// the gate — the CLI must use Verify semantics (not IsBlocked).
	fallback := sdd.FallbackFilePath(dir, "session-close")
	if err := os.MkdirAll(filepath.Dir(fallback), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallback, []byte("# Session Fallback\n"), 0644); err != nil {
		t.Fatal(err)
	}
	stubSessionCloseSeams(t,
		func(ctx context.Context, workspaceRoot, proj string) (bool, error) { return false, nil },
		nil,
	)
	if code, _, _ := runSessionCloseCLIArgs(t, "--check-only", "--cwd", dir); code != 1 {
		t.Fatalf("fallback alone: got exit %d, want 1 (still blocked)", code)
	}
}

func TestSessionCloseEmptySaveFails(t *testing.T) {
	clearProjectEnv(t)
	t.Setenv("BIGMEM_PROJECT", "biggz-ai")
	dir := t.TempDir()
	// Vacuous evidence must never reach the store: empty/whitespace --save
	// is a usage error even for the gated project.
	for _, text := range []string{"", "   "} {
		code, _, stderr := runSessionCloseCLIArgs(t, "--save", text, "--cwd", dir)
		if code != 2 {
			t.Fatalf("empty save %q: got exit %d, want 2", text, code)
		}
		if !strings.Contains(strings.ToLower(stderr), "usage") {
			t.Fatalf("empty save %q: stderr must contain usage, got %q", text, stderr)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "openspec", "changes", "*", "session-fallback.md"))
	if len(matches) > 0 {
		t.Fatalf("empty save: unexpected fallback files %v", matches)
	}
}

func TestSessionCloseSaveSuccessReportsVerified(t *testing.T) {
	clearProjectEnv(t)
	t.Setenv("BIGMEM_PROJECT", "biggz-ai")
	dir := t.TempDir()
	stubSessionCloseSeams(t, nil,
		func(ctx context.Context, workspaceRoot, change, proj, sessionID, content string, hasMCP bool) (string, error) {
			if strings.TrimSpace(content) == "" {
				t.Errorf("save seam received empty content")
			}
			return "id-123", nil
		},
	)
	code, stdout, _ := runSessionCloseCLIArgs(t, "--save", "real summary", "--json", "--cwd", dir)
	if code != 0 {
		t.Fatalf("save success: got exit %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("save success: stdout is not valid JSON: %v (%q)", err, stdout)
	}
	if payload["verified"] != true || payload["status"] != "verified" {
		t.Fatalf("save success: want verified+status=verified, got %v", payload)
	}
}

func TestSessionCloseSaveDegradedReportsFallback(t *testing.T) {
	clearProjectEnv(t)
	t.Setenv("BIGMEM_PROJECT", "biggz-ai")
	dir := t.TempDir()
	fallback := sdd.FallbackFilePath(dir, "session-close")
	stubSessionCloseSeams(t, nil,
		func(ctx context.Context, workspaceRoot, change, proj, sessionID, content string, hasMCP bool) (string, error) {
			return fallback, errors.New("store unavailable")
		},
	)
	code, stdout, _ := runSessionCloseCLIArgs(t, "--save", "real summary", "--json", "--cwd", dir)
	if code != 1 {
		t.Fatalf("save degraded: got exit %d, want 1", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("save degraded: stdout is not valid JSON: %v (%q)", err, stdout)
	}
	if payload["verified"] != false || payload["status"] != "degraded" || payload["fallback"] != fallback {
		t.Fatalf("save degraded: want verified=false degraded with fallback, got %v", payload)
	}
}

func TestSessionCloseTraversalChangeStaysAnchored(t *testing.T) {
	clearProjectEnv(t)
	dir := t.TempDir()
	anchored := sdd.FallbackFilePath(dir, "../x")
	rel, err := filepath.Rel(dir, anchored)
	if err != nil {
		t.Fatal(err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("traversal --change ../x escaped workspace: %q", anchored)
	}
}

func TestSessionCloseHelpDocumentsContract(t *testing.T) {
	clearProjectEnv(t)
	code, _, stderr := runSessionCloseCLIArgs(t, "--help")
	if code != 0 {
		t.Fatalf("help: got exit %d, want 0", code)
	}
	for _, want := range []string{"--cwd", "--json", "--check-only", "--save", "0", "1"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("help: missing %q in %q", want, stderr)
		}
	}
}
