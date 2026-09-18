/**
 * biggz-subagent-width.test.mjs — S4 width regression harness (tasks 6.1 + 6.2).
 *
 * Guards the width contract that killed pi on 2026-09-17 (j0k3r #25): a completion
 * card line of 188 code points carrying exactly two `✅` measures 190 terminal cells
 * (code-point math under-measures exactly the 2-cell graphemes), and the fail-closed
 * pi-core width guard aborted the process when the emitted line exceeded the width.
 *
 * Covered here:
 *   - fixture families `✅`, CJK, Hangul, ZWJ sequences, ANSI/OSC and combining marks;
 *   - the exact crash shape, reconstructed deterministically (no hardcoded log line):
 *     188 code points, exactly two `✅`, `visibleWidth === 190`, rendered ≤ width for
 *     EVERY width 20–200;
 *   - property: the measurement the runtime resolves (`@earendil-works/pi-tui`, real
 *     install or test-only oracle) and the oracle fallback never UNDER-measure the real
 *     pi-tui (`measure(line) >= piTui.visibleWidth(line)`);
 *   - resolver fidelity: oracle ≡ real pi-tui on every fixture family (when installed);
 *   - source scan: width measurement across `internal/assets/pi/*.js` happens ONLY via
 *     pi-tui imports — independent width math (e.g. `[...str].length`) is flagged.
 *
 * `test/pi-tui-resolver.mjs` (real install → oracle fallback) is installed before the
 * runtime links its pi-tui import, exactly like `biggz-subagent-runtime.test.mjs`.
 */
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { installPiTuiResolver, resolvePiTuiEntry, PI_TUI_SPECIFIER } from "./test/pi-tui-resolver.mjs";

const resolution = installPiTuiResolver();
const tui = await import(PI_TUI_SPECIFIER);
const realEntry = resolvePiTuiEntry(process.env);
const realTui = realEntry ? await import(pathToFileURL(realEntry.file).href) : null;
const oracle = await import("./test/pi-tui-oracle.mjs");
const { completionLine, renderCompletion } = await import("./biggz-subagent-runtime.js");

const PI_DIR = fileURLToPath(new URL(".", import.meta.url));
// The crash guard window: every integer width from 20 to 200 inclusive.
const WIDTH_SWEEP = Object.freeze(Array.from({ length: 181 }, (_, index) => 20 + index));

const FIXTURES = Object.freeze([
	{ name: "ascii baseline", line: "plain ascii baseline" },
	{ name: "emoji ✅ single", line: "✅ done" },
	{ name: "emoji ✅ pair", line: "✅✅" },
	{ name: "emoji ✅ card text", line: "sdd-apply ✅ completed · 23.0s ✅" },
	{ name: "CJK", line: "你好世界 CJK" },
	{ name: "CJK + ✅ mixed", line: "汉字与✅ mixed 汉字" },
	{ name: "Hangul", line: "한글 Hangul" },
	{ name: "Hangul + ✅", line: "가나다라 ✅" },
	{ name: "ZWJ family", line: "👨‍👩‍👧‍👦 ZWJ family" },
	{ name: "ZWJ flag", line: "🏳️‍🌈 ZWJ flag" },
	{ name: "ZWJ profession + skin tone", line: "👩🏽‍💻 ZWJ profession skin tone" },
	{ name: "combining mark", line: "e\u0301 combining" },
	{ name: "ANSI SGR", line: "\u001b[31mred\u001b[0m reset" },
	{ name: "ANSI 256-color", line: "\u001b[1;38;5;208m256-color\u001b[0m" },
	{ name: "OSC 8 hyperlink", line: "\u001b]8;;https://example.com\u0007OSC8 link\u001b]8;;\u0007" },
	{ name: "OSC title", line: "\u001b]0;title\u0007OSC title" },
]);

// The measurement the runtime's render path trusts (its pi-tui import, resolved by
// `test/pi-tui-resolver.mjs`: real install first, test-only oracle otherwise).
const moduleMeasure = (text) => tui.visibleWidth(text);
// The authoritative measurement: the real pi-tui when resolvable, else the oracle.
const referenceMeasure = (text) => (realTui ?? oracle).visibleWidth(text);

/**
 * Deterministic reconstruction of the crash line — no hardcoded fixture string:
 * `completionLine` derives the prefix, the summary is padded so the total is exactly
 * 188 code points with exactly two `✅` (the `✅` glyph + one summary emoji), which is
 * exactly the shape whose true cell width is 190 (> 188).
 */
function crashShape() {
	const prefix = completionLine({ agent: "sdd-apply", state: "completed", elapsedMs: 23000, summary: "" });
	const summary = `✅ ${"x".repeat(188 - [...prefix].length - 3 - 2)}`; // 3 = " · " join, 2 = "✅ "
	const run = { agent: "sdd-apply", state: "completed", elapsedMs: 23000, summary };
	return { run, line: completionLine(run) };
}

describe("S4 width harness: fixtures and the exact crash shape", () => {
	it("resolves a real pi-tui or the test-only oracle before the runtime imports it", () => {
		assert.match(resolution.mode, /^install:|^oracle$/, `resolution mode: ${resolution.mode}`);
		assert.equal(typeof tui.visibleWidth, "function");
		assert.equal(typeof tui.truncateToWidth, "function");
		assert.equal(typeof oracle.visibleWidth, "function");
	});

	it("reconstructs the crash shape deterministically: 188 code points, two ✅, true width 190", () => {
		const { line } = crashShape();
		assert.equal([...line].length, 188, "the fixture must be exactly 188 code points (code-point math calls it 188)");
		assert.equal((line.match(/✅/g) ?? []).length, 2, "the line must carry exactly two ✅");
		assert.equal(moduleMeasure(line), 190, "the module's measurement must see 190 cells (188 + two 2-cell ✅)");
		assert.equal(referenceMeasure(line), 190, "the authoritative measurement must see 190 cells");
	});

	it("renders the crash shape ≤ width for EVERY width 20–200", () => {
		const { run, line } = crashShape();
		assert.ok(referenceMeasure(line) > 188, "precondition: the raw line overflows the exact crash width");
		for (const width of WIDTH_SWEEP) {
			const rendered = renderCompletion(run, width);
			assert.ok(referenceMeasure(rendered) <= width, `true width ${referenceMeasure(rendered)} > ${width} at width ${width}`);
			assert.ok(moduleMeasure(rendered) <= width, `module width ${moduleMeasure(rendered)} > ${width} at width ${width}`);
		}
		assert.ok(referenceMeasure(renderCompletion(run, 188)) <= 188, "the exact crash width (188) must fit");
	});

	it("renders every fixture family ≤ width for EVERY width 20–200", () => {
		for (const { name, line } of FIXTURES) {
			const run = { agent: "probe", state: "completed", elapsedMs: 1234, summary: line };
			for (const width of WIDTH_SWEEP) {
				const rendered = renderCompletion(run, width);
				assert.ok(referenceMeasure(rendered) <= width, `fixture "${name}" rendered ${referenceMeasure(rendered)} > ${width} at width ${width}`);
			}
		}
	});
});

describe("S4 width property: never under-measure (measure ≥ real pi-tui)", () => {
	it("moduleMeasure(line) >= piTui.visibleWidth(line) for every fixture line and the crash line", (t) => {
		if (!realTui) {
			t.diagnostic("real @earendil-works/pi-tui not resolvable — the ≥ reference is the oracle itself here");
		} else {
			t.diagnostic(`real pi-tui: ${realEntry.file}`);
		}
		const lines = [...FIXTURES.map(({ line }) => line), crashShape().line];
		for (const line of lines) {
			const reference = referenceMeasure(line);
			assert.ok(moduleMeasure(line) >= reference, `module measurement ${moduleMeasure(line)} < reference ${reference} for ${JSON.stringify(line)} (never under-measure)`);
			assert.ok(oracle.visibleWidth(line) >= reference, `oracle fallback ${oracle.visibleWidth(line)} < reference ${reference} for ${JSON.stringify(line)} (the fallback must never under-measure)`);
		}
	});
});

describe("S4 resolver fidelity against the real pi-tui", () => {
	it("the oracle matches the real pi-tui on every fixture family when installed", (t) => {
		if (!realTui) {
			t.skip("real @earendil-works/pi-tui not installed — fidelity needs the real package");
			return;
		}
		const lines = [...FIXTURES.map(({ name, line }) => ({ name, line })), { name: "crash shape", line: crashShape().line }];
		for (const { name, line } of lines) {
			assert.equal(oracle.visibleWidth(line), realTui.visibleWidth(line), `visibleWidth diverges for "${name}": ${JSON.stringify(line)}`);
			for (const width of [20, 63, 80, 120, 188, 200]) {
				const mine = oracle.truncateToWidth(line, width, "…");
				const theirs = realTui.truncateToWidth(line, width, "…");
				assert.equal(oracle.visibleWidth(mine), realTui.visibleWidth(theirs), `truncateToWidth diverges for "${name}" at width ${width}`);
			}
		}
	});
});

// ── source scan (task 6.2): width measurement only via pi-tui imports ──

// Independent width math shapes. Each one re-implements the measurement that already
// lives in `@earendil-works/pi-tui` — the exact defect class behind the #25 crash.
const NAIVE_WIDTH_PATTERNS = Object.freeze([
	["spread-length measurement ([...str].length)", /\[\.\.\.[^\]\n]{1,120}\]\.length/],
	["Array.from(...).length measurement", /Array\.from\([^)\n]{1,120}\)\.length/],
	["codepoint width loop", /\.codePointAt\(0\)/],
	["spread-iteration width loop (for … of [...str])", /for \(const \w+ of \[\.\.\./],
	["local width/truncate function", /function (?:visibleWidth|visibleTextWidth|truncateToWidth|stringWidth|textWidth|charWidth|displayWidth)\b/],
]);

// Files allowed to keep their own width math, each with the reason it is exempt.
// This is a RATCHET: new files (like the subagent runtime) must import from pi-tui;
// adding an entry here requires a conscious review decision, and removing a file must
// remove its stale entry (asserted below).
const WIDTH_MATH_ALLOWLIST = new Map([
	["pi-tui-oracle.mjs", "test-only pi-tui substitute — it IS the fallback measurement implementation"],
	["pi-tui-resolver.mjs", "specifier resolver paired with the oracle (no width math of its own)"],
	["biggz-footer.js", "pre-existing S4 baseline: prefers Bun.stringWidth, codepoint fallback documented; untouched by this change"],
	["biggz-question-mouse.js", "pre-existing S4 baseline: documented ASCII-art fallback; untouched by this change"],
	["biggz-pi-pretty.js", "pre-existing S4 baseline: retired asset (no deployer since the S3 cutover), deletion follow-up recorded"],
]);

const detectNaiveWidthMath = (source) => {
	const hits = [];
	for (const [label, pattern] of NAIVE_WIDTH_PATTERNS) {
		const match = source.match(pattern);
		if (!match) continue;
		hits.push({ label, sample: match[0].slice(0, 60), line: source.slice(0, match.index).split("\n").length });
	}
	return hits;
};

const scanOffenders = (files) => {
	const offenders = [];
	for (const name of files) {
		if (WIDTH_MATH_ALLOWLIST.has(name)) continue;
		const source = fs.readFileSync(path.join(PI_DIR, name), "utf8");
		for (const hit of detectNaiveWidthMath(source)) {
			offenders.push(`${name}:${hit.line} — ${hit.label} (${JSON.stringify(hit.sample)})`);
		}
	}
	return offenders;
};

describe("S4 source scan: width measurement only via pi-tui imports", () => {
	const jsFiles = fs.readdirSync(PI_DIR).filter((name) => name.endsWith(".js")).sort();

	it("has a non-empty asset corpus including the module under test", () => {
		assert.ok(jsFiles.includes("biggz-subagent-runtime.js"), `corpus must include the runtime: ${jsFiles.join(", ")}`);
	});

	it("flags independent width math outside the recorded baseline with a clear message", () => {
		const offenders = scanOffenders(jsFiles);
		assert.deepEqual(
			offenders,
			[],
			`independent width math found — route it through @earendil-works/pi-tui (visibleWidth/truncateToWidth) instead of re-implementing it:\n${offenders.join("\n")}`,
		);
	});

	it("scanner self-test: catches the #25 defect class and stays silent on the pi-tui import path", () => {
		const naive = `const width = [...stripAnsi(text)].length;\nfunction visibleWidth(s) { return s.length; }`;
		const hits = detectNaiveWidthMath(naive);
		assert.ok(hits.some((hit) => /spread-length/.test(hit.label)), "the scanner must catch [...str].length");
		assert.ok(hits.some((hit) => /local width/.test(hit.label)), "the scanner must catch a local width function");
		assert.deepEqual(detectNaiveWidthMath(`import { truncateToWidth } from "@earendil-works/pi-tui";`), [], "pi-tui imports are clean");
	});

	it("the runtime measures only through its pi-tui import (positive + negative)", () => {
		const source = fs.readFileSync(path.join(PI_DIR, "biggz-subagent-runtime.js"), "utf8");
		assert.match(source, /import\s*\{[^}]*\btruncateToWidth\b[^}]*\}\s*from\s*"@earendil-works\/pi-tui"/, "the runtime must import truncateToWidth from @earendil-works/pi-tui");
		for (const [label, pattern] of NAIVE_WIDTH_PATTERNS) {
			assert.ok(!pattern.test(source), `the runtime must not contain ${label}`);
		}
	});

	it("keeps the allowlist honest: every recorded exempt file still exists", () => {
		for (const name of WIDTH_MATH_ALLOWLIST.keys()) {
			const target = name.endsWith(".mjs") ? path.join(PI_DIR, "test", name) : path.join(PI_DIR, name);
			assert.ok(fs.existsSync(target), `allowlist entry ${name} is stale — remove it (the ratchet must stay tight)`);
		}
	});
});
