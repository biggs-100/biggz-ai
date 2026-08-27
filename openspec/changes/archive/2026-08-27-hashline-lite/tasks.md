# Tasks: hashline-lite — Go port of oh-my-pi hashline

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 480-620 (prod ~280 + tests ~240) |
| 400-line budget risk | Medium |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `internal/edit/hashline` pkg | PR1 | `go test ./internal/edit/hashline -run TestParse -count=1` | mismatch->needs_attention; CUT ok | Delete pkg |
| 2 | hook `edit.mode` in `internal/sdd/apply.go` | PR1 c2 | `go test ./... -count=1 -timeout 180s` | off->legacy on->hashline batch ok | Revert hook |

## Phase 1: Foundation

- [x] 1.1 Create `internal/edit/hashline/snapshot.go` `Store{map[string][]byte,mu}` `Capture/Restore/Clear` via `WriteFileAtomic` <=N. Done: Restore restores; Clear empties.
- [x] 1.2 Add `Hash4`+`NoopLoopGuard` in `apply.go`: `Hash4` 4-hex upper; `NoopLoopGuard` `bytes.Equal`. Done: prefix ok; equal aborts.
- [x] 1.3 Reuse `filemerge/ComputeHash` (`nil->e3b0...`) and `WriteFileAtomic` (no Mkdir). Done: import only.

## Phase 2: Core

- [x] 2.1 Create `parser.go` `Parse` regex `^(PUT|CUT)\s+(<N|N.=M:)\s+#([0-9a-fA-F]{4})\b`. Done: valid pass; `#ZZZZ`/missing fail.
- [x] 2.2 Add `ValidateSeen` in `parser.go`. Done: `[1-20]` + `50.=60`->error; `10.=15`->pass.
- [x] 2.3 Implement `Apply` in `apply.go`: `ValidateSeen`->`NoopLoopGuard`->`ComputeHash` vs `#A1B2` -> match `WriteFileAtomic` else `HashMismatchError`. Done: match writes; mismatch unchanged.
- [x] 2.4 Add `CUT` branch in `Apply`: match removes range; mismatch preserves. Done: `CUT 5.=8` removes.
- [x] 2.5 Batch-safe: mismatch continues; `WriteFileAtomic` error preserves original. Done: A stale B fresh->B ok.

## Phase 3: Integration

- [x] 3.1 Add `edit.mode=hashline` flag in `internal/sdd/apply.go`. Done: off->legacy on->hashline.
- [x] 3.2 Add read hook capturing `seenRanges`+`Capture`; `defer Clear()` after batch. Done: fills seen; end clears.
- [x] 3.3 Wire `Parse`->`ValidateSeen`->`Apply`; mismatch->`freshHash`; parse error->fallback. Done: valid writes; stale warn.

## Phase 4: Testing

- [x] 4.1 Parser unit `parser_test.go` valid/invalid tags. Done: `go test ./internal/edit/hashline -run TestParse -count=1` pass.
- [x] 4.2 Seen-guard unit `[1-20]`. Done: `go test ./internal/edit/hashline -run TestValidateSeen -count=1` pass.
- [x] 4.3 `ComputeHash` 100-line 10-20!=whole, empty->`e3b0...`. Done: `go test ./internal/edit/hashline -run TestComputeHash -count=1` pass.
- [x] 4.4 Snapshot unit `Restore`/bounded/`Clear`. Done: `go test ./internal/edit/hashline -run TestSnapshot -count=1` pass.
- [x] 4.5 `NoopLoopGuard` equal vs differ. Done: `go test ./internal/edit/hashline -run TestNoopLoopGuard -count=1` pass.
- [x] 4.6 Integration `apply_test.go` `t.TempDir` match/mismatch `errors.As(HashMismatchError)` batch-safe. Done: `go test ./internal/edit/hashline -run TestApply -count=1` pass.
- [x] 4.7 Gates: routing, token >=60%, `go vet`+`go test ./... -count=1 -timeout 180s`, `wc -l <400`. Done: full pass.

## Phase 5: Cleanup

- [x] 5.1 No `filemerge` edits; `gofmt` clean. Done: `gofmt -l` empty; diff only hashline+hook.
- [x] 5.2 Verify single-commit revert. Done: revert clean.

## Dependencies

`1.x`->`2.x`->`3.x`->`4.x`->`5.x`. Threat N/A.

## Evidence

`go test ./... -count=1 -timeout 180s` | `go test ./internal/edit/hashline -run Test<Name> -count=1` | `go vet ./...`
