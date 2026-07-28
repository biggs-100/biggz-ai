package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/orchestrator"
	"github.com/biggz-ai/biggz/plugin"
	"github.com/biggz-ai/biggz/pipeline"
	"github.com/biggz-ai/biggz/plugintest"
	"github.com/biggz-ai/biggz/policy"
	"github.com/biggz-ai/biggz/registry"

	"github.com/biggz-ai/biggz/internal/agents/claude"
	"github.com/biggz-ai/biggz/internal/agents/opencode"
	"github.com/biggz-ai/biggz/internal/agents/qwen"
	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/internal/lens/readability"
	"github.com/biggz-ai/biggz/internal/lens/reliability"
	"github.com/biggz-ai/biggz/internal/lens/resilience"
	"github.com/biggz-ai/biggz/internal/lens/risk"
	"github.com/biggz-ai/biggz/internal/sdd"
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
	}
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

	// Pipeline stages — lenses run in order, then policy evaluation
	minEvEval := &minimumEvidenceEvaluator{}
	stages := []pipeline.Stage{
		&lensStage{lens: riskLens},
		&lensStage{lens: readabilityLens},
		&lensStage{lens: reliabilityLens},
		&lensStage{lens: resilienceLens},
		&lensStage{lens: dummyLens},
		&policyStage{evaluator: minEvEval},
	}

	orch := orchestrator.New(reg, stages...)
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

// installRun handles the "biggz install" subcommand.
// It supports --dry-run and --agent flags.
// Without --agent, tries each adapter in priority order and uses the first detected.
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

	for _, name := range toTry {
		r, err := install.Run(ctx, adapters[name], cfg)
		if err == nil && r.AgentDetected {
			result = r
			break
		}
	}

	if result == nil {
		fmt.Fprintln(os.Stderr, "error: no supported AI agent detected")
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

	active, archived, err := sdd.Status(openspecRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Print(sdd.FormatStatus(active, archived))
	return 0
}

// sddVerifyValidateRun handles the "biggz sdd-verify-validate" subcommand.
// Validates a verify report against authoritative requirement/scenario counts.
// Usage: biggz sdd-verify-validate --input <path> [--requirements N] [--scenarios N]
func sddVerifyValidateRun() int {
	input := ""
	req := -1
	scen := -1

	args := os.Args[2:]
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
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-attempt <status|begin|finish|reset> <change>")
		return 1
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
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-continue <change>")
		return 1
	}
	change := os.Args[2]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	openspecRoot := filepath.Join(cwd, "openspec")

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
