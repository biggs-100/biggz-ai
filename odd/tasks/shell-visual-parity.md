# shell-visual-parity

## Objective

Make what `biggz install` deploys into a Pi agent home look like Gentle Shell: a one-line
shell bar replacing Pi's three-line footer, a startup header, a sidebar rail with
banner/changes/todo/usage, shell cards and gauges, and the `Gentle` / `Gentleman-Cute` /
`Gentleman-Sexy` themes, with `Gentleman-Cute` as the installed default. The
`gentle-shell` / `gentle-pi` names, env names and logos are never used as biggz branding.

## Decisions

Load-bearing choices already made. D1–D5 by the human (2026-09-24); D6–D8 were taken while
implementing unit 2.4 and D9 while implementing unit 2.5, listed here so they are visible for
review.

| # | Decision | Choice |
|---|---|---|
| D1 | Deploy shape for helpers that cannot sit flat in `extensions/` | **Hybrid** — flat where Pi allows it, `extensions/biggz-shell/` subdirectory where it does not |
| D2 | Visual scope | **Full parity** — bar + banner + themes + sidebar rail + custom prompt editor |
| D3 | Owner of the single `setFooter` slot | **Retire `biggz-footer.js`** from the deploy list |
| D4 | Default theme after install | **`Gentleman-Cute`** |
| D5 | Brand strings | Forced by `TRADEMARKS.md`: rename every `gentle-pi` / `gentle-shell` identifier to `biggz-*` / `BIGGZ_*` |
| D6 | Fate of the retired `biggz-footer.js` asset | **Keep the file, retire it from the deploy list** (2.4). The file stays as the rollback reference, the way `biggz-synthesis-gate.js` does, which keeps its suite meaningful and its declared boundary in `.github/guard-baseline.txt` valid; the installer deletes any copy already in a home |
| D7 | Name and shape of the new wiring file | **`internal/assets/pi/biggz-shell.ts`, flat and `factory: true`** (2.4). pi loads direct `extensions/*.ts` files plus subdirectories that carry an `index.ts` entry point, and the 2.1 guards forbid that entry point in `biggz-shell/`, so the factory and its helper directory can coexist |
| D8 | Gates on the shell footer | **`isPrettyEnabled() && shellEnabled()`** (2.4). The first is the repo-wide kill switch every other deployed pretty asset already honours, so the documented `BIGGZ_PRETTY=0` still turns the footer off; the second is the bar's own `BIGGZ_SHELL` |
| D9 | pi's single-owner header slot (`setHeader`) | **Left unclaimed** (2.5). The startup banner is not ported with the header row. `setHeader` is a last-writer-wins slot, so claiming it is a decision that must update `pi_shell_widget_owner_test.go` clause (c) in the same unit, not a side effect of adding a row. The row uses the keyed `ctx.ui.setWidget(..., { placement: "belowEditor" })` slot instead, which coexists with other widgets; the banner itself is task 2.14 |

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

- [x] 1.2 Attribution and brand quarantine.
  Acceptance: a root `THIRD_PARTY_NOTICES.md` carries the verbatim MIT text plus
  `Copyright (c) 2025 Mario Zechner` and lists what was ported; the three ported theme files
  declare provenance in a `$comment`; the deployed surface is guarded by a test that fails
  on a brand image or binary asset under `internal/assets/pi/`, on a `gentle-shell` /
  `gentle-pi` token inside a quoted string on any non-comment line of a deployed asset, and
  on a ported file that lost its provenance.
  Deliberately NOT in scope: the `gentle-*` names this repository documents as compatibility
  aliases — `GENTLE_AI_NO_ANIMATION` (`docs/comparison-with-gentle.md:110`),
  `GENTLE_PI_NO_SKILL_REGISTRY` (`internal/skillregistry/watcher.go:31`),
  `GENTLE_PI_QUIET_TOOLS` (`biggz-quiet-tools.js:17`) — and the provenance comments that
  name the source truthfully. The trademark policy permits nominative references, so a
  blanket ban would delete documented compatibility instead of protecting the brand.
  Evidence needed: `go test ./internal/install/steps/ -run "TestPiAssets|TestPortedFiles" -v`.

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

- [x] 2.1 Hybrid deploy shape plus installer guard.
  Acceptance: `piExtensionsDeployList` supports nested targets so helpers land in
  `<agentDir>/extensions/biggz-shell/`; a new guard asserts that directory never gains
  `index.js`/`index.ts` or a `package.json` with a `pi` field (which would make Pi load it
  as an extension); the existing "expected N JS extensions" guard is updated deliberately,
  not loosened.
  Evidence needed: `go test ./internal/install/steps/`.

### Phase 3 — the shell itself

- [x] 2.2a Port the drawing primitives.
  Acceptance: `shell-gauge`, `shell-hover`, `terminal-theme`, `shell-card`, `shell-changes`,
  `shell-usage`, `shell-todo`, `shell-prompt`, `shell-sidebar` and `review-sidebar-state` land
  in `internal/assets/pi/biggz-shell/` as `factory: false` deploy entries, each carrying a
  provenance header, with co-located `*.test.mjs` ported from the upstream suite and pi-tui
  supplied by the repository's test oracle, which must be extended with whatever these
  modules import from it.
  Evidence needed: `go test ./internal/install/steps/`, and
  `node --test internal/assets/pi/biggz-shell/*.test.mjs` green.

- [x] 2.2b Port the visual-customization modules.
  Acceptance: `agent-home`, `animation-policy`, `visual-customization-policy` and
  `theme-customization` land the same way, with the upstream `GENTLE_PI_*` env names, schema
  ids and `Symbol.for` keys renamed into the `BIGGZ_*` / `biggz-ai.*` namespace and the
  config-home paths mapped to biggz's own, plus their ported tests.
  Evidence needed: the same two commands.

  Decisions taken for 2.2 (2026-09-24): the visual-customization trio is ported in this unit,
  as a second wave so each review stays small. `review-sidebar-state` is ported as-is, as a
  neutral type and snapshot whose wiring to biggz's review belongs to 2.11, because biggz's
  review contract is its own and the ported parser would never match it (risk 10). Deferred to
  2.3 by dependency rather than by choice: `shell-sidebar-banner` and `shell-sidebar-layout`
  import `shell-bar` types, which 2.3 owns, and `quiet-tools-config` duplicates the native
  `biggz-quiet-tools.js` config, so porting it would create a second source of truth for
  `BIGGZ_QUIET_TOOLS`. New files are ported with `BIGGZ_*` names only: they are new, so there
  is no compatibility alias to preserve, unlike the pre-existing aliases task 1.2 protected.

- [x] 2.3 Port `shell-bar` and its role palette.
  Acceptance: golden-line tests at widths 20/60/80/120/200; the `ROLE` map resolves against
  all seven installed themes. Shipped as `shell-bar-themes.test.mjs`, which renders the bar,
  the header, the header rule and the sidebar card through a strict stub theme for every
  shipped theme, so an undefined role fails the suite instead of silently degrading the line.
  `shell-sidebar-banner.ts` came with it, because it needs only shell-bar's type;
  `shell-sidebar-layout.ts` stays in 2.6.

- [x] 2.4 Single `setFooter` owner.
  Acceptance: `biggz-footer.js` leaves `piExtensionsDeployList` and joins the stale
  self-heal list; a test asserts exactly one deployed asset registers `setFooter`; the
  footer path no longer uses `statusLine*` keys that the shell bar does not resolve.
  Shipped as `internal/assets/pi/biggz-shell.ts`, the flat factory that ports
  `buildShellBarModel` and `createShellBarComponent` from upstream's
  `extensions/gentle-shell.ts` and installs the bar from `session_start` behind
  `isPrettyEnabled() && shellEnabled()` (D8). The retired asset stays in the repository as
  the rollback reference (D6), and `DeployPiFooter` — which had no caller and was the last
  live path that could redeploy the footer — is gone, together with the doc comment it had
  left misplaced on `DeployPiToolPills`.
  `pi_footer_owner_test.go` pins all three clauses, including retirement in both
  directions, so a target can never be deployed and retired at once. Its scanner matches
  any non-comment mention of the identifier rather than the literal `.setFooter(` call: a
  bracket form, a destructured or aliased reference, and a registration whose parenthesis
  sits on the next line all take the same slot, and a table test pins those shapes.
  Deferred by design, each named with its owner in the factory header: the sidebar part
  (2.6), the header widget (2.5), the changes tracker (2.7), the prompt editor (2.8) and the
  usage fetch (out of scope for this port). The two `TODO(2.4)` comments in `shell-bar.ts`
  are repointed at 2.5 and 2.8, which own the two items they name.

- [x] 2.5 Header row and the usage command.
  Acceptance: the header row renders through `ctx.ui.setWidget(HEADER_WIDGET_KEY, ...,
  { placement: "belowEditor" })` and stays inert until the theme's `headerPlacement` is
  `below-input` and the TUI is fullscreen, the same condition upstream's row obeys;
  `/biggz-usage` and its global `alt+u` shortcut (`BIGGZ_SHELL_USAGE_KEY` overrides the
  default) open a `UsageView` that refreshes only through a source registered on
  `USAGE_SOURCE_EVENT`, throttled per provider, with no built-in provider fetch; both kill
  switches return at factory scope, so a disabled layer installs no command, no shortcut, no
  widget and no event subscription; and `pi_shell_widget_owner_test.go` pins the widget key's
  single owner, the command name's single owner, pi's header slot as unclaimed and the footer
  slot to the 2.4 scanner.
  Why the banner is not part of this unit: D9.
  Evidence: `node --test internal/assets/pi/biggz-shell.test.mjs
  internal/assets/pi/biggz-shell/*.test.mjs` (235 assertions, 218 helper plus 17 factory),
  `go test ./internal/install/... -count=1`, and `go test ./internal/install/steps/ -run
  TestPiShellWidget -v`.

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
  than by load order. It also binds `PROMPT_FRAME_ROLE`, the editor-frame role
  `shell-bar.ts` names in its `TODO(2.8)`.

### Phase 4 — defects found while porting 2.2b and 2.3

- [ ] 2.9 Make the theme source preview tolerate pi's terminal-default colours.
  Acceptance: `sourcePalettePreview` returns a preview for `dark`, `light` and `titanium`,
  whose `colors.text` is `""`, instead of throwing "Invalid theme palette color"; a test
  exercises the empty-string role against the repository's own shipped themes rather than
  against a fixture that avoids it.
  Why: `""` is how pi encodes the terminal's default foreground, and three of the seven themes
  this repository ships use it. The behaviour is upstream's, but in biggz it is reachable with
  biggz's own theme set, so the customize view would throw as soon as it previews one of them.
  Evidence: calling the ported `sourcePalettePreview("dark", <absolute path to
  internal/assets/pi/themes/dark.json>)` throws "Invalid theme palette color", while `Gentle`,
  `Gentleman-Cute` and `solarized-osaka` return a preview. The ported suite hides this by
  supplying a literal text colour in its fixture.

- [x] 2.10 Brand sweep: widen the literal guard and fix the leaks it missed.
  Acceptance: the guard in `internal/install/steps/pi_assets_brand_test.go` fails when any
  string literal on a non-comment line of a deployed asset contains `gentle`, unless the
  literal also carries one of a small enumerated allowlist of interop and compatibility
  identifiers, each with its reason in the code; a sub-test fails when an allowlist entry no
  longer appears anywhere in the deployed set, so a stale exception cannot hide; and the two
  literals with no interop meaning are renamed — `CHANGES_COMMAND` in `shell-changes.ts`,
  which must equal the `/biggz-changes` hint the bar already prints, and the flag description
  at `skill-registry.ts:554`.
  Why: the guard shipped in 1.2 banned only `gentle-shell` and `gentle-pi`, so it passed a
  `gentle:` command name, a `gentle-ai` event name and a `gentleTodo` key through four ported
  files. A guard narrower than the invariant it claims to protect is worse than no guard,
  because it reads as coverage.
  Evidence: `go test ./internal/install/steps/ -run TestPiAssets -v`.

- [ ] 2.11 Wire the review sidebar to biggz's own review protocol.
  Acceptance: the parser in `review-sidebar-state.ts` matches the schema ids and event name
  biggz actually emits — `biggz-ai.review-integration/v1` and `/v2`, per `internal/review` and
  `contracts/review-integration/` — proven by a test built from a real envelope in
  `contracts/review-integration/v1/fixtures`, instead of upstream's
  `gentle-ai.review-integration.status/v[3-9]` and `gentle-ai:review-sidebar`.
  Why: porting the module faithfully preserved upstream's protocol strings, and biggz speaks a
  different contract, so as ported the review segment can never match a biggz envelope and the
  sidebar would silently never show review state. The 2.2 decision deferred this wiring on
  purpose; this is where it lands, and it is the reason those literals sit in the 2.10
  allowlist instead of being renamed away.
  Evidence: `node --test internal/assets/pi/biggz-shell/review-sidebar-state.test.mjs`.

- [ ] 2.12 Harden the two asset guards that are narrower than their invariants.
  Acceptance: a deploy-manifest parity check fails when the canonical
  `piExtensionsDeployList()` and either mirror disagree on an entry's asset, target or
  factory flag, or when a helper is renamed or dropped; and the `statusLine*` key scanner
  matches every literal form a deployed asset can write, not only the double-quoted one.
  Why: found by the independent verification of 2.4. The Go cross-check compares the
  factory count only and the JS mirror imports no canonical list, so renaming a factory
  entry or dropping a helper is invisible to both — demonstrated by reproducing each
  mutation and watching all three checks stay green. The key scanner misses a
  single-quoted or template-literal key. Same class as risk 11: the guard reads as
  coverage, and the reading is the hazard.

### Phase 5 — the native review of 2.5 and what it deferred

- [ ] 2.13 Close the two reliability findings the native review of 2.5 admitted.
  Acceptance: `refreshUsage`'s registry-only body is exercised by a test that reaches past
  both of its guards — a fake model, a registered source and the recorded `UsageStore` entry
  asserted end to end — and the fire-and-forget call sites (`void refreshUsage(...)` at the
  command, at the event handler and at the widget) attach rejection handling instead of
  discarding the promise, with `renderHost` cleared on `session_shutdown` so a fetch that
  settles afterwards cannot reach a torn-down TUI.
  Why: these are the two findings of the native review of 2.5 (R3-001 and R3-002, WARNING,
  evidence class inferential, disposition `informational`). The review approved the candidate
  and its closure states that none of the findings opened a correction, so they are later
  work by construction and never a reason to re-review that candidate. R3-001 is the sharper
  one: the unit's headline behaviour, the registry-only refresh, is never executed by any
  test, because every test that reaches the path stops at one of the two guards.
  Evidence: the reviewer's ledger, `internal/assets/pi/biggz-shell.ts:224-243` (R3-001) and
  `internal/assets/pi/biggz-shell.ts:347` (R3-002).

- [ ] 2.14 Startup banner via `setHeader` (deferred by D9).
  Acceptance: the banner renders through a fake `pi` render harness; its config probes respect
  the `PI_CODING_AGENT_DIR`-aware agent dir instead of hardcoding `~/.pi/agent`; and
  `pi_shell_widget_owner_test.go` clause (c) is updated deliberately, so the unit that claims
  pi's header slot is the same unit that stops asserting the slot stays unclaimed.
  Why: pi's header slot is a single last-writer-wins slot, so claiming it silently replaces
  whatever pi or another extension put there. 2.4's guard exists precisely so a slot claim
  cannot arrive unnoticed, and the row did not need that slot: the keyed below-editor widget
  coexists, so the banner was separable from the row and was made a decision (D9) instead of a
  side effect.

## Evidence

| Task | Command | Result | Commit |
|---|---|---|---|
| 1.1 | `go test ./internal/install/... ./internal/install/steps/...` | ok, both packages. `TestPiThemes_StatusLineKeysResolvable` passes for all 7 themes; `TestDefaultPiThemeIsDeployed` and `TestEnsurePiThemeWritesDefaultAndPreservesOtherSettings` pass. `git diff` on `solarized-osaka.json` is exactly the 14 added keys. | `2bfa0ec0` |
| 1.2 | `go test ./internal/install/steps/ -run "TestPiAssets|TestPortedFiles" -v`, plus pi's real `validateThemeJson` over all seven `pi/themes/*.json` | PASS. Guard logs "scanned 16 deployed pi assets"; provenance subtests pass 3/3; no brand asset under `pi/`; pi's real theme loader accepted the new `$comment` on all seven themes; `go test ./internal/install/...` green. A temporary probe proved the guard fails on a regression before reverting it. | `07aa401a` |
| 1.3 | `find ~/.gentle-shell/agent -maxdepth 2 -printf '%T@ %p\n'` snapshotted before and after `go test ./...`, with `PI_CODING_AGENT_DIR` exported to that live home | `internal/install` and `internal/install/steps` green; the only home difference was `gentle-agents/presence`, the running session's own heartbeat. Seven pre-existing failures fixed. Before the fix the same command rewrote `settings.json`, `mcp.json`, `extensions/` and `agents/` in the live home. | `bff680c1` |
| 2.1 | `go test ./internal/install/... -count=1`, `go test ./internal/install/steps/ -run TestPiExtensions -v`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | PASS. The guard now checks 16 factories instead of 13 and names the change as stricter; `TestPiExtensionsStep_DeploysNestedHelper` proves a flat factory and a nested helper land in the right places; `TestPiExtensionsLayout_Targets` covers helper-at-root, helper index file, nested factory and duplicate target; `TestPiExtensionsLayout_ShellSubdirManifest` covers the `pi` field. Node test 17/17. The brand guard still scans 16 deployed assets. | `8398ddeb` |
| 2.2a | `go test ./internal/install/... -count=1`, `node --test internal/assets/pi/biggz-shell/*.test.mjs`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`, plus an independent line-by-line diff of every ported module against upstream | PASS. 131 helper assertions and 17 factory assertions pass; both Go packages green. The guard scans 26 deployed assets (16 factories, 10 helpers) and finds no brand token outside a comment. The body diff shows seven modules identical to upstream line for line and the other three differing only in the four declared brand strings, with the header comment the only other change. | `35cdc44f` |
| 2.2b | `go test ./internal/install/... -count=1`, `node --test internal/assets/pi/biggz-shell/*.test.mjs`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`, plus an independent body diff against upstream and a `~/.biggz` mtime snapshot | PASS. 144 helper assertions and 17 factory assertions pass; both Go packages green. `theme-customization.ts` is identical to upstream line for line and the other three differ only at the declared renames. The brand guard scans 30 deployed assets with no token outside a comment, and `~/.biggz` was unchanged by the suites. | `1cffbe2c` |
| 2.3 | `go test ./internal/install/... -count=1`, `node --test internal/assets/pi/biggz-shell/*.test.mjs`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`, plus an independent body diff of both modules against upstream | PASS. 194 helper assertions, up from 144, and 17 factory assertions; both Go packages green; the guard scans 32 deployed assets. `shell-bar.ts` differs from upstream only in the five declared changes plus the two instructed comment rewrites, and `shell-sidebar-banner.ts` only in the declared brand lines. The role-palette suite reports 791 width assertions across the seven themes. | `562ef0d0` |
| 2.4 | `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiFooterOwner\|TestPiExtensions\|TestPiAssets\|TestPiThemes' -v`; `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; `go build ./...`; `go vet ./internal/install/...` | PASS. Still 16 factories and 16 helpers; the brand guard scans 32 deployed assets with its allowlist sub-test green; 223 node assertions pass, including the bar's 791 width assertions; `TestPiFooterOwner_SingleDeployedFooter` fails on the pre-change tree naming `pi/biggz-footer.js` as the owner with 20 `statusLine*` hits, then passes. Independent verification returned PASS WITH FINDINGS — six findings, none blocking: the scanner's registration forms and the retirement seed list were widened inside this unit (with a table test pinning the forms), the deploy-manifest parity gap and the single-quote `statusLine` form are task 2.12, and two findings were wording rather than code. | `da28997d` |
| 2.5 | `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiShellWidget\|TestPiFooterOwner\|TestPiExtensions\|TestPiAssets\|TestPiThemes' -v`; `go build ./...`; `go vet ./internal/install/...` | PASS. 235 node assertions, up from 223: the new `UsageView` suite contributes 8 cases and the factory suite still reports 17. `TestPiShellWidget_Owners` passes with its four clauses, the brand guard scans 33 deployed assets with its allowlist sub-test green, and both `internal/install` packages are green. `go build ./...` and `go vet` clean. An `~/.biggz` and `~/.gentle-shell/agent` mtime sweep across the whole run shows only `gentle-agents/presence`, the running session's own heartbeat, so the new suites are hermetic like 2.2b's. Independent verification of this unit (`gentle-ai-verify`) returned PASS WITH FINDINGS and two of its findings were fixed inside the unit: the kill switches moved from `session_start` to factory scope, because `registerCommand`/`registerShortcut`/`pi.events.on` are factory-time registrations that a later check cannot undo, and the `PI_TUI_SHELL_SYMBOLS` gate was widened to cover `Key`/`matchesKey`, which the new code depends on. Native review of the same tree: lineage `review-70667592872dbc72`, `approved`, two advisory findings now tracked as 2.13. | `7fe76875` |
| 2.6 | | | |
| 2.7 | | | |
| 2.8 | | | |
| 2.9 | | | |
| 2.10 | `go test ./internal/install/steps/ -run TestPiAssets -v` and `node --test internal/assets/pi/biggz-shell/*.test.mjs` | PASS. The guard scans 32 deployed assets; its stale-entry sub-test fails on an inserted stale entry with its delete-this message, verified by inserting one and reverting, and passes again after. Both renames confirmed by diff. An independent scan of the 32 assets found zero `gentle` literals outside the allowlist. | `e165d77a` |
| 2.11 | | | |

## Known risks

1. `shell-sidebar-layout.ts` binds to private pi-tui internals
   (`Symbol.for("@earendil-works/pi-tui/layout-node")`, `host.layoutRoot`, `tui.terminal`).
   A pi-tui bump can make the rail disappear silently; task 2.6 must fail visibly instead.
2. The footer is a single slot, and pi silently lets the last loaded owner win. Unit 2.4
   retired `biggz-footer.js` from the deploy list, and `TestPiFooterOwner_SingleDeployedFooter`
   now fails if a second deployed asset names the slot, if the retired asset regains its entry,
   or if a retired target is deployed again. That guard, not a convention, is why the two can
   no longer coexist.
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
   `.pre-shell-revert` backups left beside both config files. Sixteen files written into
   `agents/` (`explore.md`, `general.md` and fourteen `sdd-*.md`) were not removed, because
   the step renders them instead of copying a shipped asset, so no byte-for-byte match
   exists to justify the delete by the agreed rule.
6. Brand namespace inconsistencies found while scoping 1.2, tracked outside this change
   because they are not part of the shell port: `GENTLE_PI_AUTONOMOUS_MODE` is the only
   name for the autonomous-mode switch, with no `BIGGZ_` primary
   (`internal/policy/guardrails.go:118`, `biggz-synthesis-gate.js:1132`,
   `opencode/plugins/safety.ts:119`); and `internal/assets/pi/skill-registry.ts:26` honors
   only `GENTLE_PI_NO_SKILL_REGISTRY` while its Go sibling honors
   `BIGGZ_NO_SKILL_REGISTRY` first, so setting the biggz name stops the Go watcher and
   leaves the pi extension running.
7. The asset test suites are not wired into CI. `ci.yml` runs `go test ./... -short`, the e2e
   suite, the lens guards and three node scripts, but no `node --test` over
   `internal/assets/pi/`. `docs/validation-guide.md:504` documents the factory test as a
   command to run by hand, and the existing co-located asset suites (221 assertions) have
   the same status. The port's tests are therefore unit evidence, not a CI gate, until
   someone wires them in.
8. `sourcePalettePreview` cannot preview three of this repository's own themes, because
   `dark`, `light` and `titanium` set `colors.text` to `""`, pi's encoding for the terminal's
   default foreground, and the function only accepts a var reference, a 256-colour integer or
   a `#rrggbb`. Ported faithfully from upstream, so the defect is upstream's, but biggz's own
   theme set makes it reachable. Task 2.9 fixes it.
9. The ported `theme-customization` suite cannot exercise the symlink branch on this host:
   `symlinkSync` returns EPERM without `SeCreateSymbolicLinkPrivilege`, so the test falls back
   to the source path directly and the `realpathSync` indirection is only covered elsewhere.
10. The ported review sidebar reads upstream's review protocol, not biggz's. As shipped,
    `review-sidebar-state.ts` matches `gentle-ai.review-integration.status/v[3-9]`, the
    `gentle-ai.review-last-event-closure/v1` closure schema and the `gentle-ai:review-sidebar`
    event, while biggz's own contract is `biggz-ai.review-integration/v1|v2`
    (`internal/review`, `contracts/review-integration/v1` and `v2`). Nothing in the repository
    emits the upstream ids, so the review segment can never match and fails silently. Task 2.11
    rewires it against a real fixture from this repository's own contract.
11. Task 2.10 exists because the literal guard from 1.2 was narrower than its own invariant. It
    banned `gentle-shell` and `gentle-pi`, and therefore passed a `gentle:` command name, a
    `gentle-ai` event name and a `gentleTodo` key through four ported files. When a guard reads
    as coverage but only covers the examples found the day it was written, the reading itself
    is the hazard. The independent verification of 2.4 found two more instances of the same
    class in the asset guards — the `statusLine*` scanner only matches the double-quoted form,
    and the deploy-list mirrors compare counts rather than entries, so a renamed factory or a
    dropped helper is invisible — and task 2.12 closes both.
12. 2.4 changed what the one-line footer says, by design (D3). The retired powerline footer
    painted `branch▕change▕lineage▕lens▕budget`; the shell bar paints location, model, effort,
    context, cost, usage and extension statuses instead, so the review lineage, lens and budget
    facts have no successor in that line and move to the sidebar Status card and the RDD card
    `shell-bar.ts` already renders. The gates changed with it: `BIGGZ_PRETTY=0` still turns the
    footer off because the factory reads `isPrettyEnabled()`, and `BIGGZ_SHELL=0` is the new
    shell-specific switch. Re-examine the lineage/lens/budget gap in 2.6 rather than assuming
    the sidebar covers it.
13. The native review needs a lens model this environment does not provide by default. The
    host relay reads lens routing from `<configHome>/models.json`, where `configHome` is
    `GENTLE_PI_CONFIG_HOME` or `~/.pi/gentle-ai`, but the `gentle-shell` launcher sets
    `PI_CODING_AGENT_DIR` and `GENTLE_PI_AGENT_HOME` to its resolved home and never sets
    `GENTLE_PI_CONFIG_HOME` (`runtime/gentle-shell-launcher.mjs:947`). Under the launcher's
    default isolated home the two paths diverge, no routing file exists there, and a capture
    fails with "no model is configured for review-reliability" before running anything — a
    configuration refusal that reads like a broken candidate. It was fixed by hand here, by
    writing the four `review-*` keys into `~/.pi/gentle-ai/models.json`. biggz-ai sets neither
    variable, so its own deployed home keeps both paths coherent and does not inherit this;
    the defect is the launcher's, and upstream's to fix.
14. A test writes model routing into the user's real global config file, and it does so by
    design of the code under test rather than by a missing override in one fixture.
    `internal/tui/models.go:192` persists through
    `opencode.WriteModelConfig(opencode.DefaultModelConfigPath(), m.userConfig)`, and
    `DefaultModelConfigPath()` (`internal/opencode/models.go:464`) hardcodes `~/.biggz` with
    no env override and no injection seam. `internal/tui/models_test.go:104` then calls
    `SetModel("sdd-design", "test-model")`, so running `go test ./internal/tui/...` overwrites
    the developer's `~/.biggz/models.json` with that fixture — which is exactly what is on
    disk in this environment: `{"sdd-design": {"model": "test-model"}}`. Same class as risk 5
    and the same incident shape as 1.3, but worse, because the file it destroys is
    user-owned configuration rather than a deployed asset. Found on 2026-09-26 while
    configuring the review lenses, not yet reproduced under a controlled snapshot, and
    outside this feature's scope: it needs its own work unit in `internal/tui` and
    `internal/opencode`.

## Next

Verified outcome: the footer is real and the row under the editor follows it — `biggz install`
deploys a shell bar that owns pi's single footer slot, the old powerline footer is retired, and
the same factory paints the header row and registers `/biggz-usage`. Tasks 1.1, 1.2, 1.3, 2.1,
2.2a, 2.2b, 2.3, 2.4, 2.5 and 2.10 are done and green.

1.1 — the three Gentle Shell themes are installed assets, every theme resolves every
`statusLine*` key any deployed asset reads (repairing the pre-existing `solarized-osaka.json`
gap), and `DefaultPiTheme` is `Gentleman-Cute`.

1.2 — `THIRD_PARTY_NOTICES.md` carries the MIT notice and copyright, the three ported themes
declare provenance in a `$comment` that pi's real loader accepts, two literals that were
naming upstream on a shipped surface now name biggz-ai, and the new guard covers the 16
deployed assets plus the logo rule, with non-deployed test fixtures deliberately out of
scope.

1.3 — `internal/install` and `internal/install/steps` are hermetic against an exported
`PI_CODING_AGENT_DIR`, which fixed seven pre-existing failures and stopped the suite from
writing into a live pi agent home.

2.1 — the deploy list records each entry's role instead of inferring it from the file
extension, helpers are sealed in `extensions/biggz-shell/` where the pi loader never looks,
and both validators run before anything is written. Reading the installer also exposed a
defect fixed here: `ask-user-choice.ts`, `codegraph-tools.ts` and `skill-registry.ts` are
pi extension factories deployed flat, and the old suffix rule validated none of them, so a
broken TS factory would have crashed pi with nothing catching it.

2.2a — the ten drawing primitives are ported into `internal/assets/pi/biggz-shell/` with
provenance headers and co-located suites, and their bodies are upstream's to the line except
the four brand strings that reach users or the wire. The pi-tui test oracle gained
`wrapTextWithAnsi`, an ANSI-preserving `truncateToWidth` and a fuller width table, so these
suites verify rendering logic against a faithful approximation rather than against pi
itself; the deployed modules still resolve the real package at runtime.

2.2b — the four visual-customization modules are ported with the config home moved to
biggz's own (`BIGGZ_CONFIG_HOME` or `~/.biggz`) and both persisted schema ids renamed to
`biggz-ai.*`. `theme-customization.ts` is byte-identical to upstream and the other three
differ only at that seam. The suites prove they never touch a real home: `~/.biggz` was
unchanged across the runs, checked by the author and repeated by me.

2.3 — `shell-bar.ts` and `shell-sidebar-banner.ts` are ported with five declared changes each
side of the brand decision, and the new role-palette suite renders the bar, the header, the
header rule and the sidebar card through a strict stub for all seven shipped themes, 791 width
assertions, so an undefined theme role fails loudly instead of degrading the line. That is the
same class of gap 1.1 closed for `statusLine*`.

2.10 — the literal guard now enforces the invariant it claimed: any `gentle` in a string
literal on a non-comment line of a deployed asset, with a closed seven-entry allowlist each
carrying its reason and a sub-test that fails once an entry goes stale. Four leaks the old
two-token rule let through are gone, two by rename and two by being named in the allowlist
because their fix is a wiring change, not a relabel.

2.5 — the factory second surface is real and gated the way 2.4's is. The header row goes into
pi's keyed below-editor widget slot, which coexists with other widgets, and it stays inert
until the theme asks for `below-input` and the TUI is fullscreen, so a home that is not
fullscreen keeps exactly the one-line bar 2.4 installed. `/biggz-usage` and its global `alt+u`
shortcut open the ported `UsageView`, whose refresh is registry-only: the port ships no provider
fetch, so a provider with no source registered on `USAGE_SOURCE_EVENT` says so in the panel
instead of inventing a request. The two kill switches now return before anything registers,
which is the difference between a disabled layer and a half-alive one, and pi's header slot was
deliberately left unclaimed so the banner could not arrive as a side effect (D9, task 2.14).
This is also the first unit in this repository reviewed natively before its commit, which is
what the provider's `current-changes` projection is built for.

Known failures outside this work, identical on `HEAD` before the change (verified by
stashing): `internal/agents/pi` 4, `internal/git` 1, `internal/project` 6, `cmd/biggz` 2,
`internal/review` 2. `internal/agents/pi`'s four are the same `PI_CODING_AGENT_DIR`
hermeticity class of bug and are not yet fixed.

Next step: task 2.6, the sidebar rail, which carries the private-internals risk and owns the
lineage/lens/budget gap risk 12 names. Queued behind it are the two defect fixes 2.9 (theme
preview on terminal-default colours) and 2.11 (review sidebar protocol), then 2.7 (todo rail),
2.8 (prompt ownership), 2.12 (the two guards 2.4's verification found narrower than their
invariants), 2.13 (the two reliability findings the native review of 2.5 admitted) and 2.14
(the banner, deferred by D9).

Decided, no longer open: the sixteen biggz SDD agent files the suite rendered into the live
`~/.gentle-shell/agent/agents/` directory stay in place; task 2.1 revisits the agent deploy
path with the design in front of us (risk 5).

Commit status: ten work units on `feat/shell-visual-parity` — `2bfa0ec0` (1.1), `bff680c1`
(1.3), `07aa401a` (1.2), `8398ddeb` (2.1), `35cdc44f` (2.2a), `1cffbe2c` (2.2b), `562ef0d0`
(2.3), `e165d77a` (2.10), `da28997d` (2.4), `7fe76875` (2.5) — plus the tracking commits. Push
and pull request remain human decisions. A pre-existing untracked change,
`openspec/changes/fix-archived-change-identity/`, was left untouched.

Review size: 2.4 changed 748 lines, 613 of them new files (the factory, its suite and the new
Go guard). That is above the 400-line review budget, and it is the exception this repository
already grants a new self-contained module with its suite. It cannot be split further: pi has
one footer slot, so between two commits the slot would either be empty or doubled, which is
exactly what risk 2 forbids.

2.5 is larger again: nine paths, 1186 changed lines and 1125 insertions. It could not be split
either — the header row, the command and the guard describe one factory, and a commit between
them would leave a registered command pointing at nothing — so instead of splitting it was
reviewed natively before its commit, on the provider's `current-changes` projection: lineage
`review-70667592872dbc72`, one `review-reliability` lens, medium tier, 9 paths, correction
budget 200, `approved` with the two advisory findings that are now task 2.13. The staged tree
was compared against the reviewed candidate tree and matched byte for byte, so what is
committed is what was reviewed.
