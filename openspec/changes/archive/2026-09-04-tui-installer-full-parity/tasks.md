# Tasks: TUI Installer Full Parity

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1500–2200 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 router+enum → PR2 screens batch 1 → PR3 screens batch 2 → PR4 pickers → PR5 wiring+guards |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Router + enum extension + legacy flag | PR1 | `go test ./internal/tui/ -run TestNextPrev -v` | N/A (pure routing, no runtime) | Revert `router.go` + enum block in `install.go` |
| 2 | Welcome/Detection/Agents/Persona/Preset renders | PR2 | `go test ./internal/tui/screens/ -run TestWizardTraversal -v` | `BIGGZ_NO_ANIMATION=1 go run ./cmd/biggz-ai install` keyboard walk | Delete `wizard_welcome|detection|agents|persona|preset.go` |
| 3 | DepTree/Skills/Review/Complete renders | PR3 | `go test ./internal/tui/screens/ -run TestWizardScreens -v` | Same keyboard walk from Preset → Complete | Delete `wizard_deptree|skills|review|complete.go` |
| 4 | Per-agent pickers w/ precedence | PR4 | `go test ./internal/tui/screens/ -run TestPickerPrecedence -v` | Pick model per agent in wizard, verify config write | Delete 4 `*_picker.go` files |
| 5 | Installing wiring + motion/banner guards | PR5 | `go test ./internal/tui/... ./internal/pipeline/...` | Full install to Done + `TERM=dumb` render check | Revert `install.go` wiring + `installing.go` guard use |

## Phase 1: Router + State Foundation

- [x] 1.1 RED: create `internal/tui/router_test.go` — `Next/Prev` order, jump rejection, legacy fallback
- [x] 1.2 Create `internal/tui/router.go` — `WizardStage`, `linearRoutes`, `NextStage/PrevStage`, `LegacyInstall()`
- [x] 1.3 Extend `internal/tui/screens/install.go` step enum (`stepWizWelcome…Complete`) + fields + `BIGGZ_LEGACY_INSTALL=1` fallback

## Phase 2: Core Screens

- [x] 2.1 RED (complete, PR3): full 10-stage keyboard-only traversal — `wizard_traversal_test.go` drives `Update` with key msgs Welcome→Complete forward/back, state preserved, custom-preset DepTree picker covered
- [x] 2.2 Created `wizard_welcome.go`, `wizard_detection.go` (`RenderWizardWelcome`, `RenderWizardDetection`)
- [x] 2.3 Created `wizard_agents.go`, `wizard_persona.go`, `wizard_preset.go` (multi-select, radios)
- [x] 2.4 Created `wizard_deptree.go` (read-only plan list; checkbox picker when Custom), `wizard_skills.go` (assets skill multi-select), `wizard_review.go` (Agents+Persona+Preset+Skills summary), `wizard_complete.go` (Done-view fields) (PR3)
- [x] 2.5 Wired `install.go` Update (enter=confirm, esc=back) + View dispatch for all 10 stages; Review confirm arms Installing model, Installing confirm reaches Complete (live progress wiring stays in PR5/3.4) (PR3)

## Phase 3: Pickers + Wiring

- [x] 3.1 RED: picker precedence test — agents > user > builtin; assert zero gentle tokens in output
- [x] 3.2 Create `claude_picker.go`, `codex_picker.go`, `kiro_picker.go`, `opencode_picker.go` over `opencode.MergeModelConfigs`
- [x] 3.3 RED: progress wiring test — fake plan via `RunWithChan(32)`, 10 events 0→100 lossless, close→Complete, fail→error (`wiring_test.go`, PR5)
- [x] 3.4 Wire Review confirm → `runOrchestratorCmd` + `waitProgress(ch)` → `InstallingModel.Update` → Complete (`install.go` updateWizard, PR5)

## Phase 4: Guards + Verification

- [x] 4.1 RED: reduced-motion test — `BIGGZ_NO_ANIMATION=1` no spinner/CSI2026; `TERM=dumb` zero ANSI (`motion_test.go`, PR5)
- [x] 4.2 Apply motion guards in new views (tick nil, strip `ESC[?2026h/l`, `ansi.Strip`); no new style tokens (`wizardGuardView` + `installing.go` helpers reused, PR5)
- [x] 4.3 RED: banner-grep test — assert zero `RenderLogo|Tagline|updateBanner|advisory` in `wizard_*` + `router.go` (`banner_test.go` code-scoped, PR5)
- [x] 4.4 Run `go test ./internal/tui/... ./internal/pipeline/...` + keyboard-only walk with `BIGGZ_NO_ANIMATION=1` (green, PR5)
