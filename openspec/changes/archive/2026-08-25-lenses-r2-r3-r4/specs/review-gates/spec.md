# Delta for review-gates

## ADDED Requirements

### Requirement: Lens Findings as Candidate-Causal Gate Inputs

Lens findings from `internal/review/lens/...` MUST be treated as candidate-causal gate inputs. Heuristic findings MUST default to `inferential`; only `deterministic` findings with concrete `ProofRefs` (e.g., `go/parser` failure) MAY block `pre-pr` as hard failures. `inferential` findings MUST surface as warnings and MUST NOT block release without human confirmation.

#### Scenario: Inferential finding does not block pre-pr

- GIVEN a `pre-pr` gate with one `inferential` R4 resilience finding
- WHEN `biggz review gate pre-pr` runs
- THEN gate MUST exit zero with warning listing the finding
- AND receipt MUST remain valid

#### Scenario: Deterministic R2 finding blocks pre-pr

- GIVEN a `pre-pr` gate with one `determinative` R2 finding (`go/parser` failure with ProofRef)
- WHEN `biggz review gate pre-pr` runs
- THEN gate MUST exit non-zero and list the blocking finding
- AND `--json` MUST include `pass:false` with lens ID

#### Scenario: Scope change still blocks pre-push

- GIVEN an inferential-only pre-pr pass but scope hash changed post-gate
- WHEN `biggz review gate pre-push` runs
- THEN gate MUST exit non-zero reporting scope delta regardless of lens class

## MODIFIED Requirements

### Requirement: Pre-PR Gate

The system MUST implement `biggz review gate pre-pr` that blocks PR creation when either: the review has unresolved deterministic findings (including lens deterministic findings), or the receipt is invalid. Inferential lens findings MUST NOT block by themselves. The gate MUST exit non-zero when blocked.
(Previously: blocked on any unresolved findings without lens evidence-class distinction)

#### Scenario: Happy path — gate passes

- GIVEN an Approved review with a valid receipt and zero unresolved deterministic findings (only inferential lens warnings)
- WHEN `biggz review gate pre-pr` runs
- THEN the gate MUST exit zero and report pass with warnings if any

#### Scenario: Gate blocks on deterministic lens finding

- GIVEN a review with status NeedsChanges and an open deterministic R2 finding
- WHEN `biggz review gate pre-pr` runs
- THEN the gate MUST exit non-zero and list each blocking deterministic finding

#### Scenario: Gate blocks on invalid receipt

- GIVEN a review with a tampered chain (receipt invalid)
- WHEN `biggz review gate pre-pr` runs
- THEN the gate MUST exit non-zero and report receipt validation failure

### Requirement: Gate Result Reporting

Every gate MUST return a structured result with: pass/fail status, list of blocking reasons (if failed), lens finding breakdown by `inferential|deterministic` and `ProofRefs`, and dry-run indicator. Human-readable output MUST go to stderr; structured JSON via `--json` MUST include lens findings array. Gates MUST not duplicate `git diff --numstat -z` — lens evidence MUST reuse `DeriveRiskInput`.
(Previously: pass/fail, blocking reasons, dry-run only — no lens breakdown or DeriveRiskInput reuse constraint)

#### Scenario: Structured output with lens findings

- GIVEN a pre-PR gate that fails on one deterministic R2 finding and has one inferential R4 warning
- WHEN run with `--json`
- THEN JSON MUST contain `pass:false`, `reasons` with deterministic finding, and `lensFindings:[{class,inferential}]` with warning

#### Scenario: No duplicate diff parsing

- GIVEN gate execution
- WHEN lens input is derived
- THEN system MUST reuse `DeriveRiskInput` single `git diff --numstat -z --no-renames` derivation
- AND MUST NOT run a second `git diff --stat` parse
