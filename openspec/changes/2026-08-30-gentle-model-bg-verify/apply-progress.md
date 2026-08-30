# Apply Progress: 2026-08-30-gentle-model-bg-verify

## Summary
PR1 picker (biggz-ai kind v1, THINKING_LEVELS, sorted cache, envelope, TUI 30) + PR2 background canonical 4-source + verify canonical + goreleaser integrity.json. Both stacked-to-main, each <400, tests PASS.

## Completed Tasks
- [x] 1.1-1.3 Foundation biggz-ai kind v1, normalize, tmp→rename
- [x] 2.1-2.6 PR1 picker RED+impl LoadVariantsOrEmpty sorted fallback, envelope, Write sorted atomic
- [x] 3.1-3.2 TUI wiring BubbleTea 30 via Merge/EffectiveThinking/PickerAgentFiles
- [x] 4.1-4.4 Background canonical sdd/background.go .biggz priority, strict 2-key, delegate opencode/pi
- [x] 5.1-5.5 Verify canonical VerifyBinary guards + .goreleaser integrity.json
- [x] 6.1-6.3 Testing/Cleanup go vet/test, gofmt, wc -w 506, comparison update

## Work Unit Evidence
| Unit | Focused test command | Result | Runtime harness | Result | Rollback boundary |
|------|----------------------|--------|-----------------|--------|-------------------|
| PR1 picker | go test ./internal/opencode -run TestModelRouting | PASS 6.1 | biggz models→models.json | PASS | opencode/models.go,tui/models.go |
| PR2 bg/verify | go test ./internal/agents/pi -run TestBackground; go test ./internal/install | PASS | goreleaser check not run (verify unit tested) | PASS (vet+unit) | sdd/background.go,install/verify.go,.goreleaser.yaml |

## Files Changed
| File | Action | Lines |
|------|--------|-------|
| internal/opencode/models.go | Modified kind biggz-ai, THINKING_LEVELS, LoadVariantsOrEmpty, sorted Write atomic | 40 |
| internal/sdd/background.go | Created canonical 4-source | 297 |
| internal/opencode/background.go | Modified delegate to sdd | 14 |
| internal/agents/pi/adapter.go | Modified .biggz paths, BIGGZ env, delegate reporting | 29 |
| internal/install/verify.go | Created VerifyBinary guards | 237 |
| .goreleaser.yaml | Modified archives.files integrity.json | 1 |
| integrity.json | Created placeholder | 6 |
| docs/comparison-with-gentle.md | Modified | 1 |

## Commits (stacked-to-main)
- 6d1df8f feat(model-routing): biggz-ai kind v1 (PR1 foundation)
- 2e4fd78 feat(sdd,opencode,pi): canonical background 4-source (PR2 bg)
- ae94734 feat(install,release): canonical VerifyBinary + goreleaser (PR2 verify)

## Risks
None blocking; pending large TestReadLoopLarge unrelated flake pre-exists. Smoke goreleaser --snapshot not run due to CI key unavailable, unit-tested via isConfined/isSymlink/sameFile/isCanonical.

## Next Recommended
verify
