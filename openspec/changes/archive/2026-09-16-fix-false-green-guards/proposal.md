# Proposal: fix-false-green-guards

## Intent

`tui-sanitize/spec.md:95` mandates *"Hard-fail if any Go file outside internal/git contains `exec.Command` with git"*; line 113: no warn-only. Line 115's scenario is FALSE: 57 visible + 9 invisible sites, CI green. Root cause: `rg` absent, `2>/dev/null` eats 127, `set -e` inert in `if`.

## Scope

### In Scope
- Fail-closed wiring (`pipefail`, observed status); delete two duplicate `rg` steps.
- Compiled self-tested guard; ratchet allowlist, fail-on-stale = FAILURE.
- Extended `internal/git` surface, migration waves, dedup; `gatekeeper` store-aware; `convergence.go` fail-closed.
- Mark the 14 non-prompt `fmt.Sprintf` sites in `readability/lens.go`.

### Out of Scope
- Non-Go host spawns: pi host, outside the "Go file" invariant.
- `internal/doctor`'s `execFn` seam: exception; raw spawns forbidden.
- `internal/git` redesign beyond env-aware construction.
- Narrowing the lens guard: the marker is the spec's escape hatch.

## Capabilities

### New Capabilities
- `ci-guard-integrity`: compiled checker, positive control, fail-on-stale baseline.

### Modified Capabilities
- `tui-sanitize`: scope = Go files + `exec.Command`; `rg` → compiled checker + ratchet.
- `prompt-skill-resolver`: guard observes its scanner; 14 non-prompt uses carry the marker.
- `testing-guidance`: drop `rg` guards; primary observes `go vet`.
- `complexity-gates`: producers fail closed (no `|| true`).
- `sdd`: gatekeeper resolves artifacts per `ArtifactStore`.

## Approach

1. **PR1**: observe `go vet`'s status; delete both weaker `rg` steps; repair `no_fmt_guard_test.go` (repo-root-anchored, hard-fail on unreadable); mark the 14 non-prompt sites; land the compiled guard (must-fail fixture) freezing 66 sites.
2. **PR2..N**: extend `internal/git` (`Run`, `TopLevel`, `ResolveGitDirs`); one package group per PR, entries deleted together. Final PR: empty allowlist.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `.github/workflows/ci.yml` | Modified | 7 guards fail-closed; 2 `rg` steps deleted |
| `internal/guardcmd/**`, `.github/guard-baseline.txt` | New | Scanner; allowlist 66→0 |
| `internal/{git,review,sdd,release,install,sddattempt,doctor}/**`, `cmd/biggz/**`, `no_fmt_guard_test.go` | Modified | Route, dedupe, fail-closed |
| `docs/testing-guidance.md` | Modified | Drop `rg` contract |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Flip turns master red | High | PR1 ships wiring + allowlist |
| Indirection blind spot | Med | Flag command-position literals |
| Env hardening lost | Med | Golden `GIT_*`/locale test |

## Rollback Plan

Each PR is a revertible unit. Reverting PR1 restores prior guard behavior; waves delete code and allowlist entries together, so revert is atomic.

## Dependencies

- Analyzer via `go.mod`; `bash -e` needs `pipefail`; gatekeeper fix (preflight `both`).

## Success Criteria

- [ ] No CI step depends on `rg`; each observes its checker's status.
- [ ] Self-test rejects a git-spawn fixture outside `internal/git`.
- [ ] Allowlist non-empty at PR1 → empty at final PR; stale entries fail.
- [ ] Line 115 scenario true in CI; `go test`, `go vet`, `gofmt` clean.
