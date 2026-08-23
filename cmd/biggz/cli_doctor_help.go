package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/doctor"
)

// printHelp prints the top-level help text.
func printHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz <command> [args...]")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  install                    Install biggz-ai in your AI agent")
	fmt.Fprintln(os.Stderr, "  uninstall [--yes] [--purge]  Remove biggz-ai from your system (agent configs + ~/.biggz)")
	fmt.Fprintln(os.Stderr, "  sdd-status                 Show SDD change status [--cwd <dir>] [--json] [--instructions]")
	fmt.Fprintln(os.Stderr, "  sdd-apply <change>         Validate edit authority for apply (allow/block)")
	fmt.Fprintln(os.Stderr, "  sdd-verify-validate        Validate verify reports")
	fmt.Fprintln(os.Stderr, "  sdd-attempt                Manage attempt budgets")
	fmt.Fprintln(os.Stderr, "  sdd-continue <change>      Determine next SDD phase")
	fmt.Fprintln(os.Stderr, "  sdd-new [change-name]      Interactive wizard to create new SDD change")
	fmt.Fprintln(os.Stderr, "  bigmem save|search|get     Persistent memory")
	fmt.Fprintln(os.Stderr, "  backup create|list|restore Snapshot/restore state")
	fmt.Fprintln(os.Stderr, "  release status|tag|verify  Version management")
	fmt.Fprintln(os.Stderr, "  skill-registry refresh     Regenerate skill registry [--force] [--quiet] [--cwd <dir>] [--no-gitignore]")
	fmt.Fprintln(os.Stderr, "  review list|status|gate|start|resume|validate|repair|recover|reclaim|reconcile-authority|dispose-result|reopen-results|inspect|schema|retry-final-verification|invalidate|abandon|export|import  Review lineage commands")
	fmt.Fprintln(os.Stderr, "  doctor [--json] [--fix]   Run system health checks")
	fmt.Fprintln(os.Stderr, "  update [--dry-run]       Update biggz-ai to latest version")
	fmt.Fprintln(os.Stderr, "  sync [flags]             Deploy skills, config, prompts, and commands")
	fmt.Fprintln(os.Stderr, "  pr create <change>       Auto-generate branch and PR from SDD apply")
	fmt.Fprintln(os.Stderr, "  rdd enable|disable|status  RDD kill switch")
	fmt.Fprintln(os.Stderr, "  recovery list|show|generate|validate|export|import|delete  Recovery trace ledger")
	fmt.Fprintln(os.Stderr, "  mcp                        Start MCP server")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Without arguments: open interactive TUI")
}

// ---------------------------------------------------------------------------
// Doctor Command
// ---------------------------------------------------------------------------

// doctorRun handles the "biggz doctor" subcommand.
// Usage: biggz doctor [--json] [--fix]
func doctorRun() int {
	ctx := context.Background()

	// Parse flags
	jsonOutput := false
	fixMode := false
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--fix":
			fixMode = true
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz doctor [--json] [--fix]")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Run system health checks. Without flags, prints human-readable table.")
			fmt.Fprintln(os.Stderr, "  --json    Machine-readable JSON output")
			fmt.Fprintln(os.Stderr, "  --fix     Attempt to fix issues automatically")
			return 0
		}
	}

	// Build runner with all 9 checks.
	runner := &doctor.Runner{
		Checks: []doctor.Check{
			doctor.NewBigmemCheck(),
			doctor.NewBinaryCheck(),
			doctor.NewConfigCheck(),
			doctor.NewDiskCheck(),
			doctor.NewPathCheck(),
			doctor.NewGitCheck(),
			doctor.NewVersionCheck(),
			doctor.NewBackupCheck(),
			doctor.NewReviewCheck(),
		},
	}

	report := runner.RunAll(ctx)

	if fixMode {
		doctorFix(ctx, runner, report)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
		return report.ExitCode()
	}

	// Human-readable table output.
	printDoctorTable(report)

	return report.ExitCode()
}

// printDoctorTable renders a severity-grouped table of doctor results to stderr.
func printDoctorTable(report *doctor.Report) {
	if len(report.Critical) > 0 {
		fmt.Fprintln(os.Stderr, "=== CRITICAL ===")
		for _, r := range report.Critical {
			line := fmt.Sprintf("%s %s: %s", statusIcon(r.Status), r.ID, r.Message)
			if r.Error != "" {
				line += " (" + r.Error + ")"
			}
			fmt.Fprintln(os.Stderr, line)
		}
		fmt.Fprintln(os.Stderr)
	}

	if len(report.Warning) > 0 {
		fmt.Fprintln(os.Stderr, "=== WARNING ===")
		for _, r := range report.Warning {
			line := fmt.Sprintf("%s %s: %s", statusIcon(r.Status), r.ID, r.Message)
			if r.Error != "" {
				line += " (" + r.Error + ")"
			}
			fmt.Fprintln(os.Stderr, line)
		}
		fmt.Fprintln(os.Stderr)
	}

	if len(report.Info) > 0 {
		fmt.Fprintln(os.Stderr, "=== INFO ===")
		for _, r := range report.Info {
			line := fmt.Sprintf("%s %s: %s", statusIcon(r.Status), r.ID, r.Message)
			if r.Error != "" {
				line += " (" + r.Error + ")"
			}
			fmt.Fprintln(os.Stderr, line)
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintf(os.Stderr, "Summary: %d CRITICAL, %d WARNING, %d INFO\n",
		len(report.Critical), len(report.Warning), len(report.Info))
}

// statusIcon returns the table icon for a given status.
func statusIcon(s doctor.Status) string {
	switch s {
	case doctor.StatusPass:
		return "[ok]"
	case doctor.StatusWarn:
		return "[!!]"
	case doctor.StatusFail:
		return "[xx]"
	default:
		return "[??]"
	}
}

// doctorFix iterates results with non-nil remedies, executes them,
// and re-runs affected checks.
func doctorFix(ctx context.Context, runner *doctor.Runner, report *doctor.Report) {
	// Build a map of check ID to Check for fast lookup.
	checkMap := make(map[doctor.CheckID]doctor.Check)
	for _, check := range runner.Checks {
		checkMap[check.ID()] = check
	}

	var applied int
	for _, result := range report.All() {
		check, ok := checkMap[result.ID]
		if !ok {
			continue
		}
		remedy := check.Remedy()
		if remedy == nil {
			continue
		}
		applied++
		fmt.Fprintf(os.Stderr, "Applying remedy for %s: %s\n", result.ID, remedy.Description)
		if err := remedy.Action(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "  FAILED: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "  OK\n")
		}
	}

	if applied == 0 {
		fmt.Fprintln(os.Stderr, "No fixable issues found.")
		return
	}

	// Re-run affected checks after fixes.
	fmt.Fprintln(os.Stderr, "\nRe-running checks after fixes...")
	newReport := runner.RunAll(ctx)
	printDoctorTable(newReport)
}

// mcpRun starts the biggz-ai MCP server for agent integration.
func mcpRun() int {
	// Run the MCP server from the biggz-mcp package
	// For now, exec the separate binary
	tools := "--tools=agent"
	for _, arg := range os.Args[2:] {
		if strings.HasPrefix(arg, "--tools=") {
			tools = arg
		}
	}
	cmd := exec.Command("biggz-mcp", tools)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: mcp: %v\n", err)
		return 1
	}
	return 0
}
