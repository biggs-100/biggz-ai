/**
 * biggz-synthesis-gate — Pi extension that enforces Post-Delegation Human Checkpoint.
 *
 * Ensures orchestrator emits synthesis markdown with ## Sub-agent Result + Artifacts/Paths + Risks + Next
 * BEFORE calling ask_user_question / question. Without synthesis the orchestrator would skip to tool call
 * and human loses artifacts/paths, risks, next.
 *
 * Dual-mode (PR4 advisor):
 * - Blocking gate (default): blocks when preceding markdown lacks ## Sub-agent Result / Artifacts/Paths.
 * - Advise mode (opt-in BIGGZ_ADVISE=1 or settings flag, default OFF): when markers present but thin
 *   (Artifacts/Paths count <2 || len <50), does NOT block; injects non-blocking concern via pi.notify / ctx.ui.notify.
 *   Heuristic only, no model call, no auto-fix.
 * - PI_SUBAGENT_CHILD=1 bypasses both modes entirely.
 *
 * Behavior:
 * - Tracks last assistant markdown in this turn via pi.on("assistant_message") / pi.on("message") fallbacks.
 * - Wraps ask_user_question and question tools via pi.registerTool interception.
 * - On execute, verifies preceding markdown contains required markers.
 * - If missing, blocks with instructive error: "Please synthesize before asking — missing ## Sub-agent Result block".
 * - If thin and advise enabled, emits concern warning but allows the call.
 * - Also hooks pi.on("tool_call") as secondary guard to emit warning/concern even if wrapping missed (load-order safe).
 *
 * Minimal but functional — mirrors biggz-thinking-wrap.js pattern.
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzSynthesisGate(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;

	let lastAssistantMarkdown = "";
	let lastUpdateTime = 0;

	function recordText(text) {
		if (typeof text === "string" && text.trim()) {
			lastAssistantMarkdown = text;
			lastUpdateTime = Date.now();
		}
	}

	function hasSynthesis(text) {
		const s = String(text || "");
		return (
			s.includes("## Sub-agent Result") &&
			s.includes("Artifacts/Paths") &&
			s.includes("Risks") &&
			s.includes("Next")
		);
	}

	function hasSynthesisLoose(text) {
		const s = String(text || "");
		// loose check for backwards compat — ensure at least artifacts + risks + next present
		return (
			s.includes("Sub-agent Result") ||
			(s.includes("Artifacts/Paths") && s.includes("Risks") && s.includes("Next"))
		);
	}

	function isChildBypass() {
		return process.env.PI_SUBAGENT_CHILD === "1";
	}

	function isAdviseEnabled() {
		const env = process.env.BIGGZ_ADVISE;
		if (env === "1" || env === "true" || env === "TRUE" || (env && String(env).toLowerCase() === "true")) return true;
		try {
			const candidates = [pi?.settings, pi?.config, pi?.state?.settings, pi?.options, pi?._settings];
			for (const obj of candidates) {
				if (!obj || typeof obj !== "object") continue;
				for (const k of Object.keys(obj)) {
					if (k.toLowerCase().includes("advise")) {
						const v = obj[k];
						if (v === true || v === 1 || v === "1" || String(v).toLowerCase() === "true") return true;
					}
				}
				if (obj.biggzAdvise === true || obj.biggzAdvise === "1" || obj.biggzAdvise === 1) return true;
				if (obj.BIGGZ_ADVISE === true || obj.BIGGZ_ADVISE === "1" || obj.BIGGZ_ADVISE === 1) return true;
			}
			// also check global pi object for direct flag
			if (pi && (pi.BIGGZ_ADVISE === "1" || pi.BIGGZ_ADVISE === true)) return true;
		} catch {}
		return false;
	}

	function extractArtifactsSection(text) {
		const s = String(text || "");
		const idx = s.indexOf("Artifacts/Paths");
		if (idx === -1) return "";
		let tail = s.slice(idx + "Artifacts/Paths".length);
		// strip leading formatting chars : * # spaces and colon variants
		tail = tail.replace(/^[:\*\s]*/, "");
		// cut at next major marker — Risks, Next, or next heading ##
		let end = tail.length;
		const markers = ["Risks", "Next", "\n## "];
		for (const m of markers) {
			const i = tail.indexOf(m);
			if (i !== -1 && i < end) end = i;
		}
		tail = tail.slice(0, end);
		return tail.trim();
	}

	function countPaths(section) {
		const s = String(section || "").trim();
		if (!s) return 0;
		const lines = s.split("\n").map((l) => l.trim()).filter(Boolean);
		const bulletLines = lines.filter((l) => /^[-*]\s/.test(l) || l === "-" || l === "*");
		if (bulletLines.length > 0) {
			let total = 0;
			for (const bl of bulletLines) {
				const content = bl.replace(/^[-*]\s*/, "").trim();
				if (!content) {
					total += 1;
					continue;
				}
				if (content.includes(",")) {
					const parts = content.split(",").map((p) => p.trim()).filter(Boolean);
					total += parts.length;
				} else if (content.includes("/") && content.includes(" ")) {
					const tokens = content.split(/\s+/).filter(Boolean);
					const slashTokens = tokens.filter((t) => t.includes("/"));
					if (slashTokens.length > 0) total += slashTokens.length;
					else total += 1;
				} else {
					total += 1;
				}
			}
			// also consider if there are non-bullet lines that contain paths on same inline before bullets?
			// e.g., "file1, file2\n- file3" — count comma parts + bullets. For simplicity, if bulletLines not covers all lines, add comma count for remaining
			if (bulletLines.length !== lines.length) {
				const remaining = lines.filter((l) => !/^[-*]\s/.test(l) && l !== "-" && l !== "*").join(" ");
				if (remaining.includes(",")) {
					const parts = remaining.split(",").map((p) => p.trim()).filter(Boolean);
					// avoid double count if remaining already counted? We already counted bullets only, so add remaining parts
					// but if remaining is the same as bullet content, skip.
					// Simpler: just add if remaining not empty and not already bullet
					total += parts.length;
				} else if (remaining.includes("/")) {
					const tokens = remaining.split(/\s+/).filter((t) => t.includes("/"));
					if (tokens.length > 0) total += tokens.length;
				}
			}
			return total;
		}
		if (s.includes(",")) {
			const parts = s.split(",").map((p) => p.trim()).filter(Boolean);
			return parts.length;
		}
		const tokens = s.split(/\s+/).filter(Boolean);
		const slashTokens = tokens.filter((t) => t.includes("/"));
		if (slashTokens.length > 0) return slashTokens.length;
		if (s.includes("/")) return 1;
		return s.length > 0 ? 1 : 0;
	}

	function getArtifactsMetrics(text) {
		const section = extractArtifactsSection(text);
		const len = section.length;
		const count = countPaths(section);
		return { section, len, count };
	}

	function isThinSynthesis(text) {
		const s = String(text || "");
		// thin only if markers present (has at least Sub-agent Result + Artifacts/Paths) but metrics thin
		const hasMarkers = s.includes("Sub-agent Result") && s.includes("Artifacts/Paths");
		if (!hasMarkers) return false;
		// also require at least loose synthesis to consider it a synthesis block (but spec says thin when markers present)
		// we treat any text with markers present as candidate for thin check
		const metrics = getArtifactsMetrics(s);
		return metrics.count < 2 || metrics.len < 50;
	}

	function emitConcern(ctx, piRef, metrics) {
		const msg = `[biggz-synthesis-gate] concern: synthesis is thin (Artifacts/Paths count=${metrics.count}, len=${metrics.len} <2 or <50) — consider expanding Artifacts/Paths before asking. Advise mode (BIGGZ_ADVISE=1)`;
		console.warn(msg);
		try {
			ctx?.ui?.notify?.(msg, "warning");
		} catch {}
		try {
			piRef?.notify?.(msg, "warning");
		} catch {}
		try {
			// some pi versions expose ui at pi.ui
			piRef?.ui?.notify?.(msg, "warning");
		} catch {}
		return msg;
	}

	// Track assistant markdown via multiple possible event names (pi version tolerant)
	try {
		const events = ["assistant_message", "assistant", "message", "chunk"];
		for (const ev of events) {
			try {
				pi.on(ev, (event) => {
					try {
						// event may be string, object with text/content, or delta
						if (!event) return;
						if (typeof event === "string") {
							recordText(event);
							return;
						}
						// common shapes: {text}, {content:[{text}]}, {delta:{text}}, {message:{content}}
						const candidates = [
							event.text,
							event.content,
							event.delta?.text,
							event.message?.content,
							event.data?.text,
						];
						for (const c of candidates) {
							if (typeof c === "string" && c.includes("Sub-agent Result")) {
								recordText(c);
								return;
							}
							if (Array.isArray(c)) {
								for (const block of c) {
									if (block && typeof block.text === "string" && block.text.includes("Sub-agent Result")) {
										recordText(block.text);
										return;
									}
								}
							}
						}
						// fallback: if event looks like full text containing marker, record it
						if (typeof event.text === "string" && event.text.trim()) recordText(event.text);
						else if (typeof event.content === "string" && event.content.trim()) recordText(event.content);
					} catch {}
				});
			} catch {}
		}
	} catch {}

	// Also try to capture via pi.on("tool_call") context history fallback
	function getCtxHistory(ctx) {
		try {
			if (!ctx) return "";
			// ctx may expose messages/history/conversation
			const sources = [
				ctx.history,
				ctx.messages,
				ctx.conversation,
				ctx.getMessages?.(),
				ctx.getHistory?.(),
			];
			for (const src of sources) {
				if (!src) continue;
				if (typeof src === "string" && src.includes("Sub-agent Result")) return src;
				if (Array.isArray(src)) {
					const joined = src
						.map((m) => {
							if (!m) return "";
							if (typeof m === "string") return m;
							return m.text || m.content || m.message?.content || JSON.stringify(m);
						})
						.join("\n");
					if (joined.includes("Sub-agent Result")) return joined;
				}
			}
		} catch {}
		return "";
	}

	function getSynthesisSource(ctx) {
		const ctxText = getCtxHistory(ctx);
		if (ctxText && (hasSynthesis(ctxText) || hasSynthesisLoose(ctxText) || (ctxText.includes("Sub-agent Result") && ctxText.includes("Artifacts/Paths")))) {
			return ctxText;
		}
		if (lastAssistantMarkdown && Date.now() - lastUpdateTime < 120000) {
			if (hasSynthesis(lastAssistantMarkdown) || hasSynthesisLoose(lastAssistantMarkdown) || (lastAssistantMarkdown.includes("Sub-agent Result") && lastAssistantMarkdown.includes("Artifacts/Paths"))) return lastAssistantMarkdown;
		}
		if (lastAssistantMarkdown) {
			if (hasSynthesis(lastAssistantMarkdown) || hasSynthesisLoose(lastAssistantMarkdown) || (lastAssistantMarkdown.includes("Sub-agent Result") && lastAssistantMarkdown.includes("Artifacts/Paths"))) return lastAssistantMarkdown;
		}
		return "";
	}

	function checkSynthesisPrecondition(ctx) {
		// Prefer ctx history if available (most accurate for this turn)
		const ctxText = getCtxHistory(ctx);
		if (ctxText && hasSynthesis(ctxText)) return true;
		if (ctxText && hasSynthesisLoose(ctxText)) return true;
		if (ctxText && ctxText.includes("Sub-agent Result") && ctxText.includes("Artifacts/Paths")) return true;
		// Fallback to last recorded assistant markdown if recent (< 2 min)
		if (lastAssistantMarkdown && Date.now() - lastUpdateTime < 120000) {
			if (hasSynthesis(lastAssistantMarkdown) || hasSynthesisLoose(lastAssistantMarkdown)) return true;
			if (lastAssistantMarkdown.includes("Sub-agent Result") && lastAssistantMarkdown.includes("Artifacts/Paths")) return true;
		}
		// If we have no evidence, be permissive after 2 min but warn
		// Strict within same turn: if last markdown exists but lacks markers, fail
		if (lastAssistantMarkdown) {
			return hasSynthesis(lastAssistantMarkdown) || hasSynthesisLoose(lastAssistantMarkdown) || (lastAssistantMarkdown.includes("Sub-agent Result") && lastAssistantMarkdown.includes("Artifacts/Paths"));
		}
		return false;
	}

	// Expose helpers for testing (no network, fixture-only)
	try {
		pi._biggzSynthesisGate = {
			hasSynthesis,
			hasSynthesisLoose,
			extractArtifactsSection,
			countPaths,
			getArtifactsMetrics,
			isThinSynthesis,
			isAdviseEnabled,
			isChildBypass,
			getSynthesisSource,
			checkSynthesisPrecondition,
			emitConcern: (ctx, metrics) => emitConcern(ctx, pi, metrics),
			// test helpers to manipulate internal state
			_test: {
				recordText,
				getLast: () => lastAssistantMarkdown,
				setLast: (txt) => { lastAssistantMarkdown = txt; lastUpdateTime = Date.now(); },
				clearLast: () => { lastAssistantMarkdown = ""; lastUpdateTime = 0; },
			},
		};
	} catch {}

	// Wrap registerTool to enforce gate (primary path)
	try {
		if (typeof pi.registerTool === "function") {
			const origRegister = pi.registerTool.bind(pi);
			pi.registerTool = (def) => {
				try {
					if (
						def &&
						(def.name === "ask_user_question" || def.name === "question") &&
						typeof def.execute === "function"
					) {
						const origExecute = def.execute;
						def.execute = async (...args) => {
							// Child bypass — skip both blocking and advise entirely
							if (isChildBypass()) {
								return origExecute(...args);
							}
							// args: toolCallId, params, signal, onUpdate, ctx — ctx is last arg if object with ui/history
							let ctx = null;
							for (let i = args.length - 1; i >= 0; i--) {
								const a = args[i];
								if (a && typeof a === "object" && (a.ui || a.history || a.messages || a.conversation)) {
									ctx = a;
									break;
								}
							}
							// Also try args[args.length-1] as ctx fallback
							if (!ctx && args.length > 0) {
								const last = args[args.length - 1];
								if (last && typeof last === "object") ctx = last;
							}
							const has = checkSynthesisPrecondition(ctx);
							if (!has) {
								const reason =
									"Please synthesize before asking — missing ## Sub-agent Result block. Required markdown: ## Sub-agent Result: {phase/agent} + **Artifacts/Paths:** + **Risks / Open Questions:** + **Next Recommended:**. Emit markdown FIRST, adjacent, same turn, before ask_user_question/question.";
								console.error(`[biggz-synthesis-gate] blocked ${def.name}: ${reason}`);
								try {
									ctx?.ui?.notify?.(reason, "error");
								} catch {}
								try {
									pi.notify?.(reason, "error");
								} catch {}
								// Return error payload rather than throw to show in TUI as tool result error
								return {
									content: [{ type: "text", text: reason }],
									isError: true,
								};
							}
							// Has synthesis — check advise thin path (non-blocking concern)
							try {
								const source = getSynthesisSource(ctx);
								if (source && isAdviseEnabled() && isThinSynthesis(source)) {
									const metrics = getArtifactsMetrics(source);
									emitConcern(ctx, pi, metrics);
									// do not block — allow the call
								}
							} catch {}
							return origExecute(...args);
						};
					}
				} catch (e) {
					console.log(`[biggz-synthesis-gate] wrap error: ${e?.message || e}`);
				}
				return origRegister(def);
			};
			console.log("[biggz-synthesis-gate] wrapped registerTool for ask_user_question/question");
		}
	} catch (e) {
		console.log(`[biggz-synthesis-gate] failed to wrap registerTool: ${e?.message || e}`);
	}

	// Secondary guard via tool_call event (if extension loads after tools registered)
	try {
		if (typeof pi.on === "function") {
			pi.on("tool_call", async (event, ctx) => {
				try {
					if (isChildBypass()) return;
					const name = event?.toolName ?? event?.name ?? "";
					if (name !== "ask_user_question" && name !== "question") return;
					const has = checkSynthesisPrecondition(ctx);
					if (!has) {
						const warn =
							"[biggz-synthesis-gate] warning: ask_user_question/question called without preceding ## Sub-agent Result synthesis (Artifacts/Paths + Risks + Next missing)";
						console.warn(warn);
						try {
							ctx?.ui?.notify?.(warn, "warning");
						} catch {}
						try {
							pi.notify?.(warn, "warning");
						} catch {}
						return;
					}
					// Has synthesis — check thin + advise for concern (non-blocking)
					try {
						const source = getSynthesisSource(ctx);
						if (source && isAdviseEnabled() && isThinSynthesis(source)) {
							const metrics = getArtifactsMetrics(source);
							emitConcern(ctx, pi, metrics);
						}
					} catch {}
				} catch {}
			});
		}
	} catch {}

	pi.registerCommand?.("synthesis-gate-status", {
		description: "Show synthesis gate status (checks # Sub-agent Result before ask_user_question)",
		handler: async (_args, ctx) => {
			if (isChildBypass()) {
				const msg = "synthesis gate bypassed: PI_SUBAGENT_CHILD=1";
				ctx.ui.notify(msg, "info");
				return { content: [{ type: "text", text: msg }] };
			}
			const has = checkSynthesisPrecondition(ctx);
			if (!has) {
				const status = "✗ synthesis gate: missing ## Sub-agent Result — emit markdown before ask_user_question/question";
				ctx.ui.notify(status, "warning");
				return { content: [{ type: "text", text: status }] };
			}
			const source = getSynthesisSource(ctx);
			if (source && isAdviseEnabled() && isThinSynthesis(source)) {
				const metrics = getArtifactsMetrics(source);
				const status = `⚠ synthesis gate: thin synthesis (Artifacts/Paths count=${metrics.count}, len=${metrics.len}) — advise concern enabled (BIGGZ_ADVISE=1)`;
				ctx.ui.notify(status, "warning");
				return { content: [{ type: "text", text: status }] };
			}
			const status = "✓ synthesis gate: last markdown contains ## Sub-agent Result";
			ctx.ui.notify(status, has ? "info" : "warning");
			return { content: [{ type: "text", text: status }] };
		},
	});
}
