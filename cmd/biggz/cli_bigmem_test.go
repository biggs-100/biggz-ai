package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
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
