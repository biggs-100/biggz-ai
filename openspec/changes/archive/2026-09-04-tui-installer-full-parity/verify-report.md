```yaml
schema: biggz-ai.verify-result/v1
verdict: pass
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 13/13
test_exit_code: 0
build_exit_code: 0
```

# Verify Report: tui-installer-full-parity

## Status
- **Verdict:** PASS
- **Date (UTC):** 2026-09-04
- **Tasks:** 16/16 checked, 0 unchecked
- **Ledger:** settled passed and complete — no active attempt; verification ran against closed evidence (per task brief)

## Proposal success criteria

| Criterion | Result | Evidence |
|-----------|--------|----------|
| Wizard traverses all 10 stages keyboard-only with `BIGGZ_NO_ANIMATION=1` clean | PASS | `TestWizardTraversalFull`, `TestWizardTraversalBatch1` PASS (`go test ./internal/tui/screens/ -run TestWizard -v`) |
| Zero `RenderLogo`/`Tagline`/`updateBanner`/`advisory` in ported code | PASS | `grep -rn "RenderLogo\|Tagline\|updateBanner\|advisory" internal/tui/screens/wizard_* internal/tui/screens/*_picker.go internal/tui/router.go` → empty (exit 1); `TestWizardBannerGrepClean` PASS |
| `go test ./internal/tui/... ./internal/pipeline/...` passes | PASS | `go test ./internal/tui/... ./internal/pipeline/... -count=1` → ok all 4 packages, exit 0 |

## Spec compliance (tui)

| Requirement | Verdict | Covering test(s) | Evidence |
|-------------|---------|------------------|----------|
| REQ-WIZ-001 — Wizard stage traversal (forward / backward / keyboard-only 10-stage) | PASS | `TestWizardTraversalFull`, `TestWizardTraversalBatch1`, `TestWizardBatch1ViewsGuards`, `TestWizardBatch2ViewsGuards` | All PASS 2026-09-04; forward advance preserves selections, esc returns N-1 intact |
| REQ-WIZ-002 — Per-agent pickers adapted (precedence + biggz styling) | PASS | `TestPickerPrecedence_AgentsOverUserOverBuiltin`, `TestPickerPrecedence_EmptyAgentsDefersToUser`, `TestPickerBackgrounds_Distinct`, `TestPickerViews_ZeroGentleTokens`, `TestPickerSources_NoGentleImports` | All PASS; agents > user > builtin confirmed; zero gentle tokens |
| REQ-WIZ-003 — Router linearRoutes (order, jump rejection, legacy fallback) | PASS | `TestNextPrev`, `TestRouterRejectsJumps`, `TestLegacyInstallFallback` | All PASS; Detection→Agents order; `BIGGZ_LEGACY_INSTALL=1` → lean 6-state flow |
| REQ-WIZ-004 — Reduced-motion compliance | PASS | `TestWizardReducedMotion` (4 subtests: tick nil NO_ANIMATION, tick nil gentle flag, no spinner/CSI2026, zero ANSI TERM=dumb) | All PASS |
| REQ-WIZ-005 — Zero banner references | PASS | `TestWizardBannerGrepClean` + manual grep (empty) | PASS; code-scoped banner gate clean |

## Spec compliance (installer-pipeline)

| Requirement | Verdict | Covering test(s) | Evidence |
|-------------|---------|------------------|----------|
| REQ-WIZ-006 — Progress-channel wiring (lossless 0→100, close→Complete, failure→error, RollbackOnFailure unchanged) | PASS | `TestWizardProgressWiring/lossless_0→100`, `TestWizardReviewConfirmWiring` | Both PASS; `RunWithChan(32)` 10 events lossless, close→Complete |

## Design coherence

| Design decision | Implemented | Evidence |
|-----------------|-------------|----------|
| Extend `install.go` enum (`stepWizWelcome…Complete`), gate on `BIGGZ_LEGACY_INSTALL` | Yes | `internal/tui/screens/install.go` (+310/-1); `TestLegacyInstallFallback` PASS |
| New `internal/tui/router.go`: `WizardStage` + `linearRoutes` + `Next/Prev` | Yes | `internal/tui/router.go` exists; `TestNextPrev`, `TestRouterRejectsJumps` PASS |
| Pure `Render*` + state in `InstallModel`; `InstallingModel` stays real model | Yes | `wizard_welcome|detection|agents|persona|preset|deptree|skills|review|complete.go` present; wiring via `runOrchestratorCmd` + `waitProgress(ch)` |
| Thin picker wrappers over `opencode.MergeModelConfigs` + styles only | Yes | `claude|codex|kiro|opencode_picker.go` present; `TestPickerViews_ZeroGentleTokens` PASS |
| Keep 30-char bar; step-line from `ProgressEvent.Step` | Yes | No bar-golden breakage; pipeline tests ok |
| NOT ported: banner/community/upgrade/sync/profile-CRUD, gentle catalog/model/planner | Honored | Banner grep empty; no new style tokens |

## Task completion

- `openspec/changes/tui-installer-full-parity/tasks.md`: **16 checked / 0 unchecked** (`grep -c "^- \[ \]"` → 0, `grep -c "^- \[x\]"` → 16).
- Phases 1–4 all checked (router+enum, core screens, pickers+wiring, guards+verification).

## Commands run

| Command | Exit | Summary |
|---------|------|---------|
| `go test ./internal/tui/... ./internal/pipeline/... -count=1` | 0 | ok `tui`, `tui/screens`, `tui/styles`, `pipeline` |
| `go test ./internal/tui/screens/ -run TestWizard -v -count=1` | 0 | 8/8 wizard tests PASS incl. traversal, motion, wiring, banner |
| `go test ./internal/tui/... -run "TestWizard\|TestNextPrev\|TestPicker\|TestMotion\|TestBanner\|TestProgress\|TestWiring\|TestRouter\|TestLegacy" -v` | 0 | all targeted tests PASS |
| `grep -rn "RenderLogo\|Tagline\|updateBanner\|advisory" internal/tui/screens/wizard_* internal/tui/screens/*_picker.go internal/tui/router.go` | 1 (no matches) | empty = required clean state |
| `grep -c unchecked tasks.md` / `grep -c checked tasks.md` | 0 / 16 | 16/16 complete |

## Issues

- CRITICAL: none.
- WARNING: none.
- SUGGESTION: none.

## Final verdict

**PASS** — all 6 requirements (REQ-WIZ-001..006) have passing covering tests; proposal success criteria met; design implemented without deviation; 16/16 tasks complete.
