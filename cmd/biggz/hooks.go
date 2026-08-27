package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/hooks"
)

func hooksRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz hooks <command>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  init              Create default .biggz/hooks.yaml")
		fmt.Fprintln(os.Stderr, "  list              List configured hooks")
		fmt.Fprintln(os.Stderr, "  run <event>       Run hooks for an event")
		fmt.Fprintln(os.Stderr, "  test <event>      Dry-run hooks for an event (no execution)")
		return 1
	}

	projectRoot := detectProjectRoot()

	switch args[0] {
	case "init":
		hooksPath := filepath.Join(projectRoot, ".biggz", "hooks.yaml")
		if _, err := os.Stat(hooksPath); err == nil {
			fmt.Printf("Hook file already exists: %s\n", hooksPath)
			return 0
		}
		os.MkdirAll(filepath.Join(projectRoot, ".biggz"), 0755)
		os.WriteFile(hooksPath, []byte(hooks.DefaultHooksYAML()), 0644)
		fmt.Printf("Created: %s\n", hooksPath)

	case "list":
		mgr := hooks.NewManager(projectRoot)
		if err := mgr.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		// Can't list hooks directly without exposing config
		// Just print instructions
		fmt.Println("Hooks configured in .biggz/hooks.yaml")
		fmt.Println("Run 'biggz hooks init' to create the default file.")

	case "run":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz hooks run <event>")
			return 1
		}
		mgr := hooks.NewManager(projectRoot)
		if err := mgr.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		results := mgr.Dispatch(args[1])
		for _, r := range results.Results {
			status := "✓"
			if !r.Success {
				status = "✗"
			}
			fmt.Printf("  %s %s", status, r.Command)
			if r.Description != "" {
				fmt.Printf(" — %s", r.Description)
			}
			fmt.Println()
			if r.Output != "" {
				fmt.Printf("    Output: %s\n", strings.TrimSpace(r.Output))
			}
			if r.Error != "" {
				fmt.Printf("    Error: %s\n", r.Error)
			}
		}
		if results.Blocked {
			fmt.Println("  Hook chain blocked by error.")
			return 1
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown: hooks %s\n", args[0])
		return 1
	}
	return 0
}
