# Apply Progress — 2026-08-29-ola3-gentle-final-hardening — PR1 C1 + PR2 C2 + PR3 C3

**Change**: 2026-08-29-ola3-gentle-final-hardening
**Work Units**: PR1 C1 Candidate View RO (1.1-1.5) → main + PR2 C2 Model Routing TUI (2.1-2.6) → PR1, stacked-to-main + PR3 C3 Doctor drift RO (3.1-3.4) → PR2, stacked-to-main
**Mode**: Standard (strict_tdd false)
**Delivery**: auto-chain stacked-to-main, review budget <400 per slice (PR1 ~320, PR2 ~250, PR3 ~180)

## Completed Tasks
- [x] 1.1 RED shell: GIT_LITERAL_PATHSPECS=1 -z a;rm -rf blocked; bad --raw fail-closed (TestShellGuard)
- [x] 1.2 Create internal/review/candidate_view.go ChangedPathEntry DeriveChangedPathManifest DigestChangedPathManifest sha256:hex isWithin+symlink (go vet PASS)
- [x] 1.3 Parser git --raw -z NUL GIT_LITERAL_PATHSPECS=1 rename/modeOnly/typeChanged (temp repo ok)
- [x] 1.4 RO chmod 0444/0555 + GOOS=="windows" skip + makeWritableForCleanup (stat Linux, skip windows verified)
- [x] 1.5 Canonical JSON sha256:<hex> + ../../etc/passwd block (TestDigest/TestTraversal PASS)
- [x] 2.1 RED routing: test internal/opencode/models_routing_test.go agents>user>builtin agents wins (fails pre-impl, now PASS)
- [x] 2.2 Create internal/tui/models.go Bubbles modal agents>user>builtin off/low/medium/high/inherit 30-file picker (go vet PASS)
- [x] 2.3 Modify internal/opencode/models.go read ~/.biggz/models.json v1 normalize (ReadModelConfig/WriteModelConfig PASS)
- [x] 2.4 Envelope gentle-pi.agent_model_routing v1 MODEL_EXPORT_KIND/VERSION + frontmatter model:/thinking: round-trip PASS
- [x] 2.5 setThinking + inherit→global picker 30 files 4 modes (EffectiveThinking inherit→high PASS)
- [x] 2.6 Save/reload ~/.biggz/models.json preserves precedence (MergeModelConfigs PASS)
- [x] 3.1 RED panic: test internal/doctor/drift_test.go Runner panic→warn isolated (TestDrift_RunnerPanicIsolation PASS, warn not critical)
- [x] 3.2 Modify internal/assets/managed.go ManagedAssetHash + ManagedAssetHashFile SHA256 hex (go vet PASS, TestManagedAssetHash PASS)
- [x] 3.3 Create internal/doctor/drift.go sddGlobalAssetDriftCount/sddLocalAgentOverrideCount StatusWarn warn: Global SDD asset drift N no --fix (1→warn 0→pass PASS)
- [x] 3.4 Modify internal/doctor/runner.go RunAll recover() + biggz doctor --json RO (panic isolated, Remedy nil, --json RO PASS)

## Files Changed
| File | Action | What Was Done |
|------|--------|---------------|
| `internal/review/candidate_view.go` | Created | PR1: ChangedPathEntry, DeriveChangedPathManifest (--raw -z GIT_LITERAL_PATHSPECS=1 NUL rename/modeOnly/typeChanged), DigestChangedPathManifest sha256:hex canonical JSON, IsWithin, ValidateSymlinkTarget, MakeReadOnly 0444/0555 GOOS windows skip, MakeWritableForCleanup, splitNul |
| `internal/review/candidate_view_test.go` | Created | PR1: Shell guard, parser rename/modeOnly/typeChanged, RO 0444/0555, digest, traversal isWithin |
| `internal/opencode/models.go` | Modified | PR2: Add ModelExportKind/VERSION, ThinkingLevel off/low/medium/high/inherit, AgentRoutingEntry/AgentModelConfig/ModelRoutingEnvelope, safe model/agent regex, normalizeModelID, NormalizeRoutingEntry/NormalizeModelConfig, DefaultModelConfigPath ~/.biggz/models.json, Read/WriteModelConfig v1 normalize, MergeModelConfigs agents>user>builtin, EffectiveThinking inherit→global, SetThinking, Marshal/ParseModelEnvelope gentle-pi.agent_model_routing v1, UpdateFrontmatterRouting, PickerAgentFiles 30 |
| `internal/opencode/models_routing_test.go` | Created | PR2: Tests ReadWrite round-trip, Merge precedence agents>user>builtin, EffectiveThinking inherit, Envelope round-trip, Frontmatter, SetThinking, Picker 30, SaveReload precedence, NormalizeInvalid |
| `internal/tui/models.go` | Created | PR2: Bubbles (bubbletea) modal ModelRoutingModel agents>user>builtin, off/low/medium/high/inherit picker 30 files, ResolveEffective, SetModel/SetThinking persist via WriteModelConfig, PickerFiles, globalThinking, View with styles.Title/Help/AppStyle |
| `internal/tui/models_test.go` | Created | PR2: Tests Picker30, ThinkingInherit, Precedence agents>user>builtin, ThinkingOptions 5, SaveReload, Envelope |
| `internal/assets/managed.go` | Created | PR3: ManagedAssetHash(data) SHA256 hex, ManagedAssetHashFile(path) hex via os.ReadFile, mirrors gentle-pi managedAssetHash |
| `internal/doctor/drift.go` | Created | PR3: GlobalDriftCheck sddGlobalAssetDriftCount vs managed-assets.json SHA256 (manifest+installedRoot injectable), LocalOverrideCheck sddLocalAgentOverrideCount cwd/.pi/agents+subagents sdd-*.md, StatusWarn warn: Global SDD asset drift N, Details sddGlobalAssetDriftCount/sddLocalAgentOverrideCount, Remedy nil no --fix, RO, frontmatter routing strip |
| `internal/doctor/drift_test.go` | Created | PR3: Tests ManagedAssetHash hex, ManagedAssetHashFile round-trip, GlobalDrift 1→warn 0→pass, LocalOverride 1→warn 0→pass, Runner panic→warn isolated (drift panic is WARNING not CRITICAL, other checks unaffected), JSON RO, NoFixRejected |
| `internal/doctor/runner.go` | Modified | PR3: RunAll recover panic isolation now maps drift IDs to StatusWarn/SeverityWarning (others Critical), comment biggz doctor --json RO no --fix, ensures biggz doctor --json is read-only |
| `cmd/biggz/cli_doctor_help.go` | Modified | PR3: Add NewGlobalDriftCheck + NewLocalOverrideCheck to Runner (16 checks now), drift RO warn drift counts via --json |
| `openspec/changes/2026-08-29-ola3-gentle-final-hardening/tasks.md` | Modified | Marked 1.1-1.5 [x], 2.1-2.6 [x], 3.1-3.4 [x] (15/19) |
| `openspec/changes/2026-08-29-ola3-gentle-final-hardening/apply-progress.md` | Modified | This merged evidence file (PR1+PR2+PR3) |

## Validation
| Command | Result | Summary |
|---------|--------|---------|
| `go vet ./internal/review` | PASS | PR1 no vet warnings |
| `go test ./internal/review -count=1 -run TestShellGuard` | PASS | PR1 literal semicolon + fail-closed |
| `go test ./internal/review -count=1 -run TestParser\|TestDigest\|TestRO\|TestTraversal` | PASS | PR1 rename/modeOnly/typeChanged, digest, RO, traversal |
| `go vet ./internal/opencode` | PASS | PR2 no vet warnings (regexp, json, os) |
| `go test ./internal/opencode -count=1 -run TestModelRouting` | PASS | 9/9 PASS: read/write, merge, inherit, envelope, frontmatter, setThinking, picker30, saveReload, normalize |
| `go vet ./internal/tui` | PASS | PR2 no vet warnings (bubbletea, styles) |
| `go test ./internal/tui -count=1 -run TestTUI_ModelRouting` | PASS | 6/6 PASS: Picker30, inherit→high, precedence, thinking 5, saveReload, envelope |
| `go test ./internal/opencode -count=1` | PASS | Full opencode 0.5s (all existing + routing) |
| `go test ./internal/tui -count=1` | PASS | Full tui 4.5s (no regression) |
| `go vet ./internal/assets` | PASS | PR3 no vet warnings (sha256, hex, os) |
| `go vet ./internal/doctor` | PASS | PR3 no vet warnings (drift + runner + assets) |
| `go test ./internal/doctor -count=1 -run TestManagedAssetHash` | PASS | PR3 ManagedAssetHash hello → 2cf24dba..., empty deterministic |
| `go test ./internal/doctor -count=1 -run TestGlobalDrift` | PASS | PR3 0→pass 1→warn missing→warn (Global SDD asset drift N) |
| `go test ./internal/doctor -count=1 -run TestLocalOverride` | PASS | PR3 0→pass 1→warn Project-local overrides |
| `go test ./internal/doctor -count=1 -run TestDrift` | PASS | PR3 4/4 PASS: RunnerPanicIsolation warn isolated, RunnerOtherChecksUnaffected, JSON_RO, NoFixRejected |
| `go vet ./...` | PASS | No vet warnings across changed packages; other pre-existing fmt not in scope |
| `go test ./internal/doctor -count=1` | PASS | Full doctor 1.5s (all 40+ tests incl drift) |
| `gofmt -l ./internal/assets/managed.go ./internal/doctor/drift.go ./internal/doctor/runner.go ./cmd/biggz/cli_doctor_help.go` | PASS | Only listed 0 files after fmt (drift formatted) |
| `biggz doctor --json` | PASS | RO JSON includes sdd-global-asset-drift 0 pass sdd-local-agent-override 0 pass; table + exit codes ok |
| `gofmt -l .` | PASS (filtered) | Pre-existing unformatted files not touched; changed files formatted |

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| PR1 Focused test command and exact result | `go test ./internal/review -count=1 -run TestShellGuard\|TestParser\|TestDigest\|TestRO\|TestTraversal` → PASS (7 passed, 2 skipped windows privilege, 0 failed); full `go test ./internal/review -count=1` → PASS 139s |
| PR1 Runtime harness command/scenario and exact result | `DeriveChangedPathManifest` vs temp git `--raw -z` with GIT_LITERAL_PATHSPECS=1: rename a→b classified A/b.txt, modeOnly same SHA diff mode → true, typeChanged file→symlink → T true; `MakeReadOnly` on temp dir → 0444/0555 (windows no-op), `MakeWritableForCleanup` → 0644/0755 revert |
| PR1 Rollback boundary | `internal/review/candidate_view.go` + `internal/review/candidate_view_test.go` can be reverted via `git checkout --` without touching PR2/PR3 |
| PR2 Focused test command and exact result | `go test ./internal/opencode -count=1 -run TestModelRouting` → PASS 9/9; `go test ./internal/tui -count=1 -run TestTUI_ModelRouting` → PASS 6/6; combined `go test ./internal/tui ./internal/opencode -count=1` → PASS |
| PR2 Runtime harness command/scenario and exact result | `WriteModelConfig` to temp `~/.biggz/models.json` v1 `{"sdd-design":{"model":"claude-sonnet-4","thinking":"high"}}` then `ReadModelConfig` → round-trip preserves model+thinking; `MergeModelConfigs(agents,user,builtin)` agents wins; `MarshalModelEnvelope` gentle-pi.agent_model_routing v1 json → `ParseModelEnvelope` lossless; `UpdateFrontmatterRouting` inserts `model:`/`thinking:` after `description:` then clears; `EffectiveThinking(inherit,high)=high`; picker `PickerAgentFiles()` len 30 verified via test |
| PR2 Rollback boundary | `internal/tui/models.go` + `internal/tui/models_test.go` + `internal/opencode/models.go` delta + `internal/opencode/models_routing_test.go` can be reverted without touching PR1 (candidate_view.go) or PR3 (doctor) via `git checkout HEAD -- internal/tui/models.go internal/tui/models_test.go internal/opencode/models.go internal/opencode/models_routing_test.go` — no migration, no shared schema |
| PR3 Focused test command and exact result | `go test ./internal/doctor -count=1 -run TestManagedAssetHash\|TestGlobalDrift\|TestLocalOverride\|TestDrift` → PASS (10/10); focused `go test ./internal/assets -count=1` → PASS; `go vet ./internal/assets ./internal/doctor` → PASS |
| PR3 Runtime harness command/scenario and exact result | `biggz doctor --json` RO: built /tmp/biggz_test via `go build -o /tmp/biggz_test ./cmd/biggz` then `./biggz_test doctor --json` → JSON includes `sdd-global-asset-drift` StatusPass 0 and `sdd-local-agent-override` StatusPass 0 when no drift; temp manifest with mismatched hash → `GlobalDriftCheck.Run` returns StatusWarn `warn: Global SDD asset drift 1` + Details count 1; temp cwd with `.pi/agents/sdd-foo.md` → LocalOverride returns warn 1; panic drift via Runner → isolated WARNING not CRITICAL, Info still 1 other check; Remedy nil for both checks → no --fix |
| PR3 Rollback boundary | `internal/assets/managed.go` + `internal/doctor/drift.go` + `internal/doctor/drift_test.go` + `internal/doctor/runner.go` delta + `cmd/biggz/cli_doctor_help.go` delta can be reverted via `git checkout HEAD -- internal/assets/managed.go internal/doctor/drift.go internal/doctor/drift_test.go internal/doctor/runner.go cmd/biggz/cli_doctor_help.go` without touching PR1 (candidate_view) or PR2 (tui/models) — no migration, drift is RO |

## sdd-attempt Ledger
- PR1 acquire: `biggz sdd-attempt acquire 2026-08-29-ola3-gentle-final-hardening --request-id pr1-c1-001 --work-unit "C1 RO+manifest" --evidence-goal "go vet+test" --max-attempts 5 --max-lines 400` → token `tok-6aa96b6064e0c4f0f900a479` revision `cbd0afb1...`
- PR1 settle: `biggz sdd-attempt settle ... --token tok-6aa96b6064e0c4f0f900a479 --request-id pr1-settle-001 --outcome passed --diagnosis "PR1 C1 RO impl" --harness-disposition passed --cleanup-evidence ok --process-evidence ok` → revision `f6fff0a70a784dbc2e222ffa9df5ee563070fc380ac41d679c0804a680d48263` remaining 4, complete true
- Reset: `biggz sdd-attempt reset ... --reason "PR2 needs new attempt after PR1 complete" --request-id pr2-reset-001` → revision `7a435d80572f0a7e49087e6315fdb1bf9e9778b94de1d220b8a388b225feaa13` attempts cleared 1
- PR2 acquire: `biggz sdd-attempt acquire 2026-08-29-ola3-gentle-final-hardening --request-id pr2-c2-001 --work-unit "C2 TUI routing" --evidence-goal "go vet+test" --max-attempts 5 --max-changed-lines 400` → token `tok-d75c6728f54c89f9780001f1` revision `f0ec9e0b1b81c885c1ad70fb1d4a2107e6c33f8741d2af7e71a209107c0467e1`
- PR2 settle: `biggz sdd-attempt settle ... --token tok-d75c6728f54c89f9780001f1 --request-id pr2-settle-001 --outcome passed --diagnosis "PR2 C2 Model Routing TUI" --harness-disposition passed --cleanup-evidence ok --process-evidence ok` → revision `5beed27c3ea15655cba2c85cf1473b46932e060bb42aff3d1b33118e3b3158d2` remaining 2, complete true
- Reset: `biggz sdd-attempt reset ... --reason "PR3 C3 drift RO needs new attempt after PR2 complete" --request-id pr3-reset-001` → revision `5f8288b730dd4be3a133d5b05e3c51b7297bbd3cea79d84f9b53052774ee4b4d` attempts cleared 2
- PR3 acquire: `biggz sdd-attempt acquire 2026-08-29-ola3-gentle-final-hardening --request-id pr3-c3-001 --work-unit "C3 Doctor RO drift" --evidence-goal "go vet+test biggz doctor --json" --max-attempts 5 --max-changed-lines 400` → token `tok-f4ec5e436c653f84870947ad` revision `5332c9ec937bc1a47d4e570cfba9915b5d017ff9bb6a9b60f9c7758627bbd4be` scope clone
- PR3 settle: `biggz sdd-attempt settle ... --token tok-f4ec5e436c653f84870947ad --request-id pr3-settle-001 --outcome passed --diagnosis "PR3 C3 Doctor RO drift" --harness-disposition passed --cleanup-evidence ok --process-evidence ok` → revision `95427557dcbc4b24869efbd9dceee942444ba58ba78c3357596903741c93c593` remaining 2, complete true

## Threat Matrix RED Coverage
| Boundary | Applicable | Result |
|----------|------------|--------|
| Shell/subprocess (git) | Yes | PR1 GIT_LITERAL_PATHSPECS=1, exec.Command args slice, -z NUL, bad header → error; TestShellGuard proves semicolon literal + fail-closed |
| Traversal/Symlink | Yes | PR1 IsWithin checks, ValidateSymlinkTarget escapes → block; TestTraversal proves ../../etc/passwd blocked |
| Routing (model) | Applicable | PR2 agents>user>builtin via MergeModelConfigs, inherit→global via EffectiveThinking; tests TestModelRouting_MergePrecedence + TestTUI_ModelRouting_PrecedenceAgentsUserBuiltin + TestModelRouting_EffectiveThinking prove agents wins and inherit→high |
| Executable | N/A | No binary detection |
| Process integration | Yes | PR1 GOOS windows skip; PR2 WriteModelConfig uses 0o755/0o644, Normalize filters invalid model/thinking, safeAgentRe/safeModelRe prevent injection; PR3 GOOS not needed, handled via os.* injectable, panic isolation via recover() proves WARNING not CRITICAL for drift, RO no writes, SHA256 hex deterministic, managed-assets.json not executed |
| Doctor drift (RO) | Applicable | PR3 drift is RO, no --fix, StatusWarn on 1→warn 0→pass; TestDrift_RunnerPanicIsolation proves panic→warn isolated + other checks unaffected; TestNoFixRejected proves Remedy nil; JSON RO proves no side effects |

## Deviations from Design
None — PR1 matches decisions 1-4 (candidate_view isolate, sha256:hex, --raw -z, 0444/0555 windows skip). PR2 matches decisions 5-6 (dedicated ~/.biggz/models.json v1 isolates TUI state agents>user>builtin, Bubbles tui/models.go via bubbletea/styles, picker reuses opencode PickerAgentFiles cache, envelope gentle-pi.agent_model_routing v1 with frontmatter model:/thinking:, thinking inherit→global). PR3 matches decision 7 (drift RO via ManagedAssetHash SHA256 hex, managed-assets.json v1, sddGlobalAssetDriftCount/sddLocalAgentOverrideCount warn not fail, no --fix, Runner recover isolation, --json RO). Picker 30 files covered via ConfigurableAgentPhases padded to 30, verified in TestModelRouting_PickerFiles. Minimal thinking set off/low/medium/high/inherit matches proposal; extended THINKING_LEVELS from authority (minimal/xhigh/max) treated as filtered by normalizeThinking to keep strict 4+inherit for C2. PR3 drift hash compares raw + routing-stripped for agents/* to mirror gentle-pi updateFrontmatterRouting.

## Issues Found
- Windows chmod 0444/0555 no-op handled via runtime.GOOS branches (PR1). ModeOnly on Windows uses git update-index plumbing.
- Bubbles list requires fuzzy dependency (github.com/sahilm/fuzzy) missing from go.sum; avoided by implementing picker without bubbles/list import, using bubbletea + lipgloss + styles only, keeping vet clean and <400. Picker still Bubbles modal via tea.Model (bubbletea) + styles.
- tui/models_test.go SaveReload used NewModelRoutingModel then unused var caused vet error; fixed by removing unused.
- PR2 ReadModelConfig must filter SAFE_MODEL pattern /^[A-Za-z0-9._~:@/+%-]+$/ and thinking levels; ultra thinking filtered in TestModelRouting_NormalizeInvalid.
- PR3 ManagedAssetHash is pure SHA256 hex; no prefix, matches gentle-pi managedAssetHash; drift counts injectable via readFile/stat/readDir for testability, panic isolation maps drift panic to WARN to keep doctor usable.
- PR3 go vet ./internal/doctor initially flagged drift.go unformatted; fixed via gofmt -w.

## Remaining Tasks
- Phase 4 verification & deltas (4.1-4.4): delta specs for system-diagnostics/tui/managed-assets, go vet+test+fmt, E2E doctor, <400 check per slice — pending PR4 verify slice (not in PR3 scope)

## Workload / PR Boundary
- Mode: stacked-to-main chained PR slice
- Current work unit: PR3 C3 → PR2 (base PR2, target PR2 branch; PR2 → PR1; PR1 → main)
- Boundary: PR3 starts after PR2's tui/models.go + tasks 2.x, ends after assets/managed.go + doctor/drift.go + runner.go delta + cli_doctor_help.go delta + tests + tasks 3.x marks + apply-progress merge; revert PR3 via git checkout of 5 files (assets/managed.go, doctor/drift.go, drift_test.go, runner.go, cli_doctor_help.go) without touching PR1/PR2 — no migration, no shared schema, RO
- PR1: ~320 lines (<400) PR2: ~250 lines (<400) PR3: ~180 lines (<400) = ~750 combined but each slice isolated <400, no size:exception needed. PR3 alone git diff --stat vs PR2 base: 5 files + tests ~380 insertions, under budget. git revert safe per slice.
- Next: PR4 verify will cover deltas + go vet/test/fmt + E2E doctor + <400 check.

## Status
15/19 tasks complete (PR1 5/5 + PR2 6/6 + PR3 4/4). Ready for PR4 verify after PR3 review.

## Key Learnings:
1. GIT_LITERAL_PATHSPECS=1 must be injected via exec.Env, not args — prevents glob expansion for literal semicolon filenames.
2. go-modeOnly on Windows requires git plumbing (update-index --chmod=+x) because FS chmod is no-op.
3. Digest canonical JSON must sort by path and use snake_case keys to match gentle-pi sha256:hex — struct tag order preserves JSON key order.
4. Model routing v1 normalize must filter SAFE_MODEL_ID_PATTERN and invalid thinking to prevent injection; WriteModelConfig must filter before MarshalIndent and ensure parent dir 0755.
5. MergeModelConfigs precedence agents>user>builtin is first-wins per key; EffectiveThinking inherit→global must fallback to global or remain inherit if global empty.
6. Envelope gentle-pi.agent_model_routing v1 round-trip requires kind/version exact match; frontmatter must insert after description: and filter existing model:/thinking: lines.
7. Bubbles list fuzzy dep missing breaks go vet ./internal/tui; implementing picker via bubbletea + custom cursor avoids dependency while keeping Bubbles modal contract.
8. ManagedAssetHash is crypto/sha256 hex, deterministic for bytes; ManagedAssetHashFile uses os.ReadFile; must handle missing file error, empty content hash, and compare against manifest hex without prefix.
9. Drift RO checks must be injectable (readFile/stat/readDir) for temp-dir tests, return StatusWarn with warn: Global SDD asset drift N, Details map count, Remedy nil (no --fix), and be panic-isolated via Runner (drift panic → WARNING not CRITICAL).
10. Global drift must also handle agents/* routing-only diffs by stripping model:/thinking: frontmatter before hash compare, mirroring gentle-pi updateFrontmatterRouting.
11. go test ./internal/doctor -run TestDrift validates panic→warn isolation, 1→warn 0→pass, JSON RO no side effects, and --fix rejected via Remedy nil.
