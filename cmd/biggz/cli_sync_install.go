package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/recoverytrace"
	"github.com/biggs-100/biggz-ai/internal/update"
	"github.com/biggs-100/biggz-ai/plugin"
)

// syncRun handles the "biggz sync" subcommand.
// It deploys skills, config, prompts, and commands to the detected AI agent.
// Supports --skills, --config, --prompts, --commands, --all, --dry-run, --agent, --home flags.
// Without category flags, deploys all categories.
func syncRun() int {
	ctx := context.Background()

	// Parse flags
	dryRun := false
	var selectedAgent, homeDir string
	skills, config, prompts, commands := false, false, false, false
	all := false
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--skills":
			skills = true
		case "--config":
			config = true
		case "--prompts":
			prompts = true
		case "--commands":
			commands = true
		case "--all":
			all = true
		case "--dry-run":
			dryRun = true
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
			printSyncHelp()
			return 0
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", args[i])
			printSyncHelp()
			return 1
		}
	}

	// If no specific category flag is set and --all is not set, default to all
	if !skills && !config && !prompts && !commands {
		all = true
	}

	// Build adapter map
	adapters := agentAdapters()
	priority := priorityAgents()

	// Determine which adapter to use
	toTry := priority
	if selectedAgent != "" {
		if _, ok := adapters[selectedAgent]; !ok {
			fmt.Fprintf(os.Stderr, "error: unknown agent %q\n", selectedAgent)
			return 1
		}
		toTry = []string{selectedAgent}
	}

	// Resolve adapter (first detected, or first in priority for sync)
	home := homeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	var adapter plugin.AgentAdapter
	for _, name := range toTry {
		a := adapters[name]
		ok, _, _, _, _ := a.Detect(ctx, home)
		if ok {
			adapter = a
			break
		}
	}
	if adapter == nil {
		// Fall back to first adapter for path resolution even if not detected
		adapter = adapters[toTry[0]]
	}

	if all {
		skills = true
		config = true
		prompts = true
		commands = true
	}

	if dryRun {
		fmt.Println("Sync dry-run:")
	}

	if skills {
		skillsDir := adapter.SkillsDir(home)
		count, err := install.DeploySkills(skillsDir, assets.FS, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: deploy skills: %v\n", err)
			return 1
		}
		if dryRun {
			fmt.Printf("  skills: %d file(s) would be deployed\n", count)
		} else {
			fmt.Printf("  skills: %d file(s) deployed\n", count)
		}
	}

	if prompts {
		promptsDir := filepath.Join(adapter.GlobalConfigDir(home), "prompts", "sdd")
		if err := install.DeployPrompts(promptsDir, assets.FS, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "error: deploy prompts: %v\n", err)
			return 1
		}
		if dryRun {
			fmt.Println("  prompts: would be deployed")
		} else {
			fmt.Println("  prompts: deployed")
		}
	}

	if config {
		settingsPath := adapter.SettingsPath(home)
		merged, err := install.DeployConfig(settingsPath, assets.FS, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: deploy config: %v\n", err)
			return 1
		}
		if dryRun {
			if merged {
				fmt.Println("  config: would be merged")
			} else {
				fmt.Println("  config: would not change")
			}
		} else {
			if merged {
				fmt.Println("  config: merged")
			} else {
				fmt.Println("  config: unchanged")
			}
		}
	}

	if commands {
		commandsDir := filepath.Join(adapter.GlobalConfigDir(home), "commands")
		count, err := install.DeployCommands(commandsDir, assets.FS, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: deploy commands: %v\n", err)
			return 1
		}
		if dryRun {
			fmt.Printf("  commands: %d file(s) would be written\n", count)
		} else {
			fmt.Printf("  commands: %d file(s) written\n", count)
		}
	}

	return 0
}

// printSyncHelp prints the sync subcommand help text.
func printSyncHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz sync [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Deploy skills, config, prompts, and commands to the AI agent.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	fmt.Fprintln(os.Stderr, "  --skills           Deploy skill files")
	fmt.Fprintln(os.Stderr, "  --config           Merge and deploy configuration")
	fmt.Fprintln(os.Stderr, "  --prompts          Deploy prompt files")
	fmt.Fprintln(os.Stderr, "  --commands         Deploy command files")
	fmt.Fprintln(os.Stderr, "  --all              Deploy all categories (default)")
	fmt.Fprintln(os.Stderr, "  --dry-run          Report what would be done without writing")
	fmt.Fprintln(os.Stderr, "  --agent <name>     Select agent (opencode, claude, qwen)")
	fmt.Fprintln(os.Stderr, "  --home <dir>       Custom home directory for testing")
	fmt.Fprintln(os.Stderr, "  --help, -h         Show this help message")
}

// installRun handles the "biggz install" subcommand.
func installRun() int {
	ctx := context.Background()

	// Parse flags
	dryRun := false
	var selectedAgent, homeDir string
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
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
		}
	}

	// Build adapter map
	adapters := agentAdapters()
	priority := priorityAgents()

	// Determine which adapters to try
	toTry := priority
	if selectedAgent != "" {
		if _, ok := adapters[selectedAgent]; !ok {
			fmt.Fprintf(os.Stderr, "error: unknown agent %q\n", selectedAgent)
			return 1
		}
		toTry = []string{selectedAgent}
	}

	cfg := install.Config{DryRun: dryRun, HomeDir: homeDir}
	var result *install.Result
	var lastErr error

	for _, name := range toTry {
		r, err := install.Run(ctx, adapters[name], cfg)
		if err == nil && r.AgentDetected {
			result = r
			break
		}
		lastErr = err
	}

	if result == nil {
		fmt.Fprintln(os.Stderr, "error: no supported AI agent detected")
		if lastErr != nil {
			fmt.Fprintf(os.Stderr, "cause: %v\n", lastErr)
		}
		fmt.Fprintln(os.Stderr, "Tried:", toTry)
		fmt.Fprintln(os.Stderr, "Install one of these agents and try again, or use --agent to select one.")
		return 1
	}

	if result.DryRun {
		fmt.Printf("Dry-run: would install biggz-ai for %q\n", result.BinaryPath)
		fmt.Printf("  Skills: %d\n", result.SkillsDeployed)
		fmt.Printf("  Config merge: %v\n", result.ConfigMerged)
		fmt.Printf("  Commands: %d\n", result.CommandsWritten)
		fmt.Printf("  Plugins: %d\n", result.PluginsDeployed)
	} else {
		fmt.Println("biggz-ai installed successfully")
		fmt.Printf("  Agent: %s\n", result.BinaryPath)
		fmt.Printf("  Skills deployed: %d\n", result.SkillsDeployed)
		fmt.Printf("  Config merged: %v\n", result.ConfigMerged)
		fmt.Printf("  Commands written: %d\n", result.CommandsWritten)
		fmt.Printf("  Plugins deployed: %d\n", result.PluginsDeployed)
	}
	return 0
}

// ---------------------------------------------------------------------------
// Update Command
// ---------------------------------------------------------------------------

// updateRun handles the "biggz update" subcommand.
// Usage: biggz update [--dry-run] [--version <tag>]
//
// It discovers the latest release matching the BIGGZ_CHANNEL env var,
// downloads the archive, verifies its checksum and minisig signature,
// extracts the binary, and replaces the current executable.
func updateRun() int {
	ctx := context.Background()

	// Parse flags
	dryRun := false
	noReconcile := false
	explicitVersion := ""
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
		case "--no-reconcile":
			noReconcile = true
		case "--version":
			if i+1 < len(args) {
				i++
				explicitVersion = args[i]
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

	// Discover the release.
	var rel *update.Release
	if explicitVersion != "" {
		var err error
		rel, err = update.GetRelease(ctx, "biggz-ai", "biggz", explicitVersion)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: getting release %s: %v\n", explicitVersion, err)
			return 1
		}
	} else {
		releases, err := update.ListReleases(ctx, "biggz-ai", "biggz")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: listing releases: %v\n", err)
			return 1
		}
		rel = update.SelectRelease(releases, ch)
		if rel == nil {
			fmt.Fprintln(os.Stderr, "error: no releases found")
			return 1
		}
	}

	// Check if already up to date.
	if explicitVersion == "" && doctor.BuildVersion != "" && doctor.BuildVersion != "dev" {
		current := strings.TrimPrefix(doctor.BuildVersion, "v")
		latest := strings.TrimPrefix(rel.TagName, "v")
		if current == latest {
			fmt.Printf("Already up to date (%s)\n", rel.TagName)
			return 0
		}
	}

	// Find assets by name.
	var archiveAsset, checksumsAsset, sigAsset *update.Asset
	archiveSuffix := ".tar.gz"
	binaryName := "biggz"
	if runtime.GOOS == "windows" {
		archiveSuffix = ".zip"
		binaryName = "biggz.exe"
	}

	for i := range rel.Assets {
		name := rel.Assets[i].Name
		switch {
		case name == "checksums.txt":
			checksumsAsset = &rel.Assets[i]
		case name == "checksums.txt.minisig":
			sigAsset = &rel.Assets[i]
		case strings.HasSuffix(name, archiveSuffix) && strings.Contains(name, runtime.GOOS):
			archiveAsset = &rel.Assets[i]
		}
	}

	if checksumsAsset == nil {
		fmt.Fprintln(os.Stderr, "error: checksums.txt not found in release assets")
		return 1
	}
	if sigAsset == nil {
		fmt.Fprintln(os.Stderr, "error: checksums.txt.minisig not found in release assets")
		return 1
	}
	if archiveAsset == nil {
		fmt.Fprintln(os.Stderr, "error: no archive found for "+runtime.GOOS+"/"+runtime.GOARCH+" in release assets")
		return 1
	}

	if dryRun {
		fmt.Printf("Update would install: %s (%s)\n", rel.TagName, archiveAsset.Name)
		fmt.Printf("Channel: %s\n", channelName(ch))
		return 0
	}

	fmt.Printf("Downloading %s for %s/%s...\n", rel.TagName, runtime.GOOS, runtime.GOARCH)

	// Download checksums.txt and signature.
	checksumsData, err := update.DownloadBytes(ctx, checksumsAsset.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: downloading checksums.txt: %v\n", err)
		return 1
	}

	sigData, err := update.DownloadBytes(ctx, sigAsset.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: downloading checksums.txt.minisig: %v\n", err)
		return 1
	}

	// Verify minisig signature over checksums.txt.
	if err := update.VerifySignature(checksumsData, sigData, update.MinissignPublicKey()); err != nil {
		fmt.Fprintf(os.Stderr, "error: signature verification failed: %v\n", err)
		return 1
	}
	fmt.Println("✓ Checksums signature verified")

	// Download archive.
	archiveData, err := update.DownloadBytes(ctx, archiveAsset.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: downloading archive: %v\n", err)
		return 1
	}

	// Verify checksum.
	if err := update.VerifyChecksum(archiveData, checksumsData); err != nil {
		fmt.Fprintf(os.Stderr, "error: checksum verification failed: %v\n", err)
		return 1
	}
	fmt.Println("✓ Archive checksum verified")

	// Extract to temp dir.
	tmpDir, err := os.MkdirTemp("", "biggz-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: creating temp dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	extractedPath, err := update.ExtractArchive(archiveData, tmpDir, binaryName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: extracting archive: %v\n", err)
		return 1
	}
	fmt.Println("✓ Binary extracted")

	// Replace current binary.
	currentPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: getting current binary path: %v\n", err)
		return 1
	}

	if err := update.ReplaceBinary(extractedPath, currentPath); err != nil {
		if err == update.ErrWindowsBinaryLock {
			fmt.Println(update.ReplaceHint("github.com/biggs-100/biggz-ai"))
			fmt.Println("The running binary was not replaced. Run 'biggz update' again after installing the new binary to reconcile managed assets.")
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: replacing binary: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Updated to %s\n", rel.TagName)

	// Re-deploy managed assets so they match the new binary. Reconcile
	// problems are a warning, never a failed update: the binary updated
	// fine and `biggz sync --all` is the manual fallback.
	home, _ := os.UserHomeDir()
	fmt.Println(postUpdateReconcile(ctx, agentAdapters(), home, noReconcile))
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

// printUpdateHelp prints the update subcommand help text.
func printUpdateHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz update [--dry-run] [--version <tag>] [--no-reconcile]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Update biggz-ai to the latest release.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  --dry-run           Check for updates without downloading")
	fmt.Fprintln(os.Stderr, "  --version <tag>     Install a specific version (e.g., v1.0.0)")
	fmt.Fprintln(os.Stderr, "  --no-reconcile      Skip re-deploying managed assets after the update")
	fmt.Fprintln(os.Stderr, "  --help, -h          Show this help message")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "After a successful update, managed assets (skills, prompts, commands,")
	fmt.Fprintln(os.Stderr, "plugins, config, MCP) are re-deployed for the detected agent.")
	fmt.Fprintln(os.Stderr, "Channel selection: set BIGGZ_CHANNEL=beta for prerelease versions")
}

// recoveryRun handles the "biggz recovery" subcommand.
func recoveryRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz recovery <command> [args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  list [--project P]         List recovery ledgers")
		fmt.Fprintln(os.Stderr, "  show <id>                   Show a recovery ledger")
		fmt.Fprintln(os.Stderr, "  generate <file> [--name N]  Generate ledger from backlog JSON + rows")
		fmt.Fprintln(os.Stderr, "  validate <file>             Validate a ledger JSON file")
		fmt.Fprintln(os.Stderr, "  export <id> [file]          Export a ledger to JSON")
		fmt.Fprintln(os.Stderr, "  import <file> [--name N]    Import a ledger from JSON")
		fmt.Fprintln(os.Stderr, "  delete <id>                 Delete a ledger")
		return 1
	}

	store, err := recoverytrace.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open recovery store: %v\n", err)
		return 1
	}
	defer store.Close()

	switch args[0] {
	case "list":
		project := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "--project" && i+1 < len(args) {
				project = args[i+1]
				i++
			}
		}
		ledgers, err := store.ListLedgers(project)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if len(ledgers) == 0 {
			fmt.Println("No recovery ledgers found.")
			return 0
		}
		for _, l := range ledgers {
			fmt.Printf("  %s  %-20s  %s  (%d rows)\n", l.ID[:min(24, len(l.ID))], l.Name, l.CreatedAt[:10], l.RowCount)
		}

	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz recovery show <id>")
			return 1
		}
		ledgers, name, project, err := store.GetLedger(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Ledger: %s (%s)\n", name, project)
		fmt.Printf("Reconciliation:\n")
		fmt.Printf("  Issues:         %d\n", ledgers.Reconciliation.Issues)
		fmt.Printf("  Pull Requests:  %d\n", ledgers.Reconciliation.PullRequests)
		fmt.Printf("  Collision PRs:  %d\n", ledgers.Reconciliation.CollisionPRs)
		fmt.Printf("  Overlaps:       %d\n", ledgers.Reconciliation.Overlaps)
		fmt.Printf("  Decompositions: %d\n", ledgers.Reconciliation.Decompositions)
		fmt.Printf("Rows: %d\n", len(ledgers.Rows))
		for _, row := range ledgers.Rows {
			fmt.Printf("  %-40s %-12s %s\n", row.Path, row.Disposition, row.Contributor)
		}

	case "generate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz recovery generate <backlog.json> [--name N]")
			return 1
		}
		name := "recovery-" + time.Now().UTC().Format("20060102")
		project := ""
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--name":
				if i+1 < len(args) {
					name = args[i+1]
					i++
				}
			case "--project":
				if i+1 < len(args) {
					project = args[i+1]
					i++
				}
			}
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", args[1], err)
			return 1
		}
		ledgers, err := recoverytrace.Generate(data, nil, recoverytrace.OverlapCounts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: generate: %v\n", err)
			return 1
		}
		id, err := store.SaveLedger(name, project, ledgers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: save: %v\n", err)
			return 1
		}
		fmt.Printf("Generated ledger: %s\n", id)

	case "validate":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz recovery validate <ledger.json>")
			return 1
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", args[1], err)
			return 1
		}
		var ledgers recoverytrace.Ledgers
		if err := json.Unmarshal(data, &ledgers); err != nil {
			fmt.Fprintf(os.Stderr, "error: parse: %v\n", err)
			return 1
		}
		expected := recoverytrace.Reconciliation{
			Issues:         ledgers.Reconciliation.Issues,
			PullRequests:   ledgers.Reconciliation.PullRequests,
			CollisionPRs:   ledgers.Reconciliation.CollisionPRs,
			Overlaps:       ledgers.Reconciliation.Overlaps,
			Decompositions: ledgers.Reconciliation.Decompositions,
		}
		if err := recoverytrace.ValidateLedgers(ledgers, expected); err != nil {
			fmt.Fprintf(os.Stderr, "validation FAILED: %v\n", err)
			return 1
		}
		fmt.Println("Validation PASSED")

	case "export":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz recovery export <id> [file]")
			return 1
		}
		filePath := fmt.Sprintf("recovery-%s.json", args[1])
		if len(args) > 2 {
			filePath = args[2]
		}
		data, err := store.ExportLedger(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if err := os.WriteFile(filePath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", filePath, err)
			return 1
		}
		fmt.Printf("Exported to %s\n", filePath)

	case "import":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz recovery import <file> [--name N] [--project P]")
			return 1
		}
		name := "imported-" + time.Now().UTC().Format("20060102")
		project := ""
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--name":
				if i+1 < len(args) {
					name = args[i+1]
					i++
				}
			case "--project":
				if i+1 < len(args) {
					project = args[i+1]
					i++
				}
			}
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", args[1], err)
			return 1
		}
		id, err := store.ImportLedger(data, name, project)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: import: %v\n", err)
			return 1
		}
		fmt.Printf("Imported ledger: %s\n", id)

	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz recovery delete <id>")
			return 1
		}
		if err := store.DeleteLedger(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Deleted: %s\n", args[1])

	default:
		fmt.Fprintf(os.Stderr, "unknown: recovery %s\n", args[0])
		return 1
	}
	return 0
}
