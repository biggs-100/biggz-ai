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
	"strings"
	"time"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/orchestrator"
	"github.com/biggz-ai/biggz/plugin"
	"github.com/biggz-ai/biggz/pipeline"
	"github.com/biggz-ai/biggz/plugintest"
	"github.com/biggz-ai/biggz/policy"
	"github.com/biggz-ai/biggz/registry"
	"github.com/google/uuid"

	"github.com/biggz-ai/biggz/internal/agents/claude"
	"github.com/biggz-ai/biggz/internal/agents/opencode"
	"github.com/biggz-ai/biggz/internal/agents/qwen"
	"github.com/biggz-ai/biggz/internal/backup"
	"github.com/biggz-ai/biggz/internal/bigmem"
	"github.com/biggz-ai/biggz/internal/doctor"
	"github.com/biggz-ai/biggz/internal/assets"
	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/internal/update"
	"github.com/biggz-ai/biggz/internal/lens/readability"
	"github.com/biggz-ai/biggz/internal/lens/reliability"
	"github.com/biggz-ai/biggz/internal/lens/resilience"
	"github.com/biggz-ai/biggz/internal/lens/risk"
	"github.com/biggz-ai/biggz/internal/release"
	"github.com/biggz-ai/biggz/internal/review"
	"github.com/biggz-ai/biggz/internal/sdd"
	"github.com/biggz-ai/biggz/internal/skillregistry"
	"github.com/biggz-ai/biggz/internal/tui"
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
		case "sdd-status":
			os.Exit(sddStatusRun())
		case "sdd-verify-validate":
			os.Exit(sddVerifyValidateRun())
		case "sdd-attempt":
			os.Exit(sddAttemptRun())
		case "sdd-continue":
			os.Exit(sddContinueRun())
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
		case "review":
			os.Exit(reviewRun())
		case "doctor":
			os.Exit(doctorRun())
		case "update":
			os.Exit(updateRun())
		case "sync":
			os.Exit(syncRun())
		case "mcp":
			os.Exit(mcpRun())
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

	dummyLens := &plugintest.DummyLens{}
	if err := reg.RegisterLens(dummyLens); err != nil {
		fmt.Fprintf(os.Stderr, "error: registering lens: %v\n", err)
		os.Exit(1)
	}

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

	// Build execution graph — lenses are independent (run in PARALLEL),
	// then policy evaluation runs after all lenses complete.
	minEvEval := &minimumEvidenceEvaluator{}
	pGraph := pipeline.NewGraph()
	pGraph.AddNode(&lensStage{lens: riskLens})
	pGraph.AddNode(&lensStage{lens: readabilityLens})
	pGraph.AddNode(&lensStage{lens: reliabilityLens})
	pGraph.AddNode(&lensStage{lens: resilienceLens})
	pGraph.AddNode(&lensStage{lens: dummyLens})
	// Policy depends on all lenses
	pGraph.AddNode(&policyStage{evaluator: minEvEval},
		"lens-risk", "lens-readability", "lens-reliability",
		"lens-resilience", "lens-dummy-lens")

	// Use DAG orchestrator for parallel lens execution
	orch := orchestrator.NewWithGraph(reg, pGraph)
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
	adapters := map[string]plugin.AgentAdapter{
		"opencode": opencode.NewAdapter(),
		"qwen":     qwen.NewAdapter(),
		"claude":   claude.NewAdapter(),
	}
	priority := []string{"opencode", "claude", "qwen"}

	// Determine which adapter to use
	toTry := priority
	if selectedAgent != "" {
		if _, ok := adapters[selectedAgent]; !ok {
			fmt.Fprintf(os.Stderr, "error: unknown agent %q (supported: opencode, claude, qwen)\n", selectedAgent)
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
	adapters := map[string]plugin.AgentAdapter{
		"opencode": opencode.NewAdapter(),
		"qwen":     qwen.NewAdapter(),
		"claude":   claude.NewAdapter(),
	}
	priority := []string{"opencode", "claude", "qwen"}

	// Determine which adapters to try
	toTry := priority
	if selectedAgent != "" {
		if _, ok := adapters[selectedAgent]; !ok {
			fmt.Fprintf(os.Stderr, "error: unknown agent %q (supported: opencode, qwen)\n", selectedAgent)
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
	} else {
		fmt.Println("biggz-ai installed successfully")
		fmt.Printf("  Agent: %s\n", result.BinaryPath)
		fmt.Printf("  Skills deployed: %d\n", result.SkillsDeployed)
		fmt.Printf("  Config merged: %v\n", result.ConfigMerged)
		fmt.Printf("  Commands written: %d\n", result.CommandsWritten)
	}
	return 0
}

// sddStatusRun handles the "biggz sdd-status" subcommand.
// It scans the openspec/changes directory and reports active/archived changes.
func sddStatusRun() int {
	// Look for openspec/ relative to the current working dir
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	openspecRoot := filepath.Join(cwd, "openspec")
	if _, err := os.Stat(openspecRoot); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: no openspec/ directory found in %s\n", cwd)
		return 1
	}

	// Check RDD status
	reviewDisabled := false
	commonDir, worktreeDir := detectGitDirs()
	if commonDir != "" || worktreeDir != "" {
		rs, rddErr := review.RDDStatus(worktreeDir, commonDir)
		if rddErr == nil && rs.EffectiveMode == review.RDDModeDisabled {
			reviewDisabled = true
		}
	}

	active, archived, err := sdd.Status(openspecRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Print(sdd.FormatStatus(active, archived, sdd.StatusOptions{ReviewDisabled: reviewDisabled}))
	return 0
}

// sddVerifyValidateRun handles the "biggz sdd-verify-validate" subcommand.
// Validates a verify report against authoritative requirement/scenario counts.
// Usage: biggz sdd-verify-validate --input <path> [--requirements N] [--scenarios N]
func sddVerifyValidateRun() int {
	args := os.Args[2:]
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(os.Stderr, "Usage: biggz sdd-verify-validate --input <path> [--requirements N] [--scenarios N]")
			fmt.Fprintln(os.Stderr, "  --input <path>       — path to verify report (required)")
			fmt.Fprintln(os.Stderr, "  --requirements N     — authoritative requirement count")
			fmt.Fprintln(os.Stderr, "  --scenarios N        — authoritative scenario count")
			return 0
		}
	}

	input := ""
	req := -1
	scen := -1

	args = os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--input":
			if i+1 < len(args) {
				i++
				input = args[i]
			}
		case "--requirements":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &req)
			}
		case "--scenarios":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &scen)
			}
		}
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "error: --input is required")
		return 1
	}

	if err := sdd.ValidateVerifyReport(input, req, scen); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Println("Verify report is valid.")
	return 0
}

// sddAttemptRun handles the "biggz sdd-attempt" subcommand.
// Usage:
//   biggz sdd-attempt status <change>
//   biggz sdd-attempt begin <change> [--budget N]
//   biggz sdd-attempt finish <change> [--success] [--lines N]
//   biggz sdd-attempt reset <change>
func sddAttemptRun() int {
	args := os.Args[2:]
	if len(args) < 2 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-attempt <status|begin|finish|reset> <change>")
		fmt.Fprintln(os.Stderr, "  status  <change>            — show current attempt state")
		fmt.Fprintln(os.Stderr, "  begin   <change> [--budget N] — start new attempt")
		fmt.Fprintln(os.Stderr, "  finish  <change> [--success] [--lines N] — end attempt")
		fmt.Fprintln(os.Stderr, "  reset   <change>            — reset attempt counter")
		return 0
	}

	operation := args[0]
	change := args[1]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	openspecRoot := filepath.Join(cwd, "openspec")

	// Parse flags
	budget := 400
	success := true
	lines := 0
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--budget":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &budget)
			}
		case "--no-success":
			success = false
		case "--lines":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &lines)
			}
		}
	}

	var result *sdd.AttemptState
	switch operation {
	case "status":
		result, err = sdd.AttemptStatus(openspecRoot, change)
	case "begin":
		result, err = sdd.AttemptBegin(openspecRoot, change, budget)
	case "finish":
		result, err = sdd.AttemptFinish(openspecRoot, change, success, lines)
	case "reset":
		result, err = sdd.AttemptReset(openspecRoot, change)
	default:
		fmt.Fprintf(os.Stderr, "unknown operation %q (use: status, begin, finish, reset)\n", operation)
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("Change: %s\n", result.ChangeName)
	fmt.Printf("Status: %s\n", result.Status)
	if result.TotalAttempts > 0 {
		fmt.Printf("Attempts: %d/%d\n", result.TotalAttempts, result.MaxAttempts)
	}
	if result.ActiveAttempt > 0 {
		fmt.Printf("Active attempt: %d\n", result.ActiveAttempt)
	}
	if result.CorrectionLines > 0 {
		fmt.Printf("Correction lines: %d\n", result.CorrectionLines)
	}
	return 0
}

// sddContinueRun handles the "biggz sdd-continue" subcommand.
// Usage: biggz sdd-continue <change>
// It checks which artifacts exist and recommends the next phase.
func sddContinueRun() int {
	if len(os.Args) < 3 || os.Args[2] == "--help" || os.Args[2] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-continue <change>")
		fmt.Fprintln(os.Stderr, "  <change> — name of the SDD change to continue")
		fmt.Fprintln(os.Stderr, "Checks which artifacts exist and recommends the next phase.")
		return 0
	}
	change := os.Args[2]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	openspecRoot := filepath.Join(cwd, "openspec")

	// Check RDD status
	commonDir, worktreeDir := detectGitDirs()
	if commonDir != "" || worktreeDir != "" {
		rs, rddErr := review.RDDStatus(worktreeDir, commonDir)
		if rddErr == nil && rs.EffectiveMode == review.RDDModeDisabled {
			fmt.Println("RDD: disabled (unmanaged)")
		}
	}

	phase, err := sdd.NextPhase(openspecRoot, change)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("Change: %s\n", change)
	fmt.Printf("Next phase: %s\n", phase)
	fmt.Printf("Description: %s\n", sdd.NextPhaseDescription(phase))
	return 0
}

// bigmemRun handles the "biggz bigmem" subcommand.
// Usage: biggz bigmem save <title> <content>
//        biggz bigmem search <query>
//        biggz bigmem get <id>
func bigmemRun() int {
	store, err := bigmem.Open("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open bigmem: %v\n", err)
		return 1
	}

	args := os.Args[2:]
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: biggz bigmem <save|search|get> ...")
		fmt.Fprintln(os.Stderr, "  save <title> <type> <content>")
		fmt.Fprintln(os.Stderr, "  search <query>")
		fmt.Fprintln(os.Stderr, "  get <id>")
		return 1
	}

	switch args[0] {
	case "save":
		if len(args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: biggz bigmem save <title> <type> <content>")
			return 1
		}
		obs := &bigmem.Observation{
			Title:   args[1],
			Type:    args[2],
			Content: args[3],
		}
		if err := store.Save(obs); err != nil {
			fmt.Fprintf(os.Stderr, "error: save: %v\n", err)
			return 1
		}
		fmt.Printf("Saved: %s\n", obs.ID)

	case "search":
		results, err := store.Search(args[1], bigmem.SearchOptions{Limit: 10})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: search: %v\n", err)
			return 1
		}
		if len(results) == 0 {
			fmt.Println("No results.")
			return 0
		}
		for _, r := range results {
			fmt.Printf("  %s [%s] %s\n", r.ID[:20], r.Type, r.Title)
		}

	case "get":
		obs, err := store.Get(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: get: %v\n", err)
			return 1
		}
		fmt.Printf("ID: %s\n", obs.ID)
		fmt.Printf("Title: %s\n", obs.Title)
		fmt.Printf("Type: %s\n", obs.Type)
		fmt.Printf("Content: %s\n", obs.Content)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		return 1
	}

	return 0
}

// backupRun handles the "biggz backup" subcommand.
// Usage: biggz backup create <path> [path...]
//        biggz backup list
//        biggz backup restore <id> <target>
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
//        biggz release tag <version> — create version tag
//        biggz release verify <version> — verify tag exists
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
// Usage: biggz skill-registry refresh   — regenerate skill registry
func skillRegistryRun() int {
	args := os.Args[2:]

	if len(args) < 1 || args[0] != "refresh" {
		fmt.Fprintln(os.Stderr, "Usage: biggz skill-registry refresh")
		return 1
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	result, err := skillregistry.Refresh(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("Skill registry regenerated: %d skills\n", result.SkillCount)
	fmt.Printf("  Path: %s\n", result.Registry)
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
	case "validate":
		return reviewValidateRun()
	case "export":
		return reviewExportRun()
	case "import":
		return reviewImportRun()
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
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  gate pre-pr|pre-push <lineage>  Run publication gate")
	fmt.Fprintln(os.Stderr, "    --json                     Machine-readable JSON output")
	fmt.Fprintln(os.Stderr, "    --dry-run                  Report without failing")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  start --subject <file>         Start a new review")
	fmt.Fprintln(os.Stderr, "    [--lineage <id>]            Optional lineage ID (UUIDv7)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  resume <lineage>               Resume a review (Blocked/NeedsChanges -> InReview)")
	fmt.Fprintln(os.Stderr, "    [--force]                   Skip non-critical validations")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  validate <lineage>             Validate chain integrity")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  export <lineage>               Export review as JSON")
	fmt.Fprintln(os.Stderr, "    [--output <file>]           Write to file instead of stdout")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  import <file>                  Import a previously exported review")
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
func reviewStatusRun() int {
	args := os.Args[3:]
	useJSON := false
	var lineageID string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			useJSON = true
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
			}
		}
	}
	if lineageID == "" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review status <lineage> [--json]")
		return 1
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
	if st.Receipt != nil {
		fmt.Printf("Receipt:        %s (hash: %s)\n", "valid", st.Receipt.BindingHash[:16]+"...")
	} else {
		fmt.Printf("Receipt:        none\n")
	}
	fmt.Printf("Fix Rounds:     %d/%d\n", st.BudgetCounters.FixRounds, model.MaxFixRounds)
	fmt.Printf("Scoped Valids:  %d/%d\n", st.BudgetCounters.ScopedValidations, model.MaxScopedValidations)
	return 0
}

// reviewGateRun handles "biggz review gate pre-pr|pre-push <lineage>".
func reviewGateRun() int {
	args := os.Args[3:]
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: biggz review gate <pre-pr|pre-push> <lineage> [--json] [--dry-run]")
		return 1
	}

	gateType := args[0]
	lineageID := args[1]
	useJSON := false
	dryRun := false
	for _, a := range args[2:] {
		switch a {
		case "--json":
			useJSON = true
		case "--dry-run":
			dryRun = true
		}
	}

	if gateType != "pre-pr" && gateType != "pre-push" {
		fmt.Fprintf(os.Stderr, "error: unknown gate type %q (use: pre-pr or pre-push)\n", gateType)
		return 1
	}

	auth := review.NewAuthority("")
	chain, err := auth.LoadChain(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading chain: %v\n", err)
		return 1
	}

	commonDir, worktreeDir := detectGitDirs()
	// For gate validation, use the common dir — gates operate at clone level
	gitDir := commonDir
	if gitDir == "" {
		gitDir = worktreeDir
	}

	var result review.GateResult
	if gateType == "pre-pr" {
		result = review.PrePRGate(chain, nil, nil, dryRun, gitDir)
	} else {
		// For pre-push, try to get the current committed tree for scope comparison.
		treeOut, treeErr := exec.Command("git", "rev-parse", "HEAD:").Output()
		snapshotTree := ""
		if treeErr == nil {
			snapshotTree = strings.TrimSpace(string(treeOut))
		}
		result = review.PrePushGate(chain, nil, nil, snapshotTree, dryRun, gitDir)
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
	}
	return lastOp
}

// reviewStartRun handles "biggz review start --subject <file> [--lineage <id>]".
func reviewStartRun() int {
	args := os.Args[3:]
	var subjectFile, lineageID string
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
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review start --subject <file> [--lineage <id>]")
			return 0
		}
	}
	if subjectFile == "" {
		fmt.Fprintln(os.Stderr, "error: --subject is required")
		fmt.Fprintln(os.Stderr, "Usage: biggz review start --subject <file> [--lineage <id>]")
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

	auth := review.NewAuthority("")
	store, err := auth.Open(lineageID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening store: %v\n", err)
		return 1
	}

	r := review.New(subject)
	r.State.Role = model.RoleReviewer
	r.State.LineageID = lineageID
	r.WithStore(store)

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: starting review: %v\n", err)
		return 1
	}

	fmt.Printf("Review started: %s\n", lineageID)
	return 0
}

// reviewResumeRun handles "biggz review resume <lineage> [--force]".
func reviewResumeRun() int {
	args := os.Args[3:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review resume <lineage> [--force]")
		return 0
	}
	lineageID := args[0]
	force := false
	for _, a := range args[1:] {
		if a == "--force" {
			force = true
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
	fmt.Fprintln(os.Stderr, "  sdd-status                 Show SDD change status")
	fmt.Fprintln(os.Stderr, "  sdd-verify-validate        Validate verify reports")
	fmt.Fprintln(os.Stderr, "  sdd-attempt                Manage attempt budgets")
	fmt.Fprintln(os.Stderr, "  sdd-continue <change>      Determine next SDD phase")
	fmt.Fprintln(os.Stderr, "  bigmem save|search|get     Persistent memory")
	fmt.Fprintln(os.Stderr, "  backup create|list|restore Snapshot/restore state")
	fmt.Fprintln(os.Stderr, "  release status|tag|verify  Version management")
	fmt.Fprintln(os.Stderr, "  skill-registry refresh     Regenerate skill registry")
	fmt.Fprintln(os.Stderr, "  review list|status|gate|start|resume|validate|export|import  Review lineage commands")
	fmt.Fprintln(os.Stderr, "  doctor [--json] [--fix]   Run system health checks")
	fmt.Fprintln(os.Stderr, "  update [--dry-run]       Update biggz-ai to latest version")
	fmt.Fprintln(os.Stderr, "  sync [flags]             Deploy skills, config, prompts, and commands")
	fmt.Fprintln(os.Stderr, "  rdd enable|disable|status  RDD kill switch")
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
	explicitVersion := ""
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			dryRun = true
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
			fmt.Println(update.ReplaceHint("github.com/biggz-ai/biggz"))
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: replacing binary: %v\n", err)
		return 1
	}

	fmt.Printf("✓ Updated to %s\n", rel.TagName)
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
	fmt.Fprintln(os.Stderr, "Usage: biggz update [--dry-run] [--version <tag>]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Update biggz-ai to the latest release.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  --dry-run           Check for updates without downloading")
	fmt.Fprintln(os.Stderr, "  --version <tag>     Install a specific version (e.g., v1.0.0)")
	fmt.Fprintln(os.Stderr, "  --help, -h          Show this help message")
	fmt.Fprintln(os.Stderr, "")
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
