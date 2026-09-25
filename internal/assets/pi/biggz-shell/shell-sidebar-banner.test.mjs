// Ported from gentle-pi 3.7.0 tests/shell-sidebar-banner.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged apart from the
// biggz-ai product label below (brand rename).
// pi-tui is supplied by the repository's test oracle (see test/pi-tui-resolver.mjs).
import assert from "node:assert/strict";
import test from "node:test";
import { PI_TUI_SPECIFIER } from "../test/pi-tui-resolver.mjs";

const { visibleWidth } = await import(PI_TUI_SPECIFIER);
const { renderSidebarBanner } = await import("./shell-sidebar-banner.ts");

const plain = { fg: (_role, text) => text, bold: (text) => text };
test("sidebar heading is only the centered literal product label", () => {
	const label = "✿ biggz ✿";
	for (const width of [0, 1, 8, 11, 15, 16, 17, 20, 21, 30, 45, 46, 80]) {
		const lines = renderSidebarBanner(plain, width);
		const space = width - visibleWidth(label);
		assert.deepEqual(lines, space >= 0 ? [" ".repeat(Math.floor(space / 2)) + label + " ".repeat(Math.ceil(space / 2))] : []);
		assert.ok(lines.every((line) => visibleWidth(line) === width));
	}
});

test("heading uses the current theme on every render", () => {
	let palette = "dark";
	const theme = { ...plain, fg: (role, text) => `<${palette}:${role}>${text}` };
	assert.equal(renderSidebarBanner(theme, 46)[0].trim(), "<dark:accent>✿ <dark:text>biggz <dark:accent>✿");
	palette = "light";
	assert.equal(renderSidebarBanner(theme, 46)[0].trim(), "<light:accent>✿ <light:text>biggz <light:accent>✿");
});
