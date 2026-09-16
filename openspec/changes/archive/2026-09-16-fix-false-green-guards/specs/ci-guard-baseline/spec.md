# CI Guard Baseline Specification

## Purpose

Invariants that keep the ratchet baseline from rotting. Covers the two entry classes (debt and boundary), the debt class reaching zero as a reported count rather than an absence check, and boundary entries that persist while their boundary exists — but fail when stale.

## Requirements

### Requirement: Ratchet Baseline — Fail-on-Stale and Declared Boundary

The baseline is a two-class artifact: **debt** entries MUST be migrated to exactly zero; **boundary** entries are declared, permanent exceptions that are NOT debt. Format and matching MAY be defined by the design.

Debt class: the debt count MUST reach exactly zero when migration completes: counted and reported by a scan that resolved its targets, and MUST NOT be derived from an absence check (`grep -q .`, line counting, or similar) that a truncated, unreadable, or empty baseline could also satisfy. Zero debt entries with boundary entries remaining is a PASS; the file MUST NOT be required to become empty: the completion criterion is zero debt entries, never zero lines.

Boundary class: boundary entries persist while the boundary exists, never count toward the debt total, obey the same staleness rule, and MUST NOT be exempted by a glob that matches nothing. Two boundary sub-classes are declared in the baseline: (1) non-Go host spawns, pi `.ts`/`.js` assets spawning git via `node:child_process` outside the Go binary; (2) `internal/doctor`'s injectable `execFn` seams simulating git present/absent/broken.

An entry matching no live site MUST be a FAILURE, not a warning, not a note.

#### Scenario: New violation not in baseline blocks

- GIVEN a new spawn site with no baseline entry
- WHEN the guard runs
- THEN it MUST exit non-zero

#### Scenario: Fully matched baseline passes

- GIVEN every baseline entry matches a live site
- WHEN the guard runs
- THEN it MUST exit zero

#### Scenario: Stale entry is a failure

- GIVEN a baseline entry matching no live site
- WHEN the guard runs
- THEN it MUST exit non-zero, naming the stale entry

#### Scenario: Stale boundary entry is a failure

- GIVEN a declared non-Go host path that no longer spawns git
- WHEN the guard runs
- THEN it MUST exit non-zero, naming that boundary entry

#### Scenario: Migrated debt entry is stale

- GIVEN a debt entry whose site was migrated and no longer matches
- WHEN the guard runs
- THEN it MUST exit non-zero, naming it as debt

#### Scenario: Boundary entry with live site keeps passing

- GIVEN a boundary entry whose spawn site still exists
- WHEN the guard runs
- THEN it MUST exit zero across runs, without requiring migration

#### Scenario: Boundary entry whose site disappeared fails

- GIVEN a boundary entry whose spawn site no longer exists
- WHEN the guard runs
- THEN it MUST exit non-zero, naming that entry as stale

#### Scenario: Debt zero with boundary entries remaining passes

- GIVEN debt zero while boundary entries remain in the baseline
- WHEN the guard runs
- THEN it MUST exit zero and report a debt count of zero
- AND it MUST NOT treat the remaining boundary entries as debt or a non-empty-baseline failure
