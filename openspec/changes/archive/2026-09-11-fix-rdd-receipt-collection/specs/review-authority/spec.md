# Delta for review-authority

## ADDED Requirements

### Requirement: Single Lineage Identity Derivation

The system MUST derive exactly one lineage identity, used consistently by the review offer, the gate lookup, and the terminal receipt. The offer MUST NOT embed a lineage id; identity is derived once at start and resolved — never re-authored — by consumers.

#### Scenario: Offer, gate and receipt resolve one lineage

- GIVEN RDD enabled and a started review for a verified candidate
- WHEN the offer, the gate lookup, and the receipt resolve
- THEN all three MUST resolve the same lineage identity

#### Scenario: Offer carries no lineage id

- GIVEN any emitted review offer
- WHEN the offer payload is inspected
- THEN it MUST NOT contain an embedded lineage id

### Requirement: Subject Commit Canonicalization at Review Start

`review start` MUST canonicalize the subject commit to a full object SHA (rev-parse) or reject the start; an abbreviated SHA MUST NEVER be persisted as the lineage subject. Existing lineages recorded with abbreviated subjects MUST remain readable via read-path resolution only — persisted events MUST NOT be rewritten.

#### Scenario: Abbreviated subject canonicalized

- GIVEN `review start` with a resolvable abbreviated subject SHA
- WHEN the genesis is persisted
- THEN the subject MUST be the full object SHA and the lineage MUST be capturable

#### Scenario: Unresolvable subject rejected

- GIVEN `review start` with an unresolvable subject commit
- WHEN start evaluates
- THEN it MUST reject with a typed error and persist no lineage

#### Scenario: Legacy abbreviated lineage stays readable

- GIVEN a lineage whose recorded subject is an abbreviated SHA
- WHEN its chain is read
- THEN the subject MUST resolve on the read path and no event MUST be rewritten
