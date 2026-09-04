package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSddDedupCLI_Help(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddDedup([]string{"--help"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: biggz sdd-dedup") {
		t.Errorf("expected help text in stderr, got: %s", stderr.String())
	}
}

func TestSddDedupCLI_RecordAndCheck(t *testing.T) {
	tmpFile := t.TempDir() + "/dedup_state.json"
	
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	// First call: record
	code := runSddDedup([]string{"spec", "Create spec for feature X", "--record", "--state", tmpFile}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "OK (recorded)") {
		t.Errorf("expected OK (recorded), got: %s", stdout.String())
	}
	
	// Second call: should be blocked
	stdout.Reset()
	stderr.Reset()
	code = runSddDedup([]string{"spec", "Create spec for feature X", "--state", tmpFile}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "BLOCKED") {
		t.Errorf("expected BLOCKED, got: %s", stdout.String())
	}
}

func TestSddDedupCLI_DifferentTask(t *testing.T) {
	tmpFile := t.TempDir() + "/dedup_state.json"
	
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	// Record first task
	runSddDedup([]string{"spec", "Create spec for feature X", "--record", "--state", tmpFile}, stdout, stderr)
	
	// Different task should not be blocked
	stdout.Reset()
	stderr.Reset()
	code := runSddDedup([]string{"spec", "Create spec for feature Y", "--state", tmpFile}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("expected OK, got: %s", stdout.String())
	}
}

func TestSddDedupCLI_MissingArgs(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddDedup([]string{}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 (help), got %d", code)
	}
}

func TestSddDedupCLI_MissingTask(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddDedup([]string{"spec"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "task description is required") {
		t.Errorf("expected task error, got: %s", stderr.String())
	}
}

func TestSddDedupCLI_NoState(t *testing.T) {
	// Without --state, each call is independent (no persistence)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	// First call
	code := runSddDedup([]string{"spec", "Create spec for feature X", "--record"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	
	// Second call without state should NOT be blocked
	stdout.Reset()
	stderr.Reset()
	code = runSddDedup([]string{"spec", "Create spec for feature X"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 (no state), got %d", code)
	}
}
