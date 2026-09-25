// Ported from gentle-pi 3.7.0 tests/shell-hover.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged.
import assert from "node:assert/strict";
import test from "node:test";
import { HOVER_ROLE, paintHoverable } from "./shell-hover.ts";

const theme = {
	fg(role, text) {
		return `<${role}>${text}</${role}>`;
	},
};

test("paintHoverable paints the shared hover role only while hovered", () => {
	assert.equal(paintHoverable(theme, "refresh", true), `<${HOVER_ROLE}>refresh</${HOVER_ROLE}>`);
	assert.equal(paintHoverable(theme, "refresh", false), "refresh");
});

test("paintHoverable falls back to an idle role when given one", () => {
	assert.equal(paintHoverable(theme, "refresh", false, "dim"), "<dim>refresh</dim>");
	assert.equal(paintHoverable(theme, "refresh", true, "dim"), `<${HOVER_ROLE}>refresh</${HOVER_ROLE}>`, "hover always wins over the idle role");
});
