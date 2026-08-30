# Tasks: 2026-08-30-ola2-guardrails-preflight-synthesis — Ola 2 Guardrails / Preflight / Synthesis Gate

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~470 (251 + 152 + 67) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No (single PR) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No — retroactive formalization of commit `9f6c8be` (already on `main`); `~470` `Medium` single PR documented as `size:exception-ok` lineage or within `stacked-to-main` single slice tolerance.
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Guardrails + preflight + synthesis (single slice) | PR1 (~470, base: main) | `go vet ./...` + `go test ./internal/policy ./internal/sdd -count=1 -timeout 60s` | Manual: `IsDenied` rm/push, `Classify` autonomous, `LoadRuntimeGuardrailsConfig` merge, `WriteSddPreflightToDisk→Read` round-trip, `ShouldBlock` 120s window | `git revert 9f6c8be` — removes `internal/policy/guardrails.go`, `internal/sdd/preflight.go`, `internal/sdd/synthesis_gate.go` |

## Phase 1: Foundation — Bash Deny + Guarded Classification

- [x] 1.1 Create `internal/policy/guardrails.go` `deniedBashPatterns[6]` and `IsDenied` with index `2` (`git clean` requires `-f`/`--force` + `-d`/`--directories`) and index `3` (`git push` requires `--force` or ` -f`) refinement; validate patterns block `rm -rf /`, `~`, `$HOME`, `..`, `git reset --hard`, `chmod -R 777`, `chown -R` and unblocked without flags
- [x] 1.2 Add `guardedKeyPatterns[5]` (`gitPush`, `gitRebase`, `gitBranchDeleteForce`, `npmPublish`, `piRemove`), `autonomousDefaultActions` (`gitPush allow`, `npmPublish block`, others `confirm`), `Guard*` consts, `RuntimeGuardrailsConfig` + `safeGuardrailsConfig`; implement `ClassifyGuardedCommand` with deny-first `block`, then `!autonomousMode→confirm`, else `guardedCommands` override else defaults, else `not-guarded`
- [x] 1.3 Add `pathGuardedTools`/`pathInputKeys`, `sensitivePathPatterns[8]` (`\.ssh`, `\.credentials`, `library/keychains`, `\.aws/credentials`, `\.config/gh/hosts.yaml`, `secrets/`, `\.env`, `.(pem|key|p12|pfx)`), `isSensitivePath` (`TrimSpace`, `\→/`, `ToLower`, `~→HOME`), `collectPathInputs` recursive + `EvaluateSensitivePathTool` returning `&ToolCallDecision{Block,…}` on match

## Phase 2: Core — Config Parse, Merge, and Preflight Canonicalize

- [x] 2.1 Implement `ParseGuardrailsConfigFile` (`map[string]any` unmarshal, `validActions {allow,confirm,block}` × `validKeys {5}` allowlist filter, `(nil,false)` on malformed JSON) and `LoadRuntimeGuardrailsConfig` (`GENTLE_PI_AUTONOMOUS_MODE=1` fast-path, `gentlePiConfigHome()` via `GENTLE_PI_CONFIG_HOME` or `~/.pi/gentle-ai`, read `global` then `project`, `safeGuardrailsConfig` on either malformed, merge project `autonomousMode` wins + `guardedCommands` merged, `nil→map` guard)
- [x] 2.2 Create `internal/sdd/preflight.go` `PreflightPrefs` + `preflightCache`, `SddPreflightDiskPath` (explicit `home[0]` > `GENTLE_PI_CONFIG_HOME` > `UserHomeDir/.pi/gentle-ai` → `sdd-preflight.json`), `NormalizePreflightArtifactStore` (`both/hybrid/engram/bigmem→hybrid`, `openspec→openspec`, `none→""`), `canonicalizePrefs` (`interactive`/`stacked-to-main`/`400` defaults)
- [x] 2.3 Implement `WriteSddPreflightToDisk` (`canonicalize` + `MkdirAll 0755` + `MarshalIndent` + `WriteFile 0644` `\n`), `ReadSddPreflightToDisk` (`ReadFile` + `Unmarshal` + `canonicalize` + `(p,true)`/`(_,false)`), `Set/Get/Clear/ResolvePreflightPrefs` precedence `cache > disk > defaults {interactive,openspec,stacked-to-main,400}`, `PreflightQuestionEnvelope` + `ValidatePreflightQuestionEnvelope` enums, `SessionRecallMarkdown`, `PreflightSequence`, helpers `preflightItoa`/`jsonNumber`

## Phase 3: Integration — Synthesis Gate Markers and 120s Window

- [x] 3.1 Create `internal/sdd/synthesis_gate.go` `synthesisMarkers[4]` (`## Sub-agent Result:`, `**What was done:**`, `**Artifacts/Paths:**`, `**Next Recommended:**`), globals `currentTurnMarkdown` + `currentTurnTime`, `SetCurrentTurnMarkdown`, `HasSynthesis` (all 4), `HasSessionRecall` (`## Session Recall`), `IsChildBypass` (`PI_SUBAGENT_CHILD==1`), `IsCheckpointAsk` (`proceed/adjust/stop/continue/correct` lower)
- [x] 3.2 Implement `ShouldBlock(question, md, now)` (`false` if child/sessionRecall/notCheckpoint/`now-sub>120s`, else `!HasSynthesis`) and `CheckSynthesisPrecondition(question, md)` (`ShouldBlock(...,time.Now())` + message `synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)`)

## Phase 4: Verification

- [x] 4.1 Verify `go vet ./...` PASS and `gofmt -l` clean for `internal/policy/guardrails.go`, `internal/sdd/preflight.go`, `internal/sdd/synthesis_gate.go` (retroactive: commit `9f6c8be` pre-verified)
- [x] 4.2 Verify focused harness `go test ./internal/policy ./internal/sdd -count=1 -timeout 60s` PASS (guardrails deny/classify, config merge with `TempDir` homes, sensitive path tool guard, preflight canonicalize/persist, synthesis gate `120s` window with injectable `now`) under `strict_tdd false`
- [x] 4.3 Verify `git show 9f6c8be --stat` shows `470` lines (`251 + 152 + 67`) and `git diff HEAD -- internal/policy/guardrails.go internal/sdd/preflight.go internal/sdd/synthesis_gate.go` clean (no drift beyond `9f6c8be`)
- [x] 4.4 Verify `biggz sdd-status --json` after artifact creation shows this change `artifacts proposal done specs done design done tasks done`, `taskProgress allComplete true`, `applyState all_done`, `nextRecommended verify`

