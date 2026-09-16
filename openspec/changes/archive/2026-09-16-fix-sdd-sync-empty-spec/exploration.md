# Exploration: fix-sdd-sync-empty-spec

## Problem Statement

The native openspec sync write path (`internal/sdd/sync_helpers.go:180` `syncApplyDeltas`) calls
`ApplyDeltas(mainContent, info.deltas)` (`:189`) and then writes the result unconditionally
(`os.WriteFile`, `:204`). A NEW domain whose delta file is **full-spec shaped** (`# X Specification`
+ `## Purpose` + `## Requirements`, no delta section) parses to **zero deltas**, and
`ApplyDeltas("", [])` returns `""` (`internal/sdd/openspec-deltas.go:70-73`), so the write path
CREATES an **empty** living spec under a SUCCESS result (`applied`) — a silent false-green in the
SDD pipeline.

Incident: `fix-false-green-guards` (archive `2026-09-16`) — domains `ci-guard-integrity` and
`ci-guard-baseline` were full-spec shaped; the sync phase worked around it with a manual verbatim
copy (recorded as defect F1 in `openspec/changes/archive/2026-09-16-fix-false-green-guards/state.yaml`
→ `discovered_defects` → `sdd-sync-discards-full-spec-shaped-new-domain`, severity high).

## Reproducer

**Status: NOT EXECUTED in the explore session.** The explore session had a read-only toolset
(no shell, no write tools), so the reproduction was prepared as an executable runbook and could
not be run here. All per-case outcomes below are **predicted from a line-level trace**,
corroborated by these previously OBSERVED runs recorded in the archive trail:

- F1 (state.yaml): "ApplyDeltas(\"\", []) returns \"\" on a file with no \"## ADDED Requirements\"
  section, so it would have created empty main specs" — observed during the incident.
- `archive/rdd-auto-enabled-post-verify/archive-report.md:36,278` (case b, OBSERVED): new domain,
  ADDED-shaped delta, empty main → "rdd main 0 → created 4 ADDED → 62 lines 4 req 2007 bytes".
- Static verification of the incident inputs: both
  `archive/2026-09-16-fix-false-green-guards/specs/{ci-guard-integrity,ci-guard-baseline}/spec.md`
  contain only `## Purpose` and `## Requirements` sections — zero delta sections, therefore zero
  parsed deltas, with certainty.

### Predicted outcomes per case (trace; confirm by run)

| Case | Predicted outcome |
|------|-------------------|
| (a) single change, only full-spec-shaped file | `hasSyncDeltas(changeRoot) = false` → `syncResolveChangeRoot` returns `not-applicable`, msg `no delta specs for change X`, ZERO writes (skipped entirely) |
| (a2) mixed change (full-spec-shaped + ≥1 delta-shaped file) | change-level gate passes; the full-spec domain enters `infos` with 0 deltas (`sync_helpers.go:88` map-entry assignment); `ApplyDeltas("", []) = ""`; `os.WriteFile` at `:204` creates a 0-byte `openspec/specs/{domain}/spec.md`; `Sync()` returns `applied`, msg `sync applied for change X` — false-green |
| (b) new domain + `## ADDED Requirements` | works: `ApplyDeltas("", ADDED deltas)` rebuilds requirement blocks (no H1/Purpose header — as in the rdd precedent) |
| (c) existing domain + `## MODIFIED Requirements` | works: in-place block replacement |

## Layer Analysis

1. **Writer (primary defect locus)**: `sync_helpers.go:180` → `:189` → `:204` writes the
   `ApplyDeltas` result unconditionally. No check for "source file had content but parsed zero
   deltas", no guard against writing empty content over a missing main spec.
2. **Domain discovery / gate (co-factor)**: `syncParseDomainInfos` (`sync_helpers.go:70-92`) keeps
   zero-delta domains (the map assignment creates the entry even when `pr.Deltas` is nil);
   `hasSyncDeltas` (`openspec-deltas.go:163-185`) is CHANGE-level, so a mixed change passes the
   gate while the full-spec-only change is skipped — both behaviors exist, selected by whether any
   other delta-shaped file is present.
3. **Parser (root cause of zero deltas)**: `tryHandleRequirement`
   (`openspec-deltas_helpers.go:88-99`) discards requirements seen while `currentKind == ""`
   (`:94`) — correct for the delta dialect, blind to the full-spec dialect.
4. **Phase contract (input legitimacy)**: `internal/assets/skills/sdd-spec/SKILL.md` MANDATES the
   full-spec shape for new domains: "For NEW Specs (No Existing Spec): If this is a completely new
   domain, create a FULL spec (not a delta)" (Step 2 also: "Write a complete spec (not a delta)").
   The input is per-contract, NOT malformed. The archive phase already carries the correct rule:
   `internal/assets/skills/sdd-archive/SKILL.md:59` "If no main spec exists, the delta IS the
   spec — copy it to `openspec/specs/{domain}/spec.md`" and `internal/assets/prompts/sdd/sdd-archive.md`
   "#### If Main Spec Does NOT Exist — The delta spec IS a full spec (not a delta). Copy it directly".
   The sync path lacks the equivalent rule.

## Contract Options (ranked)

1. **New-domain verbatim create + loud fail-closed fallback (RECOMMENDED)** — when the main spec is
   missing and the delta file parses to zero deltas: if the file contains requirement headings
   (full-spec shape) → copy it verbatim into `openspec/specs/{domain}/spec.md`; if it has content
   but no requirement headings → `blocked` naming the file ("delta file has content but no
   ADDED/MODIFIED/REMOVED sections"); universally: never write an empty result when the source file
   is non-empty. Aligns sync with the archive rule and with the maintainer-approved workaround.
   Pros: kills the false-green, honors the existing sdd-spec contract, minimal surface. Cons: copy
   must be scoped to full-spec shape only — blanket verbatim copy of ADDED-shaped deltas would leak
   `# Delta for X` headers into canonicals (already visible in 4 living specs: core-review,
   pi-web-search, policy, sdd). Effort: Low-Medium.
2. **Fail-closed only** — `blocked` whenever a delta file has content but zero parsed deltas.
   Pros: simplest, safest. Cons: bricks the legitimate full-spec new-domain case that sdd-spec
   mandates; forces a simultaneous sdd-spec skill change. Effort: Low.
3. **Parser support for full-spec shape** — treat `### Requirement:` blocks under `## Requirements`
   as implicit ADDED when the main is empty. Pros: no skill changes. Cons: ambiguity with
   `## Requirements` sections appearing in non-delta contexts; dilutes the parser's explicit-section
   contract; writer still needs an empty-guard. Effort: Medium.
4. **Declare full-spec shape illegal in the change folder** — rewrite sdd-spec skill to emit
   `## ADDED Requirements` deltas for new domains too. Pros: single dialect end to end. Cons:
   contradicts the current skill text and the just-shipped convention; migration cost for writers.
   Effort: Medium-High.

## Blast Radius

- Callers of `ApplyDeltas`: `sync.go:100` (read-only `isSyncNeeded` probe), `sync_helpers.go:189`
  (the write path), tests (`openspec_deltas_retitle_test.go`). Callers of `syncApplyDeltas`:
  `sync.go:55` only. `Sync()` has NO production Go caller and NO CLI subcommand (the
  `cmd/biggz/main.go` dispatch has no `sdd-sync`; `status.go:1522` prints the phantom command
  `biggz sdd-sync <change>` — side-finding). No parallel implementation in JS/pi assets.
- Phase guards: `internal/sdd/sync_guard.go` (`syncStateNeedsDeltas`, `deriveSyncGuardReasons`) is
  blind to the defect — a full-spec domain contributes zero guard reasons; `isSyncNeeded` returns
  false for full-spec-only changes (status routes straight past sync).
- Living specs: 59 files under `openspec/specs/**/spec.md`. Current tree contains 0 empty living
  specs (the incident victims were saved by the verbatim-copy workaround; both are non-empty today:
  ci-guard-integrity 2 requirements, ci-guard-baseline 1 requirement). 4 living specs still carry
  `# Delta for` first lines (pre-existing, e.g. `openspec/specs/sdd/spec.md:1` — defect F2).
- The fix amends the `sdd` domain contract itself (`openspec/specs/sdd/spec.md:110,132` govern the
  sdd-sync phase and its implementation modules) → expect a MODIFIED delta against domain `sdd`.

## Test Coverage Gap

- Existing: `internal/sdd/openspec_deltas_retitle_test.go` — 7 tests, ONLY the
  MODIFIED-not-found retitle hint paths.
- Missing: zero references in any `*_test.go` to `Sync()`, `syncApplyDeltas`,
  `syncParseDomainInfos`, `hasSyncDeltas`, `isSyncNeeded`. No test covers: new domain + ADDED,
  new domain + full-spec shape (this bug), new domain + content-less file, mixed change, the
  skipped (`not-applicable`) case, or the empty-write guard. The prior ad-hoc overlay runner
  (`sdd-fast-lane` sync-report) was virtual and never checked in.
- Expected regression tests for the fix (RED first): (1) mixed change, full-spec new domain →
  living spec carries the requirement text, not empty; (2) full-spec-only change → `not-applicable`
  with zero writes; (3) content-but-no-requirements file → `blocked` naming the file; (4) existing
  MODIFIED path unchanged; (5) guard: no empty write ever when source non-empty.

## Could Not Determine

1. Live-run outputs of the reproducer (no shell in the explore session) — runbook provided in
   Appendix A; results still need a real run.
2. Whether historical empty canonicals ever landed in git history (not excavated).
3. Behavior of any host-side runtime clone outside this repo (in-repo pi assets verified clean).

## Appendix A — Reproducer Runbook (NOT yet executed)

Overlay technique (precedent: `archive/2026-09-13-sdd-fast-lane/sync-report.md`); zero repo
writes — the test file lives in `%TEMP%` and is injected via `go test -overlay`.

```bash
TMPD=$(mktemp -d)
# 1) write $TMPD/repro_test.go: package sdd; calls hasSyncDeltas, syncResolveChangeRoot,
#    syncParseDomainInfos, syncApplyDeltas, ParseDeltaSpec, ApplyDeltas, and full Sync() on
#    t.TempDir() workspaces for cases (a) single full-spec-only, (a2) mixed, (b) ADDED new domain,
#    (c) existing + MODIFIED. Verify-report envelope for the mixed full-Sync case:
#    schema: biggz-ai.verify-result/v1, verdict: pass, test_exit_code: 0, build_exit_code: 0,
#    requirements: 3/3, scenarios: 3/3, blockers: 0, critical_findings: 0 (counts = sum over the
#    change's spec files).
# 2) write $TMPD/overlay.json:
#    {"Replace": {"C:/Users/USER/Desktop/biggz-ai/internal/sdd/zz_repro_sync_empty_test.go":
#                 "<cygpath -m $TMPD>/repro_test.go"}}
cd C:/Users/USER/Desktop/biggz-ai
go test -overlay="<cygpath -m $TMPD>/overlay.json" ./internal/sdd -run 'TestZZReproSync' -count=1 -v
```

Expected (per trace, to confirm): (a) `not-applicable / "no delta specs for change X"` + no file
created; (a2) `applied` + `openspec/specs/<new-full>/spec.md` exists size=0 bytes; (b) file created
with requirement text; (c) main updated in place (`NEW thing` present, `OLD thing` gone).

## Reproducer — EXECUTED (2026-09-16)

**Status: EXECUTED.** The predictions in "### Predicted outcomes per case" above are TRACE-derived
(they stay as written, marked [trace] here); the table below is OBSERVED on `master` @ `97050fab`,
Windows, `go version go1.26.1 windows/amd64`. The reproducer ran in-package via `go test -overlay`,
scratch test + workspaces + overlay file in the OS temp dir; zero repository writes.

### Commands

```bash
TMPD=$(mktemp -d)
# $TMPD/repro_test.go — package sdd. Per case it builds a t.TempDir() workspace with
# openspec/changes/{change}/specs/{domain}/spec.md plus a valid biggz-ai.verify-result/v1
# verify-report.md (requirements/scenarios totals equal to the change spec heading counts),
# probes hasSyncDeltas / syncResolveChangeRoot / syncParseDomainInfos, then calls the real
# Sync(change, ws, "") and stats/reads the resulting living spec.
# $TMPD/overlay.json:
# {"Replace": {"C:/Users/USER/Desktop/biggz-ai/internal/sdd/zz_repro_sync_empty_test.go":
#              "<cygpath -m $TMPD>/repro_test.go"}}
cd C:/Users/USER/Desktop/biggz-ai
go test -overlay="<cygpath -m $TMPD>/overlay.json" ./internal/sdd -run 'TestZZReproSyncEmptySpec' -count=1 -v
# => PASS: TestZZReproSyncEmptySpec, ok github.com/biggs-100/biggz-ai/internal/sdd 0.185s (exit 0)
```

### Observed per-case table (vs the [trace] predictions)

| Case | [trace] prediction | OBSERVED |
|------|--------------------|----------|
| (a) single change, only full-spec-shaped file | `hasSyncDeltas=false` → `not-applicable`, msg `no delta specs for change X`, zero writes | CONFIRMED — `Sync() → result="not-applicable" msg="no delta specs for change chg-a"`; living spec NOT CREATED. Note: `syncParseDomainInfos` still returns domain `fullspec-domain` with `deltas=0` (the gate, not the parser, is what skips) |
| (a2) mixed change | gate passes; full-spec domain enters `infos` with 0 deltas; `ApplyDeltas("", [])=""`; `os.WriteFile` creates 0-byte living spec; `Sync()=applied` | CONFIRMED (false-green) — `Sync() → result="applied" msg="sync applied for change chg-a2"`; `openspec/specs/fullspec-domain/spec.md` EXISTS, size=0 bytes, readback `content=""`; the ADDED domain in the same change was created at 137 bytes |
| (b) new domain + `## ADDED Requirements` | works; rebuilt blocks, no H1/Purpose | CONFIRMED — `applied`; `new-domain/spec.md` created, 137 bytes, first line `### Requirement: New Domain Requirement` (no `#` / `## Purpose` header) |
| (c) existing domain + `## MODIFIED Requirements` | works; in-place block replacement | CONFIRMED — `applied`; 200 bytes; `"SHALL be NEW"=true`, `"SHALL be old"=false`; H1 / `## Purpose` / `## Requirements` header preserved |

### Extra observation — the writer is unguarded (gate bypassed, case-a shape)

The change-level `hasSyncDeltas` gate — not the writer — is the only thing preventing the empty
write in case (a). Calling `syncApplyDeltas(infos, ws2)` directly with the parsed infos of a
full-spec-only change (fresh workspace, `syncResolveChangeRoot` bypassed) was observed to create
the 0-byte file with no error:

```text
[a-direct] syncApplyDeltas(parsed infos) -> result="" msg="" err=<nil>
[a-direct] living spec EXISTS: size=0 bytes <ws2>/openspec/specs/fullspec-domain/spec.md
[a-direct] readback len=0 bytes content=""
[a-direct] *** EMPTY FILE (0 bytes) ***
```

### Raw evidence (log excerpts; `<ws>` = per-case `t.TempDir()`)

```text
[global] raw ApplyDeltas("", []RequirementDelta(nil)) = "" err=<nil> len=0

[a] hasSyncDeltas(changeRoot) = false
[a] syncResolveChangeRoot -> result="not-applicable" msg="no delta specs for change chg-a" err=<nil>
[a] syncParseDomainInfos -> result="" msg="" err=<nil>
[a]   parsed domain="fullspec-domain" deltas=0 hasRenamed=false
[a] Sync() -> result="not-applicable" msg="no delta specs for change chg-a" err=<nil>
[a] living spec <ws>/openspec/specs/fullspec-domain/spec.md NOT CREATED

[a2] hasSyncDeltas(changeRoot) = true
[a2] syncResolveChangeRoot -> result="" msg="" err=<nil>
[a2]   parsed domain="fullspec-domain" deltas=0 hasRenamed=false
[a2]   parsed domain="added-domain" deltas=1 hasRenamed=false
[a2] Sync() -> result="applied" msg="sync applied for change chg-a2" err=<nil>
[a2-fullspec] living spec EXISTS: size=0 bytes <ws>/openspec/specs/fullspec-domain/spec.md
[a2-fullspec] readback len=0 bytes content=""
[a2-fullspec] *** EMPTY FILE (0 bytes) ***
[a2-added] living spec EXISTS: size=137 bytes <ws>/openspec/specs/added-domain/spec.md
[a2-added] readback len=137 bytes content="### Requirement: New Domain Requirement\n\nThe system SHALL do new.\n\n#### Scenario: New works\n\n- **WHEN** new runs\n- **THEN** new succeeds\n"

[b] Sync() -> result="applied" msg="sync applied for change chg-b" err=<nil>
[b] living spec EXISTS: size=137 bytes <ws>/openspec/specs/new-domain/spec.md
[b] readback content="### Requirement: New Domain Requirement\n\nThe system SHALL do new.\n\n#### Scenario: New works\n\n- **WHEN** new runs\n- **THEN** new succeeds\n"

[c] Sync() -> result="applied" msg="sync applied for change chg-c" err=<nil>
[c] living spec EXISTS: size=200 bytes <ws>/openspec/specs/existing-domain/spec.md
[c] contains "SHALL be NEW"=true contains "SHALL be old"=false
```

### Verdict

**CONFIRMED** on all four cases and on both defect layers:

1. full-spec-only change → `not-applicable`, zero writes (gate skips it);
2. mixed change → 0-delta full-spec domain reaches the unconditional `os.WriteFile`, creating a
   0-byte living spec under `Sync()=applied` (the false-green);
3. the writer itself has no empty-write guard: bypassing the gate reproduces the 0-byte write even
   for the case-(a) shape;
4. ADDED-new-domain and MODIFIED-existing cases work as predicted.

No deviations from the [trace] prediction table were observed.
