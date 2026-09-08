# ci-test Specification

## Purpose

Reliable test matrix: bounded hermetic calls, ticketed quarantines only.

## Requirements

### Requirement: Hermetic Bounded Update Tests

Live update/upgrade calls MUST use a timeout context, never bare `Background`; tests MUST use an `httptest` fake or skip offline.

#### Scenario: Bad network never hangs

- GIVEN a stalled or unreachable API
- WHEN update/upgrade runs
- THEN it MUST abort or skip within the timeout, never hang

#### Scenario: Offline fake pass

- GIVEN no network with a fake server
- WHEN the tests run
- THEN they MUST pass via the fake

### Requirement: Ticketed Quarantines Only

Every skip MUST reference #24 with log-confirmed scope (first Slice task pulls CI asserts); unticketed skips MUST be blocked.

#### Scenario: Ticketed skip accepted

- GIVEN a #24 skip in log-confirmed scope
- WHEN CI runs
- THEN it MUST be accepted

#### Scenario: Unticketed skip blocked

- GIVEN a skip without a #24 reference
- WHEN reviewed
- THEN it MUST be blocked
