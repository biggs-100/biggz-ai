/**
 * biggz-extension-api — pi ExtensionAPI wiring for Runner
 * Mirrors biggz-tool-interception but delegates to Runner reusing PolicyInterceptor.
 * Handles PI_SUBAGENT_CHILD=1 bypass, block/revise short-circuit, tool_result no-mutate.
 * Rank1 port: status-line presets/separators/segments + git head watch stub.
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */

// ── Rank1: Status-line presets (mirrors oh-my-pi presets.ts) ──
export const STATUS_LINE_PRESETS = Object.freeze({
	default: {
		leftSegments: ["model", "mode", "path", "git", "pr", "context_pct", "cost"],
		rightSegments: ["session_name"],
		separator: "powerline-thin",
		segmentOptions: {
			model: { showThinkingLevel: true },
			path: { abbreviate: true, maxLength: 40, stripWorkPrefix: true },
			git: { showBranch: true, showStaged: true, showUnstaged: true, showUntracked: true },
		},
	},
	minimal: {
		leftSegments: ["path", "git"],
		rightSegments: ["session_name", "mode", "context_pct"],
		separator: "slash",
		segmentOptions: {
			path: { abbreviate: true, maxLength: 30 },
			git: { showBranch: true, showStaged: false, showUnstaged: false, showUntracked: false },
		},
	},
	compact: {
		leftSegments: ["model", "mode", "git", "pr"],
		rightSegments: ["session_name", "cost", "context_pct"],
		separator: "powerline-thin",
		segmentOptions: {
			model: { showThinkingLevel: false },
			git: { showBranch: true, showStaged: true, showUnstaged: true, showUntracked: false },
		},
	},
});

export function getPreset(name) {
	return STATUS_LINE_PRESETS[name] ?? STATUS_LINE_PRESETS.default;
}

// ── Separators (mirrors separators.ts + symbols.ts sep.*) ──
export const SEPARATORS = Object.freeze({
	"powerline-thin": { left: "›", right: "‹", endCaps: { left: "◀", right: "▶", useBgAsFg: true } },
	slash: { left: " / ", right: " / " },
	pipe: { left: " │ ", right: " │ " },
	ascii: { left: " > ", right: " < " },
	powerline: { left: "▶", right: "◀", endCaps: { left: "◀", right: "▶", useBgAsFg: true } },
	block: { left: "▌", right: "▌" },
	none: { left: " ", right: " " },
});

export function getSeparator(style) {
	return SEPARATORS[style] ?? SEPARATORS["powerline-thin"];
}

// ── Segments (subset required by task) ──
export const SEGMENTS = Object.freeze(["model", "mode", "path", "git", "pr", "context_pct", "cost"]);

export function renderSegment(id, ctx = {}) {
	switch (id) {
		case "model": {
			const name = (ctx.model?.name || ctx.model?.id || "no-model").replace(/^Claude\s+/, "");
			return name;
		}
		case "mode": {
			if (ctx.planMode?.enabled) return ctx.planMode.paused ? "Plan ⏸" : "Plan";
			if (ctx.goalMode?.enabled) return ctx.goalMode.paused ? "Goal ⏸" : "Goal";
			return "";
		}
		case "path": {
			const p = ctx.cwd || ctx.path || "";
			if (!p) return "";
			// abbreviate via shortenPath logic: ~/Projects stripping handled upstream
			return p.length > 40 ? `…${p.slice(-39)}` : p;
		}
		case "git": {
			const branch = ctx.git?.branch;
			if (!branch) return "";
			const st = ctx.git?.status;
			let suffix = "";
			if (st) {
				const parts = [];
				if (st.unstaged) parts.push(`*${st.unstaged}`);
				if (st.staged) parts.push(`+${st.staged}`);
				if (st.untracked) parts.push(`?${st.untracked}`);
				if (parts.length) suffix = ` ${parts.join(" ")}`;
			}
			return `⑂ ${branch}${suffix}`;
		}
		case "pr": {
			const pr = ctx.git?.pr;
			if (!pr) return "";
			return `⤴ #${pr.number}`;
		}
		case "context_pct": {
			const pct = ctx.contextPercent;
			if (pct == null) return "";
			const v = pct > 0 && pct < 1 ? pct.toFixed(1) : Math.round(pct);
			return `◫ ${v}%`;
		}
		case "cost": {
			if (ctx.cost == null && ctx.usageStats?.cost == null) return "";
			const c = ctx.cost ?? ctx.usageStats?.cost ?? 0;
			return `$${Number(c).toFixed(2)}`;
		}
		default: return "";
	}
}

// ── renderStatusLine (mirrors tui/status-line.ts) ──
export function renderStatusLine(opts = {}, themeSepDot = " · ") {
	const presetName = opts.preset || "default";
	const preset = getPreset(presetName);
	const leftIds = opts.leftSegments ?? preset.leftSegments;
	const rightIds = opts.rightSegments ?? preset.rightSegments;
	const sepDef = getSeparator(opts.separator ?? preset.separator);
	const sep = sepDef.left?.trim() ? ` ${sepDef.left.trim()} ` : themeSepDot;
	const ctx = opts.ctx || opts;
	const leftParts = leftIds.map((id) => renderSegment(id, ctx)).filter(Boolean);
	const rightParts = rightIds.map((id) => renderSegment(id, ctx)).filter(Boolean);
	const left = leftParts.join(sep);
	const right = rightParts.join(sep);
	if (left && right) return `${left}${themeSepDot}${right}`;
	return left || right || "";
}

// ── SPINNER_FRAMES (mirrors symbols.ts) ──
export const SPINNER_FRAMES = Object.freeze({
	unicode: { status: ["⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"], activity: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"] },
	ascii: { status: ["|", "/", "-", "\\"], activity: ["-", "\\", "|", "/"] },
});

// ── git head watch stub (low-effort, no Rust fork) ──
// Watches .git/HEAD for changes; stub returns no-op unwatch if fs unavailable.
export function watchGitHead(cwd, onChange) {
	try {
		const fs = globalThis.require?.("fs");
		if (!fs || typeof fs.watch !== "function") return () => {};
		const path = globalThis.require?.("path");
		const headPath = path ? path.join(cwd || ".", ".git", "HEAD") : `${cwd || "."}/.git/HEAD`;
		const watcher = fs.watch(headPath, () => {
			try { onChange?.(); } catch {}
		});
		return () => { try { watcher.close(); } catch {} };
	} catch {
		return () => {};
	}
}

export default function biggzExtensionAPI(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	if (typeof pi.on !== "function") return;
	// expose helpers for tests / debugging (no harness logic change)
	try {
		pi._biggzExtension = {
			STATUS_LINE_PRESETS,
			SEPARATORS,
			SEGMENTS,
			getPreset,
			getSeparator,
			renderSegment,
			renderStatusLine,
			SPINNER_FRAMES,
			watchGitHead,
		};
	} catch {}
	try {
		pi.on("tool_call", async (event, ctx) => {
			const name = event?.toolName ?? event?.name ?? event?.tool ?? "";
			const args = event?.args ?? event?.params ?? {};
			const raw = JSON.stringify(args);
			// Preserve blocked patterns parity with Go PolicyEvaluator (defense in depth)
			const blocked = ["rm -rf", "mkfs", ":(){:|:&};:"];
			for (const p of blocked) {
				if (raw.includes(p)) return { block: true, reason: `blocked by policy: ${p}` };
			}
			const mode = process.env.BIGGZ_APPROVAL_MODE || "auto";
			if (mode === "ask" && name === "user_bash") {
				const resolved = process.env.BIGGZ_TOOL_CONSENT;
				if (resolved === "deny") return { block: true, reason: "consent denied" };
				if (resolved === "allow") {
					try { ctx?.ui?.setStatus?.("tool", `tool_execution_start ${name}`); } catch {}
					return undefined;
				}
				return { block: true, reason: "awaiting consent" };
			}
			try { ctx?.ui?.setStatus?.("tool", `tool_execution_start ${name}`); } catch {}
			return undefined;
		});
		pi.on("tool_result", async () => {
			// observability-only, no mutate, no block
		});
		pi.on("session_stop", async () => {
			const pending = parseInt(process.env.BIGGZ_PENDING_FINDINGS || "0", 10);
			const lenses = parseInt(process.env.BIGGZ_PENDING_LENSES || "0", 10);
			if (pending > 0 || lenses > 0) return { block: true, reason: "CanStopSession blocked: pending work" };
			return undefined;
		});
	} catch {}
}
