# Delta for sdd

## ADDED Requirements

### Requirement: Gatekeeper Store-Aware Artifact Resolution

`internal/sdd` gatekeeper artifact validation MUST receive the active artifact store — the value produced by `NormalizePreflightArtifactStore` (`openspec`, `hybrid`, or `""` for none) — as an explicit input, and MUST NOT infer the store from the phase result it validates. Existence MUST be decided against the store's canonical artifact location for the completed phase: repo-relative and change-relative declarations MUST both resolve, and a declared artifact `Path` MUST NOT be accepted as proof of existence. Under `openspec` and `hybrid` the canonical filesystem artifact MUST exist, and a missing one MUST fail the check naming the resolved absolute path. Under `""` no filesystem artifact is expected and the check MUST be reported as skipped with an explicit reason, never as a silent pass.

#### Scenario: Repo-relative declaration is not a false negative

- GIVEN store `openspec`, the change's delta spec file exists, and the phase result declares it as the repo-relative path `openspec/changes/{change}/specs/{domain}/spec.md`
- WHEN the gatekeeper validates artifacts
- THEN the check MUST pass, resolving the path from the workspace root and not under the change directory

#### Scenario: Declared path does not substitute for the canonical artifact

- GIVEN a `spec`-phase result whose declared `Path` exists but is not that phase's canonical artifact
- WHEN the gatekeeper validates artifacts
- THEN the check MUST fail naming the missing canonical artifact path

#### Scenario: hybrid with only BigMem topics fails on the missing copy

- GIVEN store `hybrid` and a result declaring only BigMem topic keys such as `sdd/{change}/proposal`
- WHEN the gatekeeper validates artifacts
- THEN the check MUST fail naming the canonical filesystem path that is missing

#### Scenario: None store reports skip with a reason

- GIVEN store `""` and a phase result declaring BigMem topic keys
- WHEN the gatekeeper validates artifacts
- THEN the artifact check MUST be reported as skipped with an explicit reason and MUST NOT be reported as passed

### Requirement: Hook Dead-Producer Distinction

The installed pre-push hook (`internal/install/assets/hooks/pre-push.tmpl`, deployed as `.git/hooks/pre-push`) MUST distinguish three RDD states: `enabled`, `disabled`, and `unknown` — the status producer failing, absent, or missing from `PATH`. Collapsing `unknown` into `disabled` MUST be forbidden: a failed or absent `biggz rdd status` MUST NOT silently skip the guard. In the `unknown` state the hook MUST take an explicit, observable disposition — block with a non-zero exit, or proceed while printing an explicit notice line — and MUST NOT proceed with neither. The `SKIP_RDD_GATE=1` audit fallback at line 10 (`|| echo "enabled"`) MUST remain fail-closed: the recorded `rdd` mode MUST be `enabled`, never `disabled`.

#### Scenario: Dead producer is never a silent skip

- GIVEN `biggz` is absent or `biggz rdd status` exits non-zero, and no lineage or completed verify report exists
- WHEN the pre-push hook runs
- THEN it MUST NOT exit 0 with empty output; it MUST either exit non-zero or print an explicit notice line naming RDD status as unavailable

#### Scenario: Enabled and unmanaged blocks

- GIVEN `biggz rdd status` reports `RDD Status: enabled` and no lineage exists
- WHEN the pre-push hook runs
- THEN it MUST exit 1 and print the blocked hint naming the producer command

#### Scenario: Explicitly disabled allows

- GIVEN `biggz rdd status` reports `RDD Status: disabled` and no lineage exists
- WHEN the pre-push hook runs
- THEN it MUST exit 0 because `disabled` is an observed, explicit state

#### Scenario: Bypass audit fails closed

- GIVEN `SKIP_RDD_GATE=1` and `biggz rdd status` fails or `biggz` is absent
- WHEN the hook writes its audit line
- THEN the `rdd` field MUST be `enabled` (fail-closed) and MUST NOT be `disabled`
