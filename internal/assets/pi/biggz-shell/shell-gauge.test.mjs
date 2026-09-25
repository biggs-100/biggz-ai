// Focused suite for the gauge primitives (no dedicated upstream test file).
// Covers the gauge math, the tone thresholds, and the two-role paint.
import assert from "node:assert/strict";
import test from "node:test";
import { GAUGE_CELLS, gaugeTone, paintGauge, renderGauge } from "./shell-gauge.ts";

const taggedTheme = {
	fg(color, text) {
		return `<${color}>${text}</${color}>`;
	},
};

test("renderGauge maps a percentage onto filled and empty cells", () => {
	assert.equal(GAUGE_CELLS, 8);
	assert.equal(renderGauge(null), "▱▱▱▱▱▱▱▱");
	assert.equal(renderGauge(0), "▱▱▱▱▱▱▱▱");
	assert.equal(renderGauge(50), "▰▰▰▰▱▱▱▱");
	assert.equal(renderGauge(100), "▰▰▰▰▰▰▰▰");
	assert.equal(renderGauge(12), "▰▱▱▱▱▱▱▱");
});

test("renderGauge clamps out-of-range input and honours a custom cell count", () => {
	assert.equal(renderGauge(-20), "▱▱▱▱▱▱▱▱");
	assert.equal(renderGauge(400), "▰▰▰▰▰▰▰▰");
	assert.equal(renderGauge(50, 4), "▰▰▱▱");
	assert.equal(renderGauge(50, 0), "");
});

test("gaugeTone selects accent, warning, error, and dim at the boundaries", () => {
	assert.equal(gaugeTone(null), "dim");
	assert.equal(gaugeTone(0), "accent");
	assert.equal(gaugeTone(79), "accent");
	assert.equal(gaugeTone(80), "warning");
	assert.equal(gaugeTone(94), "warning");
	assert.equal(gaugeTone(95), "error");
	assert.equal(gaugeTone(100), "error");
});

test("paintGauge paints the filled run in the tone and the empty run in the border role", () => {
	assert.equal(paintGauge(50, taggedTheme), "<accent>▰▰▰▰</accent><border>▱▱▱▱</border>");
	assert.equal(paintGauge(96, taggedTheme), "<error>▰▰▰▰▰▰▰▰</error><border></border>");
	assert.equal(paintGauge(null, taggedTheme), "<dim></dim><border>▱▱▱▱▱▱▱▱</border>");
	assert.equal(paintGauge(50, taggedTheme, 4), "<accent>▰▰</accent><border>▱▱</border>");
});
