package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/biggs-100/biggz-ai/internal/uninstall"
)

// uninstallRun handles the "biggz uninstall" subcommand.
// Usage: biggz uninstall [--yes] [--dry-run] [--purge] [--agent <id>] [--home <dir>]
//
// It reverses the install inventory for every registered agent adapter (or
// only --agent), removes the shared ~/.biggz store, and reports per-agent
// results. Exit code is 1 iff any operation failed.
func uninstallRun() int {
	ctx := context.Background()

	yes, dryRun, purge := false, false, false
	var selectedAgent, homeDir string
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--yes":
			yes = true
		case "--dry-run":
			dryRun = true
		case "--purge":
			purge = true
		case "--agent":
			if i+1 < len(args) {
				i++
				selectedAgent = args[i]
			}
		case "--home":
			if i+1 < len(args) {
				i++
				homeDir = args[i]
			}
		case "--help", "-h":
			printUninstallHelp()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", args[i])
			printUninstallHelp()
			return 1
		}
	}

	res, err := uninstall.Run(ctx, agentAdapters(), uninstall.Config{
		HomeDir: homeDir,
		AgentID: selectedAgent,
		Yes:     yes,
		DryRun:  dryRun,
		Purge:   purge,
	})
	if err != nil {
		if errors.Is(err, uninstall.ErrCancelled) {
			fmt.Println("Uninstall cancelled.")
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	for _, ar := range res.AgentResults {
		if ar.RemovedFiles == 0 && ar.RewrittenConfigs == 0 {
			continue
		}
		verb := "removed"
		if res.DryRun {
			verb = "would remove"
		}
		fmt.Printf("%s: %s %d files, %d configs rewritten\n",
			ar.AgentID, verb, ar.RemovedFiles, ar.RewrittenConfigs)
	}
	for _, f := range res.Failed {
		fmt.Fprintf(os.Stderr, "%s: FAILED %s: %v\n", f.Agent, f.Op, f.Err)
	}
	fmt.Println(res.Summary)
	if len(res.Failed) > 0 {
		retry := "biggz uninstall --yes"
		if selectedAgent != "" {
			retry += " --agent " + selectedAgent
		}
		fmt.Fprintf(os.Stderr, "Run: %s to retry the failed operations\n", retry)
		return 1
	}
	return 0
}

// printUninstallHelp prints the uninstall subcommand help text.
func printUninstallHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz uninstall [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Remove biggz-ai managed artifacts from your system: agent skills,")
	fmt.Fprintln(os.Stderr, "prompts, commands, plugins, settings keys, AGENTS.md sections, MCP")
	fmt.Fprintln(os.Stderr, "config, and the shared ~/.biggz store. User data")
	fmt.Fprintln(os.Stderr, "(~/.biggz/bigmem, ~/.biggz/backups, ~/.config/biggz/custom-agents.json)")
	fmt.Fprintln(os.Stderr, "is kept unless --purge.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  --yes              Skip the confirmation prompt (required in non-interactive mode)")
	fmt.Fprintln(os.Stderr, "  --dry-run          Report what would be removed without changing anything")
	fmt.Fprintln(os.Stderr, "  --purge            Also delete ~/.biggz/bigmem and ~/.biggz/backups")
	fmt.Fprintln(os.Stderr, "  --agent <name>     Restrict to one agent adapter (e.g., opencode, claude)")
	fmt.Fprintln(os.Stderr, "  --home <dir>       Custom home directory for testing")
	fmt.Fprintln(os.Stderr, "  --help, -h         Show this help message")
}
