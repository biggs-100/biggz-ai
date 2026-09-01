/**
 * biggz-pi-pretty — FleetView wait pretty wrapper (polish-wait-visuals B intermedia)
 *
 * Throttles asyncWaitUpdate / detachedForegroundWaitUpdate 1s→3s, collapses
 * multi-run wait into 1-line headline + optional dim hint (≤2 lines), avoiding
 * full formatAsyncRunList dump. Flag-gated via BIGGZ_PRETTY=0, single-file revert.
 *
 * ADR: docs/adr/xxx-pi-subagents-wait.md (shim vs fork/vendor/config)
 *
 * Rank5: unified TRUNCATE_LENGTHS + previewWindowRows + capPreviewLines tail window
 * Preserves 2-line FleetRow guarantee and FixedRightWidth=16 from polish.go.
 *
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */
export default function biggzPiPretty(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	if (!pi || typeof pi.on !== "function") return;

	// ── Rank5 unified budgets (mirrors render-utils.ts) ──
	const TRUNCATE_LENGTHS = Object.freeze({ TITLE: 60, PREVIEW: 120, SHORT: 40, LONG: 100, LINE: 110 });
	const PREVIEW_LIMITS = Object.freeze({ COLLAPSED_LINES: 3, COLLAPSED_ITEMS: 8, EXPANDED_LINES: 12 });
	function previewWindowRows() {
		const rows = (typeof process !== "undefined" && process.stdout && process.stdout.rows) || 30;
		return Math.max(6, rows - 20);
	}
	function capPreviewLines(lines, opts = {}) {
		if (!Array.isArray(lines)) return [];
		if (opts.expanded) return lines;
		const max = opts.maxRows ?? opts.max ?? PREVIEW_LIMITS.COLLAPSED_LINES;
		if (lines.length <= max) return lines;
		const visible = max <= 1 ? [] : lines.slice(lines.length - (max - 1));
		const hidden = lines.length - visible.length;
		return [`… ${hidden} earlier ${hidden === 1 ? "line" : "lines"}`, ...visible];
	}
	function truncateToWidth(str, max) {
		const s = String(str ?? "");
		if (s.length <= max) return s;
		if (max <= 1) return "…";
		return s.slice(0, max - 1) + "…";
	}

	const THROTTLE_MS = 3000;
	let lastRender = 0;
	let pending = null;
	let pendingTimer = null;

	function nowMs() {
		return Date.now();
	}

	function shouldRender(at) {
		const now = typeof at === "number" ? at : nowMs();
		if (now - lastRender < THROTTLE_MS) return false;
		lastRender = now;
		return true;
	}

	function formatHeadline(elapsedSec, runs) {
		if (!Array.isArray(runs) || runs.length === 0) {
			return `Wait ${elapsedSec}s · 0 runs — open Fleet for detail`;
		}
		const summaries = runs.map((r) => {
			const name = (r && (r.name || r.agent || r.id || "run")) + "";
			const state = (r && (r.state || r.status || "waiting")) + "";
			return `${name.trim()} ${state.trim()}`;
		});
		const inner = summaries.join(", ");
		return `Wait ${elapsedSec}s · ${runs.length} runs (${inner}) — open Fleet for detail`;
	}

	function formatHeadlineLines(elapsedSec, runs) {
		const head = formatHeadline(elapsedSec, runs);
		const clean = head.split("\n").map((s) => s.trim()).filter(Boolean).join(" ");
		if (clean.length <= TRUNCATE_LENGTHS.PREVIEW) {
			return capPreviewLines([clean], { maxRows: 1 });
		}
		const truncated = truncateToWidth(clean, TRUNCATE_LENGTHS.PREVIEW);
		const capped = capPreviewLines([truncated, "— open Fleet for detail"], { maxRows: 2 });
		if (capped.length > 1) capped[1] = "\x1b[2m" + capped[1] + "\x1b[0m";
		return capped;
	}

	function compactK(n){n=Number(n)||0;if(n<1000)return String(n);if(n<10000)return n%1000?(n/1000).toFixed(1)+'k':n/1000+'k';return Math.floor(n/1000)+'k';}
	function rightAlign(s,w){s=String(s);return s.length>=w?s:' '.repeat(w-s.length)+s;}
	function fmtElapsed(s){s=Math.floor(Number(s)||0);return s>60?`${Math.floor(s/60)}m${String(s%60).padStart(2,'0')}s`:`${s}s`;}
	function fmtTokens(w,s){w=Number(w);s=Number(s);if(!Number.isFinite(w))w=s||0;if(!Number.isFinite(s))s=0;return w===s||w<1000?compactK(s):compactK(w)+'›'+compactK(s);}
	function glyphFor(s){s=String(s||'').toLowerCase();return s.includes('done')||s.includes('complete')||s.includes('success')?'✓':s.includes('wait')?'◌':'◐';}
	function formatSingleRun(elapsed,run){const r=run||{};const agent=String(r.agent||r.name||r.id||'general').trim()||'general';const state=String(r.state||r.status||'thinking').trim()||'thinking';let task=String(r.task||r.description||'').trim();const sec=typeof elapsed==='number'?elapsed:(parseInt(elapsed,10)||0);let w=r.windowTokens??r.window??r.tokensWindow??null;let sp=r.spentTokens??r.spent??r.tokensSpent??null;if(r.tokens&&typeof r.tokens==='object'){if(w==null)w=r.tokens.window??r.tokens.windowTokens??null;if(sp==null)sp=r.tokens.spent??r.tokens.spentTokens??null;}if(w==null&&sp==null){sp=r.tokens??r.spent??r.windowTokens??0;w=r.windowTokens??r.tokens??sp;}if(w==null)w=sp;if(sp==null)sp=0;w=Number(w);sp=Number(sp);if(Number.isNaN(w))w=0;if(Number.isNaN(sp))sp=0;const gl=glyphFor(state);const el=rightAlign(fmtElapsed(sec),5);const tk=rightAlign(fmtTokens(w,sp),10);const line1=`${gl} ${agent} · ${state} \x1b[2m${el}\x1b[0m \x1b[2m${tk}\x1b[0m`;let td=task||`1 run · ${agent}`;td=td.split('\n').map(s=>s.trim()).filter(Boolean).join(' ');if(td.length>TRUNCATE_LENGTHS.TITLE)td=truncateToWidth(td, TRUNCATE_LENGTHS.TITLE);const line2=`\x1b[2m└ ${td}\x1b[0m`;return[line1,line2];}
	function renderSingleRun(piRef,ctx,elapsed,run){const l=formatSingleRun(elapsed,run);const s=l[0]||'',d=l[1]||'';try{if(ctx&&ctx.ui&&typeof ctx.ui.setStatus==='function'){ctx.ui.setStatus('biggz-pi-pretty',s);if(d)ctx.ui.notify(d,'info');}else if(ctx&&ctx.ui&&typeof ctx.ui.notify==='function'){ctx.ui.notify(s,'info');if(d)ctx.ui.notify(d,'info');}else if(piRef&&typeof piRef.notify==='function'){piRef.notify(s,'info');if(d)piRef.notify(d,'info');}}catch{}return l;}

	function renderHeadline(piRef, ctx, elapsedSec, runs) {
		const lines = formatHeadlineLines(elapsedSec, runs);
		const solid = lines[0] || "";
		const dim = lines[1] || "";
		try {
			if (ctx && ctx.ui && typeof ctx.ui.setStatus === "function") {
				ctx.ui.setStatus("biggz-pi-pretty", solid);
				if (dim) ctx.ui.notify(dim, "info");
			} else if (ctx && ctx.ui && typeof ctx.ui.notify === "function") {
				ctx.ui.notify(solid, "info");
				if (dim) ctx.ui.notify(dim, "info");
			} else if (piRef && typeof piRef.notify === "function") {
				piRef.notify(solid, "info");
				if (dim) piRef.notify(dim, "info");
			}
		} catch {}
		return lines;
	}

	function debounceWrapper(origFn) {
		return async function (...args) {
			const now = nowMs();
			if (now - lastRender < THROTTLE_MS) {
				pending = args;
				if (!pendingTimer) {
					pendingTimer = setTimeout(() => {
						pendingTimer = null;
						if (pending) {
							const p = pending;
							pending = null;
							lastRender = nowMs();
							try { origFn(...p); } catch {}
						}
					}, THROTTLE_MS - (now - lastRender));
				}
				return;
			}
			lastRender = now;
			try { return await origFn(...args); } catch (e) { return; }
		};
	}

	function wrapWithHeadline(origFn) {
		return async function (elapsedSec, runs, ctx) {
			let elapsed = elapsedSec;
			let runList = runs;
			let context = ctx;
			if (elapsedSec && typeof elapsedSec === "object" && !Array.isArray(elapsedSec)) {
				elapsed = elapsedSec.elapsedSec ?? elapsedSec.elapsed ?? elapsedSec.waitSec ?? 0;
				runList = elapsedSec.runs ?? elapsedSec.asyncRuns ?? [];
				context = runs;
			}
			if (typeof elapsed !== "number") {
				const n = parseInt(elapsed, 10);
				elapsed = Number.isNaN(n) ? 0 : n;
			}
			if (!Array.isArray(runList)) runList = [];
			if (!shouldRender()) return;
			if (runList.length === 1) {
				renderSingleRun(pi, context, elapsed, runList[0]);
				return;
			}
			if (runList.length >= 2 && runList.length <= 8) {
				renderHeadline(pi, context, elapsed, runList);
				return;
			}
			if (typeof origFn === "function") {
				try { return await origFn(elapsed, runList, context); } catch {}
			}
		};
	}

	try {
		pi._biggzPiPretty = {
			THROTTLE_MS,
			TRUNCATE_LENGTHS,
			PREVIEW_LIMITS,
			previewWindowRows,
			capPreviewLines,
			shouldRender,
			formatHeadline,
			formatHeadlineLines,
			formatSingleRun,
			renderHeadline: (elapsed, runs, ctx) => renderHeadline(pi, ctx, elapsed, runs),
			renderSingleRun: (elapsed, run, ctx) => renderSingleRun(pi, ctx, elapsed, run),
			_test: {
				getLast: () => lastRender,
				setLast: (v) => { lastRender = v; },
				clear: () => { lastRender = 0; pending = null; if (pendingTimer) { clearTimeout(pendingTimer); pendingTimer = null; } },
				setNow: (v) => { lastRender = v - THROTTLE_MS - 1; },
			},
		};
	} catch {}

	try {
		if (typeof pi.asyncWaitUpdate === "function") {
			const orig = pi.asyncWaitUpdate.bind(pi);
			pi.asyncWaitUpdate = wrapWithHeadline(orig);
		}
		if (typeof pi.detachedForegroundWaitUpdate === "function") {
			const orig2 = pi.detachedForegroundWaitUpdate.bind(pi);
			pi.detachedForegroundWaitUpdate = wrapWithHeadline(orig2);
		}
	} catch {}

	try {
		const events = ["asyncWaitUpdate", "detachedForegroundWaitUpdate", "subagent_wait", "async_wait"];
		for (const ev of events) {
			try {
				pi.on(ev, async (event, ctx) => {
					let elapsed = event?.elapsedSec ?? event?.elapsed ?? event?.waitSec ?? event?.seconds ?? 0;
					let runs = event?.runs ?? event?.asyncRuns ?? event?.subagents ?? [];
					if (!Array.isArray(runs) && event?.args) {
						runs = event.args.runs ?? [];
						elapsed = event.args.elapsedSec ?? elapsed;
					}
					if (typeof elapsed !== "number") {
						const n = parseInt(elapsed, 10);
						elapsed = Number.isNaN(n) ? 0 : n;
					}
					if (!Array.isArray(runs)) runs = [];
					if (runs.length === 1) {
						if (!shouldRender()) return { block: true, reason: "throttled" };
						const lines = renderSingleRun(pi, ctx, elapsed, runs[0]);
						return { block: true, lines };
					}
					if (runs.length >= 2) {
						if (!shouldRender()) return { block: true, reason: "throttled" };
						const lines = renderHeadline(pi, ctx, elapsed, runs);
						return { block: true, lines };
					}
				});
			} catch {}
		}
	} catch {}

	try {
		pi.on("session_stop", async () => {
			if (pendingTimer) { clearTimeout(pendingTimer); pendingTimer = null; }
			pending = null;
		});
	} catch {}
}
