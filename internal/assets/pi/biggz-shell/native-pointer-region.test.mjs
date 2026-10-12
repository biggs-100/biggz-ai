// Ported from gentle-pi 3.7.0 tests/native-pointer-region.test.ts (245 lines).
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged.
import assert from "node:assert/strict";
import test from "node:test";
import { PI_TUI_SPECIFIER } from "../test/pi-tui-resolver.mjs";

const { Box, Container, Text } = await import(PI_TUI_SPECIFIER);
const { createNativePointerScope } = await import("./native-pointer-region.ts");

function mouse(type, button, y, height) {
	return {
		type, button, x: 0, y, screenX: 0, screenY: y, width: 40, height,
		shift: false, alt: false, ctrl: false,
	};
}

function dispatch(root, observer, event) {
	observer.beforeMouse(event);
	try {
		return root.handleMouse(event);
	} finally {
		observer.afterMouse(event);
	}
}

// Typed callback harness, not proof of physical terminal overlay UX.
async function mountOverlay(factory, options) {
	assert.equal(options.overlay, true, "the public custom factory receives overlay mode");
	return factory({}, {}, {}, () => {});
}

test("pointer regions give a public Text hover, press, and click behavior without implicit focus", () => {
	const scope = createNativePointerScope();
	const calls = [];
	const region = scope.wrap(new Text("Open editor", 0, 0), {
		onHover: () => {
			calls.push("hover");
			return { handled: true, render: true };
		},
		onPress: () => {
			calls.push("press");
			return { handled: true };
		},
		onClick: () => {
			calls.push("click");
			return { handled: true };
		},
		onWheel: () => {
			calls.push("wheel");
			return { handled: true };
		},
	});
	const root = new Container();
	root.addChild(region);
	const height = root.render(40).length;
	const observer = scope.createMouseObserver();

	assert.equal(dispatch(root, observer, mouse("move", "none", 0, height))?.render, true);
	const press = dispatch(root, observer, mouse("press", "left", 0, height));
	assert.equal(press?.handled, true);
	assert.equal(press?.focus, undefined, "a Text region never claims keyboard focus by itself");
	assert.equal(dispatch(root, observer, mouse("click", "left", 0, height))?.handled, true);
	assert.equal(dispatch(root, observer, mouse("wheel", "none", 0, height))?.handled, true);
	assert.deepEqual(calls, ["hover", "press", "click", "wheel"]);
});

test("pointer scopes clear hover only through observed lifecycle boundaries", () => {
	const scope = createNativePointerScope();
	const states = [];
	const region = scope.wrap(new Text("Hover me", 0, 0), {
		onHover: () => {
			states.push("hover");
			return { handled: true, render: true };
		},
		onLeave: () => states.push("leave"),
	});
	const root = new Container();
	root.addChild(region);
	const observer = scope.createMouseObserver();
	const height = root.render(40).length;

	dispatch(root, observer, mouse("move", "none", 0, height));
	dispatch(root, observer, mouse("move", "none", height, height + 1));
	assert.deepEqual(states, ["hover", "leave"], "an in-root empty move clears hover");

	dispatch(root, observer, mouse("move", "none", 0, height));
	region.render(30);
	assert.deepEqual(states, ["hover", "leave", "hover", "leave"], "resize clears hover");

	dispatch(root, observer, mouse("move", "none", 0, height));
	region.invalidate();
	region.setDisabled(true);
	region.dispose();
	scope.dispose();
	dispatch(root, observer, mouse("move", "none", 0, height));
	assert.deepEqual(
		states,
		["hover", "leave", "hover", "leave", "hover", "leave"],
		"invalidate, disable, disposal, and late events cannot retain or restore hover",
	);
});

test("a consuming public Box child keeps its exact native target and focus without outer callbacks", () => {
	const scope = createNativePointerScope();
	let outerClicks = 0;
	const child = {
		render: () => ["native child"],
		invalidate() {},
		handleMouse: () => ({ handled: true, focus: true, render: true }),
	};
	const box = new Box(0, 0);
	box.addChild(child);
	const region = scope.wrap(box, {
		onClick: () => {
			outerClicks++;
			return { handled: true };
		},
	});
	const root = new Container();
	root.addChild(region);
	const height = root.render(40).length;
	const result = dispatch(root, scope.createMouseObserver(), mouse("click", "left", 0, height));

	assert.equal(result?.target.component, child);
	assert.equal(result?.focusTarget, child);
	assert.equal(outerClicks, 0, "a consumed child prevents outer pointer callbacks");
});

test("the same composed region can open an input entry point without answering on press", () => {
	const scope = createNativePointerScope();
	let opened = 0;
	const panel = new Container();
	panel.addChild(scope.wrap(new Text("Open input", 0, 0), {
		onPress: () => ({ handled: true }),
		onClick: () => {
			opened++;
			return { handled: true };
		},
	}));
	const root = new Container();
	root.addChild(panel);
	const height = root.render(40).length;
	const observer = scope.createMouseObserver();

	assert.equal(dispatch(root, observer, mouse("press", "left", 0, height))?.handled, true);
	assert.equal(opened, 0, "press can prepare a control but cannot answer it");
	assert.equal(dispatch(root, observer, mouse("click", "left", 0, height))?.handled, true);
	assert.equal(opened, 1, "a caller-owned callback opens the input entry point");
});

test("a typed public custom overlay factory composes pointer content and disposes late events", async () => {
	let opened = 0;
	let leaves = 0;
	const component = await mountOverlay(() => {
		const scope = createNativePointerScope();
		const panel = new Container();
		panel.addChild(scope.wrap(new Text("Open overlay input", 0, 0), {
			onHover: () => ({ handled: true, render: true }),
			onLeave: () => { leaves++; },
			onClick: () => {
				opened++;
				return { handled: true };
			},
		}));
		const observer = scope.createMouseObserver();
		return {
			render: (width) => panel.render(width),
			handleMouse: (event) => {
				observer.beforeMouse(event);
				try {
					return panel.handleMouse(event);
				} finally {
					observer.afterMouse(event);
				}
			},
			invalidate: () => panel.invalidate(),
			dispose: () => scope.dispose(),
		};
	}, { overlay: true });
	const height = component.render(40).length;
	const hover = component.handleMouse?.(mouse("move", "none", 0, height));
	const click = component.handleMouse?.(mouse("click", "left", 0, height));

	assert.equal(hover?.render, true);
	assert.equal(click?.focus, undefined, "overlay content does not claim focus without a caller result");
	assert.equal(opened, 1, "the factory-returned component routes a native click to its callback");
	component.dispose?.();
	assert.equal(component.handleMouse?.(mouse("click", "left", 0, height)), undefined);
	assert.deepEqual([opened, leaves], [1, 1], "disposal clears hover and rejects late overlay events");
});

test("scopes apply disabled and disposed state to later Text regions", () => {
	const calls = [];
	const disabled = createNativePointerScope();
	disabled.setDisabled(true);
	const disabledRoot = new Container();
	disabledRoot.addChild(disabled.wrap(new Text("Disabled", 0, 0), {
		onClick: () => {
			calls.push("disabled");
			return { handled: true };
		},
	}));
	const disabledHeight = disabledRoot.render(40).length;
	const disabledClick = dispatch(
		disabledRoot, disabled.createMouseObserver(), mouse("click", "left", 0, disabledHeight),
	);
	disabled.setDisabled(false);
	const reenabledClick = dispatch(
		disabledRoot, disabled.createMouseObserver(), mouse("click", "left", 0, disabledHeight),
	);

	const disposed = createNativePointerScope();
	disposed.dispose();
	const disposedRoot = new Container();
	disposedRoot.addChild(disposed.wrap(new Text("Disposed", 0, 0), {
		onClick: () => {
			calls.push("disposed");
			return { handled: true };
		},
	}));
	const disposedHeight = disposedRoot.render(40).length;
	const disposedClick = dispatch(
		disposedRoot, disposed.createMouseObserver(), mouse("click", "left", 0, disposedHeight),
	);

	assert.deepEqual(
		[disabledClick?.handled, reenabledClick?.handled, disposedClick?.handled, calls],
		[undefined, true, undefined, ["disabled"]],
		"later regions inherit scope disable/disposal while a live scope can be re-enabled",
	);
});

// ---------------------------------------------------------------------------
// biggz coverage, task 2.7c. Not upstream: these three cases close the
// informational findings the native review of 2.7a admitted
// (native-pointer-region.ts:145-150, :51-54 and :117).
// ---------------------------------------------------------------------------

test("two regions in one scope clear the sibling's hover when the pointer enters the other", () => {
	const scope = createNativePointerScope();
	const hovered = [];
	const firstLeaves = [];
	let secondHovers = 0;
	let secondHoverEvent;
	const first = scope.wrap(new Text("First", 0, 0), {
		onHover: () => {
			hovered.push("first");
			return { handled: true, render: true };
		},
		onLeave: (event) => firstLeaves.push(event),
	});
	const second = scope.wrap(new Text("Second", 0, 0), {
		onHover: (event) => {
			secondHovers++;
			secondHoverEvent = event;
			return { handled: true, render: true };
		},
	});
	const root = new Container();
	root.addChild(first);
	root.addChild(second);
	const height = root.render(40).length;
	const observer = scope.createMouseObserver();

	assert.equal(height, 2, "the fixture stacks exactly two single-row regions");
	assert.equal(dispatch(root, observer, mouse("move", "none", 0, height))?.render, true);
	assert.deepEqual(
		[hovered, firstLeaves.length],
		[["first"], 0],
		"entering the first region hovers it without firing a single leave",
	);

	assert.equal(dispatch(root, observer, mouse("move", "none", 1, height))?.render, true);
	assert.deepEqual(
		[hovered, firstLeaves.length, secondHovers],
		[["first"], 1, 1],
		"entering the second region hovers it and fires the sibling's leave once",
	);
	assert.equal(
		firstLeaves[0],
		secondHoverEvent,
		"the sibling clear hands over the very same event object the entering region saw, so it carries that region's local coordinates rather than the root's",
	);

	dispatch(root, observer, mouse("move", "none", 0, height));
	assert.deepEqual(
		[hovered, firstLeaves.length, secondHovers],
		[["first", "first"], 1, 1],
		"moving back hovers the first again and leaves the second's absent callback alone",
	);
	dispatch(root, observer, mouse("move", "none", 0, height));
	dispatch(root, observer, mouse("move", "none", 0, height));
	assert.deepEqual(
		[hovered.length, firstLeaves.length],
		[4, 1],
		"staying inside one region keeps hovering it and never re-fires the sibling clear",
	);
});

test("a disabled scope gates its own callbacks, and an interactive child keeps consuming natively", () => {
	const scope = createNativePointerScope();
	const regionClicks = [];
	const passiveRoot = new Container();
	passiveRoot.addChild(scope.wrap(new Text("Passive", 0, 0), {
		onClick: () => {
			regionClicks.push("passive");
			return { handled: true };
		},
	}));
	const passiveHeight = passiveRoot.render(40).length;

	scope.setDisabled(true);
	const passiveClick = dispatch(
		passiveRoot, scope.createMouseObserver(), mouse("click", "left", 0, passiveHeight),
	);
	assert.equal(passiveClick?.handled, undefined, "a disabled scope answers nothing for a passive child");
	assert.deepEqual(regionClicks, [], "and never reaches the region's own click callback");

	// The guard's reach stops there. A child that consumes natively is dispatched
	// BEFORE the region's callback (that is what keeps an already handled child
	// result unchanged), so scope disablement cannot gate it: this pins the real
	// contract instead of assuming the flag protects the whole subtree.
	let childCalls = 0;
	const child = {
		render: () => ["interactive child"],
		invalidate() {},
		handleMouse: () => {
			childCalls++;
			return { handled: true, focus: true, render: true };
		},
	};
	const interactiveScope = createNativePointerScope();
	const box = new Box(0, 0);
	box.addChild(child);
	const interactiveRoot = new Container();
	interactiveRoot.addChild(interactiveScope.wrap(box, {
		onClick: () => {
			regionClicks.push("interactive");
			return { handled: true };
		},
	}));
	const interactiveHeight = interactiveRoot.render(40).length;
	const observer = interactiveScope.createMouseObserver();

	interactiveScope.setDisabled(true);
	const disabledInteractive = dispatch(
		interactiveRoot, observer, mouse("click", "left", 0, interactiveHeight),
	);
	assert.equal(disabledInteractive?.handled, true, "a disabled scope still lets an interactive child consume");
	assert.equal(disabledInteractive?.focusTarget, child, "and that child keeps its native focus target");
	assert.equal(childCalls, 1, "the disabled flag never reaches a child that dispatches first");

	interactiveScope.setDisabled(false);
	dispatch(interactiveRoot, observer, mouse("click", "left", 0, interactiveHeight));
	assert.equal(childCalls, 2, "re-enabling adds no gate of its own");
	assert.deepEqual(regionClicks, [], "the region's callback stays unreached while the child consumes");
});

test("the mouse observer requests exactly one render when an empty in-root move clears hover", () => {
	const scope = createNativePointerScope();
	let renders = 0;
	const region = scope.wrap(new Text("Hover me", 0, 0), {
		onHover: () => ({ handled: true, render: true }),
	});
	const root = new Container();
	root.addChild(region);
	const height = root.render(40).length;
	const observer = scope.createMouseObserver(() => {
		renders++;
	});

	dispatch(root, observer, mouse("move", "none", height, height + 1));
	assert.equal(renders, 0, "an empty move with nothing hovered asks for no render");

	dispatch(root, observer, mouse("move", "none", 0, height));
	assert.equal(renders, 0, "the hover render is the region's own result, so the observer adds none");

	dispatch(root, observer, mouse("move", "none", height, height + 1));
	assert.equal(renders, 1, "clearing a live hover asks for exactly one render");

	dispatch(root, observer, mouse("move", "none", height, height + 1));
	assert.equal(renders, 1, "a second empty move clears nothing and therefore asks for nothing");
});
