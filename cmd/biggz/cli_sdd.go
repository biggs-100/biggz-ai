package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/internal/sdd"
	"github.com/biggs-100/biggz-ai/internal/sddattempt"
	"github.com/biggs-100/biggz-ai/internal/sddprofiles"
)

// sddStatusRun handles the "biggz sdd-status" subcommand.
// It scans the openspec/changes directory and reports active/archived changes.
// Usage: biggz sdd-status [--cwd <dir>] [--json] [--instructions]
//
//	--json emits the sdd.Status payload (active + archived + review_disabled)
//	as JSON, consumed by the SDD phase failure handoff
//	(biggz-ai.sdd-task-result-failure/v1 continuation command).
//	--instructions adds the phaseInstructions block to every derived change.
func sddStatusRun() int {
	// Look for openspec/ relative to the current working dir
	args := os.Args[2:]
	emitJSON := false
	includeInstructions := false
	cwd := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			emitJSON = true
		case "--instructions":
			includeInstructions = true
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

	var err error
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
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

	active, archived, err := sdd.StatusWithOptions(openspecRoot, sdd.StatusOptions{
		ReviewDisabled:      reviewDisabled,
		IncludeInstructions: includeInstructions,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if emitJSON {
		payload := struct {
			Active         []sdd.ChangeStatus `json:"active"`
			Archived       []sdd.ChangeStatus `json:"archived"`
			ReviewDisabled bool               `json:"review_disabled"`
		}{Active: active, Archived: archived, ReviewDisabled: reviewDisabled}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Print(sdd.FormatStatus(active, archived, sdd.StatusOptions{ReviewDisabled: reviewDisabled}))
	return 0
}

// sddApplyRun handles the "biggz sdd-apply" subcommand.
// Usage: biggz sdd-apply <change>
// It is a GUARD/validation verb, not the apply phase itself: it validates
// edit authority for the change (the workspace root plus the roots the
// runtime ledger grants for this change's instance, against the repository
// roots the task plan targets) and reports allow/block. On a block it
// renders the same blocked(edit_authority_missing) reason and the typed
// consent envelope's granted invocation that sdd-status prints (mirroring
// formatOne) and exits non-zero, so an apply actor can relay the envelope
// and rerun the exact grant invocation before any edit happens.
func sddApplyRun() int {
	args := os.Args[2:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-apply <change>")
		fmt.Fprintln(os.Stderr, "  <change> — name of the SDD change whose edit authority to validate")
		fmt.Fprintln(os.Stderr, "Validates edit authority for apply and reports allow/block. On")
		fmt.Fprintln(os.Stderr, "blocked(edit_authority_missing) it prints the consent envelope and exits 1.")
		return 0
	}
	change := args[0]

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

	active, archived, err := sdd.StatusWithOptions(openspecRoot, sdd.StatusOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	var cs *sdd.ChangeStatus
	for i := range active {
		if active[i].Name == change {
			cs = &active[i]
			break
		}
	}
	if cs == nil {
		for i := range archived {
			if archived[i].Name == change {
				cs = &archived[i]
				break
			}
		}
	}
	if cs == nil {
		fmt.Fprintf(os.Stderr, "error: no such SDD change %q in %s\n", change, filepath.Join(openspecRoot, "changes"))
		return 1
	}

	if cs.EditAuthorityBlocked {
		fmt.Println(sddApplyBlockedReason(cs.MissingRoots))
		if cs.Consent != nil && len(cs.Consent.Choices) > 0 {
			fmt.Printf("consent grant: %s\n", cs.Consent.Choices[0].Invocation)
		}
		return 1
	}
	allowed := append([]string{cwd}, cs.GrantedRoots...)
	fmt.Printf("edit authority OK — allowed roots: %s\n", strings.Join(allowed, ", "))
	return 0
}

// sddApplyBlockedReason renders the blocked(edit_authority_missing) reason
// line byte-identically to internal/sdd's editAuthorityBlockedReason, which
// is what sdd-status prints (formatOne) for the same blocked change. It is a
// deliberate copy: the guard lives in cmd/biggz and must not modify
// internal/sdd, and the blocked-status CLI tests in both layers pin the
// string.
func sddApplyBlockedReason(roots []string) string {
	quoted := make([]string, 0, len(roots))
	for _, root := range roots {
		quoted = append(quoted, quotePathCLI(root))
	}
	return fmt.Sprintf(
		"blocked(edit_authority_missing): tasks.md targets repositories outside the authorized edit roots: %s; edit tasks.md so every work unit stays inside the authorized edit roots, or grant this change edit authority for those repositories",
		strings.Join(quoted, ", "),
	)
}

// sddVerifyValidateRun handles the "biggz sdd-verify-validate" subcommand.
// Validates a verify report against authoritative requirement/scenario counts.
// Usage: biggz sdd-verify-validate --input <path|-> [--requirements N --scenarios N] [--json]
func sddVerifyValidateRun() int {
	return runSDDVerifyValidate(os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
}

// runSDDVerifyValidate is the testable core of sddVerifyValidateRun.
//
// Admission rules (Phase C1 parity):
//   - --input accepts a file path or "-" for stdin; the input is capped at
//     1 MiB.
//   - --requirements and --scenarios must be provided together; when both
//     are provided they are authoritative and a report whose counts differ
//     is denied with a named reason. Lenient mode (no count comparison)
//     applies ONLY when both are omitted.
//   - --json emits the biggz-ai.verify-admission/v1 envelope; otherwise the
//     human-readable verdict is printed.
func runSDDVerifyValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	hasHelp := false
	input := ""
	declaredReq, declaredScen := -1, -1
	reqSet, scenSet := false, false
	emitJSON := false

	parseErr := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			hasHelp = true
		case "--input":
			if i+1 >= len(args) {
				parseErr = "--input requires a value"
				break
			}
			i++
			input = args[i]
		case "--requirements":
			if i+1 >= len(args) {
				parseErr = "--requirements requires a value"
				break
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				parseErr = fmt.Sprintf("invalid --requirements value %q", args[i])
				break
			}
			declaredReq, reqSet = n, true
		case "--scenarios":
			if i+1 >= len(args) {
				parseErr = "--scenarios requires a value"
				break
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				parseErr = fmt.Sprintf("invalid --scenarios value %q", args[i])
				break
			}
			declaredScen, scenSet = n, true
		case "--json":
			emitJSON = true
		default:
			parseErr = fmt.Sprintf("unknown flag %q", args[i])
		}
	}

	if hasHelp {
		fmt.Fprintln(stderr, "Usage: biggz sdd-verify-validate --input <path|-> [--requirements N --scenarios N] [--json]")
		fmt.Fprintln(stderr, "  --input <path|->    — path to verify report, or - for stdin (required)")
		fmt.Fprintln(stderr, "  --requirements N    — authoritative requirement count (must be given with --scenarios)")
		fmt.Fprintln(stderr, "  --scenarios N       — authoritative scenario count (must be given with --requirements)")
		fmt.Fprintln(stderr, "  --json              — emit the biggz-ai.verify-admission/v1 envelope")
		return 0
	}
	if parseErr != "" {
		fmt.Fprintf(stderr, "error: %s\n", parseErr)
		return 1
	}
	if input == "" {
		fmt.Fprintln(stderr, "error: --input is required")
		return 1
	}
	if reqSet != scenSet {
		fmt.Fprintln(stderr, "error: --requirements and --scenarios must be provided together")
		return 1
	}
	if reqSet && (declaredReq < 0 || declaredScen < 0) {
		fmt.Fprintln(stderr, "error: requirement and scenario counts must be nonnegative")
		return 1
	}

	var reader io.Reader = stdin
	if input != "-" {
		file, err := os.Open(input)
		if err != nil {
			fmt.Fprintf(stderr, "error: read verify report: %v\n", err)
			return 1
		}
		defer file.Close()
		reader = file
	}

	payload, err := io.ReadAll(io.LimitReader(reader, sdd.MaxVerifyReportBytes+1))
	if err != nil {
		fmt.Fprintf(stderr, "error: read verify report: %v\n", err)
		return 1
	}
	if len(payload) > sdd.MaxVerifyReportBytes {
		fmt.Fprintf(stderr, "error: verify report exceeds %d-byte limit (1 MiB)\n", sdd.MaxVerifyReportBytes)
		return 1
	}

	admission := sdd.ValidateVerifyReportAdmission(payload, declaredReq, declaredScen)

	if emitJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(admission); err != nil {
			fmt.Fprintf(stderr, "error: encoding output: %v\n", err)
			return 1
		}
		if admission.Decision == "denied" {
			return 1
		}
		return 0
	}

	if admission.Decision == "denied" {
		fmt.Fprintf(stderr, "error: %s\n", admission.Reason)
		return 1
	}
	fmt.Fprintln(stdout, "Verify report is valid.")
	return 0
}

// sddAttemptRun handles the "biggz sdd-attempt" subcommand.
// Native runtime ledger with CAS revision tracking.
func sddAttemptRun() int {
	args := os.Args[2:]
	if len(args) < 2 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(os.Stderr, sddattempt.HelpText)
		return 0
	}

	operation := args[0]
	change := args[1]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// Parse common flags
	expectedRev := ""
	objectiveID := ""
	workUnit := ""
	evidenceGoal := ""
	evidenceRev := ""
	bindingRev := ""
	bindingLineage := ""
	maxAttempts := 3
	maxLines := 400
	outcome := ""
	diagnosis := ""
	reason := ""
	resetBy := ""
	requestID := ""
	harnessDisp := ""
	cleanupEv := ""
	processEv := ""
	remediatesEv := ""
	actor := ""
	changeInstance := ""
	var roots []string

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--expected-revision":
			if i+1 < len(args) {
				i++
				expectedRev = args[i]
			}
		case "--root":
			if i+1 < len(args) {
				i++
				// The consent envelope quotes paths for copy-paste into a
				// shell; a shell strips the quotes, but an in-process rerun
				// of the invocation passes them through, so tolerate them
				// here (mirrors gentle-ai's CLI quote trimming).
				roots = append(roots, strings.Trim(strings.TrimSpace(args[i]), `"`))
			}
		case "--change-instance":
			if i+1 < len(args) {
				i++
				changeInstance = args[i]
			}
		case "--actor":
			if i+1 < len(args) {
				i++
				actor = args[i]
			}
		case "--expected-binding-revision":
			if i+1 < len(args) {
				i++
				bindingRev = args[i]
			}
		case "--successor-lineage":
			if i+1 < len(args) {
				i++
				bindingLineage = args[i]
			}
		case "--objective-id":
			if i+1 < len(args) {
				i++
				objectiveID = args[i]
			}
		case "--work-unit":
			if i+1 < len(args) {
				i++
				workUnit = args[i]
			}
		case "--evidence-goal":
			if i+1 < len(args) {
				i++
				evidenceGoal = args[i]
			}
		case "--evidence-revision":
			if i+1 < len(args) {
				i++
				evidenceRev = args[i]
			}
		case "--remediates-evidence-revision":
			if i+1 < len(args) {
				i++
				remediatesEv = args[i]
			}
		case "--outcome":
			if i+1 < len(args) {
				i++
				outcome = args[i]
			}
		case "--diagnosis":
			if i+1 < len(args) {
				i++
				diagnosis = args[i]
			}
		case "--reason":
			if i+1 < len(args) {
				i++
				reason = args[i]
			}
		case "--reset-by":
			if i+1 < len(args) {
				i++
				resetBy = args[i]
			}
		case "--request-id":
			if i+1 < len(args) {
				i++
				requestID = args[i]
			}
		case "--harness-disposition":
			if i+1 < len(args) {
				i++
				harnessDisp = args[i]
			}
		case "--cleanup-evidence":
			if i+1 < len(args) {
				i++
				cleanupEv = args[i]
			}
		case "--process-evidence":
			if i+1 < len(args) {
				i++
				processEv = args[i]
			}
		case "--max-attempts":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &maxAttempts)
			}
		case "--max-lines":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &maxLines)
			}
		}
	}

	switch operation {
	case "status":
		// An optional --change-instance scopes the granted-roots projection
		// to that instance; without it, status projects no granted roots at
		// all, which is the conservative containment for readers that have
		// not declared which change instance they serve.
		var status *sddattempt.RuntimeStatus
		var err error
		if changeInstance != "" {
			status, err = sddattempt.StatusWithInstance(change, cwd, changeInstance)
		} else {
			status, err = sddattempt.Status(change, cwd)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if status.Scope == sddattempt.ScopeMachine {
			fmt.Printf("runtime ledger: machine-scoped (no git repository; stored under %s)\n", sddattempt.MachineLedgerDir())
		}
		if status.Migrated {
			fmt.Println("Runtime ledger migrated from the legacy home-dir store (kept untouched).")
		}
		fmt.Printf("Change:           %s\n", status.ChangeName)
		fmt.Printf("Revision:         %s\n", status.Revision)
		fmt.Printf("Next action:      %s\n", status.NextAction)
		fmt.Printf("Active attempt:   %d\n", status.ActiveAttempt)
		fmt.Printf("Attempts:         %d\n", status.AttemptCount)
		fmt.Printf("Decision needed:  %v\n", status.DecisionRequired)
		fmt.Printf("Complete:         %v\n", status.Complete)
		if len(status.GrantedRoots) > 0 {
			fmt.Printf("Granted roots:    %s\n", strings.Join(status.GrantedRoots, ", "))
		}
		if status.BindingRevision != "" {
			fmt.Printf("Binding revision: %s\n", status.BindingRevision)
		}
		if status.BindingLineage != "" {
			fmt.Printf("Binding lineage:  %s\n", status.BindingLineage)
		}

	case "grant":
		var missing []string
		if len(roots) == 0 {
			missing = append(missing, "--root")
		}
		if changeInstance == "" {
			missing = append(missing, "--change-instance")
		}
		if requestID == "" {
			missing = append(missing, "--request-id")
		}
		if actor == "" {
			missing = append(missing, "--actor")
		}
		if reason == "" {
			missing = append(missing, "--reason")
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "error: sdd-attempt grant requires %s; rerun `biggz sdd-attempt grant` with those missing flags\n", strings.Join(missing, ", "))
			return 1
		}
		result, err := sddattempt.Grant(sddattempt.GrantParams{
			ChangeName:     change,
			RepoRoot:       cwd,
			ExpectedRev:    expectedRev,
			Roots:          roots,
			Reason:         reason,
			Actor:          actor,
			RequestID:      requestID,
			ChangeInstance: changeInstance,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}

	case "begin":
		result, err := sddattempt.Begin(sddattempt.BeginParams{
			ChangeName:   change,
			RepoRoot:     cwd,
			ExpectedRev:  expectedRev,
			ObjectiveID:  objectiveID,
			WorkUnit:     workUnit,
			EvidenceGoal: evidenceGoal,
			MaxAttempts:  maxAttempts,
			MaxLines:     maxLines,
			RequestID:    requestID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if result.Scope == sddattempt.ScopeMachine {
			fmt.Printf("runtime ledger: machine-scoped (no git repository; stored under %s)\n", sddattempt.MachineLedgerDir())
		}
		if result.Migrated {
			fmt.Println("Runtime ledger migrated from the legacy home-dir store (kept untouched).")
		}
		if result.AlreadyActive {
			fmt.Printf("Already active: attempt %d is still running\n", result.ActiveAttempt)
		} else {
			fmt.Printf("Attempt %d started (revision: %s)\n", result.ActiveAttempt, result.Revision)
		}

	case "finish":
		result, err := sddattempt.Finish(sddattempt.FinishParams{
			ChangeName:                 change,
			RepoRoot:                   cwd,
			ExpectedRev:                expectedRev,
			Outcome:                    outcome,
			EvidenceRevision:           evidenceRev,
			Diagnosis:                  diagnosis,
			HarnessDisposition:         harnessDisp,
			CleanupEvidence:            cleanupEv,
			ProcessEvidence:            processEv,
			ExpectedBindingRevision:    bindingRev,
			SuccessorLineageID:         bindingLineage,
			RemediatesEvidenceRevision: remediatesEv,
			RequestID:                  requestID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if result.Scope == sddattempt.ScopeMachine {
			fmt.Printf("runtime ledger: machine-scoped (no git repository; stored under %s)\n", sddattempt.MachineLedgerDir())
		}
		if result.Migrated {
			fmt.Println("Runtime ledger migrated from the legacy home-dir store (kept untouched).")
		}
		fmt.Printf("Attempt finished (revision: %s)\n", result.Revision)
		if result.Complete {
			fmt.Println("Status: COMPLETE")
		} else if result.DecisionRequired {
			fmt.Println("Status: DECISION REQUIRED")
		} else {
			fmt.Printf("Remaining attempts: %d\n", result.RemainingAttempts)
		}

	case "reset":
		if reason == "" {
			fmt.Fprintln(os.Stderr, "error: --reason is required for reset")
			return 1
		}
		result, err := sddattempt.Reset(sddattempt.ResetParams{
			ChangeName:  change,
			RepoRoot:    cwd,
			ExpectedRev: expectedRev,
			Reason:      reason,
			ResetBy:     resetBy,
			MaxAttempts: maxAttempts,
			MaxLines:    maxLines,
			ObjectiveID: objectiveID,
			RequestID:   requestID,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if result.Scope == sddattempt.ScopeMachine {
			fmt.Printf("runtime ledger: machine-scoped (no git repository; stored under %s)\n", sddattempt.MachineLedgerDir())
		}
		if result.Migrated {
			fmt.Println("Runtime ledger migrated from the legacy home-dir store (kept untouched).")
		}
		fmt.Printf("Ledger reset (revision: %s)\n", result.Revision)
		if result.NewStore {
			fmt.Println("New store created")
		} else {
			fmt.Printf("Previous attempts cleared: %d\n", result.AttemptsReset)
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown operation %q (use: status, begin, finish, reset, grant)\n", operation)
		return 1
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

// sddProfileRun handles the "biggz sdd-profile" subcommand.
// Usage: biggz sdd-profile list
//
//	biggz sdd-profile apply <name>
//	biggz sdd-profile remove <name>
func sddProfileRun() int {
	args := os.Args[2:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(os.Stderr, sddprofiles.ListProfiles())
		return 0
	}

	switch args[0] {
	case "list":
		fmt.Print(sddprofiles.ListProfiles())

	case "apply":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz sdd-profile apply <name>")
			fmt.Fprintln(os.Stderr, "Run 'biggz sdd-profile list' to see available profiles.")
			return 1
		}
		profileName := args[1]

		// Find profile
		var profile *sddprofiles.Profile
		for _, p := range sddprofiles.DefaultProfiles() {
			if p.Name == profileName {
				profile = &p
				break
			}
		}
		if profile == nil {
			fmt.Fprintf(os.Stderr, "unknown profile %q. Run 'biggz sdd-profile list' to see available profiles.\n", profileName)
			return 1
		}

		// Find settings file
		home, _ := os.UserHomeDir()
		settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
		if _, err := os.Stat(settingsPath); err != nil {
			// Try opencode.jsonc
			settingsPath = filepath.Join(home, ".config", "opencode", "opencode.jsonc")
			if _, err := os.Stat(settingsPath); err != nil {
				fmt.Fprintln(os.Stderr, "error: could not find opencode config (opencode.json or opencode.jsonc)")
				return 1
			}
		}

		if err := sddprofiles.ApplyProfile(settingsPath, *profile); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Profile %q applied. Agents added:\n", profileName)
		for _, phase := range sddprofiles.PhaseOrder {
			if model, ok := profile.Agents[phase]; ok {
				fmt.Printf("  %s → %s\n", phase, model)
			}
		}
		fmt.Printf("\nConfig: %s\n", settingsPath)

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: biggz sdd-profile remove <name>")
			return 1
		}
		home, _ := os.UserHomeDir()
		settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
		if _, err := os.Stat(settingsPath); err != nil {
			settingsPath = filepath.Join(home, ".config", "opencode", "opencode.jsonc")
			if _, err := os.Stat(settingsPath); err != nil {
				fmt.Fprintln(os.Stderr, "error: could not find opencode config")
				return 1
			}
		}
		if err := sddprofiles.RemoveProfile(settingsPath, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Printf("Profile %q removed.\n", args[1])

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q (use: list, apply, remove)\n", args[0])
		return 1
	}
	return 0
}

// sddRemediateRun handles the "biggz sdd-remediate" subcommand.
// Usage: biggz sdd-remediate <change> [--verify-report <path>]
func sddRemediateRun() int {
	args := os.Args[2:]
	if len(args) < 1 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "Usage: biggz sdd-remediate <change> [--verify-report <path>]")
		fmt.Fprintln(os.Stderr, "  Validate remediation result for a verify failure.")
		fmt.Fprintln(os.Stderr, "  --verify-report <path>     Path to the remediation result file")
		return 1
	}

	change := args[0]
	reportPath := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--verify-report" && i+1 < len(args) {
			reportPath = args[i+1]
			i++
		}
	}

	if reportPath == "" {
		fmt.Fprintln(os.Stderr, "error: --verify-report is required")
		return 1
	}

	result, err := sdd.ValidateRemediationResult(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: remediation validation failed: %v\n", err)
		return 1
	}

	fmt.Printf("Change: %s\n", change)
	fmt.Printf("Schema: %s\n", result.Schema)
	fmt.Printf("Verdict: %s\n", result.Verdict)
	if result.Blockers > 0 {
		fmt.Printf("Blockers: %d\n", result.Blockers)
	}
	fmt.Println("Remediation validation: PASSED")
	return 0
}
