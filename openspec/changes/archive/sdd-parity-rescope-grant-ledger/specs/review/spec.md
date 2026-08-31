# Delta for review

## ADDED Requirements

### Requirement: REQ-G5-01 — Adapted Passive Content Proof (8 MiB, Shebang, MDX, Exec)

The system MUST keep `ClassifyRisk` decision order unchanged (sensitive → exec-config → documentationOnly → volume → triviallyInert → medium) but replace inert check with adapted `isPassiveContentFile` gated behind `isPassiveDocumentExtension` allowlist (`.md/.markdown/.mdown/.rst/.adoc/.txt/.png/.jpg/.jpeg/.gif`). `isPassiveContentFile` MUST read at most `8 MiB`; if size `>8 MiB` or unreadable then MUST return not-passive (fail-closed). Otherwise MUST check: NUL byte or `!utf8.Valid` → not passive; `hasInterpreterDirective` (`#!` after optional BOM/whitespace at file start) → not passive; `isStaticMDXDocument` (import/export or `<{` `}>` JSX-ish markers) → not passive; cheap substring `subprocess|execute_process|exec` (case-insensitive) → not passive (process_boundary).

#### Scenario: Over-budget and unreadable fail closed

- GIVEN `docs/large.md` `9 MiB` or `docs/missing.md` unreadable with allowlisted extension
- WHEN `isPassiveContentFile` or `ClassifyRisk` with `DiffSummary` indicating authored lines evaluates
- THEN file MUST be not-passive and `docs/large.md` only change MUST NOT be `RiskLow` (escalates to medium/high)

#### Scenario: Shebang/MDX/exec escalate via fixture

- GIVEN fixture `docs/with-shebang.md` starting with `#!/usr/bin/env python` and `docs/comp.mdx` with `import x from "y"` and `docs/note.md` containing `subprocess.call`
- WHEN `isPassiveContentFile` / `triviallyInert` evaluates
- THEN each MUST be not-passive, and `ClassifyRisk` with only those paths MUST be `RiskMedium` or `RiskHigh` (not `RiskLow`), with `docs/with-shebang.md` never `RiskLow`

#### Scenario: Pure passive stays low

- GIVEN `docs/readme.md` `2 KiB` plain markdown without shebang/MDX/exec/NUL and `isDocumentationPath=true`
- WHEN `ClassifyRisk(["docs/readme.md"], 10, {"docs/readme.md":10})` is called
- THEN tier MUST be `RiskLow`

#### Scenario: Gate behind extension allowlist

- GIVEN `scripts/tool.go` containing `exec` substring but extension `.go` not allowlisted
- WHEN passive check routes
- THEN extension gate MUST skip content read and defer to normal `semanticSourceExtensions` logic (file is not `triviallyInert`)
