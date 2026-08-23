package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/orchestrator"
	"github.com/biggs-100/biggz-ai/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
	"github.com/biggs-100/biggz-ai/policy"
	"github.com/biggs-100/biggz-ai/registry"
	"github.com/google/uuid"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/backup"
	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/lens/dependencies"
	"github.com/biggs-100/biggz-ai/internal/lens/performance"
	"github.com/biggs-100/biggz-ai/internal/lens/readability"
	"github.com/biggs-100/biggz-ai/internal/lens/reliability"
	"github.com/biggs-100/biggz-ai/internal/lens/resilience"
	"github.com/biggs-100/biggz-ai/internal/lens/risk"
	"github.com/biggs-100/biggz-ai/internal/opencodeplugin"
	"github.com/biggs-100/biggz-ai/internal/recoverytrace"
	"github.com/biggs-100/biggz-ai/internal/release"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/sdd"
	"github.com/biggs-100/biggz-ai/internal/sddattempt"
	"github.com/biggs-100/biggz-ai/internal/skillregistry"
	"github.com/biggs-100/biggz-ai/internal/state"
	"github.com/biggs-100/biggz-ai/internal/tui"
	"github.com/biggs-100/biggz-ai/internal/update"
)

// ---- Pipeline Stages ----

// lensStage wraps a LensPlugin as a pipeline Stage.
type lensStage struct {
	lens plugin.LensPlugin
}

func (s *lensStage) Name() string { return "lens-" + s.lens.ID() }

func (s *lensStage) Execute(ctx context.Context, state *model.ReviewState) error {
	result, err := s.lens.Analyze(ctx, state.Subject)
	if err != nil {
		return fmt.Errorf("%s: %w", s.Name(), err)
	}
	payload, _ := json.Marshal(result)
	state.Evidence = model.AppendEvidence(state.Evidence, "lens_result", string(payload))
	return nil
}

func (s *lensStage) Rollback(ctx context.Context, state *model.ReviewState) error {
	// Pure computation — no side effects to roll back
	return nil
}

// policyStage wraps a policy.Evaluator as a pipeline Stage.
type policyStage struct {
	evaluator policy.Evaluator
}

func (s *policyStage) Name() string { return "policy-" + s.evaluator.Name() }

func (s *policyStage) Execute(ctx context.Context, state *model.ReviewState) error {
	verdict, err := s.evaluator.Evaluate(ctx, state)
	if err != nil {
		return fmt.Errorf("%s: %w", s.Name(), err)
	}
	payload, _ := json.Marshal(verdict)
	state.Evidence = model.AppendEvidence(state.Evidence, "policy_verdict", string(payload))
	return nil
}

func (s *policyStage) Rollback(ctx context.Context, state *model.ReviewState) error {
	// Pure computation — no side effects to roll back
	return nil
}

// ---- Inline Policy Evaluator ----

// minimumEvidenceEvaluator checks that at least one evidence entry exists.
type minimumEvidenceEvaluator struct{}

func (e *minimumEvidenceEvaluator) Name() string { return "minimum-evidence" }

func (e *minimumEvidenceEvaluator) Evaluate(ctx context.Context, state *model.ReviewState) (*model.PolicyVerdict, error) {
	passed := len(state.Evidence) > 0
	reason := "At least one evidence entry exists"
	severity := "info"
	if !passed {
		reason = "No evidence entries found"
		severity = "error"
	}
	return &model.PolicyVerdict{
		Policy:   e.Name(),
		Passed:   passed,
		Reason:   reason,
		Severity: severity,
	}, nil
}

// ---- CLI Entry Point ----

func main() {
	if len(os.Args) > 1 {
		// Check if first arg is a recognized subcommand
		switch os.Args[1] {
		case "install":
			os.Exit(installRun())
		case "uninstall":
			os.Exit(uninstallRun())
		case "sdd-status":
			os.Exit(sddStatusRun())
		case "sdd-apply":
			os.Exit(sddApplyRun())
		case "sdd-verify-validate":
			os.Exit(sddVerifyValidateRun())
		case "sdd-attempt":
			os.Exit(sddAttemptRun())
		case "sdd-continue":
			os.Exit(sddContinueRun())
		case "sdd-new":
			os.Exit(sddNewRun())
		case "sdd-profile":
			os.Exit(sddProfileRun())
		case "sdd-remediate":
			os.Exit(sddRemediateRun())
		case "bigmem":
			os.Exit(bigmemRun())
		case "backup":
			os.Exit(backupRun())
		case "release":
			os.Exit(releaseRun())
		case "skill-registry":
			os.Exit(skillRegistryRun())
		case "rdd":
			os.Exit(rddRun())
		case "tdd":
			os.Exit(tddRun())
		case "review":
			os.Exit(reviewRun())
		case "doctor":
			os.Exit(doctorRun())
		case "update":
			os.Exit(updateRun())
		case "sync":
			os.Exit(syncRun())
		case "plugin":
			os.Exit(pluginRun())
		case "mcp":
			os.Exit(mcpRun())
		case "pr":
			os.Exit(prCreate())
		case "export":
			os.Exit(exportRun())
		case "hooks":
			os.Exit(hooksRun())
		case "recovery":
			os.Exit(recoveryRun())
		case "version", "--version", "-v":
			v := doctor.BuildVersion
			if v == "" {
				v = "dev"
			}
			fmt.Printf("biggs-ai %s\n", v)
			return
		}
	}

	// If no recognized subcommand, check for --help
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if arg == "--help" || arg == "-h" {
				printHelp()
				return
			}
		}
	}

	// No subcommand or piped input → open TUI (unless stdin has data)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		// Interactive terminal → launch TUI
		tui.Run()
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading stdin: %v\n", err)
		os.Exit(1)
	}

	var subject model.ReviewSubject
	if err := json.Unmarshal(data, &subject); err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Build-time registry wiring
	reg := registry.New()

	riskLens := &risk.RiskLens{}
	if err := reg.RegisterLens(riskLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}

	readabilityLens := &readability.ReadabilityLens{}
	if err := reg.RegisterLens(readabilityLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}

	reliabilityLens := &reliability.ReliabilityLens{}
	if err := reg.RegisterLens(reliabilityLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}

	resilienceLens := &resilience.ResilienceLens{}
	if err := reg.RegisterLens(resilienceLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}
	performanceLens := &performance.PerformanceLens{}
	if err := reg.RegisterLens(performanceLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}
	dependenciesLens := &dependencies.DependenciesLens{}
	if err := reg.RegisterLens(dependenciesLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}

	// Build execution graph — lenses are independent (run in PARALLEL),
	// then policy evaluation runs after all lenses complete.
	minEvEval := &minimumEvidenceEvaluator{}
	pGraph := pipeline.NewGraph()
	pGraph.AddNode(&lensStage{lens: riskLens})
	pGraph.AddNode(&lensStage{lens: readabilityLens})
	pGraph.AddNode(&lensStage{lens: reliabilityLens})
	pGraph.AddNode(&lensStage{lens: resilienceLens})
	pGraph.AddNode(&lensStage{lens: performanceLens})
	pGraph.AddNode(&lensStage{lens: dependenciesLens})
	// Policy depends on all lenses
	pGraph.AddNode(&policyStage{evaluator: minEvEval},
		"lens-risk", "lens-readability", "lens-reliability",
		"lens-resilience", "lens-performance", "lens-dependencies")

	// Use DAG orchestrator for parallel lens execution
	orch := orchestrator.NewWithGraph(pGraph)
	state, err := orch.Execute(context.Background(), subject)
	if err != nil {
		// The orchestrator returns partial state on pipeline failure.
		// Output partial state + error to stderr.
		if state != nil {
			state.MerkleRoot = model.MerkleRoot(state.Evidence)
			if encErr := json.NewEncoder(os.Stdout).Encode(state); encErr != nil {
				fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", encErr)
			}
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Compute MerkleRoot from the evidence chain before output
	state.MerkleRoot = model.MerkleRoot(state.Evidence)

	if err := json.NewEncoder(os.Stdout).Encode(state); err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
		os.Exit(1)
	}
}

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

// quotePathCLI wraps a filesystem path in double quotes for copy-paste into
// a shell, mirroring internal/sdd's quotePath: fmt's %q would escape Windows
// separators and break the copied command.
func quotePathCLI(path string) string {
	return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
}

// backupRun handles the "biggz backup" subcommand.
// Usage: biggz backup create <path> [path...]
//
//	biggz backup list
//	biggz backup restore <id> <target>
func backupRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, "Usage: biggz backup <create|list|restore> ...")
		fmt.Fprintln(os.Stderr, "  create <path> [path...]  — create a backup snapshot")
		fmt.Fprintln(os.Stderr, "  list                     — list available backups")
		fmt.Fprintln(os.Stderr, "  restore <id> <target>    — restore a backup")
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
			// Accepted and ignored: biggz-ai has no EnsureATLIgnored equivalent,
			// so there is nothing a no-gitignore flag could disable. The flag is
			// accepted so the OpenCode skill-registry plugin can pass the exact
			// gentle-ai invocation shape.
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

	// --no-gitignore is an accepted no-op: biggz-ai has no EnsureATLIgnored
	// equivalent (the .atl/skill-registry.md index is never gitignored), so the
	// flag is parsed and discarded to keep the plugin invocation shape identical
	// to gentle-ai's.
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

// rddRun handles the "biggz rdd" subcommand.
// Usage: biggz rdd <enable|disable|status> [--scope worktree|clone|global] [--expected-revision <hash>]
func rddRun() int {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, "Usage: biggz rdd <enable|disable|status> [--scope worktree|clone|global] [--expected-revision <hash>]")
		fmt.Fprintln(os.Stderr, "  --scope <scope>       Scope: worktree (default), clone, or global")
		fmt.Fprintln(os.Stderr, "  --expected-revision   Verify head generation revision before writing (clone/worktree disable only)")
		return 0
	}

	op := args[0]
	scope := "worktree" // default to narrowest scope (Alan's #1973 recommendation)
	expectedRevision := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--scope" && i+1 < len(args) {
			scope = args[i+1]
			i++
		} else if args[i] == "--expected-revision" && i+1 < len(args) {
			expectedRevision = args[i+1]
			i++
		}
	}

	// Detect both git dirs: --git-common-dir (clone scope, shared by worktrees)
	// and --git-dir (worktree scope, private to this worktree)
	commonDir, worktreeDir := detectGitDirs()

	// Resolve target git dir for the chosen scope
	var targetDir string
	switch scope {
	case "worktree":
		targetDir = worktreeDir
		if targetDir == "" {
			fmt.Fprintln(os.Stderr, "error: not in a git worktree — cannot use --scope=worktree")
			return 1
		}
	case "clone":
		targetDir = commonDir
		if targetDir == "" {
			fmt.Fprintln(os.Stderr, "error: not in a git repository — cannot use --scope=clone")
			return 1
		}
	}

	var status *review.RDDStatusReport
	var err error

	switch op {
	case "status":
		status, err = review.RDDStatus(worktreeDir, commonDir)
	case "enable":
		status, err = review.RDDEnable(worktreeDir, commonDir)
	case "disable":
		// Validate expected revision before writing (CAS integrity)
		if expectedRevision != "" && targetDir != "" {
			if vErr := review.VerifyCloneRevision(targetDir, expectedRevision); vErr != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", vErr)
				return 1
			}
		}
		status, err = review.RDDDisable(worktreeDir, commonDir, scope)
	default:
		fmt.Fprintf(os.Stderr, "unknown rdd command: %s\n", op)
		return 1
	}

	if err != nil {
		// Fail closed: an unreadable kill-switch record is NOT a disabled
		// switch. The error names the exact file and the repair command.
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Print status
	fmt.Printf("RDD Status: %s\n", status.EffectiveMode)
	fmt.Printf("  Global:   %s\n", status.GlobalMode)
	if commonDir != "" {
		fmt.Printf("  Clone:    %s\n", status.CloneMode)
	}
	if worktreeDir != "" && worktreeDir != commonDir {
		fmt.Printf("  Worktree: %s\n", status.WorktreeMode)
	}
	fmt.Printf("  Source:   %s\n", status.Source)
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

// ---------------------------------------------------------------------------
// Review Commands
// ---------------------------------------------------------------------------

// reviewRun dispatches to the appropriate review subcommand.
func reviewRun() int {
	if len(os.Args) < 3 || os.Args[2] == "--help" || os.Args[2] == "-h" {
		printReviewHelp()
		return 0
	}

	switch os.Args[2] {
	case "list":
		return reviewListRun()
	case "status":
		return reviewStatusRun()
	case "gate":
		return reviewGateRun()
	case "start":
		return reviewStartRun()
	case "resume":
		return reviewResumeRun()
	case "finalize":
		return reviewFinalizeRun()
	case "capture-result":
		return reviewCaptureResultRun()
	case "refute":
		return reviewRefuteRun()
	case "validate":
		return reviewValidateRun()
	case "export":
		return reviewExportRun()
	case "import":
		return reviewImportRun()
	case "repair":
		return reviewRepairRun()
	case "recover":
		return reviewRecoverRun()
	case "reclaim":
		return reviewReclaimRun()
	case "reconcile-authority":
		return reviewReconcileAuthorityRun()
	case "dispose-result":
		return reviewDisposeResultRun()
	case "reopen-results":
		return reviewReopenResultsRun()
	case "inspect":
		return reviewInspectRun()
	case "schema":
		return reviewSchemaRun()
	case "retry-final-verification":
		return reviewRetryFinalVerificationRun()
	case "quarantine-legacy":
		return reviewQuarantineLegacyRun()
	case "invalidate":
		return reviewInvalidateRun()
	case "abandon":
		return reviewAbandonRun()
	case "bind-sdd":
		return reviewBindSDDRun()
	default:
		printReviewHelp()
		return 1
	}
}

func printReviewHelp() {
	fmt.Fprintln(os.Stderr, "Usage: biggz review <command> [args...]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  list                          List all review lineages")
	fmt.Fprintln(os.Stderr, "    --json                     Machine-readable JSON output")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  status <lineage>              Show review lineage status")
	fmt.Fprintln(os.Stderr, "    --json                     Machine-readable JSON output")
	fmt.Fprintln(os.Stderr, "    --contract <schema> --next-transition  Print ONLY the negotiated")
	fmt.Fprintln(os.Stderr, "                                  biggz-ai.review-integration/v1 routing envelope")
	fmt.Fprintln(os.Stderr, "                                  (collect/execute/stop; exit 0)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  gate post-apply|pre-commit|pre-push|pre-pr|release <lineage>  Run publication gate")
	fmt.Fprintln(os.Stderr, "    --json                     Machine-readable JSON output")
	fmt.Fprintln(os.Stderr, "    --dry-run                  Report without failing")
	fmt.Fprintln(os.Stderr, "    --base-ref <ref>           pre-pr: explicit base boundary")
	fmt.Fprintln(os.Stderr, "    --pre-pr-ci-attestation <file>  pre-pr: signed CI attestation (presence + parse, best-effort)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  start --subject <file>         Start a new review")
	fmt.Fprintln(os.Stderr, "    [--lineage <id>]            Optional lineage ID (UUIDv7)")
	fmt.Fprintln(os.Stderr, "    [--base-ref <sha>]          Base for the correction budget (default: subject commit parent, else empty tree)")
	fmt.Fprintln(os.Stderr, "    [--lenses <list>]           Selected lens slots, comma-separated (default: inferred from captured slots)")
	fmt.Fprintln(os.Stderr, "    [--consent <mode>]          Consent declaration: relay (default on a terminal), granted, or declined")
	fmt.Fprintln(os.Stderr, "                                  relay: prints the typed consent envelope and exits 0 without creating a lineage")
	fmt.Fprintln(os.Stderr, "                                  granted/declined: rerun with the human's answer for the exact candidate")
	fmt.Fprintln(os.Stderr, "                                  A start with declared lenses needs consent; with none it is silent (low risk)")
	fmt.Fprintln(os.Stderr, "    [--contract <schema>]       Negotiated mode: a medium/high candidate always relays its consent")
	fmt.Fprintln(os.Stderr, "                                  envelope (never the headless error); each choice names the exact")
	fmt.Fprintln(os.Stderr, "                                  follow-up invocation (supported: biggz-ai.review-integration/v1)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  resume <lineage>               Resume a review (Blocked/NeedsChanges -> InReview)")
	fmt.Fprintln(os.Stderr, "    [--force]                   Skip non-critical validations")
	fmt.Fprintln(os.Stderr, "    [--correction-lines <n>]    Correction forecast; must fit the frozen correction budget")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  finalize <lineage>             Finalize a fully captured review (terminal transition + persisted receipt)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha>")
	fmt.Fprintln(os.Stderr, "                                 Capture one strict reviewer result into the lineage event store")
	fmt.Fprintln(os.Stderr, "    [--repository-context <json>]  Opaque binding JSON; values must echo the flags")
	fmt.Fprintln(os.Stderr, "    [--subject-hash <sha>]         Provider-issued artifact subject hash")
	fmt.Fprintln(os.Stderr, "    --input <file>|-               Raw reviewer result JSON file or - for stdin")
	fmt.Fprintln(os.Stderr, "    [--preflight]                 Verify the binding and print the artifact subject without persisting")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  refute <lineage> --input <file>|-   Register the one read-only refuter batch")
	fmt.Fprintln(os.Stderr, "                                 Every inferential candidate-causal finding must carry a verdict")
	fmt.Fprintln(os.Stderr, "                                 (refuted|stands) in one shot before finalize")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  validate <lineage>             Validate chain integrity")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  repair <lineage>               Repair a corrupt tail event (truncates to the last valid event)")
	fmt.Fprintln(os.Stderr, "                                  Mid-chain corruption refuses and names export as the recovery path")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  recover <lineage>              Restore a LOST HEAD from the deepest fully-verified chain")
	fmt.Fprintln(os.Stderr, "                                  Intact authority is a no-op; mid-chain corruption refuses (never guesses)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  reclaim <lineage>              Move orphaned manifests/ and receipts/ artifacts to trash/<ts>/")
	fmt.Fprintln(os.Stderr, "                                  (never deleted; chain events and referenced artifacts untouched)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  reconcile-authority <lineage>  Verify BigMem mirror topics sdd/<lineage>/review/* against native state")
	fmt.Fprintln(os.Stderr, "    [--write]                   Refresh missing/stale mirrors from native state")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  dispose-result <lineage> --lens <name> --order <n> [--reason <text>]")
	fmt.Fprintln(os.Stderr, "                                 Discard a captured lens slot; re-capture is allowed afterwards")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  reopen-results <lineage>       Dispose ALL captured lens slots (bulk re-collection after a scope change)")
	fmt.Fprintln(os.Stderr, "                                  Finalize refuses until every planned slot is re-captured")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  inspect <lineage> [--json]     Inspect every event in chain order (operation, schema, size, hash)")
	fmt.Fprintln(os.Stderr, "                                  --json: full summaries; lens_result payloads are never dumped")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  schema [--event <name>]        List review event/artifact schemas, or print one schema's fields")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  retry-final-verification <lineage>")
	fmt.Fprintln(os.Stderr, "                                 Re-validate terminal state; re-materialize a missing receipt artifact")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  invalidate <lineage> <reason>  Mark the lineage invalidated; gates fail with the reason")
	fmt.Fprintln(os.Stderr, "  abandon <lineage>              Withdraw the lineage; export/import remain possible")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  quarantine-legacy              NOT implemented: biggz has no legacy quarantine store; preserved")
	fmt.Fprintln(os.Stderr, "                                 results live plugin-side at <repo>/.git/biggz/preserved-results/,")
	fmt.Fprintln(os.Stderr, "                                 outside the CLI by design")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  export <lineage>               Export review as JSON")
	fmt.Fprintln(os.Stderr, "    [--output <file>]           Write to file instead of stdout")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  import <file>                  Import a previously exported review")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  bind-sdd <change> <lineage> <revision>  Bind an approved review lineage to an SDD change")
	fmt.Fprintln(os.Stderr, "    <change>   SDD change name")
	fmt.Fprintln(os.Stderr, "    <lineage>  Review lineage ID")
	fmt.Fprintln(os.Stderr, "    <revision> Binding revision (SHA-256 hex)")
}

// reviewListRun handles "biggz review list".
func reviewListRun() int {
	useJSON := false
	for _, a := range os.Args[3:] {
		if a == "--json" {
			useJSON = true
		}
	}

	auth := review.NewAuthority("")
	lineages, err := auth.Inventory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(lineages); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
		return 0
	}

	if len(lineages) == 0 {
		fmt.Println("No review lineages found.")
		return 0
	}
	fmt.Printf("%-36s  %-20s  %s\n", "Lineage ID", "State", "Last Event")
	for _, li := range lineages {
		state := li.State
		if state == "" {
			state = "-"
		}
		last := li.LastEvent
		if last == "" {
			last = "-"
		}
		fmt.Printf("%-36s  %-20s  %s\n", li.LineageID, state, last)
	}
	return 0
}

// reviewStatusRun handles "biggz review status <lineage>".
//
// Negotiated contract mode (Phase D2): `--contract biggz-ai.review-integration/v1
// --next-transition` prints ONLY the provider-owned routing envelope and exits
// 0 — no raw status fields, nothing to interpret. The pair is mandatory: a
// half-declared request refuses rather than silently emitting a different
// shape. Unknown contract values error naming the supported ones. Without
// --contract the previous behavior is unchanged.
func reviewStatusRun() int {
	args := os.Args[3:]
	useJSON := false
	var lineageID, contract string
	nextTransition := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			useJSON = true
		case "--next-transition":
			nextTransition = true
		case "--contract":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --contract requires a value")
				return 1
			}
			i++
			contract = args[i]
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
			}
		}
	}
	if lineageID == "" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review status <lineage> [--json] [--contract <schema> --next-transition]")
		return 1
	}

	if contract != "" || nextTransition {
		if contract == "" {
			fmt.Fprintf(os.Stderr, "error: --next-transition requires --contract (supported: %s)\n", strings.Join(review.SupportedReviewContracts, ", "))
			return 1
		}
		if contract != review.ContractSchema {
			fmt.Fprintf(os.Stderr, "error: unknown review integration contract %q (supported: %s)\n", contract, strings.Join(review.SupportedReviewContracts, ", "))
			return 1
		}
		if !nextTransition {
			fmt.Fprintln(os.Stderr, "error: --contract requires --next-transition (the negotiated envelope mode)")
			return 1
		}
		env, err := review.NewAuthority("").BuildNextTransition(lineageID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(env); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
		return 0
	}

	auth := review.NewAuthority("")
	st, err := auth.Status(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(st); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Printf("Lineage ID:     %s\n", st.LineageID)
	fmt.Printf("Head Hash:      %s\n", st.HeadHash)
	fmt.Printf("Event Count:    %d\n", st.EventCount)
	fmt.Printf("Chain Valid:    %t\n", st.ChainValid)
	if st.NextTransition != nil {
		nt := st.NextTransition
		line := fmt.Sprintf("Next Transition: %s", nt.Action)
		if nt.Reason != "" {
			line += fmt.Sprintf(" (%s)", nt.Reason)
		}
		if nt.Lens != "" {
			order := "?"
			if nt.Order != nil {
				order = strconv.Itoa(*nt.Order)
			}
			line += fmt.Sprintf(" (lens: %s, order: %s)", nt.Lens, order)
		}
		if nt.BudgetRemaining > 0 {
			line += fmt.Sprintf(" (budget remaining: %d)", nt.BudgetRemaining)
		}
		if len(nt.Gates) > 0 {
			line += fmt.Sprintf(" (gates: %s)", strings.Join(nt.Gates, ", "))
		}
		fmt.Println(line)
	}
	if st.Receipt != nil {
		fmt.Printf("Receipt:        %s (hash: %s)\n", "valid", st.Receipt.BindingHash[:16]+"...")
	} else {
		fmt.Printf("Receipt:        none\n")
	}
	if st.ReceiptArtifact != nil {
		fmt.Printf("Receipt Artifact: %s (hash: %s)\n", st.ReceiptArtifact.Path, st.ReceiptArtifact.Hash)
	}
	if st.Budget != nil {
		fmt.Printf("Correction Budget: %d lines (max attempts: %d, original changed: %d)\n",
			st.Budget.CorrectionLines, st.Budget.MaxAttempts, st.Budget.OriginalChangedLines)
	}
	if st.RiskTier != "" {
		fmt.Printf("Risk Tier:      %s\n", st.RiskTier)
	}
	if len(st.LensPlan) > 0 {
		fmt.Printf("Lens Plan:      %s\n", strings.Join(st.LensPlan, ", "))
	}
	fmt.Printf("Fix Rounds:     %d/%d\n", st.BudgetCounters.FixRounds, model.MaxFixRounds)
	fmt.Printf("Scoped Valids:  %d/%d\n", st.BudgetCounters.ScopedValidations, model.MaxScopedValidations)
	if st.Refutations != nil {
		fmt.Printf("Refutations:    total %d, refuted %d, stands %d, pending %d\n",
			st.Refutations.Total, st.Refutations.Refuted, st.Refutations.Stands, st.Refutations.Pending)
	}
	return 0
}

// reviewGateRun handles "biggz review gate <post-apply|pre-commit|pre-push|pre-pr|release> <lineage>".
func reviewGateRun() int {
	args := os.Args[3:]
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: biggz review gate <post-apply|pre-commit|pre-push|pre-pr|release> <lineage> [--json] [--dry-run] [--base-ref <ref>] [--pre-pr-ci-attestation <file>]")
		return 1
	}

	gateType := review.GateKind(args[0])
	lineageID := args[1]
	useJSON := false
	var opts review.GateOptions
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--json":
			useJSON = true
		case "--dry-run":
			opts.DryRun = true
		case "--base-ref":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --base-ref requires a value")
				return 1
			}
			i++
			opts.BaseRef = args[i]
		case "--pre-pr-ci-attestation":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --pre-pr-ci-attestation requires a value")
				return 1
			}
			i++
			opts.PrePRCIAttestation = args[i]
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review gate <post-apply|pre-commit|pre-push|pre-pr|release> <lineage> [--json] [--dry-run] [--base-ref <ref>] [--pre-pr-ci-attestation <file>]")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			return 1
		}
	}

	result, err := review.EvaluateGate(gateType, "", lineageID, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintf(os.Stderr, "Gate: %s\n", result.Gate)
		fmt.Fprintf(os.Stderr, "Passed: %t\n", result.Passed)
		if result.Delivery != "" {
			fmt.Fprintf(os.Stderr, "Delivery: %s\n", result.Delivery)
		}
		if result.Reason != "" {
			fmt.Fprintf(os.Stderr, "Reason: %s\n", result.Reason)
		}
		if result.ReceiptHash != "" {
			fmt.Fprintf(os.Stderr, "Receipt: %s\n", result.ReceiptHash)
		}
		if result.Findings != nil {
			fmt.Fprintf(os.Stderr, "Findings: %d blocking, %d resolved, %d follow-up\n",
				result.Findings.Blocking, result.Findings.Resolved, result.Findings.FollowUp)
		}
		if result.DryRun {
			fmt.Fprintf(os.Stderr, "Mode: DRY RUN (exit zero regardless)\n")
		}
		if len(result.Reasons) > 0 {
			fmt.Fprintf(os.Stderr, "Reasons:\n")
			for _, r := range result.Reasons {
				fmt.Fprintf(os.Stderr, "  - %s\n", r)
			}
		}
	}

	// Exit zero for the disabled disposition (delivery follows ordinary
	// repository policy, never an approval) and under --dry-run.
	if result.Delivery == review.DeliveryDisabledUnmanaged || result.DryRun {
		return 0
	}
	if !result.Passed {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Review: start
// ---------------------------------------------------------------------------

// reviewExportData is the JSON structure for review export/import.
type reviewExportData struct {
	Schema      string          `json:"schema"`
	LineageID   string          `json:"lineage_id"`
	HeadHash    string          `json:"head_hash"`
	GenesisHash string          `json:"genesis_hash"`
	EventCount  int             `json:"event_count"`
	Events      []review.Record `json:"events"`
	Receipt     *review.Receipt `json:"receipt,omitempty"`
	Status      string          `json:"status"`
	RecordedAt  string          `json:"recorded_at"`
}

// lastOperationStatus maps a chain's last event operation to a display status.
func lastOperationStatus(lastOp string) string {
	switch lastOp {
	case "start_review", "in_review", "resume":
		return "in_review"
	case "complete_review":
		return "completed"
	case "block":
		return "blocked"
	case "invalidate":
		return "invalidated"
	case "withdraw":
		return "withdrawn"
	}
	return lastOp
}

// reviewStartRun handles "biggz review start --subject <file> [--lineage <id>]".
// The correction budget is derived from the subject's changed lines and frozen
// into the start_review event payload, alongside the content-based risk tier
// and the frozen lens plan.
//
// Consent gate (Phase D1 parity): the classifier tier decides consent. A
// low-risk candidate (documentation-only or trivial content) is silent
// structural readback; medium/high needs consent. --consent relay prints the
// typed biggz-ai.review-consent/v1 envelope and exits 0 without creating a
// lineage; the caller relays it to a human and reruns with --consent granted
// or --consent declined for the exact frozen candidate. Declined persists
// nothing. An undeclared start on a terminal falls back to relay; headless
// it errors — a review needing consent never starts silently.
//
// Negotiated contract mode (Phase D2): `--contract biggz-ai.review-integration/v1`
// turns the headless consent case into the relay envelope (never the hard
// error) and extends every choice with the exact follow-up invocation for
// that answer. --consent granted/declined keep their existing behavior.
func reviewStartRun() int {
	args := os.Args[3:]
	var subjectFile, lineageID, baseRef, lensesValue, consentValue, contract string
	interactive := terminalAttached(os.Stdout)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--subject":
			if i+1 < len(args) {
				i++
				subjectFile = args[i]
			}
		case "--lineage":
			if i+1 < len(args) {
				i++
				lineageID = args[i]
			}
		case "--base-ref":
			if i+1 < len(args) {
				i++
				baseRef = args[i]
			}
		case "--lenses":
			if i+1 < len(args) {
				i++
				lensesValue = args[i]
			}
		case "--consent":
			if i+1 < len(args) {
				i++
				consentValue = args[i]
			}
		case "--contract":
			if i+1 < len(args) {
				i++
				contract = args[i]
			}
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review start --subject <file> [--lineage <id>] [--base-ref <sha>] [--lenses <list>] [--consent relay|granted|declined] [--contract <schema>]")
			return 0
		}
	}
	if contract != "" && contract != review.ContractSchema {
		fmt.Fprintf(os.Stderr, "error: unknown review integration contract %q (supported: %s)\n", contract, strings.Join(review.SupportedReviewContracts, ", "))
		return 1
	}
	contractMode := contract == review.ContractSchema
	if subjectFile == "" {
		fmt.Fprintln(os.Stderr, "error: --subject is required")
		fmt.Fprintln(os.Stderr, "Usage: biggz review start --subject <file> [--lineage <id>] [--base-ref <sha>] [--lenses <list>] [--consent relay|granted|declined] [--contract <schema>]")
		return 1
	}

	data, err := os.ReadFile(subjectFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading subject file: %v\n", err)
		return 1
	}
	var subject model.ReviewSubject
	if err := json.Unmarshal(data, &subject); err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing subject JSON: %v\n", err)
		return 1
	}

	if lineageID == "" {
		lineageID = uuid.Must(uuid.NewV7()).String()
	}

	lenses, err := review.ParseSelectedLenses(lensesValue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Classify the subject from the same base/candidate derivation as the
	// correction budget: paths + line count → tier → frozen lens plan.
	input, err := review.DeriveRiskInput(subject.Repository, subject.CommitSHA, baseRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	tier := review.ClassifyRisk(input.Paths, input.ChangedLines, input.DiffSummary)
	planned := review.PlanLenses(tier, lenses)

	// Consent gate: nothing is persisted before this point, so a relay or
	// decline cannot create a lineage. In contract mode an undeclared consent
	// is a relay (never the headless hard error): the orchestrator always
	// receives the typed envelope.
	if contractMode && consentValue == "" {
		consentValue = string(review.ConsentModeRelay)
	}
	decision, err := review.EvaluateStartConsent(subject, lineageID, input, planned, consentValue, interactive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	switch decision.Decision {
	case "relay":
		if contractMode {
			decision.Envelope.WithFollowUpInvocations(startFollowUpBase(subjectFile, lineageID, baseRef, lensesValue))
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(decision.Envelope); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding consent envelope: %v\n", err)
			return 1
		}
		return 0
	case "declined":
		fmt.Fprintln(os.Stdout, decision.Message)
		return 0
	}

	budget, err := review.DeriveCorrectionBudget(input.ChangedLines)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	plan := review.StartEventPayload{
		Schema: review.ReviewStartEventSchema, Repository: subject.Repository,
		CommitSHA: subject.CommitSHA, BaseRef: input.BaseTree,
		OriginalChangedLines: input.ChangedLines, CorrectionBudget: budget,
		MaxCorrectionAttempts: review.MaxCompactCorrectionAttempts,
		SelectedLenses:        planned, RiskTier: string(tier), LensPlan: planned,
	}

	auth := review.NewAuthority("")
	store, err := auth.Open(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening store: %v\n", err)
		return 1
	}

	r := review.New(subject)
	r.State.Role = model.RoleReviewer
	r.State.LineageID = lineageID
	r.WithStore(store).FreezeStartPlan(plan)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: starting review: %v\n", err)
		return 1
	}

	lensLabel := "none"
	if len(planned) > 0 {
		lensLabel = strings.Join(planned, ", ")
	}
	fmt.Printf("Review started: %s (correction budget: %d lines, base %s, risk tier: %s, lenses: %s)\n",
		lineageID, budget, input.BaseTree, tier, lensLabel)
	return 0
}

// terminalAttached reports whether the given file is a terminal device.
func terminalAttached(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// startFollowUpBase renders the exact follow-up start invocation the contract
// consent envelope names for each answer: the original flags echoed (the
// frozen candidate lineage always pinned), with --consent appended by the
// envelope builder. The orchestrator runs EXACTLY the named invocation.
func startFollowUpBase(subjectFile, lineageID, baseRef, lensesValue string) string {
	parts := []string{"biggz", "review", "start"}
	if subjectFile != "" {
		parts = append(parts, "--subject", followUpShellWord(subjectFile))
	}
	if lineageID != "" {
		parts = append(parts, "--lineage", lineageID)
	}
	if baseRef != "" {
		parts = append(parts, "--base-ref", baseRef)
	}
	if lensesValue != "" {
		parts = append(parts, "--lenses", followUpShellWord(lensesValue))
	}
	return strings.Join(parts, " ")
}

// followUpShellWord quotes a follow-up value when it contains shell-significant
// characters; safe tokens are echoed verbatim so the printed invocation reads
// exactly like the flag list.
func followUpShellWord(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\"'") {
		return value
	}
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

// reviewResumeRun handles "biggz review resume <lineage> [--force]".
// The optional --correction-lines forecast is gated against the frozen
// correction budget before the resume event appends.
func reviewResumeRun() int {
	args := os.Args[3:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review resume <lineage> [--force] [--correction-lines <n>]")
		return 0
	}
	lineageID := args[0]
	force := false
	correctionLines := 0
	hasForecast := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--force":
			force = true
		case "--correction-lines":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --correction-lines requires a value")
				return 1
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: --correction-lines must be an integer, got %q\n", args[i])
				return 1
			}
			correctionLines = parsed
			hasForecast = true
		}
	}

	auth := review.NewAuthority("")
	chain, err := auth.LoadChain(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading chain: %v\n", err)
		return 1
	}
	if chain.Count == 0 {
		fmt.Fprintln(os.Stderr, "error: lineage has no events")
		return 1
	}

	lastOp := chain.Records[chain.Count-1].Operation
	prevStatus := lastOperationStatus(lastOp)

	if !force && (lastOp == "complete_review" || lastOp == "in_review" || lastOp == "resume") {
		fmt.Fprintf(os.Stderr, "error: review is not in a resumable state (last operation: %s)\n", lastOp)
		return 1
	}

	if !force {
		verdict := auth.Validate(lineageID)
		if !verdict.Valid {
			fmt.Fprintf(os.Stderr, "error: chain integrity check failed: %s\n", verdict.Reason)
			return 1
		}
	}

	if hasForecast {
		if err := review.ResumeForecastGate(chain, correctionLines); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	store, err := auth.Open(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening store: %v\n", err)
		return 1
	}

	rec := review.Record{
		Operation: "resume",
		Role:      string(model.RoleAdmin),
		Actor:     string(model.RoleAdmin),
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
	if _, err := store.Append(chain.HeadHash, rec); err != nil {
		fmt.Fprintf(os.Stderr, "error: appending resume event: %v\n", err)
		return 1
	}

	fmt.Printf("%s → in_review\n", prevStatus)
	return 0
}

// reviewFinalizeRun handles "biggz review finalize <lineage>".
func reviewFinalizeRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review finalize <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	outcome, err := review.Finalize("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if outcome.Idempotent {
		fmt.Printf("Review already finalized: %s (receipt %s, hash %s)\n",
			lineageID, outcome.ReceiptPath, outcome.ReceiptHash)
	} else {
		fmt.Printf("Review finalized: %s (receipt %s, hash %s, revision %s)\n",
			lineageID, outcome.ReceiptPath, outcome.ReceiptHash, outcome.Revision)
	}
	return 0
}

// reviewCaptureResultRun handles "biggz review capture-result".
func reviewCaptureResultRun() int {
	args := os.Args[3:]
	var lineageID, targetID, lensName, expectedRevision, repositoryContext, subjectHash, input string
	order := -1
	preflight := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--lineage":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --lineage requires a value")
				return 1
			}
			i++
			lineageID = args[i]
		case "--target":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --target requires a value")
				return 1
			}
			i++
			targetID = args[i]
		case "--lens":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --lens requires a value")
				return 1
			}
			i++
			lensName = args[i]
		case "--order":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --order requires a value")
				return 1
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: --order must be an integer, got %q\n", args[i])
				return 1
			}
			order = parsed
		case "--expected-revision":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --expected-revision requires a value")
				return 1
			}
			i++
			expectedRevision = args[i]
		case "--repository-context":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --repository-context requires a value")
				return 1
			}
			i++
			repositoryContext = args[i]
		case "--subject-hash":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --subject-hash requires a value")
				return 1
			}
			i++
			subjectHash = args[i]
		case "--input":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --input requires a value")
				return 1
			}
			i++
			input = args[i]
		case "--preflight":
			preflight = true
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha> [--repository-context <json>] [--subject-hash <sha>] --input <file>|- [--preflight]")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha> [--repository-context <json>] [--subject-hash <sha>] --input <file>|- [--preflight]")
			return 1
		}
	}

	if lineageID == "" || targetID == "" || lensName == "" || order < 0 || expectedRevision == "" {
		fmt.Fprintln(os.Stderr, "error: --lineage, --target, --lens, --order, and --expected-revision are required")
		fmt.Fprintln(os.Stderr, "Usage: biggz review capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha> [--repository-context <json>] [--subject-hash <sha>] --input <file>|- [--preflight]")
		return 1
	}
	if preflight && input != "" {
		fmt.Fprintln(os.Stderr, "error: capture-result --preflight verifies the binding only and does not accept --input")
		return 1
	}
	if !preflight && input == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required (or use --preflight)")
		fmt.Fprintln(os.Stderr, "Usage: biggz review capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha> --input <file>|- [--preflight]")
		return 1
	}

	binding := review.CaptureBinding{
		LineageID: lineageID, TargetIdentity: targetID, Lens: lensName,
		Order: order, ExpectedRevision: expectedRevision, SubjectHash: subjectHash,
	}
	if repositoryContext != "" {
		context, err := review.DecodeRepositoryContext([]byte(repositoryContext))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if err := context.Validate(binding); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		binding.Repo = context.Repo
	}

	if preflight {
		result, err := review.Preflight(binding)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
		return 0
	}

	payload, err := readReviewerResultInput(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	outcome, err := review.Capture(binding, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(outcome.Artifact); err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
		return 1
	}
	return 0
}

// reviewRefuteRun handles "biggz review refute <lineage> --input <file>|-".
// Registers the one read-only refuter batch: verdicts for every inferential
// candidate-causal finding must be supplied in one shot. The outcome JSON
// mirrors the captured-artifact surface for machine consumption.
func reviewRefuteRun() int {
	args := os.Args[3:]
	var lineageID, input string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--input":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --input requires a value")
				return 1
			}
			i++
			input = args[i]
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review refute <lineage> --input <file>|-")
			return 0
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
				continue
			}
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review refute <lineage> --input <file>|-")
			return 1
		}
	}
	if lineageID == "" || input == "" {
		fmt.Fprintln(os.Stderr, "error: <lineage> and --input are required")
		fmt.Fprintln(os.Stderr, "Usage: biggz review refute <lineage> --input <file>|-")
		return 1
	}

	payload, err := readReviewerResultInput(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	outcome, err := review.Refute("", lineageID, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(outcome); err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
		return 1
	}
	return 0
}

// readReviewerResultInput reads the reviewer result payload from a file, or
// from stdin when input is "-".
func readReviewerResultInput(input string) ([]byte, error) {
	if input == "-" {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, review.ArtifactResultLimit+1))
		if err != nil {
			return nil, fmt.Errorf("read reviewer result from stdin: %w", err)
		}
		if len(data) > review.ArtifactResultLimit {
			return nil, fmt.Errorf("read reviewer result from stdin: exceeds the native bound")
		}
		return data, nil
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("read reviewer result: %w", err)
	}
	return data, nil
}

// reviewValidateRun handles "biggz review validate <lineage>".
func reviewValidateRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review validate <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	auth := review.NewAuthority("")
	chain, err := auth.LoadChain(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading chain: %v\n", err)
		return 1
	}

	verdict := auth.Validate(lineageID)

	receiptMatch := "N/A (no events)"
	receiptOk := true
	if chain.Count > 0 {
		receipt := review.NewReceipt(chain)
		if err := receipt.Verify(chain); err != nil {
			receiptMatch = fmt.Sprintf("FAIL: %v", err)
			receiptOk = false
		} else {
			receiptMatch = "PASS"
			receiptOk = true
		}
	}

	chainStatus := "PASS"
	if !verdict.Valid {
		chainStatus = fmt.Sprintf("FAIL: %s", verdict.Reason)
	}

	scopeStatus := "N/A (validate does not check scope)"

	fmt.Println("Validation results:")
	fmt.Printf("  Chain integrity:  %s\n", chainStatus)
	fmt.Printf("  Receipt match:    %s\n", receiptMatch)
	fmt.Printf("  Scope:            %s\n", scopeStatus)

	if !verdict.Valid || !receiptOk {
		return 1
	}
	return 0
}

// reviewRepairRun handles "biggz review repair <lineage>".
// Validates the chain and repairs a corrupt tail event by truncating to the
// last valid event; mid-chain corruption refuses and names export as the
// recovery path. A healthy chain is a no-op.
func reviewRepairRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review repair <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	report, err := review.Repair("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Lineage:   %s\n", report.LineageID)
	if report.Repaired {
		fmt.Printf("Repaired:  %s\n", report.Action)
		fmt.Printf("  HEAD re-derived to: %s\n", report.HeadHash)
		fmt.Printf("  Events kept:        %d\n", report.EventCount)
		fmt.Printf("  Records truncated:  %d (removed)\n", report.Truncated)
		fmt.Printf("  Detail:             %s\n", report.Detail)
	} else {
		fmt.Printf("Repaired:  no (%s)\n", report.Detail)
	}
	return 0
}

// reviewRecoverRun handles "biggz review recover <lineage>".
// Restores a LOST HEAD from the deepest fully-verified chain; a HEAD that
// exists with an intact chain is a no-op; mid-chain corruption refuses.
func reviewRecoverRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review recover <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	report, err := review.Recover("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Lineage:   %s\n", report.LineageID)
	if report.Recovered {
		fmt.Printf("Recovered: yes (%s)\n", report.Action)
		fmt.Printf("  HEAD restored to: %s\n", report.HeadHash)
		fmt.Printf("  Events kept:      %d\n", report.EventCount)
		fmt.Printf("  Detail:           %s\n", report.Detail)
	} else {
		fmt.Printf("Recovered: no (%s)\n", report.Detail)
	}
	return 0
}

// reviewReclaimRun handles "biggz review reclaim <lineage>".
// Moves orphaned manifests/ and receipts/ artifacts to trash/<ts>/; chain
// events and referenced artifacts are untouched.
func reviewReclaimRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review reclaim <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	report, err := review.Reclaim("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Lineage:  %s\n", report.LineageID)
	if report.Reclaimed == 0 {
		fmt.Printf("Reclaimed: 0 (%s)\n", report.Detail)
		return 0
	}
	fmt.Printf("Reclaimed: %d artifact(s) moved to %s\n", report.Reclaimed, report.TrashDir)
	for _, path := range report.Paths {
		fmt.Printf("  %s\n", path)
	}
	fmt.Printf("  %s\n", report.Detail)
	return 0
}

// reviewReconcileAuthorityRun handles "biggz review reconcile-authority
// <lineage> [--write]". Read-only by default; --write refreshes missing/stale
// BigMem mirror topics from native state.
func reviewReconcileAuthorityRun() int {
	args := os.Args[3:]
	var lineageID string
	write := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--write":
			write = true
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review reconcile-authority <lineage> [--write]")
			return 0
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
				continue
			}
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review reconcile-authority <lineage> [--write]")
			return 1
		}
	}
	if lineageID == "" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review reconcile-authority <lineage> [--write]")
		return 1
	}

	report, err := review.ReconcileAuthority("", lineageID, write)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Lineage:       %s\n", report.LineageID)
	fmt.Printf("Project:       %s\n", report.Project)
	fmt.Printf("Chain valid:   %t\n", report.ChainValid)
	fmt.Printf("BigMem mirrors:\n")
	for _, topic := range report.Topics {
		line := fmt.Sprintf("  %-52s %s", topic.Topic, topic.Status)
		if topic.Detail != "" {
			line += " (" + topic.Detail + ")"
		}
		fmt.Println(line)
	}
	if write {
		fmt.Printf("Refreshed:     %d topic(s) from native state\n", report.Refreshed)
	} else {
		fmt.Printf("Refreshed:     0 (read-only; pass --write to refresh missing/stale mirrors)\n")
	}
	return 0
}

// reviewDisposeResultRun handles "biggz review dispose-result <lineage>
// --lens <name> --order <n> [--reason <text>]". Discards a captured lens slot;
// re-capture for the slot is allowed afterwards.
func reviewDisposeResultRun() int {
	args := os.Args[3:]
	var lineageID, lensName, reason string
	order := -1
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--lens":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --lens requires a value")
				return 1
			}
			i++
			lensName = args[i]
		case "--order":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --order requires a value")
				return 1
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: --order must be an integer, got %q\n", args[i])
				return 1
			}
			order = parsed
		case "--reason":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --reason requires a value")
				return 1
			}
			i++
			reason = args[i]
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review dispose-result <lineage> --lens <name> --order <n> [--reason <text>]")
			return 0
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
				continue
			}
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review dispose-result <lineage> --lens <name> --order <n> [--reason <text>]")
			return 1
		}
	}
	if lineageID == "" || lensName == "" || order < 0 {
		fmt.Fprintln(os.Stderr, "Usage: biggz review dispose-result <lineage> --lens <name> --order <n> [--reason <text>]")
		return 1
	}

	revision, err := review.DisposeResult("", lineageID, lensName, order, reason)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Lens slot disposed: %s order %d (lineage %s, revision %s)\n", lensName, order, lineageID, revision)
	if reason != "" {
		fmt.Printf("  Reason: %s\n", reason)
	}
	fmt.Println("  Re-capture the slot to supersede the disposal; finalize refuses until then.")
	return 0
}

// reviewReopenResultsRun handles "biggz review reopen-results <lineage>".
// Disposes every captured lens slot in one bulk transition.
func reviewReopenResultsRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review reopen-results <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	revision, err := review.ReopenResults("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Review reopened: %s (revision %s)\n", lineageID, revision)
	fmt.Println("  Every captured lens slot is disposed; re-capture all planned slots before finalize.")
	return 0
}

// reviewInspectRun handles "biggz review inspect <lineage> [--json]".
// Lists every event in chain order; lens_result payloads are never dumped.
func reviewInspectRun() int {
	args := os.Args[3:]
	var lineageID string
	useJSON := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			useJSON = true
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review inspect <lineage> [--json]")
			return 0
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
				continue
			}
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review inspect <lineage> [--json]")
			return 1
		}
	}
	if lineageID == "" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review inspect <lineage> [--json]")
		return 1
	}

	result, err := review.Inspect("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: encoding output: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Printf("Lineage: %s (head %s, %d event(s))\n", result.LineageID, shortHash(result.HeadHash), result.EventCount)
	fmt.Printf("%-3s %-22s %-40s %-6s %s\n", "#", "Operation", "Schema", "Size", "Revision")
	for index, event := range result.Events {
		detail := ""
		switch {
		case event.Lens != "" && event.Order != nil:
			detail = fmt.Sprintf(" lens=%s order=%d", event.Lens, *event.Order)
		case event.ReceiptPath != "":
			detail = " receipt=" + event.ReceiptPath
		case len(event.DisposedSlots) > 0:
			detail = fmt.Sprintf(" slots=%d", len(event.DisposedSlots))
		}
		fmt.Printf("%-3d %-22s %-40s %-6d %s%s\n", index+1, event.Operation, event.Schema, event.Size, shortHash(event.Revision), detail)
	}
	return 0
}

// reviewSchemaRun handles "biggz review schema [--event <name>]".
// Lists every event/artifact schema biggz understands, or prints one schema's
// documented field set.
func reviewSchemaRun() int {
	args := os.Args[3:]
	event := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--event":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --event requires a value")
				return 1
			}
			i++
			event = args[i]
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review schema [--event <name>]")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review schema [--event <name>]")
			return 1
		}
	}
	if event != "" {
		info, err := review.SchemaInfoOf(event)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("%s: %s\n", info.Name, info.SchemaID)
		fmt.Printf("  fields: %s\n", strings.Join(info.Fields, ", "))
		return 0
	}
	fmt.Println("Review event/artifact schemas:")
	for _, info := range review.SchemaList() {
		fmt.Printf("  %-18s %s\n", info.Name, info.SchemaID)
	}
	fmt.Println()
	fmt.Printf("Field sets: 'biggz review schema --event <name>' (supported: %s)\n", strings.Join(review.SchemaNames(), ", "))
	return 0
}

// reviewRetryFinalVerificationRun handles "biggz review
// retry-final-verification <lineage>". Re-validates the terminal state and
// re-materializes a missing receipt artifact from the canonical payloads.
func reviewRetryFinalVerificationRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review retry-final-verification <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	report, err := review.RetryFinalVerification("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Lineage:          %s\n", report.LineageID)
	fmt.Printf("Chain integrity:  %s\n", passFailLabel(report.ChainValid))
	if report.ReceiptReMaterialized {
		fmt.Printf("Receipt match:    %s (receipt re-materialized from canonical payloads)\n", passFailLabel(report.ReceiptMatch))
	} else {
		fmt.Printf("Receipt match:    %s\n", passFailLabel(report.ReceiptMatch))
	}
	if report.ReceiptPath != "" {
		fmt.Printf("Receipt artifact: %s (hash: %s)\n", report.ReceiptPath, report.ReceiptHash)
	}
	for _, reason := range report.Reasons {
		fmt.Printf("  - %s\n", reason)
	}
	fmt.Printf("Result:           %s\n", passFailLabel(report.Passed))
	if !report.Passed {
		return 1
	}
	return 0
}

// reviewQuarantineLegacyRun handles "biggz review quarantine-legacy".
// biggz has no legacy quarantine store by design: preserved reviewer results
// live plugin-side at <repo>/.git/biggz/preserved-results/ and are outside the
// CLI. The verb exists only to explain that.
func reviewQuarantineLegacyRun() int {
	fmt.Fprintln(os.Stderr, "error: quarantine-legacy is not implemented in biggz: there is no legacy quarantine store; preserved reviewer results live plugin-side at <repo>/.git/biggz/preserved-results/ and are outside the CLI by design")
	return 1
}

// passFailLabel renders a bool as PASS/FAIL.
func passFailLabel(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// shortHash abbreviates a revision for table output.
func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

// reviewInvalidateRun handles "biggz review invalidate <lineage> <reason>".
// Appends an invalidate event with the reason; the lineage state becomes
// invalidated and subsequent gates fail with the reason.
func reviewInvalidateRun() int {
	if len(os.Args) < 5 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review invalidate <lineage> <reason>")
		return 0
	}
	lineageID := os.Args[3]
	reason := strings.Join(os.Args[4:], " ")

	revision, err := review.Invalidate("", lineageID, reason)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Review invalidated: %s (revision %s, state invalidated)\n", lineageID, revision)
	fmt.Printf("  Subsequent gates fail with: %s\n", reason)
	return 0
}

// reviewAbandonRun handles "biggz review abandon <lineage>".
// Appends a withdraw event; the lineage state becomes withdrawn and gates
// fail. Export/import remain possible.
func reviewAbandonRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review abandon <lineage>")
		return 0
	}
	lineageID := os.Args[3]

	revision, err := review.Abandon("", lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	fmt.Printf("Review abandoned: %s (revision %s, state withdrawn)\n", lineageID, revision)
	fmt.Println("  Export/import remain possible.")
	return 0
}

// reviewExportRun handles "biggz review export <lineage> [--output <file>]".
func reviewExportRun() int {
	args := os.Args[3:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review export <lineage> [--output <file>]")
		return 0
	}
	lineageID := args[0]
	outputFile := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--output" && i+1 < len(args) {
			i++
			outputFile = args[i]
		}
	}

	auth := review.NewAuthority("")
	chain, err := auth.LoadChain(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading chain: %v\n", err)
		return 1
	}

	lastOp := ""
	lastTS := ""
	if chain.Count > 0 {
		last := chain.Records[chain.Count-1]
		lastOp = last.Operation
		lastTS = last.Timestamp
	}

	var receipt *review.Receipt
	if chain.Count > 0 && chain.Valid {
		r := review.NewReceipt(chain)
		receipt = &r
	}

	exp := reviewExportData{
		Schema:      "biggz-ai.review-export/v1",
		LineageID:   chain.LineageID,
		HeadHash:    chain.HeadHash,
		GenesisHash: chain.GenesisHash,
		EventCount:  chain.Count,
		Events:      chain.Records,
		Receipt:     receipt,
		Status:      lastOperationStatus(lastOp),
		RecordedAt:  lastTS,
	}

	data, err := json.MarshalIndent(exp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshaling export: %v\n", err)
		return 1
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing export file: %v\n", err)
			return 1
		}
		fmt.Printf("Review exported: %s\n", outputFile)
	} else {
		os.Stdout.Write(data)
		fmt.Println()
	}
	return 0
}

// reviewImportRun handles "biggz review import <file>".
func reviewImportRun() int {
	if len(os.Args) < 4 || os.Args[3] == "--help" || os.Args[3] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review import <file>")
		return 0
	}
	inputFile := os.Args[3]

	data, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading import file: %v\n", err)
		return 1
	}

	var exp reviewExportData
	if err := json.Unmarshal(data, &exp); err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing export JSON: %v\n", err)
		return 1
	}

	if exp.LineageID == "" {
		fmt.Fprintln(os.Stderr, "error: export data has no lineage_id")
		return 1
	}
	if len(exp.Events) == 0 {
		fmt.Fprintln(os.Stderr, "error: export data has no events")
		return 1
	}

	auth := review.NewAuthority("")
	existing, err := auth.LoadChain(exp.LineageID)
	if err == nil && existing.Count > 0 {
		fmt.Fprintf(os.Stderr, "error: lineage %s already exists in the store\n", exp.LineageID)
		return 1
	}

	store, err := auth.Open(exp.LineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening store: %v\n", err)
		return 1
	}

	var prevRev string
	for i, ev := range exp.Events {
		rev, err := store.Append(prevRev, ev)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: importing event %d: %v\n", i, err)
			return 1
		}
		prevRev = rev
	}

	fmt.Printf("Review imported: %s (%d events)\n", exp.LineageID, len(exp.Events))
	return 0
}

// reviewBindSDDRun handles "biggz review bind-sdd <change> <lineage> <revision>".
// Binds an approved review lineage to an SDD change so the runtime ledger
// records which review governs the change's verification.
func reviewBindSDDRun() int {
	args := os.Args[3:]
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: biggz review bind-sdd <change> <lineage> <revision>")
		fmt.Fprintln(os.Stderr, "  <change>   SDD change name")
		fmt.Fprintln(os.Stderr, "  <lineage>  Review lineage ID")
		fmt.Fprintln(os.Stderr, "  <revision> Binding revision (SHA-256 hex)")
		return 1
	}

	changeName := args[0]
	lineageID := args[1]
	bindingRev := args[2]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	scope, err := sdd.BindApprovedReview(changeName, cwd, lineageID, bindingRev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if scope == sddattempt.ScopeMachine {
		fmt.Printf("runtime ledger: machine-scoped (no git repository; stored under %s)\n", sddattempt.MachineLedgerDir())
	}

	fmt.Printf("Review %q bound to SDD change %q (revision: %s)\n", lineageID, changeName, bindingRev)
	return 0
}

// detectGitDir returns the git common dir (e.g., .git) by running
// git rev-parse --git-common-dir. Returns "" if not in a git repo.
// detectGitDirs returns (commonDir, worktreeDir) for the current directory.
//   - commonDir:  `git rev-parse --git-common-dir` — shared by all worktrees
//   - worktreeDir: `git rev-parse --git-dir` — private to this worktree
//
// For the main worktree (non-linked), both return the same path.
// For a linked worktree, they differ: commonDir is the shared .git dir,
// worktreeDir is .git/worktrees/<name>.
func detectGitDirs() (commonDir, worktreeDir string) {
	// --git-common-dir (clone scope, shared)
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err == nil {
		commonDir = strings.TrimSpace(string(out))
	}
	// --git-dir (worktree scope, private)
	out, err = exec.Command("git", "rev-parse", "--git-dir").Output()
	if err == nil {
		worktreeDir = strings.TrimSpace(string(out))
	}
	return
}

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

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
