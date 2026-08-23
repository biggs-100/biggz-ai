package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/update"
)

// discoverRelease deduplicates the ListReleases/GetRelease/SelectRelease block.
// It returns the release for explicitVersion when given, otherwise the latest
// release matching the channel.
func discoverRelease(ctx context.Context, ch update.Channel, explicitVersion string) (*update.Release, error) {
	if explicitVersion != "" {
		rel, err := update.GetRelease(ctx, "biggz-ai", "biggz", explicitVersion)
		if err != nil {
			return nil, fmt.Errorf("getting release %s: %w", explicitVersion, err)
		}
		return rel, nil
	}
	releases, err := update.ListReleases(ctx, "biggz-ai", "biggz")
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}
	rel := update.SelectRelease(releases, ch)
	if rel == nil {
		return nil, fmt.Errorf("no releases found")
	}
	return rel, nil
}

// updateRun handles the "biggz update" check-only subcommand.
// Usage: biggz update [--version <tag>] [--help]
//
// It discovers the latest release matching BIGGZ_CHANNEL and reports whether
// an upgrade is available without downloading or mutating any files.
func updateRun() int {
	ctx := context.Background()

	// Parse flags: check-only has minimal surface. Keep --version as query.
	explicitVersion := ""
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version":
			if i+1 < len(args) {
				i++
				explicitVersion = args[i]
			} else {
				fmt.Fprintln(os.Stderr, "error: --version requires a tag")
				printUpdateHelp()
				return 1
			}
		case "--help", "-h":
			printUpdateHelp()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
			printUpdateHelp()
			return 1
		}
	}

	ch := update.ParseChannel()

	rel, err := discoverRelease(ctx, ch, explicitVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Explicit version query: report if that tag would be available.
	if explicitVersion != "" {
		current := strings.TrimPrefix(doctor.BuildVersion, "v")
		requested := strings.TrimPrefix(rel.TagName, "v")
		if doctor.BuildVersion != "" && doctor.BuildVersion != "dev" && current == requested {
			fmt.Printf("Already up to date (%s)\n", rel.TagName)
			return 0
		}
		cv := doctor.BuildVersion
		if cv == "" {
			cv = "dev"
		}
		fmt.Printf("Update available: %s (current: %s)\n", rel.TagName, cv)
		fmt.Println("Run 'biggz upgrade' to install")
		return 0
	}

	// Normal channel check.
	if doctor.BuildVersion != "" && doctor.BuildVersion != "dev" {
		current := strings.TrimPrefix(doctor.BuildVersion, "v")
		latest := strings.TrimPrefix(rel.TagName, "v")
		if current == latest {
			fmt.Printf("Already up to date (%s)\n", rel.TagName)
			return 0
		}
	}

	cv := doctor.BuildVersion
	if cv == "" {
		cv = "dev"
	}
	fmt.Printf("Update available: %s (current: %s)\n", rel.TagName, cv)
	fmt.Println("Run 'biggz upgrade' to install")
	return 0
}

// channelName returns the human-readable name of a channel.
func channelName(ch update.Channel) string {
	switch ch {
	case update.ChannelBeta:
		return "beta"
	default:
		return "stable"
	}
}

// printUpdateHelp prints the update (check-only) subcommand help text.
func printUpdateHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz update [--version <tag>]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Check for available updates without downloading.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  --version <tag>     Check a specific version (e.g., v1.0.0)")
	fmt.Fprintln(os.Stderr, "  --help, -h          Show this help message")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Prints whether an upgrade is available and hints to run")
	fmt.Fprintln(os.Stderr, "'biggz upgrade' to install. Does not download or modify files.")
	fmt.Fprintln(os.Stderr, "Channel selection: set BIGGZ_CHANNEL=beta for prerelease versions")
}

// printUpgradeHelp prints the upgrade subcommand help text.
func printUpgradeHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz upgrade [--dry-run] [--version <tag>] [--no-reconcile] [--no-backup]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Upgrade biggz-ai to the latest release.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  --dry-run           Check for updates without downloading")
	fmt.Fprintln(os.Stderr, "  --version <tag>     Install a specific version (e.g., v1.0.0)")
	fmt.Fprintln(os.Stderr, "  --no-reconcile      Skip re-deploying managed assets after the upgrade")
	fmt.Fprintln(os.Stderr, "  --no-backup         Alias for --no-reconcile (skip pre-upgrade snapshot)")
	fmt.Fprintln(os.Stderr, "  --help, -h          Show this help message")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "After a successful upgrade, managed assets (skills, prompts, commands,")
	fmt.Fprintln(os.Stderr, "plugins, config, MCP) are re-deployed for the detected agent.")
	fmt.Fprintln(os.Stderr, "Channel selection: set BIGGZ_CHANNEL=beta for prerelease versions")
}
