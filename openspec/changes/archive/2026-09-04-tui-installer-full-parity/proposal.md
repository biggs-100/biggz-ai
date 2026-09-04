# Proposal: TUI Installer Full Parity (sans gentle banner)

## Intent

biggz-ai's installer (`Idle→Detect→Select→Review→Running→Done`, 30-char bar) feels sparse next to gentle-ai's guided wizard. Port the full visual wizard flow so first-run install is discoverable and consistent, without importing gentle-ai branding.

## Scope

### In Scope
- Linear wizard: Welcome → Detection → Agents multi-select → Persona → Preset → DependencyTree → SkillPicker → Review → Installing → Complete
- Per-agent model pickers (Claude/Codex/Kiro/OpenCode+Pi backgrounds) adapted to biggz styles
- Router `linearRoutes` + state machine extension in `internal/tui/screens/`
- Keep Pipeline Orchestrator (`RollbackOnFailure`, `ProgressChan(32)`), `BIGGZ_NO_ANIMATION` guards, Rose Pine palette

### Out of Scope
- gentle-ai banner: no `RenderLogo`/`Tagline`/`updateBanner`/advisory port
- Backend install logic changes (pipeline steps, asset manifests)
- Community tools / upgrade / sync screens; profile CRUD beyond installer needs

## Capabilities

### New Capabilities
- None (all behavior lands under existing spec domains)

### Modified Capabilities
- `tui`: wizard screens, router, reduced-motion compliance for new views
- `installer-pipeline`: progress events feed Installing view (100%-close contract unchanged)

## Approach

Port gentle-ai `screens/` wizard (~75 files, selective subset) into biggz-ai `internal/tui/screens/`: new files `welcome.go`, `detection.go`, `agents.go`, `persona.go`, `preset.go`, `dependency_tree.go`, `skill_picker.go`, `review.go`, `complete.go`, per-agent pickers; extend `install.go` state machine + `router.go` `linearRoutes`; wire `Installing` view to existing Orchestrator `ProgressChan(32)`. Restyle to `internal/tui/styles/styles.go`, strip banner calls, gate spinners on `BIGGZ_NO_ANIMATION`/`TERM=dumb`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/tui/screens/` | New/Modified | Wizard screens, `install.go`, `installing.go` |
| `internal/tui/router.go` | Modified | `linearRoutes` wizard order |
| `internal/tui/styles/` | Modified | Palette conformance only, no new tokens |
| `internal/pipeline/` | Modified | Progress wiring only, no API change |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Scope creep (porting all 75 files) | Med | Allowlist above; reject banner/community files in review |
| Picker/state divergence from pipeline | Low | Reuse Orchestrator + `ProgressChan(32)`; no parallel runner |
| Animation goldens break | Low | `BIGGZ_NO_ANIMATION=1` in tests; sync-output fallback |

## Rollback Plan

Revert wizard commits; `install.go` state machine falls back to `Idle→Detect→Select→Review→Running→Done`. No migration or persisted state to unwind. Feature-flag via env `BIGGZ_LEGACY_INSTALL=1` if partial rollout.

## Dependencies

- `installer-pipeline` Orchestrator + `ProgressChan` contract; `tui` styles/sync-output helpers

## Success Criteria

- [ ] Wizard traverses all 10 stages keyboard-only with `BIGGZ_NO_ANIMATION=1` clean
- [ ] No `RenderLogo`/`Tagline`/`updateBanner` references in ported code
- [ ] `go test ./internal/tui/... ./internal/pipeline/...` passes
