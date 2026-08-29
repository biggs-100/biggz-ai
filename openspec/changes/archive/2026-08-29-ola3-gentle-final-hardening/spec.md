# Spec — 2026-08-29-ola3-gentle-final-hardening — Gentle Final Hardening Ola 3
Concatenated delta spec. Per-domain deltas live in `specs/{domain}/spec.md`. No banner.

## review/candidate-view — NEW (C1) `candidate-view-ro`

### Requirement: Read-Only Candidate View with SHA256 Manifest and Symlink Guard

The system MUST materialize candidate view read-only (`0444` files, `0555` dirs), emit `digestChangedPathManifest` as `sha256:<hex>` over canonical JSON, invoke `git --raw -z` with `GIT_LITERAL_PATHSPECS=1`, handle renames/modeOnly/typeChanged, block `../../etc/passwd` via `isWithin`/`resolve`, and skip chmod on `GOOS=windows`.

#### Scenario: RO permissions
- GIVEN view materializes via `internal/review/candidate_view.go`
- WHEN `stat` inspects files/dirs on non-Windows
- THEN files MUST be `0444` and dirs `0555`

#### Scenario: SHA256 manifest
- GIVEN `digestChangedPathManifest` over canonical JSON
- WHEN manifest emitted
- THEN value MUST equal `sha256:<hex(sha256(JSON))>`

#### Scenario: Raw -z rename/modeOnly/typeChanged
- GIVEN repo has rename `a→b`, modeOnly, typeChanged
- WHEN `git --raw -z` with `GIT_LITERAL_PATHSPECS=1` parses
- THEN each kind MUST be classified correctly

#### Scenario: Traversal and symlink blocked, Windows skip
- GIVEN path `../../etc/passwd` or symlink escaping repo
- WHEN `isWithin` validates
- THEN it MUST block; on `GOOS=windows` chmod MUST be skipped

## model-routing — NEW (C2) `model-routing-tui` + `tui` picker

### Requirement: Per-Agent Model Routing TUI (30 files, 4 thinking modes)

The system MUST provide `internal/tui/models.go` Bubbles modal mapping `agents → user > builtin`, persist `~/.biggz/models.json` v1 `{"sdd-design":{"model":"claude-sonnet-4","thinking":"high"}}`, support per-agent `model`+`thinking(off/low/medium/high/inherit)`, emit envelope `gentle-pi.agent_model_routing v1` with `MODEL_EXPORT_KIND/VERSION`, and picker over 30 files.

#### Scenario: Modal precedence and persistence
- GIVEN user picks `sdd-design`=`claude-sonnet-4`/`high` in modal
- WHEN saved and reloaded
- THEN `~/.biggz/models.json` MUST contain v1 entry and precedence `agents > user > builtin` MUST hold

#### Scenario: Thinking inherit
- GIVEN agent `thinking=inherit` and global `high`
- WHEN routing resolves
- THEN effective MUST be `high`; `off/low/medium` MUST apply directly

#### Scenario: Envelope round-trip and picker
- GIVEN envelope `gentle-pi.agent_model_routing v1` written with frontmatter
- WHEN parsed via `1334-1346`/`1377-1381`
- THEN fields MUST round-trip and picker MUST list 30 files across 4 modes

## doctor — MODIFIED (C3) `system-diagnostics` + `managed-assets`

### Requirement: Doctor Read-Only Drift Checks

The system MUST add `biggz doctor` RO checks `sddGlobalAssetDriftCount`/`sddLocalAgentOverrideCount` derived via `assets/managed.go:ManagedAssetHash` SHA256 vs `managed-assets.json` v1, report `warn: Global SDD asset drift N` when `N>0` (`warn` not `fail`), expose no `--fix`, and keep Runner panic-isolated.

#### Scenario: Drift warn
- GIVEN global `sdd-foo.md` hash differs from manifest
- WHEN `biggz doctor` runs
- THEN result MUST be `warn` with `Global SDD asset drift 1`

#### Scenario: No drift pass, no fix, JSON
- GIVEN all hashes match
- WHEN `biggz doctor` and `biggz doctor --json` run
- THEN counts MUST be `0` status `pass` and `--fix` MUST be absent

#### Scenario: Runner isolation
- GIVEN drift check panics inside `diagnostics/doctor.go` Runner
- WHEN `Run()` completes
- THEN drift result MUST be `warn`/`fail` with message and other checks unaffected
