# contracts — frozen wire-envelope formalization

This directory is the formalization layer for biggz-ai's wire envelopes: a
frozen set of JSON Schemas (draft 2020-12) plus one positive fixture per
schema. It is the direct port of gentle-ai's `contracts/` directory, with one
structural improvement: biggz adds a contracts-dir walk test that compiles
every schema and validates every fixture (`internal/contracts/walk_test.go`),
something gentle never had.

## The const-vs-$id split

Every biggz envelope carries an in-envelope `schema` const — the dotted Go
string the engine emits and reads (e.g. `biggz-ai.review-start-event/v1`).
Every schema FILE also declares a JSON-Schema `$id` — the URL that names the
schema resource itself:

- `$id`: `https://biggz-ai.dev/contracts/<family>/v<NN>/schemas/<name>.schema.json`
- in-envelope `schema` const: `biggz-ai.<envelope>/v<NN>` (unchanged Go string)

The two NEVER collide: the `$id` is a URL that identifies the schema
document; the const is the wire value carried inside emitted bytes. The one
notable mapping quirk: `start.schema.json` formalizes the envelope whose
const is `biggz-ai.review-start-event/v1` (the file name is `start`, the
const is `review-start-event`) — the file names follow the artifact they
formalize, the const follows the Go constant. See the per-family tables
below.

## Validation stance: test-only + opt-in emission checks (never runtime)

Inherited from gentle-ai: JSON Schema validation is CI-time conformance of
emitted bytes, NOT a runtime schema check. The engine's own strict decoders
and validators (DisallowUnknownFields, chain integrity, receipt re-binding)
are the runtime authority and never consult these files. The `internal/contracts`
package is used by:

1. the walk test — every schema compiles with its declared `$id` and every
   fixture validates against its same-name schema;
2. the conformance tests — real engine output (contract envelope, consent
   envelope, captured artifact, persisted receipt, refutation round-trip,
   SDD consent and admission envelopes) is marshaled and validated;
3. optional test-only emission helpers — never failing paths inside
   Append/Finalize/gate.

## Excluded formats (stay Go-validated)

- Fenced-YAML envelopes: `biggz-ai.verify-result/v1` and
  `biggz-ai.remediation-result/v1` are Markdown-fenced YAML documents parsed
  by internal/sdd/verify.go, not JSON wire envelopes — excluded by design.
- Legacy gentle-ai read-compat aliases: `gentle-ai.verify-result/v1` and
  `gentle-ai.remediation-result/v1` remain accepted by Go admission code for
  historical reports — excluded, they are compatibility tokens, not emitted
  formats.
- `biggz.review-compact-state/v1` (the legacy compact state marker) is
  internal persisted state, not a wire envelope — excluded.
- Gate results (`biggz-ai.review-gate-result/v1` does NOT exist and must NOT
  be added): `GateResult` has no schema field and is documented as future
  work. Adding a schema const to `GateResult` would mutate gate output bytes
  — forbidden by the additive-only rule below.

## Additive-only rule

Validation NEVER mutates or rejects existing ledgers. Adding the contracts
layer must not change a single byte of any content-addressed event, receipt,
or mirror payload: the layer is purely observational. The ledger regression
test (`internal/review/ledger_regression_test.go`) bakes a pre-existing chain
fixture and asserts LoadChain + IntegrityVerdict + PersistedReceipt.Validate
+ receiptArtifactOf behave identically with the contracts layer present.

## Versioning policy

- A breaking or additive shape change to a wire envelope is a NEW version:
  a new `v<N+1>/` directory with the complete schema set, never a mutation
  of the frozen `v<N>/` files.
- Freezing a version is recorded in `contracts/<family>/v<N>/FREEZE.md`
  (gentle's wording: "frozen, not deleted" — byte-unchanged requirement and
  an exit-evidence checklist). Biggz v1 directories START UNFROZEN: no
  FREEZE.md is created until a first release consumer pins the version, so
  the first release can still correct a formalization error cheaply.

## review-integration/v1 — schemas

| file | in-envelope const (Go constant) | source struct |
|---|---|---|
| contract.schema.json | `biggz-ai.review-integration/v1` | ContractEnvelope + NextTransitionEnvelope (internal/review/contract.go) |
| start.schema.json | `biggz-ai.review-start-event/v1` | StartEventPayload (internal/review/finalize.go) |
| consent.schema.json | `biggz-ai.review-consent/v1` | ConsentEnvelope (internal/review/consent_relay.go) |
| result-artifact.schema.json | `biggz-ai.review-result-artifact/v1` | CapturedArtifact (internal/review/capture.go) |
| artifact-subject.schema.json | `biggz-ai.review-artifact-subject/v1` | ArtifactSubject (internal/review/artifact.go) |
| preflight.schema.json | `biggz-ai.review-capture-preflight/v1` | PreflightResult (internal/review/capture.go) |
| receipt.schema.json | `biggz-ai.review-receipt/v1` | PersistedReceipt + ReceiptLensSubject (internal/review/finalize.go) |
| record.schema.json | `biggz-ai.review-record/v1` | Record (internal/review/store.go) |
| refutation.schema.json | `biggz-ai.review-refutation/v1` | RefutationInput + RefutationVerdict (internal/review/refute.go) |
| refutation-event.schema.json | `biggz-ai.review-refutation-event/v1` | refutationEventPayload (internal/review/refute.go) |
| lens-result-event.schema.json | `biggz-ai.review-lens-result-event/v1` | lensResultEventPayload (internal/review/capture.go) |
| complete-event.schema.json | `biggz-ai.review-complete-event/v1` | completeEventPayload (internal/review/finalize.go) |
| invalidate-event.schema.json | `biggz-ai.review-invalidate-event/v1` | invalidateEventPayload (internal/review/terminal.go) |
| withdraw-event.schema.json | `biggz-ai.review-withdraw-event/v1` | withdrawEventPayload (internal/review/terminal.go) |
| dispose-event.schema.json | `biggz-ai.review-dispose-event/v1` | disposeEventPayload (internal/review/dispose.go) |
| reopen-event.schema.json | `biggz-ai.review-reopen-event/v1` | reopenEventPayload (internal/review/dispose.go) |
| verification-retry.schema.json | `biggz-ai.review-verification-retry/v1` | VerificationReport (internal/review/verify_retry.go) |
| reconcile.schema.json | `biggz-ai.review-reconcile/v1` | ReconcileReport + mirror structs (internal/review/reconcile.go) |
| inspect.schema.json | `biggz-ai.review-inspect/v1` | InspectResult + EventInspectSummary (internal/review/inspect.go) |
| rdd-status.schema.json | `biggz-ai.rdd-status/v1` | RDDStatusReport (internal/review/rdd.go) |
| rdd-consent.schema.json | `biggz-ai.rdd-consent/v1` | rddConsent (internal/review/consent.go) |

## sdd-integration/v1 — schemas

| file | in-envelope const | source struct |
|---|---|---|
| edit-authority-consent.schema.json | `biggz-ai.edit-authority-consent/v1` + `biggz-ai.sdd-integration/v1` | EditAuthorityConsentResult (internal/sdd/edit_authority_consent.go) |
| verify-admission.schema.json | `biggz-ai.verify-admission/v1` | VerifyAdmission (internal/sdd/verify.go) |

## Style conventions

- draft 2020-12, `type: object`, `additionalProperties: false` everywhere.
- Shared shapes live in `$defs`: `sha256` (`^sha256:[0-9a-f]{64}$`),
  `sha256_hex` (bare 64-hex content address), `git_tree`
  (`^[0-9a-f]{40}([0-9a-f]{24})?$`), `lens`.
- Choice pairs (consent envelopes) use `prefixItems` with granted then
  declined and `items: false`.
- Cross-field dependencies use `allOf` + `if/then/else`; field groups use
  `dependentRequired`.
- Hashes are never checked for REAL content equivalence — only shape: a
  fixture's hashes are realistic-looking, pattern-valid values.

## Adding a contract version

1. Create `contracts/<family>/v<N+1>/schemas/` and `fixtures/` with the full
   schema set for the new version.
2. Write each schema FROM the current Go struct (never the reverse): the
   struct's emitted JSON is the truth; the schema formalizes it.
3. Add one positive fixture per schema; it MUST validate under the walk
   test. Negative cases are programmatic mutations in the walk test, not
   fixture files.
4. Register nothing at runtime: `internal/contracts` embeds the whole tree
   with `go:embed`, so new versions are picked up automatically.
5. Freeze the previous version by adding its `FREEZE.md` when a release
   consumer pins it.
