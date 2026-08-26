/**
 * biggz-synthesis-gate — Pi extension that enforces Post-Delegation Human Checkpoint.
 *
 * Ensures orchestrator emits synthesis markdown with ## Sub-agent Result + Artifacts/Paths + Risks + Next
 * BEFORE calling ask_user_question / question. Without synthesis the orchestrator would skip to tool call
 * and human loses artifacts/paths, risks, next.
 *
 * Behavior:
 * - Tracks last assistant markdown in this turn via pi.on("assistant_message") / pi.on("message") fallbacks.
 * - Wraps ask_user_question and question tools via pi.registerTool interception.
 * - On execute, verifies preceding markdown contains required markers.
 * - If missing, blocks with instructive error: "Please synthesize before asking — missing ## Sub-agent Result block".
 * - Also hooks pi.on("tool_call") as secondary guard to emit warning even if wrapping missed (load-order safe).
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

	function checkSynthesisPrecondition(ctx) {
		// Prefer ctx history if available (most accurate for this turn)
		const ctxText = getCtxHistory(ctx);
		if (ctxText && hasSynthesis(ctxText)) return true;
		if (ctxText && hasSynthesisLoose(ctxText)) return true;
		// Fallback to last recorded assistant markdown if recent (< 2 min)
		if (lastAssistantMarkdown && Date.now() - lastUpdateTime < 120000) {
			if (hasSynthesis(lastAssistantMarkdown) || hasSynthesisLoose(lastAssistantMarkdown)) return true;
		}
		// If we have no evidence, be permissive after 2 min but warn
		// Strict within same turn: if last markdown exists but lacks markers, fail
		if (lastAssistantMarkdown) {
			return hasSynthesis(lastAssistantMarkdown) || hasSynthesisLoose(lastAssistantMarkdown);
		}
		return false;
	}

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
								// Return error payload rather than throw to show in TUI as tool result error
								return {
									content: [{ type: "text", text: reason }],
									isError: true,
								};
							}
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
					const name = event?.toolName ?? event?.name ?? "";
					if (name !== "ask_user_question" && name !== "question") return;
					if (checkSynthesisPrecondition(ctx)) return;
					// Emit warning — actual block is via wrapper; this is best-effort visibility
					const warn =
						"[biggz-synthesis-gate] warning: ask_user_question/question called without preceding ## Sub-agent Result synthesis (Artifacts/Paths + Risks + Next missing)";
					console.warn(warn);
					try {
						ctx?.ui?.notify?.(warn, "warning");
					} catch {}
				} catch {}
			});
		}
	} catch {}

	pi.registerCommand?.("synthesis-gate-status", {
		description: "Show synthesis gate status (checks # Sub-agent Result before ask_user_question)",
		handler: async (_args, ctx) => {
			const has = checkSynthesisPrecondition(ctx);
			const status = has
				? "✓ synthesis gate: last markdown contains ## Sub-agent Result"
				: "✗ synthesis gate: missing ## Sub-agent Result — emit markdown before ask_user_question/question";
			ctx.ui.notify(status, has ? "info" : "warning");
			return { content: [{ type: "text", text: status }] };
		},
	});
}
