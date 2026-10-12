# shell-visual-parity

## Objective

Make what `biggz install` deploys into a Pi agent home look like Gentle Shell: a one-line
shell bar replacing Pi's three-line footer, a startup header, a sidebar rail with
banner/changes/todo/usage, shell cards and gauges, and the `Gentle` / `Gentleman-Cute` /
`Gentleman-Sexy` themes, with `Gentleman-Cute` as the installed default. The
`gentle-shell` / `gentle-pi` names, env names and logos are never used as biggz branding.

## Decisions

Load-bearing choices already made. D1–D5 by the human (2026-09-24); D6–D8 were taken while
implementing unit 2.4, D9 while implementing unit 2.5, D10–D12 while implementing unit 2.6, and
D13–D14 while implementing unit 2.7 — the ones answered by the human from the mapping of each
unit are marked as such, and all are listed here so they are visible for review.

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
| D10 | Review facts in the sidebar rail | **None** (2.6). The rail carries no lineage, lens or budget. Upstream's shell never shows them either — its Status card's RDD group is unreachable, because the event that would carry it (`gentle-ai:review-sidebar`) has no emitter and no consumer in 3.7.0 — and the retired powerline footer read speculative `ctx.*`/`footerData.*` fields that no producer ever supplied, so its `lineage▕lens▕budget` segments never rendered real data. The real facts live in biggz's review CLI and store, and giving the shell access is a producer problem: risk 12's successor is **task 2.11** |
| D11 | Who paints the header when the rail is active | **The rail, for the `top` placement** (2.6). `headerPlacement` has exactly two disjoint values, so the rail's `"header"` part owns `top` and the below-editor widget keeps owning `below-input`, and no value can make both paint. Faithful to upstream, which registers the part unconditionally and gates the widget on `below-input` |
| D12 | Behaviour when the pi-tui internals are missing | **Silent, clean no-rail bar** (2.6), superseding risk 1's "task 2.6 must fail visibly instead". Upstream's own behaviour: every absent internal is a silent `state.active = false`. With no runtime signal, the loud detector has to be the suite, so the resolver gained a resolution report and the suite now fails — naming the directory and the missing symbols — when a discovered install stopped exposing a required symbol. The one case no in-module guard can cover is R3-001 (task 2.15): a static import that stops resolving fails at link time, before any guard runs |
| D13 | Whether biggz ships a todo producer | **Yes, ported whole** (2.7, by the human). Upstream's producer is the extension itself: a `todo` tool whose result carries the state in its `details`, replayed from the session branch on `session_start`. Nothing else in biggz produces a todo list, so without the producer the rail's `"todo"` section can never render and the already-deployed `shell-todo.ts` stays orphaned. This adds a model-facing tool to the deployed harness, which is why it was a decision and not an implementation detail |
| D14 | The todo port's naming and placement defaults | **`aboveEditor` (upstream's own default, by the human), `TODO_DETAILS_KEY` renamed to `biggzTodo` with the brand allowlist entry deleted, `BIGGZ_TODO` / `BIGGZ_TODO_KEY`, the child guard renamed to the established `PI_SUBAGENT_CHILD`, and the `"agents"` rail section left empty** (2.7). The placement is upstream's, so the todo card never competes with `"biggz-subagents"`; the key rename deletes an allowlist entry whose stated reason was exactly this unit, rather than keeping a brand literal with a resolved reason — the hazard class 2.10 and 2.12 exist for. Porting `GENTLE_PI_AGENTS_CHILD` literally would have been a functional bug, not a brand leak, because biggz's children set `PI_SUBAGENT_CHILD` |
| D15 | Whether `setDisabled` must gate interactive children too | **No: the child-first boundary IS the port's contract** (2.7c, by the human, on a recommendation). `NativePointerScope.setDisabled(true)` gates the region's own callbacks; a child that consumes natively is dispatched first by pi-tui's `MouseRegion` and never gated, because the check lives in `NativePointerRegion.handlePointer` and that callback never runs — upstream's own order, which the class comment already stated. Widening it would end the byte-fidelity that 2.7a's acceptance verifies (the diff against upstream must stay the two provenance lines) and that the port's whole evidence rests on. Nothing reachable calls the combination: the only consumer is `biggz-todo.ts`, which wraps a passive card with no `handleMouse` and never calls `setDisabled`, and the shell rail implements its own `handleMouse` path rather than using this helper. The 2.7c cases now hold the real contract, so the boundary fails loudly if it ever changes |
| D16 | Whether the theme preview may diverge from upstream for the empty colour role | **Yes, by the task's own acceptance** (2.9). Upstream's preview can only render an indexed colour or `#rrggbb` and throws otherwise, but `""` is how pi encodes the terminal's own default and three of the seven themes this repository ships use it for `text`, so the port was unreachable with biggz's own theme set. The empty value now renders the reset-to-default escape (`39` foreground, `49` background) and the deviation is declared in the module header rather than left implicit. The port's fidelity claim is therefore "verbatim apart from that one declared deviation" from here on, and an audit of it has to read the header |
| D17 | Which review vocabulary the sidebar shows, and whether it supersedes D10 | **The provider's lifecycle protocol, mirroring gentle-shell; D10's deferral ends here** (2.11, by the owner: "así como gentle-shell"). The measurement behind it: `biggz-ai.review-integration/v1` is `{schema, lineage, next_transition}` with no `authority.state`, `action`, `outcome`, `closure` or `projection.paths`, so the display states cannot be derived from biggz's own contract, while the envelopes the review tools return carry exactly those fields. What D10 protected still holds and is still asserted: no `lineage`, `lens` or `budget` reaches the rail, because those are authority facts and the rail paints display state only. D10's remaining sentence - "giving the shell access is a producer problem: risk 12's successor is task 2.11" - is precisely what this unit delivered |

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

- [x] 2.6 Sidebar rail.
  Acceptance: the rail activates only when `host.layoutRoot` and the pi-tui layout symbol are
  both present, and falls back to a clean no-rail shell bar otherwise, with a test covering both
  branches — asserted against `root[NODE]()`, the node shape, `sidebarState.active`, the bottom
  component's `render()` and the untouched root, so a port that never replaced the node fails.
  It also carries the Status card as the `"footer"` rail part, hands `invalidateSidebar` through
  the render host hook, disposes the widget, the header part and the rail together, and the new
  helper is a `factory: false` deploy entry.
  Why the unit is shaped this way: D10 (no review facts, risk 12 reassigned to 2.11), D11 (the
  rail owns the `top` header case), D12 (silent no-rail degradation, with the suite as the loud
  detector).
  Evidence: `node --test internal/assets/pi/biggz-shell.test.mjs
  internal/assets/pi/biggz-shell/*.test.mjs` (256 assertions, 218 of them the pre-existing
  helper suites plus the rail's 38), `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`
  (17), `go test ./internal/install/... -count=1`, `go test ./internal/install/steps/ -run
  'TestPiShellWidget|TestPiFooterOwner|TestPiExtensions|TestPiAssets|TestPiThemes' -v`,
  `go build ./...` and `go vet ./internal/install/...`.

- [x] 2.7a Port the native pointer region helper.
  Acceptance: `internal/assets/pi/biggz-shell/native-pointer-region.ts` is a faithful port of
  upstream `lib/native-pointer-region.ts` — the diff is the two-line provenance header and nothing
  else — with its upstream suite ported case for case; `MouseRegion` is covered by the test
  plumbing (the resolver's `PI_TUI_SHELL_SYMBOLS` and a faithful oracle implementation); and the
  module enters the deploy list as a `factory: false` helper sealed in the `biggz-shell`
  subdirectory, leaving the factory count at 16.
  Why it is its own slice: upstream's todo extension imports this helper as a hard dependency, so
  the producer cannot be written before it exists; splitting on that dependency keeps each slice
  reviewable.
  Evidence: `node --test internal/assets/pi/biggz-shell/native-pointer-region.test.mjs` (6 cases,
  the same six upstream asserts, against the real pi-tui install), the rail suite at 267
  assertions, `go test ./internal/install/... -count=1`, brand guard at 35 deployed assets,
  `go build ./...` and `go vet` clean. One repair shipped with it: adding `MouseRegion` broke the
  R3-002 subprocess proof in the rail suite, whose fixtures hardcoded the symbol list, so both
  lists now derive from `PI_TUI_SHELL_SYMBOLS` and the proof additionally asserts that the
  doctored fixture omits exactly the layout shapes — the assertion that fails if the list is
  widened again without the fixtures being revisited.

- [x] 2.7b Port the todo producer (the `todo` tool and the rail part). *(verified: PASS; `fdf89b6e`)*
  Acceptance: the ported extension registers the `todo` tool the model calls and replays the last
  successful result on `session_start`, renders the card through the `"todo"` rail part with the
  faithful `aboveEditor` placement for the widget, and coexists with `"biggz-subagents"` (distinct
  `setWidget` keys — pi keeps one keyed map per placement and deletes only the same key, so two
  factories do coexist); a test asserts both registrations survive and that the rail section
  renders; the brand rule holds with the `gentleTodo` allowlist entry DELETED rather than kept,
  since its stated reason was this unit.
  Why the producer is ported at all: nothing else in biggz produces a todo list, so without it the
  rail's `"todo"` section can never render and `shell-todo.ts` stays orphaned (D13).
  Evidence observed: **PASS, with one acceptance clause NOT covered — recorded as 2.7e item 1**.
  `internal/assets/pi/biggz-todo.ts` (291 lines) is deployed as a factory and registers the `todo`
  tool (`renderShell: "self"`), the card through `sidebarPart(tui, "todo", …)` and the widget under
  the distinct key `biggz-todo`; its suite (332 lines, 10 cases) covers the write/show path, the
  snapshot in `details`, the replay from the branch on `session_start` (rpiv-todo results
  included), the drop of an already-finished list, the staleness signal, the collapse shortcut, the
  hover/click behaviour and the R3-001 containment. The brand rule holds with the `gentleTodo`
  allowlist entry DELETED and the key renamed to `biggzTodo`, the factory count moved 16 → 17 in
  both guards, and `TestPiExtensionsDeployList_ShellBeforeTodo` pins the load order the benign
  visibility race depends on. Numbers: 10/10 the new suite, 267 shell assertions, 9 in `test/`, 18
  factory, `go test ./internal/install/...` green, `go build ./...` and `go vet` clean.
  Native review of the same tree: lineage `review-e12d5b1d91db6315`, 8 paths, 683 changed lines,
  risk `medium`, correction budget 200, the provider consolidating the two selected lenses into
  `review-reliability`, `approved` — three advisory findings, all `informational`, none opening a
  correction, now tracked as 2.7e.

- [x] 2.7c Close the three coverage findings the native review of 2.7a admitted. *(verified: PASS; `901dc435`)*
  Acceptance: the scope-level hover coordination is exercised across TWO regions (leave one,
  enter the other, assert the sibling's hover was cleared) instead of only single-region scopes;
  the disabled guard is proved against an interactive child that WOULD consume the event, not only
  against a passive `Text`; and `createMouseObserver`'s render request on a hover clear is
  asserted.
  Why: all three are `informational` advisory findings of the review of 2.7a
  (`native-pointer-region.ts:145-150`, `:51-54` and `:117`). They are gaps in upstream's own
  coverage rather than defects introduced here, which is why the port stayed faithful — but 2.7b
  wires this helper to a clickable todo card, so they stop being academic the moment it lands.
  Evidence observed: **PASS, with one boundary the coverage exposed**. Three cases were appended to
  `native-pointer-region.test.mjs` (+152 lines) under a "biggz coverage, task 2.7c" banner: the suite
  goes from 6 to 9 and the shell suites from 267 to 270 assertions. Each case was proved
  discriminating by mutating the module and confirming that only its own case fails — removing the
  sibling clear, and handing the sibling a copy of the event instead of the event itself, both fail
  the two-region case; dropping the `disabled` gate fails the new disabled case and the pre-existing
  one; making the observer request a render unconditionally, and never requesting it, both fail the
  observer case. The module was restored byte-identically after every mutation
  (`sha256:882843f2ae8e81f32879d2cc13b4f6e0521be7adb1d95aa156543ef9baa000f2`, equal to the HEAD blob),
  so the port's fidelity claim stands.
  What the coverage found: the `disabled` guard reaches the region's own callbacks and NOT an
  interactive child. A passive `Text` child stays silent while the scope is disabled, but a child
  that consumes natively (`Box` + `handleMouse`) is dispatched first by pi-tui's `MouseRegion` and is
  never gated, because the region's `handlePointer` — where the check lives — is not reached. The
  test now pins that real contract instead of the guarantee the word `disabled` suggests, and the
  class comment already documented the order ("Child dispatch stays first, so an already handled
  child result is unchanged"). Independent verification (`gentle-ai-verify`) returned PASS for all
  three items with six mutations of its own and no defect, and judged the boundary a fidelity
  requirement of the port rather than a defect: the behaviour lives in upstream pi-tui's dispatch and
  the module is byte-identical to HEAD. Production exposure is nil today — the only consumer is
  `biggz-todo.ts`, which wraps a passive card with no `handleMouse` and never calls `setDisabled`,
  and the shell rail implements its own `handleMouse` path instead of using this helper.
  Decision (recorded as **D15**, by the human, on the recommendation below): accept that boundary as
  the port's contract. The question was whether to diverge from upstream so `setDisabled` gates
  interactive children too; it stays as upstream has it, because nothing reachable calls that
  combination and a divergence would end the byte-fidelity this port is verified by. The test pins
  the real contract, so the boundary is now held by an assertion that fails if it changes, not by
  documentation.
  Advisory findings of the verification, none of them defects: the fixture-shape guard on the region
  count can only fail if the fixture changes; the first crossing's "no leave" assertion is vacuous
  for the second region because nothing is hovered yet; the class comment documents the child-first
  boundary while `setDisabled` itself does not (this one cannot be closed without editing the ported
  module, which 2.7a's acceptance forbids beyond the two provenance lines, so it stays recorded here).
  Environment note: this suite resolves pi-tui 1.1.0, while 2.7a's evidence cites 0.85.1, and the
  interactive expectations derive from pi-tui's `MouseRegion` dispatch — so a future bump is what
  would move them.

- [ ] 2.7d (not planned) The `"agents"` rail section stays empty on purpose.
  Recorded so nobody re-opens it as a defect: upstream 3.7.0 removed the agents rail part
  explicitly (`extensions/gentle-agents.ts:907-911`), `visibility.agents` is parsed but never
  filtered by the rail, and the biggz subagents surface is a plain `setWidget`. Producing it would
  be new work, not parity.

- [x] 2.7e Close the coverage 2.7b's acceptance asked for and the three findings its review admitted.
  *(verified: PASS; `c5b521c7`)*
  Acceptance: a test loads both the todo and the subagents factories into one host and asserts both
  widget keys survive with the `"todo"` rail part registered; and each of the three advisory
  findings of `review-e12d5b1d91db6315` is either closed by a case that fails before the fix or
  re-worded as an explicit non-goal.
  Why item 1 is real: 2.7b's acceptance asks for "a test asserts both registrations survive and that
  the rail section renders", and no test does that today. Each extension's suite asserts only its
  own key (`biggz-todo` in the new suite, `biggz-subagents` at `biggz-subagent-runtime.test.mjs:1456`)
  and `TestPiExtensionsDeployList_ShellBeforeTodo` pins load order, not coexistence, so nothing fails
  if the two keys ever collide or if the rail part stops being registered.
  The findings, verbatim from the closure: R3-001 WARNING at `biggz-todo.ts:127` (the
  `if (sidebarLayout) sidebarLayout.invalidateSidebar(tui)` guard on the lazily loaded rail module),
  R3-002 WARNING at `:263-265` (the `session_shutdown` teardown that deletes the session entry) and
  R3-003 SUGGESTION at `:172-176` (`onLeave` clearing hover and requesting a render outside the
  region protocol's return value). The provider published ids, severity, disposition and locations
  only: the descriptions here are this document's reading of the cited lines, and the reviewer's own
  text stays in the native receipt. Because none of them opened a correction they are later work by
  construction and never a reason to re-review the committed candidate.
  Evidence observed: **PASS, with one item closed as an explicit non-goal**. `WIDGET_KEY` is now
  exported from `biggz-todo.ts` so the coexistence case asserts against the real value instead of a
  copied literal. That case mounts both factories on one host, drives the subagents runtime's own
  publisher into that host, and asserts both widget keys live in one map with distinct placements
  (`biggz-subagents` at `belowEditor`; the todo takes pi's default), that the `"todo"` rail part is
  registered on the same terminal, and that the `todo` tool survives mounting the other factory.
  R3-002 was closed by a case that failed before the fix: `session_shutdown` dropped the session
  state but left the widget painted for the next session, and the shell extension already clears its
  own on shutdown. R3-003 by a case that hovers, clears and asserts exactly one render, and none for
  a second clear with nothing hovered. R3-001 is **fixed but not tested**: `invalidateSidebar` is a
  no-op until the lazily imported rail module resolves, so a `show()` before that point never asked
  the rail to repaint its new part; loading the module now invalidates every live session once.
  Forcing that ordering deterministically needs an injected loader, and the module keeps its cache in
  a private symbol, so a test would pin a scheduling detail or reach into another module's private
  state rather than the contract - recorded here as the non-goal the acceptance allows, not dressed
  up as verified. Four mutations confirm the new cases discriminate: giving the todo the runtime's
  key, dropping the runtime's placement, registering the rail part under another key, and removing
  the render request from `onLeave` each fail exactly the case that covers them, with both modules
  restored byte-identically (`a7a65c9d...`, `6038ea19...`). Numbers: 13/13 in the todo suite (10
  upstream + 3 new), 270 shell assertions, 9 in `test/`, 18 factory, `go test
  ./internal/install/...` green, `go build ./...` and `go vet` clean.

- [ ] 2.8 Custom prompt editor and prompt ownership.
  Acceptance: the prompt editor owner is established once, and competing editor/working
  registrations (including the `pi-pretty` effect) are gated by the ownership guard rather
  than by load order. It also binds `PROMPT_FRAME_ROLE`, the editor-frame role
  `shell-bar.ts` names in its `TODO(2.8)`.

### Phase 4 — defects found while porting 2.2b and 2.3

- [x] 2.9 Make the theme source preview tolerate pi's terminal-default colours. *(verified: PASS; `7dabf94d`)*
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
  Evidence observed: **PASS**. Reproduced first: `sourcePalettePreview("dark", .../themes/dark.json)`
  threw `Invalid theme palette color.` while `Gentle`, `Gentleman-Sexy` and `solarized-osaka`
  previewed. The cause is the last line of `escape()`: anything that is neither an indexed colour
  nor `#rrggbb` throws, and `""` is neither. The empty value now renders the escape that returns to
  the terminal's default (`39` foreground, `49` background).
  The case previews **every shipped theme as it ships** - the seven files, no doctored palette and no
  injected colour - and asserts which of them carry the empty role (`dark`, `light`, `titanium`), so
  the coverage cannot be lost quietly. The ported suite's own header admits it supplies a text colour
  because the fixture leaves the role empty, which is how the gap stayed hidden inside the fixture
  that existed to cover it. Mutation-checked: rendering `0m` instead of `39m` fails exactly this case
  and nothing else, and the module was restored byte-identically. This is a **deliberate deviation
  from upstream**, declared in the port's header, which until now described the file as verbatim
  (D16). Numbers: theme-customization 4/4, all 32 asset suites 518 tests with 0 failures,
  `go test ./internal/install/...` green, `go build ./...` and `go vet` clean. Native review of the
  same tree: lineage `review-8c94163dee5a14e0`, 2 paths, 48 changed lines, risk `medium`, correction
  budget 24, lens `review-reliability`, `approved` with no findings.

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

- [x] 2.11 Wire the review sidebar to the review protocol it speaks. *(verified: PASS; `c5ad5731`)*
  Note on the title and the acceptance below: both named the wrong protocol. The unit's evidence records
  what was measured instead, and D17 records the decision.
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
  Evidence observed: **PASS, and the premise above was wrong.** Measured, not assumed (D17): biggz's own
  `biggz-ai.review-integration/v1` is `{schema, lineage, next_transition}` and carries none of
  `authority.state`, `action`, `outcome`, `closure` or `projection.paths`, so the twelve display states
  cannot be derived from it, while the envelopes the review tools actually return do carry them
  (`gentle-ai.review-integration.status/v9`, `gentle-ai.review-last-event-closure/v1`) - two of them are
  pinned verbatim in the suite from this session's own 2.9 review. What was really missing was the other
  half of the module: upstream pairs `reviewSidebarSnapshot` with `createReviewSidebarPublisher`, which the
  port never brought over, so the parser had no caller and `buildShellBarModel` never set `review`. The
  rail's review group could not render at all, whatever the strings said.
  Delivered: the publisher ported with upstream's resolution logic unchanged, adapted to observation
  (`tool_call` carries the input, `tool_result` the input and the details, because this port does not own
  the review tool definitions and cannot wrap them), wired into `biggz-shell.ts` at factory scope and
  threaded into the bar model. The adaptation and its one fidelity limit are declared in the module
  header: without the thrown error a cancellation cannot be told from a failure, so both report
  `unavailable` where upstream reports `unknown`, and that is the single upstream case this suite does not
  port.
  Two findings worth keeping. The real v1 closure nests its target identity under
  `acknowledgement.binding.target_identity` while upstream's own fixture puts it at the top, so a capture
  that closes loses the file scope (the state still resolves correctly); the suite pins the observed
  behaviour and says why. And the D10 guard had to be updated rather than deleted: it still fails if
  `lineage`, `lens` or `budget` reaches the rail, and it now asserts the display state arrives through the
  publisher's event; its allowlist gained the two review tool names, and the three review-protocol entries
  lost the reason they carried for this unit, which was that same wrong premise.
  Numbers: 528 asset tests with 0 failures (290 in the shell, timing and test-directory suites), `go test
  ./internal/install/...` green, `go build ./...` and `go vet` clean.
  **Native review: NOT run.** The session closed at the owner's request, so `c5ad5731` stands un-reviewed
  and the next session decides whether to review it or fold it into the next candidate.

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

### Phase 5 — native review findings, and what they deferred

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

- [x] 2.15a Close the two WARNING findings of the native review of 2.6.
  Acceptance: (a) a pi-tui release that drops `ScrollView` or `VStack` no longer takes the whole
  shell extension down — the rail is loaded through a cached lazy import whose failure maps to a
  handle or `undefined`, the former `invalidateSidebar` call sites degrade to no-ops, a module
  that resolves after the footer callback still installs the rail exactly once, a throw on that
  late path is contained, and disposal across the async gap is correct; a subprocess fixture with
  a stub pi-tui lacking both exports proves the footer, the widget, `/biggz-usage` and `alt+u`
  still install with the rail absent and the layout root untouched; (b) the D12 detector fails
  only when nothing resolved, so a stale pi-tui beside a usable one, or an explicit `PI_TUI_DIR`,
  cannot produce a spurious hard failure with a message that misstates the fallback.
  Evidence: `node --test internal/assets/pi/biggz-shell.test.mjs
  internal/assets/pi/biggz-shell/*.test.mjs` (258 assertions), `node --test internal/assets/pi/test/*.test.mjs`
  (5, covering the containment fixture and the three timing cases), `go test ./internal/install/... -count=1`.
  Both directions of the D12 rule and both timing guards were mutation-checked: dropping the
  `disposed` check fails the dispose-before-resolve case, removing the `try/catch` fails the
  throwing-late-install case, and the doctored candidate still fails the detector while a stale
  sibling beside a usable install passes.
  Deliberate boundary: the shell's own public pi-tui symbols (`truncateToWidth`, `visibleWidth`,
  `wrapTextWithAnsi`, `Key`, `matchesKey`) remain hard static dependencies, so losing one of those
  still takes the shell down. That is the shell's rendering API, not the rail's private internals.

- [x] 2.15b Close the four remaining coverage and behaviour findings, plus the one the review of
  this unit found.
  Acceptance: (a) the synchronous rail install branch in `biggz-shell.ts` is exercised — it is the
  steady-state path for every session after the first in the same process, and no test reached it
  because the timing suite always gated the module and the containment fixture made it unloadable;
  (b) a rail installed after the footer callback requests a render, so the first session's rail
  cannot stay invisible until an unrelated render; (c) `reclaimFooterRow`'s re-host path has a test
  whose fixture builds a dock whose layout node holds a nested entry, so the early return is not
  taken; (d) the rail wiring is asserted rather than grepped: the footer digest tracking the live
  model, the header usage-span click handler, and the release chain on a throwing component.
  Why: (a) and (b) are the two findings of the native review of 2.15a (R3-001 WARNING and
  `deterministic` — the unproved branch is the common one — and R3-002 SUGGESTION about the missing
  render); (c) and (d) are R3-003 and R3-004 from the review of 2.6. All four were advisory and
  `informational`, so they were later work by construction and never a reason to re-review an
  approved candidate.
  Evidence: `node --test internal/assets/pi/biggz-shell.test.mjs
  internal/assets/pi/biggz-shell/*.test.mjs` (261 assertions), `node --test internal/assets/pi/test/*.test.mjs`
  (9, covering the containment fixture, the six timing cases and the earlier controls),
  `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` (17), `go test
  ./internal/install/... -count=1`, `go build ./...`, `go vet ./internal/install/...`; brand guard
  scans 34 deployed assets; `~/.biggz` unchanged. Non-vacuity was mutation-checked twice: dropping
  `quiet` from the rail uninstall fails the new quiet-branch case on its diagnostics assertion, and
  removing the per-step guard fails the first-step-throw case. The rail module stayed byte-identical
  to upstream — item (c) found no defect to fix.
  The unit's own review found one CRITICAL that the coverage work could not have caught: the new
  late `requestRender()` sat outside the D12 `try`/`catch`, so a throwing render rejected the promise
  installing the rail and could surface as an unhandled rejection. A refuter corroborated it, the
  provider required one bounded correction (22 declared diff lines of a 157-line budget), and a
  targeted validator approved the corrected candidate.

- [ ] 2.15c Guard the dispose diagnostic itself.
  Acceptance: `describeDisposeFailure` cannot throw out of the release loop's `catch` — its own
  call is guarded, so a step that throws a value whose string conversion throws, or an `Error`
  whose `message` is not a string, still leaves the remaining releases running — with a test that
  injects exactly that kind of thrown value and asserts the later releases still ran.
  Why: it is the one advisory finding left by the review of 2.15b (R3-002 WARNING, `informational`,
  `internal/assets/pi/biggz-shell.ts:470`). The loop was added so one failing step cannot skip the
  rest; a diagnostic that can itself throw defeats exactly that, and the fix is one guarded call.
  Evidence: the reviewer's ledger for lineage `review-e529ee8994efc911`.

- [x] 2.16 Decide whether the deployed Pi asset suites should be CI-gated. *(done: `7935638b`, merged as `63c34d5e`)*
  What is true today: no `*.test.mjs` under `internal/assets/pi/` runs in CI. `ci.yml` invokes exactly
  three node scripts (`check-skill-lint.mjs`, `check-provider-contract.mjs`,
  `verify-package-files.mjs`), there is no `package.json`, and `scripts/preflight.sh` does not run
  them either; only two Go tests shell out to node, each driving one specific asset. Every assertion
  count in the evidence table below - the rail's 270, the test directory's 9, the factory guard's 18,
  the ported suites - is therefore a **local run**, observed and reported as such, never a merge gate.
  Found while mapping 2.7e, not caused by it.
  Acceptance if it is taken on: one CI job that runs the suites the way this table ran them, so a
  regression in a deployed asset fails a PR instead of waiting for a human to remember the command.
  Evidence observed: the owner took it on and it is merged. `ci.yml` gained an `asset-suites` job
  (`needs: format`) that runs every suite in one command - `node --test internal/assets/pi/*.test.mjs
  internal/assets/pi/biggz-shell/*.test.mjs internal/assets/pi/test/*.test.mjs` - so the numbers in
  the table below are a merge gate from here on, not a local run. Two requirements came out of
  running the job rather than assuming it: it installs `@earendil-works/pi-tui@1.1.0` and resolves it
  through `PI_TUI_DIR`, because the resolver's oracle fallback is link-only (measured 505 tests, 491
  pass, **10 fail**, 4 skipped - the pointer-region and todo suites cannot link `MouseRegion`, filed
  as #172); and it pins Node 24 instead of the Node 20 of the other node jobs, because the suites
  import `.ts` modules directly. Measured: 221 tests on the trunk, 517 on this branch before 2.9 and
  518 after it, all green, and the job passed on the Linux runner inside the PR that added it
  (#170, #171).

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
| 2.6 | `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; the rail suite four ways (default resolution, explicit `PI_TUI_DIR` on real 0.85.1, no pi-tui at all, doctored candidate); `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiShellWidget\|TestPiFooterOwner\|TestPiExtensions\|TestPiAssets\|TestPiThemes' -v`; `go build ./...`; `go vet ./internal/install/...` | PASS. 256 node assertions (218 pre-existing plus the rail's 38) and 17 factory assertions, all green; the brand guard scans 34 deployed assets with its allowlist sub-test green; both `internal/install` packages green; build and vet clean. The port's fidelity was checked by diff, not by line count: against upstream `lib/shell-sidebar-layout.ts` (342 lines) the only hunks are the header comment and the cache-symbol rename. The D12 detector was proved adversarial rather than trusted: with a doctored candidate whose `dist/index.js` names every shell symbol except `ScrollView`/`VStack`, the suite FAILS naming that directory and those symbols, while with no pi-tui present it skips and says the oracle is the shape reference. Independent verification returned PASS WITH FINDINGS and both findings were closed in-unit — the D12 skip gap (now a fail) and a wrong upstream line count. Native review of the same tree: lineage `review-194f0bd3d9575e4b`, 6 paths, 1508 changed lines, correction budget 200, `approved`, four advisory findings now tracked as 2.15. | `bca03d1f` |
| 2.7 | | | |
| 2.8 | | | |
| 2.9 | | | |
| 2.10 | `go test ./internal/install/steps/ -run TestPiAssets -v` and `node --test internal/assets/pi/biggz-shell/*.test.mjs` | PASS. The guard scans 32 deployed assets; its stale-entry sub-test fails on an inserted stale entry with its delete-this message, verified by inserting one and reverting, and passes again after. Both renames confirmed by diff. An independent scan of the 32 assets found zero `gentle` literals outside the allowlist. | `e165d77a` |
| 2.11 | | | |
| 2.15a | `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/test/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; the containment fixture and the timing suite individually; both D12 directions; `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiShellWidget\|TestPiFooterOwner\|TestPiExtensions\|TestPiAssets\|TestPiThemes' -v`; `go build ./...`; `go vet ./internal/install/...` | PASS. 258 rail-suite assertions, 5 in the test directory (the containment fixture plus the three timing cases) and 17 factory, all green; brand guard scans 34 deployed assets; both `internal/install` packages green; build and vet clean; `~/.biggz` unchanged across the run. The containment is proved by execution, not prose: with a stub pi-tui that lacks `ScrollView`/`VStack`, the fixture's first test pins the link error itself (a `SyntaxError` naming the missing export, so the control cannot pass for an unrelated reason) and the second asserts the footer, widget, `/biggz-usage` and `alt+u` all install with the rail absent. The async-gap cases settle a test-held promise rather than sleeping, and both guards were mutation-checked. Independent verification returned PASS WITH FINDINGS and all four items were closed in-unit, including the unchecked cast — now `typeof import(...)`, validated by a scoped `tsc --noEmit` run whose error set is identical before and after. Native review of the same tree: lineage `review-8e81366b1896f49f`, 6 paths, 577 changed lines, `approved`, two advisory findings now in 2.15b. | `e7b5e3bf` |
| 2.15b | `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/test/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; the six timing cases and the dock re-host case individually; `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiShellWidget\|TestPiFooterOwner\|TestPiExtensions\|TestPiAssets\|TestPiThemes' -v`; `go build ./...`; `go vet ./internal/install/...` | PASS. 261 rail-suite assertions, 9 in the test directory and 17 factory, all green; brand guard scans 34 deployed assets; both `internal/install` packages green; build and vet clean; `~/.biggz` unchanged. Two independent verifications: the first returned PASS WITH FINDINGS with two WARNINGs (the last-step-only dispose test, and a swallow with no diagnostic) and both were closed in-unit; the second returned PASS WITH FINDINGS with three SUGGESTIONs, two of which were closed (the unexecuted `quiet` branch now has a case, the comment now matches the four-step list). Native review of the same tree: lineage `review-e529ee8994efc911`, 4 paths, 313 changed lines, correction budget 157 — and it found a CRITICAL the coverage work could not have caught, `tui.requestRender()` sitting outside the D12 `try`/`catch` on the late install path, so a throwing render could reject the installing promise. A refuter corroborated it; one bounded correction was declared (22 diff lines) and applied; a targeted validator approved the corrected candidate; the review closed `approved` with one advisory finding left (2.15c). | `516a1742` |
| 2.7a | `node --test internal/assets/pi/biggz-shell/native-pointer-region.test.mjs`; `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/test/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiExtensions\|TestPiAssets\|TestPiShellWidget\|TestPiFooterOwner' -v`; `go build ./...`; `go vet ./internal/install/...` | PASS. The port is byte-faithful: `diff -U0` against upstream shows one hunk, the two provenance lines, and the ported suite's assertion stream is identical to upstream's (6 cases, same order). 267 rail-suite assertions, 9 in the test directory and 17 factory, all green; the brand guard now scans 35 deployed assets with the factory count still 16; both `internal/install` packages green; build and vet clean; `~/.biggz` unchanged. Independent verification returned PASS with three SUGGESTIONs and no defect — one of them (the derived fixture losing the independent cross-check on `PI_TUI_SHELL_SYMBOLS`) was closed in-unit by asserting that the doctored fixture omits exactly the layout shapes. Native review of the same tree: lineage `review-1bfa97db2f9357cf`, 6 paths, 486 changed lines, `approved`, three advisory findings now tracked as 2.7c. The first reviewer run died on a provider transport error (`Stream ended without finish_reason` after 157s, no mutation) and the fresh-STATUS-then-resubmit route recovered it. | `303e7b55` |
| 2.7b | `node --test internal/assets/pi/biggz-todo.test.mjs`; `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/test/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; `go test ./internal/install/... -count=1`; `go test ./internal/install/steps/ -run 'TestPiExtensions\|TestPiAssets' -count=1`; `go build ./...`; `go vet ./internal/install/...` | PASS, with one acceptance clause uncovered. 10/10 in the new suite (the tool write/show path, the snapshot in `details`, the branch replay, the finished-list drop, staleness, the collapse key, hover/click, and the R3-001 containment), 267 shell assertions, 9 in the test directory, 18 factory; both `internal/install` packages green; build and vet clean; `~/.biggz` unchanged. The brand guard's `gentleTodo` allowlist entry is deleted, not kept, and the factory count moved 16 → 17 in the deploy list and both guards. NOT covered: no test loads the todo and subagents factories together to assert both registrations survive, so 2.7b's coexistence clause rests on the two keys being distinct and each suite asserting its own — recorded as 2.7e. Native review of the same tree: lineage `review-e12d5b1d91db6315`, 8 paths, 683 changed lines, risk `medium`, correction budget 200, one consolidated lens `review-reliability`, `approved`, three advisory findings now in 2.7e. | `fdf89b6e` |
| 2.7c | `node --test internal/assets/pi/biggz-shell/native-pointer-region.test.mjs`; `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/test/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; `go test ./internal/install/... -count=1`; plus six mutations of the module, each restored byte-identically | PASS. 9/9 in the pointer-region suite (6 upstream + 3 new), 270 shell assertions, 9 in the test directory, 18 factory; both `internal/install` packages green. Every new case is discriminating: six mutations (no sibling clear; sibling clear handed a copy; `disabled` gate dropped; module gating the interactive child; observer requesting a render unconditionally; observer never requesting one) each fail exactly the case that covers them, and `git hash-object` of the module returned to the HEAD blob every time. The coverage exposed a real boundary rather than a defect: `disabled` gates the region's callbacks but not an interactive child, which pi-tui's `MouseRegion` dispatches first. Independent verification returned PASS for all three items, judged the boundary a fidelity requirement of the port, and found no defect (five advisories, all wording or environment). | `901dc435` |
| 2.7e | `node --test internal/assets/pi/biggz-todo.test.mjs`; `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs`; `node --test internal/assets/pi/test/*.test.mjs`; `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs`; `go test ./internal/install/... -count=1`; `go build ./...`; `go vet ./internal/install/...`; plus four mutations of the two modules, each restored byte-identically | PASS, with R3-001 closed as a non-goal. 13/13 in the todo suite (10 upstream + 3 new), 270 shell assertions, 9 in `test/`, 18 factory; both `internal/install` packages green; build and vet clean. The coexistence case drives the subagents runtime's own publisher into the same host and asserts one widget map holding both keys with distinct placements plus the registered `"todo"` rail part; R3-002 failed before its fix and passes after (`session_shutdown` now clears the card, matching the shell extension); R3-003 pins exactly one render on a hover clear. R3-001 is fixed without a test by deliberate decision: the pre-load ordering needs an injected loader and the module's cache is private, so the test would pin scheduling rather than the contract. Four mutations (the todo taking the runtime's key, the runtime losing its placement, the rail part under another key, `onLeave` without its render request) each fail exactly the case that covers them. | `c5b521c7` |
| 2.9 | `node --test internal/assets/pi/biggz-shell/theme-customization.test.mjs`; `node --test internal/assets/pi/*.test.mjs internal/assets/pi/biggz-shell/*.test.mjs internal/assets/pi/test/*.test.mjs`; `go test ./internal/install/... -count=1`; `go build ./...`; `go vet ./internal/install/...`; plus a mutation of the module | PASS. Reproduced the throw before the fix and observed the RED case fail with the same error. 4/4 in the theme-customization suite, 518 tests in the full asset run with 0 failures, both `internal/install` packages green, build and vet clean. The new case previews the seven shipped themes exactly as they ship and asserts which ones exercise the empty `text` role, and a mutation rendering `0m` instead of `39m` fails exactly it. First unit in this feature to declare a deviation from upstream in the ported module's own header (D16). Native review of the same tree: lineage `review-8c94163dee5a14e0`, 2 paths, 48 changed lines, risk `medium`, budget 24, one consolidated lens `review-reliability`, `approved`, no findings. | `7dabf94d` |
| 2.11 | `node --test internal/assets/pi/biggz-shell/review-sidebar-state.test.mjs`; `node --test internal/assets/pi/biggz-shell.test.mjs internal/assets/pi/biggz-shell/*.test.mjs internal/assets/pi/test/*.test.mjs`; the full asset run (`node --test internal/assets/pi/*.test.mjs internal/assets/pi/biggz-shell/*.test.mjs internal/assets/pi/test/*.test.mjs`); `go test ./internal/install/... -count=1`; `go build ./...`; `go vet ./internal/install/...` | PASS, with the unit's premise corrected in-unit. 528 asset tests, 0 failures (17 in the review-sidebar suite: the 7 that existed plus 10 publisher cases adapted from upstream and pinned with two verbatim envelopes from this session's own review); both `internal/install` packages green; build and vet clean. The brand guard caught the two review tool names as new literals - the allowlist gained them, and correcting their reasons exposed that the three review-protocol entries repeated 2.11's wrong premise. Native review NOT run: the session closed at the owner's request, so this candidate is un-reviewed. | `c5ad5731` |

## Known risks

1. `shell-sidebar-layout.ts` binds to private pi-tui internals
   (`Symbol.for("@earendil-works/pi-tui/layout-node")`, `host.layoutRoot`, `tui.terminal`).
   A pi-tui bump can make the rail disappear silently. D12 chose upstream's silent degradation
   over a visible notice, so the suite is the detector: a discovered install that stopped naming
   a required symbol fails the suite with the directory and the symbols, and only a machine with
   no pi-tui at all falls back to the oracle and skips. Two limits stay open in task 2.15: the
   detector cannot see a bump that keeps every symbol name, and a dropped `ScrollView`/`VStack`
   export fails at ESM link time and takes the whole shell extension, not just the rail (R3-001).
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
    context, cost, usage and extension statuses instead. Re-examined in 2.6 and answered there
    (D10): the gap is real but the three facts have no shell-side successor, because upstream's
    shell never showed them either, and the retired footer's segments read fields no producer
    ever supplied. Their owner is now **task 2.11**, which already has to build the producer side
    of the review sidebar. The gates changed with the footer: `BIGGZ_PRETTY=0` still turns it off
    because the factory reads `isPrettyEnabled()`, and `BIGGZ_SHELL=0` is the shell-specific
    switch.
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
    the defect is the launcher's, and upstream's to fix. Two further facts learned by running the
    flow: the refuter and the targeted validator are host-mediated ROLES with their own routing
    keys (`review-refuter` and `review-validator`, `lib/review-host-relay.ts:306-309`), so a config
    with only the four `review-*` lens keys fails the moment a lens raises an inferential blocker;
    and a bounded correction's plan must be DECLARED BEFORE the tree is edited — the correction
    binding carries the frozen target identity, so editing first makes the binding mismatch and the
    capture is rejected with "does not carry one non-empty matching provider lineage and target
    token", which is recoverable only by restoring the reviewed bytes and declaring the plan first.
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

Verified outcome: the footer is real, the row under the editor follows it and fullscreen gets
its rail — `biggz install` deploys a shell bar that owns pi's single footer slot, the old
powerline footer is retired, the same factory paints the header row and registers `/biggz-usage`,
and a second deployed helper replaces pi's private layout root with the transcript-plus-rail
split when the terminal can hold it. Tasks 1.1, 1.2, 1.3, 2.1, 2.2a, 2.2b, 2.3, 2.4, 2.5, 2.6
and 2.10 are done and green.

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

2.6 — the rail is the port's most invasive move and it is exactly as invasive as upstream's.
`installSidebar` monkey-patches pi's private layout node from inside the `setFooter` callback,
replacing the root with the transcript-plus-rail split, saving and restoring the property
descriptor, re-hosting the dock to reclaim the footer's reserved row, and re-binding on a 100 ms
timer because pi swaps renderers without an event. The port is faithful to the line — the diff is
two hunks — and every guard came with it. Three choices shaped the unit: the rail carries no
review facts (D10, with risk 12 handed to 2.11), the rail owns the `top` header case so the
header cannot be painted twice (D11), and missing internals degrade silently with the suite as
the detector (D12). That last one forced the test plumbing open: the oracle gained
`ScrollView`/`VStack`, the symbol gate gained both names, and the resolver gained a report so the
suite can tell "nothing installed" from "installed and no longer exposing the symbols" — a
distinction the first version of that detector got wrong, which is exactly what R3-002 caught.

Known failures outside this work, identical on `HEAD` before the change (verified by
stashing): `internal/agents/pi` 4, `internal/git` 1, `internal/project` 6, `cmd/biggz` 2,
`internal/review` 2. `internal/agents/pi`'s four are the same `PI_CODING_AGENT_DIR`
hermeticity class of bug and are not yet fixed.

Next step: 2.9 and 2.11 are done, and 2.11's candidate `c5ad5731` is committed WITHOUT its native review
because the owner closed the session, so the first decision of the next session is to review it or fold it
into the next candidate. Queued behind that: 2.8 (prompt ownership), 2.12 (the two guards 2.4's
verification found narrower than their invariants), 2.13 (the two reliability findings the native review of
2.5 admitted), 2.14 (the banner, deferred by D9) and 2.15c (the one advisory finding the review of 2.15b
left). 2.16 is closed: the asset suites run in CI, so the counts in the table above are a merge gate rather
than a local claim.

Decided, no longer open: the sixteen biggz SDD agent files the suite rendered into the live
`~/.gentle-shell/agent/agents/` directory stay in place; task 2.1 revisits the agent deploy
path with the design in front of us (risk 5).

Commit status: fourteen work units on `feat/shell-visual-parity` — `2bfa0ec0` (1.1), `bff680c1`
(1.3), `07aa401a` (1.2), `8398ddeb` (2.1), `35cdc44f` (2.2a), `1cffbe2c` (2.2b), `562ef0d0`
(2.3), `e165d77a` (2.10), `da28997d` (2.4), `7fe76875` (2.5), `bca03d1f` (2.6), `e7b5e3bf`
(2.15a), `516a1742` (2.15b), `303e7b55` (2.7a) — plus the tracking commits. Push and pull request
remain human decisions. A pre-existing untracked change,
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

2.6 is the largest unit yet by changed lines: six paths, 1508 changed lines and 1479 insertions,
of which 779 are its suite and 357 the rail module itself. It was not split either — the rail
module, its wiring and its guard describe one seam, and the deploy entry cannot land before the
file it points at — so it followed 2.5 down the same path: native review before the commit, on
the `current-changes` projection, lineage `review-194f0bd3d9575e4b`, one `review-reliability`
lens, medium tier, correction budget 200, `approved` with four advisory findings (2.15). The
staged tree was again compared against the reviewed candidate tree and matched byte for byte.

2.15a is the first unit written to close another unit's review, and it changed a promise rather
than a feature: D12 said losing the private internals costs the rail and nothing else, but the
static import meant a dropped `ScrollView`/`VStack` export cost the entire shell, because no guard
runs when the specifier stops resolving. The rail is now loaded lazily, with the type still
derived from the module so the call signatures stay checked, and the containment is proved by a
stub pi-tui in a subprocess rather than argued. Its own review then found the two things a
containment fix usually leaves unproven — the synchronous branch that every later session takes,
and the render a late install never requests — which is exactly the kind of finding the review is
for, and they are 2.15b.

2.15b is what happens when the review is allowed to review its own fixes, and it earned its keep
twice over. The four findings it closed were coverage: the steady-state synchronous install that
no test reached, the render a late install never requested, the dock re-host path every fixture
skipped, and a wiring block asserted by grepping the source. Closing them introduced the render
request — and the review of that change found a CRITICAL nobody's coverage could have caught: the
request sat outside the D12 `try`/`catch`, so a throwing render would have rejected the promise
that installs the rail and surfaced as an unhandled rejection, which is precisely the failure mode
the containment exists to prevent. The flow then did what it is designed to do: a refuter
corroborated the finding, the provider required one bounded correction, the plan was declared (22
diff lines against a 157-line budget) and applied, and a targeted validator approved the corrected
candidate. The lesson worth keeping is operational and now lives in risk 13: declare the plan
before editing, because the binding carries the frozen target identity.

- 2026-10-11 — Unit 2.7b closed as `fdf89b6e` (the todo producer). The uncommitted slice moved from
  `feat/pi-native-mcp` to this branch without touching its bytes: the six modified paths are
  identical at both branch tips, so `git switch` carried them over and `git diff --binary` plus the
  two new files' digests (`sha256:792cd722…`, `e2be2aef…`) were equal before and after. The review
  route needed the intended-untracked selection declared up front — `inspect` with
  `untrackedScope: "select"` naming the two new files, which keeps the unrelated
  `openspec/changes/fix-archived-change-identity/` out of the candidate — and the first `start` came
  back `lenses-invalid` until the lens reason dropped its backticks and escaped quotes: the provider
  validates the reason text, not only the lens names. The provider then consolidated the two
  selected lenses into one `review-reliability` pass, which came back `approved` on the first
  reviewer run after a `pi_host_relay` forecast of exactly one model run. One acceptance clause is
  uncovered (the coexistence test) and became 2.7e item 1; the three review findings are 2.7e
  items 2-4. Lesson for the next unit: read the acceptance clause by clause before ticking it, not
  after.
- 2026-10-11 — Unit 2.7c closed as `901dc435` (the three coverage findings of the 2.7a review). The
  work was three cases, and it paid for itself by falsifying an assumption rather than confirming
  one: proving the disabled guard against an interactive child showed that the guard does not reach
  it, because pi-tui dispatches the child first and the region's own callback — where the check
  lives — never runs. The test now pins that contract, the independent verifier called it upstream
  fidelity rather than a defect, and the open question (accept the boundary or diverge) is recorded in
  2.7c for the owner. Two operational notes: the native `assess` cannot produce a tier while the
  unrelated untracked `openspec/changes/fix-archived-change-identity/` is present, so this test-only
  unit was gated by the independent verifier its plan mandates instead of by a review; and this suite
  resolves pi-tui 1.1.0 where 2.7a's evidence cites 0.85.1, which matters only because the
  interactive expectations come from that dispatch.
- 2026-10-11 — Decision D15 recorded (by the human): the child-first boundary of the `disabled` guard
  stays as upstream has it. The 2.7c coverage is what made the question concrete — without the test,
  the field would still read as a guarantee nobody had measured — and the answer keeps the port's
  byte-fidelity intact while the three new cases hold the real behaviour in place.
- 2026-10-11 — Unit 2.7e closed as `c5b521c7`: the coexistence clause 2.7b's acceptance asked for and
  the three advisories its review admitted. Two of those were real defects rather than coverage
  gaps: `session_shutdown` left the card painted for the next session, and a `show()` that ran before
  the lazy rail import resolved never asked the rail to repaint. The second is fixed but untestable
  without an injected loader, so it is recorded as the non-goal the acceptance permits instead of
  being dressed up as verified. The coexistence case reads the other widget's key from the deployed
  asset - and `WIDGET_KEY` is now exported for the same reason - because a copied literal is exactly
  what would hide a collision. Mapping this unit also surfaced 2.16: no `*.test.mjs` suite under
  `internal/assets/pi/` runs in CI, so every assertion count in the table above is a local run rather
  than a merge gate.
- 2026-10-11 — Unit 2.9 closed as `7dabf94d`. It is the first slice in this feature to change a
  ported module's behaviour on purpose: upstream's preview throws on the empty colour role that pi
  uses for the terminal default, and three of the seven themes this repository ships use it, so the
  customize view was unreachable with biggz's own theme set. The deviation is declared in the
  module's header (D16) because that header used to say the file was verbatim, and a fidelity claim
  nobody re-reads is how a port quietly stops being one. The test that closes it previews the
  shipped themes as they ship instead of a doctored fixture, which is the difference between
  covering a role and covering a role-shaped stand-in.
- 2026-10-11 — Unit 2.16 closed, and it is the only slice in this feature that is repo-wide rather
  than part of the port: CI now runs every asset suite (`7935638b`, merged as `63c34d5e`). The
  evidence table above stops being a local claim and becomes a merge gate, which is what the table
  should have been all along - the suites existed and passed for fifteen units while nothing ran
  them. Two details only running it could establish: the job must install a real `pi-tui` (the
  oracle cannot link two of the suites) and it needs Node 24 (the suites import `.ts` directly).
- 2026-10-11 — Unit 2.11 closed as `c5ad5731`. The unit's own premise was the thing to correct: it said
  the parser must move to biggz's contract because "biggz speaks a different contract", and measuring showed
  the opposite - biggz's contract has no lifecycle vocabulary to display, and the ported strings already
  matched the envelopes the review tools return. The real gap was the module's missing half: upstream pairs
  the parser with a publisher, the port brought only the parser, so nothing called it and the bar never
  received a review. Porting that half plus the wiring made the group render for the first time. Three
  by-products were worth more than the feature: the D10 guard had to be reasoned about rather than deleted
  (it still bans the authority vocabulary and now asserts the display arrives through the event), the brand
  guard caught two new literals whose allowlist reasons turned out to repeat the wrong premise, and the real
  closure envelope turned out to differ from upstream's own fixture in a way that costs the file scope when
  a capture closes. The native review did not run: the owner closed the session, so the candidate is
  committed un-reviewed and that is recorded here rather than implied.
