package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/backup"
	"github.com/biggs-100/biggz-ai/internal/release"
	"github.com/biggs-100/biggz-ai/internal/skillregistry"
	"github.com/biggs-100/biggz-ai/internal/tui"
)

// test injection for TUI launcher
var tuiRunWithScreen = tui.RunWithScreen

// backupRun handles the "biggz backup" subcommand.
// Usage: biggz backup create <path> [path...]
//
//	biggz backup list
//	biggz backup restore <id> <target>
func backupRun() int {
	args := os.Args[2:]
	// --tui flag parsing with precedence over subverbs and --json
	hasTUI := false
	unknownFlag := ""
	for _, a := range args {
		if a == "--tui" {
			hasTUI = true
		} else if strings.HasPrefix(a, "--") && a != "--help" && a != "-h" && a != "--tui" && a != "--json" && a != "help" {
			// Check for unknown long flags
			if a != "--help" && a != "--json" && a != "--tui" {
				unknownFlag = a
			}
		} else if strings.HasPrefix(a, "-") && a != "-h" && !strings.HasPrefix(a, "--") {
			// short flag other than -h is unknown
			unknownFlag = a
		}
	}
	// More precise unknown detection: any --* not in known set
	knownLong := map[string]bool{"--tui": true, "--help": true, "--json": true}
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			if !knownLong[a] && a != "--help" {
				// also allow --help as help trigger
				if _, ok := knownLong[a]; !ok {
					unknownFlag = a
				}
			}
		}
	}
	if unknownFlag != "" {
		fmt.Fprintf(os.Stderr, "error: unknown flag %s\n", unknownFlag)
		return 1
	}
	if hasTUI {
		if err := checkTUIInteractive(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		tuiRunWithScreen(tui.ScreenBackup)
		return 0
	}
	// Help should be shown for any --help/-h/help, including subverbs like "create --help"
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			fmt.Fprintln(os.Stderr, "Usage: biggz backup <create|list|restore> ...")
			fmt.Fprintln(os.Stderr, "  create <path> [path...]  — create a backup snapshot")
			fmt.Fprintln(os.Stderr, "  list                     — list available backups")
			fmt.Fprintln(os.Stderr, "  restore <id> <target>    — restore a backup")
			fmt.Fprintln(os.Stderr, "  --tui                    — launch interactive Bubbletea TUI")
			return 0
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: biggz backup <create|list|restore> ...")
		fmt.Fprintln(os.Stderr, "  create <path> [path...]  — create a backup snapshot")
		fmt.Fprintln(os.Stderr, "  list                     — list available backups")
		fmt.Fprintln(os.Stderr, "  restore <id> <target>    — restore a backup")
		fmt.Fprintln(os.Stderr, "  --tui                    — launch interactive Bubbletea TUI")
		return 0
	}
	switch args[0] {
	case "create":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz backup create <path> [path...]")
			return 1
		}
		b, err := backup.Create("", args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Backup created: %s\n", b.ID)
		fmt.Printf("  Size: %d bytes\n", b.Size)
		fmt.Printf("  Paths: %v\n", b.Paths)
		for _, s := range b.Skipped {
			fmt.Fprintf(os.Stderr, "warning: skipped %s\n", s)
		}
	case "list":
		backups, err := backup.List("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if len(backups) == 0 {
			fmt.Println("No backups found.")
			return 0
		}
		for _, b := range backups {
			fmt.Printf("  %s  (%d bytes, %s)\n", b.ID, b.Size, b.CreatedAt.Format("2006-01-02 15:04"))
		}
	case "restore":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: biggz backup restore <id> <target-dir>")
			return 1
		}
		if err := backup.Restore("", args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Restored %s to %s\n", args[1], args[2])
	default:
		fmt.Fprintf(os.Stderr, "unknown backup command: %s\n", args[0])
		return 1
	}
	return 0
}

// helpRun handles the "biggz help" subcommand.
func helpRun() int {
	args := os.Args[2:]
	hasTUI := false
	hasHelp := false
	unknownFlag := ""
	knownLong := map[string]bool{"--tui": true, "--help": true, "--json": true}
	for _, a := range args {
		if a == "--tui" {
			hasTUI = true
		} else if a == "--help" || a == "-h" {
			hasHelp = true
		} else if strings.HasPrefix(a, "--") {
			if !knownLong[a] {
				unknownFlag = a
			}
		} else if strings.HasPrefix(a, "-") && a != "-h" {
			unknownFlag = a
		}
	}
	if unknownFlag != "" {
		fmt.Fprintf(os.Stderr, "error: unknown flag %s\n", unknownFlag)
		return 1
	}
	if hasTUI {
		if err := checkTUIInteractive(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return 1
		}
		tuiRunWithScreen(tui.ScreenHelp)
		return 0
	}
	// Default help text (also for --help)
	fmt.Fprintln(os.Stderr, "Usage: biggz help [--tui] [--help]")
	fmt.Fprintln(os.Stderr, "  --tui    launch interactive Bubbletea TUI")
	fmt.Fprintln(os.Stderr, "  --help   show this help")
	if hasHelp || len(args) == 0 {
		return 0
	}
	// Unknown subarg shows usage
	fmt.Fprintln(os.Stderr, "unknown help argument")
	return 0
}

// releaseRun handles the "biggz release" subcommand.
// Usage: biggz release status       — show git state
//
//	biggz release tag <version> — create version tag
//	biggz release verify <version> — verify tag exists
func releaseRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, "Usage: biggz release <status|tag|verify> ...")
		fmt.Fprintln(os.Stderr, "  status              — show git state")
		fmt.Fprintln(os.Stderr, "  tag <version>       — create version tag")
		fmt.Fprintln(os.Stderr, "  verify <version>    — verify tag exists")
		return 0
	}
	switch args[0] {
	case "status":
		state, err := release.CheckGitState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Branch: %s\n", state.Branch)
		fmt.Printf("Commit: %s\n", state.Commit)
		fmt.Printf("Clean: %v\n", state.Clean)
		if state.LastTag != "" {
			fmt.Printf("Last tag: %s\n", state.LastTag)
		}
	case "tag":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz release tag <version>")
			return 1
		}
		tag, err := release.Tag(args[1], false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Created tag: %s\n", tag)
	case "verify":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz release verify <version>")
			return 1
		}
		commit, err := release.VerifyTag(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Tag %s found at commit %s\n", args[1], commit)
	default:
		fmt.Fprintf(os.Stderr, "unknown release command: %s\n", args[0])
		return 1
	}
	return 0
}

// skillRegistryRun handles the "biggz skill-registry" subcommand.
// Usage: biggz skill-registry refresh [--force] [--quiet] [--cwd <dir>] [--no-gitignore]
//
//	— regenerate skill registry
func skillRegistryRun() int {
	args := os.Args[2:]
	if len(args) < 1 || args[0] != "refresh" {
		fmt.Fprintln(os.Stderr, "Usage: biggz skill-registry refresh [--force] [--quiet] [--cwd <dir>] [--no-gitignore]")
		return 1
	}
	force := false
	quiet := false
	noGitignore := false
	cwd := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--force":
			force = true
		case "--quiet":
			quiet = true
		case "--no-gitignore":
			noGitignore = true
		case "--cwd":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --cwd requires a directory")
				return 1
			}
			i++
			cwd = args[i]
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag %s\n", args[i])
			return 1
		}
	}
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}
	_ = noGitignore
	result, err := skillregistry.Refresh(cwd, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if quiet {
		return 0
	}
	if result.Cached {
		fmt.Println("Skill registry cache valid, no regeneration needed.")
		fmt.Printf("  Path: %s\n", result.Registry)
	} else {
		fmt.Printf("Skill registry regenerated: %d skills\n", result.SkillCount)
		fmt.Printf("  Path: %s\n", result.Registry)
	}
	return 0
}
