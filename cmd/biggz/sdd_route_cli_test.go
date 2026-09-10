package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSddRouteCLI_Help(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddRoute([]string{"--help"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: biggz sdd-route") {
		t.Errorf("expected help text in stderr, got: %s", stderr.String())
	}
}

func TestSddRouteCLI_Direct(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddRoute([]string{"--files", "2", "--lines", "50", "--acceptance-clear", "--single-domain", "--verify"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 (direct), got %d", code)
	}
	if !strings.Contains(stdout.String(), "direct:") {
		t.Errorf("expected direct verdict in output, got: %s", stdout.String())
	}
}

func TestSddRouteCLI_Ask(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddRoute([]string{"--files", "7", "--lines", "200"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 (ask-sdd), got %d", code)
	}
	if !strings.Contains(stdout.String(), "ask-sdd:") {
		t.Errorf("expected ask-sdd verdict in output, got: %s", stdout.String())
	}
}

func TestSddRouteCLI_JSON(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddRoute([]string{"--new-domain", "--json"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 (ask-sdd), got %d", code)
	}
	if !strings.Contains(stdout.String(), `"route":"ask-sdd"`) || !strings.Contains(stdout.String(), "new-domain") {
		t.Errorf("expected JSON verdict with trigger, got: %s", stdout.String())
	}
}

func TestSddRouteCLI_UsageError(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddRoute([]string{"--bogus-flag"}, stdout, stderr)
	if code != 2 {
		t.Errorf("expected exit code 2 (usage error), got %d", code)
	}
}
