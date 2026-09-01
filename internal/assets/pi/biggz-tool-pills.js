/**
 * biggz-tool-pills — compact colored pill labels for Pi tools
 * Ported from tomsej/pi-ext tool-pills (MIT) as offline biggz-native extension.
 * No Shiki — uses theme tokens + ANSI for lightweight pills and collapsed output.
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */

// ── Pill definitions: tool → { label, bg, fg } using theme color keys ──
export const TOOL_PILL_MAP = Object.freeze({
	read: { label: "READ", bg: "toolPendingBg", fg: "toolTitle" },
	write: { label: "WRITE", bg: "toolSuccessBg", fg: "success" },
	edit: { label: "EDIT", bg: "toolSuccessBg", fg: "warning" },
	bash: { label: "BASH", bg: "toolPendingBg", fg: "bashMode" },
	grep: { label: "GREP", bg: "toolPendingBg", fg: "muted" },
	find: { label: "FIND", bg: "toolPendingBg", fg: "muted" },
	task: { label: "TASK", bg: "toolPendingBg", fg: "accent" },
	question: { label: "ASK", bg: "toolPendingBg", fg: "accent" },
	ask_user_question: { label: "ASK", bg: "toolPendingBg", fg: "accent" },
	subagent: { label: "AGENT", bg: "toolPendingBg", fg: "accent" },
});

export function getToolPill(toolName) {
	if (!toolName) return null;
	const key = String(toolName).toLowerCase();
	return TOOL_PILL_MAP[key] || { label: key.toUpperCase().slice(0, 6), bg: "toolPendingBg", fg: "muted" };
}

// Minimal ANSI pill: [ LABEL ] with theme-aware colors fallback to ANSI codes
function ansiPill(label, opts = {}) {
	// Use 256-color fallbacks that match titanium/dark palette
	const bgMap = {
		toolPendingBg: "\x1b[48;5;235m",
		toolSuccessBg: "\x1b[48;5;22m",
		toolErrorBg: "\x1b[48;5;52m",
	};
	const fgMap = {
		accent: "\x1b[38;5;39m",
		success: "\x1b[38;5;84m",
		warning: "\x1b[38;5;214m",
		error: "\x1b[38;5;196m",
		muted: "\x1b[38;5;244m",
		bashMode: "\x1b[38;5;84m",
		toolTitle: "\x1b[38;5;39m",
	};
	const bg = bgMap[opts.bg] || "\x1b[48;5;235m";
	const fg = fgMap[opts.fg] || "\x1b[38;5;244m";
	const reset = "\x1b[0m";
	return `${bg}${fg} ${label} ${reset}`;
}

// Collapsed output helper: show first 3 lines + hidden count
export function collapseOutput(text, maxLines = 3) {
	if (typeof text !== "string") return "";
	const lines = text.split("\n");
	if (lines.length <= maxLines) return text;
	const visible = lines.slice(0, maxLines);
	const hidden = lines.length - maxLines;
	return `${visible.join("\n")}\n\x1b[2m… ${hidden} more lines\x1b[22m`;
}

export default function biggzToolPills(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	if (typeof pi.on !== "function") return;

	// Expose helpers for tests / other extensions
	try {
		pi._biggzToolPills = { TOOL_PILL_MAP, getToolPill, ansiPill, collapseOutput };
		if (pi._biggzExtension) {
			pi._biggzExtension.getToolPill = getToolPill;
		}
	} catch {}

	// Hook tool_call to inject pill metadata (non-blocking, observability)
	try {
		pi.on("tool_call", async (event) => {
			const name = event?.toolName ?? event?.name ?? "";
			const pill = getToolPill(name);
			// Attach pill info to event for downstream renderers (if pi supports event.pill)
			try {
				if (event && pill) event._pill = pill;
			} catch {}
		});
	} catch {}

	// Hook tool_result to provide pill-aware rendering helper
	try {
		pi.on("tool_result", async (event) => {
			// No block, just ensure pill info persists
			try {
				const name = event?.toolName ?? event?.name ?? "";
				if (name && event && !event._pill) event._pill = getToolPill(name);
			} catch {}
		});
	} catch {}
}
