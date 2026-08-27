# Tasks: prompt-skill-resolver — Prompt-as-File + skill:// Resolver

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700-850 |
| 400-line budget risk | High |
| 800-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 templates → PR2 resolver+CI |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | 6 .md prompts + embed + lens loader | PR1 | `go test ./internal/assets -run TestPromptTemplates -count=1` | `go test ./internal/review/lens -run TestLensPromptRender -count=1` | Delete `prompts/review/*.md`; revert `embed.go` + `lens/*/*.go` |
| 2 | Resolver + CI guard: priority, skill:// containment, no-fmtSprintf | PR2 | `go test ./internal/skillregistry -run TestResolveSkillURI -count=1` | `rg 'fmt\.Sprintf' internal/review/lens; go vet ./...` | Delete `resolver.go`; revert `registry.go` + `ci.yml` |

## Phase 1: Foundation

- [x] 1.1 Create `internal/assets/prompts/review/` 6 `.md` (R1-R4/external/shared) with `{{.Var}}` — done when `assets.FS.ReadDir` returns 6
- [x] 1.2 Edit `internal/assets/embed.go` add `all:prompts/review` to `//go:embed` — `go vet ./internal/assets` passes
- [x] 1.3 Create `internal/skillregistry/resolver.go` skeleton: `ProviderPriority [7]string`, `ScanSkillsFromDir`, `ResolveSkillURI`, `LoadPrompt` — `go vet` passes

## Phase 2: Core Implementation

- [x] 2.1 RED: `internal/skillregistry/resolver_traversal_test.go` with `TestTraversalDotDot` (`skill://foo/../../etc/passwd` → error), `TestSymlinkEscape` (`link->/etc/passwd` → error), `TestAbsoluteRejected` (`skill://foo//etc/passwd` → error)
- [x] 2.2 Implement `ResolveSkillURI` via `path.Clean`+reject `..`/absolute+`Join+EvalSymlinks+HasPrefix` — GREEN 2.1; valid `skill://foo/docs/a.md` returns bytes under root
- [x] 2.3 Implement `ScanSkillsFromDir` via `os.ReadDir` non-recursive, filter `disabledExtensions: skill:<name>` and `ignored/include` globs
- [x] 2.4 Wire 7-provider ordered `[7]string` first-win in `internal/skillregistry/registry.go` — provider 2 wins over 5
- [x] 2.5 Implement `LoadPrompt(name)` via `assets.FS.ReadFile`+`template.Option("missingkey=error")` — missing var returns error
- [x] 2.6 Migrate `internal/review/lens/*/*.go` from `fmt.Sprintf` prompts to `LoadPrompt+Execute` — zero prompt `fmt.Sprintf` remains

## Phase 3: Integration

- [x] 3.1 Edit `.github/workflows/ci.yml` add `no-fmtSprintf` step (`rg 'fmt\.Sprintf' internal/review/lens` fails, `//lint:ignore no-fmtSprintf` allowed, required)
- [x] 3.2 Keep `lens.Lens.Analyze` pure (`LensInput`); prompt rendered before heuristic — `go test ./internal/review/lens -run TestAnalyze`
- [x] 3.3 Define `{{.Var}}` Data structs per prompt (inventory) — all vars covered by `missingkey=error` test

## Phase 4: Testing

- [x] 4.1 `TestTemplatesEmbedded` 6 `.md` OK, `TestMissingVarFails` error, `TestRenderNoBraces` output has values no `{{`
- [x] 4.2 `TestPriorityDeterministic`, `TestNonRecursive`, `TestDisabledExtensions`, `TestGlobFiltering` via `t.TempDir`
- [x] 4.3 CI guard: prompt `fmt.Sprintf` → fail, clean → pass, allowlisted → pass
- [x] 4.4 Evidence: `go test ./... -count=1 -timeout 180s` + `go vet ./...` + `gofmt -l .` + `rg 'fmt\.Sprintf' internal/review/lens` clean

## Phase 5: Cleanup

- [x] 5.1 `go mod tidy`; no `html/template` for prompts; remove test fixtures
- [x] 5.2 Document `ProviderPriority` order (oh-my-pi parity)

## Dependencies & Order

1.1→1.2→1.3 → 2.1 RED before 2.2 → 2.3/2.4 → 3.1 → 4.x. Evidence: `go test ./... -count=1 -timeout 180s`, `go vet ./...`, `gofmt -l .`.
