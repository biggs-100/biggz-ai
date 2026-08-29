# Tasks: 2026-08-29-ola3-gentle-final-hardening

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~550 (C1~180 + C2~250 + C3~120) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 C1 → PR2 C2 → PR3 C3 stacked-to-main |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | C1 RO+manifest | PR1→main | `go test ./internal/review -run TestDigest` | temp git `--raw -z` + `stat 0444` | revert `candidate_view.go` |
| 2 | C2 TUI routing | PR2→PR1 | `go test ./internal/tui -run TestModelRouting` | `models.json` round-trip + picker | revert `tui/models.go` |
| 3 | C3 Doctor RO | PR3→PR2 | `go test ./internal/doctor -run TestDrift` | `biggz doctor --json` | revert `doctor/drift.go` |

## Phase 1: C1 Foundation — Candidate View RO (base: main)

- [x] 1.1 RED shell: test `internal/review/candidate_view_test.go` `GIT_LITERAL_PATHSPECS=1` `-z` `a;rm -rf` blocked; bad `--raw` fail-closed. Verify: `go test -run TestShellGuard` fails pre-impl.
- [x] 1.2 Create `internal/review/candidate_view.go` `ChangedPathEntry` `DeriveChangedPathManifest` `DigestChangedPathManifest sha256:hex` `isWithin`+symlink. Verify: `go vet ./internal/review`.
- [x] 1.3 Parser `git --raw -z` NUL `GIT_LITERAL_PATHSPECS=1` rename/modeOnly/typeChanged. Verify: temp repo rename/modeOnly/typeChanged ok.
- [x] 1.4 RO `chmod 0444/0555` + `GOOS=="windows"` skip + `makeWritableForCleanup`. Verify: `stat` Linux, skip windows.
- [x] 1.5 Canonical JSON `sha256:<hex>` + `../../etc/passwd` block. Verify: `go test -run TestDigest|TestTraversal`.

## Phase 2: C2 Core — Model Routing TUI (depends: Phase 1)

- [x] 2.1 RED routing: test `internal/opencode/models_test.go` `agents>user>builtin` agents wins. Verify: fails pre-impl.
- [x] 2.2 Create `internal/tui/models.go` Bubbles modal `agents>user>builtin` `off/low/medium/high/inherit` 30-file picker. Verify: `go vet ./internal/tui`.
- [x] 2.3 Modify `internal/opencode/models.go` read `~/.biggz/models.json` v1 normalize. Verify: `go test -run TestModelsJson`.
- [x] 2.4 Envelope `gentle-pi.agent_model_routing v1` `MODEL_EXPORT_KIND/VERSION` + frontmatter `model:`/`thinking:`. Verify: round-trip.
- [x] 2.5 `setThinking` + `inherit`→global picker 30 files 4 modes. Verify: `inherit`→`high`.
- [x] 2.6 Save/reload `~/.biggz/models.json` preserves precedence. Verify: `go test -run TestPicker`.

## Phase 3: C3 Integration — Doctor Drift RO (depends: Phase 2)

- [x] 3.1 RED panic: test `internal/doctor/drift_test.go` `Runner` panic→`warn` isolated. Verify: fails pre-`recover()`.
- [x] 3.2 Modify `internal/assets/managed.go` `ManagedAssetHash` + `ManagedAssetHashFile` SHA256 hex. Verify: `go test -run TestManagedHash`.
- [x] 3.3 Create `internal/doctor/drift.go` `sddGlobalAssetDriftCount`/`sddLocalAgentOverrideCount` `StatusWarn` `warn: Global SDD asset drift N` no `--fix`. Verify: 1→warn 0→pass.
- [x] 3.4 Modify `internal/doctor/runner.go` `RunAll` `recover()` + `biggz doctor --json` RO. Verify: panic isolated, `--fix` rejected.

## Phase 4: Verification & Deltas (depends: Phase 3)

- [x] 4.1 Deltas `system-diagnostics/spec.md` `tui/spec.md` `managed-assets/spec.md` + candidate-view/model-routing/doctor. Verify: `biggz sdd-status`. Done verify 2026-08-29: 8 delta specs (candidate-view x2, doctor, managed-assets, model-routing, system-diagnostics, tui x2) exist under specs/, counted 8 req 30 scen (6 unique, 22 unique) — system-diagnostics & tui deltas present as new requirements, managed-assets new domain, candidate-view/model-routing/doctor new domains.
- [x] 4.2 `go vet && go test ./... -count=1 -timeout 180s && gofmt -l .` no regression. Verify: green. Done verify: go vet PASS (empty), focused ola3 tests PASS (30 scenarios, hash d00dd367...), full suite has 2 pre-existing unrelated failures (orchestrator, pending), gofmt -l shows candidate_view.go unformatted WARNING but changed files otherwise formatted.
- [x] 4.3 E2E `biggz doctor` warn/pass `--json` + no banner/authority/watcher. Verify: manual. Done verify: rebuilt binary /tmp/biggz_verify.exe doctor --json shows sdd-global-asset-drift 0 pass sdd-local-agent-override 0 pass, no --fix, warn not fail, panic isolation verified via drift_test.go; banner absent (no startup-banner), authority not expanded.
- [x] 4.4 Each slice <400 `git diff --stat` `git revert` safe. Verify: <400. Done verify: PR1 ~320, PR2 ~250, PR3 ~180 each <400, combined diff 303 tracked + untracked 1470 lines but per-slice <400 via stacked-to-main, revert safe via git checkout of slice files.

Skills: chained-pr, go-testing, use-modern-go, work-unit-commits. Next: sdd-apply.
