/**
 * pi-tui-oracle — test-only fallback for `@earendil-works/pi-tui` (S1b, task 3.7).
 *
 * Resolved by `pi-tui-resolver.mjs` ONLY when no real pi-tui install is reachable,
 * so the runtime test suite runs on machines without pi. Implements the cell-aware
 * subset the runtime and the ported shell modules render with — `visibleWidth`,
 * `truncateToWidth` and `wrapTextWithAnsi` — mirroring pi-tui 0.85.1 semantics for
 * the S4 fixture families (ASCII, emoji, CJK, Hangul, ZWJ, ANSI, combining marks).
 * The real pi-tui stays authoritative: the fidelity
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

// pi-tui resolves these Misc Symbols / Dingbats to width 2 (emoji presentation);
// the rest of the 0x2600-0x27bf block is width 1, so a blanket range would
// mis-measure the shell's petals (✿ ❀ ❁ ✾) and status glyphs (✓ ◐ ▰ ✎).
const WIDE_MISC_SYMBOLS = new Set([
	0x2614, 0x2615, 0x2630, 0x2631, 0x2632, 0x2633, 0x2634, 0x2635, 0x2636, 0x2637,
	0x2648, 0x2649, 0x264a, 0x264b, 0x264c, 0x264d, 0x264e, 0x264f, 0x2650, 0x2651,
	0x2652, 0x2653, 0x267f, 0x268a, 0x268b, 0x268c, 0x268d, 0x268e, 0x268f, 0x2693,
	0x26a1, 0x26aa, 0x26ab, 0x26bd, 0x26be, 0x26c4, 0x26c5, 0x26ce, 0x26d4, 0x26ea,
	0x26f2, 0x26f3, 0x26f5, 0x26fa, 0x26fd, 0x2705, 0x270a, 0x270b, 0x2728, 0x274c,
	0x274e, 0x2753, 0x2754, 0x2755, 0x2757, 0x2795, 0x2796, 0x2797, 0x27b0, 0x27bf,
]);

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
	WIDE_MISC_SYMBOLS.has(cp) ||
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

// Word wrap that preserves ANSI codes, mirroring pi-tui's wrapTextWithAnsi:
// only breaks at whitespace, hard-breaks a token wider than the limit, and
// never pads. The resolver supplies this symbol to the ported shell modules.
const ANSI_CODE_RE = /\u001B\[[0-9;?]*[ -/]*[@-~]|\u001B\][\s\S]*?(?:\u0007|\u001B\\|\u009C|$)|\u001B[@-Z\\-_]|\u009B[0-9;?]*[ -/]*[@-~]/y;

function ansiCodeAt(text, index) {
	ANSI_CODE_RE.lastIndex = index;
	const match = ANSI_CODE_RE.exec(text);
	return match && match.index === index ? match[0] : null;
}

function wrapTokens(line) {
	const tokens = [];
	let pendingAnsi = "";
	let index = 0;
	while (index < line.length) {
		const ansi = ansiCodeAt(line, index);
		if (ansi) {
			pendingAnsi += ansi;
			index += ansi.length;
			continue;
		}
		let end = index;
		while (end < line.length && !ansiCodeAt(line, end)) end += 1;
		for (const { segment } of SEGMENTER.segment(line.slice(index, end))) {
			const space = segment === " ";
			const text = pendingAnsi + segment;
			pendingAnsi = "";
			const last = tokens[tokens.length - 1];
			if (last && last.space === space) last.text += text;
			else tokens.push({ space, text });
		}
		index = end;
	}
	if (pendingAnsi) {
		const last = tokens[tokens.length - 1];
		if (last) last.text += pendingAnsi;
		else tokens.push({ space: false, text: pendingAnsi });
	}
	return tokens;
}

function breakToken(token, width) {
	const parts = [];
	let current = "";
	let used = 0;
	let index = 0;
	while (index < token.length) {
		const ansi = ansiCodeAt(token, index);
		if (ansi) {
			current += ansi;
			index += ansi.length;
			continue;
		}
		const segment = graphemes(token.slice(index))[0] ?? token[index];
		const segmentWidth = graphemeWidth(segment);
		if (used > 0 && used + segmentWidth > width) {
			parts.push(current);
			current = "";
			used = 0;
		}
		current += segment;
		used += segmentWidth;
		index += segment.length;
	}
	if (current) parts.push(current);
	return parts.length > 0 ? parts : [""];
}

function wrapSingleLine(line, width) {
	if (!line) return [""];
	if (visibleWidth(line) <= width) return [line];
	const lines = [];
	let current = "";
	let currentWidth = 0;
	for (const token of wrapTokens(line)) {
		const tokenWidth = visibleWidth(token.text);
		if (tokenWidth > width && !token.space) {
			if (current) {
				lines.push(current.trimEnd());
				current = "";
				currentWidth = 0;
			}
			const broken = breakToken(token.text, width);
			for (let i = 0; i < broken.length - 1; i += 1) lines.push(broken[i]);
			current = broken[broken.length - 1];
			currentWidth = visibleWidth(current);
			continue;
		}
		if (currentWidth > 0 && currentWidth + tokenWidth > width) {
			lines.push(current.trimEnd());
			if (token.space) {
				current = "";
				currentWidth = 0;
			} else {
				current = token.text;
				currentWidth = tokenWidth;
			}
		} else {
			current += token.text;
			currentWidth += tokenWidth;
		}
	}
	if (current) lines.push(current.trimEnd());
	return lines.length > 0 ? lines.map((wrapped) => wrapped.trimEnd()) : [""];
}

export function wrapTextWithAnsi(text, width) {
	if (!text) return [""];
	const limit = Math.max(1, Math.floor(Number(width) || 0));
	const result = [];
	for (const inputLine of String(text).split(/\r\n|\r|\n/)) {
		for (const wrapped of wrapSingleLine(inputLine, limit)) result.push(wrapped);
	}
	return result.length > 0 ? result : [""];
}

export function truncateToWidth(text, maxWidth, ellipsis = "...", pad = false) {
	const limit = Math.floor(Number(maxWidth) || 0);
	const raw = stripTerminalSequences(text);
	if (limit <= 0) return "";
	const width = visibleWidth(raw);
	if (width <= limit) return pad ? text + " ".repeat(limit - width) : text;
	const ellipsisWidth = visibleWidth(ellipsis);
	if (ellipsisWidth >= limit) {
		// Mirror pi-tui: clip the ellipsis itself when it cannot fit alongside text.
		const clipped = sliceByWidth(ellipsis, limit);
		if (!clipped) return pad ? " ".repeat(limit) : "";
		return pad ? clipped + " ".repeat(limit - visibleWidth(clipped)) : clipped;
	}
	return sliceByWidth(raw, limit - ellipsisWidth) + ellipsis;
}

// ---------------------------------------------------------------------------
// Key / matchesKey — the legacy-sequence subset of pi-tui's keys.ts.
//
// The ported shell modules match input by key identifier (`Key.escape`,
// `matchesKey(data, Key.up)`) instead of comparing raw bytes, so the oracle
// offers the same API for the fallback path. It implements the legacy terminal
// sequences and printable keys with their ctrl/alt/shift forms; the Kitty
// keyboard protocol and the modifyOtherKeys encoding are not part of the
// fallback, so a sequence this table does not name simply does not match.
// ---------------------------------------------------------------------------

export const Key = Object.freeze({
	escape: "escape",
	esc: "esc",
	enter: "enter",
	return: "return",
	tab: "tab",
	space: "space",
	backspace: "backspace",
	delete: "delete",
	insert: "insert",
	clear: "clear",
	home: "home",
	end: "end",
	pageUp: "pageUp",
	pageDown: "pageDown",
	up: "up",
	down: "down",
	left: "left",
	right: "right",
	f1: "f1", f2: "f2", f3: "f3", f4: "f4", f5: "f5", f6: "f6",
	f7: "f7", f8: "f8", f9: "f9", f10: "f10", f11: "f11", f12: "f12",
	backtick: "`", hyphen: "-", equals: "=", leftbracket: "[", rightbracket: "]",
	backslash: "\\", semicolon: ";", quote: "'", comma: ",", period: ".", slash: "/",
	exclamation: "!", at: "@", hash: "#", dollar: "$", percent: "%", caret: "^",
	ampersand: "&", asterisk: "*", leftparen: "(", rightparen: ")", underscore: "_",
	plus: "+", pipe: "|", tilde: "~", leftbrace: "{", rightbrace: "}", colon: ":",
	lessthan: "<", greaterthan: ">", question: "?",
	ctrl: (key) => `ctrl+${key}`,
	shift: (key) => `shift+${key}`,
	alt: (key) => `alt+${key}`,
	super: (key) => `super+${key}`,
	ctrlShift: (key) => `ctrl+shift+${key}`,
	shiftCtrl: (key) => `shift+ctrl+${key}`,
	ctrlAlt: (key) => `ctrl+alt+${key}`,
	altCtrl: (key) => `alt+ctrl+${key}`,
	shiftAlt: (key) => `shift+alt+${key}`,
	altShift: (key) => `alt+shift+${key}`,
});

const LEGACY_KEY_SEQUENCES = {
	up: ["\x1b[A", "\x1bOA"],
	down: ["\x1b[B", "\x1bOB"],
	right: ["\x1b[C", "\x1bOC"],
	left: ["\x1b[D", "\x1bOD"],
	home: ["\x1b[H", "\x1bOH", "\x1b[1~", "\x1b[7~"],
	end: ["\x1b[F", "\x1bOF", "\x1b[4~", "\x1b[8~"],
	insert: ["\x1b[2~"],
	delete: ["\x1b[3~"],
	pageUp: ["\x1b[5~", "\x1b[[5~"],
	pageDown: ["\x1b[6~", "\x1b[[6~"],
	clear: ["\x1b[E", "\x1bOE"],
	f1: ["\x1bOP", "\x1b[11~", "\x1b[[A"],
	f2: ["\x1bOQ", "\x1b[12~", "\x1b[[B"],
	f3: ["\x1bOR", "\x1b[13~", "\x1b[[C"],
	f4: ["\x1bOS", "\x1b[14~", "\x1b[[D"],
	f5: ["\x1b[15~", "\x1b[[E"],
	f6: ["\x1b[17~"],
	f7: ["\x1b[18~"],
	f8: ["\x1b[19~"],
	f9: ["\x1b[20~"],
	f10: ["\x1b[21~"],
	f11: ["\x1b[23~"],
	f12: ["\x1b[24~"],
};

const LEGACY_SHIFT_SEQUENCES = {
	up: ["\x1b[a"],
	down: ["\x1b[b"],
	right: ["\x1b[c"],
	left: ["\x1b[d"],
	clear: ["\x1b[e"],
	insert: ["\x1b[2$"],
	delete: ["\x1b[3$"],
	pageUp: ["\x1b[5$"],
	pageDown: ["\x1b[6$"],
	home: ["\x1b[7$"],
	end: ["\x1b[8$"],
};

const LEGACY_CTRL_SEQUENCES = {
	up: ["\x1bOa"],
	down: ["\x1bOb"],
	right: ["\x1bOc"],
	left: ["\x1bOd"],
	clear: ["\x1bOe"],
	insert: ["\x1b[2^"],
	delete: ["\x1b[3^"],
	pageUp: ["\x1b[5^"],
	pageDown: ["\x1b[6^"],
	home: ["\x1b[7^"],
	end: ["\x1b[8^"],
};

function parseKeyId(keyId) {
	const parts = String(keyId).toLowerCase().split("+");
	const key = parts[parts.length - 1];
	if (!key) return null;
	return { key, ctrl: parts.includes("ctrl"), shift: parts.includes("shift"), alt: parts.includes("alt"), super: parts.includes("super") };
}

function matchesAny(data, sequences) {
	return sequences?.includes(data) === true;
}

export function matchesKey(data, keyId) {
	const parsed = parseKeyId(keyId);
	if (!parsed) return false;
	const { key, ctrl, shift, alt, super: superModifier } = parsed;
	if (superModifier) return false;
	const plain = !ctrl && !shift && !alt;

	if (key === "escape" || key === "esc") return plain && data === "\x1b";
	if (key === "enter" || key === "return") {
		if (shift) return false;
		if (ctrl) return data === "\n" || data === "\r";
		if (alt) return data === "\x1b\r" || data === "\x1b\n";
		return data === "\r" || data === "\n" || data === "\x1bOM";
	}
	if (key === "tab") {
		if (shift && !ctrl && !alt) return data === "\x1b[Z";
		return plain && data === "\t";
	}
	if (key === "space") {
		if (ctrl && !alt && !shift) return data === "\x00";
		if (alt && !ctrl && !shift) return data === "\x1b ";
		return plain && data === " ";
	}
	if (key === "backspace") {
		if (ctrl || shift) return false;
		return alt ? data === "\x1b\x7f" || data === "\x1b\b" : data === "\x7f" || data === "\x08";
	}
	if (key in LEGACY_KEY_SEQUENCES || key === "clear" || key === "insert" || key === "delete" || key === "home" || key === "end" || /^f\d{1,2}$/.test(key)) {
		if (shift) return matchesAny(data, LEGACY_SHIFT_SEQUENCES[key]);
		if (ctrl) return matchesAny(data, LEGACY_CTRL_SEQUENCES[key]);
		return plain && matchesAny(data, LEGACY_KEY_SEQUENCES[key]);
	}
	// Alt-modified printable keys arrive as ESC followed by the key.
	if (alt && !ctrl && !shift && key.length === 1) return data === `\x1b${key}`;
	// Ctrl+letter is the C0 control code (ctrl+a = \x01 ... ctrl+z = \x1a).
	if (ctrl && !shift && !alt && /^[a-z]$/.test(key)) return data === String.fromCharCode(key.charCodeAt(0) - 96);
	return plain && data === key;
}
