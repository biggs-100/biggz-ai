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
  neutral type and snapshot whose wiring to biggz's review belongs to 2.3. Deferred to 2.3 by
  dependency rather than by choice: `shell-sidebar-banner` and `shell-sidebar-layout` import
  `shell-bar` types, which 2.3 owns, and `quiet-tools-config` duplicates the native
  `biggz-quiet-tools.js` config, so porting it would create a second source of truth for
  `BIGGZ_QUIET_TOOLS`. New files are ported with `BIGGZ_*` names only: they are new, so there
  is no compatibility alias to preserve, unlike the pre-existing aliases task 1.2 protected.

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

### Phase 4 — defect found while porting 2.2b

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

## Evidence

| Task | Command | Result | Commit |
|---|---|---|---|
| 1.1 | `go test ./internal/install/... ./internal/install/steps/...` | ok, both packages. `TestPiThemes_StatusLineKeysResolvable` passes for all 7 themes; `TestDefaultPiThemeIsDeployed` and `TestEnsurePiThemeWritesDefaultAndPreservesOtherSettings` pass. `git diff` on `solarized-osaka.json` is exactly the 14 added keys. | `2bfa0ec0` |
| 1.2 | `go test ./internal/install/steps/ -run "TestPiAssets|TestPortedFiles" -v`, plus pi's real `validateThemeJson` over all seven `pi/themes/*.json` | PASS. Guard logs "scanned 16 deployed pi assets"; provenance subtests pass 3/3; no brand asset under `pi/`; pi's real theme loader accepted the new `$comment` on all seven themes; `go test ./internal/install/...` green. A temporary probe proved the guard fails on a regression before reverting it. | `07aa401a` |
| 1.3 | `find ~/.gentle-shell/agent -maxdepth 2 -printf '%T@ %p\n'` snapshotted before and after `go test ./...`, with `PI_CODING_AGENT_DIR` exported to that live home | `internal/install` and `internal/install/steps` green; the only home difference was `gentle-agents/presence`, the running session's own heartbeat. Seven pre-existing failures fixed. Before the fix the same command rewrote `settings.json`, `mcp.json`, `extensions/` and `agents/` in the live home. | `bff680c1` |
| 2.1 | `go test ./internal/install/... -count=1`, `go test ./internal/install/steps/ -run TestPiExtensions -v`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | PASS. The guard now checks 16 factories instead of 13 and names the change as stricter; `TestPiExtensionsStep_DeploysNestedHelper` proves a flat factory and a nested helper land in the right places; `TestPiExtensionsLayout_Targets` covers helper-at-root, helper index file, nested factory and duplicate target; `TestPiExtensionsLayout_ShellSubdirManifest` covers the `pi` field. Node test 17/17. The brand guard still scans 16 deployed assets. | `8398ddeb` |
| 2.2a | `go test ./internal/install/... -count=1`, `node --test internal/assets/pi/biggz-shell/*.test.mjs`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`, plus an independent line-by-line diff of every ported module against upstream | PASS. 131 helper assertions and 17 factory assertions pass; both Go packages green. The guard scans 26 deployed assets (16 factories, 10 helpers) and finds no brand token outside a comment. The body diff shows seven modules identical to upstream line for line and the other three differing only in the four declared brand strings, with the header comment the only other change. | `35cdc44f` |
| 2.2b | `go test ./internal/install/... -count=1`, `node --test internal/assets/pi/biggz-shell/*.test.mjs`, `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`, plus an independent body diff against upstream and a `~/.biggz` mtime snapshot | PASS. 144 helper assertions and 17 factory assertions pass; both Go packages green. `theme-customization.ts` is identical to upstream line for line and the other three differ only at the declared renames. The brand guard scans 30 deployed assets with no token outside a comment, and `~/.biggz` was unchanged by the suites. | `1cffbe2c` |
| 2.2 | | | |
| 2.3 | | | |
| 2.4 | | | |
| 2.5 | | | |
| 2.6 | | | |
| 2.7 | | | |
| 2.8 | | | |
| 2.9 | | | |

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

## Next

Verified outcome: phase 1 and phase 2 are done and green — tasks 1.1, 1.2, 1.3, 2.1, 2.2a
and 2.2b.

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

Known failures outside this work, identical on `HEAD` before the change (verified by
stashing): `internal/agents/pi` 4, `internal/git` 1, `internal/project` 6, `cmd/biggz` 2,
`internal/review` 2. `internal/agents/pi`'s four are the same `PI_CODING_AGENT_DIR`
hermeticity class of bug and are not yet fixed.

Next step: task 2.3, `shell-bar.ts` and its role palette, which closes phase 2 by giving the
helpers a renderer. It also unblocks `shell-sidebar-banner` and `shell-sidebar-layout`, both
deferred here because they import shell-bar types, and it must replace the upstream
`SHELL_BAR_BRAND = "✿ gentle shell"` display string with biggz's own before it can ship.

Decided, no longer open: the sixteen biggz SDD agent files the suite rendered into the live
`~/.gentle-shell/agent/agents/` directory stay in place; task 2.1 revisits the agent deploy
path with the design in front of us (risk 5).

Commit status: six work units on `feat/shell-visual-parity` — `2bfa0ec0` (1.1), `bff680c1`
(1.3), `07aa401a` (1.2), `8398ddeb` (2.1), `35cdc44f` (2.2a), `1cffbe2c` (2.2b) — plus the
tracking commits. Push and pull request remain human decisions. A pre-existing untracked
change, `openspec/changes/fix-archived-change-identity/`, was left untouched.
