// Ported from gentle-pi 3.7.0 tests/shell-usage-view.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged; the additions
// declared below are biggz-ai.
//
// The ported coverage is upstream's five test cases: the framed panel (empty
// state, then one row per provider after a fetch), r / esc / q, the clickable
// footer hints, the shared hover role, and the rejected-refresh behavior. This
// suite extends it with three cases the upstream file does not assert: one
// render row per usage window with its own reset text, the active provider with
// no data yet showing the provider note from providerNote, and the pending note
// a runtime-registered source supplies through the registry.
// pi-tui is supplied by the repository's test oracle (see test/pi-tui-resolver.mjs).
import assert from "node:assert/strict";
import test from "node:test";
import { PI_TUI_SPECIFIER } from "../test/pi-tui-resolver.mjs";
import { stripAnsi } from "./terminal-theme.ts";

const { visibleWidth } = await import(PI_TUI_SPECIFIER);
const { parseCodexUsage, UsageSourceRegistry, UsageStore, USAGE_SOURCE_SCHEMA } = await import("./shell-usage.ts");
const { UsageView } = await import("./shell-usage-view.ts");

// The subscriptions overlay: one framed panel listing every provider the
// store knows, with r to refetch and esc to close.

const NOW = 1_788_600_000_000;
const plainTheme = {
	fg(_color, text) {
		return text;
	},
};

function payload(percent) {
	return { plan_type: "pro", rate_limit: { primary_window: { used_percent: percent, limit_window_seconds: 604_800, reset_at: NOW / 1000 + 7200 } } };
}

// Two windows on one limit: the panel must render one row per window, each with
// its own reset text, instead of one row per provider.
function twoWindowPayload() {
	return {
		plan_type: "pro",
		rate_limit: {
			primary_window: { used_percent: 40, limit_window_seconds: 604_800, reset_at: NOW / 1000 + 7200 },
			secondary_window: { used_percent: 12, limit_window_seconds: 18_000, reset_after_seconds: 3600 },
		},
	};
}

test("UsageView frames the panel, keeps every line at width, and shows the empty state", () => {
	const store = new UsageStore();
	const events = [];
	const view = new UsageView(store, { theme: plainTheme, now: () => NOW, active: () => undefined, onRefresh: async () => events.push("refresh"), onClose: () => events.push("close"), requestRender: () => events.push("render") });
	const empty = view.render(90).map(stripAnsi);
	assert.match(empty[0], /^╭─ ✿ Subscriptions ─+╮$/);
	assert.match(empty[1], /No subscription usage yet/);
	assert.match(empty[empty.length - 2], /r refresh .* esc close/);
	assert.match(empty[empty.length - 1], /^╰─+╯$/);

	store.record(parseCodexUsage(payload(40), NOW));
	const lines = view.render(90);
	for (const line of lines) assert.equal(visibleWidth(line), 90, `"${stripAnsi(line)}" is not 90 wide`);
	const plain = lines.map(stripAnsi);
	assert.match(plain[1], /^│ openai-codex · pro · updated just now +│$/);
	assert.match(plain[2], /^│ {3}codex week +[▰▱]{16} +40% · resets in 2h 0m +│$/);
	assert.match(plain[3], /r refresh .* esc close/);
});

test("UsageView renders one row per window, each with its own reset text", () => {
	const store = new UsageStore();
	store.record(parseCodexUsage(twoWindowPayload(), NOW));
	const view = new UsageView(store, { theme: plainTheme, now: () => NOW, active: () => undefined, onRefresh: async () => {}, onClose: () => {}, requestRender: () => {} });
	const plain = view.render(90).map(stripAnsi);

	assert.match(plain[2], /^│ {3}codex week +[▰▱]{16} +40% · resets in 2h 0m +│$/, "the primary window is one row");
	assert.match(plain[3], /^│ {3}codex 5h +[▰▱]{16} +12% · resets in 1h 0m +│$/, "the secondary window is its own row with its own reset text");
	assert.match(plain[4], /r refresh .* esc close/, "no extra row is inserted between the window rows and the hints");
});

test("UsageView shows the provider note for an active provider with no usage yet", () => {
	const store = new UsageStore();
	const view = new UsageView(store, { theme: plainTheme, now: () => NOW, active: () => ({ provider: "acme" }), onRefresh: async () => {}, onClose: () => {}, requestRender: () => {} });
	assert.match(stripAnsi(view.render(90)[1]), /^│ ✿ acme · no subscription usage for this provider +│$/, "a provider no source registered for says so instead of rendering nothing");
});

test("UsageView resolves a runtime source's pending note through the registry", () => {
	const store = new UsageStore();
	const registry = new UsageSourceRegistry();
	registry.register({ schema: USAGE_SOURCE_SCHEMA, provider: "acme", pendingNote: "usage arrives with the first response", fetch: async () => undefined });
	const view = new UsageView(store, { theme: plainTheme, now: () => NOW, active: () => ({ provider: "acme" }), registry: () => registry, onRefresh: async () => {}, onClose: () => {}, requestRender: () => {} });
	assert.match(stripAnsi(view.render(90)[1]), /^│ ✿ acme · usage arrives with the first response +│$/, "the registered source's own note replaces the generic one");
});

test("UsageView refetches on r and closes on escape or q", async () => {
	const store = new UsageStore();
	const events = [];
	const view = new UsageView(store, {
		theme: plainTheme,
		now: () => NOW,
		active: () => ({ provider: "openai-codex" }),
		onRefresh: async () => {
			store.record(parseCodexUsage(payload(55), NOW));
			events.push("refresh");
		},
		onClose: () => events.push("close"),
		requestRender: () => events.push("render"),
	});
	view.handleInput("r");
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(events, ["render", "refresh", "render"]);
	assert.match(stripAnsi(view.render(90)[2]), /55%/);
	assert.match(stripAnsi(view.render(90)[1]), /^│ ✿ openai-codex · pro · updated just now/);
	view.handleInput("\x1b");
	view.handleInput("q");
	assert.equal(events.filter((event) => event === "close").length, 2);
});

function click(x, y, width, height) {
	return { type: "click", button: "left", x, y, screenX: x, screenY: y, width, height, shift: false, alt: false, ctrl: false };
}

test("UsageView.handleMouse clicks the footer hints like the matching key", async () => {
	const store = new UsageStore();
	const events = [];
	let resolveRefresh;
	const view = new UsageView(store, {
		theme: plainTheme,
		now: () => NOW,
		active: () => undefined,
		onRefresh: () =>
			new Promise((resolve) => {
				events.push("refresh");
				resolveRefresh = resolve;
			}),
		onClose: () => events.push("close"),
		requestRender: () => events.push("render"),
	});
	const width = 90;
	const lines = view.render(width);
	const rowIndex = lines.findIndex((line) => stripAnsi(line).includes("r refresh"));
	assert.ok(rowIndex > 0, "the footer hints row must be present");
	const plain = stripAnsi(lines[rowIndex]);
	const refreshStart = plain.indexOf("r refresh");
	const closeStart = plain.indexOf("esc close");
	assert.ok(refreshStart >= 0 && closeStart >= 0);

	// A click outside any hint span is ignored, like a click on an empty part of the frame.
	assert.equal(view.handleMouse(click(0, rowIndex, width, lines.length)), undefined);
	assert.deepEqual(events, []);

	// A click on "r refresh" triggers the same refresh as the r key.
	assert.deepEqual(view.handleMouse(click(refreshStart + 1, rowIndex, width, lines.length)), { handled: true, render: true });
	assert.deepEqual(events, ["render", "refresh"]);

	// A second click while the refresh is still in flight must not re-enter it.
	assert.deepEqual(view.handleMouse(click(refreshStart + 1, rowIndex, width, lines.length)), { handled: true, render: true });
	assert.deepEqual(events, ["render", "refresh"]);
	resolveRefresh?.();
	await new Promise((resolve) => setTimeout(resolve, 0));
	assert.deepEqual(events, ["render", "refresh", "render"]);

	// A click on "esc close" closes the overlay, like the esc/q keys.
	assert.deepEqual(view.handleMouse(click(closeStart + 1, rowIndex, width, lines.length)), { handled: true, render: true });
	assert.equal(events.filter((event) => event === "close").length, 1);
});

function move(x, y, width, height) {
	return { type: "move", button: "none", x, y, screenX: x, screenY: y, width, height, shift: false, alt: false, ctrl: false };
}

// H1 (odd/tasks/usage-click-and-changes-attribution.md): the same shared
// hover role every other clickable surface uses.
test("UsageView.handleMouse paints the shared hover role over a footer hint, and clears it on leave", () => {
	const store = new UsageStore();
	const taggedTheme = { fg: (role, text) => `<${role}>${text}</${role}>` };
	const view = new UsageView(store, {
		theme: taggedTheme,
		now: () => NOW,
		active: () => undefined,
		onRefresh: async () => {},
		onClose: () => {},
		requestRender: () => {},
	});
	const width = 90;
	const lines = view.render(width);
	// The frame draws "│ " (2 columns) before the fitted footer content, and
	// hints are laid out as plain "key label" text joined by 3 spaces -- these
	// column offsets are computed purely from that plain text, independent of
	// the theme, so they hold under taggedTheme exactly as under plainTheme.
	const rowIndex = lines.length - 2;
	const refreshStart = 2;
	const closeStart = refreshStart + "r refresh".length + 3;
	const height = lines.length;

	assert.equal(view.handleMouse(move(0, rowIndex, width, height)), undefined, "a move outside any hint is not this component's gesture");
	assert.doesNotMatch(view.render(width).join("\n"), /<warning>/, "idle: nothing painted yet");

	const entered = view.handleMouse(move(refreshStart + 1, rowIndex, width, height));
	assert.deepEqual(entered, { handled: true, render: true });
	assert.match(view.render(width).join("\n"), /<warning>r refresh<\/warning>/, "hovering the refresh hint paints it, key and label together");
	assert.doesNotMatch(view.render(width).join("\n"), new RegExp(`<warning>esc close`), "the other hint stays unpainted");

	// Moving straight to the other hint switches which one is painted.
	const switched = view.handleMouse(move(closeStart + 1, rowIndex, width, height));
	assert.deepEqual(switched, { handled: true, render: true });
	assert.match(view.render(width).join("\n"), /<warning>esc close<\/warning>/);
	assert.doesNotMatch(view.render(width).join("\n"), /<warning>r refresh/);

	const left = view.handleMouse(move(0, rowIndex, width, height));
	assert.deepEqual(left, { handled: true, render: true }, "leaving the last hovered hint still requests a repaint");
	assert.doesNotMatch(view.render(width).join("\n"), /<warning>/, "nothing stays painted once the pointer leaves");
});

test("a rejected refresh from a click or key never escapes as an unhandled rejection", async () => {
	const store = new UsageStore();
	const events = [];
	const unhandled = [];
	const onUnhandled = (reason) => unhandled.push(reason);
	process.on("unhandledRejection", onUnhandled);
	try {
		const view = new UsageView(store, { theme: plainTheme, now: () => NOW, active: () => undefined, onRefresh: async () => { throw new Error("provider down"); }, onClose: () => events.push("close"), requestRender: () => events.push("render") });
		const lines = view.render(80);
		const row = lines.length - 2;
		const column = lines[row].indexOf("r refresh") + 1;
		view.handleMouse({ type: "click", button: "left", x: column, y: row, width: 80, height: lines.length, screenX: column, screenY: row, shift: false, alt: false, ctrl: false });
		await new Promise((resolve) => setImmediate(resolve));
		view.handleInput("r");
		await new Promise((resolve) => setImmediate(resolve));
		await new Promise((resolve) => setImmediate(resolve));
		assert.deepEqual(unhandled, [], "a failing usage fetch is reported by the panel, never thrown at the process");
		assert.match(view.render(80).join("\n"), /r refresh/, "the refresh hint is back after the failure");
	} finally {
		process.off("unhandledRejection", onUnhandled);
	}
});
