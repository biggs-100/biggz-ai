# Proposal: wire-pi-review-relay

## Intent

`capture-result --agent pi` valida el handshake pero nunca ejecuta al reviewer (`PiAdapter` sin caller; `cli_review.go:1060-1065`). Cerrarlo: el CLI ejecuta al reviewer y captura bytes crudos.

## Scope

### In Scope
- `--execute` en `review capture-result --agent pi`, exclusivo con `--input`/`--preflight`/`--materialize`.
- Pipeline `Preflight → MaterializeReviewerTask → PiAdapter.Review → Capture`; admission/CAS intactos.
- El CLI compone: rol (`internal/assets/prompts/review/*.md`) + task materializada, vía stdin.
- Timeout acotado (10 min, `--timeout`); cancelación mata; fallos tipados sin captura (vacío, timeout, exit≠0).
- Tests: unit seams, CLI shim `pi`, regresiones `review_materialize_test`/`capture_test`/`PluginWiresCapture`.

### Out of Scope
- Launcher host pi-side, ruta opencode/plugin, transportes alternativos.
- Admission, CAS, receipts, ledger: sin cambios.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `review`: ADD modo execute (exclusividad, pipeline, rol embebido, timeout/cancelación, fallos tipados con captura cero, stdout crudo a `Capture`); MODIFY "Verbatim Transport and Tool-Less Reviewer": verbatim anclado al segmento materializado (prompt binary-owned, nunca caller-authored).

## Approach

`--execute` reutiliza la cadena con handshake (`pi_relay.go:82-87`) y kill-switch RDD (`cli_review.go:1068`); la task viaja byte-idéntica y el stdout crudo llega a `Capture` (`pi_adapter.go:75-77`).

### Divergence (documented)

Upstream compone CERO rol host-side (`C5`/`C6`) y acota por escala: 900 s + 900 s/MiB, techo 2 h (`C12`). Elegimos rol embebido (única fuente binary-owned in-tree) y default fijo 10 min overridable; aceptable: el ejecutor es nuestro CLI, no una extensión host.

### Open questions (design)

(a) orden/contenido rol+task; (b) regla Windows del shim `pi` (`C33`) + smoke real; (c) default 10 min vs 478 s/1.58 MB; (d) diagnósticos con cero bytes (`#943`).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/biggz/cli_review.go` | Modified | Ruteo `--execute` |
| `internal/review/pi_adapter.go` | Modified | Deadline/seams |
| `internal/review/` (nuevo) | New | Rol+task, timeout |
| Tests `internal/review`, `cmd/biggz` | New/Modified | Seams, shim `pi` |
| `openspec/specs/review/spec.md` | Modified | Delta |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Run de modelo largo bloquea el verbo | High | Default + `--timeout`; kill |
| Shim `.cmd` Windows (`C33`) | Med | (b) + smoke |
| stdout sin cap pre-`Capture` | Med | Cap en ruta nueva |
| Reviewer no emite JSON estricto | Med | El rol decide; cero captura |

## Rollback Plan

Revertir el commit: `--execute` aditivo; `--input`/`--preflight`/`--materialize` intactos; handshake y plugin opencode sin tocar.

## Dependencies

- `pi` en PATH (shim npm); `BIGGZ_PI_REVIEW_RELAY_CONTRACT`; RDD habilitado.

## Success Criteria

- [ ] Unit: prompt == `MaterializeReviewerTask` byte a byte; stdout crudo a `Capture`.
- [ ] CLI shim `pi`: `start → execute → finalize → gate`.
- [ ] Timeout: fallo tipado, cero captura.
- [ ] Regresiones: `review_materialize_test`, `capture_test`, `PluginWiresCapture`.
