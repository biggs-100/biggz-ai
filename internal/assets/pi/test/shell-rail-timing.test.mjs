// R3-001 async-gap proof (unit 2.15): the lazy rail module resolving on either
// side of a session's lifetime.
//
// biggz-shell.ts imports shell-sidebar-layout.ts lazily, so the footer callback
// can return before that module is available. This suite owns the exact moment
// the module becomes available: a module hook resolves the rail specifier to a
// stub whose top-level await blocks on a test-held promise, and each scenario
// chooses when to resolve it. There are no sleeps: `settle(mode)` releases the
// gate, awaits the stub module's evaluation directly, then crosses one macrotask
// boundary, which drains the factory's `.then(installRail)` continuation (all
// remaining work is a microtask).
//
// Cases:
//   (a) session_shutdown/dispose before the module resolves -> the rail is never
//       installed, even after the module resolves.
//   (b) the footer callback runs before the module resolves -> the rail IS
//       installed exactly once once it resolves, and dispose runs its disposer.
//   (c) the late install throws -> the factory swallows it (D12), so no
//       unhandled rejection escapes and shutdown stays safe.
//
// A subprocess is not needed here (unlike the R3-001/R3-002 proofs): each
// scenario uses its own stub URL (`?mode=...`), so the ESM cache keeps the three
// gated modules distinct inside one process.
import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { registerHooks } from "node:module";
import { pathToFileURL } from "node:url";
import "./pi-tui-resolver.mjs"; // supplies @earendil-works/pi-tui for the shell's other imports

// Keep resolveVisualSettings() hermetic: a config home that does not exist
// exercises the default branch instead of a real user config.
process.env.BIGGZ_CONFIG_HOME = path.join(os.tmpdir(), "biggz-rail-timing-config-does-not-exist");
process.env.BIGGZ_PRETTY = "1";

// The shared control surface the gated stub talks to. `ensureGate` is the one
// source of truth: whether the stub or the test asks first, it is the same
// deferred, so the test can never release a different promise than the stub
// awaits.
const railGates = new Map();
function ensureGate(mode) {
	if (!railGates.has(mode)) {
		let resolve;
		const promise = new Promise((resolver) => {
			resolve = resolver;
		});
		railGates.set(mode, { promise, resolve });
	}
	return railGates.get(mode);
}
globalThis.__biggzRailStates = {};
globalThis.__biggzRailThrowModes = new Set();
globalThis.__biggzRailGateFor = (mode) => ensureGate(mode).promise;

const stubDir = fs.mkdtempSync(path.join(os.tmpdir(), "biggz-rail-timing-"));
const stubFile = path.join(stubDir, "rail-stub.mjs");
fs.writeFileSync(
	stubFile,
	`const mode = new URL(import.meta.url).searchParams.get("mode");\n` +
		`const state = { mode, installCalls: 0, uninstallCalls: 0, invalidateCalls: 0, loaded: false, args: undefined };\n` +
		`globalThis.__biggzRailStates[mode] = state;\n` +
		`await globalThis.__biggzRailGateFor(mode);\n` +
		`state.loaded = true;\n` +
		`export function installSidebar(...args) {\n` +
		`\tstate.installCalls += 1;\n` +
		`\tstate.args = args;\n` +
		`\tif (globalThis.__biggzRailThrowModes.has(mode)) throw new Error("rail install failed (timing fixture)");\n` +
		`\treturn () => { state.uninstallCalls += 1; };\n` +
		`}\n` +
		`export function invalidateSidebar() { state.invalidateCalls += 1; }\n`,
);
process.on("exit", () => fs.rmSync(stubDir, { recursive: true, force: true }));

function stubUrl(mode) {
	const url = pathToFileURL(stubFile);
	url.searchParams.set("mode", mode);
	return url.href;
}

let currentMode = "a";
registerHooks({
	resolve(specifier, context, nextResolve) {
		if (specifier.includes("shell-sidebar-layout.ts")) return { url: stubUrl(currentMode), shortCircuit: true };
		return nextResolve(specifier, context);
	},
});

const { default: biggzShell } = await import("../biggz-shell.ts");

const THEME = { fg: (_color, text) => text, bold: (text) => text };
const LAYOUT_NODE = Symbol.for("@earendil-works/pi-tui/layout-node");

function fakePi() {
	const handlers = new Map();
	const pi = {
		events: { on: () => () => {} },
		on(event, handler) {
			handlers.set(event, handler);
		},
		registerCommand() {},
		registerShortcut() {},
		getThinkingLevel() {
			return "medium";
		},
	};
	return { pi, handlers };
}

function fakeCtx() {
	let footerFactory;
	const uiCalls = [];
	const ui = {
		setFooter(factory) {
			uiCalls.push("setFooter");
			footerFactory = factory;
		},
		setWidget() {
			uiCalls.push("setWidget");
		},
		custom() {
			uiCalls.push("custom");
		},
	};
	const ctx = {
		hasUI: true,
		model: undefined,
		modelRegistry: { isUsingOAuth: () => false, getApiKeyForProvider: async () => undefined },
		sessionManager: { getCwd: () => "/tmp/x", getSessionName: () => undefined, getEntries: () => [] },
		getContextUsage: () => undefined,
		ui,
	};
	return { ctx, uiCalls, footerFactory: () => footerFactory };
}

const footerData = {
	getGitBranch: () => null,
	getExtensionStatuses: () => new Map(),
	getAvailableProviderCount: () => 0,
	onBranchChange: () => () => {},
};

function fullscreenTui() {
	const native = { type: "vstack", entries: [] };
	const root = { render: () => ["transcript"], invalidate() {}, [LAYOUT_NODE]: () => native };
	return { tui: { mode: "fullscreen", terminal: { columns: 140 }, layoutRoot: root, requestRender() {} }, root, native };
}

// Start one session against a factory instance whose lazy rail import is the
// mode's gated stub. `ensureGate` makes the gate exist before the import can
// start, so there is no window where the stub would create a different one.
function start(mode) {
	currentMode = mode;
	ensureGate(mode);
	const { pi, handlers } = fakePi();
	biggzShell(pi, {});
	const { ctx, footerFactory } = fakeCtx();
	handlers.get("session_start")({}, ctx);
	return { handlers, footerFactory: footerFactory(), state: () => globalThis.__biggzRailStates[mode] };
}

// Release the module the lazy import is waiting on, wait for it to evaluate,
// then drain the factory's continuation. Every step is an explicit signal, not a
// delay: the gate resolves the stub, the dynamic import resolves on evaluation,
// and one macrotask boundary flushes the remaining microtasks.
async function settle(mode) {
	ensureGate(mode).resolve();
	await import(stubUrl(mode));
	await new Promise((resolve) => setImmediate(resolve));
}

test("(a) dispose before the rail module resolves installs nothing, even after it resolves", async () => {
	const session = start("a");
	const { tui, root, native } = fullscreenTui();
	session.footerFactory(tui, THEME, footerData); // starts the lazy import, blocked on the gate
	session.handlers.get("session_shutdown")(); // dispose arrives first
	await settle("a");
	const state = session.state();
	assert.equal(state.loaded, true, "the stub module must have evaluated");
	assert.equal(state.installCalls, 0, "a disposed session must never call installSidebar");
	assert.equal(state.uninstallCalls, 0, "no install means no disposer to run");
	assert.equal(root[LAYOUT_NODE](), native, "the layout root stays untouched");
});

test("(b) a rail module that resolves after the footer callback installs the rail exactly once", async () => {
	const session = start("b");
	const { tui } = fullscreenTui();
	session.footerFactory(tui, THEME, footerData);
	await settle("b");
	const state = session.state();
	assert.equal(state.loaded, true);
	assert.equal(state.installCalls, 1, "the late module installs the rail exactly once");
	assert.equal(state.args[0], tui, "the rail installs against the session's own tui");
	assert.equal(state.args[2](), "auto", "the placement closure reads the live presentation");
	assert.equal(state.args[3](), "top", "the header placement closure reads the live presentation");
	assert.equal(state.args[4](), "comfortable", "the density closure reads the live presentation");
	assert.equal(state.uninstallCalls, 0, "nothing disposed yet, so the disposer has not run");
	session.handlers.get("session_shutdown")();
	assert.equal(state.uninstallCalls, 1, "dispose runs the installed rail's disposer");
	session.handlers.get("session_shutdown")();
	assert.equal(state.uninstallCalls, 1, "session_shutdown stays idempotent");
});

test("(c) a late install that throws is contained and never surfaces as an unhandled rejection", async () => {
	globalThis.__biggzRailThrowModes.add("c");
	const session = start("c");
	const { tui } = fullscreenTui();
	const unhandled = [];
	const onUnhandled = (reason) => unhandled.push(reason);
	process.on("unhandledRejection", onUnhandled);
	session.footerFactory(tui, THEME, footerData);
	await settle("c");
	process.off("unhandledRejection", onUnhandled);
	const state = session.state();
	assert.equal(state.installCalls, 1, "the late install was attempted");
	assert.equal(state.uninstallCalls, 0, "the throwing install returned no disposer");
	assert.deepEqual(unhandled, [], "the throw must be swallowed inside the factory (D12)");
	assert.doesNotThrow(() => session.handlers.get("session_shutdown")());
});
