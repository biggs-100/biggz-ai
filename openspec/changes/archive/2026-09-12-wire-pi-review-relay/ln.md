schema: biggz-ai.sdd-ln/v1
revision: 1
change: wire-pi-review-relay
phase: research
lane: upstream-relay-design
outcome: done
artifact_store: hybrid

# Lifecycle record — wire-pi-review-relay / research

## Selected request (retained before source access)

Lane: **upstream relay design** — cómo el producto TypeScript `Gentleman-Programming/gentle-pi` hostea sus lentes de review (spawn/host, prompt binding, transporte, timeouts, manejo de procesos Windows) y la evidencia dura detrás del síntoma de relay con stdout vacío en Windows (#943 y clúster #941, #940, #937, #936, #935, #926, #924, #893, #871), con evaluación de transferencia a nuestro diseño Go (CLI ejecuta al reviewer vía `PiAdapter`, rol embebido por el CLI, timeout acotado, stdout capturado).

## Admission and grants

- `biggz-ai.sdd-research-capability/v1` — admitido.
- Grants observados: `documentation` (read/grep/find/ls) y `open-web` (web_search/web_fetch).
- Denegaciones: ninguna. Fallbacks de proveedor web: no usados (fetches servidos por T1).

## Execution summary

- Fuentes primarias: snapshot local `C:\Users\USER\AppData\Local\Temp\gentle-pi-main` (código y contratos de `upstream/main`) leído directamente.
- Fuentes web: 12 páginas de issues de GitHub (S15–S26) + 1 fetch crudo `raw.githubusercontent.com` (S14) + 4 fuentes secundarias Go/Windows vía búsqueda (S27–S30). `accessed_at` registrados por fuente.
- Claims validados C1–C33 mapeados a source ids; contradicciones, incertidumbre y frescura explícitas en `research.md`.
- El sub-agente opera con toolset **read-only** (sin escritores de archivos, sin BigMem): los bytes exactos se devuelven inline y la persistencia queda a cargo del orquestador.

## Persistence targets (orchestrator)

- `openspec/changes/wire-pi-review-relay/research.md` ↔ BigMem `sdd/wire-pi-review-relay/research` — bytes idénticos, `revision: 1`, `outcome: done`.
- `openspec/changes/wire-pi-review-relay/ln.md` ↔ BigMem `sdd/wire-pi-review-relay/ln` — bytes idénticos, `revision: 1`.
- Modo híbrido: exigir lectura de ambos stores y verificar revisión y bytes iguales antes de readiness.

## Handoff

- Preproposal rev 2 sigue `proposal_ready: false`; con este research `done` y decisiones `confirmed`, el orquestador actualiza `biggz-ai.sdd-preproposal/v1` (referencia de evidencia) al persistir.
- Bloqueos propios: ninguno. Sin escrituras realizadas por el sub-agente (read-only por diseño).
