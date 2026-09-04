# Design: TUI Installer Full Parity

## Technical Approach

Extend the 7-state `InstallModel` (`install.go`) into a 10-stage wizard mirroring gentle-ai's `linearRoutes`, adding pure render screens under `internal/tui/screens/` and reusing `Orchestrator.RunWithChan` + `ProgressChan(32)` + `InstallingModel`. No pipeline API change; no new style tokens.

```
Welcome → Detection → Agents → Persona → Preset → DepTree → SkillPicker → Review → Installing → Complete
   │          │          │         │         │          │           │            │           │           │
   │     detectAdapters  │   opencode cfg  │     static plan  opencode skills  │  runOrchestratorCmd  │
   └─ InstallModel (extended step enum) ───┴─────────┴──────────┴───────────┴──┴─ ProgressChan(32) ──┘
```

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Extend `install.go` enum vs new `wizard.go` | Duplicates detect/review/running; enum keeps legacy fallback trivial | Extend enum (`stepWizWelcome…Complete`); gate `NewInstallModel` on `BIGGZ_LEGACY_INSTALL` |
| `router.go` vs `tui.go` int consts | `tui.go` uses ints; gentle uses typed `Screen`+map | New `internal/tui/router.go`: `WizardStage` + `linearRoutes` + `Next/Prev`; one `screenInstall` maps to wizard |
| Pure `Render*` + state in `InstallModel` vs 10 Bubble models | 10 models = message-plumbing explosion | Hybrid: pure `Render*` funcs + state in `InstallModel`; `InstallingModel` stays a real model |
| Reuse `ModelPickerScreen` vs port gentle pickers | Gentle pickers need `catalog/model` biggz lacks; biggz precedence already tested | Adapt: thin wrappers (`claude/codex/kiro/opencode_picker.go`) over `opencode.MergeModelConfigs` + `styles` only |
| Port gentle item-logs vs keep 30-char bar | Richer list breaks bar goldens | Keep `InstallingModel` bar; step-line from `ProgressEvent.Step` (already rendered) |

## Data Flow

Selections accumulate in `InstallModel`; Review summarizes; confirm calls `runOrchestratorCmd(adapter, ch)` + `waitProgress(ch)`; events → `InstallingModel.Update`; close → `progressDoneMsg` → Complete; failure → error view. `RollbackOnFailure` unchanged.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/tui/router.go` | Create | `WizardStage` type, `linearRoutes` map (10 stages), `NextStage/PrevStage`, legacy guard |
| `internal/tui/screens/wizard_welcome.go` | Create | `RenderWizardWelcome` (title+steps, no logo/tagline/advisory) |
| `internal/tui/screens/wizard_detection.go` | Create | `RenderWizardDetection` from `detectAdapters()` result |
| `internal/tui/screens/wizard_agents.go` | Create | Multi-select agents (space toggle, checkbox render) |
| `internal/tui/screens/wizard_persona.go` | Create | Persona radio (biggz personas, not gentleman) |
| `internal/tui/screens/wizard_preset.go` | Create | Preset radio (Full/DevStack/Minimal/Custom labels, biggz wording) |
| `internal/tui/screens/wizard_deptree.go` | Create | Read-only plan list; checkbox picker when Custom |
| `internal/tui/screens/wizard_skills.go` | Create | Skill multi-select from `opencode`/assets skill list |
| `internal/tui/screens/wizard_review.go` | Create | Agents+Persona+Preset+Skills summary; Install/Back |
| `internal/tui/screens/wizard_complete.go` | Create | Success summary (reuse Done-view fields) |
| `internal/tui/screens/claude_picker.go`, `codex_picker.go`, `kiro_picker.go`, `opencode_picker.go` | Create | Thin wrappers over `ModelPickerScreen`/`opencode.MergeModelConfigs` per background |
| `internal/tui/screens/install.go` | Modify | Extend step enum, selection fields, Update (forward/back per stage), View dispatch, `BIGGZ_LEGACY_INSTALL=1` fallback |
| `internal/tui/screens/review.go` | Modify only if needed | Prefer `wizard_review.go`; avoid breaking existing review |
| `internal/tui/screens/wizard_*_test.go`, `router_test.go` | Create | Traversal, routing, reduced-motion, banner-grep, progress wiring tests |

NOT ported: `RenderLogo/Tagline/updateBanner/advisory`, community/upgrade/sync/profile-CRUD, gentle `catalog/model/planner`.

## Interfaces / Contracts

```go
// internal/tui/router.go
type WizardStage int
const (StageWelcome WizardStage = iota; StageDetection; StageAgents; StagePersona; StagePreset; StageDepTree; StageSkillPicker; StageReview; StageInstalling; StageComplete)
var linearRoutes = map[WizardStage]Route // {Forward, Backward}
func NextStage(s WizardStage) (WizardStage, bool)
func PrevStage(s WizardStage) (WizardStage, bool)
func LegacyInstall() bool // os.Getenv("BIGGZ_LEGACY_INSTALL")=="1"
```

Messages mirror gentle (pure `Render*(state,cursor)` + router): `Update` handles `tea.KeyMsg` (enter=confirm, esc=back), `pipeline.ProgressEvent`, `progressDoneMsg`, `installResultMsg`. No new `tea.Msg`. Pickers persist via `opencode` config write.

Reduced motion: reuse `installing.go` helpers — disabled → tick nil + static; no-sync → strip `ESC[?2026h/l`; `TERM=dumb`/`BIGGZ_PRETTY=0` → `ansi.Strip`. No duplicates.

Banner gate: `grep -rn "RenderLogo\|Tagline\|updateBanner\|advisory" internal/tui/screens/wizard_* internal/tui/router.go` must be empty (test + review checklist).

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit (001) | Traversal, back-preserves, keyboard-only 10-stage | Drive `Update` with key msgs; assert stage+state |
| Unit (002) | Precedence agents>user>builtin; zero gentle tokens | `modelpickers_test.go` pattern; scan output |
| Unit (003) | Route order; reject jumps; legacy fallback | `router_test.go` table + `BIGGZ_LEGACY_INSTALL=1` |
| Unit (004) | No spinner/CSI2026 under NO_ANIMATION; zero ANSI dumb | Env per test; assert `View()` |
| Unit (005) | Banner grep clean | Grep allowlist; assert empty |
| Integ (006) | Lossless 0→100; close→Complete; fail→error+rollback | Fake plan + `RunWithChan(32)` → `waitProgress`; reuse `installing_test.go` |

## Threat Matrix

N/A — in-process Bubble Tea state + existing channel; no new exec, network, or privilege change.

## Migration / Rollout

No migration. Rollback: revert wizard commits or set `BIGGZ_LEGACY_INSTALL=1` for the lean 6-state flow.

## Open Questions

None — REQ-WIZ-001..006 answered, no scope expansion.
