// Ported from gentle-pi 3.7.0 tests/shell-sidebar-layout.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged apart from the
// biggz-ai product label and brand wording below (brand rename), plus the D10
// (no review facts in the rail), D11 (the rail owns the `top` header case) and
// D12 (fail loudly on a pi-tui shape change) checks this port adds.
//
// pi-tui is supplied by the repository's test resolver (test/pi-tui-resolver.mjs):
// the real install when reachable (PI_TUI_DIR, or a discovered candidate),
// otherwise the test-only oracle. The layout module binds to pi-tui 0.85.1
// private shapes, so the "shapes" test below fails loudly when the resolved
// package stops exposing them (D12); it never adds a user-visible warning to the
// shipped rail. The R3-002 fail rule and its comment enumerate all four resolver
// states; the R3-002 proof at the foot of this file runs the failure and the
// handled-stale-sibling directions in subprocesses.
import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { installPiTuiResolver, resolvePiTuiEntry, piTuiResolutionReport, PI_TUI_SPECIFIER, PI_TUI_SHELL_SYMBOLS } from "../test/pi-tui-resolver.mjs";

const resolution = installPiTuiResolver();
const report = piTuiResolutionReport(process.env);
const tui = await import(PI_TUI_SPECIFIER);
const { ScrollView, VStack, visibleWidth } = tui;
const realEntry = resolvePiTuiEntry(process.env);
const realTui = realEntry ? await import(pathToFileURL(realEntry.file).href) : null;
const { installSidebar, invalidateSidebar } = await import("./shell-sidebar-layout.ts");
const { sidebarHeader, sidebarPart, sidebarState } = await import("./shell-sidebar.ts");

const MODULE_PATH = path.join(path.dirname(fileURLToPath(import.meta.url)), "shell-sidebar-layout.ts");
const WIRING_PATH = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "biggz-shell.ts");
const NODE = Symbol.for("@earendil-works/pi-tui/layout-node");
const CACHE = Symbol.for("biggz-ai.experimental-sidebar.cache");
const BANNER = "✿ biggz ✿";
const theme = { fg: (_color, text) => text, bold: (text) => text };

function fixture(mode = "fullscreen", columns = 140) {
	const original = () => ({ type: "vstack", entries: [] });
	const root = { render: () => ["transcript"], invalidate() {}, [NODE]: original };
	let renders = 0;
	const host = { mode, terminal: { columns }, layoutRoot: root, requestRender() { renders++; } };
	const tui = host;
	const bottom = sidebarPart(tui, "footer", { render: (_width = 80) => ["Status"], invalidate() {} });
	return { host, tui, root, original, bottom, renders: () => renders };
}

function rail(f) {
	const node = f.root[NODE]();
	assert.equal(node.type, "hstack");
	return node.entries[1].component;
}

// Finds the [left, scroll] hstack regardless of whether it is returned directly
// (no active header) or nested one level under the header vstack.
function hstackOf(f) {
	const node = f.root[NODE]();
	if (node.type === "hstack") return node;
	assert.equal(node.type, "vstack");
	const nested = node.entries[1]?.component[NODE]?.();
	assert.ok(nested, "the vstack's second entry must expose the hstack via NODE");
	return nested;
}

function railWithHeader(f) {
	return hstackOf(f).entries[1].component;
}

// ── D12: the resolved pi-tui must still expose the shapes the rail binds to ──

test("the resolved pi-tui exposes the 0.85.1 rail shapes or the suite fails loudly (D12)", (t) => {
	t.diagnostic(`pi-tui resolution: ${resolution.mode}; real entry: ${realEntry?.file ?? "none"}`);
	for (const symbol of ["ScrollView", "VStack"]) {
		assert.ok(PI_TUI_SHELL_SYMBOLS.includes(symbol), `the resolver's shell-symbol list must name ${symbol}: a discovered install is only accepted when it mentions every symbol the shell modules import`);
	}
	assert.equal(typeof ScrollView, "function", "the rail needs the ScrollView constructor");
	assert.equal(typeof VStack, "function", "the rail needs the VStack constructor");
	assert.equal(typeof visibleWidth, "function", "the rail needs visibleWidth to bound every rail line");
	const scroll = new ScrollView({ render: () => ["x"], invalidate() {} }, { scrollbar: "auto", follow: "none", primary: false, overscroll: "contain" });
	for (const method of ["getContentWidth", "scrollBy", "updateLayout", "hideTransientScrollbar", "handleMouse", "invalidate"]) {
		assert.equal(typeof scroll[method], "function", `ScrollView.${method} is required by the rail and must stay callable`);
	}
	assert.equal(scroll.getContentWidth(50), 50, "an auto scrollbar must not reserve a column");
	assert.equal(scroll.scrollTop, 0);
	assert.equal(scroll.scrollbar, "auto");
	const stack = new VStack([], { gap: 2, align: "stretch" });
	assert.equal(typeof stack[NODE], "function", "VStack must expose the layout-node symbol the rail patches");
	assert.equal(stack[NODE]().type, "vstack");
	const header = fs.readFileSync(MODULE_PATH, "utf8").split("\n").slice(0, 40).join("\n");
	assert.match(header, /0\.85\.1/, "the module header must state the pinned pi-tui shape version (D12)");
	assert.match(header, /pi-tui/, "the module header must name pi-tui as the pinned dependency (D12)");
});

// D12's fail rule, split from the shape assertions so the reach of "fails
// loudly" is explicit, and kept in exactly one place (R3-002). The resolver has
// four states here:
//   1. an install resolves -> the shape assertions above cover it; return.
//   2. nothing resolved AND a candidate was rejected for missing
//      PI_TUI_SHELL_SYMBOLS names -> this suite is the only detector, so FAIL
//      with the directory and the missing names.
//   3. a missing-symbols candidate exists but some other install resolved (a
//      usable sibling, or an explicit PI_TUI_DIR) -> the resolver skipped the
//      stale sibling by design, so it is a handled condition; return.
//   4. neither a usable install nor a missing-symbols candidate: no install at
//      all, or only directories classified `not-a-pi-tui-package` (wrong
//      package name, a missing or renamed `main`, or an unparsable
//      package.json) -> skip; the oracle is the shape reference in that run.
test("a discovered pi-tui that loses a required shell symbol fails the suite instead of degrading silently (D12)", (t) => {
	if (realEntry) return; // state 1 and 3: something resolved, rejected siblings were skipped by design
	const missingSymbols = report.rejected.filter((candidate) => candidate.reason === "missing-symbols");
	if (missingSymbols.length === 0) {
		// state 4: nothing resolved and no candidate lost a symbol
		t.skip("no usable pi-tui install resolved (absent, or only directories that are not pi-tui packages) — the oracle is the shape reference in this run");
		return;
	}
	assert.fail(
		"a pi-tui install exists but no longer names every shell symbol, so the rail would silently fall back to the oracle: " +
			missingSymbols.map((candidate) => `${candidate.dir} (missing ${(candidate.missing ?? []).join(", ") || "unknown"})`).join("; ") +
			" — restore the symbol names or update PI_TUI_SHELL_SYMBOLS deliberately",
	);
});

test("the real pi-tui exposes ScrollView/VStack with the shapes the rail binds to", (t) => {
	if (!realTui) {
		t.skip("no usable pi-tui install resolved (absent, or only directories that are not pi-tui packages) — the oracle is the shape reference in this run");
		return;
	}
	assert.deepEqual(report.entry, realEntry, "the resolution report must agree with resolvePiTuiEntry");
	assert.equal(typeof realTui.ScrollView, "function");
	assert.equal(typeof realTui.VStack, "function");
	const scroll = new realTui.ScrollView({ render: () => ["x"], invalidate() {} }, { scrollbar: "auto" });
	assert.equal(scroll.getContentWidth(50), 50);
	assert.equal(typeof scroll.handleMouse, "function", "the real ScrollView must inherit a mouse dispatcher the rail can fall back to");
	const stack = new realTui.VStack([]);
	assert.equal(stack[NODE]().type, "vstack");
});

// ── installation lifecycle ──────────────────────────────────────────────────

test("installation on a missing-terminal host is a harmless no-op", () => {
	const dispose = installSidebar({}, theme);
	assert.doesNotThrow(dispose);
	dispose();
});

test("only fullscreen at 140 columns activates; shrinking restores bottom paint", (t) => {
	for (const [mode, width, active] of [["regular", 140, false], ["fullscreen", 139, false], ["fullscreen", 140, true]]) {
		const f = fixture(mode, width);
		t.after(installSidebar(f.tui, theme));
		assert.equal(f.root[NODE]().type, active ? "hstack" : "vstack", `${mode}/${width}`);
		assert.deepEqual(f.bottom.render(80), active ? [] : ["Status"]);
		f.host.terminal.columns = 139;
		assert.equal(f.root[NODE]().type, "vstack");
		assert.deepEqual(f.bottom.render(80), ["Status"]);
	}
});

test("bottom paint follows placement and resize before the next layout pass", (t) => {
	const f = fixture("fullscreen", 140);
	let placement = "auto";
	t.after(installSidebar(f.tui, theme, () => placement));
	assert.equal(f.root[NODE]().type, "hstack");
	assert.deepEqual(f.bottom.render(80), []);

	placement = "bottom";
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	assert.equal(f.root[NODE]().type, "vstack");
	placement = "right";
	assert.equal(f.root[NODE]().type, "hstack");
	f.host.terminal.columns = 139;
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	assert.equal(f.root[NODE]().type, "vstack");
	placement = "hidden";
	f.host.terminal.columns = 180;
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	assert.equal(f.root[NODE]().type, "vstack");
	placement = "auto";
	assert.equal(f.root[NODE]().type, "hstack");
	assert.deepEqual(f.bottom.render(80), []);
});

// ── the two-branch proof ────────────────────────────────────────────────────

test("rail active: layoutRoot + node symbol + fullscreen + width installs the [rail, transcript] hstack", (t) => {
	const f = fixture("fullscreen", 140);
	t.after(installSidebar(f.tui, theme));
	const node = f.root[NODE]();
	assert.equal(node.type, "hstack");
	assert.equal(node.gap, 3);
	assert.deepEqual(node.entries.map(({ basis, grow, shrink, minSize }) => ({ basis, grow, shrink, minSize })), [
		{ basis: 0, grow: 1, shrink: 1, minSize: 1 },
		{ basis: 50, grow: 0, shrink: 0, minSize: 50 },
	]);
	assert.deepEqual(f.bottom.render(80), [], "the registered footer part suppresses its narrow paint while the rail is active");
	assert.equal(sidebarState(f.tui).active, true);
	assert.match(rail(f).render(50).join("\n"), /Status/);
	assert.deepEqual(f.root.render(), ["transcript"], "the native render is not patched");
});

test("no-rail: a root without the layout symbol stays untouched and the bottom keeps painting", (t) => {
	const f = fixture("fullscreen", 140);
	Reflect.deleteProperty(f.root, NODE);
	t.after(installSidebar(f.tui, theme));
	assert.equal(typeof f.root[NODE], "undefined", "the missing symbol must not be invented");
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	assert.equal(sidebarState(f.tui).active, false);
});

test("no-rail: a host without layoutRoot keeps native behavior", (t) => {
	const f = fixture("fullscreen", 140);
	delete f.host.layoutRoot;
	t.after(installSidebar(f.tui, theme));
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	assert.equal(sidebarState(f.tui).active, false);
	assert.deepEqual(f.root.render(), ["transcript"]);
});

test("a real layout-root object is patched through NODE and its native render survives", (t) => {
	const transcript = { render: () => ["transcript"], invalidate() {} };
	const root = new VStack([{ component: transcript, basis: 0, grow: 1, minSize: 1 }]);
	const originalRender = root.render;
	const host = { mode: "fullscreen", terminal: { columns: 140 }, layoutRoot: root, requestRender() {} };
	const bottom = sidebarPart(host, "footer", { render: (_width) => ["Status"], invalidate() {} });
	t.after(installSidebar(host, theme));
	const node = root[NODE]();
	assert.equal(node.type, "hstack");
	assert.match(node.entries[1].component.render(50).join("\n"), /Status/);
	assert.equal(root.render, originalRender, "the native root render is not patched");
	assert.deepEqual(root.render(140), ["transcript"]);
	assert.deepEqual(bottom.render(80), []);
});

// ── header row (D11) ────────────────────────────────────────────────────────

test("an active header wraps the hstack in a vstack and removes the banner from the rail", (t) => {
	const f = fixture();
	sidebarHeader(f.tui, { render: (width) => [`HEADER ${width}`], invalidate() {} });
	t.after(installSidebar(f.tui, theme));

	const node = f.root[NODE]();
	assert.equal(node.type, "vstack");
	assert.equal(node.gap, 0);
	assert.equal(node.align, "stretch");
	assert.deepEqual(node.entries.map(({ basis, grow, shrink, minSize }) => ({ basis, grow, shrink, minSize })), [
		{ basis: "auto", grow: 0, shrink: 0, minSize: 1 },
		{ basis: 0, grow: 1, shrink: 1, minSize: 1 },
	]);
	const header = node.entries[0].component;
	assert.deepEqual(header.render(0), ["HEADER 138"], "the header renders at the terminal width minus the rail's right inset, not the rail width");
	const hstack = hstackOf(f);
	assert.equal(hstack.type, "hstack");
	const lines = railWithHeader(f).render(50);
	assert.ok(!lines.join("\n").includes(BANNER), "the header carries the brand now, not the banner");
	assert.equal(lines[0]?.trim(), "", "the rail opens with a blank row under the header");
	assert.notEqual(lines[1]?.trim(), "", "the first card starts on the second row");
});

test("the header leaf carries the rule row: a two-line header renders both lines at full width", (t) => {
	const f = fixture();
	sidebarHeader(f.tui, { render: (width) => [`HEADER ${width}`, `RULE ${width}`], invalidate() {} });
	t.after(installSidebar(f.tui, theme));
	const node = f.root[NODE]();
	assert.equal(node.type, "vstack");
	assert.deepEqual(node.entries[0].component.render(0), ["HEADER 138", "RULE 138"]);
});

test("below the sidebar breakpoint a rule-like header paints above native layout without a rail", (t) => {
	const f = fixture("fullscreen", 139);
	sidebarHeader(f.tui, { render: (width) => [`HEADER ${width}`, `RULE ${width}`], invalidate() {} });
	t.after(installSidebar(f.tui, theme));
	const node = f.root[NODE]();
	assert.equal(node.type, "vstack");
	assert.deepEqual(node.entries[0].component.render(139), ["HEADER 139", "RULE 139"]);
	assert.deepEqual(f.bottom.render(139), ["Status"]);
});

test("without a registered header the rail keeps the banner and the plain hstack", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	assert.equal(f.root[NODE]().type, "hstack");
	assert.ok(rail(f).render(50).join("\n").includes(BANNER));
});

test("a header too narrow to show anything falls back to the plain hstack and the banner", (t) => {
	const f = fixture();
	sidebarHeader(f.tui, { render: () => [""], invalidate() {} });
	t.after(installSidebar(f.tui, theme));
	assert.equal(f.root[NODE]().type, "hstack", "a blank header line does not earn its own row");
	assert.ok(rail(f).render(50).join("\n").includes(BANNER));
});

test("header content changes reach the header row while an unrelated part is untouched", (t) => {
	const f = fixture();
	let label = "one";
	sidebarHeader(f.tui, { digest: () => label, render: () => [`HEADER ${label}`], invalidate() {} });
	t.after(installSidebar(f.tui, theme));
	const header = () => f.root[NODE]().entries[0].component;
	assert.deepEqual(header().render(0), ["HEADER one"]);
	label = "two";
	assert.deepEqual(header().render(0), ["HEADER two"]);
});

test("top header remains above native layout at narrow widths and wide bottom/hidden without a rail", (t) => {
	for (const width of [100, 60, 180]) {
		for (const placement of width === 180 ? ["bottom", "hidden"] : ["auto"]) {
			const f = fixture("fullscreen", width);
			sidebarHeader(f.tui, { render: (available) => [`HEADER ${available}`, `RULE ${available}`], invalidate() {} });
			t.after(installSidebar(f.tui, theme, () => placement));
			const node = f.root[NODE]();
			assert.equal(node.type, "vstack", `${width}/${placement} keeps the top header`);
			assert.deepEqual(node.entries[0].component.render(width), [`HEADER ${width}`, `RULE ${width}`]);
			assert.deepEqual(node.entries[1].component[NODE](), f.original(), "native layout remains underneath");
			assert.deepEqual(f.bottom.render(width), ["Status"], "bottom Status survives without a rail");
			assert.equal(sidebarState(f.tui).active, false);
		}
	}
});

test("below-input header never duplicates into the top row, even when the rail is inactive", (t) => {
	for (const [width, placement] of [[60, "auto"], [100, "auto"], [180, "bottom"], [180, "hidden"]]) {
		const f = fixture("fullscreen", width);
		sidebarHeader(f.tui, { render: () => ["HEADER"], invalidate() {} });
		t.after(installSidebar(f.tui, theme, () => placement, () => "below-input"));
		assert.deepEqual(f.root[NODE](), f.original(), "the rail never paints the below-input header");
		assert.deepEqual(f.bottom.render(width), ["Status"]);
	}
});

test("below-input header removes only the rail's top row", (t) => {
	const f = fixture();
	let header = "top";
	sidebarHeader(f.tui, { render: () => ["header"], invalidate() {} });
	t.after(installSidebar(f.tui, theme, () => "auto", () => header));
	assert.equal(f.root[NODE]().type, "vstack");
	header = "below-input";
	assert.equal(f.root[NODE]().type, "hstack");
	assert.deepEqual(f.bottom.render(80), []);
	header = "top";
	assert.equal(f.root[NODE]().type, "vstack");
});

test("the header component's handleMouse delegates to the registered header part", (t) => {
	const f = fixture();
	const received = [];
	let result = { handled: true, render: true };
	sidebarHeader(f.tui, {
		render: () => ["HEADER"],
		invalidate() {},
		handleMouse(event) {
			received.push(event);
			return result;
		},
	});
	t.after(installSidebar(f.tui, theme));
	const node = f.root[NODE]();
	assert.equal(node.type, "vstack");
	const header = node.entries[0].component;
	const event = { type: "click", button: "left", x: 5, y: 0, screenX: 5, screenY: 0, width: 140, height: 1, shift: false, alt: false, ctrl: false };
	assert.deepEqual(header.handleMouse(event), { handled: true, render: true });
	assert.equal(received.length, 1);
	assert.equal(received[0], event, "the event reaches the registered part unmodified");
	result = undefined;
	assert.equal(header.handleMouse(event), undefined, "an ignored click returns undefined, same as any other component");
});

test("the header component's handleMouse is a harmless no-op when no header part is registered", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	assert.equal(f.root[NODE]().type, "hstack", "no header part means no header leaf to click at all");
});

// ── rail content: branding, ordering, visibility ────────────────────────────

test("rail orders unified Status, agents and TODO and never renders a standalone changes part", (t) => {
	const f = fixture();
	for (const key of ["todo", "agents", "changes"]) {
		sidebarPart(f.tui, key, { render: () => [key, ""], invalidate() {} });
	}
	t.after(installSidebar(f.tui, theme));
	assert.deepEqual(rail(f).render(50).map((line) => line.trim()), [BANNER, "", "Status", "", "agents", "", "todo"]);
});

test("branding belongs to scroll content before Status, never transcript or narrow bottom", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	const scroll = rail(f);
	const lines = scroll.render(50);
	const brandIndex = lines.findIndex((line) => line.includes(BANNER));
	const statusIndex = lines.findIndex((line) => line.includes("Status"));
	assert.ok(brandIndex >= 0 && brandIndex < statusIndex);
	assert.deepEqual(f.root.render(), ["transcript"]);
	const usableWidth = scroll.getContentWidth(50) - 2;
	const spare = usableWidth - visibleWidth(BANNER);
	const scrollbarWidth = 50 - scroll.getContentWidth(50);
	assert.equal(lines[brandIndex], " ".repeat(1 + Math.floor(spare / 2)) + BANNER + " ".repeat(1 + Math.ceil(spare / 2) + scrollbarWidth));
	scroll.updateLayout(lines.length, 2, () => {});
	scroll.scrollBy(9);
	assert.ok(scroll.scrollTop > 0, "the rail scrolls when its content overflows the viewport");
	f.host.terminal.columns = 80;
	assert.equal(f.root[NODE]().type, "vstack");
	assert.deepEqual(f.bottom.render(80), ["Status"]);
});

test("TODO visibility hides both rail and bottom without mutating the registered part", (t) => {
	const f = fixture();
	const todo = sidebarPart(f.tui, "todo", { render: (_width) => ["todo"], invalidate() {} });
	t.after(installSidebar(f.tui, theme));
	assert.match(rail(f).render(50).join("\n"), /todo/);
	sidebarState(f.tui).visibility = { todo: false };
	invalidateSidebar(f.tui);
	assert.doesNotMatch(rail(f).render(50).join("\n"), /todo/);
	f.host.terminal.columns = 100;
	assert.equal(f.root[NODE]().type, "vstack");
	assert.deepEqual(todo.render(100), []);
	sidebarState(f.tui).visibility = { todo: true };
	assert.deepEqual(todo.render(100), ["todo"]);
});

test("the rail wiring paints through the deployed Status card and carries no review facts (D10)", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	const wiring = fs.readFileSync(WIRING_PATH, "utf8");
	assert.match(wiring, /renderShellSidebarBar\(/, "the rail must reuse the deployed Status card renderer");
	for (const file of [MODULE_PATH, WIRING_PATH]) {
		const banned = /\b(lineage|lens|budget)\b|REVIEW_SIDEBAR|model\.review/;
		const source = fs.readFileSync(file, "utf8");
		for (const [index, line] of source.split("\n").entries()) {
			const trimmed = line.trim();
			if (trimmed.startsWith("//") || trimmed.startsWith("*") || trimmed.startsWith("/*")) continue;
			assert.ok(!banned.test(line), `${path.basename(file)}:${index + 1} leaks a review fact into the rail: ${trimmed}`);
		}
	}
});

// ── scrolling and mouse dispatch ────────────────────────────────────────────

test("wheel scrolls the rail and is consumed at both boundaries and blank space", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	const scroll = rail(f);
	scroll.updateLayout(20, 5, () => {});
	for (const [delta, expected] of [[-1, 0], [3, 3], [100, 15], [1, 15], [-100, 0]]) {
		const result = scroll.handleMouse({ type: "wheel", wheelDelta: delta, x: 3, y: 4, screenX: 93, screenY: 6, width: 50, height: 5 });
		assert.equal(result?.handled, true);
		assert.equal(scroll.scrollTop, expected);
		assert.equal(result?.target?.component, scroll);
	}
	f.host.terminal.columns = 139;
	assert.equal(f.root[NODE]().type, "vstack");
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	f.host.terminal.columns = 160;
	const restored = rail(f);
	assert.deepEqual(f.bottom.render(80), []);
	const layout = f.root[NODE]();
	assert.equal(layout.gap, 3);
	assert.deepEqual(layout.entries.map(({ basis, grow, shrink, minSize }) => ({ basis, grow, shrink, minSize })), [
		{ basis: 0, grow: 1, shrink: 1, minSize: 1 },
		{ basis: 50, grow: 0, shrink: 0, minSize: 50 },
	]);
	const restoredLines = restored.render(50);
	for (const line of restoredLines) assert.ok(visibleWidth(line) <= 50);
	assert.ok(restoredLines.some((line) => line.startsWith(" Status ")));
	assert.deepEqual(f.root.render(), ["transcript"]);
	restored.updateLayout(20, 5, () => {});
	const before = restored.scrollTop;
	const result = restored.handleMouse({ type: "wheel", wheelDelta: 2, x: 3, y: 4, screenX: 113, screenY: 6, width: 50, height: 5 });
	assert.equal(result?.handled, true);
	assert.equal(result?.target?.component, restored);
	assert.equal(restored.scrollTop, Math.min(15, before + 2));
	for (const line of restored.render(50)) assert.ok(visibleWidth(line) <= 50);
	assert.deepEqual(f.root.render(), ["transcript"]);
	scroll.updateLayout(1, 5, () => {});
	assert.equal(scroll.handleMouse({ type: "wheel", wheelDelta: 1 })?.handled, true);
	assert.equal(scroll.scrollTop, 0);
});

test("rail dispatches a clipped, scroll-translated left click to only the matching sidebar part", (t) => {
	const f = fixture();
	let clicks = 0;
	const received = [];
	const todo = {
		render: () => ["Todo header", "Todo body"],
		invalidate() {},
		handleMouse(event) {
			received.push(event);
			if (event.type !== "click" || event.button !== "left" || event.y !== 0) return undefined;
			clicks++;
			return { handled: true, render: true };
		},
	};
	sidebarPart(f.tui, "todo", todo);
	const dispose = installSidebar(f.tui, theme);
	t.after(dispose);
	const scroll = rail(f);
	const content = scroll.render(50);
	scroll.updateLayout(content.length, 5, () => {});
	scroll.scrollBy(1);
	const headerY = content.findIndex((line) => line.includes("Todo header")) - scroll.scrollTop;
	assert.ok(headerY >= 0);
	const mouse = (type, button, y) => ({ type, button, x: 2, y, screenX: 92, screenY: 30 + y, width: 50, height: 5, shift: false, alt: false, ctrl: false });

	assert.equal(scroll.handleMouse(mouse("press", "left", headerY)), undefined);
	assert.equal(scroll.handleMouse(mouse("click", "right", headerY)), undefined);
	assert.equal(scroll.handleMouse(mouse("click", "left", headerY + 1)), undefined);
	assert.equal(clicks, 0);
	const hit = scroll.handleMouse(mouse("click", "left", headerY));
	assert.equal(hit?.handled, true);
	assert.equal(clicks, 1);
	assert.deepEqual(received.at(-1) && { x: received.at(-1).x, y: received.at(-1).y, width: received.at(-1).width, height: received.at(-1).height }, { x: 1, y: 0, width: 48, height: 2 });

	assert.equal(scroll.handleMouse(mouse("click", "left", headerY - 1)), undefined, "section gaps do not hit a neighbor");
	assert.equal(scroll.handleMouse({ ...mouse("click", "left", headerY), x: 49 }), undefined, "padding outside the clipped part is inert");
	invalidateSidebar(f.tui);
	assert.equal(scroll.handleMouse(mouse("click", "left", headerY)), undefined, "stale geometry is inert");
	f.host.terminal.columns = 139;
	assert.equal(scroll.handleMouse(mouse("click", "left", headerY)), undefined, "resized-away rails are inert");
	dispose();
	assert.equal(scroll.handleMouse(mouse("click", "left", headerY)), undefined, "disposed rails are inert");
});

test("rail rejects removed or replaced parts before cached geometry is prepared again", (t) => {
	const f = fixture();
	let originalClicks = 0;
	let replacementClicks = 0;
	const original = {
		render: () => ["Todo header"],
		invalidate() {},
		handleMouse(event) {
			if (event.type !== "click" || event.button !== "left") return undefined;
			originalClicks++;
			return { handled: true };
		},
	};
	const mounted = sidebarPart(f.tui, "todo", { render: () => ["Todo bottom"], invalidate() {} }, original);
	const dispose = installSidebar(f.tui, theme);
	t.after(dispose);
	const scroll = rail(f);
	const content = scroll.render(50);
	scroll.updateLayout(content.length, 5, () => {});
	const headerY = content.findIndex((line) => line.includes("Todo header"));
	const click = () => ({ type: "click", button: "left", x: 2, y: headerY, screenX: 92, screenY: 30 + headerY, width: 50, height: 5, shift: false, alt: false, ctrl: false });
	assert.equal(scroll.handleMouse(click())?.handled, true, "healthy cached geometry still dispatches");
	assert.equal(originalClicks, 1);

	mounted.dispose();
	assert.equal(scroll.handleMouse(click()), undefined, "removed parts are inert before the next prepare");
	assert.equal(originalClicks, 1);

	const replacement = {
		render: () => ["Todo replacement"],
		invalidate() {},
		handleMouse(event) {
			if (event.type !== "click" || event.button !== "left") return undefined;
			replacementClicks++;
			return { handled: true };
		},
	};
	sidebarPart(f.tui, "todo", { render: () => ["Todo bottom"], invalidate() {} }, replacement);
	assert.equal(scroll.handleMouse(click()), undefined, "a replacement cannot receive stale cached geometry");
	assert.equal(originalClicks, 1);
	assert.equal(replacementClicks, 0);

	rail(f);
	assert.equal(scroll.handleMouse(click())?.handled, true, "prepared replacement dispatches normally");
	assert.equal(replacementClicks, 1);
});

test("rail mouse dispatch still maps clicks correctly with the header row above it", (t) => {
	const f = fixture();
	let clicks = 0;
	const todo = {
		render: () => ["Todo header", "Todo body"],
		invalidate() {},
		handleMouse(event) {
			if (event.type !== "click" || event.button !== "left" || event.y !== 0) return undefined;
			clicks++;
			return { handled: true, render: true };
		},
	};
	sidebarPart(f.tui, "todo", todo);
	sidebarHeader(f.tui, { render: () => ["HEADER"], invalidate() {} });
	const dispose = installSidebar(f.tui, theme);
	t.after(dispose);
	const scroll = railWithHeader(f);
	const content = scroll.render(50);
	scroll.updateLayout(content.length, 5, () => {});
	const headerY = content.findIndex((line) => line.includes("Todo header"));
	assert.ok(headerY >= 0);
	const hit = scroll.handleMouse({ type: "click", button: "left", x: 2, y: headerY, screenX: 92, screenY: 31 + headerY, width: 50, height: 5, shift: false, alt: false, ctrl: false });
	assert.equal(hit?.handled, true);
	assert.equal(clicks, 1);
});

// ── memoization: whole-rail and per-section ─────────────────────────────────

test("reuses final sidebar presentation until a relevant invalidation", (t) => {
	const f = fixture();
	sidebarPart(f.tui, "todo", { render: () => Array.from({ length: 10 }, (_, index) => `Todo ${index}`), invalidate() {} });
	t.after(installSidebar(f.tui, theme));

	const first = f.root[NODE]();
	assert.equal(f.root[NODE](), first);
	const scroll = first.entries[1].component;
	scroll.updateLayout(scroll.render(50).length, 1, () => {});
	scroll.scrollBy(1);
	const scrolled = f.root[NODE]();
	assert.notEqual(scrolled, first);
	assert.equal(f.root[NODE](), scrolled);

	scroll.invalidate();
	const invalidated = f.root[NODE]();
	assert.notEqual(invalidated, scrolled);
	assert.equal(f.root[NODE](), invalidated);

	f.host.terminal.columns = 141;
	const resized = f.root[NODE]();
	assert.notEqual(resized, invalidated);
	assert.equal(f.root[NODE](), resized);
	f.host.mode = "regular";
	assert.equal(f.root[NODE]().type, "vstack");
	assert.deepEqual(f.bottom.render(80), ["Status"]);
});

test("a footer digest change does not re-render Agents or TODO", (t) => {
	const f = fixture();
	let footerLabel = "one";
	const counts = { footer: 0, agents: 0, todo: 0 };
	sidebarPart(f.tui, "footer", { render: () => ["Status"], invalidate() {} }, {
		digest: () => footerLabel,
		render: () => { counts.footer++; return [`Footer ${footerLabel}`]; },
		invalidate() {},
	});
	for (const key of ["agents", "todo"]) {
		sidebarPart(f.tui, key, { render: () => [key], invalidate() {} }, {
			render: () => { counts[key]++; return [key]; },
			invalidate() {},
		});
	}
	t.after(installSidebar(f.tui, theme));

	assert.match(rail(f).render(50).join("\n"), /Footer one/);
	assert.deepEqual(counts, { footer: 1, agents: 1, todo: 1 });

	assert.match(rail(f).render(50).join("\n"), /Footer one/);
	assert.deepEqual(counts, { footer: 1, agents: 1, todo: 1 }, "an unchanged frame reuses every section");

	footerLabel = "two";
	assert.match(rail(f).render(50).join("\n"), /Footer two/);
	assert.deepEqual(counts, { footer: 2, agents: 1, todo: 1 }, "only the section whose digest changed re-renders");

	invalidateSidebar(f.tui);
	assert.match(rail(f).render(50).join("\n"), /Footer two/);
	assert.deepEqual(counts, { footer: 2, agents: 2, todo: 2 }, "revision invalidation reaches only the sections with no digest of their own");
});

test("a throwing digest still degrades to invalidation-only under the per-section memo", (t) => {
	const f = fixture();
	let todoRenders = 0;
	sidebarPart(f.tui, "todo", { render: () => ["Todo"], invalidate() {} }, {
		digest: () => { throw new Error("broken digest"); },
		render: () => { todoRenders++; return ["Todo card"]; },
		invalidate() {},
	});
	t.after(installSidebar(f.tui, theme));

	assert.match(rail(f).render(50).join("\n"), /Todo card/);
	assert.equal(todoRenders, 1);
	assert.match(rail(f).render(50).join("\n"), /Todo card/);
	assert.equal(todoRenders, 1, "an unchanged frame reuses the cached section even without a digest");

	invalidateSidebar(f.tui);
	assert.match(rail(f).render(50).join("\n"), /Todo card/);
	assert.equal(todoRenders, 2, "explicit invalidation still reaches a rail without a digest");
});

test("a part replaced under the same key never reuses the previous part's cached lines", (t) => {
	const f = fixture();
	let firstRenders = 0;
	sidebarPart(f.tui, "todo", { render: () => ["Todo"], invalidate() {} }, {
		render: () => { firstRenders++; return ["first"]; },
		invalidate() {},
	});
	t.after(installSidebar(f.tui, theme));
	assert.match(rail(f).render(50).join("\n"), /first/);
	assert.equal(firstRenders, 1);

	let secondRenders = 0;
	sidebarPart(f.tui, "todo", { render: () => ["Todo"], invalidate() {} }, {
		render: () => { secondRenders++; return ["second"]; },
		invalidate() {},
	});
	const text = rail(f).render(50).join("\n");
	assert.match(text, /second/);
	assert.doesNotMatch(text, /first/);
	assert.equal(secondRenders, 1);
});

test("section memo survives breakpoint and mode dips; a replaced part re-renders", (t) => {
	const f = fixture();
	const counts = { footer: 0, agents: 0, todo: 0 };
	let todo = "Todo one";
	for (const key of ["footer", "agents", "todo"]) {
		sidebarPart(f.tui, key, { render: () => ["bottom"], invalidate() {} }, {
			render: () => { counts[key]++; return [key === "todo" ? todo : key]; },
			invalidate() {},
		});
	}
	t.after(installSidebar(f.tui, theme));
	const scroll = rail(f);
	scroll.render(50);
	assert.deepEqual(counts, { footer: 1, agents: 1, todo: 1 });
	rail(f).render(50);
	assert.deepEqual(counts, { footer: 1, agents: 1, todo: 1 });

	todo = "Todo two";
	scroll.invalidate();
	assert.match(rail(f).render(50).join("\n"), /Todo two/);
	assert.deepEqual(counts, { footer: 2, agents: 2, todo: 2 });

	f.host.terminal.columns = 139;
	assert.equal(f.root[NODE]().type, "vstack");
	f.host.terminal.columns = 140;
	rail(f).render(50);
	assert.deepEqual(counts, { footer: 2, agents: 2, todo: 2 }, "the per-section cache survives a breakpoint dip");

	f.host.mode = "regular";
	f.root[NODE]();
	f.host.mode = "fullscreen";
	rail(f).render(50);
	assert.deepEqual(counts, { footer: 2, agents: 2, todo: 2 }, "the cache survives a mode dip");

	let replacementRenders = 0;
	sidebarPart(f.tui, "todo", { render: () => ["bottom"], invalidate() {} }, {
		render: () => { replacementRenders++; return ["Todo replacement"]; },
		invalidate() {},
	});
	assert.match(rail(f).render(50).join("\n"), /Todo replacement/);
	assert.equal(replacementRenders, 1);
});

// ── scrollbar, cleanup and degenerate roots ─────────────────────────────────

test("rail scrollbar is transient: hidden by default and visible only while scrolling overflow", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	const scroll = rail(f);
	assert.equal(scroll.scrollbar, "auto", "the rail keeps a transient scrollbar, never the always-on geometry");
	sidebarPart(f.tui, "todo", { render: () => Array.from({ length: 60 }, (_, index) => `todo row ${index}`), invalidate() {} });
	rail(f).render(50);
	scroll.updateLayout(60, 20, () => {});
	assert.equal(scroll.isScrollbarVisible, false, "no scroll yet: hidden even with overflow");
	scroll.scrollBy(1);
	assert.equal(scroll.isScrollbarVisible, true, "rail scroll reveals the transient scrollbar");
	scroll.hideTransientScrollbar();
	assert.equal(scroll.isScrollbarVisible, false, "hidden again after the transient window");
});

test("cleanup restores the native layout and bottom paint without disposing widgets", (t) => {
	const f = fixture();
	const dispose = installSidebar(f.tui, theme);
	rail(f);
	dispose();
	assert.equal(f.root[NODE], f.original);
	assert.deepEqual(f.bottom.render(80), ["Status"]);
	assert.equal(sidebarState(f.tui).parts.size, 1);
	assert.ok(f.renders() >= 2);
});

test("unsupported roots, empty rails and overflowing parts leave native layout intact", (t) => {
	for (const lines of [[], ["x".repeat(100)]]) {
		const f = fixture();
		sidebarPart(f.tui, "footer", { render: () => lines, invalidate() {} });
		t.after(installSidebar(f.tui, theme));
		assert.equal(f.root[NODE]().type, "vstack");
		assert.equal(sidebarState(f.tui).active, false);
	}
	const f = fixture();
	Reflect.deleteProperty(f.root, NODE);
	t.after(installSidebar(f.tui, theme));
	assert.deepEqual(f.bottom.render(80), ["Status"]);
});

test("the rail cache symbol is renamed off upstream's brand key", (t) => {
	const f = fixture();
	t.after(installSidebar(f.tui, theme));
	rail(f).render(50);
	const symbols = Object.getOwnPropertySymbols(f.tui.terminal).map(String);
	assert.ok(symbols.includes(String(CACHE)), "the cache must use the biggz-ai symbol");
	assert.ok(!symbols.some((symbol) => symbol.includes("gentle")), "no upstream brand key may be attached to the terminal");
});

// ── R3-002 proof: the detector's reach, by execution ────────────────────────
//
// The detector above decides when a pi-tui shape change is a hard failure. This
// test builds two throwaway candidate trees and re-runs this same file in a
// subprocess, so the rule is exercised end to end instead of trusted:
//   (a) only a doctored install exists (package name + main, every shell symbol
//       but ScrollView/VStack) -> nothing resolves and the detector must FAIL,
//       naming the directory and the missing symbols;
//   (b) a stale install sits beside a usable one -> the detector must PASS,
//       because resolvePiTuiEntry deliberately skipped the stale sibling.
// A subprocess is required: installPiTuiResolver() is idempotent, so the
// @earendil-works/pi-tui target cannot change inside one process. The child is
// marked with BIGGZ_D12_PROOF_CHILD so this test skips there and cannot recurse.
const RAIL_SUITE = fileURLToPath(import.meta.url);
const ORACLE_PATH = path.join(path.dirname(RAIL_SUITE), "..", "test", "pi-tui-oracle.mjs");
// The shell symbols except ScrollView/VStack, so a doctored candidate is
// rejected for exactly the two rail shapes rather than for a short list.
const SHELL_SYMBOLS_WITHOUT_LAYOUT = ["truncateToWidth", "visibleWidth", "wrapTextWithAnsi", "Key", "matchesKey"];

function candidateDir(root, relative) {
	const dir = path.join(root, relative);
	fs.mkdirSync(path.join(dir, "dist"), { recursive: true });
	fs.writeFileSync(path.join(dir, "package.json"), JSON.stringify({ name: PI_TUI_SPECIFIER, main: "dist/index.js" }));
	return dir;
}

function writeDoctoredCandidate(dir) {
	const body = SHELL_SYMBOLS_WITHOUT_LAYOUT.map((name) => `export const ${name} = 1;`).join("\n");
	fs.writeFileSync(path.join(dir, "dist", "index.js"), `${body}\n`);
}

function writeUsableCandidate(dir) {
	const names = [...SHELL_SYMBOLS_WITHOUT_LAYOUT, "ScrollView", "VStack"];
	fs.writeFileSync(path.join(dir, "dist", "index.js"), `export { ${names.join(", ")} } from ${JSON.stringify(pathToFileURL(ORACLE_PATH).href)};\n`);
}

function runRailSuite({ appData, home }) {
	// Strip the parent test runner's context: a nested `node --test` that inherits
	// NODE_TEST_CONTEXT refuses to run files, which would make this proof vacuous.
	const env = { ...process.env, APPDATA: appData, USERPROFILE: home, HOME: home, PI_TUI_DIR: "", BIGGZ_D12_PROOF_CHILD: "1" };
	delete env.NODE_TEST_CONTEXT;
	return spawnSync(process.execPath, ["--test", RAIL_SUITE], { encoding: "utf8", env });
}

test("the detector is loud only when nothing resolved (R3-002 proof)", (t) => {
	if (process.env.BIGGZ_D12_PROOF_CHILD === "1") {
		t.skip("child run of the R3-002 proof");
		return;
	}
	const appData = fs.mkdtempSync(path.join(os.tmpdir(), "biggz-d12-appdata-"));
	const home = fs.mkdtempSync(path.join(os.tmpdir(), "biggz-d12-home-"));
	t.after(() => {
		fs.rmSync(appData, { recursive: true, force: true });
		fs.rmSync(home, { recursive: true, force: true });
	});

	// (a) one doctored install and nothing else: no candidate resolves, so the
	// suite must fail rather than silently degrade to the oracle.
	const doctored = candidateDir(appData, path.join("npm", "node_modules", "@earendil-works", "pi-coding-agent", "node_modules", "@earendil-works", "pi-tui"));
	writeDoctoredCandidate(doctored);
	const failing = runRailSuite({ appData, home });
	const failingOutput = `${failing.stdout}\n${failing.stderr}`;
	assert.notEqual(failing.status, 0, `the detector must fail when no pi-tui resolves and a candidate lost ScrollView/VStack\n${failingOutput}`);
	assert.match(failingOutput, /loses a required shell symbol/, "the failing child must actually have run the detector");
	assert.match(failingOutput, /ScrollView/, "the failure must name a missing symbol");
	assert.ok(failingOutput.includes(doctored), "the failure must name the rejected directory");

	// (b) the same stale candidate beside a usable one: the resolver skips the
	// stale sibling, so both the shape checks and the detector must pass.
	const usable = candidateDir(home, path.join(".pi", "agent", "npm", "node_modules", "@earendil-works", "pi-tui"));
	writeUsableCandidate(usable);
	const passing = runRailSuite({ appData, home });
	assert.equal(passing.status, 0, `a stale candidate beside a usable one must not fail the suite\n${passing.stdout}\n${passing.stderr}`);
	assert.match(passing.stdout, /✔ a discovered pi-tui that loses a required shell symbol/, `the passing child must actually have run the detector\n${passing.stdout}\n${passing.stderr}`);
	const failureLine = failingOutput.split("\n").find((line) => line.includes("no longer names every shell symbol")) ?? "";
	t.diagnostic(`R3-002 (a): nothing resolved; child exit ${failing.status}; ${failureLine.trim()}`);
	t.diagnostic(`R3-002 (b): stale sibling skipped; child exit ${passing.status}; detector passed=${/✔ a discovered pi-tui that loses a required shell symbol/.test(passing.stdout)}`);
});
