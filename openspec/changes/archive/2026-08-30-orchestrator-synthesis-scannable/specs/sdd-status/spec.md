# Delta for sdd-status

## ADDED Requirements

### Requirement: Four-Block Scannable Rendering

The system MUST render `sdd-status` as 4 blocks in `Outcome + Quick path + Details` shape (cognitive-doc-design: answer first, chunking, signposting) with progressive disclosure. Each block MUST contain Outcome (1-line status), Quick path (numbered next steps 1..n), Details (`| Topic | Decision |` table). The 4 blocks MUST be `Status Overview`, `Artifact Progress`, `Next Action`, `Risks/Blockers` (order fixed, 5s scannable). Banner MUST be adaptive (truncated via `TruncateToWidth`). All text MUST be sanitized before width measure.

#### Scenario: Four blocks present
- GIVEN `sdd-status` with artifacts and nextRecommended
- WHEN rendered in terminal
- THEN output MUST contain 4 headings in Outcome/Quick path/Details order and each block MUST have Outcome line + numbered Quick path + Details table

#### Scenario: Quick path actionable
- GIVEN `nextRecommended: sync` and verify PASS
- WHEN Quick path of Next Action block read
- THEN it MUST list `1. biggz sdd-sync <change>` and `2. verify` as numbered steps

#### Scenario: Banner adaptive truncation
- GIVEN change name with 100 chars + ANSI on 80-col terminal
- WHEN banner rendered
- THEN it MUST strip ANSI and `TruncateToWidth` to terminal width with `…` and `VisibleWidth ≤ width`

### Requirement: Progressive Disclosure Chunking and Sanitized Truncation

The system MUST provide progressive disclosure (collapsible blocks with `… +N more` hint) and chunking <7 rows per Details table. Content MUST be sanitized via `stripAnsi/stripOsc/CONTROL_CHAR` then `TruncateToWidth` before `VisibleWidth` (`internal/tui/sanitize.go`). Empty blocks MUST be omitted. Chunk hint MUST show hidden count. Coverage MUST include `sdd-status` blocks and docs in same `Outcome + Quick path + Details` shape.

#### Scenario: Chunking under seven
- GIVEN Details table with 12 rows and terminal width 60
- WHEN rendered
- THEN it MUST split into ≥2 chunks each ≤7 rows with per-cell truncation and show `… +5 more` hint on first chunk

#### Scenario: Collapsed block hint
- GIVEN Risks block with 10 blockers on narrow viewport
- WHEN initially rendered
- THEN it MUST collapse after 7 rows and display hint with hidden count
- AND expansion MUST reveal remaining rows

#### Scenario: Sanitized truncation CJK/ANSI
- GIVEN table cell with `\x1b[31m` ANSI + OSC + `中` CJK + 100 chars
- WHEN `TruncateToWidth` to 20 applied before measure
- THEN `VisibleWidth` MUST be ≤20, CJK counted as 2, ANSI as 0, no split wide rune, ends with `…`

#### Scenario: Empty omitted
- GIVEN `BlockedReasons` empty and no risks
- WHEN `sdd-status` rendered
- THEN Risks/Blockers block MUST be omitted or show `None` without empty table

