package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

func captureBigmemRun(args []string) (int, string, string) {
	savedArgs := os.Args
	savedStderr := os.Stderr
	savedStdout := os.Stdout
	savedHome := os.Getenv("HOME")
	savedUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		os.Args = savedArgs
		os.Stderr = savedStderr
		os.Stdout = savedStdout
		_ = os.Setenv("HOME", savedHome)
		_ = os.Setenv("USERPROFILE", savedUserProfile)
	}()
	os.Args = append([]string{"biggz", "bigmem"}, args...)

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	code := bigmemRun()

	wOut.Close()
	wErr.Close()
	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)
	return code, outBuf.String(), errBuf.String()
}

func TestBigmemSync_HelpContainsFlags(t *testing.T) {
	_, _, stderr := captureBigmemRun([]string{"sync", "--help"})
	for _, flag := range []string{"--from-engram", "--engram-dir", "--project"} {
		if !strings.Contains(stderr, flag) {
			t.Errorf("help should contain %q, got %q", flag, stderr)
		}
	}
}

// TestBigmemSave_SessionIDRoutesToUpsert proves the CLI half of REQ-SC1:
// `save --type session_summary --session-id S` uses the deterministic id and
// repeated closes update one row in place.
func TestBigmemSave_SessionIDRoutesToUpsert(t *testing.T) {
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)
	args := []string{"save", "Session summary", "first close", "--type", "session_summary", "--scope", "project", "--project", "biggz-ai", "--session-id", "sess-cli-1"}
	code, stdout, stderr := captureBigmemRun(args)
	if code != 0 {
		t.Fatalf("save exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Saved: session-summary-sess-cli-1") {
		t.Fatalf("save must print deterministic id, got %q", stdout)
	}
	args[2] = "second close"
	code, stdout2, stderr2 := captureBigmemRun(args)
	if code != 0 {
		t.Fatalf("repeat save exit %d stderr=%q", code, stderr2)
	}
	if !strings.Contains(stdout2, "Saved: session-summary-sess-cli-1") {
		t.Fatalf("repeat save must reuse deterministic id, got %q", stdout2)
	}
	store, err := bigmem.Open("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	results, err := store.Search("", bigmem.SearchOptions{Type: "session_summary", Limit: 5, Project: "biggz-ai"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want exactly 1 session_summary after repeat CLI close, got %d", len(results))
	}
	if results[0].Content != "second close" {
		t.Fatalf("repeat close must update in place, content=%q", results[0].Content)
	}
}
func TestBigmemSync_HelpListsFromEngram(t *testing.T) {
	_, _, stderr := captureBigmemRun([]string{"sync", "--help"})
	if !strings.Contains(stderr, "--from-engram") {
		t.Fatalf("help missing --from-engram: %q", stderr)
	}
	if !strings.Contains(stderr, "--engram-dir") {
		t.Fatalf("help missing --engram-dir: %q", stderr)
	}
}

func TestBigmemSyncImport_MissingManifestExit1(t *testing.T) {
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)
	emptyEngram := dir + "/empty-engram"
	_ = os.MkdirAll(emptyEngram, 0755)
	code, _, stderr := captureBigmemRun([]string{"sync", "--import", "--from-engram", "--engram-dir", emptyEngram})
	if code != 1 {
		t.Fatalf("expected exit 1 for missing manifest, got %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "manifest.json") {
		t.Fatalf("stderr should mention manifest.json, got %q", stderr)
	}
}

func TestBigmemSyncImport_FromEngramFlagParsing(t *testing.T) {
	// Verify that --from-engram --engram-dir and --project are accepted without error when dir exists but empty manifest vs no panic
	// Use a temp .engram with empty manifest (0 chunks) => should succeed with 0 imports
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)
	engramDir := dir + "/.engram"
	_ = os.MkdirAll(engramDir+"/chunks", 0755)
	_ = os.WriteFile(engramDir+"/manifest.json", []byte(`{"version":1,"chunks":[]}`), 0644)
	// Also need to handle --project flag passthrough; just verify exit 0
	code, _, stderr := captureBigmemRun([]string{"sync", "--import", "--from-engram", "--engram-dir", engramDir, "--project", "biggz-ai"})
	if code != 0 {
		t.Fatalf("expected exit 0 for empty manifest with --from-engram, got %d stderr=%q", code, stderr)
	}
}

func TestBigmemSyncImport_EngramDirEqualsForm(t *testing.T) {
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)
	engramDir := dir + "/.engram"
	_ = os.MkdirAll(engramDir+"/chunks", 0755)
	_ = os.WriteFile(engramDir+"/manifest.json", []byte(`{"version":1,"chunks":[]}`), 0644)
	code, _, stderr := captureBigmemRun([]string{"sync", "--import", "--from-engram", "--engram-dir=" + engramDir})
	if code != 0 {
		t.Fatalf("expected exit 0 for --engram-dir= form, got %d stderr=%q", code, stderr)
	}
}

// TestBigmemSearch_ZeroHintAndHyphenHit proves the CLI half of REQ-FTS1: the
// all-mode zero retry hint, the silent any-mode zero, and hyphenated hits.
func TestBigmemSearch_ZeroHintAndHyphenHit(t *testing.T) {
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)
	for _, seed := range [][]string{
		{"save", "Hyphen note", "marcador gentle-pi unico", "--type", "note", "--scope", "project", "--project", "probe"},
		{"save", "Ruido note", "ruido blanco suave", "--type", "note", "--scope", "project", "--project", "probe"},
	} {
		if code, _, stderr := captureBigmemRun(seed); code != 0 {
			t.Fatalf("seed save exit %d stderr=%q", code, stderr)
		}
	}

	t.Run("all-mode zero prints retry hint", func(t *testing.T) {
		code, stdout, _ := captureBigmemRun([]string{"search", "gentle-pi qqq-inexistente", "--project", "probe"})
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		if !strings.Contains(stdout, "No results.") {
			t.Fatalf("stdout must keep the zero signal, got %q", stdout)
		}
		if !strings.Contains(stdout, "--match-mode any") {
			t.Errorf("stdout must mirror the retry hint, got %q", stdout)
		}
	})

	t.Run("any-mode zero has no retry hint", func(t *testing.T) {
		code, stdout, _ := captureBigmemRun([]string{"search", "qqq-inexistente zzz-inexistente", "--match-mode", "any", "--project", "probe"})
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		if !strings.Contains(stdout, "No results.") {
			t.Fatalf("stdout must keep the zero signal, got %q", stdout)
		}
		if strings.Contains(stdout, "--match-mode any to broaden") {
			t.Errorf("any-mode zero must not hint, got %q", stdout)
		}
	})

	t.Run("any-mode hyphenated hits", func(t *testing.T) {
		code, stdout, _ := captureBigmemRun([]string{"search", "gentle-pi ruido", "--match-mode", "any", "--project", "probe"})
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
		for _, want := range []string{"Hyphen note", "Ruido note"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("any-mode hyphenated search must return %q, got %q", want, stdout)
			}
		}
	})
}

// waitNextSecond blocks until the wall clock crosses the next second
// boundary. Session start_time is stored with RFC3339 (1s) precision, so a
// session created after this point sorts strictly newer under
// ORDER BY start_time DESC.
func waitNextSecond() {
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(1100 * time.Millisecond)))
}

// TestBigmemContext_FullNewestSummary proves REQ-FR1 on the CLI surface: the
// newest session_summary returns full in one `context` call, while older
// summaries keep the 120-char preview. The seeding uses the session-guard
// bash fallback shape (`save --type session_summary --session-id`), which
// writes the deterministic observation that `context` must resolve.
func TestBigmemContext_FullNewestSummary(t *testing.T) {
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)

	older := strings.Repeat("O", 150) + "-CLI-OLDER-TAIL-c1a7"
	newer := strings.Repeat("N", 150) + "-CLI-NEWER-TAIL-e5b9"

	if code, _, stderr := captureBigmemRun([]string{"save", "Session summary", older, "--type", "session_summary", "--scope", "project", "--project", "ctx-cli-probe", "--session-id", "sess-cli-old"}); code != 0 {
		t.Fatalf("older save exit %d stderr=%q", code, stderr)
	}
	waitNextSecond()
	if code, _, stderr := captureBigmemRun([]string{"save", "Session summary", newer, "--type", "session_summary", "--scope", "project", "--project", "ctx-cli-probe", "--session-id", "sess-cli-new"}); code != 0 {
		t.Fatalf("newer save exit %d stderr=%q", code, stderr)
	}

	code, stdout, stderr := captureBigmemRun([]string{"context", "ctx-cli-probe"})
	if code != 0 {
		t.Fatalf("context exit %d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, newer) {
		t.Errorf("context must print the newest summary (%d chars) untruncated in one call, got %q", len(newer), stdout)
	}
	if strings.Contains(stdout, "-CLI-OLDER-TAIL-c1a7") {
		t.Errorf("older summary must stay previewed at 120 chars, got %q", stdout)
	}
	if !strings.Contains(stdout, older[:120]) {
		t.Errorf("older summary head must remain as its 120-char preview, got %q", stdout)
	}
}

// TestBigmemGet_FullSummaryAndUnknownID proves REQ-FR1's by-id clause and the
// unknown-id scenario: get returns the full content in one call, and an
// unknown id exits non-zero with an explicit not-found error (no silent empty).
func TestBigmemGet_FullSummaryAndUnknownID(t *testing.T) {
	dir := t.TempDir()
	_ = os.Setenv("HOME", dir)
	_ = os.Setenv("USERPROFILE", dir)
	long := strings.Repeat("F", 200) + "-GET-TAIL-3c0d"
	if code, _, stderr := captureBigmemRun([]string{"save", "Session summary", long, "--type", "session_summary", "--scope", "project", "--project", "ctx-cli-probe", "--session-id", "sess-cli-get"}); code != 0 {
		t.Fatalf("save exit %d stderr=%q", code, stderr)
	}

	t.Run("full content by id", func(t *testing.T) {
		code, stdout, stderr := captureBigmemRun([]string{"get", "session-summary-sess-cli-get"})
		if code != 0 {
			t.Fatalf("get exit %d stderr=%q", code, stderr)
		}
		if !strings.Contains(stdout, long) {
			t.Errorf("get must print the full content (%d chars) in one call, got %q", len(long), stdout)
		}
	})

	t.Run("unknown id exits non-zero", func(t *testing.T) {
		code, stdout, stderr := captureBigmemRun([]string{"get", "obs-does-not-exist-0000"})
		if code == 0 {
			t.Fatalf("unknown id must exit non-zero, got 0 (stdout=%q)", stdout)
		}
		if !strings.Contains(stderr, "not found") {
			t.Errorf("stderr must surface an explicit not-found error, got %q", stderr)
		}
	})
}
