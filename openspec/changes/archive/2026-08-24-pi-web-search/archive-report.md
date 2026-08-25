# Archive Report: pi-web-search

**Archived**: 2026-08-24
**Mode**: Standard (strict_tdd: false)
**Artifact Store**: openspec
**Change**: pi-web-search
**Archived to**: `openspec/changes/archive/2026-08-24-pi-web-search/`

## Summary

Implemented `pi-web-search` — Pi extension `biggz-web-search.js` exposing `web_search`/`web_fetch` with provider fallback `Tavily → Brave → DuckDuckGo`, 3-tier fetch `T1 net/http → T2 tls-client/utls chrome124/safari17 → T3 headless gated`, Markdown extract via `go-readability`/`html-to-markdown`, 10s abort, 1MB truncate, exponential backoff honoring `Retry-After`, SSRF guard, and `sdd-research` gating. Delivered across 3 stacked PRs (auto-chain stacked-to-main), 18/18 tasks complete, 11/11 requirements and 36/36 scenarios PASS, 0 blockers, 0 CRITICAL.

- **PR 1 — Foundation**: `internal/assets/pi/biggz-web-search.js` skeleton, SSRF guard, provider fallback, `internal/install/pi_web_search.go` atomic deploy via `filemerge.WriteFileAtomic`, `go.mod` deps `tls-client`/`utls`+readability
- **PR 2 — Core 3-tier**: T1→T2 TLS on 403, T3 gated `BIGGZ_WEB_FETCH_HEADLESS`, backoff with `Retry-After`, HTML→Markdown extract, caps, loud `FetchBlocked`
- **PR 3 — Wiring**: `internal/install/install.go` `DeployPiWebSearch` in `Run()`, `sdd-overlay-multi.json` gating, `sdd-research/SKILL.md` docs, `internal/doctor/pi_web_search.go` `PiWebSearchCheck` with panic isolation, `cmd/biggz/cli_doctor_help.go` registration

## Spec Compliance

**Verdict**: PASS (per `verify-report.md` evidence_revision sha256:7dc7ec303ab4a40b6f56dfb835e7207baa5e43a3710e4c820e2fc33d4e662456, verified via `biggz sdd-verify-validate` admitted)

- **Requirements**: 11/11 compliant
- **Scenarios**: 36/36 compliant (0 PARTIAL, 0 UNTESTED, 0 FAILING)
- **Build**: `go vet ./internal/install ./internal/doctor` → exit 0
- **Tests**: `go test ./internal/install ./internal/doctor -count=1 -v` → 30 passed / 0 failed (16 install + 14 doctor + 3 assets contracts)
- **Critical findings**: 0
- **Blockers**: 0
- **Coverage**: Not configured (no threshold in `openspec/config.yaml`)

Spec matrix: `pi-web-search` REQ-001..007 (7 reqs, 18 scenarios) + `agent-install` REQ-INST-001/002 (2 reqs, 8 scenarios) + `system-diagnostics` REQ-DIAG-001/002 (2 reqs, 10 scenarios) — all COMPLIANT via `TestDeployPiWebSearch*`, `TestWebSearchJS_CapsAndGuards`, `TestDeployPiSubAgents`, `TestOverlayWebToolsGating`, `TestPiWebSearch_*` suite.

## Spec Sync

Delta specs merged into main specs (source of truth) before archive move:

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| pi-web-search | Created | 7 REQ, 18 scenarios (ADDED) — mechanical copy via `cp` with `diff -r` empty (4.4K, 140 lines) | `openspec/specs/pi-web-search/spec.md` ✅ |
| agent-install | Updated | 2 REQ (REQ-INST-001, REQ-INST-002), 8 scenarios appended — shell merge `sed -n '/^### Requirement:/,$p'` + `cat` with validation (94→151 lines) | `openspec/specs/agent-install/spec.md` ✅ |
| system-diagnostics | Updated | 2 REQ (REQ-DIAG-001, REQ-DIAG-002), 10 scenarios appended — shell merge (135→203 lines) | `openspec/specs/system-diagnostics/spec.md` ✅ |

For `pi-web-search` (new domain, no prior main spec), delta was copied mechanically with shell per Mechanical Copy Contract. For existing domains, requirements were appended preserving all OTHER requirements (Agent Detection, Asset Deployment, File Merge, Plugintest Support; Check Framework, Report Categorization, etc.). No REMOVED or RENAMED requirements.

## Archived Artifacts

All SDD artifacts preserved in `openspec/changes/archive/2026-08-24-pi-web-search/`:

| Artifact | Path | Status |
|----------|------|--------|
| Proposal | `proposal.md` | ✅ 3.7K — intent: open-web web_search/web_fetch, 3-tier, gating |
| Exploration | `exploration.md` / `explore.md` | ✅ 17K — Approach 2 + 3 analysis |
| Specs | `specs/pi-web-search/spec.md` | ✅ 7 reqs, 18 scenarios |
| Specs | `specs/agent-install/spec.md` | ✅ 2 reqs, 8 scenarios |
| Specs | `specs/system-diagnostics/spec.md` | ✅ 2 reqs, 10 scenarios |
| Design | `design.md` | ✅ 7.0K — hosted-API+TLS, SSRF, caps |
| Tasks | `tasks.md` | ✅ 18/18 [x] complete (4+5+6+3) |
| Verify Report | `verify-report.md` | ✅ PASS 11/11 36/36, 0 blockers |
| Archive Report | `archive-report.md` | ✅ (this file) |

Archived `tasks.md` has no unchecked implementation tasks. Active changes directory no longer contains `pi-web-search` (verified via `Get-ChildItem openspec/changes`).

## Task Completion Gate

All 18 tasks marked `[x]` in persisted `tasks.md` (Phase1 4/4, Phase2 5/5, Phase3 6/6, Phase4 3/3). `Select-String "^- \[ \]"` → 0 unchecked, `Select-String "^- \[x\]"` → 18. Gate PASS — no stale checkboxes. `sdd-verify` reports `allComplete: true`, `verify: ready`.

## Mechanical Copy Evidence

Archival is a mechanical filesystem operation per skill. File content never passed through model Read/Write for copy/move — shell only, verified by `diff -r`:

### Spec creation — pi-web-search (new domain)

```text
target_dir="openspec/specs/pi-web-search"
temp_path="$(mktemp "$target_dir/.spec.md.XXXXXX")"  # → openspec/specs/pi-web-search/.spec.md.jH6HxE
cp "openspec/changes/pi-web-search/specs/pi-web-search/spec.md" "$temp_path"
copy_status=0
diff -r "openspec/changes/pi-web-search/specs/pi-web-search/spec.md" "$temp_path"
diff_status=0
# (no output — empty diff is only passing evidence)
mv "$temp_path" "openspec/specs/pi-web-search/spec.md"
# ls -lh → 4.4K, 140 lines, head shows "# Delta for pi-web-search"
```

Verbatim empty `diff -r` confirms byte-identity (no truncation).

### Merges — agent-install & system-diagnostics

```text
# agent-install: extracted 56 lines from delta via sed '/^### Requirement:/,$p'
cat main (94 lines) + extracted (56) → new main 151 lines
grep REQ-INST-001 && grep "Agent Detection" → validation both present
cp tmp_main → temp_verify; diff -r tmp_main temp_verify → empty PASS

# system-diagnostics: extracted 67 lines, 135 → 203 lines, both old+new present
cp tmp_main2 → temp_verify2; diff -r → empty PASS
```

### Archive move — change folder to dated archive

```text
source="openspec/changes/pi-web-search"
destination="openspec/changes/archive/2026-08-24-pi-web-search"
snapshot_root="$(mktemp -d "${TMPDIR:-/tmp}/sdd-archive.XXXXXX")"  # → /tmp/sdd-archive.Te8HvC
cp -R "$source" "$snapshot_root/source"
mkdir -p openspec/changes/archive
git mv "$source" "$destination"  # → fatal: source directory is empty (not tracked), status 128
# fallback: diff -r "$snapshot_root/source" "$source" → 0 (source unchanged)
mv "$source" "$destination"      # → success
[ -e "$source" ] → false (source gone)
diff -r "$snapshot_root/source" "$destination" → 0
# (no output — empty diff, only passing evidence)
ls -R "$destination" confirms 7 files + specs (3 domains)
```

Empty `diff -r` for both copy and move is the mandatory readback; agent self-report never sufficient.

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/pi-web-search/spec.md` — new, 7 requirements (REQ-001..007)
- `openspec/specs/agent-install/spec.md` — updated, now 6 requirements (4 existing + REQ-INST-001/002)
- `openspec/specs/system-diagnostics/spec.md` — updated, now 14 requirements (12 existing + REQ-DIAG-001/002)

Preserved requirements not mentioned in delta remain unchanged.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived:

- Proposal → Exploration → Design → Spec → Tasks → Apply (3 PRs) → Verify (PASS) → Archive (mechanical copy with empty diffs)

Ready for the next change. No open blockers, no CRITICAL issues, no stale tasks.

## Implementation Files (for traceability)

| File | Action | Lines/Notes |
|------|--------|-------------|
| `internal/assets/pi/biggz-web-search.js` | Created | 3-tier, SSRF, backoff, extract |
| `internal/install/pi_web_search.go` | Created | `DeployPiWebSearch` atomic |
| `internal/install/install.go` | Modified | Wire `DeployPiWebSearch` in `Run()` |
| `internal/assets/opencode/sdd-overlay-multi.json` | Modified | web_* in sdd-research only |
| `internal/assets/skills/sdd-research/SKILL.md` | Modified | gating docs |
| `internal/doctor/pi_web_search.go` | Created | `PiWebSearchCheck` |
| `cmd/biggz/cli_doctor_help.go` | Modified | Register check |
| `go.mod/sum` | Modified | tls-client/utls/readability |
