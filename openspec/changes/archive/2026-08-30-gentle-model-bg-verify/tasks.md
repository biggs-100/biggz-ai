# Tasks: 2026-08-30-gentle-model-bg-verify

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~600 (PR1 340 + PR2 260) |
| 400-line budget risk | Medium (total >400, per-PR Low) |
| Chained PRs recommended | Yes |
| Suggested split | PR1 Picker → PR2 Background+Verify |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Picker THINKING_LEVELS+cache+envelope+TUI30 | PR1 → main | `go test ./internal/opencode -run TestModelRouting` | `biggz models` → `cat ~/.biggz/models.json` | Revert `opencode/models.go`, `tui/models.go`, `model-variants.ts` |
| 2 | Background 4-source + Verify + goreleaser | PR2 → main | `go test ./internal/agents/pi -run TestBackground; go test ./internal/install -run TestVerify` | `goreleaser --snapshot --clean` + `sha256sum -c dist/checksums.txt` | Revert `sdd/background.go`, `install/verify.go`, `.goreleaser.yaml` |

## Phase 1: Foundation

- [x] 1.1 `opencode/models.go`: `biggz-ai.agent_model_routing` v1, `ThinkingLevel off|low|medium|high|inherit` + `IsValid` (S1)
- [x] 1.2 `normalizeModelID`/`normalizeThinking`/`NormalizeModelConfig` drop `bad model with spaces`+`ultra` (S5)
- [x] 1.3 `model-variants.ts` `tmp→rename` `randomBytes` + `sort()` + `JSON.stringify(null,2)`

## Phase 2: PR1 Picker RED+Impl

- [x] 2.1 RED `ThinkingInherit` inherit+high→high, off verbatim, empty→inherit (S2)
- [x] 2.2 `EffectiveThinking`+`MergeModelConfigs` agents>user>builtin + `SetThinking` (S1)
- [x] 2.3 RED `CacheEnrich` sorted `Record<provider,Record<model,string[]>>` exact→fallback (S6)
- [x] 2.4 `LoadVariants`/`LoadVariantsOrEmpty`/`EnrichWithVariants` sorted fallback, missing→empty (S7/S8)
- [x] 2.5 RED `Envelope` kind v1 bad kind/version→error (S10/S11)
- [x] 2.6 `MarshalEnvelope`/`ParseEnvelope`/`ReadModelConfig`/`WriteModelConfig` sorted atomic + `UpdateFrontmatter` lossless + walk_test sorted nil clears (S9/S12)

## Phase 3: PR1 TUI Wiring

- [x] 3.1 `tui/models.go` BubbleTea table via `Merge`/`EffectiveThinking`/`PickerAgentFiles()==30` orchestrator+SDD+JD+review (S4)
- [x] 3.2 Wire `LoadVariants`→picker enrich, fixture `claude-sonnet-4:[low,high]`→sorted `EffortLevels()` (S6)

## Phase 4: PR2 Background Canonical

- [x] 4.1 `sdd/background.go` `ResolvePolicy(cwd)` project `.biggz/bg.json` > global `.biggz/` (`BIGGZ_CONFIG_HOME`>`GENTLE_PI`) > env > off max2 reads (S1)
- [x] 4.2 Strict 2-key `{"schema":"gentle-pi.background-subagents/v1","policy":"on"|"off"}` extra→malformed→off no fallback + bad JSON→off (S2/S3)
- [x] 4.3 `ResolveCapability` ready if `subagent_run` else absent; `BIGGZ`>`GENTLE_PI`; status `background subagents: <policy> (decided by <source>; capability: <capability>)` (S7)
- [x] 4.4 Delegate `opencode/background.go`+`pi/adapter.go` to `sdd`, `.biggz/` paths, `RenderReport` source/malformed/capability disabled|unmanaged warn outranked (S4-S6)

## Phase 5: PR2 Verify Canonical + Release

- [x] 5.1 RED `isConfined` outside versionDir→fail, `isSymlink` binary/manifest/dirs→ `PackageLocalGentleAiBinaryMissingError` (S3/S4)
- [x] 5.2 RED `sameFile` dev/ino/size/mtimeMs mismatch→fail, `isCanonicalManifest` extra key/whitespace `JSON.stringify(expected)+"\n"`→false (S5/S6)
- [x] 5.3 `install/verify.go` `VerifyBinary` sha256 vs `integrity.json` `expectedManifest`/`signedReleaseManifest` + `BIGGZ_DEV_BINARY` bypass pin keep checks (S1/S2)
- [x] 5.4 `.goreleaser.yaml` `archives.files` add `integrity.json` keep `README`/`LICENSE`/`minisign.pub`, `checksum sha256` `checksums.txt` `signs` `.minisig` (S7)
- [x] 5.5 Smoke `goreleaser --snapshot --clean` →5 archives `integrity.json` `version/asset/assetSha256/binarySha256` sha256==pin via `VerifyBinary` (S8/S9)

## Phase 6: Testing / Verification + Cleanup

- [x] 6.1 `go test ./internal/opencode ./internal/agents/pi ./internal/install ./internal/tui -count=1 -timeout 180s` + `go vet` + `gofmt -l`
- [x] 6.2 `sha256sum -c dist/checksums.txt` + `minisign -Vm checksums.txt -p minisign.pub -x checksums.txt.minisig` + `biggz --version BuildVersion!=""` + missing `integrity.json` fails (S10/S11)
- [x] 6.3 Final gate `wc -w tasks.md` <530 + update `comparison-with-gentle.md`
