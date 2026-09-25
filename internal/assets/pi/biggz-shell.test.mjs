// biggz-ai shell factory guard (unit 2.4, not ported from upstream).
//
// The shell extension owns pi's footer slot: it is the one deployed extension
// that calls ctx.ui.setFooter, it installs the ported shell bar, and it paints
// exactly one line. This suite pins that contract — the model the footer builds,
// the two kill switches that gate installation, the single-line render at the
// widths that matter, and the fact that the factory registers a footer and
// nothing else (no widget, no header, no editor).
//
// pi-tui is supplied by the repository's test oracle (see test/pi-tui-resolver.mjs).
import assert from "node:assert/strict";
import test from "node:test";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { PI_TUI_SPECIFIER } from "./test/pi-tui-resolver.mjs";

// Keep the visual-settings read hermetic: resolveVisualSettings() reads
// <config home>/visual-customization.json, and the suite must not depend on a
// real user config. A path that does not exist exercises the default branch.
process.env.BIGGZ_CONFIG_HOME = join(tmpdir(), "biggz-shell-test-config-does-not-exist");

const { visibleWidth } = await import(PI_TUI_SPECIFIER);
const { buildShellBarModel, createShellBarComponent, default: biggzShell } = await import("./biggz-shell.ts");

const TAGGED_THEME = {
	fg(color, text) {
		return `<${color}>${text}</${color}>`;
	},
	bold(text) {
		return text;
	},
};

// A pi stub that records every method it is asked for, so a test can prove the
// factory touched nothing else.
function fakePi({ thinkingLevel = "medium" } = {}) {
	const calls = [];
	const handlers = new Map();
	const pi = new Proxy(
		{},
		{
			get(_target, prop) {
				if (prop === "getThinkingLevel") {
					return () => {
						calls.push("getThinkingLevel");
						return thinkingLevel;
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
	return { pi, calls, handlers };
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

// A ctx stub whose ui is a recording proxy: any UI call the factory makes is
// captured, so "registered the footer and nothing else" is an assertion rather
// than a hope.
function fakeCtx({
	hasUI = true,
	cwd = "/home/tester/project",
	sessionName = undefined,
	entries = [],
	usage = undefined,
	model = undefined,
	oauth = false,
} = {}) {
	const uiCalls = [];
	let footerFactory;
	const ui = new Proxy(
		{},
		{
			get(_target, prop) {
				return (...args) => {
					uiCalls.push(prop);
					if (prop === "setFooter") footerFactory = args[0];
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
		modelRegistry: { isUsingOAuth: () => oauth },
		sessionManager: {
			getCwd: () => cwd,
			getSessionName: () => sessionName,
			getEntries: () => entries,
		},
		getContextUsage: () => usage,
		ui,
	};
	return { ctx, uiCalls, footerFactory: () => footerFactory };
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
// footer factory.
function startSession(env = {}) {
	const { pi, calls, handlers } = fakePi();
	biggzShell(pi);
	return withEnv({ BIGGZ_PRETTY: undefined, BIGGZ_SHELL: undefined, PI_SUBAGENT_CHILD: undefined, ...env }, () => {
		const { ctx, uiCalls, footerFactory } = fakeCtx();
		handlers.get("session_start")({}, ctx);
		return { pi, calls, handlers, ctx, uiCalls, footerFactory };
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
	const { uiCalls } = startSession({ BIGGZ_PRETTY: "0" });
	assert.deepEqual(uiCalls, [], "the repo-wide pretty kill switch must keep pi's built-in footer");
});

test("factory installs nothing when BIGGZ_SHELL=0", () => {
	const { uiCalls } = startSession({ BIGGZ_SHELL: "0" });
	assert.deepEqual(uiCalls, []);
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

test("factory registers the footer and nothing else", () => {
	const { pi, calls, handlers } = fakePi();
	biggzShell(pi);
	const { ctx, uiCalls } = fakeCtx();
	handlers.get("session_start")({}, ctx);

	assert.deepEqual([...handlers.keys()].sort(), ["session_shutdown", "session_start"]);
	assert.deepEqual(uiCalls, ["setFooter"], "the filter slice must not install a widget, header or editor");
	assert.deepEqual(
		calls.filter((call) => call !== "on"),
		[],
		"the factory must not register commands, tools or providers",
	);
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

test("the extension's session_shutdown disposes the installed footer", () => {
	const { pi, handlers } = fakePi();
	biggzShell(pi);
	const footerData = fakeFooterData();
	const { ctx, footerFactory } = fakeCtx();
	handlers.get("session_start")({}, ctx);
	const component = footerFactory()({ requestRender() {} }, TAGGED_THEME, footerData.data);
	assert.equal(footerData.subscribers.size, 1);

	handlers.get("session_shutdown")();
	assert.equal(footerData.subscribers.size, 0);
	handlers.get("session_shutdown")();
	assert.equal(footerData.unsubscribed.length, 1, "session_shutdown must be idempotent");
	component.invalidate();
});
