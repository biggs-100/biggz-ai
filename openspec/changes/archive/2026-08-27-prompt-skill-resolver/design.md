# Design: prompt-skill-resolver — Prompt-as-File + skill:// Resolver

## Technical Approach

Move 6 prompts (R1-R4/external/shared) from `fmt.Sprintf` in `internal/review/lens/*/*.go` to `internal/assets/prompts/review/*.md` via `text/template {{.Var}}` with `missingkey=error`, embedded through `assets.FS` (`//go:embed all:prompts/review`). Add `internal/skillregistry/resolver.go` for 7-provider priority, non-recursive `os.ReadDir`, `disabledExtensions` (`skill:<name>`), glob filtering, and secure `skill://` (`path.Clean` + `EvalSymlinks` + `HasPrefix`). Add CI `no-fmtSprintf` guard on `internal/review/lens`. Satisfies 4 req / 15 scenarios.

## Architecture Decisions

| Decision | Chosen | Alternative | Tradeoff | Rationale |
|----------|--------|-------------|----------|-----------|
| Template engine | `text/template` | `html/template` | `html/template` escapes `<>&"` corrupts diffs | Parity with oh-my-pi Handlebars; spec forbids `html/template` |
| Embed scope | `all:prompts/review` | per-file list | Per-file misses new prompts | Existing `embed.go` already `all:prompts`; idiomatic |
| Priority | ordered `[7]string` first-win | `map[string]int` | Map random iteration | Deterministic oh-my-pi order; provider-2 over 5 |
| Scan | `os.ReadDir` non-recursive | `WalkDir` recursive | Recursive leaks `nested/SKILL.md` | Spec ignores `a/nested/SKILL.md` |
| Containment | `Clean`+`EvalSymlinks`+`HasPrefix` | `Clean` only | `Clean` misses symlink `-> /etc/passwd` | RED test requires realpath check |

## Data Flow

```
assets.FS ──→ LoadTemplate ──→ Parse(missingkey=error) ──→ Execute(data) ──→ prompt
LensInput (RiskInput+Hunks) ──────────────────────────────→ lens.Analyze() ──→ LensResult
ScanSkillsFromDir ──→ ReadDir(non-recursive) ──→ filter disabledExtensions+globs ──→ Registry (7-provider first-win)
skill://foo/docs/a.md ──→ Clean(reject ..)+reject absolute ──→ Join+EvalSymlinks ──→ HasPrefix(rootReal,candidateReal) ──→ bytes|error
```

Missing var fails fast; traversal fails without FS access outside root.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/assets/prompts/review/*.md` (6) | Create | R1-R4/external/shared templates |
| `internal/assets/embed.go` | Modify | Add `all:prompts/review` to embed |
| `internal/review/lens/*/*.go` | Modify | `fmt.Sprintf` → `template.Execute` via loader |
| `internal/skillregistry/resolver.go` | Create | Priority + `ScanSkillsFromDir` + `ResolveSkillURI` |
| `internal/skillregistry/registry.go` | Modify | Wire priority, non-recursive, globs |
| `.github/workflows/ci.yml` | Modify | `no-fmtSprintf` step on `internal/review/lens` |
| `*_test.go` (lens+registry) | Modify | Template + priority + traversal tests |

## Interfaces / Contracts

```go
var ProviderPriority = [7]string{"user:opencode","user:biggz","user:claude","user:kilo","project:skills","project:opencode","project:github"}

func ScanSkillsFromDir(dir string, opts ScanOpts) ([]Entry, error)
func ResolveSkillURI(uri string, roots map[string]string) ([]byte, error) // skill://<name>/<path>, Clean+EvalSymlinks+HasPrefix
func LoadPrompt(name string) (*template.Template, error) { // assets.FS.ReadFile + Option("missingkey=error")
}
```

`lens.Lens.Analyze` unchanged; prompt rendered before heuristic.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit | 6 `.md` embedded; `missingkey=error` without var; render no `{{`; zero `fmt.Sprintf` | `FS.ReadDir`, `Execute` error, `rg` scan |
| Unit | Priority 2>5; non-recursive; `skill:foo` exclude; `ignored: ["*_test*"]` | `t.TempDir` table-driven |
| Unit | `skill://` valid inside; `../`, symlink, `//etc` rejected via realpath | Temp roots + symlink fixtures |
| Integration | CI guard fail/pass/allowlist | Run `rg` script |
| E2E | `go vet`/`gofmt`/`go test ./...` clean | Existing gates |

## Threat Matrix

| Boundary | Cases | Applicability | Design response | Planned RED tests |
|----------|-------|---------------|-----------------|-------------------|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, MD/MDX, `README.sh` | N/A — no execution | — | None |
| Git repo selection | `git -C`, relative/absolute | N/A — reuse `DeriveRiskInput` | — | None |
| Commit state | staged, `commit -a`, empty | N/A — no commit | — | None |
| Push state | tracking, first push, refspec | N/A — no push | — | None |
| PR commands | `--head`, env prefix | N/A — no PR | — | None |
| Skill URI traversal | `../`, symlink `->/etc/passwd`, `//etc` | Applicable | `Clean` rejects `..`/absolute; `EvalSymlinks`+`HasPrefix` blocks symlink; no outside FS access | `TraversalDotDot`, `SymlinkEscape`, `AbsoluteRejected`, `ValidInside` |

Applicable rows propagate to tasks/RED unchanged.

## Migration / Rollout

No migration. Revert: delete `prompts/review/*.md`+`resolver.go`, restore `fmt.Sprintf`, revert `embed.go`/`ci.yml`, `go mod tidy`.

## Open Questions

- [ ] `{{.Var}}` inventory per prompt — define `Data` structs in tasks
- [ ] Verify exact 7-provider order vs oh-my-pi `packages/skills`
```
