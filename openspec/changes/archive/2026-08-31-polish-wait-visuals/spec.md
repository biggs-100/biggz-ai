# Spec pointer: polish-wait-visuals (B intermedia)

Polish wait visuals — cut noise -70%, keep diagnostics. Four delta specs, 14 requirements, 31 scenarios.

## Domains

| Domain | File | Requirements | Scenarios |
|--------|------|--------------|-----------|
| tui-sanitize | `specs/tui-sanitize/spec.md` | POLISH-TS-01..03 | 7 |
| tui | `specs/tui/spec.md` | POLISH-TUI-01..07 | 14 |
| orchestrator | `specs/orchestrator/spec.md` | POLISH-ORCH-01..02 | 5 |
| pi-integration | `specs/pi-integration/spec.md` | POLISH-PI-01..02 | 5 |

## Coverage map

| # | Brief | Domain | ID |
|---|-------|--------|----|
| 1 | Tokens compactos 4.1k›2.2k hide window | tui-sanitize + orchestrator | POLISH-TS-01, POLISH-ORCH-01 |
| 2 | Columnas fijas 5c+10c right never cut | tui-sanitize, orchestrator | POLISH-TS-02, POLISH-ORCH-01 |
| 3 | Layout 2 líneas/fila | tui | POLISH-TUI-01 |
| 4 | Workflow 2 líneas │ | tui | POLISH-TUI-02 |
| 5 | Cabecera 2 grupos | tui | POLISH-TUI-03 |
| 6 | Panes colapsables | tui | POLISH-TUI-04 |
| 7 | visibleWorkflowRows al final | tui | POLISH-TUI-05 |
| 8 | Throttle 3s headline ≤2 líneas | orchestrator+pi-integration | POLISH-ORCH-02, POLISH-PI-01 |
| 9 | Colores unified | tui | POLISH-TUI-06 |
| 10 | Truncation estable CJK2 | tui-sanitize | POLISH-TS-03 |
| 11 | Estabilidad no jitter | tui | POLISH-TUI-07 |
| 12 | ADR upstream shim+PR | pi-integration | POLISH-PI-02 |

## Quick path

1. Read `specs/tui-sanitize/spec.md` → sanitize width/token invariants
2. Read `specs/tui/spec.md` → 2-line + header + panes + stability
3. Read `specs/orchestrator/spec.md` → table + headline contract
4. Read `specs/pi-integration/spec.md` → shim throttle + ADR

## Verification

- `go vet` PASS, `node --test biggz-synthesis-gate` 22/22
- TUI width goldens 80→120c stable, 60c narrow no right cut
- Wait ≤2 líneas 3 steps, no scroll
