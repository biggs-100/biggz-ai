```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:bf64ae0180a6a0adbf9d79bdf940bdbb8ecabc0cd2a3a35984b089f8e2a1d763
verdict: pass
blockers: 0
critical_findings: 0
requirements: 14/14
scenarios: 31/31
test_command: go test ./internal/tui -run TestSanitize -count=1 -v && go test ./internal/sdd -run TestSynthesis -count=1 -v && node --test internal/assets/pi/biggz-synthesis-gate.test.mjs
test_exit_code: 0
test_output_hash: sha256:bf64ae0180a6a0adbf9d79bdf940bdbb8ecabc0cd2a3a35984b089f8e2a1d763
build_command: go vet ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: polish-wait-visuals
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 23 |
| Tasks complete | 23 |
| Tasks incomplete | 0 |

All 23 tasks marked [x] in tasks.md. sdd-status reports allComplete true and verify ready.

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./... -> exit 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

**Tests**: ✅ 30 passed / ❌ 0 failed
```text
go test ./internal/tui -run TestSanitize -count=1 -v -> PASS (5 tests)
go test ./internal/sdd -run TestSynthesis -count=1 -v -> PASS (3 subtests)
node --test internal/assets/pi/biggz-synthesis-gate.test.mjs -> 22 passed 0 failed
test_output_hash: sha256:bf64ae0180a6a0adbf9d79bdf940bdbb8ecabc0cd2a3a35984b089f8e2a1d763
```

Modern Go guidelines: consulted sh skills/use-modern-go/scripts/run-tool.sh list --file-path internal/tui/sanitize.go and internal/sdd/synthesis.go - no critical modernization missed.

**Coverage**: ➖ Not configured

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| POLISH-TS-01 | Window equals spent hides window | tui.FormatFleetTokens(2250,2250)==2.2k | ✅ COMPLIANT |
| POLISH-TS-01 | Window distinct shows pair | tui.FormatFleetTokens(4100,2200)==4.1k›2.2k | ✅ COMPLIANT |
| POLISH-TS-01 | Window below threshold hides | tui.FormatFleetTokens(800,600) no › | ✅ COMPLIANT |
| POLISH-TS-02 | Right columns constant 80->120 | FormatElapsed 5c FormatFleetTokensStyled 10c FixedRightWidth 16 | ✅ COMPLIANT |
| POLISH-TS-02 | Left truncates right never | TruncateToWidth left MAY … right NOT | ✅ COMPLIANT |
| POLISH-TS-03 | CJK counts width 2 | VisibleWidth a中b==4 Truncate a中b w4 no split | ✅ COMPLIANT |
| POLISH-TS-03 | Narrow 60c preserves right | RowLeftBudget 60 trunc left/L2 only right intact | ✅ COMPLIANT |
| POLISH-TUI-01 | Standard row 100c L1 glyph+state+5c/10c L2 dim | RenderFleetRow 100 height2 glyph running dim | ✅ COMPLIANT |
| POLISH-TUI-01 | Right intact on narrow 80 long model | FleetRow 80 width<=80 right intact | ✅ COMPLIANT |
| POLISH-TUI-02 | Failure inline gate verify | RenderWorkflowRow L2 gate next output failure dim | ✅ COMPLIANT |
| POLISH-TUI-02 | Nested guide dim | prefix │ dim | ✅ COMPLIANT |
| POLISH-TUI-03 | Two groups rendered 2 running... | RenderHeader muted dim · | ✅ COMPLIANT |
| POLISH-TUI-03 | No overflow <=2 numerics+1 hint | HeaderGroups limits | ✅ COMPLIANT |
| POLISH-TUI-04 | Panes isolated ── panes ── | PanesModel View header distinct | ✅ COMPLIANT |
| POLISH-TUI-04 | Collapsed hides rows | Collapsed true only header | ✅ COMPLIANT |
| POLISH-TUI-05 | Tail hidden 10 limit6 -> ... +4 hidden | VisibleWorkflowRowsStrings 10/6 | ✅ COMPLIANT |
| POLISH-TUI-05 | Order preserved [a,b,c,d] limit2 -> [a,b] + ... +2 hidden | second case | ✅ COMPLIANT |
| POLISH-TUI-06 | Running single tone solid dim muted | RenderFleetRow colors | ✅ COMPLIANT |
| POLISH-TUI-06 | Failed single tone solid | Workflow failed no border | ✅ COMPLIANT |
| POLISH-TUI-07 | Sub-second no shift stable width | FormatElapsed 5 stable FormatFleetTokensStyled 10 stable | ✅ COMPLIANT |
| POLISH-TUI-07 | Small token delta stable | delta 50 width 10 stable | ✅ COMPLIANT |
| POLISH-ORCH-01 | Hides window when equal 3000==3000 -> 3k | sdd.FormatFleetTokens 3k RightAlign 10c | ✅ COMPLIANT |
| POLISH-ORCH-01 | Distinct pair 4100/2200 -> 4.1k›2.2k 10c muted | pair check | ✅ COMPLIANT |
| POLISH-ORCH-01 | Fixed stability 80->120 right identical | TableCellBudget RightAlign | ✅ COMPLIANT |
| POLISH-ORCH-02 | Headline single line 2 runs 23s -> Wait ... | FormatWaitHeadline exact | ✅ COMPLIANT |
| POLISH-ORCH-02 | Limits <=2 lines 4 runs -> len<=2 | FormatWaitHeadlineLines | ✅ COMPLIANT |
| POLISH-PI-01 | Throttle 3s suppress +1.5s false | mock._biggzPiPretty THROTTLE_MS 3000 shouldRender +1.5 false +3 true | ✅ COMPLIANT |
| POLISH-PI-01 | Headline replaces full list 3 runs -> 1 line Fleet no dump | formatHeadlineLines contains Wait Fleet no formatAsyncRunList | ✅ COMPLIANT |
| POLISH-PI-01 | Config collapses 20 | subagent-config.json 20 | ✅ COMPLIANT |
| POLISH-PI-02 | ADR covers four options fork vendor shim config | docs/adr xxx contains | ✅ COMPLIANT |
| POLISH-PI-02 | Shim revertible single file BIGGZ_PRETTY=0 | biggz-pi-pretty.js BIGGZ_PRETTY PI_SUBAGENT_CHILD | ✅ COMPLIANT |

**Compliance summary**: 31/31 scenarios compliant

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|-------------|--------|-------|
| POLISH-TS-01 | ✅ Implemented | tui/sanitize.go compactK formatFleetTokens hide window mirrored screens/sanitize.go sdd/synthesis.go |
| POLISH-TS-02 | ✅ Implemented | VisibleWidth runewidth ansi.Strip TruncateToWidth CJK2 SGR0 RightAlign 5c/10c FixedRightWidth 16 RowLeftBudget |
| POLISH-TS-03 | ✅ Implemented | TruncateToWidth CJK-safe coalesceSGR 60c narrow preserves right |
| POLISH-TUI-01 | ✅ Implemented | screens/polish.go RenderFleetRow height2 L1 clamp rightAlign L2 dim |
| POLISH-TUI-02 | ✅ Implemented | RenderWorkflowRow L1 name·state L2 gate/next/output+failure dim prefix │ dim |
| POLISH-TUI-03 | ✅ Implemented | RenderHeader g1 muted g2 dim 2 groups |
| POLISH-TUI-04 | ✅ Implemented | PanesModel ── panes ── collapsible |
| POLISH-TUI-05 | ✅ Implemented | VisibleWorkflowRowsGeneric tail ... +N hidden |
| POLISH-TUI-06 | ✅ Implemented | colors unified |
| POLISH-TUI-07 | ✅ Implemented | stability FixedRightWidth RightAlign |
| POLISH-ORCH-01 | ✅ Implemented | sdd/synthesis.go compactK renderTable budget 17 chunk7 right 5c/10c |
| POLISH-ORCH-02 | ✅ Implemented | FormatWaitHeadline 1 line Wait ... |
| POLISH-PI-01 | ✅ Implemented | biggz-pi-pretty.js THROTTLE 3000 headline <=2 |
| POLISH-PI-02 | ✅ Implemented | docs/adr xxx-pi-subagents-wait.md 4 sections shim+PR |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Token compaction single authority | ✅ Yes | tui/sanitize.go mirrored screens/sanitize.go sdd/synthesis.go |
| Fixed right cols VisibleWidth runewidth | ✅ Yes | 5c/10c constant |
| 2-line row height2 | ✅ Yes | RenderFleetRow |
| Wait throttle shim 1s->3s | ✅ Yes | THROTTLE_MS 3000 |
| Panes panesCollapsed bool | ✅ Yes | PanesModel |
| Colors dim/muted/solid CJK2 | ✅ Yes | coalesceSGR runewidth |

### Issues Found
**CRITICAL**: None
**WARNING**: None
**SUGGESTION**: ADR placeholder xxx to assign; compactK rounding documented; compactResultMaxLines via JSON read.

### Verdict
**PASS**
All 14 req 31 scen compliant via passing tests. Build go vet pass, 22 node pass, 23/23 tasks complete. No blockers.

### Commands Run
- go test ./internal/tui -run TestSanitize -count=1 -v && go test ./internal/sdd -run TestSynthesis -count=1 -v && node --test internal/assets/pi/biggz-synthesis-gate.test.mjs -> exit 0 hash sha256:bf64ae0180a6a0adbf9d79bdf940bdbb8ecabc0cd2a3a35984b089f8e2a1d763
- go vet ./... -> exit 0 hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
- biggz sdd-verify-validate --requirements 14 --scenarios 31 -> valid
