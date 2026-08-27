# Proposal: prompt-skill-resolver — Prompt-as-File + skill:// Resolver

## Intent

Externalize review lens prompts from `fmt.Sprintf` to static `.md` via `text/template {{.Var}}` (oh-my-pi C parity: `.md`+Handlebars→Go `text/template`). Add 7-provider priority and secure `skill://` resolver. `assets/prompts/sdd` already `go:embed` but `review/lens` still inline `fmt.Sprintf`; `skillregistry` lacks priority and `skill://` containment.

## Scope

### In Scope
- 6 `.md` at `internal/assets/prompts/review/` (R1-R4/external/shared) via `go:embed`
- Render via `text/template {{.Var}}` — no `html/template`, literals, or `fmt.Sprintf` for prompts
- CI `no-fmtSprintf` lint forbidding `fmt.Sprintf` in lenses
- `internal/skillregistry/resolver.go` — 7-provider priority, `ScanSkillsFromDir` non-recursive, `disabledExtensions: skill:<name>`, `ignored/include` globs
- `skill://<name>/<path>` with `path.Clean`+`EvalSymlinks`+`HasPrefix`, `..` rejection, traversal tests

### Out of Scope
- Lens logic (R1–R4) unchanged
- `hashline-lite`, `tui`, `sdd` pipeline; prompts beyond review

## Capabilities

### New Capabilities
- `skill-resolver`: 7-provider priority, non-recursive scan, disabledExtensions, globs, `skill://` with containment
- `prompt-templates`: static `.md` review prompts via `text/template`

### Modified Capabilities
- `review-lenses`: prompts → `.md`+`go:embed`; logic unchanged; `no-fmtSprintf` lint

## Approach

Move strings from `internal/review/lens/*/*.go` to `prompts/review/<lens>.md`; add `//go:embed all:prompts/review` to `embed.go`. Replace `fmt.Sprintf` with `template.Parse`+`Execute` (`missingkey=error`). Add `resolver.go` with priority slice, `os.ReadDir` non-recursive, `disabledExtensions` map, glob filter, `ResolveSkillURI` via `path.Clean`+`EvalSymlinks`+`HasPrefix`; reject absolute/`..`. CI: `rg 'fmt\.Sprintf' internal/review/lens` fails. Tests: `../`/symlink escape must error.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/prompts/review/*.md` | New | 6 templates |
| `internal/assets/embed.go` | Modified | Add `prompts/review` to embed |
| `internal/review/lens/*/*.go` | Modified | `fmt.Sprintf` → `template.Execute` + loader |
| `internal/skillregistry/resolver.go` | New | Priority + `skill://` |
| `internal/skillregistry/registry.go` | Modified | Priority, non-recursive, disabledExtensions, globs |
| `.github/workflows/ci.yml` | Modified | `no-fmtSprintf` guard |
| `*_test.go` (lens+registry) | Modified | Traversal/priority/template tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Traversal/symlink bypass | Low | Realpath `EvalSymlinks`+`HasPrefix`; tests `../`, symlink |
| Priority inversion | Med | Explicit array + deterministic order; 7-provider regression |
| Template var typo | Low | `missingkey=error` + per-template test |
| Lint false positives | Low | Narrow to `lens` + `//lint:ignore` allowlist |

## Rollback Plan

`git revert` one commit: delete `prompts/review/*.md`+`resolver.go`, restore `fmt.Sprintf`, remove embed+CI step, `go mod tidy`. No data loss; <5 min.

## Dependencies

- `text/template` stdlib; oh-my-pi `packages/skills` provider order
- `go test ./... -count=1 -timeout 180s` gate

## Success Criteria

- [ ] 6 `.md` embedded via `assets.FS`; lenses use `text/template` not `fmt.Sprintf`
- [ ] `rg fmt\.Sprintf internal/review/lens` == 0 for prompts; CI fails otherwise
- [ ] `skill://` via `Clean`+`EvalSymlinks`+`HasPrefix`; `../`/symlink rejected
- [ ] Priority (7), non-recursive scan, disabledExtensions, globs honored
- [ ] `go test` + `go vet` + `gofmt` clean

## Proposal question round

Assumptions: 6 prompts (R1–R4+external+shared), oh-my-pi priority, `skill://` any-depth with containment. Correct/skip/second round?
