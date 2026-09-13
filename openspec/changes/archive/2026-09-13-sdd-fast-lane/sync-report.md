# Sync Report: sdd-fast-lane

**Fecha**: 2026-09-13
**Resultado**: `applied` — `sync applied for change sdd-fast-lane`
**Store**: `openspec` (file-backed) — `openspec/config.yaml` no declara `artifact_store`, por lo que `declaredArtifactStore` resuelve al default `openspec`; el camino de sync file-backed es idéntico al de `hybrid`.
**Método**: implementación propia del repositorio — `sdd.Sync()` (`internal/sdd/sync.go`) ejecutado a través de un overlay Go de solo lectura (test virtual en `%TEMP%`, cero archivos nuevos en el repo), que internamente usa `ParseDeltaSpec` + `ApplyDeltas` (`internal/sdd/openspec-deltas.go`) y las guardas de `internal/sdd/sync_helpers.go`.

> Nota: el precedente citado en el dispatch (`openspec/changes/archive/2026-09-12-wire-pi-review-relay/sync-report.md`) no existe en este repo — no hay ningún `sync-report.md` previo — así que este reporte sigue los campos exigidos por el dispatch.

## Domains synced

| Dominio | Requisito delta | Operación | Spec canónico |
|---------|-----------------|-----------|---------------|
| `sdd-status` | `Plan-Only Fast Lane Alias` (7 scenarios) | `ADDED` | `openspec/specs/sdd-status/spec.md` |
| `orchestrator` | `Fast Lane Runs Inside sdd-ff` (4 scenarios) | `ADDED` | `openspec/specs/orchestrator/spec.md` |

Ningún otro dominio fue tocado; ningún artifact del change fue modificado.

## Line delta

| Archivo | +/− | Líneas antes → después | Hunk |
|---------|-----|------------------------|------|
| `openspec/specs/sdd-status/spec.md` | **+46 / −0** | 250 → 296 | `@@ -248,3 +248,49 @@` |
| `openspec/specs/orchestrator/spec.md` | **+28 / −0** | 769 → 797 | `@@ -767,3 +767,31 @@` |
| **Total** | **+74 / −0** | 1019 → 1093 | `2 files changed, 74 insertions(+)` |

## Counts before → after

| Spec | Requisitos | Scenarios |
|------|------------|-----------|
| `sdd-status` | 12 → **13** (+1) | 33 → **40** (+7) |
| `orchestrator` | 33 → **34** (+1) | 113 → **117** (+4) |

El crecimiento equivale exactamente a los deltas (1 req + 7 scen; 1 req + 4 scen) y no hay headings duplicados (`grep -c` del nuevo `### Requirement:` = 1 en ambos specs). Se preservaron todos los requisitos y scenarios preexistentes (0 líneas borradas).

## Hashes

| Archivo | Antes (sha256) | Después (sha256) |
|---------|----------------|------------------|
| `openspec/specs/sdd-status/spec.md` | `7faf0e2de9a4ae55fd075570b0254efe0260344a8f50b590e0e5ecf0347edff4` | `5fb490ce7a3eee6837bcda8ee6ae2854da8ff087cb9a05febd6cc01a490d0265` |
| `openspec/specs/orchestrator/spec.md` | `147ba800e9e55f1eea46c51469e37b8bf7ddf37ab4a5a103d34dd00e49a6e848` | `c9d98da31d592913068ab90172a9c3f905ec6dfb81c003bc1eb79615c72809c5` |

Los hashes "después" son byte-idénticos a la predicción in-memory (`ApplyDeltas`) calculada ANTES de escribir, y un re-apply en memoria de los deltas sobre el spec ya sincronizado es no-op (`IDEMPOTENT true` en ambos dominios).

## Verification reference

- Verdict: `pass_with_warnings` — admisible como PASS por `checkVerifyVerdict` (`internal/sdd/verify.go:230`): `pass` y `pass_with_warnings` marcan `Passing=true`.
- Envelope: `requirements 2/2` · `scenarios 11/11` · `blockers 0` · `critical_findings 0` · `test_exit_code 0` · `build_exit_code 0` · `evidence_revision sha256:763ab5a06ebcc138781b54f83508713d7d0b42929a581e83ca6fad5ac94c6fdd`.
- Reporte: `openspec/changes/sdd-fast-lane/verify-report.md` · sha256 de archivo `05141185499905b4ebe27e31e0c47638f7269c8a05d7ffdfc4fc82c9da710aaa` (confirmado con `sha256sum` en esta corrida).

## Non-destructive note

Ambos deltas contienen únicamente `## ADDED Requirements`; no existen bloques `REMOVED` ni `MODIFIED`. La guarda `syncIsDestructive` evaluó `false` y `git diff --numstat` confirma `−0` líneas borradas: no se requirió (ni aplicó) `allow-destructive`. El maintainer aprobó el sync aditivo y el archive (checkpoint 2026-09-13).

## Guardrails (todos PASS)

| Guard | Resultado |
|-------|-----------|
| Store gate | `openspec` file-backed → sync aplica (no `engram`/`none`) |
| Verify PASS | `verifyResult.Passing = true` |
| RENAMED | `false` en ambos deltas |
| Legacy flat | `false` en ambos specs canónicos |
| Destructive | `false` (solo ADDED) |
| Collision | sin colisión — único change activo es `sdd-fast-lane` |

## Invariants

- `openspec/changes/sdd-fast-lane/` sigue existiendo — sin archive move.
- Sin commits: `HEAD` sigue en `d0cd87c6cb861a4ecfa3665ac48652498212cf74`.
- `git status --porcelain openspec/specs/` lista solo los dos specs canónicos: ` M openspec/specs/orchestrator/spec.md` y ` M openspec/specs/sdd-status/spec.md`.
- Post-sync `biggz sdd-status --json` reporta `nextRecommended: archive` (ya no `sync`), `applyState: all_done`, los 6 artifacts `done`, `route: sdd`. La proyección `blockedReasons` está ausente porque no hay blockers.

## Commands used to verify

```text
# 1. Estado previo (antes de tocar nada)
$ sha256sum openspec/specs/sdd-status/spec.md openspec/specs/orchestrator/spec.md
7faf0e2de9a4ae55fd075570b0254efe0260344a8f50b590e0e5ecf0347edff4 *openspec/specs/sdd-status/spec.md
147ba800e9e55f1eea46c51469e37b8bf7ddf37ab4a5a103d34dd00e49a6e848 *openspec/specs/orchestrator/spec.md
$ for f in ...; do grep -c '^### Requirement:' "$f"; grep -c '^#### Scenario:' "$f"; done
sdd-status: REQ=12 SCEN=33 · orchestrator: REQ=33 SCEN=113
$ git status --porcelain openspec/specs/
(vacío)

# 2. Dry-run read-only con el parser del repo (overlay Go; sin escrituras)
$ go test -overlay="%TEMP%/sdd-sync-fastlane/overlay.json" ./internal/sdd -run 'TestZZSyncOverlayDryRun' -count=1 -v
BEFORE sdd-status: sha256=7faf0e2d… lines=250 req=12 scen=33
DELTA sdd-status: deltas=1 hasRenamed=false legacyFlat=false ops=[ADDED:Plan-Only Fast Lane Alias]
PREDICTED sdd-status: sha256=5fb490ce… lines=296 req=13 scen=40
BEFORE orchestrator: sha256=147ba800… lines=769 req=33 scen=113
DELTA orchestrator: deltas=1 hasRenamed=false legacyFlat=false ops=[ADDED:Fast Lane Runs Inside sdd-ff]
PREDICTED orchestrator: sha256=c9d98da3… lines=797 req=34 scen=117
--- PASS

# 3. Aplicación con Sync() del repo (guards + ParseDeltaSpec + ApplyDeltas)
$ SYNC_APPLY=1 go test -overlay="%TEMP%/sdd-sync-fastlane/overlay.json" ./internal/sdd -run 'TestZZSyncOverlayApply' -count=1 -v
SYNC RESULT=applied MSG=sync applied for change sdd-fast-lane
AFTER sdd-status: sha256=5fb490ce… lines=296 req=13 scen=40
IDEMPOTENT sdd-status: true
AFTER orchestrator: sha256=c9d98da3… lines=797 req=34 scen=117
IDEMPOTENT orchestrator: true
CHANGE DIR EXISTS: true
--- PASS

# 4. Verificación independiente post-apply
$ sha256sum openspec/specs/sdd-status/spec.md openspec/specs/orchestrator/spec.md
5fb490ce7a3eee6837bcda8ee6ae2854da8ff087cb9a05febd6cc01a490d0265 *openspec/specs/sdd-status/spec.md
c9d98da31d592913068ab90172a9c3f905ec6dfb81c003bc1eb79615c72809c5 *openspec/specs/orchestrator/spec.md
$ git diff --numstat -- openspec/specs/
28      0       openspec/specs/orchestrator/spec.md
46      0       openspec/specs/sdd-status/spec.md
$ git diff --stat -- openspec/specs/
 2 files changed, 74 insertions(+)
$ git diff -- openspec/specs/ | grep '^@@'
@@ -767,3 +767,31 @@ (orchestrator)
@@ -248,3 +248,49 @@ (sdd-status)
$ grep -c "### Requirement: Plan-Only Fast Lane Alias" openspec/specs/sdd-status/spec.md   → 1
$ grep -c "### Requirement: Fast Lane Runs Inside sdd-ff" openspec/specs/orchestrator/spec.md → 1
$ git status --porcelain openspec/specs/
 M openspec/specs/orchestrator/spec.md
 M openspec/specs/sdd-status/spec.md
$ git log --oneline -1
d0cd87c6 Merge pull request #75 from biggs-100/fix/sdd-dx-followups   (sin commit nuevo)
$ ls openspec/changes/sdd-fast-lane/
_meta.yaml apply-progress.md design.md exploration.md preproposal.md proposal.md review-subject.json specs tasks.md verify-report.md

# 5. Routing post-sync
$ go run ./cmd/biggz sdd-status --json --cwd C:/Users/USER/Desktop/biggz-ai
name=sdd-fast-lane · nextRecommended=archive · applyState=all_done · artifacts{todo done} · route=sdd
```
