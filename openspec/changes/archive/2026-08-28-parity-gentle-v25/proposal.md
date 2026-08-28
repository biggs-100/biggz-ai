# Proposal: parity-gentle-v25 — Alineación 6 invariantes fail-closed Gentle v2.5.0-rc.1

## Intent

Alinear biggz-ai 688bdab con gentle 3e2e8c24 v2.5.0-rc.1 en 6 invariantes fail-closed verificados solo por código (`docs/comparison-with-gentle.md` ignorado), sin superar 400 líneas por PR.

## Scope

### In Scope
- **I1 Presupuesto (BLOCKER)** `model/review.go`: `MaxFixRounds=3→1`, `MaxScopedValidations=5→1`.
- **I2 FixDelta (BLOCKER)** `internal/review/finalize.go`: `payloadSHA256` → `FixDeltaHashForSnapshot("fix-delta/v1\x00"+trees)` solo cumulative.
- **I3 Cadena (CRITICAL)** `model/hash.go`: `|` → `writeLengthPrefixed`+`\x00`+dominios.
- **I4 Store/lock (CRITICAL)** `store.go`→`GitCommonDir`, `lock.go` `O_EXCL`→`flock`.
- **I5 Burn (BLOCKER)** `finalize.go`: `os.Remove` → `BurnEnabled=false`/tombstone + `DeliveryBurned`.
- **I6 SDD v2 (CRITICAL)** `status_v2.go`+`edit_authority.go`: quitar `applyEditAuthorityBlock`, authority-free.
- 3 PRs `stacked-to-main` <400: P0=I1+I2, P1=I3+I4+I5, P2=I6. Tests parity + 21 gates.

### Out of Scope
- Rename docs, TUI/MCP, lenses R1-R4, `agent/*`, nuevo `ReviewGate`.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `core-review`: presupuesto hard-1 (3/5→1).
- `review`: `FixDeltaHashForSnapshot`, cadena domain+length-prefix, `GitCommonDir+flock`, `BurnEnabled`.
- `sdd-status`: V2 authority-free sin `applyEditAuthorityBlock`.

## Approach

Port verbatim gentle, helpers mínimos, P0→P2 stacked.

1. **P0** — const 1 + `FixDeltaHashForSnapshot`; `TestBudgetParity`, `TestFixDeltaBinding`.
2. **P1** — `writeLengthPrefixed`+dominios, `GitCommonDir` (dual-write ya en main), `flock`, `BurnEnabled`+tombstone.
3. **P2** — quitar bloque V2; `sdd-status` no filtra `allowedEditRoots`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `model/review.go` | Modified | `MaxFixRounds`/`MaxScopedValidations` |
| `model/fsm.go` | Modified | guard `<1` |
| `internal/review/finalize.go` | Modified | `FixDeltaHashForSnapshot`, `BurnEnabled` |
| `internal/review/receipt.go` | Modified | binding cumulative |
| `model/hash.go` | Modified | `writeLengthPrefixed`, `\x00` |
| `internal/review/store.go` | Modified | `GitCommonDir` |
| `internal/review/lock.go` | Modified | `flock` |
| `internal/sdd/status_v2.go` | Modified | quitar `applyEditAuthorityBlock` |
| `internal/sdd/edit_authority.go` | Modified | desacoplado V2 |
| `orchestrator.md` | Modified | docs burn/authority |
| `gate.js` | Modified | `DeliveryBurned` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Presupuesto 3→1 rompe 2-3 rounds | Medium | Fail-closed `Escalated` |
| Colisión hash sin prefix | Medium | `writeLengthPrefixed` verbatim |
| Worktree `GitCommonDir` split | Medium | Lector flat+worktree, reversible |
| Pérdida burn `os.Remove` | Low | Tombstone+flag |
| SDD bloquea edits | High | Validar solo en apply |

## Rollback Plan

Revert P2→P0 (`git revert`). `BurnEnabled` flag reversible. Store dual-lector sin pérdida. Temporal `MaxFixRounds=3` con WARN si P0 rompe linajes.

## Dependencies

- gentle 3e2e8c24 source (verbatim).
- Dual-write ya en 688bdab.
- `go test ./...`, `go vet`, 21 gates.

## Success Criteria

- [ ] `TestBudgetParity`: `1/1`; round2 rechazado.
- [ ] `TestFixDeltaBinding`: `FixDeltaHashForSnapshot("fix-delta/v1\x00"+trees)` ≠ flat.
- [ ] Cadena: vectors `\x00`+`writeLengthPrefixed` OK.
- [ ] Store/lock: `GitCommonDir`+`flock` verde.
- [ ] Burn: `BurnEnabled=false`, `DeliveryBurned` activo.
- [ ] SDD: `sdd-status v2` sin `allowedEditRoots`; 21 gates verdes.
- [ ] 3 PRs <400; `go test`+`go vet` verdes.
