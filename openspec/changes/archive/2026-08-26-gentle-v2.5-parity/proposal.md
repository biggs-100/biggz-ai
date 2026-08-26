# Proposal: Gentle v2.5 Parity — Research, Status v2, Last-Event Closure

## Intent

biggz-ai trails gentle-ai v2.5.0-rc.1 (7afe50d1, 2026-08-26, "The lifecycle closes where proof ends"). Status is v1 with authority/bindings, review uses receipts + second decision, no Research lane, implicit intent, runtime missing grouped isolation/Codex hooks/Windows beta, TUI missing reduced-motion/Gentleman-Cute. v2.5 retires v1 explicitly; biggz-ai preserves silently. Close divergence.

## Scope

### In Scope
- Research: `biggz-ai.sdd-research-capability/v1` (`documentation`/`open-web` exact grant; Bash/generic MCP denied), auditable integrity, hybrid same-bytes, blocks `propose`.
- Review last-event: reviewer/refuter/validator/correction-plan/zero-lens burn lineage; compact receipts retired; delivery normal.
- Status v2: sole `biggz-ai.sdd-status/v2` without authority (planning/tasks/verification/attempts without bindings/receipts); rescope never inherits exhausted allowance; historic v1 rejected.
- Orchestrator: explicit intent required; investigate/conditional ≠ permission.
- Runtime/platform: OpenCode grouped isolation (scheduling only), Windows self-update beta fixes, Pi package progress/manifest parse, cooperative lock, Codex hooks → backup.
- TUI: reduced-motion + Gentleman-Cute refresh.

### Out of Scope
- Features outside v2.5-rc.1.
- Auto-migration of v1 artifacts.

## Capabilities

### New Capabilities
- `sdd-research`: admission, integrity, hybrid, gate.

### Modified Capabilities
- `sdd-status`: v1→v2 clean break.
- `review-lifecycle`: last-event, burn lineage, retire receipts.
- `orchestrator`: explicit intent.
- `runtime/platform`: isolation, Windows beta, Pi, lock, hooks.
- `tui`: reduced-motion + palette.

## Approach

Port contracts/prompts/skills from `C:\Users\USER\Desktop\herramientas\gentle-ai` v2.5.0-rc.1. Update `internal/sdd/status.go` to v2, `internal/review` to last-event, add `internal/sdd/research` lifecycle, harden orchestrator guard, port runtime (`internal/opencode`, `internal/platform`, `internal/update`, `internal/filemerge`, `internal/agents/pi`) and TUI. No DAG; hybrid same-bytes.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/status.go` | Modified | v2 projection, clean break |
| `internal/sdd/research*`, `internal/assets/skills/sdd-research/*` | New | Research lane |
| `internal/review/*` | Modified | Last-event closure |
| `internal/sdd/edit_authority*` | Modified | Explicit intent |
| `internal/opencode/*`, `internal/platform/*`, `internal/update/*`, `internal/agents/pi/*`, `internal/backup/*` | Modified | Runtime/platform |
| `internal/tui/*` | Modified | Reduced-motion + theme |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Breaking reviews (burn) | High | Ledger tests, no second decision |
| Status break | Med | Clean-break instruction |
| Time | Med | Auto-chain slices |

## Rollback Plan

Revert status to v1, restore compact receipts, disable Research lane. No migration.

## Dependencies

- gentle-ai v2.5.0-rc.1 at `C:\Users\USER\Desktop\herramientas\gentle-ai`.
- `2026-08-26-complexity-gates` archived.

## Success Criteria

- [ ] Research blocks `propose` until `done`; Bash/MCP denied; hybrid same-bytes.
- [ ] `biggz sdd-status --contract biggz-ai.sdd-status/v1` fails clean-break; v2 sole.
- [ ] Last-event burns lineage; receipts retired; no second decision.
- [ ] Rescope never inherits exhausted allowance; historic rejected.
- [ ] Explicit intent required; investigate/conditional not permission.
- [ ] Runtime: grouped isolation, Windows beta, Pi progress/manifest, lock, Codex hooks.
- [ ] TUI reduced-motion + Gentleman-Cute.
- [ ] `go test ./... -count=1 -timeout 180s` pass.
