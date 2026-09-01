package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

func captureRecall(args []string, fn func() int) (int, string, string) {
	savedArgs := os.Args
	savedStdout := os.Stdout
	savedStderr := os.Stderr
	savedHome := os.Getenv("HOME")
	savedUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		os.Args = savedArgs
		os.Stdout = savedStdout
		os.Stderr = savedStderr
		_ = os.Setenv("HOME", savedHome)
		_ = os.Setenv("USERPROFILE", savedUserProfile)
	}()

	// args already include binary name prefix? recallRun expects Os.Args[2:]
	// For direct runRecall, we bypass Os.Args; use helper that sets Os.Args for recallRun.
	// Instead we call fn which reads Os.Args internally if needed.
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	code := fn()

	wOut.Close()
	wErr.Close()
	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)
	// Keep args variable used to avoid unused import warning; fn closure captures args
	_ = args
	return code, outBuf.String(), errBuf.String()
}

func TestRecall_HelpContainsRecencyNote(t *testing.T) {
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	os.Args = []string{"biggz", "recall", "--help"}
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = wOut
	os.Stderr = wErr
	code := recallRun()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)
	combined := outBuf.String() + errBuf.String()
	if code != 0 {
		t.Errorf("recall --help should exit 0, got %d", code)
	}
	if !strings.Contains(combined, "ORDER BY updated_at DESC") {
		t.Errorf("help should contain 'ORDER BY updated_at DESC', got %q", combined)
	}
	if !strings.Contains(combined, "bigmem search --query \"\"") {
		t.Errorf("help should contain 'bigmem search --query \"\"', got %q", combined)
	}
	if !strings.Contains(combined, "never use FTS term search for 'latest'") {
		t.Errorf("help should contain guardrail literal, got %q", combined)
	}
}

func TestBigmemRecent_HelpContainsRecencyNote(t *testing.T) {
	code, _, stderr := captureBigmemRun([]string{"recent", "--help"})
	if code != 0 {
		t.Errorf("bigmem recent --help should exit 0, got %d", code)
	}
	if !strings.Contains(stderr, "ORDER BY updated_at DESC") {
		t.Errorf("recent help should contain recency note, got %q", stderr)
	}
	if !strings.Contains(stderr, "never use FTS term search for 'latest'") {
		t.Errorf("recent help should contain guardrail, got %q", stderr)
	}
}

func TestRecall_AndRecent_BothCallRecent(t *testing.T) {
	// Use isolated HOME for temp DB
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	// Seed via bigmem save? Use direct Store
	// We seed via bigmemRun save for integration coverage
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()

	// Save two observations with different types
	os.Args = []string{"biggz", "bigmem", "save", "Fresh recall", "content fresh", "--type", "session_summary", "--project", "biggz-ai"}
	if code := bigmemRun(); code != 0 {
		t.Fatalf("save fresh failed code %d", code)
	}
	// Ensure different timestamps by sleeping
	// Use direct sleep? The save will have close timestamps but order by updated_at DESC still deterministic by insertion
	// Also save another
	os.Args = []string{"biggz", "bigmem", "save", "Other", "content other", "--type", "decision", "--project", "biggz-ai"}
	if code := bigmemRun(); code != 0 {
		t.Fatalf("save other failed code %d", code)
	}

	// Test recall --json --limit 2 returns JSON array ordered by updated_at DESC (most recent first)
	var recallOut string
	{
		rOut, wOut, _ := os.Pipe()
		rErr, wErr, _ := os.Pipe()
		oldOut := os.Stdout
		oldErr := os.Stderr
		os.Stdout = wOut
		os.Stderr = wErr
		os.Args = []string{"biggz", "recall", "--json", "--limit", "2", "--project", "biggz-ai"}
		code := recallRun()
		wOut.Close()
		wErr.Close()
		os.Stdout = oldOut
		os.Stderr = oldErr
		var outBuf bytes.Buffer
		_, _ = outBuf.ReadFrom(rOut)
		_, _ = bytes.NewBuffer(nil).ReadFrom(rErr)
		if code != 0 {
			// readErr for debug
			t.Fatalf("recall --json failed code %d", code)
		}
		recallOut = outBuf.String()
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(recallOut), &arr); err != nil {
		t.Fatalf("recall json unmarshal failed: %v out=%q", err, recallOut)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 results, got %d out=%q", len(arr), recallOut)
	}
	if _, ok := arr[0]["updated_at"]; !ok {
		t.Errorf("json should contain updated_at, got %v", arr[0])
	}

	// Test alias biggz bigmem recent --json --limit 2 same flags
	code, out, _ := captureBigmemRun([]string{"recent", "--json", "--limit", "2", "--project", "biggz-ai"})
	if code != 0 {
		t.Fatalf("bigmem recent --json failed code %d", code)
	}
	var arr2 []map[string]any
	if err := json.Unmarshal([]byte(out), &arr2); err != nil {
		t.Fatalf("recent json unmarshal failed: %v out=%q", err, out)
	}
	if len(arr2) != 2 {
		t.Fatalf("recent expected 2 results, got %d", len(arr2))
	}
}

func TestRecall_FlagsForwarded(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	// Seed
	savedArgs := os.Args
	defer func() { os.Args = savedArgs }()
	os.Args = []string{"biggz", "bigmem", "save", "Session 1", "content", "--type", "session_summary", "--project", "biggz-ai"}
	if code := bigmemRun(); code != 0 {
		t.Fatalf("save session_summary failed %d", code)
	}
	os.Args = []string{"biggz", "bigmem", "save", "Decision 1", "content", "--type", "decision", "--project", "biggz-ai"}
	if code := bigmemRun(); code != 0 {
		t.Fatalf("save decision failed %d", code)
	}

	// recall --type session_summary should only return session_summary
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = wOut
	os.Stderr = wErr
	os.Args = []string{"biggz", "recall", "--type", "session_summary", "--json", "--project", "biggz-ai"}
	code := recallRun()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = bytes.NewBuffer(nil).ReadFrom(rErr)
	if code != 0 {
		t.Fatalf("recall --type failed %d", code)
	}
	var arr []map[string]any
	if err := json.Unmarshal(outBuf.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, o := range arr {
		if o["type"] != "session_summary" {
			t.Errorf("filter leaked type %v", o["type"])
		}
	}
}

func TestRecall_LimitCap(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	// Seed 10 observations directly via Store to avoid WAL deadlock from 10x bigmemRun saves.
	// Single Open/Close lifecycle prevents leftover WAL connections on Windows.
	store, err := bigmem.Open("")
	if err != nil {
		t.Fatalf("Open for seed: %v", err)
	}
	for i := 0; i < 10; i++ {
		obs := &bigmem.Observation{
			Title:   fmt.Sprintf("Limit test %d %d", i, time.Now().UnixNano()),
			Type:    "discovery",
			Content: fmt.Sprintf("content unique %d %d", i, time.Now().UnixNano()),
			Project: "biggz-ai",
		}
		if err := store.Save(obs); err != nil {
			store.Close()
			t.Fatalf("Save %d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	store.Close()

	// --limit 100 should clamp to 50 but we only have 10, so should return 10, not error
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = wOut
	os.Stderr = wErr
	savedArgs := os.Args
	os.Args = []string{"biggz", "recall", "--limit", "100", "--json", "--project", "biggz-ai"}
	code := recallRun()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	os.Args = savedArgs
	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = bytes.NewBuffer(nil).ReadFrom(rErr)
	if code != 0 {
		t.Fatalf("recall limit 100 failed %d", code)
	}
	var arr []map[string]any
	if err := json.Unmarshal(outBuf.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v out=%q", err, outBuf.String())
	}
	if len(arr) > 50 {
		t.Errorf("limit 100 should clamp to 50, got %d", len(arr))
	}
	if len(arr) != 10 {
		t.Errorf("expected 10, got %d", len(arr))
	}
}

func TestRecall_UnknownFlag(t *testing.T) {
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = wOut
	os.Stderr = wErr
	savedArgs := os.Args
	os.Args = []string{"biggz", "recall", "--unknown"}
	code := recallRun()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	os.Args = savedArgs
	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)
	if code == 0 {
		t.Errorf("unknown flag should fail")
	}
	if !strings.Contains(errBuf.String()+outBuf.String(), "unknown flag") {
		t.Errorf("should report unknown flag, got %q %q", outBuf.String(), errBuf.String())
	}
}

func TestBigmemSearch_HelpWarnsRecency(t *testing.T) {
	_, _, stderr := captureBigmemRun([]string{"search", "--help"})
	if !strings.Contains(stderr, "ORDER BY updated_at DESC") {
		t.Errorf("search help should warn recency, got %q", stderr)
	}
	if !strings.Contains(stderr, "For recency use") {
		t.Errorf("search help should contain guardrail, got %q", stderr)
	}
}
