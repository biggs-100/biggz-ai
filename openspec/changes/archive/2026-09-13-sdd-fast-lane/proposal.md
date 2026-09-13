# Proposal: sdd-fast-lane

## Intent

Hoy el pipeline SDD tiene una sola profundidad: un fix de 40 líneas y una migración de protocolo recorren las diez fases (explore→archive). Coste del último cambio: ~15 sub-agent runs, 14 artifacts, ~1.5k líneas markdown para ~1.1k de código. Propuesta: profundidad según tamaño/riesgo con el mecanismo existente — lane inferida del artifact presente (`exploration.md`).

## Scope

### In Scope
- **Alias `plan.md`**: documento fusionado (intent, requisitos con scenarios, approach, checklist) mapeado a proposal/specs/design/tasks en `resolveArtifactPaths` (`internal/sdd/status.go:994`) y `bigmemArtifactPaths` (`internal/sdd/engram_status.go:168`); precedencia por-slot (artifact real gana); headings `### Requirement:`/`#### Scenario:` para verify (`internal/sdd/verify.go:121,188`).
- **Modo lane en `sdd-ff`** (`internal/assets/prompts/sdd/sdd-ff.md`): sin comando, overlay ni entrada AGENTS.md.
- **Texto**: `internal/sdd/instructions.go:19,25`, `internal/assets/skills/_shared/sdd-status-contract.md:223`, `internal/assets/biggz/biggz-orchestrator-workflow.md:338`.
- **Tests**: lane (plan-only→apply→verify→archive; precedencia; legado), paridad BigMem, dogfood CLI (`cmd/biggz/sdd_status_cli_test.go`).

### Out of Scope
- Sin meta-command, parser `_meta.yaml`, persistencia de `sdd-route` (`internal/sdd/route.go:93`, advisory), cambios a counts de verify (`internal/sdd/verify.go:188,197,201`) ni tokens `nextRecommended`.
- **Gates intactos**: RDD receipt (`internal/sdd/status.go:942`), `sdd-verify-validate` (`cmd/biggz/cli_sdd.go:351`), workload guard (`internal/sdd/workload_guard.go:121`), session-summary (`internal/sdd/session_guard.go:415`), edit authority (`internal/sdd/edit_authority.go:407`); apply/verify/archive obligatorios.

## Capabilities

### New Capabilities
None — reutiliza artifacts y tokens existentes.

### Modified Capabilities
- `sdd-status`: alias `plan.md` por-slot en `resolveArtifactPaths`/`bigmemArtifactPaths`; instrucciones apply/verify lane-aware.
- `orchestrator`: modo lane en `sdd-ff`; los gates nunca se saltan.

## Approach

Approach 1 (alias-inference): `resolveArtifactPaths` (`internal/sdd/status.go:994`) ofrece `plan.md` como candidato de los cuatro slots, tras cada artifact real; `collectArtifactDerivation`, `coreReady` (`internal/sdd/status.go:868`) y `resolveDependencies` (`internal/sdd/status.go:1161`) sin cambios dan apply/verify/archive. BigMem replica (`internal/sdd/engram_status.go:168,271`). Promoción mid-flight: un artifact real gradua sin migración.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/status.go`, `internal/sdd/instructions.go` | Modified | Alias `:994`; apply/verify lane-aware |
| `internal/sdd/engram_status.go` | Modified | Paridad BigMem |
| `internal/assets/prompts/sdd/sdd-ff.md`, `internal/assets/skills/_shared/sdd-status-contract.md`, `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | Modo lane; enseñanza |
| `internal/sdd/status_lane_test.go`, `cmd/biggz/sdd_status_cli_test.go` | New | Lane, paridad, dogfood |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| `internal/sdd/engram_status.go:271` duplicado → lanes divergentes | Med | Alias en ambos stores + test |
| `plan.md` stale/scratch sombrea artifacts | Med | Precedencia + test |
| Sin headings canónicos, verify falla counts | Med | Modo lane lo exige |
| Renderers legacy; "Never skip phases" (`:338`) | Low | Aceptado; enmendar `:338` |

## Rollback Plan

Revertir el alias restaura el pipeline único sin migración: `plan.md` deja de leerse y el dispatcher vuelve a exigir cuatro artifacts.

## Dependencies

None — mecanismo íntegro en el repo (`exploration.md`).

## Success Criteria

- [ ] Fixture plan.md-only llega a apply/verify/archive vía dispatcher real.
- [ ] Paridad BigMem/filesystem cubierta por test.
- [ ] Legacy de cuatro artifacts intacto (suite verde).
- [ ] Apply/verify dejan de exigir cuatro artifacts.
- [ ] Dogfood en cambio real pequeño.
