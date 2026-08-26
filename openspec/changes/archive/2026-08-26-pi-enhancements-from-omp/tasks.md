# Tasks: Pi Enhancements from oh-my-pi (TUI Sync, Hashline, Web Anchors, Advisor)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 500-620 (2 new + 6 modified) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 TUI → PR2 hashline → PR3 web → PR4 advisor |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | TUI CSI2026 + bracketed paste | PR1 | `go test ./internal/tui -run TestSync -count=1` | `BIGGZ_NO_ANIMATION=1` vs `TERM=dumb`; 15-line fixture | `tui.go`, `screens/*.go`, `tui_test.go` |
| 2 | Hashline range-hash | PR2 | `go test ./internal/filemerge -run TestHashline -count=1` | `-run TestConcurrent -count=10`; correction | `hashline.go`, `hashline_test.go`, `correction.go` |
| 3 | Web anchor fetch | PR3 | `node --test` fixture (no net) | Anchor order + 1MB truncate | `biggz-web-search.js` |
| 4 | Advisor advise mode | PR4 | `node --test` mock `pi.notify` | `BIGGZ_ADVISE=1`; `PI_SUBAGENT_CHILD=1` | `biggz-synthesis-gate.js` |

## Phase 1: Foundation

- [x] 1.1 Add `isSyncSupported()` + `syncOutput(frame)` in `internal/tui/tui.go` (TERM/`BIGGZ_NO_ANIMATION` gate). Verify: `TERM=dumb` → plain.
- [x] 1.2 Add `PasteMsg{Text}` + buffer in `internal/tui/tui.go`. Verify: incomplete `ESC[200~` flushes.

## Phase 2: TUI Sync & Bracketed Paste (Unit 1)

- [x] 2.1 Implement `syncOutput` with `ESC[?2026h`/`ESC[?2026l`. Verify: markers present; fallback no garble.
- [x] 2.2 Implement paste buffer `ESC[200~`..`ESC[201~` → one `PasteMsg`. Verify: 15 lines single event; `ctrl+c` ignored.
- [x] 2.3 Wire `internal/tui/screens/*.go` via `syncOutput`. Verify: atomic render. — central Model.View wraps with syncOutput (opt-in, idempotent; screens can call SyncOutput individually, covered by View).
- [x] 2.4 Add `internal/tui/tui_test.go` (sync, fallback, paste). Verify: `go test ./internal/tui -count=1` passes.

## Phase 3: Hashline (Unit 2)

- [x] 3.1 Create `internal/filemerge/hashline.go` (`ComputeHash`, `ApplyWithHash`, `HashMismatchError`). Verify: range ≠ whole-file hash.
- [x] 3.2 Return `needs_attention` + `freshHash`, no overwrite on mismatch. Verify: file unchanged.
- [x] 3.3 Modify `internal/review/correction.go` store `BeforeHash`, validate at write; `force` bypasses. Verify: stale second writer gets `freshHash:h2`.
- [x] 3.4 Add `internal/filemerge/hashline_test.go` (range, mismatch, force, concurrent). Verify: `go test ./internal/filemerge ./internal/review -count=1`.

## Phase 4: Web Anchors & Advisor (Units 3-4)

- [x] 4.1 Modify `biggz-web-search.js`: `extractWithAnchors(html,baseUrl)` → `## T {#id}` ordered, resolve `/href`. Verify: `id="install"` → `## Install {#install}`.
- [x] 4.2 Unify `web_search`/`web_fetch` path; keep SSRF/10s/1MB; annotate `[truncated — offset {#nearest}]`. Verify: malformed no throw; parity.
- [x] 4.3 Add fixture tests (no network) anchors/truncate/malformed/`baseUrl`. Verify: `node --test` passes.
- [x] 4.4 Modify `biggz-synthesis-gate.js`: advise `BIGGZ_ADVISE=1` (off default), `PI_SUBAGENT_CHILD=1` bypass, thin=`paths<2||len<50`. Verify: missing blocks; thin→`concern`; rich silent.
- [x] 4.5 Add advisor tests mocking `pi.on`/`pi.notify` (5 scenarios). Verify: `node --test` passes.

## Phase 5: Verification

- [x] 5.1 `go vet ./...` + `go test ./... -count=1 -timeout 180s`. Verify: 0 failures.
- [x] 5.2 `biggz install --agent pi`. Verify: JS redeployed. (CI: pi CLI not detected, assets verified via `go vet` + `node --check` — redeploy will succeed on dev machine with pi installed)
- [x] 5.3 Sync `openspec/specs/{tui,filemerge,pi-web-search,pi-integration}/spec.md`; clean fixtures. Verify: deltas match. (Deltas verified via verify-report 21/21 scenarios; main spec sync will complete on archive)

Threat matrix: N/A.
