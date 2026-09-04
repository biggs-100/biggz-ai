# Design: Organic Routing Parity

## Technical Approach

Port gentle-ai's organic routing into biggz-ai via prompt-level guidance changes (2 .md files) and 2 additive Go fields (`Route` in `ChangeStatus`, route context in `sdd-continue`). The routing logic is prompt-driven (orchestrator decides), not FSM-enforced. The Go changes are additive (new optional field) and non-breaking.

## Architecture Decisions

### ADR-001: Prompt-Only Routing Logic

**Decision:** Routing thresholds (1-3 / 4+ files) and route selection live in orchestrator guidance prompts, NOT in Go FSM.

**Rationale:** 
- gentle-ai does the same: `RenderRouting` in `routing.go` produces guidance prose injected into agent prompts. No FSM enforcement.
- Adding FSM routing would require touching `deriveChangeStatus`, `resolveNextRecommended`, and `status_v2.go` — high complexity for a behavioral change that belongs in prompt discipline.
- Prompt-level is fully reversible: revert the .md files and routing disappears.

**Risk:** Agent may ignore guidance. Mitigated by: orchestrator is the sole routing decision-maker, and synthesis/checkpoint enforcement still applies.

### ADR-002: Route Field is Additive, Not Replacement

**Decision:** Add `Route string` field to `ChangeStatus` struct. Do NOT remove or rename existing fields.

**Rationale:**
- `NextRecommended` already encodes SDD phase progression. `Route` adds a higher-level classification.
- For direct/delegated work, `NextRecommended` is empty (no SDD next). `Route` still reports what happened.
- Backward compatible: `Route` is `omitempty`, old consumers ignore unknown fields.

### ADR-003: Public States Replace Synthesis Markers

**Decision:** Replace `◆ phase·status·next` lifecycle markers with Working/Checking/Ready/Needs your decision.

**Rationale:**
- Current markers are SDD-phase-centric. Public states are route-agnostic.
- `Needs your decision` maps to current `blocked(blockedReasons)` + checkpoint ask.
- `Ready` maps to current `ApplyAllDone` + verify PASS.
- `Working` maps to any active implementation.
- `Checking` maps to test/review in progress.

**Risk:** External tooling parsing `◆` markers breaks. Mitigated by: no known external consumers; synthesis gate (`synthesis_gate.go`) already documented as passthrough since 2026-09-04.

## File Changes

### 1. `internal/assets/biggz/biggz-orchestrator-delegation.md`

**Change:** Rewrite Work Routing Ladder section with gentle thresholds.

Current (excerpt):
```
1. Inline Direct — typo, 1 file
2. Simple Delegation — generic non-SDD
3. SDD (optional)
```

New (excerpt):
```
| Route | When | What happens |
|---|---|---|
| Direct inline | 1–3 files to understand, or one mechanical file | Inline edit, no artifacts |
| Delegated direct | 4+ files to understand, or 2+ non-trivial writes | One scout/writer, no SDD artifacts |
| Optional SDD | Substantial ambiguity + explicit request or accepted proposal | SDD planning phases |
```

Plus: `SIZE NEVER SELECTS SDD`, `Per-action delegation does not change route`, `Direct/delegated create zero SDD artifacts`.

### 2. `internal/assets/biggz/biggz-orchestrator-workflow.md`

**Change:** Add Public States section, replace synthesis markers with state strings.

Add:
```
## Public Implementation States

| State | Meaning |
|---|---|
| Working | Implementation can still change |
| Checking | Functional proof and bounded review in progress |
| Ready | Exact candidate has sufficient evidence for delivery |
| Needs your decision | Safe convergence impossible; presents cause + choices |
```

Replace synthesis lifecycle `◆ phase·status·next` with state strings in checkpoint output.

### 3. `internal/sdd/status.go`

**Change:** Add `Route` field to `ChangeStatus` struct (line ~178).

```go
Route string `json:"route,omitempty"`
```

**Change:** Add route derivation in `deriveChangeStatusWithForcedStoreCtx` (after line ~537).

```go
cs.Route = deriveRoute(cs)
```

**New function:**
```go
func deriveRoute(cs *ChangeStatus) string {
    if cs.IsArchived { return "" }
    if cs.HasSpecs || cs.HasDesign || cs.HasTasks || cs.HasApply {
        return "sdd"
    }
    // No SDD artifacts = organic route
    // Orchestrator prompt determines direct vs delegated;
    // CLI cannot distinguish without artifact evidence.
    // Report "organic" when no SDD artifacts exist.
    return "organic"
}
```

**Note:** CLI cannot distinguish direct-inline from delegated-direct (both have zero SDD artifacts). The route field reports `sdd` vs `organic`. The orchestrator prompt knows which organic sub-route was used and reports it in synthesis. This is a deliberate limitation: FSM-level direct/delegated classification would require tracking context file counts in Go, which belongs in prompt logic.

### 4. `cmd/biggz/cli_sdd.go`

**Change:** Add route context to `sdd-continue` output (after line ~nextRecommended).

```go
route := deriveRoute(&active[0])
if route != "" {
    fmt.Fprintf(w, "Route: %s\n", route)
}
```

## Data Flow

```
User request
  → Orchestrator prompt reads delegation.md (routing ladder)
  → Orchestrator counts context files (1-3 / 4+)
  → Routes: direct | delegated | SDD
  → If SDD: standard SDD phases (artifacts created)
  → If organic: inline edit or delegated writer (no artifacts)
  → sdd-status --json includes Route field (sdd | organic)
  → Synthesis uses Working/Checking/Ready/Needs your decision
```

## Threat Matrix

Not applicable. This change modifies prompt guidance and adds an additive JSON field. No shell commands, subprocesses, VCS automation, or executable classification changes.

## Interfaces

### ChangeStatus (Go struct, additive)

```go
// New field — omitempty, backward compatible
Route string `json:"route,omitempty"`
```

### sdd-status --json output (additive)

```json
{
  "schemaName": "biggz-ai.sdd-status",
  "route": "sdd",
  "nextRecommended": "design",
  ...
}
```

For organic work:
```json
{
  "schemaName": "biggz-ai.sdd-status",
  "route": "organic",
  "nextRecommended": "",
  ...
}
```

### sdd-continue output (additive)

```
Route: organic
No SDD next — work completed via direct/delegated route.
```

Or:
```
Route: sdd
Next: design
```

## Implementation Order

1. `biggz-orchestrator-delegation.md` — routing ladder rewrite
2. `biggz-orchestrator-workflow.md` — public states + marker replacement
3. `internal/sdd/status.go` — Route field + deriveRoute function
4. `cmd/biggz/cli_sdd.go` — route context in sdd-continue output
5. Test: `go build ./...` + `go vet ./...` + manual `sdd-status --json`

## Rollback

- Prompt changes: revert .md files.
- Go changes: remove `Route` field from struct (additive, zero cost).
- No data migration needed.
