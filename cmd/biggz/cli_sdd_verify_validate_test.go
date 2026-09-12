package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// verifyReportPath writes the shared verify-report fixture (review_parity_test.go)
// to a temp file so both input forms can be compared against the same bytes.
func verifyReportPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "verify-report.md")
	if err := os.WriteFile(path, []byte(verifyReportFixture), 0644); err != nil {
		t.Fatalf("write verify report: %v", err)
	}
	return path
}

func TestSDDVerifyValidate_PositionalPathEqualsInputFlag(t *testing.T) {
	path := verifyReportPath(t)
	flagCode, flagOut, flagErr := runVerifyValidate([]string{"--input", path, "--requirements", "5", "--scenarios", "10", "--json"}, "unused")
	posCode, posOut, posErr := runVerifyValidate([]string{path, "--requirements", "5", "--scenarios", "10", "--json"}, "unused")
	if posCode != flagCode || posOut != flagOut || posErr != flagErr {
		t.Fatalf("positional form diverged from --input form:\npositional: code=%d stdout=%q stderr=%q\n--input:    code=%d stdout=%q stderr=%q",
			posCode, posOut, posErr, flagCode, flagOut, flagErr)
	}
	if posCode != 0 {
		t.Fatalf("positional form exit code = %d, want 0", posCode)
	}
	if !strings.Contains(posOut, `"decision": "admitted"`) {
		t.Errorf("positional form stdout should admit the report, got: %s", posOut)
	}
}

func TestSDDVerifyValidate_PositionalPathHumanOutput(t *testing.T) {
	path := verifyReportPath(t)
	code, stdout, stderr := runVerifyValidate([]string{path}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.TrimSpace(stdout) != "Verify report is valid." {
		t.Errorf("stdout = %q, want human-readable validity line", stdout)
	}
}

func TestSDDVerifyValidate_StdinDashUnchanged(t *testing.T) {
	flagCode, flagOut, flagErr := runVerifyValidate([]string{"--input", "-", "--requirements", "5", "--scenarios", "10", "--json"}, verifyReportFixture)
	if flagCode != 0 {
		t.Fatalf("--input - exit code = %d, want 0 (stderr: %s)", flagCode, flagErr)
	}
	posCode, posOut, posErr := runVerifyValidate([]string{"-", "--requirements", "5", "--scenarios", "10", "--json"}, verifyReportFixture)
	if posCode != flagCode || posOut != flagOut || posErr != flagErr {
		t.Fatalf("positional - diverged from --input -:\npositional: code=%d stdout=%q stderr=%q\n--input:    code=%d stdout=%q stderr=%q",
			posCode, posOut, posErr, flagCode, flagOut, flagErr)
	}
}

func TestSDDVerifyValidate_PositionalAndInputConflict(t *testing.T) {
	path := verifyReportPath(t)
	for _, args := range [][]string{
		{path, "--input", path},
		{"--input", path, path},
	} {
		code, stdout, stderr := runVerifyValidate(args, "unused")
		if code != 1 {
			t.Errorf("args %v: exit code = %d, want 1", args, code)
		}
		if stdout != "" {
			t.Errorf("args %v: stdout = %q, want empty", args, stdout)
		}
		if !strings.Contains(stderr, "positional") || !strings.Contains(stderr, "--input") {
			t.Errorf("args %v: stderr must name both forms, got %q", args, stderr)
		}
	}
}

func TestSDDVerifyValidate_ExtraPositionalRejected(t *testing.T) {
	path := verifyReportPath(t)
	code, stdout, stderr := runVerifyValidate([]string{path, "extra-report.md"}, "unused")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `"extra-report.md"`) {
		t.Errorf("stderr must name the offending token, got %q", stderr)
	}
	if !strings.Contains(stderr, "unexpected positional") {
		t.Errorf("stderr should be a positional usage error, got %q", stderr)
	}
}

func TestSDDVerifyValidate_MissingInputUsageError(t *testing.T) {
	code, stdout, stderr := runVerifyValidate([]string{"--requirements", "5", "--scenarios", "10"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "input is required") {
		t.Errorf("stderr = %q, want required-input usage error", stderr)
	}
}

func TestSDDVerifyValidate_HelpDocumentsBothForms(t *testing.T) {
	code, _, stderr := runVerifyValidate([]string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "sdd-verify-validate <path|->") {
		t.Errorf("help must document the positional form, got %q", stderr)
	}
	if !strings.Contains(stderr, "--input <path|->") {
		t.Errorf("help must document the --input form, got %q", stderr)
	}
}

func TestSDDVerifyValidate_UnknownFlagRejected(t *testing.T) {
	code, _, stderr := runVerifyValidate([]string{"--nope"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, `unknown flag "--nope"`) {
		t.Errorf("stderr = %q, want unknown flag error", stderr)
	}
}
