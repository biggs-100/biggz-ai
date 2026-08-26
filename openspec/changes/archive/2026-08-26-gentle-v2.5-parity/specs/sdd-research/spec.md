# sdd-research Specification

## Purpose

Selected research is mandatory and evidence-backed. Admission is closed, evidence is auditable, and hybrid persistence requires byte-equal revisions.

## Requirements

### Requirement: Closed Capability Admission

The lane MUST accept only `biggz-ai.sdd-research-capability/v1` with classes `documentation` and `open-web`; admission MUST verify declaration and exact grant. Unknown version/class, missing declaration, Bash, generic MCP, and unnamed inherited tools MUST be denied.

#### Scenario: Supported documentation request

- GIVEN agent `claude-code` declares `documentation` with grant `WebFetch`
- WHEN admission evaluates the exact grant
- THEN it MUST record allowed and verified grants

#### Scenario: Supported open-web request

- GIVEN agent `claude-code` declares `open-web` with grants `WebSearch`+`WebFetch`
- WHEN admission evaluates
- THEN it MUST allow with both grants verified

#### Scenario: Denied Bash or generic MCP

- GIVEN a request claims Bash or generic MCP satisfies evidence
- WHEN admission runs
- THEN it MUST deny, emit no claim, and block research

#### Scenario: Unknown class or version

- GIVEN request has unknown class `open-web-extended` or version `v2`
- WHEN admission runs
- THEN it MUST deny and record denial

### Requirement: Auditable Evidence Integrity

Completed `biggz-ai.sdd-research/v1` artifacts MUST record questions, admission/grants, sources with `id,class,title,publisher,url,accessed_at,excerpt`, claim-to-source mappings, contradictions, uncertainty/freshness, and separate non-authoritative product choices. Partial or blocked outcomes MUST exclude unvalidated claims and set readiness false.

#### Scenario: Complete source-backed result

- GIVEN admission succeeded and sources answer questions
- WHEN artifact is completed with `done`
- THEN each claim MUST map to source IDs and product choices MUST remain separate

#### Scenario: Partial or blocked research

- GIVEN research is `partial` or `blocked`
- WHEN artifact is persisted
- THEN outcome MUST be explicit, unvalidated claims excluded, and readiness MUST be false

### Requirement: Hybrid Completion and Recovery

Selected research MUST persist revisioned intent, admission, outcome, and references via `biggz-ai.sdd-research/v1` and `biggz-ai.sdd-preproposal/v1`. `openspec` validates OpenSpec only, `engram` Engram only, `hybrid` requires equal revision and bytes in both, `none` MUST NOT set `proposal_ready`. Failure, missing artifact, divergence, partial, or blocked MUST retain intent and block proposal.

#### Scenario: Matching restart restores

- GIVEN both stores recover equivalent revisions with `done`
- WHEN pre-proposal recovery runs
- THEN request and evidence references MUST be restored and `proposal_ready` MAY be true

#### Scenario: Divergent restart blocks

- GIVEN recovered revisions differ or one store failed to write
- WHEN recovery runs
- THEN proposal MUST remain blocked and neither copy silently preferred

#### Scenario: One-sided hybrid write recovery

- GIVEN one hybrid write failed and pre-write intent plus canonical desired content are retained
- WHEN recovery runs
- THEN it MUST write a new positive revision to both stores and read both back for equal revision and bytes before readiness

#### Scenario: Missing recovery intent stays blocked

- GIVEN retained pre-write intent is unavailable
- WHEN hybrid recovery runs
- THEN it MUST remain blocked and require explicit re-entry without inventing state
