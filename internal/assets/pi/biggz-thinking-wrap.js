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
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
// pi-pretty termWidth caps at 210 - 4 = 206ish; thinking wraps there natively.
export default function biggzThinkingWrap(pi) {
	pi.on("session_start", async (_event, ctx) => {
		if (ctx.mode !== "tui") return;
		// Gentle-pi style: thinking uses Markdown streaming:true with muted/blue.
		// We keep it visible but hint that Ctrl+T collapses like tools Ctrl+O.
		const hint = ctx.ui?.theme
			? ctx.ui.theme.fg("muted", "thinking: wrap on \u2014 Ctrl+T to collapse/expand")
			: "thinking: wrap on \u2014 Ctrl+T to collapse/expand";
		// Use status line briefly so it matches pi-pretty's FFF indexed hint pattern.
		try {
			ctx.ui?.setStatus?.("biggz-thinking-wrap", hint);
			setTimeout(() => ctx.ui?.setStatus?.("biggz-thinking-wrap", undefined), 3500);
		} catch (_) {
			// no-op if UI not ready
		}
	});

	// Optional command to toggle thinking visibility explicitly.
	pi.registerCommand("thinking-wrap", {
		description: "Toggle thinking wrap hint (Ctrl+T collapses/expands thinking like Ctrl+O for tools)",
		handler: async (_args, ctx) => {
			ctx.ui.notify("Thinking wraps at terminal width. Press Ctrl+T to collapse/expand (like Ctrl+O for tools).", "info");
		},
	});
}
