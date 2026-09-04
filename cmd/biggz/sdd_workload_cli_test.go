package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSddWorkloadCLI_Help(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddWorkload([]string{"--help"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: biggz sdd-workload") {
		t.Errorf("expected help text in stderr, got: %s", stderr.String())
	}
}

func TestSddWorkloadCLI_Allow(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	forecast := `{"estimated_lines":100}`
	code := runSddWorkload([]string{"--forecast", forecast, "--strategy", "ask-on-risk"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "ALLOW") {
		t.Errorf("expected ALLOW in output, got: %s", stdout.String())
	}
}

func TestSddWorkloadCLI_Ask(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	forecast := `{"estimated_lines":500,"chained_prs_recommended":true}`
	code := runSddWorkload([]string{"--forecast", forecast, "--strategy", "ask-on-risk"}, stdout, stderr)
	if code != 2 {
		t.Errorf("expected exit code 2 (ASK), got %d", code)
	}
	if !strings.Contains(stdout.String(), "ASK") {
		t.Errorf("expected ASK in output, got: %s", stdout.String())
	}
}

func TestSddWorkloadCLI_Block(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	forecast := `{"estimated_lines":500,"chained_prs_recommended":true}`
	code := runSddWorkload([]string{"--forecast", forecast, "--strategy", "single-pr"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1 (BLOCK), got %d", code)
	}
	if !strings.Contains(stdout.String(), "BLOCK") {
		t.Errorf("expected BLOCK in output, got: %s", stdout.String())
	}
}

func TestSddWorkloadCLI_AutoChainWithStrategy(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	forecast := `{"estimated_lines":500,"chained_prs_recommended":true}`
	code := runSddWorkload([]string{"--forecast", forecast, "--strategy", "auto-chain", "--chain", "stacked-to-main"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "ALLOW") {
		t.Errorf("expected ALLOW in output, got: %s", stdout.String())
	}
}

func TestSddWorkloadCLI_AutoChainWithoutStrategy(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	forecast := `{"estimated_lines":500,"chained_prs_recommended":true}`
	code := runSddWorkload([]string{"--forecast", forecast, "--strategy", "auto-chain"}, stdout, stderr)
	if code != 2 {
		t.Errorf("expected exit code 2 (ASK for chain strategy), got %d", code)
	}
}

func TestSddWorkloadCLI_MissingArgs(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddWorkload([]string{}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 (help), got %d", code)
	}
}

func TestSddWorkloadCLI_MissingForecast(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddWorkload([]string{"--strategy", "ask-on-risk"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--forecast is required") {
		t.Errorf("expected --forecast error, got: %s", stderr.String())
	}
}

func TestSddWorkloadCLI_MissingStrategy(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddWorkload([]string{"--forecast", "{}"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--strategy is required") {
		t.Errorf("expected --strategy error, got: %s", stderr.String())
	}
}

func TestSddWorkloadCLI_InvalidStrategy(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddWorkload([]string{"--forecast", "{}", "--strategy", "invalid"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid strategy") {
		t.Errorf("expected invalid strategy error, got: %s", stderr.String())
	}
}

func TestSddWorkloadCLI_InvalidChainStrategy(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddWorkload([]string{"--forecast", "{}", "--strategy", "auto-chain", "--chain", "invalid"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid chain strategy") {
		t.Errorf("expected invalid chain strategy error, got: %s", stderr.String())
	}
}
