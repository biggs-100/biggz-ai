# Testing Guidance — Contract-First Filter

> **Source of truth for Good vs Bad tests in biggz-ai.** Zero `internal/*` logic change — docs + linter + CI only. See `openspec/changes/testing-guidance`.

## Good vs Bad Filter

A test is **Good** iff it satisfies **≥1** of:

| Criterion | What it means | Example |
|-----------|---------------|---------|
| **Failure mode** | Names and exercises a concrete failure (race, traversal, timeout, corruption) | `TestBlob_ConcurrentSameBytes` races `PutBlob` — fails without `-race` or without dedup guard |
| **Transformation / Branch** | Exercises a distinct code path or state transition | `TestBranch_Traversal` (`TestBranchCreateChild` / `TestBranchListGetChain` / `TestGetLeafPathChain`) walks `parent_id → leaf_id` via `modernc.org/sqlite` |
| **External contract** | Asserts on observable output (DB row, file, API, `modernc.org/sqlite` query) not source text | `TestBranch_Traversal` asserts `SELECT parent_id, leaf_id FROM sessions` |
| **Regression** | Pins a previously shipped bug as a failing case | `ledger_regression_test.go` pins frozen chain; `rapid` FSM pins compact-store invariants |

A test is **Bad** iff it is **any** of:

| Anti-pattern | Symptom | Why it hurts |
|--------------|---------|--------------|
| **Static echo** | `expect(src).toContain("foo")` — asserts source contains a string | Breaks on rename; proves nothing about runtime |
| **Passthrough** | Calls function and asserts it returns what it was given without edge | No branch covered |
| **Wording** | Asserts error message wording exactly | Brittle; hides contract |
| **Duplicate** | Repeats an already-covered branch without new failure mode | Inflates suite, no signal |

**Rule: Good overrides Bad.** If a test meets any Good criterion it is Good even if it superficially looks like an echo. The linter enforces the mechanical bans below; the table is the human filter.

## Pinned Examples (grep-able anchors)

### Good: `TestBlob_ConcurrentSameBytes` — failure mode + external contract

- **File:** `internal/bigmem/blobstore_test.go:TestBlob_ConcurrentSameBytes` — comment `// Good: failure mode + external contract — races PutBlob with -race`
- **What it does:** `sync.WaitGroup` races two `PutBlob` with identical 200 KiB payload, asserts `addrs[0] == addrs[1]` and `GetBlob` returns uncorrupted bytes. Requires `go test -run TestBlob_ConcurrentSameBytes -race -count=1`.
- **Why Good:** Named failure mode (concurrent dedup race), external contract (blobstore file + `ValidateAddr`), `-race` detector. See `bench:guard` — this is not a bench test.
- **Threat:** Test flake if `HOME` not isolated or `-race` omitted. Mitigated by `t.TempDir()` + `t.Setenv("HOME", …)` and `isolatedHome` helper.

### Good: `TestBranch_Traversal` — transformation/branch + external contract via `modernc.org/sqlite`

- **File:** `internal/bigmem/branch_test.go:TestBranchCreateChild` / `TestBranchListGetChain` / `TestGetLeafPathChain` — comment `// Good: transformation/branch + external contract via modernc.org/sqlite`
- **What it does:** Creates `root → a → b → c` branches, lists via `ListBranches()`, traverses `GetLeafPath(leaf)` asserting `leaf→root` order `[l, b, r]` by querying `sessions.parent_id/leaf_id` from `modernc.org/sqlite`.
- **Why Good:** Covers branching transformation (parent/leaf columns, cycle guard, depth 100 truncation), asserts DB state not source text, uses real `modernc.org/sqlite` driver (no mocks).
- **Context:** Complements `internal/review/compact_state_rapid_test.go:TestRapid_*` (FSM via `pgregory.net/rapid`) and `internal/review/ledger_regression_test.go` (frozen chain) — the three pillars of contract testing in this repo.

### Bad: `TestExpectSrcContains` — static echo + source-grep (must be rejected)

- **Anti-example (do NOT do this):**
  ```go
  // Bad: source-grep — fragile, not a contract
  func TestExpectSrcContains(t *testing.T) {
      src, _ := os.ReadFile("internal/bigmem/blobstore.go")
      if !strings.Contains(string(src), "PutBlob") {
          t.Fatal("missing PutBlob")
      }
  }
  // Also bad: expect(src).toContain("PutBlob") in JS/TS tests
  ```
- **Why Bad:** Static echo (asserts file contains substring), source-grep (reads `*.go` source and `Contains`), wording-dependent. The `no-source-grep` linter (`tools/nosourcegrep`) fails this with `source-grep: os.ReadFile on source file *.go/*.md is banned`.
- **Fix:** Assert the contract instead — e.g. call `PutBlob` and assert `IsBlobAddr` / `GetBlob` round-trip (as `TestBlob_ConcurrentSameBytes` does) or query `sessions` table (as `TestBranch_Traversal` does).

## Bans

### Ban: `mock.module` — never use

- **Banned:** `mock.module` (Bun/Jest global mock) in any `*_test.go`, any import or call containing `mock.module`.
- **Rationale:** Global leak — `oven-sh/bun#12823` documents `mock.module` leaking across tests via global registry, causing order-dependent flakes and cross-test pollution. biggz-ai forbids it repository-wide.
- **Allowed instead:** Explicit interfaces, `t.TempDir()`-scoped fakes, `sql.Open("sqlite", t.TempDir()+"/db")` with `modernc.org/sqlite`, or `httptest.NewServer`. No global mutation.
- **Enforcement:** `rg -n "mock\.module" --glob '*_test.go'` in CI (`lint-no-source-grep` job) — fails on any hit. Analyzer also flags string literal `mock.module` in `*_test.go`.

### Ban: Source-grep assertions — never assert on source text

- **Banned patterns (in `*_test.go`):**
  - `os.ReadFile` on `*.go` / `*.md` source followed by `strings.Contains` / `bytes.Contains` / `strings.Contains(string(src), …)`
  - `expect(src).toContain(...)` / `expect(source).toContain` / any `ToContain` on source
  - `os.ReadFile("internal/foo.go")` + `Contains`, `ReadFile("docs/architecture.md")` + `Contains`, or equivalent grep on source
- **Allowed:** `os.ReadFile` on `testdata/` fixtures, `t.TempDir()` outputs, or `BlobRoot()` artifacts — `testdata/` is allowlisted. DB-query assertions via `modernc.org/sqlite` are the canonical alternative.
- **Enforcement:** Primary `tools/nosourcegrep` analyzer (`go vet -vettool=./tools/nosourcegrep ./...` + `golangci-lint` custom `nosourcegrep`), fallback `rg -n "os\.ReadFile.*Contains|expect\(src\)" --glob '*_test.go'` in CI. Scoped to `*_test.go`, `testdata/` excluded.

## bench:guard

> `go test ./bench -count=1` **never proves driven execution**.

- **What it means:** Bench/journey tests (`go test ./bench`, `go test ./internal/... -run Bench`) are informational — they measure throughput/latency, not contract correctness. A green bench does **not** prove the harness drove the agent through a real SDD cycle.
- **Driven-mode proof requires:** explicit journey corpus or contract tests — `TestRapid_*` (`pgregory.net/rapid` FSM in `compact_state_rapid_test.go`, `lifecycle_rapid_test.go`), `ledger_regression_test.go` (frozen chain), or `TestBranch_Traversal` / `TestBlob_ConcurrentSameBytes` above.
- **CI rule:** `lint-no-source-grep` + `go test -run TestRapid ./... -count=1 -timeout 180s` are the gates; bench success is not a substitute and MUST NOT mark driven execution verified.
- **Doc anchor:** This paragraph is the `bench:guard` pin — search `bench:guard` to find it.

## CI Enforcement

```
*_test.go → go vet -vettool=./tools/nosourcegrep ./...   (local, *_test.go only)
         → golangci-lint -E nosourcegrep (.golangci.yml)
         → CI lint-no-source-grep (analyzer || rg fallback)
         → go test -run TestRapid ./... -count=1 -timeout 180s
         → go test ./... -count=1 -timeout 180s + go vet ./... + gofmt -l
```

- `testdata/` is allowlisted in both analyzer and `rg` fallback.
- `rg -n "mock\.module"` must be empty — any hit fails CI.

## References

- Rapid FSM: `pgregory.net/rapid` — `internal/review/compact_state_rapid_test.go`, `internal/review/lifecycle_rapid_test.go`
- SQLite contract: `modernc.org/sqlite` — `internal/bigmem/branch_test.go`, `internal/bigmem/blobstore_test.go`
- Regression pin: `internal/review/ledger_regression_test.go`, `internal/review/contract_test.go`
- Linter: `tools/nosourcegrep/analyzer.go` (`*ast.CallExpr`, `*_test.go` scope, `testdata` allowlist), `.golangci.yml` custom `nosourcegrep`
- Bun leak: `oven-sh/bun#12823`
