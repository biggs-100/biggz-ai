# Design: 2026-08-29-ola3-gentle-final-hardening — Gentle Final Hardening Ola 3

## Technical Approach

Port gentle-pi patterns (RO view, model routing, doctor drift) into Go harness as 3 stacked slices. C1 hardens git subprocess; C2 TUI routing; C3 RO diagnostics via `ManagedAssetHash`. No banner/authority. Each <400 lines, `stacked-to-main`, `go vet`+`go test` green.

## Architecture Decisions

| # | Decision | Options (Tradeoff) | Choice & Rationale |
|---|----------|--------------------|---------------------|
| 1 | Candidate view file | A) `internal/review/candidate_view.go` new B) extend `capture.go` | **A** — isolates RO+manifest+symlink; mirrors `lib/review-candidate-view.ts`; no coupling to capture |
| 2 | Manifest digest | A) `sha256:hex(JSON)` sorted+snake_case B) raw SHA | **A** — identical to `digestChangedPathManifest`; canonical JSON deterministic |
| 3 | Git parsing | A) `--name-status` B) `--raw -z`+`GIT_LITERAL_PATHSPECS=1` | **B** — only `--raw` exposes modes/SHAs for `modeOnly`/`typeChanged`; `GIT_LITERAL_PATHSPECS=1` blocks glob; `-z` handles special chars |
| 4 | RO enforcement | A) `chmod 0444/0555`+`GOOS=windows` skip B) build tags | **A** — runtime check mirrors `makeReadonly`; `GOOS` skip; `makeWritableForCleanup` for rm |
| 5 | Model persistence | A) `~/.biggz/models.json` v1 B) `opencode.json` | **A** — dedicated v1 isolates TUI state; `agents>user>builtin` via authority; no pollution |
| 6 | TUI framework | A) Bubbles `internal/tui/models.go` B) raw bubbletea | **A** — matches `tui.go` (`syncOutput`/styles); picker reuses `opencode` cache |
| 7 | Doctor severity | A) `warn` B) `fail` | **A** — `warn: Global SDD asset drift N` non-blocking; `StatusWarn`; no `--fix`; `Runner` recovers |

## Data Flow

```
C1: git --raw -z ──→ parseManifest ──→ digest sha256:hex ──→ RO 0444/0555+isWithin
      GIT_LITERAL_PATHSPECS=1   NUL rename/modeOnly/typeChanged
C2: TUI models.go ──→ ~/.biggz/models.json v1 ──→ envelope agent_model_routing v1 ──→ frontmatter
     Bubbles 30 files   agents>user>builtin      MODEL_EXPORT_KIND/VERSION
C3: managed-assets.json ──→ ManagedAssetHash ──→ doctor.Runner ──→ warn/pass (RO)
     SHA256 hex compare      panic-isolated
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/review/candidate_view.go` | Create | RO `0444/0555`, `digestChangedPathManifest`, `--raw -z`, `isWithin`+symlink, `GOOS` skip |
| `internal/tui/models.go` | Create | Bubbles modal, 30-file picker, 4 modes+`inherit`, `agents>user>builtin` |
| `internal/opencode/models.go` | Modify | Read `~/.biggz/models.json` v1, normalize, export envelope |
| `internal/assets/managed.go` | Modify | Expose `ManagedAssetHash`; keep skip/force |
| `internal/doctor/drift.go` | Create | `sddGlobalAssetDriftCount`+`sddLocalAgentOverrideCount`, `StatusWarn` |
| `internal/doctor/runner.go` | Modify | `RunAll` panic isolation |
| `openspec/specs/system-diagnostics/spec.md` | Modify | Delta drift RO |
| `openspec/specs/tui/spec.md` | Modify | Delta picker |
| `openspec/specs/managed-assets/spec.md` | Modify | Delta `ManagedAssetHash` |

## Interfaces / Contracts

```go
// C1 — internal/review/candidate_view.go
type ChangedPathEntry struct {
    Path string; Status string; OldMode, NewMode string
    Deleted, TypeChanged, ModeOnly bool
}
func DeriveChangedPathManifest(cwd, baseTree, candidateTree string) ([]ChangedPathEntry, error)
func DigestChangedPathManifest(manifest []ChangedPathEntry) string // "sha256:<hex>"

// C2 — internal/tui/models.go + internal/opencode/models.go
type ThinkingLevel string // "off"|"low"|"medium"|"high"|"inherit"
type AgentModelConfig map[string]struct{ Model string; Thinking ThinkingLevel }
const ModelExportKind = "gentle-pi.agent_model_routing"; const ModelExportVersion = 1

// C3 — internal/assets/managed.go + internal/doctor/drift.go
func ManagedAssetHash(data []byte) string
func ManagedAssetHashFile(path string) (string, error) // hex
type DriftCheck struct{} // implements doctor.Check, ID="sddGlobalAssetDrift"
```

Envelope: `{"kind":"gentle-pi.agent_model_routing","version":1,"agents":{...}}`; frontmatter `model:`+`thinking:` after `description:`.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `DigestChangedPathManifest`, `isWithin`, `modeOnly`/`typeChanged`, `ManagedAssetHash` | Table-driven `go test`, temp dirs |
| Integration | `--raw -z` NUL parse, RO `0444/0555`, `agents>user>builtin` | Temp git repos, `chmod` asserts (`GOOS=windows` skip), JSON round-trip |
| E2E | `biggz doctor` warn/pass, `--json`, no `--fix`, Runner isolation | Panicking check, CLI `--json` golden |

## Threat Matrix

`references/threat-matrix.md` not found; evaluated:

| Boundary | Applicable | Safe/Failure Behavior | RED Test |
|----------|------------|----------------------|----------|
| Shell/subprocess (git) | Applicable | `GIT_LITERAL_PATHSPECS=1`, `-z`, fail-closed | `a;rm -rf` blocked; bad `--raw` → error |
| VCS/PR | N/A | No PR creation | — |
| Routing (model) | Applicable | `agents>user>builtin`, `inherit`→global | All layers set, agents wins |
| Executable | N/A | No detection | — |
| Process integration | Applicable | `GOOS=windows` skip, `recover()` isolates | Windows skip; panic→warn |

## Migration / Rollout

No migration. 3 slices `stacked-to-main`, each <400, `go vet`+`go test`+`gofmt`:

- **C1 RO (~180)**: `candidate_view.go` — base `main`
- **C2 TUI (~250)**: `tui/models.go`+`opencode/models.go` — base `C1`
- **C3 drift (~120)**: `doctor/drift.go`+`managed.go` — base `C2`

Per-slice `git revert`. No `--fix`/banner.

## Open Questions

- [ ] `digestChangedPathManifest` JSON byte parity vs gentle-pi `JSON.stringify` — verify in C1
- [ ] Picker 30-file source: `model-variants.json` cache vs static — confirm at C2
