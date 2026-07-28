package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/biggz-ai/biggz/internal/agents/opencode"
	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/internal/lens/readability"
	"github.com/biggz-ai/biggz/internal/lens/reliability"
	"github.com/biggz-ai/biggz/internal/lens/resilience"
	"github.com/biggz-ai/biggz/internal/lens/risk"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/orchestrator"
	"github.com/biggz-ai/biggz/plugin"
	"github.com/biggz-ai/biggz/pipeline"
	"github.com/biggz-ai/biggz/plugintest"
	"github.com/biggz-ai/biggz/policy"
	"github.com/biggz-ai/biggz/registry"
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
	if len(os.Args) > 1 && os.Args[1] == "install" {
		os.Exit(installRun())
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
// It detects the OpenCode agent, deploys skills, merges config, and writes
// command files. Returns 0 on success, 1 on failure.
func installRun() int {
	ctx := context.Background()
	adapter := opencode.NewAdapter()

	dryRun := false
	for _, arg := range os.Args[2:] {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	result, err := install.Run(ctx, adapter, install.Config{DryRun: dryRun})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if result.DryRun {
		fmt.Println("Dry-run: would install biggz-ai for OpenCode")
		fmt.Printf("  Skills: %d\n", result.SkillsDeployed)
		fmt.Printf("  Config merge: %v\n", result.ConfigMerged)
		fmt.Printf("  Commands: %d\n", result.CommandsWritten)
	} else {
		fmt.Println("biggz-ai installed successfully")
		fmt.Printf("  Agent: %s (%s)\n", "opencode", result.BinaryPath)
		fmt.Printf("  Skills deployed: %d\n", result.SkillsDeployed)
		fmt.Printf("  Config merged: %v\n", result.ConfigMerged)
		fmt.Printf("  Commands written: %d\n", result.CommandsWritten)
	}
	return 0
}
