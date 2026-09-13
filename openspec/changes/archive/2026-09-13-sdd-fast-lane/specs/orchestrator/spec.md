# Delta for orchestrator

## ADDED Requirements

### Requirement: Fast Lane Runs Inside sdd-ff

The fast lane MUST run inside the existing `sdd-ff` meta-command (`internal/assets/prompts/sdd/sdd-ff.md`) with no new meta-command: one merged `plan.md` artifact replaces proposal, specs, design, and tasks, and phase depth MUST follow the artifact set present. Gates MUST NOT be skipped in the lane — the RDD delivery receipt (`internal/sdd/status.go:942`), `biggz sdd-verify-validate` (`cmd/biggz/cli_sdd.go:351`), the PR workload guard (`internal/sdd/workload_guard.go:121`), the session-summary guard (`internal/sdd/session_guard.go:415`), and edit authority (`internal/sdd/edit_authority.go:407`) all keep applying. The workflow rule currently written as "Never skip phases" MUST become "never skip gates" (`internal/assets/biggz/biggz-orchestrator-workflow.md:338`). `sdd-route` MUST stay advisory: the lane is inferred from the artifact present, and no verdict is persisted or read for routing (`internal/sdd/route.go:93`).

#### Scenario: Lane depth follows artifacts

- GIVEN `sdd-ff` starts on a change whose only planning artifact is `plan.md`
- WHEN phase depth resolves
- THEN it MUST proceed to apply, verify, and archive without demanding four artifacts

#### Scenario: Gates apply in the lane

- GIVEN a lane change reaching a terminal gate
- WHEN gates evaluate
- THEN all five guards MUST apply exactly as in the full pipeline

#### Scenario: Workflow wording amended

- GIVEN `internal/assets/biggz/biggz-orchestrator-workflow.md:338` read
- WHEN searched
- THEN it MUST say never skip gates and MUST NOT retain "Never skip phases"

#### Scenario: Route stays advisory

- GIVEN `sdd-route` evaluates a lane-sized change
- WHEN routing runs
- THEN no verdict MUST be persisted or read to select the lane
