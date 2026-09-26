// biggz-ai shell factory guard (units 2.4 and 2.5, not ported from upstream).
//
// The shell extension owns pi's footer slot: it is the one deployed extension
// that calls ctx.ui.setFooter, it installs the ported shell bar, and it paints
// exactly one line. This suite pins that contract — the model the footer builds,
// the two kill switches that gate installation, the single-line render at the
// widths that matter, and (unit 2.5) the rest of the UI surface the factory
// claims: the below-editor header widget, the /biggz-usage command and its
// optional shortcut, and the fact that it still claims no header slot and no
// editor.
//
// pi-tui is supplied by the repository's test oracle (see test/pi-tui-resolver.mjs).
import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { PI_TUI_SPECIFIER } from "./test/pi-tui-resolver.mjs";

// Keep the visual-settings read hermetic: resolveVisualSettings() reads
// <config home>/visual-customization.json, and the suite must not depend on a
// real user config. A path that does not exist exercises the default branch.
process.env.BIGGZ_CONFIG_HOME = join(tmpdir(), "biggz-shell-test-config-does-not-exist");

const { visibleWidth } = await import(PI_TUI_SPECIFIER);
const { buildShellBarModel, createShellBarComponent, default: biggzShell } = await import("./biggz-shell.ts");
const { DEFAULT_VISUAL_SETTINGS, HEADER_PLACEMENT, VISUAL_SCHEMA } = await import("./biggz-shell/visual-customization-policy.ts");

const TAGGED_THEME = {
	fg(color, text) {
		return `<${color}>${text}</${color}>`;
	},
	bold(text) {
		return text;
	},
};

// A pi stub that records every method it is asked for, so a test can prove the
// factory touched nothing else. Commands, shortcuts and event subscriptions are
// captured by name so a test can assert the registration surface directly.
function fakePi({ thinkingLevel = "medium" } = {}) {
	const calls = [];
	const handlers = new Map();
	const commands = new Map();
	const shortcuts = new Map();
	const eventHandlers = new Map();
	const events = {
		on(event, handler) {
			calls.push(`events.on:${event}`);
			eventHandlers.set(event, handler);
			return () => eventHandlers.delete(event);
		},
	};
	const pi = new Proxy(
		{ events },
		{
			get(_target, prop) {
				if (prop === "events") return events;
				if (prop === "getThinkingLevel") {
					return () => {
						calls.push("getThinkingLevel");
						return thinkingLevel;
					};
				}
				if (prop === "registerCommand") {
					return (name, options) => {
						calls.push("registerCommand");
						commands.set(name, options);
					};
				}
				if (prop === "registerShortcut") {
					return (key, options) => {
						calls.push("registerShortcut");
						shortcuts.set(key, options);
					};
				}
				return (...args) => {
					calls.push(prop);
					if (prop === "on") handlers.set(args[0], args[1]);
					return undefined;
				};
			},
		},
	);
	return { pi, calls, handlers, commands, shortcuts, eventHandlers };
}

// A ReadonlyFooterDataProvider stub: records branch-change subscribers so a
// test can prove dispose() unsubscribed, and can fire a branch change.
function fakeFooterData({ branch = "main", statuses = new Map(), availableProviders = 1 } = {}) {
	const subscribers = new Set();
	const unsubscribed = [];
	return {
		data: {
			getGitBranch: () => branch,
			getExtensionStatuses: () => statuses,
			getAvailableProviderCount: () => availableProviders,
			onBranchChange: (callback) => {
				subscribers.add(callback);
				return () => {
					subscribers.delete(callback);
					unsubscribed.push(callback);
				};
			},
		},
		subscribers,
		unsubscribed,
		emitBranchChange() {
			for (const callback of [...subscribers]) callback();
		},
	};
}

// A ctx stub whose ui is a recording proxy: every UI call the factory makes is
// captured, with its arguments, so "registered the footer and the below-editor
// widget and nothing else" is an assertion rather than a hope.
function fakeCtx({
	hasUI = true,
	cwd = "/home/tester/project",
	sessionName = undefined,
	entries = [],
	usage = undefined,
	model = undefined,
	oauth = false,
	apiKey = undefined,
} = {}) {
	const uiCalls = [];
	const setFooterCalls = [];
	const setWidgetCalls = [];
	const customCalls = [];
	let footerFactory;
	const ui = new Proxy(
		{},
		{
			get(_target, prop) {
				return (...args) => {
					uiCalls.push(prop);
					if (prop === "setFooter") {
						footerFactory = args[0];
						setFooterCalls.push(args);
					}
					if (prop === "setWidget") setWidgetCalls.push(args);
					if (prop === "custom") customCalls.push(args);
					return undefined;
				};
			},
		},
	);
	const ctx = {
		hasUI,
		mode: "tui",
		cwd,
		model,
		modelRegistry: {
			isUsingOAuth: () => oauth,
			getApiKeyForProvider: async () => apiKey,
		},
		sessionManager: {
			getCwd: () => cwd,
			getSessionName: () => sessionName,
			getEntries: () => entries,
		},
		getContextUsage: () => usage,
		ui,
	};
	return { ctx, uiCalls, setFooterCalls, setWidgetCalls, customCalls, footerFactory: () => footerFactory };
}

// The model builder needs only these context fields; a plain object is a truer
// stub than a full ExtensionContext would be.
function modelCtx({
	hasUI = true,
	cwd = "/home/tester/project",
	sessionName = undefined,
	entries = [],
	usage = undefined,
	model = undefined,
	oauth = false,
	...rest
} = {}) {
	return {
		hasUI,
		mode: "tui",
		cwd,
		model,
		modelRegistry: { isUsingOAuth: () => oauth },
		sessionManager: {
			getCwd: () => cwd,
			getSessionName: () => sessionName,
			getEntries: () => entries,
		},
		getContextUsage: () => usage,
		...rest,
	};
}

function withEnv(vars, fn) {
	const saved = new Map();
	for (const [key, value] of Object.entries(vars)) {
		saved.set(key, process.env[key]);
		if (value === undefined) delete process.env[key];
		else process.env[key] = value;
	}
	try {
		return fn();
	} finally {
		for (const [key, value] of saved) {
			if (value === undefined) delete process.env[key];
			else process.env[key] = value;
		}
	}
}

// Run one session_start and return the recorded UI proxy plus the registered
// footer factory. The overrides are injected into both the extension factory
// (usageShortcut/shellEnabled read the passed env) and process.env (the repo-wide
// isPrettyEnabled() kill switch reads process.env). With a kill switch on the
// factory returns before registering anything, so there is no session_start
// handler to run; that is itself the asserted behaviour.
function startSession(env = {}) {
	return withEnv({ BIGGZ_PRETTY: undefined, BIGGZ_SHELL: undefined, PI_SUBAGENT_CHILD: undefined, BIGGZ_SHELL_USAGE_KEY: undefined, ...env }, () => {
		const { pi, calls, handlers, commands, shortcuts, eventHandlers } = fakePi();
		biggzShell(pi, env);
		const { ctx, uiCalls, setFooterCalls, setWidgetCalls, customCalls, footerFactory } = fakeCtx();
		handlers.get("session_start")?.({}, ctx);
		return { pi, calls, handlers, commands, shortcuts, eventHandlers, ctx, uiCalls, setFooterCalls, setWidgetCalls, customCalls, footerFactory };
	});
}

test("buildShellBarModel maps footer, session, model and usage fields", () => {
	const { pi, calls } = fakePi({ thinkingLevel: "high" });
	const footerData = fakeFooterData({
		branch: "feat/x",
		statuses: new Map([
			["b", "second"],
			["a", "first"],
		]),
	});
	const ctx = modelCtx({
		cwd: "/home/tester/work",
		sessionName: "session-7",
		usage: { percent: 42, contextWindow: 200000 },
		model: { id: "claude-sonnet-4", reasoning: true, contextWindow: 100000 },
		oauth: true,
		entries: [
			{ type: "message", message: { role: "assistant", usage: { cost: { total: 0.25 } } } },
			{ type: "message", message: { role: "assistant", usage: { cost: { total: 0.5 } } } },
			{ type: "message", message: { role: "user" } },
			{ type: "tool", message: { role: "assistant", usage: { cost: { total: 99 } } } },
		],
	});
	const model = buildShellBarModel(pi, ctx, footerData.data, {
		home: "/home/tester",
		dirty: 3,
		profile: "work",
		usage: { provider: "openai" },
	});

	assert.equal(model.cwd, "~/work");
	assert.equal(model.branch, "feat/x");
	assert.equal(model.dirty, 3);
	assert.equal(model.sessionName, "session-7");
	assert.equal(model.modelId, "claude-sonnet-4");
	assert.equal(model.effort, "high");
	assert.equal(model.contextPercent, 42);
	assert.equal(model.contextWindow, 200000);
	assert.equal(model.costTotal, 0.75, "cost totals only assistant message usage entries");
	assert.equal(model.subscription, true);
	assert.deepEqual(model.statuses, ["first", "second"], "extension statuses sort by key, not by insertion order");
	assert.equal(model.profile, "work");
	assert.equal(model.usage.provider, "openai");
	assert.deepEqual(calls, ["getThinkingLevel"], "effort is read from pi only for a reasoning model");
});

test("buildShellBarModel falls back when model and usage are absent", () => {
	const { pi, calls } = fakePi();
	const footerData = fakeFooterData();
	const ctx = modelCtx({ cwd: "/home/tester" });
	const model = buildShellBarModel(pi, ctx, footerData.data, { home: "/home/tester" });

	assert.equal(model.cwd, "~", "home prefix shortens to ~");
	assert.equal(model.modelId, "no-model");
	assert.equal(model.effort, undefined);
	assert.equal(model.contextPercent, null);
	assert.equal(model.contextWindow, 0);
	assert.equal(model.costTotal, 0);
	assert.equal(model.subscription, false);
	assert.equal(model.dirty, undefined);
	assert.deepEqual(model.statuses, []);
	assert.deepEqual(calls, [], "no reasoning model means pi.getThinkingLevel() is never called");
});

test("buildShellBarModel keeps the model context window and omits effort without reasoning", () => {
	const { pi } = fakePi({ thinkingLevel: "high" });
	const ctx = modelCtx({
		usage: undefined,
		model: { id: "m", reasoning: false, contextWindow: 128000 },
	});
	const model = buildShellBarModel(pi, ctx, fakeFooterData().data, {});

	assert.equal(model.effort, undefined);
	assert.equal(model.contextWindow, 128000);
	assert.equal(model.subscription, false);
	assert.equal(model.modelId, "m");
});

test("factory installs the footer when both gates pass", () => {
	const { uiCalls, footerFactory } = startSession();
	assert.deepEqual(uiCalls, ["setFooter"]);
	assert.equal(typeof footerFactory(), "function");
});

test("factory installs nothing when BIGGZ_PRETTY=0", () => {
	const { uiCalls, commands, shortcuts, eventHandlers } = startSession({ BIGGZ_PRETTY: "0" });
	assert.deepEqual(uiCalls, [], "the repo-wide pretty kill switch must keep pi's built-in footer");
	// The kill switch is evaluated at factory scope (upstream's early return),
	// so it must pre-empt registerCommand/registerShortcut and the usage-source
	// event subscription too. A switch that only skips session_start still
	// leaves /biggz-usage and the global shortcut installed and reachable.
	assert.equal(commands.size, 0, "BIGGZ_PRETTY=0 must register no command");
	assert.equal(shortcuts.size, 0, "BIGGZ_PRETTY=0 must register no shortcut");
	assert.equal(eventHandlers.size, 0, "BIGGZ_PRETTY=0 must subscribe to no usage-source event");
});

test("factory installs nothing when BIGGZ_SHELL=0", () => {
	const { uiCalls, commands, shortcuts, eventHandlers } = startSession({ BIGGZ_SHELL: "0" });
	assert.deepEqual(uiCalls, [], "BIGGZ_SHELL=0 must claim no UI slot");
	assert.equal(commands.size, 0, "BIGGZ_SHELL=0 must register no command");
	assert.equal(shortcuts.size, 0, "BIGGZ_SHELL=0 must register no shortcut");
	assert.equal(eventHandlers.size, 0, "BIGGZ_SHELL=0 must subscribe to no usage-source event");
});

test("factory installs nothing for a subagent child", () => {
	const { uiCalls } = startSession({ PI_SUBAGENT_CHILD: "1" });
	assert.deepEqual(uiCalls, []);
});

test("factory installs nothing when ctx.hasUI is false", () => {
	const { pi, handlers } = fakePi();
	biggzShell(pi);
	const { ctx, uiCalls } = fakeCtx({ hasUI: false });
	handlers.get("session_start")({}, ctx);
	assert.deepEqual(uiCalls, []);
});

test("factory registers the footer, the below-editor header widget, the usage command and its shortcut, and no header or editor", () => {
	const { calls, handlers, commands, shortcuts, uiCalls, setFooterCalls, setWidgetCalls, eventHandlers, footerFactory } = startSession();

	assert.deepEqual([...handlers.keys()].sort(), ["session_shutdown", "session_start"], "exactly the two session lifecycle handlers, named");
	assert.deepEqual([...commands.keys()], ["biggz-usage"], "exactly one usage command, in the biggz namespace");
	assert.deepEqual([...shortcuts.keys()], ["alt+u"], "exactly one shortcut, with the upstream default");
	assert.deepEqual([...eventHandlers.keys()], ["biggz-ai:usage-source/v1"], "the only pi.events.on subscription is the usage-source event, and no unexpected subscription is added");
	assert.deepEqual(
		calls.slice().sort(),
		["events.on:biggz-ai:usage-source/v1", "on", "on", "registerCommand", "registerShortcut"],
		"the factory subscribes to the usage-source event, wires both lifecycle handlers, and registers exactly one command and one shortcut",
	);
	assert.equal(setFooterCalls.length, 1, "exactly one footer registration");
	assert.equal(typeof setFooterCalls[0][0], "function", "the single footer registration is a component factory");
	// pi calls the footer factory when it builds the footer component, which is
	// when the header widget is registered; drive that step before asserting.
	footerFactory()({ requestRender() {} }, TAGGED_THEME, fakeFooterData().data);
	assert.deepEqual(uiCalls.slice().sort(), ["setFooter", "setWidget"], "the factory owns the footer and the below-editor row, and nothing else");
	assert.ok(!uiCalls.includes("setHeader"), "the startup banner is not part of this port: nothing claims pi's single header slot");
	assert.ok(!uiCalls.includes("setEditorComponent") && !uiCalls.includes("setWorkingVisible"), "the prompt editor belongs to unit 2.8");
	assert.equal(setWidgetCalls.length, 1, "exactly one widget registration");
	assert.equal(setWidgetCalls[0][0], "biggz-shell-below-input-header", "the widget key stays in the biggz namespace");
	assert.equal(setWidgetCalls[0][2].placement, "belowEditor", "the header row follows the editor");
});

test("the usage shortcut is registered once and disappears for an empty or off value", () => {
	for (const value of ["", "off", "OFF", "  "]) {
		const { commands, shortcuts } = startSession({ BIGGZ_SHELL_USAGE_KEY: value });
		assert.equal(commands.size, 1, "the command stays registered when only the shortcut is disabled");
		assert.equal(shortcuts.size, 0, `BIGGZ_SHELL_USAGE_KEY=${JSON.stringify(value)} must disable the shortcut`);
	}
	const custom = startSession({ BIGGZ_SHELL_USAGE_KEY: "ctrl+u" });
	assert.deepEqual([...custom.shortcuts.keys()], ["ctrl+u"], "a custom value is used verbatim");
});

test("the usage command opens the overlay through ctx.ui.custom", async () => {
	const { commands, ctx, customCalls } = startSession();
	await commands.get("biggz-usage").handler("", ctx);
	assert.equal(customCalls.length, 1, "the command opens exactly one overlay");
	const [factory, options] = customCalls[0];
	assert.equal(typeof factory, "function", "the overlay is built from a component factory");
	assert.equal(options.overlay, true, "the panel is an overlay, not a full-screen replacement");
	assert.deepEqual(options.overlayOptions, { width: "70%", minWidth: 60, anchor: "center" });
});

// The header row is gated on both the below-input placement and a fullscreen
// terminal, so its positive path needs a real visual-customization.json. The
// suite writes one into a throwaway config home and removes it afterwards.
const PLAIN_THEME = {
	fg(_color, text) {
		return text;
	},
	bold(text) {
		return text;
	},
};

function belowInputConfigHome() {
	const home = mkdtempSync(join(tmpdir(), "biggz-shell-widget-"));
	writeFileSync(join(home, "visual-customization.json"), `${JSON.stringify({ schema: VISUAL_SCHEMA, ...DEFAULT_VISUAL_SETTINGS, headerPlacement: HEADER_PLACEMENT.BELOW_INPUT })}\n`);
	return home;
}

test("the header widget stays inert unless placement is below-input and the terminal is fullscreen", () => {
	const home = belowInputConfigHome();
	try {
		const regular = { requestRender() {}, mode: "regular" };
		const fullscreen = { requestRender() {}, mode: "fullscreen" };

		// Default placement is "top": inert even in fullscreen.
		const top = startSession();
		top.footerFactory()(fullscreen, PLAIN_THEME, fakeFooterData().data);
		assert.deepEqual(top.setWidgetCalls[0][1](fullscreen, PLAIN_THEME).render(120), [], "top placement keeps the row inert even in fullscreen");

		// below-input: still inert in a regular-mode terminal.
		const below = startSession({ BIGGZ_CONFIG_HOME: home });
		below.footerFactory()(regular, PLAIN_THEME, fakeFooterData().data);
		below.footerFactory()(fullscreen, PLAIN_THEME, fakeFooterData().data);
		assert.equal(below.setWidgetCalls.length, 2);
		assert.deepEqual(below.setWidgetCalls[0][1](regular, PLAIN_THEME).render(120), [], "a regular-mode terminal keeps the row inert even below-input");

		// below-input plus fullscreen: the bar and its rule.
		const lines = below.setWidgetCalls[1][1](fullscreen, PLAIN_THEME).render(120);
		assert.equal(lines.length, 2, "below-input plus fullscreen paints the bar and its rule");
		assert.match(lines[0], /biggz/, "the row carries the shell brand");
		assert.equal(visibleWidth(lines[0]), 120);
		assert.match(lines[1], /^─{120}$/, "the row is underlined by a full-width rule");

		// The click handler shares the gate: an inert row never swallows a click.
		assert.equal(top.setWidgetCalls[0][1](fullscreen, PLAIN_THEME).handleMouse({ type: "click", button: "left", x: 100, y: 0, width: 120, height: 2 }), undefined);
	} finally {
		rmSync(home, { recursive: true, force: true });
	}
});

test("the header row's usage span opens the overlay on a left click", async () => {
	const home = belowInputConfigHome();
	try {
		const fullscreen = { requestRender() {}, mode: "fullscreen" };
		const { footerFactory, setWidgetCalls, customCalls } = startSession({ BIGGZ_CONFIG_HOME: home });
		footerFactory()(fullscreen, PLAIN_THEME, fakeFooterData().data);
		const widget = setWidgetCalls[0][1](fullscreen, PLAIN_THEME);
		const line = widget.render(120)[0];
		const x = line.indexOf("usage");
		assert.ok(x >= 0, "the row must carry the standing usage segment");
		assert.equal(widget.handleMouse({ type: "click", button: "left", x: x + 1, y: 1, width: 120, height: 2 }), undefined, "the rule row is decorative and never clickable");
		assert.deepEqual(widget.handleMouse({ type: "click", button: "left", x: x + 1, y: 0, width: 120, height: 2 }), { handled: true, render: true });
		await new Promise((resolve) => setTimeout(resolve, 0));
		assert.equal(customCalls.length, 1, "the usage span opens the same overlay as the command");
		assert.equal(customCalls[0][1].overlay, true);
	} finally {
		rmSync(home, { recursive: true, force: true });
	}
});

test("the registered footer renders exactly one line at every width", () => {
	const { footerFactory } = startSession();
	const footerData = fakeFooterData({ branch: "main", statuses: new Map([["k", "ready"]]) });
	const component = footerFactory()({ requestRender() {} }, TAGGED_THEME, footerData.data);
	for (const width of [20, 40, 80, 120, 200]) {
		const lines = component.render(width);
		assert.equal(lines.length, 1, `the footer must paint one line at width ${width}, got ${lines.length}`);
		assert.ok(visibleWidth(lines[0]) <= width, `line width ${visibleWidth(lines[0])} must not exceed ${width}`);
	}
});

test("dispose releases the branch subscription and a branch change requests a render", () => {
	const { pi } = fakePi();
	const footerData = fakeFooterData();
	const renders = { count: 0 };
	const host = { requestRender: () => { renders.count += 1; } };
	const component = createShellBarComponent(pi, modelCtx(), host, TAGGED_THEME, footerData.data);

	assert.equal(footerData.subscribers.size, 1);
	footerData.emitBranchChange();
	assert.equal(renders.count, 1);

	component.dispose();
	assert.equal(footerData.subscribers.size, 0);
	assert.equal(footerData.unsubscribed.length, 1);

	footerData.emitBranchChange();
	assert.equal(renders.count, 1, "a disposed component must no longer request renders");
});

test("the extension's session_shutdown clears the header widget and disposes the installed footer", () => {
	const { handlers, footerFactory, setWidgetCalls } = startSession();
	const footerData = fakeFooterData();
	const component = footerFactory()({ requestRender() {} }, TAGGED_THEME, footerData.data);
	assert.equal(footerData.subscribers.size, 1);
	assert.equal(setWidgetCalls.length, 1, "the widget is registered while the session is live");

	handlers.get("session_shutdown")();
	assert.equal(footerData.subscribers.size, 0);
	assert.equal(setWidgetCalls.length, 2, "session_shutdown must clear the widget");
	assert.equal(setWidgetCalls[1][0], "biggz-shell-below-input-header");
	assert.equal(setWidgetCalls[1][1], undefined, "the cleared widget is removed, not left painting a stale row");
	handlers.get("session_shutdown")();
	assert.equal(footerData.unsubscribed.length, 1, "session_shutdown must be idempotent");
	component.invalidate();
});

// R3-001 containment, proved by execution. The fixture runs in a fresh process
// with a module hook that resolves @earendil-works/pi-tui to a stub missing
// ScrollView/VStack: it asserts the footer, the below-editor widget, /biggz-usage
// and the shortcut still install while no rail is installed, and that the stub
// really does break the rail module's link. A subprocess is required because
// installPiTuiResolver() cannot be re-pointed inside one process.
test("a pi-tui that cannot provide ScrollView/VStack leaves the shell working and the rail absent (R3-001)", () => {
	const fixture = fileURLToPath(new URL("./test/pi-tui-missing-scrollview.test.mjs", import.meta.url));
	// Strip the parent test runner's context: a nested `node --test` that inherits
	// NODE_TEST_CONTEXT refuses to run files, which would make this proof vacuous.
	const env = { ...process.env };
	delete env.NODE_TEST_CONTEXT;
	const result = spawnSync(process.execPath, ["--test", fixture], { encoding: "utf8", env });
	assert.equal(result.status, 0, `the rail's import must be contained to the rail\n${result.stdout}\n${result.stderr}`);
	assert.match(result.stdout, /✔ a pi-tui without ScrollView\/VStack still installs the shell and no rail/, `the child fixture must actually have run\n${result.stdout}\n${result.stderr}`);
});
