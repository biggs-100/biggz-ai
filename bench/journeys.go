// Package main: journey corpus for ceremony friction.
//
// Each journey builds its own fixture (temp HOME, no git repo so the
// machine-scoped ledger applies), drives the product binary, and asserts
// observable behavior: exit codes, output markers, persisted state.
// Journeys must stay black-box: argv, exit code, stdout/stderr, files.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// journey is one friction scenario.
type journey struct {
	id   string
	desc string
	run  func(bin string) JourneyResult
}

var journeys = []journey{
	{"j01-settle-admits-without-acquire", "settle with unknown token exits 0 with warning (admissible ledger)", jSettleAdmits},
	{"j02-settle-strict-blocks", "settle --strict with unknown token exits 1 (lock preserved)", jSettleStrict},
	{"j03-check-only-preserves-guard", "session-close --check-only without summary exits 1 with token (fail-closed intact)", jCheckOnly},
	{"j04-install-deploys-quiet-markers", "install deploys quiet-ceremony prompt markers", jInstallMarkers},
}

// fixture creates an isolated HOME and work dir, returning cleanup.
func fixture() (home, work string, cleanup func(), err error) {
	base, err := os.MkdirTemp("", "ceremony-bench-")
	if err != nil {
		return "", "", nil, err
	}
	home = filepath.Join(base, "home")
	work = filepath.Join(base, "work")
	if err := os.MkdirAll(home, 0755); err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(work, 0755); err != nil {
		return "", "", nil, err
	}
	return home, work, func() { os.RemoveAll(base) }, nil
}

// runCapture runs the binary with isolated env and full stdout/stderr capture.
func runCapture(bin, home, work string, args ...string) Observation {
	cmd := exec.Command(bin, args...)
	cmd.Dir = work
	cmd.Env = []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
		// Pin the project so gates behave as in biggz-ai (temp dir
		// basenames would otherwise detect as foreign projects).
		"BIGGZ_PROJECT=biggz-ai",
		"PATH=" + os.Getenv("PATH"),
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 99
			stderr.WriteString("start failed: " + err.Error())
		}
	}
	return Observation{Args: args, ExitCode: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

func fail(id, format string, args ...any) JourneyResult {
	return JourneyResult{ID: id, Pass: false, Detail: fmt.Sprintf(format, args...)}
}

func pass(id, detail string, steps []Observation) JourneyResult {
	return JourneyResult{ID: id, Pass: true, Detail: detail, Steps: steps}
}

// acquireArgs builds the sdd-attempt acquire argv (<change> positional).
func acquireArgs(change, req string) []string {
	return []string{"sdd-attempt", "acquire", change,
		"--request-id", req,
		"--work-unit", "bench",
		"--evidence-goal", "bench",
	}
}

// settleArgs builds the common sdd-attempt settle argv (<change> positional).
func settleArgs(change, token, req, outcome string, extra ...string) []string {
	a := []string{"sdd-attempt", "settle", change,
		"--token", token,
		"--request-id", req,
		"--outcome", outcome,
		"--diagnosis", "bench",
		"--harness-disposition", "bench",
		"--cleanup-evidence", "bench",
		"--process-evidence", "bench",
	}
	return append(a, extra...)
}

// jSettleAdmits: unknown token settles with exit 0 and a warning field.
func jSettleAdmits(bin string) JourneyResult {
	const id = "j01-settle-admits-without-acquire"
	home, work, cleanup, err := fixture()
	if err != nil {
		return fail(id, "fixture: %v", err)
	}
	defer cleanup()
	// Create the ledger first; the unknown token is settled afterwards.
	acq := runCapture(bin, home, work, acquireArgs("bench-change", "req-bench-1a")...)
	if acq.ExitCode != 0 {
		return JourneyResult{ID: id, Pass: false,
			Detail: fmt.Sprintf("acquire fixture failed: %s", firstLine(acq.Stderr)),
			Steps:  []Observation{acq}}
	}
	obs := runCapture(bin, home, work, settleArgs("bench-change", "tok-bench-unknown", "req-bench-1", "passed",
		"--evidence-revision", "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")...)
	if obs.ExitCode != 0 {
		return JourneyResult{ID: id, Pass: false,
			Detail: fmt.Sprintf("expected exit 0, got %d: %s", obs.ExitCode, firstLine(obs.Stderr)),
			Steps:  []Observation{obs}}
	}
	if !strings.Contains(obs.Stdout, `"warning"`) {
		return JourneyResult{ID: id, Pass: false,
			Detail: "exit 0 but no warning field in settle output",
			Steps:  []Observation{obs}}
	}
	return pass(id, "unknown token admitted with warning", []Observation{obs})
}

// jSettleStrict: --strict restores the block.
func jSettleStrict(bin string) JourneyResult {
	const id = "j02-settle-strict-blocks"
	home, work, cleanup, err := fixture()
	if err != nil {
		return fail(id, "fixture: %v", err)
	}
	defer cleanup()
	acq := runCapture(bin, home, work, acquireArgs("bench-change", "req-bench-2a")...)
	if acq.ExitCode != 0 {
		return JourneyResult{ID: id, Pass: false,
			Detail: fmt.Sprintf("acquire fixture failed: %s", firstLine(acq.Stderr)),
			Steps:  []Observation{acq}}
	}
	args := append(settleArgs("bench-change", "tok-bench-unknown", "req-bench-2", "passed"), "--strict")
	obs := runCapture(bin, home, work, args...)
	if obs.ExitCode == 0 {
		return JourneyResult{ID: id, Pass: false,
			Detail: "expected non-zero exit in strict mode, got 0",
			Steps:  []Observation{obs}}
	}
	if !strings.Contains(obs.Stderr, "invalid_continuation") && !strings.Contains(obs.Stdout, "invalid_continuation") {
		return JourneyResult{ID: id, Pass: false,
			Detail: fmt.Sprintf("expected invalid_continuation, got exit %d: %s", obs.ExitCode, firstLine(obs.Stderr)),
			Steps:  []Observation{obs}}
	}
	return pass(id, "strict mode blocks with invalid_continuation", []Observation{obs})
}

// jCheckOnly: check-only without summary stays fail-closed.
func jCheckOnly(bin string) JourneyResult {
	const id = "j03-check-only-preserves-guard"
	home, work, cleanup, err := fixture()
	if err != nil {
		return fail(id, "fixture: %v", err)
	}
	defer cleanup()
	obs := runCapture(bin, home, work, "session-close", "--check-only", "--cwd", work)
	if obs.ExitCode == 0 {
		return JourneyResult{ID: id, Pass: false,
			Detail: "expected non-zero exit with no summary persisted",
			Steps:  []Observation{obs}}
	}
	combined := obs.Stdout + obs.Stderr
	if !strings.Contains(combined, "session_summary_missing") {
		return JourneyResult{ID: id, Pass: false,
			Detail: fmt.Sprintf("expected session_summary_missing token, got: %s", firstLine(combined)),
			Steps:  []Observation{obs}}
	}
	return pass(id, "check-only fail-closed with gate token", []Observation{obs})
}

// jInstallMarkers: deployed prompt carries quiet-ceremony markers.
func jInstallMarkers(bin string) JourneyResult {
	const id = "j04-install-deploys-quiet-markers"
	home, work, cleanup, err := fixture()
	if err != nil {
		return fail(id, "fixture: %v", err)
	}
	defer cleanup()
	obs := runCapture(bin, home, work, "install", "--agent", "pi", "--yes", "--home", home)
	if obs.ExitCode != 0 {
		return JourneyResult{ID: id, Pass: false,
			Detail: fmt.Sprintf("install failed: %s", firstLine(obs.Stdout+obs.Stderr)),
			Steps:  []Observation{obs}}
	}
	// Walk deployed files under the temp HOME for the quiet marker.
	found := false
	var checked int
	_ = checked
	filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), "Post-Delegation Report") {
			found = true
		}
		return nil
	})
	if !found {
		return JourneyResult{ID: id, Pass: false,
			Detail: "deployed files lack Post-Delegation Report marker",
			Steps:  []Observation{obs}}
	}
	return pass(id, "quiet-ceremony markers deployed", []Observation{obs})
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
