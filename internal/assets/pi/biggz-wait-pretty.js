/**
 * biggz-wait-pretty — subagent_wait pretty wrapper (polish-wait-visuals C)
 *
 * Extends polish-wait-visuals B: single-run now renders as 2-line FleetRow
 * ◐ agent · state  2m04s   4.1k›80k / └ task truncated 60 — no UUID/fan-out/output dump.
 * Throttles 3s, ≤2 lines, flag BIGGZ_PRETTY=0 to disable, child bypass.
 *
 * biggz-pi-pretty — FleetView wait pretty wrapper (polish-wait-visuals B intermedia)
 *
 * Throttles asyncWaitUpdate / detachedForegroundWaitUpdate 1s→3s, collapses
 * multi-run wait into 1-line headline + optional dim hint (≤2 lines), avoiding
 * full formatAsyncRunList dump. Flag-gated via BIGGZ_PRETTY=0, single-file revert.
 *
 * ADR: docs/adr/xxx-pi-subagents-wait.md (shim vs fork/vendor/config)
 *
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */
export default function biggzPiPretty(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	if (!pi || typeof pi.on !== "function") return;

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
		// Normalize whitespace, ensure ≤2 lines, never dump full list
		const clean = head.split("\n").map((s) => s.trim()).filter(Boolean).join(" ");
		if (clean.length <= 120) {
			return [clean];
		}
		// Excessively long → truncate to 120 chars, second line dim hint
		const truncated = clean.slice(0, 117) + "…";
		return [truncated, "\x1b[2m— open Fleet for detail\x1b[0m"];
	}

	function compactK(n){n=Number(n)||0;if(n<1000)return String(n);if(n<10000)return n%1000?(n/1000).toFixed(1)+'k':n/1000+'k';return Math.floor(n/1000)+'k';}
	function rightAlign(s,w){s=String(s);return s.length>=w?s:' '.repeat(w-s.length)+s;}
	function fmtElapsed(s){s=Math.floor(Number(s)||0);return s>60?`${Math.floor(s/60)}m${String(s%60).padStart(2,'0')}s`:`${s}s`;}
	function fmtTokens(w,s){w=Number(w);s=Number(s);if(!Number.isFinite(w))w=s||0;if(!Number.isFinite(s))s=0;return w===s||w<1000?compactK(s):compactK(w)+'›'+compactK(s);}
	function glyphFor(s){s=String(s||'').toLowerCase();return s.includes('done')||s.includes('complete')||s.includes('success')?'✓':s.includes('wait')?'◌':'◐';}
	function formatSingleRun(elapsed,run){const r=run||{};const agent=String(r.agent||r.name||r.id||'general').trim()||'general';const state=String(r.state||r.status||'thinking').trim()||'thinking';let task=String(r.task||r.description||'').trim();const sec=typeof elapsed==='number'?elapsed:(parseInt(elapsed,10)||0);let w=r.windowTokens??r.window??r.tokensWindow??null;let sp=r.spentTokens??r.spent??r.tokensSpent??null;if(r.tokens&&typeof r.tokens==='object'){if(w==null)w=r.tokens.window??r.tokens.windowTokens??null;if(sp==null)sp=r.tokens.spent??r.tokens.spentTokens??null;}if(w==null&&sp==null){sp=r.tokens??r.spent??r.windowTokens??0;w=r.windowTokens??r.tokens??sp;}if(w==null)w=sp;if(sp==null)sp=0;w=Number(w);sp=Number(sp);if(Number.isNaN(w))w=0;if(Number.isNaN(sp))sp=0;const gl=glyphFor(state);const el=rightAlign(fmtElapsed(sec),5);const tk=rightAlign(fmtTokens(w,sp),10);const line1=`${gl} ${agent} · ${state} \x1b[2m${el}\x1b[0m \x1b[2m${tk}\x1b[0m`;let td=task||`1 run · ${agent}`;td=td.split('\n').map(s=>s.trim()).filter(Boolean).join(' ');if(td.length>60)td=td.slice(0,59)+'…';const line2=`\x1b[2m└ ${td}\x1b[0m`;return[line1,line2];}
	function renderSingleRun(piRef,ctx,elapsed,run){const l=formatSingleRun(elapsed,run);const s=l[0]||'',d=l[1]||'';try{if(ctx&&ctx.ui&&typeof ctx.ui.setStatus==='function'){ctx.ui.setStatus('biggz-pi-pretty',s);if(d)ctx.ui.notify(d,'info');}else if(ctx&&ctx.ui&&typeof ctx.ui.notify==='function'){ctx.ui.notify(s,'info');if(d)ctx.ui.notify(d,'info');}else if(piRef&&typeof piRef.notify==='function'){piRef.notify(s,'info');if(d)piRef.notify(d,'info');}}catch{}return l;}

	function renderHeadline(piRef, ctx, elapsedSec, runs) {
		const lines = formatHeadlineLines(elapsedSec, runs);
		const solid = lines[0] || "";
		const dim = lines[1] || "";
		try {
			// Prefer FleetView headline via ctx.ui if available, else pi.notify
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
				// suppress churn, keep pending latest for trailing edge
				pending = args;
				if (!pendingTimer) {
					pendingTimer = setTimeout(() => {
						pendingTimer = null;
						if (pending) {
							const p = pending;
							pending = null;
							lastRender = nowMs();
							try {
								origFn(...p);
							} catch {}
						}
					}, THROTTLE_MS - (now - lastRender));
				}
				return;
			}
			lastRender = now;
			// Extract elapsed/runs from args if shape matches {elapsed, runs}
			// Fallback: treat args[0] as elapsedSec, args[1] as runs
			// For Pi API, actual signature may be (runs, elapsed) – handle both
			try {
				return await origFn(...args);
			} catch (e) {
				// swallow to keep TUI stable
				return;
			}
		};
	}

	function wrapWithHeadline(origFn) {
		return async function (elapsedSec, runs, ctx) {
			// Support varied arg shapes: (opts) or (elapsed, runs)
			let elapsed = elapsedSec;
			let runList = runs;
			let context = ctx;
			// If first arg is object with elapsed/runs
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
			if (!shouldRender()) {
				return;
			}
			if (runList.length === 1) {
				renderSingleRun(pi, context, elapsed, runList[0]);
				return;
			}
			// Render compact headline instead of full list when 2-4 runs waiting
			if (runList.length >= 2 && runList.length <= 8) {
				renderHeadline(pi, context, elapsed, runList);
				// Do not call original that would dump formatAsyncRunList
				return;
			}
			// For other cases, fallback to original but still throttled
			if (typeof origFn === "function") {
				try {
					return await origFn(elapsed, runList, context);
				} catch {}
			}
		};
	}

	// Expose helpers for throttle mock tests (no network)
	try {
		pi._biggzPiPretty = {
			THROTTLE_MS,
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

	// Wrap known Pi wait hooks if present (direct method exposure)
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

	// Also intercept via pi.on tool_call / async_wait events for coverage
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
						// Block original full-list dump
						return { block: true, lines };
					}
				});
			} catch {}
		}
	} catch {}

	// Graceful idle: clear pending on session_stop
	try {
		pi.on("session_stop", async () => {
			if (pendingTimer) {
				clearTimeout(pendingTimer);
				pendingTimer = null;
			}
			pending = null;
		});
	} catch {}
}
