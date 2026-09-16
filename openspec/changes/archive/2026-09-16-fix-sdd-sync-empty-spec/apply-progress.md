# Apply Progress: fix-sdd-sync-empty-spec

Work unit: `sync-empty-write-guard` (single unit, 27 tasks). Mode: Standard (Strict TDD OFF).
Branch: `fix/sdd-sync-empty-write-guard` (off `master` @ `97050fab`). Status: implementation complete and green; delivery withheld by the 400-line budget gate (see Delivery Note).

## Phase 1 — RED evidence (observed on `97050fab`, Windows, Go 1.26.1)

Command: `go test ./internal/sdd -run 'TestSyncMixedFullSpecVerbatim|TestSyncContentWithoutBlocksBlocked|TestSyncGateBypassNoEmptyWrite' -count=1 -v`

```text
=== RUN   TestSyncMixedFullSpecVerbatim
    sync_helpers_test.go:177: living spec ...\openspec\specs\fullspec-domain\spec.md is empty under result "applied" (false-green)
--- FAIL: TestSyncMixedFullSpecVerbatim (0.03s)
=== RUN   TestSyncContentWithoutBlocksBlocked
    sync_helpers_test.go:240: result = "applied", want "blocked" (msg "sync applied for change chg-noblocks")
--- FAIL: TestSyncContentWithoutBlocksBlocked (0.02s)
=== RUN   TestSyncGateBypassNoEmptyWrite
    sync_helpers_test.go:315: living spec ...\openspec\specs\fullspec-domain\spec.md is 0 bytes (false-green)
--- FAIL: TestSyncGateBypassNoEmptyWrite (0.01s)
FAIL	github.com/biggs-100/biggz-ai/internal/sdd	0.182s
```

Pins pre-fix (must pass before and after), same commit:

```text
=== RUN   TestSyncFullSpecOnlyNotApplicable
--- PASS: TestSyncFullSpecOnlyNotApplicable (0.01s)
=== RUN   TestSyncModifiedInPlace
--- PASS: TestSyncModifiedInPlace (0.03s)
PASS	ok  	github.com/biggs-100/biggz-ai/internal/sdd	0.270s
```

## Phase 2–3 — GREEN

T1–T5 + `TestSyncResolveDomainWrite` (7 subtests) all PASS; `sync.go` untouched.

```text
go test ./internal/sdd -run 'TestSync' -count=1 -v
--- PASS: TestSyncMixedFullSpecVerbatim
--- PASS: TestSyncFullSpecOnlyNotApplicable
--- PASS: TestSyncContentWithoutBlocksBlocked
--- PASS: TestSyncModifiedInPlace
--- PASS: TestSyncGateBypassNoEmptyWrite
--- PASS: TestSyncResolveDomainWrite (7/7 subtests)
ok  	github.com/biggs-100/biggz-ai/internal/sdd	0.162s
```

## Phase 4 — Verification

```text
go test ./internal/sdd -count=1        → ok github.com/biggs-100/biggz-ai/internal/sdd 24.354s (0 FAIL)
go vet ./internal/sdd                  → clean
gofmt -l internal/sdd/sync_helpers.go internal/sdd/sync_helpers_test.go → no output
   (internal/review/rdd_helpers.go, the known pre-existing finding, untouched)
go build -o /tmp/gitexec.exe ./tools/gitexec/cmd/gitexec && /tmp/gitexec.exe -root .
   → gitexec: OK — self-check 9 sites, 351 .go + 17 host targets, 0 debt, 7 boundary, baseline matched (exit 0)
```

## Delivery Note — budget gate (STOP per delegation)

Measured changed lines: **644** (`sync_helpers.go` +148/−23 = 171; `sync_helpers_test.go` 473 new), vs the ledger max of 400 and the forecast's ~375. Per the delegation ("if your measured diff exceeds 400 lines, STOP and report the split instead of overflowing") the PR was **not opened**; the branch carries one local commit.

Viable split (no rewrite needed, both slices under 400):

- PR1 (resolver contract): `sync_helpers.go` (171) + `sync_helpers_test.go` holding only the 2 needed fixtures + `TestSyncResolveDomainWrite` (≈ 190) ≈ **361**.
- PR2 (stacked, sync-level integration): the workspace builder + constants + T1–T5 (≈ 280) ≈ **280**.

Alternative: `size:exception` on the single PR (precedent: PR #86, +961 declared).

## Rollback boundary

Revert `sync_helpers.go` to `97050fab`; delete `sync_helpers_test.go`. Copies fire only on absent living specs — no migration, nothing to heal. Existing 0-byte living specs are not auto-healed (D8 → `blocked`).

## Archive Addendum — post-verify delivery updates (appended at archive)

> The snapshot above (Phases 1–4 + Delivery Note) was truthful at the STOP point. The final-state
> facts below outrank it (launch-prompt handoff + native ledger). The BigMem observation for this
> artifact — `sdd/fix-sdd-sync-empty-spec/apply-progress`, `obs-1789583484495097300-1`, updated
> 2026-09-16T19:23:52Z — was last upserted with the post-verify CI-fix content quoted here so the
> archived trail is self-contained.

From `obs-1789583484495097300-1` (architecture):

- **What**: CI FIX apply — decomposed `sdd.syncResolveDomainWrite` (`internal/sdd/sync_helpers.go`)
  into 6 named helpers so the CI Complexity job passes; zero behavior change.
- **Why**: PR #102 Complexity job was RED (gocyclo 18 > 15, gocognit 26 > 20 on `syncResolveDomainWrite`).
- **Where**: `internal/sdd/sync_helpers.go` (commit `c673889d` on `fix/sdd-sync-empty-write-guard`;
  merge `d516cf66` into `fix/sdd-sync-integration-proof`).
- **Helpers extracted**: `syncReadMainSpec`, `syncSplitSources`, `syncBlockEmptySources`,
  `syncResolveFullSpecWrite`, `syncWriteFullSpecTarget`, `syncResolveDeltaWrite`. Top-level is now a
  4-branch decision ladder (gocyclo 4 / gocognit 3); max helper 5/5.
- **Evidence**: `gocyclo -over 15` → no output exit 0; `gocognit -over 20` → no output exit 0;
  `go test ./internal/sdd -count=1` ok on both branches (resolver table 7 sub-cases + T1–T5);
  `go vet` clean; `gofmt -l` clean; `gitexec` exit 0 (0 debt, 7 boundary, baseline matched).
- **Diff budget**: PR1 total 397 changed lines (≤400), work unit 89 lines (≤120).

Delivery outcome (final state, outranks the Delivery Note above): the split described there was
**executed** — PR #102 `fix/sdd-sync-empty-write-guard` (397 changed lines: writer guard + resolver
unit table, merge `a89f3493`) and stacked PR #103 `fix/sdd-sync-integration-proof` (+303 lines:
`sync_integration_test.go` T1–T5, merge `e2adc96c`); `master` head at close `e2adc96c`; issue #101
closed by the merge. CI: 18/18 on both PRs (PR #103 after retargeting to `master`).
