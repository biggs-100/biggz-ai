package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/orchestrator"
	"github.com/biggs-100/biggz-ai/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
	"github.com/biggs-100/biggz-ai/policy"

	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/lens/dependencies"
	"github.com/biggs-100/biggz-ai/internal/lens/performance"
	"github.com/biggs-100/biggz-ai/internal/lens/readability"
	"github.com/biggs-100/biggz-ai/internal/lens/reliability"
	"github.com/biggs-100/biggz-ai/internal/lens/resilience"
	"github.com/biggs-100/biggz-ai/internal/lens/risk"
	"github.com/biggs-100/biggz-ai/internal/tui"
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
			fmt.Printf("biggz-ai %s\n", v)
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

	riskLens := &risk.RiskLens{}
	readabilityLens := &readability.ReadabilityLens{}
	reliabilityLens := &reliability.ReliabilityLens{}
	resilienceLens := &resilience.ResilienceLens{}
	performanceLens := &performance.PerformanceLens{}
	dependenciesLens := &dependencies.DependenciesLens{}

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
