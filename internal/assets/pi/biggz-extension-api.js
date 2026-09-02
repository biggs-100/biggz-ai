/**
 * biggz-extension-api — pi ExtensionAPI wiring for Runner
 * Mirrors biggz-tool-interception but delegates to Runner reusing PolicyInterceptor.
 * Handles PI_SUBAGENT_CHILD=1 bypass, block/revise short-circuit, tool_result no-mutate.
 * Rank1 port: status-line presets/separators/segments + git head watch stub.
 * Pi parity slice: PR hyperlink OSC 8, subagents +N, contextUsage memo 10-char gauge,
 * SafeToolRenderer try/catch, stabilizeStreamingPreviews.
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */

// ── Rank1: Status-line presets (mirrors oh-my-pi presets.ts) ──
// Guard note: BIGGZ_PRETTY=0 check lives at top of biggzExtensionAPI() below; keep it there for single-file revert.
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
	"powerline-thin": { left: "›", right: "‹", endCaps: { left: "◀", right: "▶", useBgAsFg: true }, nerdLeft: "\uE0B1", nerdRight: "\uE0B3" },
	slash: { left: " / ", right: " / " },
	pipe: { left: " │ ", right: " │ " },
	ascii: { left: " > ", right: " < " },
	powerline: { left: "\uE0B0", right: "\uE0B2", asciiFallback: "▕", endCaps: { left: "◀", right: "▶", useBgAsFg: true } },
	block: { left: "▌", right: "▌" },
	none: { left: " ", right: " " },
});

export function isPrettyEnabled(){ return process.env.BIGGZ_PRETTY!=="0"&&process.env.PI_SUBAGENT_CHILD!=="1"; }
export function isDumbTerm(){ return process.env.TERM==="dumb"; }
export function isAnimationEnabled(){ return isPrettyEnabled()&&!isDumbTerm()&&process.env.BIGGZ_NO_ANIMATION!=="1"&&process.env.GENTLE_AI_NO_ANIMATION!=="1"; }
export function isSyncSupported(){ return isPrettyEnabled()&&isAnimationEnabled()&&!isDumbTerm(); }
export function isSubagentChild(){ return process.env.PI_SUBAGENT_CHILD==="1"; }
export function hasNerdFont(opts={}){
	if(isDumbTerm()||!isPrettyEnabled()) return false;
	if(process.env.BIGGZ_NERDFONT==="0") return false;
	if(process.env.BIGGZ_NERDFONT==="1") return true;
	if(opts.hasNerdFont!=null) return !!opts.hasNerdFont;
	if(opts.theme&&opts.theme.symbolPreset==="nerd") return true;
	if(opts.ctx&&opts.ctx.hasNerdFont===true) return true;
	if(opts.ctx&&opts.ctx.hasNerdFont===false) return false;
	if(opts.ctx&&opts.ctx.symbolPreset==="nerd") return true;
	return false;
}
export function getSeparator(style, opts){
	const base = SEPARATORS[style] ?? SEPARATORS["powerline-thin"];
	if(isDumbTerm()||!isPrettyEnabled()){
		if(style==="slash") return { left: " / ", right: " / " };
		return { left: "▕", right: "▕" };
	}
	const useNerd = hasNerdFont(opts);
	if(!useNerd){
		if(style==="powerline"||style==="powerline-thin") return { left: "▕", right: "▕", asciiFallback: "▕" };
		if(base.nerdLeft) return { left: "▕", right: "▕" };
	}
	if(useNerd && base.nerdLeft) return { left: base.nerdLeft, right: base.nerdRight||base.nerdLeft };
	return base;
}

// 16ms trailing coalesce for pill streaming (mirrors tui 60fps sync)
let _pillPending=null; let _pillTimer=null;
export function schedulePillUpdate(pills, flushFn){
	if(!isSyncSupported()||isSubagentChild()) return;
	_pillPending=pills;
	if(_pillTimer) return;
	_pillTimer=setTimeout(()=>{ const toFlush=_pillPending; _pillPending=null; _pillTimer=null; try{ flushFn?.(toFlush); }catch{} },16);
}
export function flushPillQueue(flushFn){
	if(_pillTimer){ clearTimeout(_pillTimer); _pillTimer=null; }
	const p=_pillPending; _pillPending=null;
	if(p&&flushFn) try{ flushFn(p); }catch{}
	return p;
}
export function _resetPillThrottleForTest(){ if(_pillTimer){clearTimeout(_pillTimer);_pillTimer=null;} _pillPending=null; }
export function isPillThrottled(){ return !!_pillTimer; }
// Pill collapse for streaming (order-preserving, >3 → … +N hidden)
export function collapsePillsForStream(pills, limit=3){ if(!Array.isArray(pills)) return {visible:[],hidden:0,suffix:""}; if(pills.length<=limit) return {visible:pills.slice(),hidden:0,suffix:""}; return {visible:pills.slice(0,limit),hidden:pills.length-limit,suffix:`… +${pills.length-limit} hidden`}; }
export function renderPillsForStream(pills, limit=3){
	if(!isSyncSupported()||isSubagentChild()) return "";
	const {visible,suffix}=collapsePillsForStream(pills,limit);
	let s=visible.map(p=> typeof p==="string"?p:String(p.label??p.name??p.tool??p)).filter(Boolean).join(" ");
	if(suffix) s+=(s?" ":"")+suffix;
	if(isDumbTerm()||!isPrettyEnabled()) return s.replace(/\x1b\[[0-9;]*[A-Za-z]/g,"");
	return s;
}

// ── Context-usage memo (mirrors component.ts messageFingerprint) ──
// Cheap O(blocks) fingerprint, hash of last message drives memo invalidation.
export function messageFingerprint(msg) {
	if (!msg || typeof msg !== "object") return "";
	const role = msg.role || "";
	const ts = msg.timestamp || 0;
	let textLen = 0;
	let blocks = 0;
	let images = 0;
	const content = msg.content;
	if (typeof content === "string") {
		textLen += content.length;
	} else if (Array.isArray(content)) {
		blocks = content.length;
		for (const b of content) {
			if (!b || typeof b !== "object") continue;
			if (b.type === "text" && typeof b.text === "string") textLen += b.text.length;
			else if (b.type === "image") images++;
			else if (b.type === "thinking" && typeof b.thinking === "string") textLen += b.thinking.length;
			else if (b.type === "toolCall" && typeof b.name === "string") textLen += b.name.length;
		}
	}
	// bashExecution-like messages
	if (typeof msg.command === "string") textLen += msg.command.length;
	if (typeof msg.output === "string") textLen += msg.output.length;
	if (role === "assistant" && msg.usage && typeof msg.usage.totalTokens === "number") {
		textLen += msg.usage.totalTokens;
	}
	return `${role}:${ts}:${textLen}:${blocks}:${images}`;
}

export function hashMessages(messages) {
	if (!Array.isArray(messages) || messages.length === 0) return "empty:0";
	// Full structural hash: O(n) over fingerprints, catches in-place tail growth and mid-history mutations
	// Last-message-only would miss edits to earlier messages that affect context tokens.
	const len = messages.length;
	let acc = `${len}:`;
	for (let i = 0; i < len; i++) acc += messageFingerprint(messages[i]) + "|";
	// cheap djb2-like short-circuit to keep memo key bounded
	let hash = 5381;
	for (let i = 0; i < acc.length; i++) hash = ((hash << 5) + hash) ^ acc.charCodeAt(i);
	return `${len}:${hash >>> 0}:${messageFingerprint(messages[len - 1])}`;
}

// 10-char gauge: ━ filled, ─ empty (mirrors pi progress gauge for context)
export function getContextGauge(pct) {
	const p = Math.max(0, Math.min(100, Number(pct) || 0));
	const filled = Math.round(p / 10);
	const f = Math.max(0, Math.min(10, filled));
	return "━".repeat(f) + "─".repeat(10 - f);
}

let _ctxMemo = { hash: null, pct: null, gauge: null, memo: null };
export function getContextUsageMemo(messages, pct, contextWindow) {
	const hash = hashMessages(messages);
	if (_ctxMemo.hash === hash && _ctxMemo.pct === pct && _ctxMemo.contextWindow === contextWindow) {
		return _ctxMemo.memo;
	}
	const gauge = getContextGauge(pct);
	const memo = { pct, gauge, hash, contextWindow };
	_ctxMemo = { hash, pct, contextWindow, gauge, memo };
	return memo;
}

// ── SafeToolRenderer + stabilizeStreamingPreviews (mirrors tool-execution.ts) ──
export function stabilizeStreamingPreviews(input) {
	// Strip trailing `+` artefacts and trailing whitespace from streamed preview
	// before capPreviewLines. Handles string, array<string>, and PerFile-like objects.
	if (typeof input === "string") {
		let s = input.replace(/\s+$/g, "");
		// diff streaming may leave a lone trailing `+` where the added line hasn't arrived yet
		if (s.endsWith("+")) s = s.slice(0, -1).replace(/\s+$/g, "");
		return s;
	}
	if (Array.isArray(input)) {
		let changed = false;
		const out = input.map((v) => {
			const nv = stabilizeStreamingPreviews(v);
			if (nv !== v) changed = true;
			return nv;
		});
		return changed ? out : input;
	}
	if (input && typeof input === "object") {
		// PerFileDiffPreview-like { diff?: string, path?: string }
		if (typeof input.diff === "string") {
			const stabilized = stabilizeStreamingPreviews(input.diff);
			if (stabilized !== input.diff) return { ...input, diff: stabilized };
		}
		// contentPadding normalization: strip trailing whitespace
		if (typeof input.content === "string") {
			const c = input.content.replace(/\s+$/g, "");
			if (c !== input.content) return { ...input, content: c };
		}
	}
	return input;
}

export function SafeToolRenderer(renderFn, fallbackText = "✗ render error") {
	return (...args) => {
		try {
			return renderFn(...args);
		} catch (e) {
			// dim fallback, never throw into stream
			return `\x1b[2m${fallbackText}\x1b[22m`;
		}
	};
}

// Class wrapper mirroring SafeToolRendererComponent in tool-execution.ts
export class SafeToolRendererComponent {
	constructor(toolName, component, fallback) {
		this.toolName = toolName;
		this.component = component;
		this.fallback = fallback;
		this._warned = false;
	}
	render(width) {
		try {
			if (this.component && typeof this.component.render === "function") return this.component.render(width);
			if (typeof this.component === "function") return this.component(width);
			return this.component;
		} catch (err) {
			if (!this._warned) {
				this._warned = true;
				try { console.warn(`Tool renderer failed ${this.toolName}: ${String(err)}`); } catch {}
			}
			try {
				if (typeof this.fallback === "function") {
					const fb = this.fallback();
					if (fb && typeof fb.render === "function") return fb.render(width);
					return fb;
				}
				if (this.fallback) return [String(this.fallback)];
			} catch {}
			return ["\x1b[2m✗ render error\x1b[22m"];
		}
	}
}

// ── Segments (subset required by task, extended with Pi parity) ──
export const SEGMENTS = Object.freeze(["model", "mode", "path", "git", "pr", "subagents", "context_pct", "cost"]);

function renderPR(pr) {
	if (!pr) return "";
	const label = `⤴ #${pr.number}`;
	if (pr.url) {
		// OSC 8 hyperlink with BEL terminators (ST \x1b\\ also valid, BEL is canonical for gh helper)
		// Format: ESC ]8;;URL BEL label ESC ]8;; BEL
		return `\x1b]8;;${pr.url}\x07${label}\x1b]8;;\x07`;
	}
	// Fallback: construct GH URL if owner/repo known via pr.repo
	if (pr.repo && pr.number) {
		const url = `https://github.com/${pr.repo}/pull/${pr.number}`;
		return `\x1b]8;;${url}\x07${label}\x1b]8;;\x07`;
	}
	return label;
}

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
			return renderPR(pr);
		}
		case "subagents": {
			const n = ctx.subagentCount ?? ctx.subagents ?? ctx.subagentsCount ?? ctx.runningSubagents ?? 0;
			if (!n || Number(n) <= 0) return "";
			// Spec: subagents count `+N` (compact badge for parallel workers)
			// Keep icon variant available via ctx.icon flag but default to +N compact
			if (ctx.compactSubagents === false) {
				return `👥 ${n}`;
			}
			return `+${n}`;
		}
		case "context_pct": {
			const pct = ctx.contextPercent ?? ctx.pct;
			if (pct == null) return "";
			const v = pct > 0 && pct < 1 ? pct.toFixed(1) : Math.round(pct);
			// contextUsage memo gauge: 10-char ━/─, memoized on messages hash
			if (Array.isArray(ctx.messages) && ctx.contextWindow != null) {
				const memo = getContextUsageMemo(ctx.messages, pct, ctx.contextWindow);
				// gauge is memoized; render as `◫ 45% ━━━━──────` compact
				return `◫ ${v}% ${memo.gauge}`;
			}
			// Fallback gauge without memo
			const gauge = getContextGauge(pct);
			// Minimal display keeps `◫ 45%` compat; append gauge when space allows (compact flag off)
			if (ctx.withGauge === false) return `◫ ${v}%`;
			return `◫ ${v}% ${gauge}`;
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
	// stabilize streamed preview fields before rendering if present
	const stableCtx = ctx && (ctx.preview || ctx.streamedPreview) ? { ...ctx, preview: stabilizeStreamingPreviews(ctx.preview), streamedPreview: stabilizeStreamingPreviews(ctx.streamedPreview) } : ctx;
	const leftParts = leftIds.map((id) => renderSegment(id, stableCtx)).filter(Boolean);
	const rightParts = rightIds.map((id) => renderSegment(id, stableCtx)).filter(Boolean);
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
			messageFingerprint,
			hashMessages,
			getContextGauge,
			getContextUsageMemo,
			stabilizeStreamingPreviews,
			SafeToolRenderer,
			SafeToolRendererComponent,
			renderPR,
			isPrettyEnabled,
			isDumbTerm,
			isAnimationEnabled,
			isSyncSupported,
			isSubagentChild,
			hasNerdFont,
			schedulePillUpdate,
			flushPillQueue,
			_resetPillThrottleForTest,
			isPillThrottled,
			collapsePillsForStream,
			renderPillsForStream,
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
			// SafeToolRenderer wrap is applied at render time; here we ensure even if upstream rendering throws we don't crash stream
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
