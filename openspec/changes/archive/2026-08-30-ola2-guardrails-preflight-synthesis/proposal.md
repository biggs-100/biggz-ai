# Proposal: 2026-08-30-ola2-guardrails-preflight-synthesis — Ola 2 Guardrails / Preflight / Synthesis Gate

## Intent

Port gentle-pi Ola 2 hardening to biggz-ai. Commit `9f6c8be` already implements `470` lines across three files; this change retroactively formalizes intent, requirements, design, tasks, and apply evidence so `biggz sdd-status` advances to `verify` and history stays traceable.

## Problem

- **Bash patterns unblocked:** No `guardrails.go`. `rm -rf /|~|$HOME|..`, `git reset --hard`, `git clean -f -d`, `git push --force`, `chmod 777`, `chown -R` not denied; guarded commands (`git push`, `rebase`, `branch -D`, `npm publish`, `pi remove`) lacked `autonomousMode` classification and two-file merge (`global→project`, env fast-path).
- **Sensitive paths unguarded:** `read/write/edit` could access `.ssh`, `.credentials`, `.aws/credentials`, `hosts.yaml`, `secrets/`, `.env`, `.pem` without blocking.
- **Preflight not persisted:** No `preflight.go`. Missing `Normalize` (`hybrid` aliases, `none→""`), `canonicalize` defaults, `Write/ReadSddPreflightToDisk`, and `Resolve` (`cache>disk>defaults`).
- **Synthesis gate absent:** No `synthesis_gate.go`. Checkpoint asks within `120s` not required to contain four markers (`## Sub-agent Result:`, `**What was done:**`, `**Artifacts/Paths:**`, `**Next Recommended:**`) unless `Session Recall` or `PI_SUBAGENT_CHILD=1` bypass.

## Solution

Single retroactive PR — `9f6c8be` already implements it:

- `internal/policy/guardrails.go` (251): `deniedBashPatterns`/`IsDenied` (git refinements), `guardedKeyPatterns`/`ClassifyGuardedCommand`, `Parse/LoadRuntimeGuardrailsConfig` merge, `EvaluateSensitivePathTool`.
- `internal/sdd/preflight.go` (152): `PreflightPrefs`, `SddPreflightDiskPath`, `Normalize`+`canonicalize`, `Write/Read` + cache `Resolve`.
- `internal/sdd/synthesis_gate.go` (67): `HasSynthesis` (4 markers), `ShouldBlock` 120s + bypasses.

Store `openspec`, `strict_tdd false`, `interactive/auto-chain/400`, single slice.

## Scope

**In scope:** SDD artifacts; `9f6c8be` three files; spec deltas `policy` (4 req) + `sdd` (3 req) — 7 req, 21 scenarios; `go vet/test` evidence, `470` lines, `git revert` rollback.

**Out of scope:** Ola 1/3 layers; banner/authority/watch; BigMem migration; `verify-report`.

## Success Criteria

- [ ] `IsDenied` blocks `rm -rf /|~|$HOME|..`, `git reset --hard`, `git clean -f -d`, `git push --force`, `chmod 777`, `chown -R`; unforced not blocked.
- [ ] `ClassifyGuardedCommand` denied→`block`, `!auto→confirm`, auto defaults `gitPush allow`/`npmPublish block`, custom override, unknown `not-guarded`.
- [ ] `LoadRuntimeGuardrailsConfig` merges `global+project` (project wins), `env=1` fast-path, malformed→safe.
- [ ] `EvaluateSensitivePathTool` blocks `.ssh/.aws/.env/secrets/.pem` on guarded tools.
- [ ] `Normalize` alias→`hybrid`, `none→""`; `canonicalize` defaults `interactive/400`.
- [ ] `Write→Read` round-trip, `Resolve` `cache>disk>defaults`.
- [ ] `ShouldBlock` 120s requires 4 markers, bypasses `recall/child`, expires past 120s.
- [ ] `go vet` PASS, `470` lines, no drift; `sdd-status` `verify ready`.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `rm -rf` over-blocks `./scoped` | Low | Restricted to `/(~|$HOME|..)` roots |
| Config merge mutates global | Low | Copy-on-merge; `TempDir` isolated test |
| `120s` clock flake | Low | Injectable `now`; prod `time.Now` only in wrapper |

## Rollback Plan

`git revert 9f6c8be` deletes three files (`470`). No migration. Remove `openspec/changes/…` dir or `sdd-attempt reset` if needed. Isolated from Ola 1/3.

## Dependencies

- gentle-pi `guardrails.ts`, `sdd-preflight.ts`, `synthesis-gate.ts` (verbatim).
- Commit `9f6c8be` on `main`; Go 1.25, `go vet`, `go test -count=1`, `biggz sdd-status`.

## Alternatives Considered

- Single doc vs full slice: rejected — `sdd-status` requires `proposal/spec/design/tasks/apply` to reach `verify`.
- Chained PR vs single: rejected — one commit already merged; document as `Medium` `size:exception-ok`.
- BigMem store: rejected — project is `openspec`; matches archived Ola 1/3.

