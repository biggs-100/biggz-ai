/**
 * biggz-thinking-wrap — pi thinking wrap + collapsible.
 *
 * Pi renders thinking via assistant-message Markdown (italic, thinkingText color)
 * at terminal width. This extension makes the UX match pi-pretty's tool results:
 *   - thinking wraps at termWidth (handled natively by pi-tui Markdown)
 *   - collapsed preview shows a single line with expand hint
 *   - toggle with Ctrl+T (app.thinking.toggle) like tools use Ctrl+O (app.tools.expand)
 *
 * Pi already supports collapsing via settings.hideThinkingBlock and the
 * Ctrl+T keybinding. This extension ensures the behavior is discoverable:
 *   - shows status hint on session_start
 *   - logs activation for verification (biggz install --agent pi)
 *
 * Keep as gentle-pi (single column, theme colors) but with wrap desplegable.
 * See: assistant-message.js -> Markdown(thinkingBlocks, outputPad) with
 * theme.fg("thinkingText", ...) and termWidth() from pi-tui.
 * Pi parity slice: thinkingLevel palette mapping (minimal→low→medium→high→max to ColorThinkingMinimal…Max)
 * and SepPresets per level — thinking block border uses sep powerline.
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
// pi-pretty termWidth caps at 210 - 4 = 206ish; thinking wraps there natively.

// ── Thinking level palette (mirrors internal/tui/styles/theme.go DarkPalette + LightPalette fallback) ──
export const THINKING_LEVELS = Object.freeze(["off", "minimal", "low", "medium", "high", "xhigh", "max"]);

export const THINKING_LEVEL_COLORS = Object.freeze({
	off: "#3d424a",
	minimal: "#5f6673",
	low: "#178fb9",
	medium: "#0088fa",
	high: "#b281d6",
	xhigh: "#e5c1ff",
	max: "#e5c1ff",
});

// Fallback hex per level when theme.getThinkingBorderColor unavailable (mirrors theme.go GetThinkingBorderColor)
export function getThinkingHexFallback(level) {
	return THINKING_LEVEL_COLORS[level] || THINKING_LEVEL_COLORS.off;
}

function hexToRgb(hex) {
	const h = String(hex || "").replace(/^#/, "");
	if (h.length === 3) return [parseInt(h[0] + h[0], 16), parseInt(h[1] + h[1], 16), parseInt(h[2] + h[2], 16)];
	if (h.length === 6) return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
	return [95, 102, 115];
}

function ansiFgFromHex(hex) {
	const [r, g, b] = hexToRgb(hex);
	return `\x1b[38;2;${r};${g};${b}m`;
}

// Use theme.getThinkingBorderColor(level) if available, else fallback hex wrapper
export function getThinkingBorderColor(level, theme) {
	if (theme && typeof theme.getThinkingBorderColor === "function") {
		try {
			const fn = theme.getThinkingBorderColor(level);
			if (typeof fn === "function") return fn;
			// some themes return ANSI string wrapper directly
			if (typeof fn === "string") {
				const s = fn;
				return (txt) => `${s}${txt}\x1b[39m`;
			}
		} catch {}
	}
	if (theme && typeof theme.fg === "function") {
		// map level to ThemeColor token
		const tokenMap = {
			off: "thinkingOff",
			minimal: "thinkingMinimal",
			low: "thinkingLow",
			medium: "thinkingMedium",
			high: "thinkingHigh",
			xhigh: "thinkingXhigh",
			max: "thinkingMax",
		};
		const token = tokenMap[level] || "thinkingOff";
		try {
			return (txt) => theme.fg(token, txt);
		} catch {}
	}
	const hex = getThinkingHexFallback(level);
	const ansi = ansiFgFromHex(hex);
	return (txt) => `${ansi}${txt}\x1b[39m`;
}

// ── SepPresets per level (mirrors internal/tui/styles/theme.go SepPresets + symbols.ts) ──
export const SepPresets = Object.freeze({
	unicode: Object.freeze({ powerline: "▕", powerlineThin: "┆", block: "▌", dot: " · ", slash: " / ", pipe: " │ " }),
	nerd: Object.freeze({ powerline: "\ue0b0", powerlineThin: "\ue0b1", block: "█", dot: " · ", slash: "\ue0bb", pipe: "\ue0b3" }),
	ascii: Object.freeze({ powerline: ">", powerlineThin: ">", block: "#", dot: " - ", slash: " / ", pipe: " | " }),
});

export function getSepPreset(presetName) {
	return SepPresets[presetName] || SepPresets.unicode;
}

// Thinking block border uses sep powerline (per spec: "ensure thinking-level border uses sep powerline")
export function getThinkingSep(presetName, level) {
	const preset = getSepPreset(presetName);
	// minimal/low use thin variant for subtler border; medium+ use solid powerline
	if (level === "minimal" || level === "low") return preset.powerlineThin;
	return preset.powerline;
}

export function getThinkingBorder(level, theme, presetName) {
	const colorFn = getThinkingBorderColor(level, theme);
	const sep = getThinkingSep(presetName, level);
	return { colorFn, sep, level, hex: getThinkingHexFallback(level) };
}

// Render helper: wraps thinking content with thinking-level colored border using sep powerline
export function wrapThinkingBlock(content, level, theme, presetName, width) {
	const { colorFn, sep } = getThinkingBorder(level, theme, presetName);
	const lines = String(content || "").split("\n");
	const border = colorFn(sep);
	// Each line prefixed with colored sep; width trimming left to Markdown
	return lines.map((l) => `${border} ${l}`).join("\n");
}

export default function biggzThinkingWrap(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	// expose helpers for tests / verification
	try {
		pi._biggzThinkingWrap = {
			THINKING_LEVELS,
			THINKING_LEVEL_COLORS,
			getThinkingHexFallback,
			getThinkingBorderColor,
			SepPresets,
			getSepPreset,
			getThinkingSep,
			getThinkingBorder,
			wrapThinkingBlock,
		};
		if (pi._biggzExtension) {
			pi._biggzExtension.thinking = {
				THINKING_LEVELS,
				THINKING_LEVEL_COLORS,
				getThinkingBorderColor,
				SepPresets,
				getThinkingSep,
				getThinkingBorder,
				wrapThinkingBlock,
			};
		}
	} catch {}
	pi.on("session_start", async (_event, ctx) => {
		if (ctx.mode !== "tui") return;
		// Gentle-pi style: thinking uses Markdown streaming:true with muted/blue.
		// We keep it visible but hint that Ctrl+T collapses like tools Ctrl+O.
		const hintBase = "thinking: wrap on — Ctrl+T to collapse/expand";
		const hint = ctx.ui?.theme
			? ctx.ui.theme.fg("muted", hintBase)
			: hintBase;
		// Use status line briefly so it matches pi-pretty's FFF indexed hint pattern.
		try {
			ctx.ui?.setStatus?.("biggz-thinking-wrap", hint);
			setTimeout(() => ctx.ui?.setStatus?.("biggz-thinking-wrap", undefined), 3500);
		} catch (_) {
			// no-op if UI not ready
		}
		// Apply thinking-level border coloring if theme available
		try {
			const theme = ctx.ui?.theme;
			const level = ctx.session?.state?.thinkingLevel || _event?.thinkingLevel || "medium";
			if (theme && THINKING_LEVELS.includes(level)) {
				const border = getThinkingBorder(level, theme, theme.symbolPreset || theme.getSymbolPreset?.() || "unicode");
				// Tag UI with current thinking border for downstream Markdown rendering
				if (ctx.ui) ctx.ui._biggzThinkingBorder = border;
			}
		} catch {}
	});

	// Optional: handle thinking block events if pi emits them — apply level-aware border
	try {
		pi.on("thinking", async (event, ctx) => {
			if (process.env.BIGGZ_PRETTY === "0") return;
			const level = event?.level || event?.thinkingLevel || ctx?.session?.state?.thinkingLevel || "medium";
			const theme = ctx?.ui?.theme;
			const preset = theme?.symbolPreset || theme?.getSymbolPreset?.() || "unicode";
			const border = getThinkingBorder(level, theme, preset);
			try { if (ctx?.ui) ctx.ui._biggzThinkingBorder = border; } catch {}
			// No block — just observation for status/metadata
		});
	} catch {}

	// Optional command to toggle thinking visibility explicitly.
	pi.registerCommand("thinking-wrap", {
		description: "Toggle thinking wrap hint (Ctrl+T collapses/expands thinking like Ctrl+O for tools)",
		handler: async (_args, ctx) => {
			ctx.ui.notify("Thinking wraps at terminal width. Press Ctrl+T to collapse/expand (like Ctrl+O for tools).", "info");
		},
	});
}
