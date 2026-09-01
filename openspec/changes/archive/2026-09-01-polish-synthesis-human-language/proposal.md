# Proposal: Polish Synthesis Human Language

## Intent
Fix disordered mixed-language Sub-agent Result (fix-bigmem-recall-recency). Make synthesis human-readable, language-aware (matches human language), scannable 5-section fixed. Markers/tech identifiers stay English.

## Scope

### In Scope
- `internal/sdd/synthesis.go` RenderSynthesis (4+6 markers, table, sanitize)
- `internal/assets/biggz/biggz-orchestrator.md` + `biggz-orchestrator-workflow.md` (template, language detection)
- `internal/assets/biggz/bigmem-protocol.md` + `docs/architecture.md` (Language Boundary)
- Hybrid hint injection into sdd-* prompts + fallback translation at render

### Out of Scope
- Sub-agent internals beyond `executive_summary` hint
- Translating paths/code/branches/topic_keys
- Changing gate validation (`synthesis_gate.go`/`biggz-synthesis-gate.js` `b0d2fc1` stays verbatim)

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `orchestrator`: language-aware synthesis — localized content, English markers, fixed 5-section
- `pi-integration`: gate docs — localized content does not affect `HasSynthesis`/`isCheckpointAsk` check

## Approach

| Opt | Summary | Pros | Cons |
|-----|---------|------|------|
| A | Orchestrator-only translation at render (detect last human turn, translate summary, keep markers/paths English) | Minimal risk | Post-hoc translation needed |
| B | Prompt injection only (pass human language to sdd-* prompts) | Native generation | Touches 5 prompts, larger surface |

**Recommend A+C hybrid**: detect language, inject hint into sdd-* prompts, fallback-translate at render. Keeps markers `## Sub-agent Result:`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**` English; paths English; low drift.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/synthesis.go` | Modified | Add language param, translate content only |
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | Template: content localized, markers English |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | Add detection before render |
| `internal/assets/biggz/bigmem-protocol.md` | Modified | Language Boundary note |
| `docs/architecture.md` | Modified | Harness vs artifact boundary |
| `internal/sdd/synthesis_gate.go`, `biggz-synthesis-gate.js` | Ref | Unchanged — validate 4 English markers |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Over-translates paths | Med | Whitelist paths — never translate |
| Marker drift breaks gate | Low | Keep 4 markers verbatim; tests cover |
| Wrong detection | Med | Default English; hint helps |

## Rollback Plan
`git revert` synthesis.go + prompts/docs. Markers stay English so gate `b0d2fc1` validates. No migration. Verify `go test ./internal/sdd -run TestSynthesis` PASS.

## Dependencies
- `sdd-init/biggz-ai` exists; `internal/sdd/synthesis_gate.go` gate invariant

## Success Criteria
- [ ] Spanish human → synthesis Spanish; markers English; paths English
- [ ] English human → synthesis English; markers/paths English
- [ ] Fixed 5-section: table+checklist, `◆ Phase · Status · Next`, Artifacts/Paths, Risks, Next (+ optional Preview/Diff/etc)
- [ ] Human decides proceed/adjust/stop without raw logs
- [ ] `go test ./internal/sdd -run TestSynthesis` PASS; `biggz-synthesis-gate.test.mjs` + `go test ./internal/assets/biggz` PASS and block missing synthesis
