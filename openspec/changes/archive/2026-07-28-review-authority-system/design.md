# Design: Review Authority System

## Technical Approach

Greenfield content-addressed event store under `.git/biggz/review-transactions/` with SHA-256 chain integrity, replacing the in-memory 5-state FSM. Each transition is an immutable JSON file named by its content hash. A 13-state FSM with role-based guard table and budget counters validates every transition before persisting. Receipts bind to the full chain (genesis → head → count). Publication gates (`pre-pr`, `pre-push`) are CLI commands with scope-change detection and dry-run mode.

Reference: gentle-ai's `reviewtransaction` package for state model patterns, but stripped of CompactState, v1/v2 compatibility, and Judgment Day mode — biggz is greenfield.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|---|---|---|---|
| **Event store location** | `.git/biggz/review-transactions/<lineage>/` | `~/.biggz/` or project root | Inside `.git/` means git operations carry it; auto-cleaned on clone; no env config |
| **File naming** | Raw SHA-256 hex (no `sha256:` prefix) | `sha256:` prefix (gentle-ai style) | Simpler file I/O; hash collision detection via content compare on `os.IsExist` |
| **Record schema version** | `biggz-ai.review-record/v1` | No schema versioning | Forward compat for future transitions; matches existing biggz patterns |
| **Receipt binding** | `SHA-256(genesis \|\| head \|\| count \|\| lineage_id)` | MerkleRoot-only (current) | Chain-level binding — tampering any event invalidates the receipt |
| **13 states** | Unreviewed → Archived via explicit transitions | 5-state (current) | Matches spec's role guard table; aligns with gentle-ai's 13-state lifecycle minus Judgment Day |
| **Budget enforcement** | Counters on FSM state + guard table | External policy evaluator | Self-contained; policy integration is an extension point, not a runtime dependency |
| **Locking** | File-based LOCK per lineage directory | In-memory RWLock (current) | Cross-process safety; store is file-based, so lock must be too |
| **Gate config** | Per-repo `.biggz/config.yaml` | CLI flags only | Repository-specific opt-out; no env vars needed |

## Data Flow

```
CLI (biggz review ...)
   │
   ├── gate pre-pr ──→ LoadChain() ──→ FSM.Validate(reviewable) ──→ Receipt.Verify() ──→ GateResult
   │
   ├── gate pre-push ─→ LoadChain() ──→ FSM.Validate(reviewable) ──→ ScopeDiff(currentTree, snapshottedTree) ──→ GateResult
   │
   ├── list ──────────→ DiscoverStores() ──→ [LineageID, State, LastEvent]
   │
   └── status <id> ──→ LoadChain() ──→ ValidateChain() ──→ {Head, Count, Valid, Receipt, Budget}

Event Store Append:
   Append(prevRevision, record) → os.MkdirAll(events/)
   → acquire LOCK → write temp → rename → write HEAD → release LOCK

Chain Validation:
   LoadChain(headRevision) → walk PreviousRevision to genesis
   → verify each file's content matches its SHA-256 name
   → verify PreviousRevision continuity → verify genesis is valid
```

## File Changes

| File | Action | Description |
|---|---|---|
| `model/fsm.go` | Modify | 5→13 states, guard table, budget counter checks |
| `model/review.go` | Modify | Add LineageID, BudgetCounters fields to ReviewState |
| `internal/review/store.go` | Create | Content-addressed event store with Append, LoadChain, Validate |
| `internal/review/receipt.go` | Rewrite | Bind to genesis+head+count+lineage, verify by replaying chain |
| `internal/review/authority.go` | Rewrite | Become store facade — inventory, status, chain validation |
| `internal/review/gate.go` | Rewrite | Pre-PR + pre-push gates; scope change via `git diff-tree`; dry-run + JSON output |
| `internal/review/correction.go` | Modify | Budget counter enforcement via FSM guards |
| `internal/review/lock.go` | Modify | Add file-based LOCK in addition to in-memory |
| `internal/review/review.go` | Modify | Adapt lifecycle to use event store + new FSM |
| `internal/review/ledger.go` | Modify | Persist via event store instead of in-memory array |
| `cmd/biggz/main.go` | Modify | Add `review list`, `review status`, `review gate` subcommands |
| `.git/biggz/review-transactions/` | Create | Event store root (created on first append) |
| `.biggz/config.yaml` | Create | Per-repo gate opt-out config |

## Interfaces / Contracts

```go
// Store — content-addressed event store
type Store struct {
    Dir       string // .git/biggz/review-transactions/<lineage>/
    LineageID string
}

func Open(repo, lineageID string) (Store, error)
func (s Store) Append(prevRevision string, rec Record) (revision string, err error)
func (s Store) LoadChain() (ValidatedChain, error)
func (s Store) Validate() IntegrityVerdict

// FSM — 13-state with role guards and budget
type FSM struct{}

func (FSM) Transition(current, target ReviewStatus, role string, counters BudgetCounters) error
func (FSM) GuardTable() []GuardEntry

// Receipt — chain-bound proof
type Receipt struct {
    LineageID       string
    GenesisRevision string
    HeadRevision    string
    EventCount      int
    BindingHash     string
}

func NewReceipt(chain ValidatedChain) Receipt
func (r Receipt) Verify(chain ValidatedChain) error

// Gate — publication check
type GateResult struct {
    Gate    string   `json:"gate"`
    Passed  bool     `json:"passed"`
    Reasons []string `json:"reasons,omitempty"`
    DryRun  bool     `json:"dry_run"`
}

func PrePRGate(chain ValidatedChain, findings []Finding) GateResult
func PrePushGate(chain ValidatedChain, currentTree string) GateResult
```

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Event store Append/LoadChain/Validate | In-memory temp dir; SHA-256 collision tests; empty lineage; tampered file |
| Unit | FSM guard table (13 states × 4 roles) | Table-driven tests — every (state_from, state_to, role) pair; budget edge cases |
| Unit | Receipt bind + verify | Valid chain, tampered chain, empty chain, clone-proof |
| Unit | Scope change detection | Stub `git diff-tree` output; compare against snapshotted tree |
| Integration | Gate pre-PR + pre-push | Create chain → gate passes → tamper → gate fails → dry-run exits 0 |
| Integration | CLI `review list` + `review status` | Create N lineages → enumerate → check output format (JSON + table) |
| Integration | Correction budget exhaustion | Advance to max fix rounds → next attempt rejected |
| RED | Threat matrix cases | Git repo selection, commit state, staged vs committed scope |

## Threat Matrix

| Boundary | Applicability | Design Response | Planned RED Tests |
|---|---|---|---|
| Git repo selection | **Applicable** — `rev-parse --git-dir` for store root | Validate return is within repo; reject empty/relative that escapes | Relative path, symlink repo, non-repo cwd |
| Commit state | **Applicable** — `git diff-tree` for scope detection | Compare committed tree hash (not staged); gate documents the distinction | Staged-only change, dirty worktree, clean state |
| Documentation-like paths | N/A — no executable classification | — | — |
| Push state | N/A — no push hooks | — | — |
| PR commands | N/A — no `gh` composition | — | — |

## Migration / Rollout

No migration required. Current in-memory review state has no persistent event store. The new store will have zero events until the first transition, which creates the genesis record. No backward-compat layer needed.

## Open Questions

- [ ] What is the exact scope snapshot format? A GTB (git tree hash) of the current HEAD? Or a full paths + digests snapshot?
- [ ] The spec says budget max fix_rounds=2, scoped_validations=2 — but some spec scenarios use 3 and 5. Confirm authoritative max values.
- [ ] Should `biggz review list` include an optional `--json` flag for machine output, or always emit both? (Current plan: `--json` flag)
