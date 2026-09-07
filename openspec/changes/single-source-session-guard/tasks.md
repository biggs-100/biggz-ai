# Tasks: Single-Source Session Guard

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~70 (nuevo fichero ~57 + 4 ediciones de 1 línea + −57) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Move completo + verificación | PR 1 | `node --test internal/assets/pi/biggz-session-stop.test.mjs` | N/A (cambio estático JS + lista deploy; sin runtime nuevo) | Revert del commit: borrar `biggz-session-guard.js`, restaurar bloque e imports |

## Phase 1: Módulo guard (fundación)

- [x] 1.1 Crear `internal/assets/pi/biggz-session-guard.js` con el bloque movido verbatim (líneas 8–64 de `biggz-tool-interception.js`, incluido `import { execFileSync }` y comentarios APPLY-DECIDE). Verify: `node --check internal/assets/pi/biggz-session-guard.js` → sin errores.

## Phase 2: Re-export y repoints (núcleo)

- [x] 2.1 Editar `internal/assets/pi/biggz-tool-interception.js`: borrar líneas 8–64, insertar `export { checkSessionStop, SESSION_STOP_TIMEOUT_MS, _setSessionStopExecForTest } from "./biggz-session-guard.js";`. Verify: `rg "function checkSessionStop|_sessionStopExecForTest =" internal/assets/pi/biggz-tool-interception.js` → sin matches.
- [x] 2.2 Editar `internal/assets/pi/biggz-extension-api.js` línea 16: import → `./biggz-session-guard.js`; refrescar comentario líneas 11–15. Verify: `grep -n "checkSessionStop" internal/assets/pi/biggz-extension-api.js` → apunta a guard.
- [x] 2.3 Editar `internal/assets/pi/biggz-session-stop.test.mjs` líneas 5–10: import de los 3 símbolos → `./biggz-session-guard.js` (intención intacta). Verify: `grep -n "from './biggz" internal/assets/pi/biggz-session-stop.test.mjs` → guard.
- [x] 2.4 Editar `internal/install/steps/pi_extensions.go`: insertar `{"pi/biggz-session-guard.js", "biggz-session-guard.js"},` tras la línea 96 (después de `extension-api.js`). Verify: `grep -n "session-guard" internal/install/steps/pi_extensions.go` → línea 97.

## Phase 3: Verificación (contrato)

- [x] 3.1 Tests contrato: `node --test internal/assets/pi/biggz-session-stop.test.mjs` → 10/10 pass (cubre argv-array sin shell y degrade; threat-matrix preservado, sin RED nuevo).
- [x] 3.2 Definición única: `rg "function checkSessionStop|const SESSION_STOP_TIMEOUT_MS|function _setSessionStopExecForTest" internal/assets/pi/*.js` → solo el guard.
- [x] 3.3 Aciclicidad + build: `grep import internal/assets/pi/biggz-session-guard.js` → solo `node:child_process`; luego `go build ./...` → exit 0.
