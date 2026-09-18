/**
 * pi-tui-oracle — test-only fallback for `@earendil-works/pi-tui` (S1b, task 3.7).
 *
 * Resolved by `pi-tui-resolver.mjs` ONLY when no real pi-tui install is reachable,
 * so the runtime test suite runs on machines without pi. Implements the cell-aware
 * subset the runtime renders with — `visibleWidth` / `truncateToWidth` — mirroring
 * pi-tui 0.85.1 semantics for the S4 fixture families (ASCII, emoji, CJK, Hangul,
 * ZWJ, ANSI, combining marks). The real pi-tui stays authoritative: the fidelity
 * test in `biggz-subagent-runtime.test.mjs` compares both implementations whenever
 * an install is available. No runtime code may use this module directly.
 */

// CSI/OSC/escape sequences (same family pi-tui strips before measuring).
const ANSI_RE = /[\u001B\u009B][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[-a-zA-Z\d/#&.:=?%@~_]*)*)?\u0007)|(?:(?:\d{1,4}(?:;\d{0,4})*)?[\dA-PR-TZcf-nq-uy=><~]))/g;
const CONTROL_RE = /[\u0000-\u0008\u000B-\u001F\u007F]/g;
const ZERO_WIDTH_RE = /[\u00AD\u200B-\u200F\u2028-\u202E\u2060-\u2064\uFEFF]/;
const SEGMENTER = new Intl.Segmenter("en", { granularity: "grapheme" });

const isCombining = (cp) =>
	(cp >= 0x300 && cp <= 0x36f) ||
	(cp >= 0x1ab0 && cp <= 0x1aff) ||
	(cp >= 0x1dc0 && cp <= 0x1dff) ||
	(cp >= 0x20d0 && cp <= 0x20ff) ||
	(cp >= 0xfe00 && cp <= 0xfe0f) ||
	(cp >= 0xfe20 && cp <= 0xfe2f);

// East-Asian-wide / emoji ranges covering the harness fixtures (subset of get-east-asian-width).
const isWide = (cp) =>
	(cp >= 0x1100 && cp <= 0x115f) ||
	cp === 0x2329 ||
	cp === 0x232a ||
	(cp >= 0x2e80 && cp <= 0xa4cf && cp !== 0x303f) ||
	(cp >= 0xac00 && cp <= 0xd7a3) ||
	(cp >= 0xf900 && cp <= 0xfaff) ||
	(cp >= 0xfe30 && cp <= 0xfe6f) ||
	(cp >= 0xff00 && cp <= 0xff60) ||
	(cp >= 0xffe0 && cp <= 0xffe6) ||
	(cp >= 0x231a && cp <= 0x231b) ||
	(cp >= 0x2600 && cp <= 0x27bf) ||
	(cp >= 0x2b1b && cp <= 0x2b1c) ||
	cp === 0x2b50 ||
	cp === 0x2b55 ||
	(cp >= 0x1f000 && cp <= 0x1faff);

export function stripTerminalSequences(str) {
	return String(str ?? "").replace(ANSI_RE, "").replace(CONTROL_RE, "");
}

function graphemes(text) {
	return [...SEGMENTER.segment(text)].map((part) => part.segment);
}

function graphemeWidth(segment) {
	let width = 0;
	for (const ch of segment) {
		const cp = ch.codePointAt(0);
		if (ch === "\u200d" || ZERO_WIDTH_RE.test(ch) || isCombining(cp)) continue;
		width = Math.max(width, isWide(cp) ? 2 : 1);
	}
	return width;
}

export function visibleWidth(str) {
	const text = stripTerminalSequences(str);
	let total = 0;
	for (const segment of graphemes(text)) total += graphemeWidth(segment);
	return total;
}

function sliceByWidth(text, limit) {
	let out = "";
	let used = 0;
	for (const segment of graphemes(text)) {
		const width = graphemeWidth(segment);
		if (used + width > limit) break;
		out += segment;
		used += width;
	}
	return out;
}

export function truncateToWidth(text, maxWidth, ellipsis = "...", pad = false) {
	const limit = Math.floor(Number(maxWidth) || 0);
	const raw = stripTerminalSequences(text);
	if (limit <= 0) return "";
	const width = visibleWidth(raw);
	if (width <= limit) return pad ? raw + " ".repeat(limit - width) : raw;
	const ellipsisWidth = visibleWidth(ellipsis);
	if (ellipsisWidth >= limit) {
		// Mirror pi-tui: clip the ellipsis itself when it cannot fit alongside text.
		const clipped = sliceByWidth(ellipsis, limit);
		if (!clipped) return pad ? " ".repeat(limit) : "";
		return pad ? clipped + " ".repeat(limit - visibleWidth(clipped)) : clipped;
	}
	return sliceByWidth(raw, limit - ellipsisWidth) + ellipsis;
}
