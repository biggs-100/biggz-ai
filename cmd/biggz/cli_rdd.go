package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/opencodeplugin"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/state"
	"github.com/biggs-100/biggz-ai/plugin"
)

// rddRun handles the "biggz rdd" subcommand.
// Usage: biggz rdd <enable|disable|status> [--scope worktree|clone|global] [--expected-revision <hash>] [--json]
func rddRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, "Usage: biggz rdd enable|disable|status [--scope worktree|clone|global] [--expected-revision <hash>] [--json]")
		fmt.Fprintln(os.Stderr, "  --scope worktree|clone|global (default worktree for enable/disable)")
		fmt.Fprintln(os.Stderr, "  --expected-revision <hash> only for disable --scope=clone|worktree (CAS)")
		fmt.Fprintln(os.Stderr, "  --json                Machine-readable JSON output (status only)")
		return 0
	}

	op := args[0]
	scope := "worktree" // default to narrowest scope (Alan's #1973 recommendation)
	expectedRevision := ""
	jsonOutput := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--scope=") {
			scope = strings.TrimPrefix(arg, "--scope=")
			continue
		}
		if strings.HasPrefix(arg, "--expected-revision=") {
			expectedRevision = strings.TrimPrefix(arg, "--expected-revision=")
			continue
		}
		switch arg {
		case "--scope":
			if i+1 < len(args) {
				scope = args[i+1]
				i++
			}
		case "--expected-revision":
			if i+1 < len(args) {
				expectedRevision = args[i+1]
				i++
			}
		case "--json":
			jsonOutput = true
		}
	}

	// Detect both git dirs: --git-common-dir (clone scope, shared by worktrees)
	// and --git-dir (worktree scope, private to this worktree)
	commonDir, worktreeDir := detectGitDirs()

	var status *review.RDDStatusReport
	var err error

	switch op {
	case "status":
		status, err = review.RDDStatus(worktreeDir, commonDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if encErr := enc.Encode(status); encErr != nil {
				fmt.Fprintf(os.Stderr, "error: encoding status: %v\n", encErr)
				return 1
			}
			return 0
		}
	case "enable":
		// Enable does not use CAS; expectedRevision is ignored for global enable (clear).
		if expectedRevision != "" && (scope == "clone" || scope == "worktree") {
			// For clone/worktree enable, clearing with CAS is not required; treat as advisory.
			// Still forward to the CAS-aware clear via RDDEnable which tolerates non-git dirs.
		}
		status, err = review.RDDEnable(worktreeDir, commonDir)
	case "disable":
		// Forward expectedRevision to the CAS-aware writer for clone/worktree.
		switch scope {
		case "worktree":
			if worktreeDir == "" {
				fmt.Fprintln(os.Stderr, "error: not in a git worktree — cannot use --scope=worktree")
				return 1
			}
			if expectedRevision != "" {
				_, werr := review.SetWorktreeRDDMode(worktreeDir, review.RDDModeDisabled, expectedRevision)
				if werr != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", werr)
					return 1
				}
				status, err = review.RDDStatus(worktreeDir, commonDir)
			} else {
				status, err = review.RDDDisable(worktreeDir, commonDir, scope)
			}
		case "clone":
			if commonDir == "" {
				fmt.Fprintln(os.Stderr, "error: not in a git repository — cannot use --scope=clone")
				return 1
			}
			if expectedRevision != "" {
				_, werr := review.SetCloneLocalRDDMode(worktreeDir, commonDir, review.RDDModeDisabled, expectedRevision)
				if werr != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", werr)
					return 1
				}
				status, err = review.RDDStatus(worktreeDir, commonDir)
			} else {
				status, err = review.RDDDisable(worktreeDir, commonDir, scope)
			}
		default: // global
			if expectedRevision != "" {
				fmt.Fprintln(os.Stderr, "error: --expected-revision is only supported for --scope=clone and --scope=worktree")
				return 1
			}
			status, err = review.RDDDisable(worktreeDir, commonDir, scope)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown rdd command: %s\n", op)
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Print status (text mode). JSON already returned.
	if !jsonOutput || op != "status" {
		fmt.Printf("RDD Status: %s\n", status.EffectiveMode)
		fmt.Printf("  Global:   %s\n", status.GlobalMode)
		if commonDir != "" {
			fmt.Printf("  Clone:    %s\n", status.CloneMode)
		}
		if worktreeDir != "" && worktreeDir != commonDir {
			fmt.Printf("  Worktree: %s\n", status.WorktreeMode)
		}
		fmt.Printf("  Source:   %s\n", status.Source)
		if status.Revision != "" {
			fmt.Printf("  Revision: %s\n", status.Revision)
		}
		if status.Reach != "" {
			fmt.Printf("  Reach:    %s\n", status.Reach)
		} else if op == "disable" && (scope == "clone" || scope == "worktree") {
			// After a write, show Reach if available; otherwise status Reach is unreported.
			fmt.Printf("  Reach:    %s\n", status.Reach)
		}
		if status.RecordedAt != nil {
			fmt.Printf("  Since:    %s\n", status.RecordedAt.Format(time.RFC3339))
		}

		// Blast radius announcement (Alan's #1973 item 1)
		if op == "disable" && status.WorktreeCount > 0 {
			fmt.Printf("\n⚠️  Disabled RDD for scope %q — this reaches %d linked worktrees of this clone.\n",
				scope, status.WorktreeCount)
			fmt.Println("   Use --scope=worktree to disable for THIS worktree only.")
		}
		if op == "enable" && status.WorktreeCount > 0 {
			fmt.Printf("\n⚠️  Enabled RDD globally — this re-enables RDD for all %d worktrees of this clone.\n",
				status.WorktreeCount)
		}
	}

	return 0
}

// tddRun handles "biggz tdd <enable|disable|status>".
func tddRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz tdd <enable|disable|status>")
		fmt.Fprintln(os.Stderr, "  enable   — Enable Strict TDD Mode (injects marker into AGENTS.md)")
		fmt.Fprintln(os.Stderr, "  disable  — Disable Strict TDD Mode (removes marker from AGENTS.md)")
		fmt.Fprintln(os.Stderr, "  status   — Show current Strict TDD mode")
		return 0
	}

	op := args[0]
	home, _ := os.UserHomeDir()

	// Detect the agent
	var agent plugin.AgentAdapter
	for _, name := range priorityAgents() {
		a := agentAdapters()[name]
		installed, _, _, _, _ := a.Detect(context.Background(), home)
		if installed {
			agent = a
			break
		}
	}
	if agent == nil {
		agent = agentAdapters()["opencode"] // default fallback
	}

	switch op {
	case "enable":
		if err := install.DeployStrictTDDMode(agent, home, true, false); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		state.SetBigMemConfig("strict_tdd", true)
		fmt.Println("Strict TDD Mode: enabled")
		fmt.Println("  sdd-apply will now follow RED → GREEN → REFACTOR cycle")
		fmt.Println("  sdd-verify will validate TDD compliance")

	case "disable":
		if err := install.DeployStrictTDDMode(agent, home, false, false); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		state.SetBigMemConfig("strict_tdd", false)
		fmt.Println("Strict TDD Mode: disabled")

	case "status":
		enabled, _ := state.GetBigMemConfig("strict_tdd")
		// Also check AGENTS.md marker
		promptFile := agent.SystemPromptFile(home)
		if data, err := os.ReadFile(promptFile); err == nil {
			if strings.Contains(string(data), "<!-- biggz:strict-tdd-mode -->") {
				enabled = true
			}
		}
		if enabled {
			fmt.Println("Strict TDD Mode: enabled")
		} else {
			fmt.Println("Strict TDD Mode: disabled")
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown tdd command: %s (use enable, disable, status)\n", op)
		return 1
	}
	return 0
}

// updateStrictTDDInConfig reads opencode.json, sets strict_tdd, and writes back.
// pluginRun handles "biggz plugin <install|uninstall|list>".
func pluginRun() int {
	args := os.Args[2:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(os.Stderr, opencodeplugin.FormatPluginList())
		return 0
	}

	switch args[0] {
	case "list":
		installed, err := opencodeplugin.ListInstalled()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if len(installed) == 0 {
			fmt.Println("No community plugins installed.")
			fmt.Println()
			fmt.Print(opencodeplugin.FormatPluginList())
		} else {
			fmt.Println("Installed community plugins:")
			for _, p := range installed {
				fmt.Printf("  • %s\n", p)
			}
		}

	case "install":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz plugin install <name>")
			return 1
		}
		if err := opencodeplugin.Install(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Plugin %q installed.\n", args[1])

	case "uninstall":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz plugin uninstall <name>")
			return 1
		}
		if err := opencodeplugin.Uninstall(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Plugin %q uninstalled.\n", args[1])

	default:
		fmt.Fprintf(os.Stderr, "unknown plugin command %q (use: install, uninstall, list)\n", args[0])
		return 1
	}
	return 0
}

func _deprecated_updateStrictTDDInConfig(cfgPath string, enabled bool) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	cfg["strict_tdd"] = enabled
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(cfgPath, out, 0644)
}

var _ = time.Now
