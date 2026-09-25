# shell-visual-parity

## Objective

Make what `biggz install` deploys into a Pi agent home look like Gentle Shell: a one-line
shell bar replacing Pi's three-line footer, a startup header, a sidebar rail with
banner/changes/todo/usage, shell cards and gauges, and the `Gentle` / `Gentleman-Cute` /
`Gentleman-Sexy` themes, with `Gentleman-Cute` as the installed default. The
`gentle-shell` / `gentle-pi` names, env names and logos are never used as biggz branding.

## Decisions

Load-bearing choices already made by the human (2026-09-24):

| # | Decision | Choice |
|---|---|---|
| D1 | Deploy shape for helpers that cannot sit flat in `extensions/` | **Hybrid** — flat where Pi allows it, `extensions/biggz-shell/` subdirectory where it does not |
| D2 | Visual scope | **Full parity** — bar + banner + themes + sidebar rail + custom prompt editor |
| D3 | Owner of the single `setFooter` slot | **Retire `biggz-footer.js`** from the deploy list |
| D4 | Default theme after install | **`Gentleman-Cute`** |
| D5 | Brand strings | Forced by `TRADEMARKS.md`: rename every `gentle-pi` / `gentle-shell` identifier to `biggz-*` / `BIGGZ_*` |

## Tasks

### Phase 1 — visible win, low risk

- [x] 1.1 Port the three themes and close the `statusLine*` gap.
  Acceptance: `internal/assets/pi/themes/` holds `Gentle.json`, `Gentleman-Cute.json`,
  `Gentleman-Sexy.json` (names unchanged from source); each theme carries the full
  `statusLine*` key set that the deployed footer reads (`statusLinePath`, `statusLineSep`,
  `statusLineModel`, `statusLineGitClean`, `statusLineGitDirty`, `statusLineContext`,
  `statusLineSpend`, `statusLineCost`, `statusLineBg`, `statusLineStaged`,
  `statusLineDirty`, `statusLineUntracked`, `statusLineOutput`, `statusLineSubagents`);
  a test asserts that every `pi/themes/*.json` resolves every `statusLine*` key referenced
  by any deployed asset, so `theme.fg` can never throw on an unknown key;
  `internal/install/install.go` installs `Gentleman-Cute` by default.
  Evidence needed: `go test ./internal/install/...`, `go test ./internal/install/steps/...`.

- [ ] 1.2 Attribution and brand quarantine.
  Acceptance: a root `NOTICE` (or `THIRD_PARTY_NOTICES.md`) carries the verbatim MIT text
  plus `Copyright (c) 2025 Mario Zechner` for the ported portions; every ported file header
  names the source project, version and license; a test or lint step fails if any
  `gentle-shell` / `gentle-pi` string, glyph or logo appears under `internal/assets/pi/`.
  Evidence needed: `grep -ri "gentle-shell\|gentle-pi" internal/assets/pi/` returns nothing.

### Phase 1.5 — blocker found while verifying 1.1

- [x] 1.3 Make `internal/install` tests hermetic against the ambient `PI_CODING_AGENT_DIR`.
  Acceptance: no test in `internal/install` writes into the developer's real pi agent
  home when `PI_CODING_AGENT_DIR` is exported; `go test ./internal/install/...` passes on a
  machine where that variable is set (for example a gentle-shell install); the tests that
  deliberately exercise the override keep setting it explicitly with `t.Setenv`.
  Why it blocks 1.1: the port's own evidence command is `go test ./internal/install/...`,
  and before this task that command fails for seven pre-existing tests and mutates the
  real pi home while doing it.
  Evidence needed: `PI_CODING_AGENT_DIR=/some/real/home go test ./internal/install/...`
  green, plus a check that the real home's mtime is unchanged.

### Phase 2 — deploy plumbing

- [ ] 2.1 Hybrid deploy shape plus installer guard.
  Acceptance: `piExtensionsDeployList` supports nested targets so helpers land in
  `<agentDir>/extensions/biggz-shell/`; a new guard asserts that directory never gains
  `index.js`/`index.ts` or a `package.json` with a `pi` field (which would make Pi load it
  as an extension); the existing "expected N JS extensions" guard is updated deliberately,
  not loosened.
  Evidence needed: `go test ./internal/install/steps/`.

### Phase 3 — the shell itself

- [ ] 2.2 Port the pure primitives.
  Acceptance: `shell-gauge`, `shell-hover`, `terminal-theme`, `shell-card`, `shell-changes`,
  `shell-usage`, `shell-todo`, `shell-prompt` land with co-located `*.test.mjs` covering
  width maths, card frames and `sanitizeTerminalText` on OSC/CSI input.

- [ ] 2.3 Port `shell-bar` and its role palette.
  Acceptance: golden-line tests at widths 20/60/80/120/200; the `ROLE` map resolves against
  all seven installed themes.

- [ ] 2.4 Single `setFooter` owner.
  Acceptance: `biggz-footer.js` leaves `piExtensionsDeployList` and joins the stale
  self-heal list; a test asserts exactly one deployed asset registers `setFooter`; the
  footer path no longer uses `statusLine*` keys that the shell bar does not resolve.

- [ ] 2.5 Startup banner via `setHeader`.
  Acceptance: the banner renders through a fake `pi` render harness; its config probes
  respect the `PI_CODING_AGENT_DIR`-aware agent dir instead of hardcoding `~/.pi/agent`.

- [ ] 2.6 Sidebar rail.
  Acceptance: the rail activates only when `host.layoutRoot` and the pi-tui layout symbol
  are both present, and falls back to a clean no-rail shell bar otherwise; a test covers
  both branches. This unit carries the private-internals risk.

- [ ] 2.7 Todo rail.
  Acceptance: the todo widget coexists with the `biggz-subagents` widget (distinct
  `setWidget` keys, no row-budget collision); a test asserts both registrations survive.

- [ ] 2.8 Custom prompt editor and prompt ownership.
  Acceptance: the prompt editor owner is established once, and competing editor/working
  registrations (including the `pi-pretty` effect) are gated by the ownership guard rather
  than by load order.

## Evidence

| Task | Command | Result | Commit |
|---|---|---|---|
| 1.1 | `go test ./internal/install/... ./internal/install/steps/...` | ok, both packages. `TestPiThemes_StatusLineKeysResolvable` passes for all 7 themes; `TestDefaultPiThemeIsDeployed` and `TestEnsurePiThemeWritesDefaultAndPreservesOtherSettings` pass. `git diff` on `solarized-osaka.json` is exactly the 14 added keys. | `2bfa0ec0` |
| 1.2 | | | |
| 1.3 | `find ~/.gentle-shell/agent -maxdepth 2 -printf '%T@ %p\n'` snapshotted before and after `go test ./...`, with `PI_CODING_AGENT_DIR` exported to that live home | `internal/install` and `internal/install/steps` green; the only home difference was `gentle-agents/presence`, the running session's own heartbeat. Seven pre-existing failures fixed. Before the fix the same command rewrote `settings.json`, `mcp.json`, `extensions/` and `agents/` in the live home. | `bff680c1` |
| 2.1 | | | |
| 2.2 | | | |
| 2.3 | | | |
| 2.4 | | | |
| 2.5 | | | |
| 2.6 | | | |
| 2.7 | | | |
| 2.8 | | | |

## Known risks

1. `shell-sidebar-layout.ts` binds to private pi-tui internals
   (`Symbol.for("@earendil-works/pi-tui/layout-node")`, `host.layoutRoot`, `tui.terminal`).
   A pi-tui bump can make the rail disappear silently; task 2.6 must fail visibly instead.
2. The footer is a single slot. Until 2.4 lands, the shell bar and `biggz-footer.js` must
   never both be deployed.
3. `extensions/gentle-shell.ts` is 1967 lines but only ~120 are the shell wiring. The port
   is selective; vim engines, command palette and subscription-usage HTTP are out of scope
   unless a task asks for them.
4. `@heyhuynhgiabuu/pi-pretty` is an npm Pi package. biggz already reimplements its effects
   natively (`biggz-quiet-tools.js`, `biggz-thinking-wrap.js`, `biggz-tool-pills.js`), so
   task 2.8 copies the ownership guard and does not add the dependency.
5. Test hermeticity. `AgentConfigPath` gives `PI_CODING_AGENT_DIR` priority over the
   `HomeDir` a caller passes, so any `internal/install` test that forgets the override
   writes into that real directory. Seven tests shipped with that hole; running
   `go test ./internal/install/...` on 2026-09-24 deployed biggz assets into a live
   `~/.gentle-shell/agent` home. Task 1.3 closes it at the package level. The live home was
   reverted the same day: `settings.json` and `mcp.json` restored to their recorded byte
   sizes (351 and 562), `extensions/` (16 files) and `themes/` (7 files) removed, and
   `.pre-shell-revert` backups left beside both config files. Fourteen files written into
   `agents/` were not removed, because the step renders them instead of copying a shipped
   asset and no byte-for-byte match exists to justify the delete.

## Next

Verified outcome: task 1.1 is done and green. The three Gentle Shell themes are installed
assets under `internal/assets/pi/themes/`, every theme now resolves every `statusLine*` key
that any deployed asset reads (which also repaired the pre-existing `solarized-osaka.json`
gap, where selecting that theme threw inside the footer), and `DefaultPiTheme` is
`Gentleman-Cute` so the Gentle look is the installed default.

Task 1.3 is done and green. `internal/install` and `internal/install/steps` are now
hermetic against an exported `PI_CODING_AGENT_DIR`, which fixed seven pre-existing failures
and stopped the suite from writing into a live pi agent home.

Known failures outside this work, identical on `HEAD` before the change (verified by
stashing): `internal/agents/pi` 4, `internal/git` 1, `internal/project` 6, `cmd/biggz` 2,
`internal/review` 2. `internal/agents/pi`'s four are the same `PI_CODING_AGENT_DIR`
hermeticity class of bug and are not yet fixed.

Next step: task 1.2, attribution and brand quarantine, then task 2.1 (the hybrid deploy
shape) which every later unit depends on.

Open items pending human input: whether to also remove the fourteen files this suite wrote
into the live `~/.gentle-shell/agent/agents/` directory (see risk 5).

Commit status: landed as two work units on `feat/shell-visual-parity` — `2bfa0ec0` (1.1,
themes and the `statusLine*` guard) and `bff680c1` (1.3, test hermeticity). Push and pull
request remain human decisions. A pre-existing untracked change,
`openspec/changes/fix-archived-change-identity/`, was left untouched.
