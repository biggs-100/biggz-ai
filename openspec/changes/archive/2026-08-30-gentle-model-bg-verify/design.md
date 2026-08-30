# Design: 2026-08-30-gentle-model-bg-verify — Model Picker + Background 4-Sources + Verify Canonical

## Overview

Two stacked PRs (`stacked-to-main`, `<400/PR`) closing 3 gaps. PR1: picker `THINKING_LEVELS`, cache, `models.json`, `biggz-ai.agent_model_routing v1`, BubbleTea 30 table. PR2: `sdd/background.go` (`project>global>env>off`, strict 2-key, `ready/absent`) + `install/verify.go` (`sha256==integrity.json` guards). Covers 9 requirements.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|----------|--------|--------------|-----------|
| **ADR-1 Cache determinism** | Sorted `Record<provider,Record<model,string[]>>`, exact then `modelID` fallback, `LoadVariantsOrEmpty`. Plugin `tmp→rename`+`randomBytes`. | Unsorted/live fetch | Fixes #766/#786 race; deterministic fallback idempotent; miss→empty keeps picker usable. |
| **ADR-2 Background ownership** | Canonical `internal/sdd/background.go`; `opencode/background.go` + `pi/adapter.go` delegate. Paths `.biggz/` honoring `BIGGZ_CONFIG_HOME` > `GENTLE_PI_CONFIG_HOME`; env `BIGGZ_BACKGROUND_SUBAGENTS` > `GENTLE_PI_BACKGROUND_SUBAGENTS`. | Keep `pi/adapter.go` owner | SDD owns policy; `.biggz` fixes drift; delegate prevents recompute divergence; strict 2-key fails closed without fallback. |
| **ADR-3 Verify guards** | `VerifyBinary`=`sha256`+`isConfined`+`isSymlink`+`sameFile`+`isCanonicalManifest`+`signedReleaseManifest` (port `lib/gentle-ai-binary.ts`). `BIGGZ_DEV_BINARY` bypasses pin but keeps checks. | Re-download/single checksum | Pin avoids network/TOCTOU; guards close traversal/symlink/injection. |

## Data Flow

**Picker:** `model-variants.ts tmp→rename → cache → LoadModels → LoadVariantsOrEmpty → EnrichWithVariants (exact→fallback sorted) → TUI (agents>user>builtin, EffectiveThinking) → WriteModelConfig(sorted)/Envelope/Frontmatter`

**Background:** `resolveBackgroundSubagentsPolicy(cwd)` = `project > global > env > off` (2 reads max, extra-key→`off` no fallback) + `ready|absent` via `subagent_run` → `renderReport`.

**Verify:** `VerifyBinary`: `lstat dirs → isConfined → lstat binary → sha256 → expectedManifest → isCanonical → sameFile → OK`. Goreleaser adds `integrity.json` per archive.

## Interfaces / Contracts

```go
// model-routing — internal/opencode/models.go
const MODEL_EXPORT_KIND = "biggz-ai.agent_model_routing"; const MODEL_EXPORT_VERSION = 1
type ThinkingLevel string // off|low|medium|high|inherit
func LoadVariants(path string) (map[string]map[string][]string, error)
func LoadVariantsOrEmpty(path string) map[string]map[string][]string
func EnrichWithVariants(cached map[string]Provider, variantsPath string)
func NormalizeModelConfig(raw map[string]any) AgentModelConfig
func EffectiveThinking(entry, global ThinkingLevel) ThinkingLevel
func ReadModelConfig(path string) (AgentModelConfig,error)
func WriteModelConfig(path string, cfg AgentModelConfig) error
func MergeModelConfigs(layers ...AgentModelConfig) AgentModelConfig
func MarshalModelEnvelope(cfg AgentModelConfig) ([]byte,error)
func ParseModelEnvelope([]byte)(AgentModelConfig,error)
func UpdateFrontmatterRouting(content string, e *AgentRoutingEntry) string
func PickerAgentFiles() []string // 30: orchestrator+SDD+JD+review

// runtime — internal/sdd/background.go (canonical)
type BackgroundSubagentsResolution struct { Policy,Source string; Malformed bool; ProjectFile,GlobalFile string; EnvValue *string }
func ResolveBackgroundSubagentsPolicy(cwd string, opts LoadBackgroundSubagentsOptions) BackgroundSubagentsResolution
func RenderBackgroundSubagentsReport(r Resolution, capability string, wrote *Policy) Report
func ResolveBackgroundSubagentsCapability(homeDir string) string // ready|absent

// release — internal/install/verify.go
func VerifyBinary(binaryPath, versionDir, manifestPath string) (string,error)
func isConfined(path, directory string) bool
func isSymlink(path string) bool
func sameFile(before, after os.FileInfo) bool
func isCanonicalManifest(contents string, manifest, expected map[string]string) bool
func signedReleaseManifest(asset) map[string]string
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/opencode/models.go` | Modify | Rename kind to `biggz-ai.agent_model_routing`; add `LoadVariantsOrEmpty`, sorted fallback, `WriteModelConfig` sorted atomic. |
| `internal/assets/opencode/plugins/model-variants.ts` | Modify | Verify `tmp→rename` + `sort()` + `JSON.stringify(variants,null,2)`; no drift. |
| `internal/tui/models.go` | Modify | BubbleTea table via `MergeModelConfigs`/`EffectiveThinking`/`PickerAgentFiles()` (30). |
| `internal/sdd/background.go` | Create | Canonical 4-source resolver, strict 2-key, `BIGGZ_CONFIG_HOME`, max 2 reads, `ready/absent`. |
| `internal/opencode/background.go` | Modify | Delegate to `sdd.ResolveBackgroundSubagentsPolicy`; keep scheduling-only invariant. |
| `internal/agents/pi/adapter.go` | Modify | `gentleAiConfigHome()` honors `BIGGZ_CONFIG_HOME` first; `.biggz` paths; `BIGGZ_BACKGROUND_SUBAGENTS` priority. |
| `internal/install/verify.go` | Create | Port `lib/gentle-ai-binary.ts` guards + dev-binary override. |
| `.goreleaser.yaml` | Modify | `archives.files` add `integrity.json`; keep `checksum sha256` + `signs`. |

## Threat Matrix

| Row | Applicable | Reason | Failure → RED |
|-----|------------|--------|----------------|
| Routing/shell, VCS/PR | N/A | No shell/PR change | — |
| Executable classification | Yes | `VerifyBinary` gates exec | symlink → `PackageLocalGentleAiBinaryMissingError` |
| Path traversal | Yes | `isConfined` | outside `versionDir` → error |
| TOCTOU | Yes | `sameFile` | dev/ino/mtime mismatch → error |
| Manifest injection | Yes | canonical JSON | extra key/whitespace → `isCanonicalManifest false` |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `THINKING_LEVELS`, normalize, `EffectiveThinking`, envelope, sorted keys | `TestModelRouting`; walk_test unsorted→sorted, `bad model with spaces` dropped |
| Unit | Background 4-source, strict 2-key, malformed→off | `TestBackground`; `project on` overrides `global off`+`env on`; `extra:1`→`malformed` |
| Unit | `VerifyBinary` guards | `TestVerify`; tampered byte → error, symlink → fail |
| Integration | Cache enrich + missing | Fixture `claude-sonnet-4:[low,high]`→sorted; absent→empty |
| Integration | TUI precedence + 30 files | `Write→Read→Merge`; `PickerAgentFiles()==30` |
| Smoke | Goreleaser `integrity.json` | `goreleaser --snapshot` → 5 archives, `sha256sum -c` + `minisign -Vm` + `VerifyBinary` |

## Alternatives

- **Single PR ~600 lines**: rejected — exceeds 400 budget, rollback too large.
- **Keep `pi/adapter.go` owner**: rejected — SDD ownership + `.pi` drift.
- **Re-download on verify**: rejected — pin avoids network, hermetic.

## Migration / Rollback

No migration. `~/.biggz/models.json` v1 shape unchanged; old `gentle-pi.*` envelopes rejected (kind check). `git revert PR2 → PR1`; remove `sdd/background.go` + `install/verify.go`, revert kind, drop `integrity.json` from `archives`. Gate `go vet && go test ./internal/opencode ./internal/agents/pi ./internal/install ./internal/tui`.

## Open Questions

- None — oracles fully ported; `BIGGZ_CONFIG_HOME` order confirmed.
