package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/project"
	"github.com/biggs-100/biggz-ai/internal/sdd"
)

// sessionCloseDefaultChange is the fallback attribution for degraded saves.
// APPLY-DECIDE Q1: keep the design default; deriving the active SDD change
// is out of scope for Cut 1.
const sessionCloseDefaultChange = "session-close"

// sessionCloseVerify and sessionCloseSave delegate to internal/sdd/session_guard.go.
// They are vars for test injection; production code must not duplicate BigMem logic.
var (
	sessionCloseVerify = sdd.VerifySessionSummaryWithWorkspace
	sessionCloseSave   = sdd.SaveSessionSummaryWithFallbackForChange
)

// sessionClosePayload is the --json contract for both modes.
type sessionClosePayload struct {
	Verified bool   `json:"verified"`
	Reason   string `json:"reason"`
	Fallback string `json:"fallback"`
	Status   string `json:"status,omitempty"`
}

func sessionCloseUsage() {
	fmt.Fprintln(os.Stderr, "Usage: biggz session-close (--check-only | --save \"text\") [--cwd <dir>] [--change <name>] [--json]")
	fmt.Fprintln(os.Stderr, "  --check-only   verify a persisted session_summary (no writes)")
	fmt.Fprintln(os.Stderr, "  --save \"text\"  persist summary, else write session-fallback.md (status degraded)")
	fmt.Fprintln(os.Stderr, "  --cwd <dir>    workspace root (default: .)")
	fmt.Fprintln(os.Stderr, "  --change <name>  fallback attribution (default: session-close)")
	fmt.Fprintln(os.Stderr, "  --json         machine-readable output {verified,reason,fallback[,status]}")
	fmt.Fprintln(os.Stderr, "Exits: 0 verified/saved, 1 blocked(session_summary_missing)/degraded, 2 usage error")
}

// sessionCloseRun is the thin session-close wrapper: parse flags, then
// delegate to session_guard.go with no duplicated BigMem logic.
func sessionCloseRun() int {
	args := os.Args[2:]
	cwd := "."
	change := sessionCloseDefaultChange
	jsonOut := false
	checkOnly := false
	saveText := ""
	hasSave := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--check-only":
			checkOnly = true
		case arg == "--json":
			jsonOut = true
		case arg == "--help" || arg == "-h":
			sessionCloseUsage()
			return 0
		case arg == "--cwd" || arg == "--change" || arg == "--save":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: %s requires a value\n", arg)
				sessionCloseUsage()
				return 2
			}
			i++
			switch arg {
			case "--cwd":
				cwd = args[i]
			case "--change":
				change = args[i]
			case "--save":
				saveText, hasSave = args[i], true
			}
		case strings.HasPrefix(arg, "--cwd="):
			cwd = strings.TrimPrefix(arg, "--cwd=")
		case strings.HasPrefix(arg, "--change="):
			change = strings.TrimPrefix(arg, "--change=")
		case strings.HasPrefix(arg, "--save="):
			saveText, hasSave = strings.TrimPrefix(arg, "--save="), true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintf(os.Stderr, "error: unknown flag %s\n", arg)
			sessionCloseUsage()
			return 2
		default:
			fmt.Fprintf(os.Stderr, "error: unexpected argument %s\n", arg)
			sessionCloseUsage()
			return 2
		}
	}

	if checkOnly == hasSave {
		if checkOnly {
			fmt.Fprintln(os.Stderr, "error: --check-only and --save are mutually exclusive")
		} else {
			fmt.Fprintln(os.Stderr, "error: one of --check-only or --save is required")
		}
		sessionCloseUsage()
		return 2
	}
	if strings.TrimSpace(change) == "" {
		change = sessionCloseDefaultChange
	}
	// An empty summary would persist vacuous evidence that satisfies the
	// gate. Reject it as usage error before touching the store.
	if hasSave && strings.TrimSpace(saveText) == "" {
		fmt.Fprintln(os.Stderr, "error: --save requires non-empty text")
		sessionCloseUsage()
		return 2
	}

	// Project filter: the gate applies only to biggz-ai. Foreign projects
	// verify as true without touching the store or writing fallback files.
	if info := project.DetectProjectFull(cwd); info.Project != "" && info.Project != "biggz-ai" {
		return sessionCloseAllow(cwd, info.Project, jsonOut, hasSave)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if checkOnly {
		return sessionCloseCheck(ctx, cwd, change, jsonOut)
	}
	return sessionCloseSaveRun(ctx, cwd, change, saveText, jsonOut)
}

// sessionCloseAllow reports the foreign-project fast path.
func sessionCloseAllow(cwd, proj string, jsonOut, isSave bool) int {
	if jsonOut {
		payload := sessionClosePayload{Verified: true}
		if isSave {
			payload.Status = "verified"
		}
		_ = json.NewEncoder(os.Stdout).Encode(payload)
		return 0
	}
	fmt.Fprintf(os.Stdout, "session-close: verified (project %s not gated, cwd %s)\n", proj, cwd)
	return 0
}

// sessionCloseCheck verifies a persisted session_summary via the shared guard.
func sessionCloseCheck(ctx context.Context, cwd, change string, jsonOut bool) int {
	info := project.DetectProjectFull(cwd)
	verified, err := sessionCloseVerify(ctx, cwd, info.Project)
	if err == nil && verified {
		if jsonOut {
			_ = json.NewEncoder(os.Stdout).Encode(sessionClosePayload{Verified: true})
			return 0
		}
		fmt.Fprintln(os.Stdout, "session-close: verified")
		return 0
	}
	reason := sdd.SessionSummaryMissingReason
	if err != nil {
		reason = fmt.Sprintf("%s: %v", sdd.SessionSummaryMissingReason, err)
	}
	fallback := sdd.FallbackFilePath(cwd, change)
	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(sessionClosePayload{Verified: false, Reason: reason, Fallback: fallback})
		return 1
	}
	fmt.Fprintf(os.Stderr, "%s\n", reason)
	fmt.Fprintf(os.Stderr, "To save: biggz session-close --save \"summary text\" --cwd %s\n", cwd)
	fmt.Fprintf(os.Stderr, "On persistent failure a degraded fallback is written to %s (does not satisfy the gate)\n", fallback)
	return 1
}

// sessionCloseSaveRun persists via the shared guard (direct store, retry-once
// and degraded-file semantics stay inside the guard).
func sessionCloseSaveRun(ctx context.Context, cwd, change, text string, jsonOut bool) int {
	info := project.DetectProjectFull(cwd)
	id, err := sessionCloseSave(ctx, cwd, change, info.Project, "", text, true)
	if err == nil {
		if jsonOut {
			_ = json.NewEncoder(os.Stdout).Encode(sessionClosePayload{Verified: true, Status: "verified"})
			return 0
		}
		if id != "" {
			fmt.Fprintf(os.Stdout, "session-close: saved %s\n", id)
		} else {
			fmt.Fprintln(os.Stdout, "session-close: saved")
		}
		return 0
	}
	// Persistent failure: the guard wrote session-fallback.md and returned
	// its path as id. Degraded evidence never satisfies the gate.
	fallback := id
	if fallback == "" {
		fallback = sdd.FallbackPath(change)
	}
	reason := err.Error()
	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(sessionClosePayload{Verified: false, Reason: reason, Fallback: fallback, Status: "degraded"})
		return 1
	}
	fmt.Fprintf(os.Stderr, "%s\n", reason)
	fmt.Fprintf(os.Stderr, "Degraded fallback written to %s (does not satisfy the gate)\n", fallback)
	return 1
}
