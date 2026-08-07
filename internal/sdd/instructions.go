package sdd

import "fmt"

// renderPhaseInstructions builds the static biggz phase instruction blocks
// with the change name and dependency state interpolated. Apply, verify, and
// remediate carry the native runtime begin/finish block naming biggz's
// sdd-attempt verbs; remediation additionally names the
// --remediates-evidence-revision finish flag that binds the charged
// correction to the exact failed evidence revision. Archive is the
// terminal guidance: verify-report.md exists and every task checkbox is
// complete.
func renderPhaseInstructions(cs ChangeStatus) PhaseInstructions {
	change := cs.Name
	workspace := cs.ActionContext.WorkspaceRoot
	applyInstructions := []string{
		fmt.Sprintf("Change: %s", change),
		fmt.Sprintf("State: %s", cs.Dependencies.Apply),
		"Read proposal, specs, design, and tasks before editing.",
		"Implement only unchecked tasks and update tasks.md checkboxes as work completes.",
	}
	verifyInstructions := []string{
		fmt.Sprintf("Change: %s", change),
		fmt.Sprintf("State: %s", cs.Dependencies.Verify),
		"Verify implementation against proposal, specs, design, and task completion.",
		"Run final verification only after every task is complete; apply-progress never makes final verification ready.",
	}
	remediateInstructions := []string{
		fmt.Sprintf("Change: %s", change),
		"Remediation is allowed only when the persisted verify evidence is bound.",
		"Bind focused tests, runtime harness evidence, and rollback evidence to the exact failed evidence revision.",
		"A passing bound remediation MUST finish atomically with --remediates-evidence-revision so the charged evidence shares one native HEAD CAS.",
		"A fresh independent verification is required before archive.",
	}
	archiveInstructions := []string{
		fmt.Sprintf("Change: %s", change),
		fmt.Sprintf("State: %s", cs.Dependencies.Archive),
		"Archive only when verify-report.md exists and every task checkbox is complete.",
	}
	return PhaseInstructions{
		Apply:     append(applyInstructions, runtimeInstructions(workspace, change)...),
		Verify:    append(verifyInstructions, runtimeInstructions(workspace, change)...),
		Remediate: append(remediateInstructions, runtimeRemediateInstructions(workspace, change)...),
		Archive:   archiveInstructions,
	}
}

// runtimeInstructions renders the native runtime launch block for ordinary
// apply and verify work.
func runtimeInstructions(workspace, change string) []string {
	return []string{
		fmt.Sprintf("Before any runtime-bearing apply, verify, or remediation launch, run `biggz sdd-attempt begin --cwd %s --change %q --request-id \"<id>\" --work-unit \"<label>\" --evidence-goal \"<goal>\" --max-attempts <n> --max-changed-lines <n>`.", quotePath(workspace), change),
		fmt.Sprintf("After the external run, call `biggz sdd-attempt finish --cwd %s --change %q --request-id \"<id>\" --outcome <passed|failed|interrupted> --evidence-revision <sha256> --diagnosis \"<d>\"`.", quotePath(workspace), change),
		"Reset is exceptional, requires an explicit maintainer scope decision, and is never automatic.",
	}
}

// runtimeRemediateInstructions renders the remediation variant: the finish
// call must declare --remediates-evidence-revision so the passing bound
// correction binds to the exact failed evidence revision.
func runtimeRemediateInstructions(workspace, change string) []string {
	return []string{
		fmt.Sprintf("Before any runtime-bearing apply, verify, or remediation launch, run `biggz sdd-attempt begin --cwd %s --change %q --request-id \"<id>\" --work-unit \"<label>\" --evidence-goal \"<goal>\" --max-attempts <n> --max-changed-lines <n>`.", quotePath(workspace), change),
		fmt.Sprintf("After the external run, call `biggz sdd-attempt finish --cwd %s --change %q --request-id \"<id>\" --outcome <passed|failed|interrupted> --evidence-revision <sha256> --diagnosis \"<d>\" --remediates-evidence-revision <sha256>`.", quotePath(workspace), change),
		"Reset is exceptional, requires an explicit maintainer scope decision, and is never automatic.",
	}
}
