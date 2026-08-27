package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/biggs-100/biggz-ai/internal/extension"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/review/lens"
	"github.com/biggs-100/biggz-ai/internal/review/lens/external"
	readability "github.com/biggs-100/biggz-ai/internal/review/lens/readability"
	"github.com/biggs-100/biggz-ai/internal/review/lens/reliability"
	"github.com/biggs-100/biggz-ai/internal/review/lens/resilience"
	"github.com/biggs-100/biggz-ai/internal/sdd"
	"github.com/biggs-100/biggz-ai/internal/sddattempt"
	"github.com/biggs-100/biggz-ai/model"
)

func init() {
	// Hybrid facade sequential lenses: R2 readability, R3 reliability, R4
	// resilience, plus ExternalLensAdapter bridging capture-result JSON.
	// Build-time Registry is last-win; unknown IDs are skipped by Ordered.
	// Sequential pipeline.Stage wiring reuses the single DeriveRiskInput
	// derivation (no per-lens diff):
	//   input, _ := review.DeriveRiskInput(repo, commit, baseRef)
	//   hunks, truncated := deriveLensHunks(repo, input) // ≤8MiB, Truncated flag
	//   lensInput := lens.NewLensInput(input, hunks, truncated, repo)
	//   stages := lensStagesForReview(lens.Ordered(review.PlanLenses(tier, declared)), lensInput)
	api := extension.New()
	readability.Register(api)
	lens.RegisterLens(&reliability.Lens{})
	lens.RegisterLens(&resilience.Lens{})
	lens.RegisterLens(&external.ExternalLensAdapter{LensID: "external"})
}

// deriveLensHunks derives hunk-bounded diff content for LensInput, capped at
// 8MiB total with Truncated flag. It reuses the single DeriveRiskInput
// derivation (no per-lens diff) and never falls back to full file reads for R4.
func deriveLensHunks(repo string, input review.RiskInput) (map[string][]byte, bool) {
	// Placeholder: in production this runs `git diff --raw -z` plus `git show` per path
	// to collect hunks, then caps via lens.NewLensInput. For wiring verification,
	// return empty map with truncated derived from input size; real hunks are
	// supplied by the caller (e.g., review start pipeline).
	return map[string][]byte{}, false
}

// buildLensInput is the single derivation entry point for all lenses:
// DeriveRiskInput → hunks ≤8MiB with Truncated → LensInput.
// No per-lens diff is performed; the frozen RiskInput is reused.
func buildLensInput(repo, commitSHA, baseRef string, hunks map[string][]byte) (lens.LensInput, error) {
	riskInput, err := review.DeriveRiskInput(repo, commitSHA, baseRef)
	if err != nil {
		return lens.LensInput{}, err
	}
	return lens.NewLensInput(riskInput, hunks, false, repo), nil
}

// lensStagesForReview adapts ordered lenses to sequential pipeline.Stages in
// PlanLenses order (risk→resilience→readability→reliability) with reverse
// rollback on failure. No graph.go/DAG is involved.
func lensStagesForReview(ordered []lens.Lens, input lens.LensInput) []lensStage {
	stages := make([]lensStage, 0, len(ordered))
	for _, l := range ordered {
		stages = append(stages, lensStage{lens.NewLensStage(l, input)})
	}
	return stages
}

// lensStage is a thin alias to avoid importing pipeline at init.
type lensStage struct {
	*lens.LensStage
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
	fmt.Fprintln(os.Stderr, "    [--agent <name>]            Review runtime: pi requires BIGGZ_PI_REVIEW_RELAY_CONTRACT=biggz-pi.review-relay/v1")
	fmt.Fprintln(os.Stderr, "                                  (compat: gentle-pi.review-relay/v1 via GENTLE_PI_REVIEW_RELAY_CONTRACT)")
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
	var lineageID, contract, agentValue string
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
		case "--agent":
			if i+1 < len(args) {
				i++
				agentValue = args[i]
			}
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review status <lineage> [--json] [--contract <schema> --next-transition] [--agent <name>]")
			return 0
		default:
			if lineageID == "" && !strings.HasPrefix(args[i], "--") {
				lineageID = args[i]
			}
		}
	}
	if lineageID == "" {
		fmt.Fprintln(os.Stderr, "Usage: biggz review status <lineage> [--json] [--contract <schema> --next-transition] [--agent <name>]")
		return 1
	}
	if agentValue == "pi" {
		if err := review.ValidatePiAgent(agentValue); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
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

	// Exit zero for the disabled/burned dispositions (delivery follows ordinary
	// repository policy, never an approval) and under --dry-run.
	if result.Delivery == review.DeliveryDisabledUnmanaged || result.Delivery == review.DeliveryBurned || result.DryRun {
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
	var subjectFile, lineageID, baseRef, lensesValue, consentValue, contract, agentValue string
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
		case "--agent":
			if i+1 < len(args) {
				i++
				agentValue = args[i]
			}
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: biggz review start --subject <file> [--lineage <id>] [--base-ref <sha>] [--lenses <list>] [--consent relay|granted|declined] [--contract <schema>] [--agent <name>]")
			return 0
		}
	}
	// Pi host relay gate: pi is eligible only while the relay handshake is
	// declared (BIGGZ_PI_REVIEW_RELAY_CONTRACT or gentle compat). This is the
	// required conjunct that can only narrow the compiled boundary, never
	// expand it. Without it, review start --agent pi refuses before any
	// repository, target, or authority work, exactly like gentle's
	// reviewImmutableRuntimeCapability for pi.
	if agentValue == "pi" {
		if err := review.ValidatePiAgent(agentValue); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
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
	var lineageID, targetID, lensName, expectedRevision, repositoryContext, subjectHash, input, agentValue string
	order := -1
	preflight := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --agent requires a value")
				return 1
			}
			i++
			agentValue = args[i]
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
			fmt.Fprintln(os.Stderr, "Usage: biggz review capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha> [--repository-context <json>] [--subject-hash <sha>] [--agent <name>] --input <file>|- [--preflight]")
			return 0
		default:
			fmt.Fprintf(os.Stderr, "error: unknown flag %q\n", args[i])
			fmt.Fprintln(os.Stderr, "Usage: biggz review capture-result --lineage <id> --target <id> --lens <name> --order <n> --expected-revision <sha> [--repository-context <json>] [--subject-hash <sha>] [--agent <name>] --input <file>|- [--preflight]")
			return 1
		}
	}

	if agentValue == "pi" {
		if err := review.ValidatePiAgent(agentValue); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		// Host relay available: the capture path could materialize via
		// PiAdapter.Review with the Go-issued opaque prompt and then submit
		// the raw bytes through the existing --input path. The current minimal
		// port keeps the existing file/stdin input path unchanged; the
		// adapter is available for future materialize/execute routing when the
		// provider prompt binding is added.
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
