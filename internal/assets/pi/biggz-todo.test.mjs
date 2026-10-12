import assert from "node:assert/strict";
import test from "node:test";
import { spawnSync } from "node:child_process";
import "./test/pi-tui-resolver.mjs";
import { stripAnsi } from "./biggz-shell/terminal-theme.ts";

// The Biggz Todo extension: the `todo` tool, the card above the editor, the
// per-turn prompt block, and the staleness signal, driven by fakes. Ported from
// gentle-pi 3.7.0 tests/gentle-todo.test.ts (MIT, Copyright (c) 2025 Mario
// Zechner).
//
// pi-tui is supplied by the repository's test oracle; the factory is imported
// dynamically so its static pi-tui import resolves after the oracle hook is
// installed.
const { default: biggzTodo, todoCollapseKey, todoEnabled } = await import("./biggz-todo.ts");

const plainTheme = {
	fg(_color, text) {
		return text;
	},
	strikethrough(text) {
		return `~${text}~`;
	},
};
const fakeTui = { requestRender() {} };
function fakePi() {
	const handlers = new Map();
	const tools = new Map();
	const shortcuts = new Map();
	const pi = {
		on(event, handler) {
			handlers.set(event, [...(handlers.get(event) ?? []), handler]);
		},
		registerTool(tool) {
			tools.set(tool.name, tool);
		},
		registerShortcut(key, registration) {
			shortcuts.set(key, registration);
		},
	};
	const fire = async (event, ctx, payload = {}) => {
		let last;
		for (const handler of handlers.get(event) ?? []) last = await handler(payload, ctx);
		return last;
	};
	return { pi, tools, shortcuts, fire };
}

function fakeContext(branch = [], hasUI = true) {
	const widgets = new Map();
	const ctx = {
		hasUI,
		sessionManager: { getSessionId: () => "s1", getBranch: () => branch },
		ui: {
			setWidget(key, content) {
				if (content === undefined) widgets.delete(key);
				else widgets.set(key, content);
			},
		},
	};
	const widgetComponent = () => {
		const factory = widgets.get("biggz-todo");
		return factory?.(fakeTui, plainTheme);
	};
	const widget = () => widgetComponent()?.render(70).map(stripAnsi);
	return { ctx, widgets, widget, widgetComponent };
}

test("todoEnabled and todoCollapseKey read their environment flags", () => {
	assert.equal(todoEnabled({}), true);
	assert.equal(todoEnabled({ BIGGZ_TODO: "0" }), false);
	assert.equal(todoEnabled({ PI_SUBAGENT_CHILD: "1" }), false);
	assert.equal(todoCollapseKey({}), "ctrl+shift+t");
	assert.equal(todoCollapseKey({ BIGGZ_TODO_KEY: "alt+t" }), "alt+t");
	assert.equal(todoCollapseKey({ BIGGZ_TODO_KEY: "off" }), undefined);
	const off = fakePi();
	biggzTodo(off.pi, { BIGGZ_TODO: "off" });
	assert.equal(off.tools.size, 0);
});

test("todo registration owns its transparent transcript shell", () => {
	const { pi, tools } = fakePi();
	biggzTodo(pi, {});
	assert.equal(tools.get("todo")?.renderShell, "self");
});

test("the todo tool writes the list, shows the card after the call, and carries the snapshot in details", async () => {
	const { pi, tools, fire } = fakePi();
	biggzTodo(pi, {});
	const { ctx, widget } = fakeContext();
	await fire("session_start", ctx);
	assert.equal(widget(), undefined, "no card without tasks");

	const tool = tools.get("todo");
	const result = await tool.execute("c1", { action: "write", tasks: [{ title: "Write the parser", status: "in_progress", note: "parsing" }, { title: "Add tests" }] }, undefined, undefined, ctx);
	assert.match(result.content[0].text, /2 tasks · 0 done · 1 in progress/);
	assert.equal(result.details.biggzTodo.tasks.length, 2);
	await fire("tool_execution_end", ctx, { toolName: "todo" });
	const lines = widget();
	assert.match(lines[0], /^╭─ ❀ Todos ▾ Collapse · 0 of 2 ─+ ctrl\+shift\+t collapse ╮$/);
	assert.match(lines[1], /◐ Write the parser · parsing/);
	assert.match(lines[2], /○ Add tests/);
	assert.equal(lines[lines.length - 1], "", "a blank line keeps the card off the prompt");

	const bad = await tool.execute("c2", { action: "update", id: 9, status: "done" }, undefined, undefined, ctx);
	assert.match(bad.content[0].text, /Error: no task #9/);
	assert.equal(bad.details.error, "no task #9");
	assert.match(tool.renderCall({ action: "write" }, plainTheme).render(40).join(""), /❀ todo · write/);
	assert.equal(tool.renderResult({ content: [{ type: "text", text: "a\nb" }] }, { expanded: false }, plainTheme).render(40).join("|").trimEnd(), "a");
});

test("the Todo header is a fullscreen left-click control while non-click pointer events stay inert", async () => {
	const { pi, tools, fire } = fakePi();
	biggzTodo(pi, {});
	const { ctx, widgetComponent } = fakeContext();
	await fire("session_start", ctx);
	await tools.get("todo").execute("c1", { action: "write", tasks: [{ title: "A", status: "in_progress" }, { title: "B" }] }, undefined, undefined, ctx);
	await fire("tool_execution_end", ctx, { toolName: "todo" });

	const component = widgetComponent();
	assert.match(stripAnsi(component.render(70)[0]), /Todos ▾ Collapse/);
	const event = (type, button, y = 0) => ({
		type, button, x: 1, y, screenX: 1, screenY: y, width: 70, height: 5, shift: false, alt: false, ctrl: false,
	});
	assert.equal(component.handleMouse?.(event("press", "left")), undefined);
	assert.equal(component.handleMouse?.(event("click", "right")), undefined);
	assert.equal(component.handleMouse?.(event("click", "left", 1)), undefined);
	assert.match(stripAnsi(component.render(70)[0]), /Todos ▾ Collapse/, "only a left click on the header toggles");
	assert.equal(component.handleMouse?.(event("click", "left"))?.handled, true);
	assert.match(stripAnsi(component.render(70)[0]), /Todos ▸ Expand/);
});

// H1 (odd/tasks/usage-click-and-changes-attribution.md): the header control
// now paints the same shared hover role every other clickable surface uses.
test("the Todo header paints the shared hover role while hovered, and clears it off the header row or on leave", async () => {
	const { pi, tools, fire } = fakePi();
	biggzTodo(pi, {});
	const { ctx, widgetComponent } = fakeContext();
	await fire("session_start", ctx);
	await tools.get("todo").execute("c1", { action: "write", tasks: [{ title: "A" }] }, undefined, undefined, ctx);
	await fire("tool_execution_end", ctx, { toolName: "todo" });
	const component = widgetComponent();
	const move = (y) => ({ type: "move", button: "none", x: 1, y, screenX: 1, screenY: y, width: 70, height: 5, shift: false, alt: false, ctrl: false });
	assert.match(stripAnsi(component.render(70)[0]), /Todos ▾ Collapse/);

	const entered = component.handleMouse?.(move(0));
	assert.deepEqual(entered, { handled: true, render: true });
	assert.match(stripAnsi(component.render(70)[0]), /Todos ▾ Collapse/, "the collapse label is unchanged; only its role changes (not observable through plainTheme here)");

	// Moving to another row of the card (still inside the region, but off the
	// clickable header) clears the hover.
	const movedOff = component.handleMouse?.(move(1));
	assert.deepEqual(movedOff, { handled: true, render: true });

	// Re-entering, then a second move at the same row is a no-op (already hovered).
	component.handleMouse?.(move(0));
	assert.deepEqual(component.handleMouse?.(move(0)), { handled: true });
});

test("every turn carries the open tasks in the system prompt and the card goes stale after two silent turns", async () => {
	const { pi, tools, fire } = fakePi();
	biggzTodo(pi, {});
	const { ctx, widget } = fakeContext();
	await fire("session_start", ctx);
	await fire("before_agent_start", ctx, { systemPrompt: "base" });
	await tools.get("todo").execute("c1", { action: "write", tasks: [{ title: "Fix the bug" }] }, undefined, undefined, ctx);
	await fire("tool_execution_end", ctx, { toolName: "todo" });

	const withTasks = await fire("before_agent_start", ctx, { systemPrompt: "base" });
	assert.match(withTasks.systemPrompt, /^base\n\n## Todo list/);
	assert.match(withTasks.systemPrompt, /1\. \[pending\] Fix the bug/);
	assert.doesNotMatch(widget()[0], /stale/);

	const stale = await fire("before_agent_start", ctx, { systemPrompt: "base" });
	assert.match(stale.systemPrompt, /stale: 2 turns without an update/);
	assert.match(widget()[0], /ctrl\+shift\+t collapse/);
	assert.match(widget()[1], /stale · 2 turns/);

	await tools.get("todo").execute("c2", { action: "update", id: 1, status: "in_progress", note: "on it" }, undefined, undefined, ctx);
	await fire("tool_execution_end", ctx, { toolName: "todo" });
	assert.doesNotMatch(widget()[0], /stale/);
});

test("a finished list stays for its turn and clears at the next, and the collapse key folds the card", async () => {
	const { pi, tools, shortcuts, fire } = fakePi();
	biggzTodo(pi, {});
	const { ctx, widget } = fakeContext();
	await fire("session_start", ctx);
	await tools.get("todo").execute("c1", { action: "write", tasks: [{ title: "A", status: "in_progress" }, { title: "B" }] }, undefined, undefined, ctx);
	await fire("tool_execution_end", ctx, { toolName: "todo" });
	await shortcuts.get("ctrl+shift+t").handler(ctx);
	assert.equal(widget().length, 4, "collapsed: top, one row, bottom, spacer");
	assert.match(widget()[1], /◐ A/);
	await shortcuts.get("ctrl+shift+t").handler(ctx);
	assert.equal(widget().length, 5);

	await tools.get("todo").execute("c2", { action: "write", tasks: [{ id: 1, title: "A", status: "done" }, { id: 2, title: "B", status: "done" }] }, undefined, undefined, ctx);
	await fire("tool_execution_end", ctx, { toolName: "todo" });
	await fire("agent_end", ctx);
	assert.match(widget()[0], /Todos ▾ Collapse · 2 of 2/, "the finished list is still visible at the end of its turn");
	const next = await fire("before_agent_start", ctx, { systemPrompt: "base" });
	assert.equal(next, undefined, "nothing open, nothing to add to the prompt");
	assert.equal(widget(), undefined, "the card clears at the next turn");
});

test("session_start replays the list from the branch, rpiv-todo results included, and counts past turns", async () => {
	const { pi, fire } = fakePi();
	biggzTodo(pi, {});
	const branch = [
		{ type: "message", message: { role: "user", content: "hi" } },
		{ type: "message", message: { role: "toolResult", toolName: "todo", isError: false, details: { action: "create", params: {}, tasks: [{ id: 1, subject: "Old task", status: "in_progress", activeForm: "still going" }], nextId: 2 } } },
		{ type: "message", message: { role: "user", content: "again" } },
	];
	const { ctx, widget } = fakeContext(branch);
	await fire("session_start", ctx);
	const lines = widget();
	assert.match(lines[0], /Todos ▾ Collapse · 0 of 1/);
	assert.match(lines[1], /◐ Old task · still going/);
	const headless = fakeContext(branch, false);
	await fire("session_start", headless.ctx);
	assert.equal(headless.widget(), undefined);
});

test("session_start drops a list that was already finished, so a reload never shows stale done work", async () => {
	const { pi, fire } = fakePi();
	biggzTodo(pi, {});
	const finished = [
		{ type: "message", message: { role: "user", content: "hi" } },
		{ type: "message", message: { role: "toolResult", toolName: "todo", isError: false, details: { biggzTodo: { tasks: [{ id: 1, title: "Done A", status: "done" }, { id: 2, title: "Done B", status: "done" }], nextId: 3, updatedTurn: 1 } } } },
	];
	const { ctx, widget } = fakeContext(finished);
	await fire("session_start", ctx);
	assert.equal(widget(), undefined, "nothing to show after a reload of finished work");
	const prompt = await fire("before_agent_start", ctx, { systemPrompt: "base" });
	assert.equal(prompt, undefined, "and nothing is injected into the prompt");
});

// Unit 2.7b (R3-001) containment, proved by execution. The fixture runs in a
// fresh process with a module hook that resolves @earendil-works/pi-tui to a
// stub missing ScrollView/VStack: it asserts the `todo` tool and its card still
// register while the rail module cannot link. A subprocess is required because
// the stub specifier cannot be re-pointed in a process that already imported
// the real (or oracle) pi-tui through the repository resolver.
test("a pi-tui that cannot provide ScrollView/VStack still registers the todo tool and widget (R3-001)", () => {
	const child = `
import assert from "node:assert/strict";
import { registerHooks } from "node:module";
const oracleUrl = ${JSON.stringify(new URL("./test/pi-tui-oracle.mjs", import.meta.url).href)};
const factoryUrl = ${JSON.stringify(new URL("./biggz-todo.ts", import.meta.url).href)};
const layoutUrl = ${JSON.stringify(new URL("./biggz-shell/shell-sidebar-layout.ts", import.meta.url).href)};
const stubSource = "export { truncateToWidth, visibleWidth, wrapTextWithAnsi, Key, matchesKey, MouseRegion } from " + JSON.stringify(oracleUrl) + "; export class Text { render() { return []; } }";
const stub = "data:text/javascript," + encodeURIComponent(stubSource);
registerHooks({
	resolve(specifier, context, nextResolve) {
		if (specifier === "@earendil-works/pi-tui") return { url: stub, shortCircuit: true };
		return nextResolve(specifier, context);
	},
});
process.on("unhandledRejection", (error) => {
	console.error("unhandled rejection escaped the containment", error);
	process.exitCode = 1;
});

// Control: the stub really cannot satisfy the rail module, so a passing run
// below cannot be a stub that accidentally kept ScrollView/VStack.
await assert.rejects(
	() => import(layoutUrl),
	(error) => {
		assert.ok(error instanceof SyntaxError, "the rail must fail at ESM link time with a SyntaxError, got " + error);
		assert.match(error.message, /does not provide an export named '(ScrollView|VStack)'/, "the link error must name the missing ScrollView/VStack export: " + error.message);
		return true;
	},
	"the stub has no ScrollView/VStack, so shell-sidebar-layout.ts must fail to link",
);

const { default: biggzTodo } = await import(factoryUrl);
const handlers = new Map();
const tools = new Map();
const widgets = new Map();
const pi = {
	on(event, handler) {
		handlers.set(event, [...(handlers.get(event) ?? []), handler]);
	},
	registerTool(tool) {
		tools.set(tool.name, tool);
	},
	registerShortcut() {},
};
biggzTodo(pi, {});
assert.ok(tools.get("todo"), "the todo tool must still register without the rail module");
assert.equal(tools.get("todo").renderShell, "self", "the tool keeps its transparent transcript shell");

const ctx = {
	hasUI: true,
	sessionManager: { getSessionId: () => "s1", getBranch: () => [] },
	ui: {
		setWidget(key, content) {
			if (content === undefined) widgets.delete(key);
			else widgets.set(key, content);
		},
	},
};
for (const handler of handlers.get("session_start") ?? []) await handler({}, ctx);
await tools.get("todo").execute("c1", { action: "write", tasks: [{ title: "A" }] }, undefined, undefined, ctx);
for (const handler of handlers.get("tool_execution_end") ?? []) await handler({ toolName: "todo" }, ctx);
assert.ok(widgets.has("biggz-todo"), "the card must still register without the rail module");

// Deterministic flush: the control already forced the rail module's failed
// link, so the factory's own lazy import of the same specifier rejects on a
// settled microtask. If the rejection were unhandled, the handler above turns
// it into a non-zero exit; no wall-clock sleep is involved.
for (let i = 0; i < 100; i++) await Promise.resolve();

console.log("biggz-todo rail containment ok");
`;
	// Strip the parent test runner's context: a nested runner in the child would
	// otherwise inherit it and refuse to run.
	const env = { ...process.env };
	delete env.NODE_TEST_CONTEXT;
	const result = spawnSync(process.execPath, ["--input-type=module", "--eval", child], { encoding: "utf8", env });
	// The discriminating assertion is this exit-status check, not the child's
	// semantic asserts (tools.get("todo") / widgets.has("biggz-todo")). If the
	// factory regressed to a static import of the rail, the child would die at
	// its top-level `await import(factoryUrl)` when shell-sidebar-layout.ts
	// fails to link, long before any of those semantic asserts run. A non-zero
	// status is therefore the only signal that reaches the parent in the
	// regression case; the semantic asserts can only ever execute once the
	// subprocess has already survived the import, so they confirm the happy path
	// rather than catch the containment break.
	assert.equal(result.status, 0, `the rail's import must be contained to the rail\n${result.stdout}\n${result.stderr}`);
	assert.match(result.stdout, /biggz-todo rail containment ok/, `the child fixture must actually have run\n${result.stdout}\n${result.stderr}`);
});
