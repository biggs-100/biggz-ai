// Ported from gentle-pi 3.7.0 tests/terminal-theme.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged.
import assert from "node:assert/strict";
import test from "node:test";
import { sanitizeTerminalText, stripAnsi } from "./terminal-theme.ts";

test("sanitizeTerminalText removes user-controlled ANSI and control characters", () => {
	assert.equal(
		sanitizeTerminalText("safe\x1b[31mred\x1b[0m\x1b]52;c;Zm9v\u0007text"),
		"saferedtext",
	);
});

test("sanitizeTerminalText removes unterminated OSC payloads", () => {
	assert.equal(sanitizeTerminalText("safe\x1b]52;c;Zm9vtext"), "safe");
	assert.equal(sanitizeTerminalText("safe\x9d52;c;Zm9vtext"), "safe");
});

test("stripAnsi removes OSC payloads and ANSI styling", () => {
	assert.equal(
		stripAnsi("safe \x1b]52;c;Zm9v\u0007\x1b[31mred\x1b[0m"),
		"safe red",
	);
});

test("sanitizeTerminalText removes CSI and single-character escapes from hostile titles", () => {
	assert.equal(sanitizeTerminalText("a\x1b[1;31;44mBIG\x1b[0m c\x1bMD\x9b2Ke"), "aBIG cDe");
});
