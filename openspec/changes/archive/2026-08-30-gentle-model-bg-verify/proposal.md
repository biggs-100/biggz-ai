# Proposal: 2026-08-30-gentle-model-bg-verify — Model Picker + Background 4-Sources + Verify Canonical

## Intent

Close 3 MEDIUM/HIGH `gentle-pi` Deferred gaps. `model-variants.json` written but never read (picker stub); background pi-scoped with drifted paths; verify lacks `integrity.json` pin. Slice 2 finishes Top-5, no overlap Slice 1.

## Scope

### In Scope
- **Model picker:** Port `lib/model-routing-authority.ts` `THINKING_LEVELS=[off,low,medium,high,inherit]`+`normalizeModelConfig`, `gentle:models`→`biggz models` BubbleTea table, read `~/.biggz/cache/model-variants.json`, write `~/.biggz/models.json`+cache, export/restore `MODEL_EXPORT_KIND="biggz-ai.agent_model_routing"` `v1`+frontmatter, `internal/opencode/model_picker.go` stub→full.
- **Background:** `internal/sdd/background.go` `project .biggz/background-subagents.json > global ~/.biggz/background-subagents.json > BIGGZ_BACKGROUND_SUBAGENTS env > off`, strict 2-key (extra→malformed→`off`), `gentle-pi.background-subagents/v1`, capability `ready/absent` via `subagent_run` vs `package.json`.
- **Verify:** `internal/install/verify.go` `sha256(readBinary)` vs `integrity.json`, `isConfined`+`isSymlink`+`sameFile`, `signedReleaseManifest` (port `lib/gentle-ai-binary.ts`), publish `integrity.json` via `.goreleaser.yaml`.

### Out of Scope
- Persona/banner/themes, Herdr, sync, CodeGraph, lenses, bench, optional runtime — Deferred.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `model-routing`: THINKING_LEVELS+normalize+cache parity+envelope+30-file picker.
- `runtime`: background 4-source strict+capability (`.biggz` paths, sdd ownership).
- `release-pipeline`: `integrity.json` publish+canonical verify.

## Approach

2 PRs `stacked-to-main`, `auto-chain`, `<400/PR`, `strict_tdd false`:

- **PR1 Picker ~340:** `model_picker.go` full + `models.go` wiring + `tui/models.go` table. Sorted `Record<provider,Record<model,string[]>>`, `walk_test` style.
- **PR2 Bg ~110 + Verify ~150 (~260):** `sdd/background.go` from `extensions/gentle-ai.ts` + delegate `opencode/background.go` + `install/verify.go` + `.goreleaser.yaml`. 1 PR >600 rejected; 3 PRs overhead.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/opencode/model_picker.go` | New | THINKING_LEVELS, normalize, `models.json`+cache |
| `internal/opencode/models.go` | Modified | Wire `LoadVariants`→picker |
| `internal/tui/models.go` | Modified | BubbleTea table, `agents>user>builtin` |
| `internal/sdd/background.go` | New | 4-source, strict 2-key, `ready/absent` |
| `internal/opencode/background.go` | Modified | Delegate to `sdd` |
| `internal/install/verify.go` | New | `sha256`+path guards+`signedReleaseManifest` |
| `.goreleaser.yaml` | Modified | Publish `integrity.json` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Cache divergence | Med | Freeze sorted schema; contract test |
| 2 JSON reads | Low | `LoadVariantsOrEmpty`; enrich on hit |
| `integrity.json` missing | Med | `archives.files`+checksum smoke; `go test -run Verify` |
| Path drift `.pi` vs `.biggz` | Low | `sdd` canonical; `pi` shim deprecated |

## Rollback Plan

`git revert` PR2 then PR1. No migrations. Remove `sdd/background.go`+`install/verify.go`, revert `model_picker.go`, drop `integrity.json`. Gate `go vet`+`go test`.

## Dependencies

- Oracle `lib/model-routing-authority.ts`, `lib/gentle-ai-binary.ts`, `extensions/gentle-ai.ts`, `comparison-with-gentle.md`.
- Existing `model-variants.ts` `tmp→rename`, `internal/opencode/models.go`, `pi/adapter.go`, `.goreleaser.yaml`.

## Success Criteria

- [ ] `THINKING_LEVELS`+`normalize` round-trips `models.json`+cache; `biggz models` 30 files.
- [ ] `MODEL_EXPORT_KIND="biggz-ai.agent_model_routing"` `v1` export/restore+frontmatter lossless.
- [ ] `sdd/background.go` `project>global>env>off`, strict 2-key malformed→`off`, `ready/absent`.
- [ ] `install/verify.go` `sha256==integrity.json`+guards+`signedReleaseManifest`; `integrity.json` in release.
- [ ] `go vet` PASS, `go test ./internal/opencode ./internal/sdd ./internal/install` PASS, each PR `<400`.

## Alternatives Considered

- Single PR: rejected — ~600 >400, loses granularity.
- Keep `pi/adapter.go`: rejected — SDD ownership, `.biggz` drift.
- Re-download verify: rejected — pin avoids network.
