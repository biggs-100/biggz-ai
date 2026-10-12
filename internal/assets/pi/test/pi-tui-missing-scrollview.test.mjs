// R3-001 proof fixture (unit 2.15). Run in a subprocess by
// internal/assets/pi/biggz-shell.test.mjs.
//
// It installs a module hook that resolves @earendil-works/pi-tui to a stub which
// exports every runtime symbol the shell's static import graph needs BUT NOT
// ScrollView and VStack, then imports the shell factory and proves:
//   - the stub really cannot satisfy shell-sidebar-layout.ts (the control);
//   - the footer, /biggz-usage and its shortcut still install and the below-
//     editor widget is still registered;
//   - no rail is installed: the layout root is never patched and the sidebar
//     state stays inactive.
//
// A subprocess is required because installPiTuiResolver() is idempotent: once
// the repository resolver has registered its hooks in a process, the
// @earendil-works/pi-tui target cannot change there. A fresh process lets this
// fixture register its own hook before the factory is imported.
import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { registerHooks } from "node:module";
import { pathToFileURL } from "node:url";

// Keep resolveVisualSettings() hermetic: a config home that does not exist
// exercises the default branch instead of a real user config.
process.env.BIGGZ_CONFIG_HOME = path.join(os.tmpdir(), "biggz-missing-scrollview-config-does-not-exist");
process.env.BIGGZ_PRETTY = "1";

const ORACLE = new URL("./pi-tui-oracle.mjs", import.meta.url);
const LAYOUT_NODE = Symbol.for("@earendil-works/pi-tui/layout-node");
const THEME = { fg: (_color, text) => text, bold: (text) => text };

// A stub with every shell symbol except the two the rail binds to, so the rail
// module fails its ESM link while the rest of the shell links.
const stubDir = fs.mkdtempSync(path.join(os.tmpdir(), "biggz-pi-tui-stub-"));
const stubFile = path.join(stubDir, "pi-tui-stub.mjs");
fs.writeFileSync(
	stubFile,
	`export {\n\ttruncateToWidth, visibleWidth, wrapTextWithAnsi, Key, matchesKey,\n} from ${JSON.stringify(ORACLE.href)};\n`,
);
process.on("exit", () => fs.rmSync(stubDir, { recursive: true, force: true }));

registerHooks({
	resolve(specifier, context, nextResolve) {
		if (specifier === "@earendil-works/pi-tui") return { url: pathToFileURL(stubFile).href, shortCircuit: true };
		return nextResolve(specifier, context);
	},
});

const { sidebarState } = await import("../biggz-shell/shell-sidebar.ts");
const { default: biggzShell } = await import("../biggz-shell.ts");

function fakePi() {
	const handlers = new Map();
	const commands = new Map();
	const shortcuts = new Map();
	const pi = {
		events: { on: () => () => {} },
		on(event, handler) {
			handlers.set(event, handler);
		},
		registerCommand(name, options) {
			commands.set(name, options);
		},
		registerShortcut(key, options) {
			shortcuts.set(key, options);
		},
		getThinkingLevel() {
			return "medium";
		},
	};
	return { pi, handlers, commands, shortcuts };
}

function fakeCtx() {
	const uiCalls = [];
	const setWidgetCalls = [];
	let footerFactory;
	const ui = {
		setFooter(factory) {
			uiCalls.push("setFooter");
			footerFactory = factory;
		},
		setWidget(key, widget, options) {
			uiCalls.push("setWidget");
			setWidgetCalls.push([key, widget, options]);
		},
		custom() {
			uiCalls.push("custom");
		},
	};
	const ctx = {
		hasUI: true,
		model: undefined,
		modelRegistry: { isUsingOAuth: () => false, getApiKeyForProvider: async () => undefined },
		sessionManager: { getSessionId: () => "test-session", getCwd: () => "/tmp/x", getSessionName: () => undefined, getEntries: () => [] },
		getContextUsage: () => undefined,
		ui,
	};
	return { ctx, uiCalls, setWidgetCalls, footerFactory: () => footerFactory };
}

const footerData = {
	getGitBranch: () => null,
	getExtensionStatuses: () => new Map(),
	getAvailableProviderCount: () => 0,
	onBranchChange: () => () => {},
};

test("the stub really cannot satisfy the rail module (R3-001 control)", async () => {
	// Pin the rejection to the exact failure this control represents: an ESM
	// link-time SyntaxError naming the missing export, so an unrelated rejection
	// (a syntax error in this fixture, a missing file) cannot pass as the control.
	await assert.rejects(
		() => import("../biggz-shell/shell-sidebar-layout.ts"),
		(error) => {
			assert.ok(error instanceof SyntaxError, `the rail must fail at ESM link time with a SyntaxError, got ${error?.constructor?.name ?? typeof error}: ${error?.message}`);
			assert.match(error.message, /does not provide an export named '(ScrollView|VStack)'/, `the link error must name the missing ScrollView/VStack export: ${error?.message}`);
			return true;
		},
		"the stub has no ScrollView/VStack, so shell-sidebar-layout.ts must fail to link",
	);
});

test("a pi-tui without ScrollView/VStack still installs the shell and no rail (R3-001)", async () => {
	const { pi, handlers, commands, shortcuts } = fakePi();
	biggzShell(pi, {});
	const { ctx, uiCalls, setWidgetCalls, footerFactory } = fakeCtx();
	handlers.get("session_start")({}, ctx);
	assert.equal(typeof footerFactory(), "function", "the footer factory must still be registered");

	// A host shaped to satisfy the rail's activation guard: fullscreen, wide
	// enough, with a NODE-bearing layout root. If the rail installed it would
	// replace root[NODE]; containment means it never does.
	const native = { type: "vstack", entries: [] };
	const root = { render: () => ["transcript"], invalidate() {}, [LAYOUT_NODE]: () => native };
	const tui = { mode: "fullscreen", terminal: { columns: 140 }, layoutRoot: root, requestRender() {} };
	footerFactory()(tui, THEME, footerData);
	// Let the lazy rail import reject and any install attempt settle.
	await new Promise((resolve) => setTimeout(resolve, 50));

	assert.deepEqual(uiCalls.slice().sort(), ["setFooter", "setWidget"], "the footer and the below-editor widget still install");
	assert.equal(setWidgetCalls.length, 1, "exactly one widget registration from the factory");
	assert.ok(commands.has("biggz-usage"), "/biggz-usage still registers");
	assert.ok(shortcuts.has("alt+u"), "the usage shortcut still registers");
	assert.equal(sidebarState(tui).active, false, "the rail state stays inactive without the rail module");
	assert.equal(root[LAYOUT_NODE](), native, "the layout root is never patched without the rail module");
});
