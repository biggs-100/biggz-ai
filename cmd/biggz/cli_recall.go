package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/internal/project"
)

func recallHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz recall [--type T] [--project P] [--scope S] [--limit N] [--json] [--all|--all-projects] [--match-mode all|any]")
	fmt.Fprintln(os.Stderr, "       biggz bigmem recent has same flags (alias)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Recency uses empty query ordered by updated_at DESC (bigmem.go:1801)")
	fmt.Fprintln(os.Stderr, "  For recency use bigmem search --query \"\" ORDER BY updated_at DESC or biggz recall; never use FTS term search for 'latest'.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  --type T              Filter by observation type (e.g. session_summary)")
	fmt.Fprintln(os.Stderr, "  --project P           Filter by project")
	fmt.Fprintln(os.Stderr, "  --scope S             Filter by scope (project|personal)")
	fmt.Fprintln(os.Stderr, "  --limit N             Max results (default 20, capped at 50)")
	fmt.Fprintln(os.Stderr, "  --json                Output JSON array with updated_at")
	fmt.Fprintln(os.Stderr, "  --all, --all-projects Search across all projects")
	fmt.Fprintln(os.Stderr, "  --match-mode all|any  Match mode forwarded to Search (default all)")
}

func parseRecallArgs(args []string) (bigmem.SearchOptions, bool, error) {
	opts := bigmem.SearchOptions{Limit: 20}
	useJSON := false
	hasProject := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			return opts, useJSON, fmt.Errorf("help")
		case "--json":
			useJSON = true
		case "--type":
			if i+1 >= len(args) {
				return opts, useJSON, fmt.Errorf("missing value for --type")
			}
			opts.Type = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return opts, useJSON, fmt.Errorf("missing value for --project")
			}
			opts.Project = args[i+1]
			hasProject = true
			i++
		case "--scope":
			if i+1 >= len(args) {
				return opts, useJSON, fmt.Errorf("missing value for --scope")
			}
			opts.Scope = args[i+1]
			i++
		case "--limit":
			if i+1 >= len(args) {
				return opts, useJSON, fmt.Errorf("missing value for --limit")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return opts, useJSON, fmt.Errorf("invalid --limit %q", args[i+1])
			}
			opts.Limit = n
			i++
		case "--all", "--all-projects":
			opts.AllProjects = true
		case "--match-mode":
			if i+1 >= len(args) {
				return opts, useJSON, fmt.Errorf("missing value for --match-mode")
			}
			mm := args[i+1]
			if mm != "all" && mm != "any" {
				return opts, useJSON, fmt.Errorf("invalid --match-mode %q: must be \"all\" or \"any\"", mm)
			}
			opts.MatchMode = mm
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return opts, useJSON, fmt.Errorf("unknown flag %q", args[i])
			}
			return opts, useJSON, fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	// Auto-detect project if not specified and not --all, mirroring search behavior.
	if !hasProject && !opts.AllProjects {
		// Attempt to detect project from cwd; if ambiguous, warn and keep searching all.
		// We reuse project.DetectProjectFull for parity with search.
		info := project.DetectProjectFull(".")
		if info.Project != "" && info.Project != "unknown" && info.Error == nil {
			opts.Project = info.Project
		}
	}
	// Clamp limit to 50 (Search also clamps, but explicit for help messaging).
	if opts.Limit > 50 {
		opts.Limit = 50
	}
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	_ = hasProject
	return opts, useJSON, nil
}

// runRecall is the shared handler for both biggz recall and biggz bigmem recent.
func runRecall(args []string) int {
	// Check for help early without opening DB.
	for _, a := range args {
		if a == "--help" || a == "-h" {
			recallHelp()
			return 0
		}
	}
	opts, useJSON, err := parseRecallArgs(args)
	if err != nil {
		if err.Error() == "help" {
			recallHelp()
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		recallHelp()
		return 1
	}
	store, err := bigmem.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open bigmem: %v\n", err)
		return 1
	}
	defer store.Close()

	results, err := store.Recent(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: recall: %v\n", err)
		return 1
	}
	if useJSON {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return 0
	}
	if len(results) == 0 {
		fmt.Println("No results.")
		return 0
	}
	for _, r := range results {
		// Human format: id [type] title (updated_at)
		updated := r.UpdatedAt.Format(time.RFC3339)
		if r.UpdatedAt.IsZero() {
			updated = r.CreatedAt.Format(time.RFC3339)
		}
		fmt.Printf("  %s [%s] %s (%s)\n", r.ID, r.Type, r.Title, updated)
	}
	return 0
}

// recallRun handles the top-level biggz recall command.
func recallRun() int {
	args := os.Args[2:]
	// If no args, just run with defaults (show recent).
	if len(args) == 0 {
		return runRecall(nil)
	}
	return runRecall(args)
}
