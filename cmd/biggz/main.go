package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/orchestrator"
	"github.com/biggs-100/biggz-ai/pipeline"
	"github.com/biggs-100/biggz-ai/plugin"
	"github.com/biggs-100/biggz-ai/policy"
	"github.com/biggs-100/biggz-ai/registry"

	"github.com/biggs-100/biggz-ai/internal/backup"
	"github.com/biggs-100/biggz-ai/internal/doctor"
	"github.com/biggs-100/biggz-ai/internal/lens/dependencies"
	"github.com/biggs-100/biggz-ai/internal/lens/performance"
	"github.com/biggs-100/biggz-ai/internal/lens/readability"
	"github.com/biggs-100/biggz-ai/internal/lens/reliability"
	"github.com/biggs-100/biggz-ai/internal/lens/resilience"
	"github.com/biggs-100/biggz-ai/internal/lens/risk"
	"github.com/biggs-100/biggz-ai/internal/release"
	"github.com/biggs-100/biggz-ai/internal/skillregistry"
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

// shortHash abbreviates a revision for table output.
func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
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

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
