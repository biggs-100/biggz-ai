/**
 * biggz-footer — single-line powerline footer (biggz-native port)
 *
 * Port of tomsej/pi-ext custom-footer (powerline compact, belowEditor widget + path abbreviate, git branch, context/model rendering)
 * and @rokiy/pi-ui status bar (two-line core footer: model/thinking, cwd, git branch, context%, tokens ↑↓, cost, extension statuses).
 *
 * This implementation merges both into a biggz-native single-line powerline footer:
 *   path (abbrev) │ git branch │ tokens ↑↓ │ cost │ context% │ model • thinking
 *
 * Example (tomsej): ~/project (main) │ ↑12k ↓8k $0.42 │ 42%/200k │ ⚡ claude-sonnet-4 • medium
 * Example (rokiy):  󰏿 | 󰚩 deepseek/deepseek-chat 󰧑 medium  my-project  main 󰨊 45% 󰓡 ↑1.2k ↓3.4k 󰈁 0.0012
 * Biggz unified:   ~/project ▕ ⑂ main ▕ ↑12k ↓8k ▕ $0.42 ▕ ◫ 42% ▕ ⚡ claude-sonnet-4 • medium
 *
 * Uses STATUS_LINE_PRESETS / SEPARATORS from biggz-extension-api.js (preset + separator source)
 * and theme colors via ctx.ui.theme.fg("statusLinePath" etc.) — no hardcoded ANSI.
 * Powerline: separator from STATUS_LINE_PRESETS.default.separator ("powerline-thin" → "›" / fallback "▕")
 * rendered with theme.fg("statusLineSep") so color follows active theme (dark/light/titanium etc.).
 *
 * Deploy: ~/.pi/agent/extensions/biggz-footer.js via install.DeployPiFooter + assets.FS embed all:pi
 * Flags: BIGGZ_PRETTY=0 disables footer, PI_SUBAGENT_CHILD=1 bypass (no footer in subagents).
 *
 * Pure rendering helpers exported for tests; no side effects outside pi.on("session_start").
 *
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */

// ── STATUS_LINE_PRESETS import (biggz-native single source) ──────────────────────────
// Mirrors biggz-extension-api.js STATUS_LINE_PRESETS.default.separator = "powerline-thin"
// Keep static import for pi runtime (jiti resolves .js); fallback defined for node --check without bundler.
import {
	STATUS_LINE_PRESETS as IMPORTED_PRESETS,
	SEPARATORS as IMPORTED_SEPARATORS,
	getSeparator as importedGetSeparator,
} from "./biggz-extension-api.js";

// Fallback presets mirror biggz-extension-api.js when import unavailable (tests/native node)
const FALLBACK_PRESETS = Object.freeze({
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
		segmentOptions: { path: { abbreviate: true, maxLength: 30 }, git: { showBranch: true } },
	},
	compact: {
		leftSegments: ["model", "mode", "git", "pr"],
		rightSegments: ["session_name", "cost", "context_pct"],
		separator: "powerline-thin",
		segmentOptions: { model: { showThinkingLevel: false }, git: { showBranch: true } },
	},
});
const PRESETS = IMPORTED_PRESETS || FALLBACK_PRESETS;

const FALLBACK_SEPARATORS = Object.freeze({
	"powerline-thin": { left: "›", right: "‹", endCaps: { left: "◀", right: "▶", useBgAsFg: true } },
	slash: { left: " / ", right: " / " },
	pipe: { left: " │ ", right: " │ " },
	ascii: { left: " > ", right: " < " },
	powerline: { left: "▶", right: "◀", endCaps: { left: "◀", right: "▶", useBgAsFg: true } },
	block: { left: "▌", right: "▌" },
	none: { left: " ", right: " " },
});
const SEPS = IMPORTED_SEPARATORS || FALLBACK_SEPARATORS;

function getSep(style) {
	try {
		if (typeof importedGetSeparator === "function") return importedGetSeparator(style);
	} catch {}
	return SEPS[style] ?? SEPS["powerline-thin"] ?? SEPS.pipe;
}

// Footer preset: single-line powerline — derives separator from STATUS_LINE_PRESETS.default
export const FOOTER_PRESET = Object.freeze({
	presetName: "default",
	get separator() {
		return (PRESETS.default && PRESETS.default.separator) || "powerline-thin";
	},
	leftSegments: Object.freeze(["path", "git", "tokens", "cost", "context_pct", "model"]),
	segmentOptions: Object.freeze({
		path: { abbreviate: true, maxLength: 40, stripWorkPrefix: true },
		git: { showBranch: true },
	}),
});

// ── Icons (unicode default; nerd variants auto if Nerd Font detected via theme symbolPreset) ──
export const FOOTER_ICONS = Object.freeze({
	branch: "⑂",
	branchNerd: "",
	tokens: "↕",
	usage: "󰓡",
	cost: "$",
	context: "◫",
	model: "⚡",
	thinkingDot: "•",
	sepPowerline: "▕",
	sepThin: "┆",
	sepPipe: "│",
});

// ── Guards + Nerd detection (mirrors biggz-extension-api isPretty/isDumb + NerdFont fallback) ──
export function isPrettyEnabled(){ return typeof process!=="undefined"? process.env.BIGGZ_PRETTY!=="0"&&process.env.PI_SUBAGENT_CHILD!=="1" : true; }
export function isDumbTerm(){ return typeof process!=="undefined"? process.env.TERM==="dumb" : false; }
export function isSubagentChild(){ return typeof process!=="undefined"? process.env.PI_SUBAGENT_CHILD==="1" : false; }
export function isAnimationEnabled(){ return isPrettyEnabled()&&!isDumbTerm()&&typeof process!=="undefined" && process.env.BIGGZ_NO_ANIMATION!=="1"&&process.env.GENTLE_AI_NO_ANIMATION!=="1"; }
export function hasNerdFont(theme, ctx){
	if(isDumbTerm()||!isPrettyEnabled()) return false;
	if(typeof process!=="undefined" && process.env.BIGGZ_NERDFONT==="0") return false;
	if(typeof process!=="undefined" && process.env.BIGGZ_NERDFONT==="1") return true;
	if(theme && theme.symbolPreset==="nerd") return true;
	if(ctx && ctx.hasNerdFont===true) return true;
	if(ctx && ctx.hasNerdFont===false) return false;
	if(ctx && ctx.symbolPreset==="nerd") return true;
	return false;
}
export const NERD_GLYPHS = Object.freeze({ powerline: "\uE0B0", powerlineRight: "\uE0B2", thin: "›", fallback: "▕", slashFallback: " / " });
export function getNerdAwareSeparator(style, theme, ctx){
	const base = getSep(style);
	const rawLeft = base && base.left ? String(base.left).trim() : "";
	if(isDumbTerm()||!isPrettyEnabled()) return { left: "▕", right: "▕" };
	if(!hasNerdFont(theme, ctx)){
		if(style==="slash") return { left: " / ", right: " / " };
		if(rawLeft==="›"||rawLeft==="\uE0B0"||rawLeft==="\uE0B1"||rawLeft==="▶"||rawLeft==="▌"||base.nerdLeft) return { left: "▕", right: "▕" };
		return { left: "▕", right: "▕" };
	}
	if(base.nerdLeft) return { left: base.nerdLeft, right: base.nerdRight||base.nerdLeft };
	return base;
}

// ── Thinking roles → theme color token (mirrors tomsej/shared/thinking-colors + rokiy) ──
export const THINKING_ROLES = Object.freeze({
	off: "dim",
	minimal: "dim",
	low: "success",
	medium: "warning",
	high: "bashMode",
	xhigh: "error",
	max: "error",
	default: "dim",
});

export function thinkingRoleFor(level) {
	const k = String(level || "off").toLowerCase();
	return THINKING_ROLES[k] || THINKING_ROLES.off;
}

// ── ANSI helpers (mirrors biggz-question-mouse visibleWidth fallback) ──
function stripAnsi(s) {
	try {
		return String(s ?? "")
			.replace(/\x1b\[[0-9;]*[A-Za-z]/g, "")
			.replace(/\x1b\][^\x07]*\x07/g, "")
			.replace(/\x1b\(B/g, "");
	} catch {
		return String(s ?? "");
	}
}

export function visibleWidth(str) {
	if (!str) return 0;
	// Prefer pi-tui's Bun.stringWidth if available at runtime; fallback to codepoint scan
	try {
		// eslint-disable-next-line no-undef
		if (typeof globalThis !== "undefined" && globalThis.Bun && typeof globalThis.Bun.stringWidth === "function") {
			return globalThis.Bun.stringWidth(stripAnsi(String(str)));
		}
	} catch {}
	const s = stripAnsi(String(str));
	let w = 0;
	for (const ch of [...s]) {
		const cp = ch.codePointAt(0);
		if (
			cp !== undefined &&
			((cp >= 0x1100 && cp <= 0x115f) ||
				(cp >= 0x2e80 && cp <= 0xa4cf) ||
				(cp >= 0xac00 && cp <= 0xd7a3) ||
				(cp >= 0xf900 && cp <= 0xfaff) ||
				(cp >= 0xfe10 && cp <= 0xfe6f) ||
				(cp >= 0xff00 && cp <= 0xff60) ||
				(cp >= 0xffe0 && cp <= 0xffe6))
		)
			w += 2;
		else w += 1;
	}
	return w;
}

export function truncateToWidth(text, maxWidth, ellipsis = "…") {
	maxWidth = Math.max(0, maxWidth | 0);
	const s = String(text ?? "");
	if (visibleWidth(s) <= maxWidth) return s;
	if (maxWidth <= 0) return "";
	if (maxWidth === 1) return ellipsis === "" ? "" : "…";
	let out = "";
	let w = 0;
	const ellW = ellipsis ? 1 : 0;
	const target = maxWidth - ellW;
	for (const ch of [...s]) {
		const cw = visibleWidth(ch);
		if (w + cw > target) break;
		out += ch;
		w += cw;
	}
	return out + (ellipsis || "");
}

// ── Tokens formatting (mirrors tomsej/renderers fmtTokens + rokiy/footer formatTokens) ──
export function fmtTokens(n) {
	const v = Number(n) || 0;
	if (v < 1000) return String(v);
	if (v < 10_000) return `${(v / 1000).toFixed(1)}k`;
	if (v < 1_000_000) return `${Math.round(v / 1000)}k`;
	return `${(v / 1_000_000).toFixed(1)}M`;
}

// ── Path helpers (mirrors tomsej/renderers buildPathString + renderPath) ──
export function buildPathString(cwd, branch) {
	let pwd = String(cwd || ".");
	const home = (typeof process !== "undefined" && (process.env.HOME || process.env.USERPROFILE)) || "";
	if (home && pwd.startsWith(home)) pwd = `~${pwd.slice(home.length)}`;
	// strip Work prefix if configured like STATUS_LINE_PRESETS path.stripWorkPrefix
	// Keep generic: if preset requests stripWorkPrefix and path contains /Workspace/ or similar, keep as is for now
	if (branch) return `${pwd} (${branch})`;
	return pwd;
}

export function abbreviatePath(cwd, maxLength = 40) {
	const raw = buildPathString(cwd, null);
	if (visibleWidth(raw) <= maxLength) return raw;
	if (maxLength < 10) return truncateToWidth(raw, maxLength);
	return "…" + raw.slice(-(maxLength - 1));
}

export function renderPath(pathRaw, budget, theme) {
	if (budget < 10) return "";
	if (visibleWidth(pathRaw) <= budget) {
		return theme && typeof theme.fg === "function" ? theme.fg("statusLinePath", pathRaw) : pathRaw;
	}
	const truncated = "…" + String(pathRaw).slice(-(budget - 1));
	return theme && typeof theme.fg === "function" ? theme.fg("statusLinePath", truncateToWidth(truncated, budget)) : truncateToWidth(truncated, budget);
}

// ── Context usage label (mirrors tomsej/renderContextUsage + rokiy/getContextLabel) ──
export function renderContextUsage(pct, win, theme) {
	const raw = win ? `${Number(pct || 0).toFixed(0)}%/${fmtTokens(win)}` : `${Number(pct || 0).toFixed(0)}%`;
	if (pct > 90) return theme && typeof theme.fg === "function" ? theme.fg("error", raw) : raw;
	if (pct > 70) return theme && typeof theme.fg === "function" ? theme.fg("warning", raw) : raw;
	return theme && typeof theme.fg === "function" ? theme.fg("success", raw) : raw;
}

export function getContextLabel(ctx) {
	if (!ctx || typeof ctx.getContextUsage !== "function") {
		// fallback via ctx.contextPercent or ctx.pct
		const pct = ctx?.contextPercent ?? ctx?.pct ?? ctx?.context?.percent;
		if (pct == null) return "-";
		return `${Math.max(0, Math.min(100, Number(pct))).toFixed(Number(pct) >= 10 ? 0 : 1)}%`;
	}
	try {
		const u = ctx.getContextUsage();
		if (!u) return "-";
		// handle fraction/used/limit fallbacks like rokiy
		let pct = null;
		if (typeof u.percent === "number") pct = u.percent;
		else if (typeof u.fraction === "number") pct = u.fraction * 100;
		else if (typeof u.used === "number" && typeof u.limit === "number" && u.limit > 0) pct = (u.used / u.limit) * 100;
		if (pct == null || Number.isNaN(pct)) return "-";
		const clamped = Math.max(0, Math.min(100, pct));
		return `${clamped.toFixed(clamped >= 10 ? 0 : 1)}%`;
	} catch {
		return "-";
	}
}

// ── Usage collection (mirrors rokiy collectUsage) ──
export function collectUsage(ctx) {
	let input = 0;
	let output = 0;
	let cost = 0;
	try {
		if (ctx && ctx.sessionManager && typeof ctx.sessionManager.getBranch === "function") {
			for (const entry of ctx.sessionManager.getBranch()) {
				if (entry?.type !== "message" || entry?.message?.role !== "assistant") continue;
				const u = entry.message.usage;
				if (!u) continue;
				input += Number(u.input || u.promptTokens || 0) || 0;
				output += Number(u.output || u.completionTokens || 0) || 0;
				cost += Number(u.cost?.total ?? u.cost ?? 0) || 0;
			}
			if (input || output || cost) return { input, output, cost };
		}
	} catch {}
	// Fallbacks: direct usage on ctx
	try {
		if (ctx?.usage) {
			input = Number(ctx.usage.input ?? ctx.usage.promptTokens ?? 0) || input;
			output = Number(ctx.usage.output ?? ctx.usage.completionTokens ?? 0) || output;
			cost = Number(ctx.usage.cost?.total ?? ctx.usage.cost ?? 0) || cost;
		}
		if (ctx?.usageStats) {
			input = input || Number(ctx.usageStats.input ?? 0) || 0;
			output = output || Number(ctx.usageStats.output ?? 0) || 0;
			cost = cost || Number(ctx.usageStats.cost ?? 0) || 0;
		}
		if (ctx?.tokens) {
			input = input || Number(ctx.tokens.input ?? 0) || 0;
			output = output || Number(ctx.tokens.output ?? 0) || 0;
		}
	} catch {}
	return { input, output, cost };
}

export function getUsageLabel(usage) {
	if (!usage || (!usage.input && !usage.output)) return "-";
	return `↑${fmtTokens(usage.input)} ↓${fmtTokens(usage.output)}`;
}

export function getCostLabel(cost) {
	const v = Number(cost);
	if (!Number.isFinite(v) || v <= 0) return "-";
	// Match biggz-extension-api renderSegment cost: toFixed(2) ; keep 4 decimals for micro costs like rokiy
	if (v < 0.01) return `$${v.toFixed(4)}`;
	return `$${v.toFixed(2)}`;
}

export function getModelLabel(ctx) {
	if (!ctx || !ctx.model) return "no-model";
	const p = ctx.model.provider ? `${ctx.model.provider}/` : "";
	return `${p}${ctx.model.id || ctx.model.name || "no-model"}`;
}

export function getThinkingLabel(pi) {
	try {
		return (pi && typeof pi.getThinkingLevel === "function" && pi.getThinkingLevel()) || "off";
	} catch {
		return "off";
	}
}

export function renderModelInfo(modelName, provider, thinking, theme) {
	const thinkSuffix = thinking && thinking !== "off" ? ` ${FOOTER_ICONS.thinkingDot} ${thinking}` : "";
	const raw = `⚡ ${modelName}${provider ? ` (${provider})` : ""}${thinkSuffix}`;
	const rawWidth = visibleWidth(raw);
	let text = "";
	if (theme && typeof theme.fg === "function") {
		text = theme.fg("statusLineModel", `⚡ ${modelName}`);
		if (provider) text += theme.fg("muted", ` (${provider})`);
		if (thinking && thinking !== "off") {
			const role = thinkingRoleFor(thinking);
			text += theme.fg("dim", ` ${FOOTER_ICONS.thinkingDot} `) + theme.fg(role, thinking);
		}
	} else {
		text = raw;
	}
	return { text, raw, rawWidth };
}

// ── Footer segments (theme colors via statusLine* tokens) ──
export function buildFooterSegments(theme, footerData, ctx, pi, usage) {
	// New contract: branch|change|lineage|lens 1/4|budget 1/1 — detect if new data present else legacy path|branch|tokens|cost|context|model
	const rawBranch = (()=>{ try{ return footerData?.getGitBranch?.() || ctx?.branch || ctx?.git?.branch || ctx?.cwdBranch || null; }catch{ return ctx?.branch||ctx?.git?.branch||null; }})();
	const rawChange = ctx?.change || ctx?.changeName || ctx?.sddChange || ctx?.changeId || footerData?.change || footerData?.changeName || null;
	const rawLineage = ctx?.lineage ?? ctx?.lineageId ?? footerData?.lineage ?? footerData?.lineageId ?? null;
	const rawLens = ctx?.lens ?? ctx?.lensProgress ?? ctx?.lensLabel ?? footerData?.lens ?? footerData?.lensProgress ?? null;
	const rawBudget = ctx?.budget ?? ctx?.budgetLabel ?? ctx?.budgetProgress ?? footerData?.budget ?? footerData?.budgetProgress ?? null;
	const hasNew = rawChange!=null || rawLineage!=null || rawLens!=null || rawBudget!=null;
	const col = (raw, token)=>{
		if(!isPrettyEnabled()||isDumbTerm()||!theme||typeof theme.fg!=="function") return raw;
		try{ return theme.fg(token, raw); }catch{ return raw; }
	};
	if(hasNew){
		const branchRaw = rawBranch ? String(rawBranch) : "";
		const changeRaw = rawChange ? String(rawChange) : "";
		const lineageRaw = rawLineage!=null ? `lineage ${String(rawLineage)}` : "";
		const lensRaw = (()=>{
			if(rawLens==null) return "";
			if(typeof rawLens==="string") return rawLens.includes("lens")? rawLens : `lens ${rawLens}`;
			if(typeof rawLens==="object"){
				if(rawLens.current!=null&&rawLens.total!=null) return `lens ${rawLens.current}/${rawLens.total}`;
				if(rawLens.label) return `lens ${rawLens.label}`;
				if(rawLens.value) return `lens ${rawLens.value}`;
			}
			return `lens ${String(rawLens)}`;
		})();
		const budgetRaw = (()=>{
			if(rawBudget==null) return "";
			if(typeof rawBudget==="string") return rawBudget.includes("budget")? rawBudget : `budget ${rawBudget}`;
			if(typeof rawBudget==="object"){
				if(rawBudget.current!=null&&rawBudget.total!=null) return `budget ${rawBudget.current}/${rawBudget.total}`;
				if(rawBudget.label) return `budget ${rawBudget.label}`;
				if(rawBudget.value) return `budget ${rawBudget.value}`;
			}
			return `budget ${String(rawBudget)}`;
		})();
		const branchSeg = branchRaw ? col(branchRaw, "statusLineGitClean") : "";
		const changeSeg = changeRaw ? col(changeRaw, "statusLinePath") : "";
		const lineageSeg = lineageRaw ? col(lineageRaw, "statusLineContext") : "";
		const lensSeg = lensRaw ? col(lensRaw, "statusLineSpend") : "";
		const budgetSeg = budgetRaw ? col(budgetRaw, "statusLineCost") : "";
		const segments = [branchSeg, changeSeg, lineageSeg, lensSeg, budgetSeg].filter(Boolean);
		const rawsOrdered = [branchRaw, changeRaw, lineageRaw, lensRaw, budgetRaw].filter(Boolean);
		return {
			segments, rawsOrdered,
			raw: { branch: branchRaw, change: changeRaw, lineage: lineageRaw, lens: lensRaw, budget: budgetRaw },
			widths: { branch: visibleWidth(branchRaw), change: visibleWidth(changeRaw), lineage: visibleWidth(lineageRaw), lens: visibleWidth(lensRaw), budget: visibleWidth(budgetRaw) },
			allColored: segments,
			pathRaw: changeRaw || branchRaw || "",
			branchSeg, changeSeg, lineageSeg, lensSeg, budgetSeg,
			tokensSeg: lensSeg, costSeg: budgetSeg, contextSeg: lineageSeg, modelSeg: "",
			rawLegacy: { path: changeRaw, branch: branchRaw, tokens: lensRaw, cost: budgetRaw, context: lineageRaw, model: "" },
		};
	}
	// legacy fallback: path|branch|tokens|cost|context|model
	const gitBranch = rawBranch;
	const cwd = ctx?.cwd || (typeof process !== "undefined" ? process.cwd() : ".");
	const pathRawFull = buildPathString(cwd, null);
	let pct = null; let win = 0;
	try{ const u = ctx?.getContextUsage?.(); if(u){ if(typeof u.percent==="number") pct=u.percent; else if(typeof u.fraction==="number") pct=u.fraction*100; else if(typeof u.used==="number"&&typeof u.limit==="number"&&u.limit>0) pct=(u.used/u.limit)*100; win=Number(u.contextWindow??u.limit??ctx?.model?.contextWindow??0)||0; } }catch{}
	if(pct==null){ const lab=getContextLabel(ctx); pct=lab==="-"?0:parseFloat(lab)||0; }
	const usageTotals = usage || collectUsage(ctx);
	const costVal = Number(usageTotals?.cost ?? ctx?.cost ?? ctx?.usageStats?.cost ?? 0)||0;
	const ctxLabelRaw = pct!=null ? (win? `${Number(pct).toFixed(0)}%/${fmtTokens(win)}`:`${Number(pct).toFixed(0)}%`):"-";
	const tokensRaw=getUsageLabel(usageTotals); const costRaw=getCostLabel(costVal);
	const branchSeg = gitBranch ? col(`${FOOTER_ICONS.branch} ${gitBranch}`, "statusLineGitClean") : "";
	const tokensSeg = tokensRaw==="-"?"":col(tokensRaw,"statusLineSpend");
	const costSeg = costRaw==="-"?"":col(costRaw,"statusLineCost");
	const contextSeg = (ctxLabelRaw==="-"||pct==null)?"":renderContextUsage(Number(pct)||0, win, theme);
	const modelName=getModelLabel(ctx).split("/").pop()||getModelLabel(ctx); const provider=ctx?.model?.provider||""; const thinking=getThinkingLabel(pi); const modelInfo=renderModelInfo(modelName,provider,thinking,theme); const modelSeg=modelInfo.text;
	return {
		pathRaw: pathRawFull, branchSeg, tokensSeg, costSeg, contextSeg, modelSeg,
		raw: { path: pathRawFull, branch: gitBranch?`${FOOTER_ICONS.branch} ${gitBranch}`:"", tokens: tokensRaw, cost: costRaw, context: ctxLabelRaw, model: modelInfo.raw },
		widths: { branch: visibleWidth(gitBranch?`${FOOTER_ICONS.branch} ${gitBranch}`:""), tokens: visibleWidth(tokensRaw==="-"?"":tokensRaw), cost: visibleWidth(costRaw==="-"?"":costRaw), context: visibleWidth(ctxLabelRaw==="-"?"":ctxLabelRaw), model: modelInfo.rawWidth, path: visibleWidth(pathRawFull) },
		allColored: [pathRawFull, branchSeg, tokensSeg, costSeg, contextSeg, modelSeg],
	};
}

function themeSpacer(theme) { return " "; }

function sepStr(theme, ctx) {
	if(!isPrettyEnabled()||isDumbTerm()){
		const raw=" ▕ ";
		if(theme && typeof theme.fg==="function" && !isDumbTerm() && isPrettyEnabled()){ try{ return theme.fg("statusLineSep", raw); }catch{ try{return theme.fg("dim", raw);}catch{return raw;}} }
		return raw;
	}
	const presetSep = PRESETS.default.separator || "powerline-thin";
	let def;
	try{ def = getNerdAwareSeparator(presetSep, theme, ctx); }catch{ def = getSep(presetSep); }
	let glyph = (def && def.left && String(def.left).trim()) || FOOTER_ICONS.sepPowerline;
	if(!hasNerdFont(theme, ctx)){
		if(glyph==="›"||glyph==="\uE0B0"||glyph==="\uE0B1"||glyph==="▶"||glyph==="▌") glyph="▕";
		if(glyph.includes("/") && glyph.trim()!=="▕") glyph="/";
		else if(glyph.trim()===""|| (glyph!=="▕"&&glyph!=="/")){
			if(glyph!=="▕"&&glyph!=="/") glyph="▕";
		}
	} else {
		if(glyph==="▕") glyph="›";
	}
	const raw=` ${glyph} `;
	if(theme && typeof theme.fg==="function" && !isDumbTerm() && isPrettyEnabled()){
		try{ return theme.fg("statusLineSep", raw); }catch{ try{return theme.fg("dim", raw);}catch{return raw;}}
	}
	return raw;
}

export function joinFooterSections(sections, theme, ctx) {
	const s = sepStr(theme, ctx);
	return sections.filter(Boolean).join(s);
}

export function renderFooterLine(width, theme, segments, ctxArg) {
	const ctx = ctxArg || segments?.ctx || null;
	const s = sepStr(theme, ctx);
	const sepW = visibleWidth(s);

	// New contract branch|change|lineage|lens|budget — budgeted width with order preservation + nerd fallback
	const hasNew = segments && (segments.rawOrdered || (segments.raw && (segments.raw.change!=null || segments.raw.lineage!=null || segments.raw.lens!=null || segments.raw.budget!=null)));
	if(hasNew){
		// Resolve ordered raws and colored segs for new contract
		const rawOrdered = segments.rawOrdered || [segments.raw.branch, segments.raw.change, segments.raw.lineage, segments.raw.lens, segments.raw.budget].filter(Boolean);
		const segMap = {};
		try{
			segMap.branch = segments.branchSeg || "";
			segMap.change = segments.changeSeg || "";
			segMap.lineage = segments.lineageSeg || "";
			segMap.lens = segments.lensSeg || "";
			segMap.budget = segments.budgetSeg || "";
			// fallback to segments array order if named segs missing
			if(!segMap.branch && !segMap.change && segments.segments){
				const keys=["branch","change","lineage","lens","budget"];
				const rawKeys=[segments.raw.branch,segments.raw.change,segments.raw.lineage,segments.raw.lens,segments.raw.budget].filter(Boolean);
				segments.segments.forEach((c,i)=>{ const k=keys[rawKeys.indexOf(segments.raw[keys[i]])]; if(k&&!segMap[k]) segMap[k]=c; });
			}
		}catch{}
		const orderedKeys=["branch","change","lineage","lens","budget"];
		const attemptsNew=[
			["branch","change","lineage","lens","budget"],
			["branch","change","lineage","lens"],
			["branch","change","lineage"],
			["branch","change"],
			["branch"],
			["change"],
		];
		for(const attempt of attemptsNew){
			const coloreds=[]; const raws=[];
			for(const k of attempt){ if(segments.raw[k] && String(segments.raw[k]).length){ const c=segMap[k]; if(c) coloreds.push(c); raws.push(segments.raw[k]); } }
			if(coloreds.length===0) continue;
			const totalRawW = raws.reduce((a,v)=>a+visibleWidth(v),0);
			const sepCount = Math.max(0, coloreds.length-1);
			const totalW = totalRawW + sepCount*sepW;
			if(totalW<=width){
				const rendered = joinFooterSections(coloreds, theme, ctx);
				if(visibleWidth(rendered)<=width) return rendered;
			}
			// if width extremely narrow, try next smaller attempt
		}
		// fallback: render whatever fits truncated
		const allColored = segments.segments || Object.values(segMap).filter(Boolean);
		const renderedAll = joinFooterSections(allColored, theme, ctx);
		if(visibleWidth(renderedAll)<=width) return renderedAll;
		return truncateToWidth(stripAnsi(renderedAll), width);
	}
	// Legacy Build attempts like rokiy: try full, then drop least critical (cost) etc., always keep path+branch+model minimal
	const { pathRaw, branchSeg, tokensSeg, costSeg, contextSeg, modelSeg, raw, widths } = segments;

	// Helper to compute colored segment for path with budget
	function pathWithBudget(budget) {
		return renderPath(pathRaw, budget, theme);
	}

	const attempts = [
		{ name: "full", segs: [null, branchSeg, tokensSeg, costSeg, contextSeg, modelSeg], keepCost: true, keepContext: true, keepTokens: true },
		{ name: "no-cost", segs: [null, branchSeg, tokensSeg, "", contextSeg, modelSeg], keepCost: false, keepContext: true, keepTokens: true },
		{ name: "no-tokens-cost", segs: [null, branchSeg, "", "", contextSeg, modelSeg], keepCost: false, keepContext: true, keepTokens: false },
		{ name: "path-branch-model", segs: [null, branchSeg, "", "", "", modelSeg], keepCost: false, keepContext: false, keepTokens: false },
		{ name: "path-branch", segs: [null, branchSeg, "", "", "", ""], keepCost: false, keepContext: false, keepTokens: false },
		{ name: "path-only", segs: [null, "", "", "", "", ""], keepCost: false, keepContext: false, keepTokens: false },
	];

	for (const attempt of attempts) {
		// Build non-path segments for this attempt (indices 1..5)
		const nonPathColored = attempt.segs.slice(1).filter(Boolean);
		const nonPathRaw = [];
		if (attempt.segs[1]) nonPathRaw.push(raw.branch);
		if (attempt.segs[2]) nonPathRaw.push(raw.tokens);
		if (attempt.segs[3]) nonPathRaw.push(raw.cost);
		if (attempt.segs[4]) nonPathRaw.push(raw.context);
		if (attempt.segs[5]) nonPathRaw.push(raw.model);

		const nonPathWidth = nonPathRaw.reduce((a, v) => a + visibleWidth(v), 0);
		const separatorsForNonPath = Math.max(0, nonPathColored.length - 1) * sepW;
		// path segment plus one sep to first non-path if both exist
		const needsPathSep = nonPathColored.length > 0 ? sepW : 0;
		const budgetForPath = width - nonPathWidth - separatorsForNonPath - needsPathSep;

		// Require at least 10 chars for path to keep meaningful directory (e.g. "…/project") — otherwise try dropping a segment before hiding path
		const minPathBudget = 10;
		if (budgetForPath < minPathBudget) {
			// If this is already the most minimal fallback, render whatever fits
			if (attempt.name === "path-only") {
				const pathSegFallback = pathWithBudget(Math.max(8, width));
				if (pathSegFallback) return truncateToWidth(pathSegFallback, width);
				continue;
			}
			if (attempt.name === "path-branch" && visibleWidth(raw.branch) > 0) {
				// Allow path-branch minimal with reduced budget (still try next attempt if still no budget)
				if (budgetForPath < 8) continue;
			}
			if (attempt.name !== "path-branch" && attempt.name !== "path-only") {
				// Not enough room for path + these segments → try fewer segments
				continue;
			}
		}

		let pathSeg = "";
		if (budgetForPath >= 8) {
			pathSeg = pathWithBudget(budgetForPath);
		} else if (attempt.name === "path-only" && width >= 8) {
			pathSeg = pathWithBudget(width);
		} else if (nonPathColored.length === 0) {
			pathSeg = pathWithBudget(width);
		}

		const finalColored = [];
		if (pathSeg) finalColored.push(pathSeg);
		for (const seg of nonPathColored) finalColored.push(seg);

		if (finalColored.length === 0) continue;

		const rendered = joinFooterSections(finalColored, theme);
		if (visibleWidth(rendered) <= width) return rendered;
		// If path-only attempt still too wide, truncate whole line
		if (attempt.name === "path-only") {
			return truncateToWidth(rendered, width);
		}
	}

	// Fallback: minimal path truncated to width
	const minimal = renderPath(pathRaw, Math.max(8, width), theme) || truncateToWidth(pathRaw, width);
	return truncateToWidth(minimal, width);
}

export function renderFooter(width, theme, footerData, ctx, pi) {
	if(!isPrettyEnabled()||isSubagentChild()) return [""];
	if(isDumbTerm()){
		const segs = buildFooterSegments(theme, footerData, ctx, pi);
		const line = stripAnsi(renderFooterLine(width, {fg:(_,t)=>t}, segs, ctx));
		return [stripAnsi(line)];
	}
	const segs = buildFooterSegments(theme, footerData, ctx, pi);
	segs.ctx = ctx;
	const line = renderFooterLine(width, theme, segs, ctx);
	if(isDumbTerm()) return [stripAnsi(line)];
	return [line];
}

// ── Extension entry ───────────────────────────────────────────────────────────
export default function biggzFooter(pi) {
	if (typeof process !== "undefined" && process.env.PI_SUBAGENT_CHILD === "1") return;
	if (typeof process !== "undefined" && process.env.BIGGZ_PRETTY === "0") return;
	if (!pi || typeof pi.on !== "function") return;

	// Expose helpers for tests / verification (mirrors biggz-extension-api pattern)
	try {
		pi._biggzFooter = {
			FOOTER_PRESET,
			PRESETS,
			SEPARATORS: SEPS,
			FOOTER_ICONS,
			THINKING_ROLES,
			thinkingRoleFor,
			visibleWidth,
			truncateToWidth,
			fmtTokens,
			buildPathString,
			abbreviatePath,
			renderPath,
			renderContextUsage,
			collectUsage,
			getUsageLabel,
			getCostLabel,
			getModelLabel,
			getThinkingLabel,
			renderModelInfo,
			buildFooterSegments,
			joinFooterSections,
			renderFooterLine,
			renderFooter,
			stripAnsi,
			isPrettyEnabled,
			isDumbTerm,
			isSubagentChild,
			isAnimationEnabled,
			hasNerdFont,
			NERD_GLYPHS,
			getNerdAwareSeparator,
			sepStr,
		};
		if (pi._biggzExtension) {
			pi._biggzExtension.footer = pi._biggzFooter;
		}
	} catch {}

	let rerender = null;

	pi.on("session_start", async (_event, ctx) => {
		if (ctx.mode && ctx.mode !== "tui") return;

		let tuiRef = null;

		// Primary: setFooter (pi's native single-line footer)
		try {
			ctx.ui.setFooter((tui, theme, footerData) => {
				tuiRef = tui;
				rerender = () => {
					try { tui.requestRender(); } catch {}
				};
				return {
					dispose: (() => {
						try {
							return footerData.onBranchChange(() => {
								try { tui.requestRender(); } catch {}
							});
						} catch {
							return () => {};
						}
					})(),
					render(width) {
						try {
							return renderFooter(width, theme, footerData, ctx, pi);
						} catch {
							// Fallback: minimal path
							try {
								const cwd = ctx.cwd || process.cwd();
								const p = buildPathString(cwd, null);
								const sep = theme && typeof theme.fg === "function" ? theme.fg("statusLineSep", " │ ") : " │ ";
								const line = truncateToWidth(p, width);
								return [line];
							} catch {
								return [""];
							}
						}
					},
					invalidate() {},
				};
			});
		} catch {}

		// Also watch context usage changes via interval? Rerender on agent events
		try {
			pi.on("agent_start", async () => { try { rerender?.(); } catch {} });
			pi.on("agent_end", async () => { try { rerender?.(); } catch {} });
			pi.on("message_update", async () => { try { rerender?.(); } catch {} });
			pi.on("thinking_level_select", async () => { try { rerender?.(); tuiRef?.requestRender?.(); } catch {} });
		} catch {}

		// Hint: expose that footer is active (mirrors biggz-thinking-wrap status hint)
		try {
			const hintBase = "footer: powerline single-line — path │ branch │ tokens ↑↓ │ cost │ context% │ model • thinking";
			const hint = ctx.ui?.theme ? ctx.ui.theme.fg("muted", hintBase) : hintBase;
			ctx.ui?.setStatus?.("biggz-footer", hint);
			setTimeout(() => { try { ctx.ui?.setStatus?.("biggz-footer", undefined); } catch {} }, 3000);
		} catch {}
	});

	pi.on("session_shutdown", async () => {
		rerender = null;
	});
}
