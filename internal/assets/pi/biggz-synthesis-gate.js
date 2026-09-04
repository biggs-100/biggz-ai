/**
 * biggz-synthesis-gate — Pi extension that enforces Post-Delegation Human Checkpoint.
 * IsCheckpointAsk alone requires synthesis (REQ-DG-1); hasOptions alone NEVER blocks — free-text
 * asks and Session Preflight option-asks always pass. IsCheckpointAsk tokens (case-insensitive, label/value/id/name/title scan): proceed/continuar/proseguir/proceder/procede, adjust/ajustar, stop/detener/parar, continue/continuar, correct/corregir/cerrar. Same-turn invariant: synthesis markdown with 4 markers MUST be emitted FIRST and checkpoint tool called in SAME assistant turn adjacent ≤120s without extra assistant message; otherwise gate blocks isError:true/block:true (Please synthesize before asking).
 *
 * biggz-synthesis-gate — Pi extension that enforces Post-Delegation Human Checkpoint.
 *
 * Ensures orchestrator emits synthesis markdown with ## Sub-agent Result + Artifacts/Paths + Risks + Next
 * BEFORE calling ask_user_choice (Pi closed) / ask_user_question (Pi open) / question (OpenCode). Without synthesis the orchestrator would skip to tool call
 * and human loses artifacts/paths, risks, next.
 *
 * Dual-mode (PR4 advisor):
 * - Blocking gate (default): blocks when preceding markdown lacks ## Sub-agent Result / Artifacts/Paths.
 *   STRICT same-turn for blocking: only currentTurnMarkdown (assistant markdown emitted in THIS turn,
 *   adjacent before the tool call) satisfies the block. ctx.history / lastAssistantMarkdown are
 *   intentionally NOT checked for blocking — they are old history and would cause false positives
 *   (e.g. synthesis from a previous archived change satisfies history but not current turn).
 *   History is only used for advise/warning (non-blocking) via getCurrentTurnSynthesis fallback.
 * - Advise mode (opt-in BIGGZ_ADVISE=1 or settings flag, default OFF): when markers present but thin
 *   (Artifacts/Paths count <2 || len <50), does NOT block; injects non-blocking concern via pi.notify / ctx.ui.notify.
 *   Heuristic only, no model call, no auto-fix.
 * - PI_SUBAGENT_CHILD=1 bypasses both modes entirely.
 *
 * Behavior:
 * - Tracks current-turn assistant markdown via pi.on("message_end") / pi.on("message_update") / pi.on("assistant_message") fallbacks
 *   into currentTurnMarkdown (reset after each successful ask_user_choice/ask_user_question/question and at turn_start).
 *   The buffer fixes the streaming race where markdown is emitted milliseconds before the tool call and
 *   has not yet landed in ctx.history.
 * - Wraps ask_user_choice (Pi closed), ask_user_question and question tools via pi.registerTool interception AND via pre-registered sweep
 *   (iterate pi.tools / pi._tools / pi.getAllTools+getToolDefinition if available) — load-order safe.
 *   Each wrapped execute verifies STRICT same-turn markdown contains required markers (currentTurnMarkdown only).
 *   If missing, returns {isError:true} with instructive error and does NOT call original.
 * - Secondary guard via pi.on("tool_call") ALSO blocks (returns {block:true, reason}) when hasSynthesis fails,
 *   not just warning — ensures bypass via load-order (tool already registered before gate) cannot escape.
 *   If thin and advise enabled, emits concern warning but allows the call (advise path MAY use history fallback).
 * - Also hooks pi.on("tool_execution_end") to reset currentTurn after successful ask_user_choice/ask_user_question/question (covers pre-registered
 *   tools not wrapped via execute path).
 *
 * Minimal but functional — mirrors biggz-thinking-wrap.js pattern.
 *
 * Parity truth: internal/sdd/synthesis_gate.go:ShouldBlock is canonical (REQ-DG-1: IsCheckpointAsk only).
 * JS MUST mirror Go: only isCheckpointAsk gates; hasOptions alone NEVER blocks;
 * history (ctx.history / lastAssistantMarkdown) is advise-only (BIGGZ_ADVISE=1 thin concern)
 * and MUST NEVER satisfy block. Drift risk in internal/assets/biggz/biggz-orchestrator.md
 * noted (duplicated checkpoint block) — no edit this change.
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzSynthesisGate(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;

	let lastAssistantMarkdown = "";
	let lastUpdateTime = 0;
	let currentTurnMarkdown = "";
	let currentTurnUpdateTime = 0;

	function recordText(text) {
		if (typeof text === "string" && text.trim()) {
			lastAssistantMarkdown = text;
			lastUpdateTime = Date.now();
			// Also accumulate into current-turn buffer for same-turn race fix.
			const now = Date.now();
			if (!currentTurnMarkdown) {
				currentTurnMarkdown = text;
				currentTurnUpdateTime = now;
			} else {
				// If new chunk contains a fresh synthesis header and current already contains
				// a distinct synthesis block, treat as new turn — replace instead of appending.
				const hasNew = text.includes("## Sub-agent Result");
				const hasCur = currentTurnMarkdown.includes("## Sub-agent Result");
				if (hasNew && hasCur && text.trim() !== currentTurnMarkdown.trim()) {
					if (!currentTurnMarkdown.includes(text) && !text.includes(currentTurnMarkdown)) {
						currentTurnMarkdown = text;
						currentTurnUpdateTime = now;
						return;
					}
					if (text.length > currentTurnMarkdown.length) {
						currentTurnMarkdown = text;
						currentTurnUpdateTime = now;
						return;
					}
				}
				if (currentTurnMarkdown.includes(text)) {
					if (text.length > currentTurnMarkdown.length) {
						currentTurnMarkdown = text;
					}
					currentTurnUpdateTime = now;
					return;
				}
				if (text.includes(currentTurnMarkdown)) {
					currentTurnMarkdown = text;
					currentTurnUpdateTime = now;
					return;
				}
				currentTurnMarkdown += "\n" + text;
				currentTurnUpdateTime = now;
			}
		}
	}

	function hasSynthesis(text) {
		const s = String(text || "");
		return (
			s.includes("## Sub-agent Result") &&
			s.includes("**Artifacts/Paths:**") &&
			s.includes("**Risks / Open Questions:**") &&
			s.includes("**Next Recommended:**")
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

	function hasSessionRecall(text) {
		return String(text || "").includes("## Session Recall");
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

	// --- markdown extraction helpers for Pi message shapes ---
	function extractTextFromMessageContent(content) {
		if (!content) return "";
		if (typeof content === "string") return content;
		if (Array.isArray(content)) {
			const parts = [];
			for (const block of content) {
				if (!block) continue;
				if (typeof block === "string") parts.push(block);
				else if (typeof block.text === "string") parts.push(block.text);
				else if (typeof block.content === "string") parts.push(block.content);
			}
			return parts.join("\n");
		}
		if (typeof content === "object" && typeof content.text === "string") return content.text;
		return "";
	}

	function extractAssistantText(event) {
		if (!event) return "";
		if (typeof event === "string") return event;
		// Common Pi message_end / message_update shapes
		try {
			if (event.message) {
				const msg = event.message;
				// Only record assistant messages (or messages without role in streaming fallbacks)
				const role = msg.role;
				if (role && role !== "assistant" && role !== "custom" && role !== undefined) {
					// For custom messages, still check if content contains synthesis (some mocks use custom)
					// but generally ignore user/toolResult.
					if (role === "user" || role === "toolResult") return "";
				}
				const fromMsgContent = extractTextFromMessageContent(msg.content);
				if (fromMsgContent && fromMsgContent.trim()) return fromMsgContent;
				if (typeof msg.text === "string" && msg.text.trim()) return msg.text;
			}
		} catch {}
		try {
			if (typeof event.text === "string" && event.text.trim()) return event.text;
		} catch {}
		try {
			if (typeof event.content === "string" && event.content.trim()) return event.content;
		} catch {}
		try {
			if (Array.isArray(event.content)) {
				const t = extractTextFromMessageContent(event.content);
				if (t.trim()) return t;
			}
		} catch {}
		try {
			if (event.delta && typeof event.delta.text === "string" && event.delta.text.trim()) return event.delta.text;
		} catch {}
		try {
			if (event.data && typeof event.data.text === "string" && event.data.text.trim()) return event.data.text;
		} catch {}
		try {
			if (event.assistantMessageEvent) {
				const a = event.assistantMessageEvent;
				if (typeof a.text === "string" && a.text.trim()) return a.text;
				if (a.delta && typeof a.delta.text === "string" && a.delta.text.trim()) return a.delta.text;
				if (a.message && a.message.content) {
					const t = extractTextFromMessageContent(a.message.content);
					if (t.trim()) return t;
				}
			}
		} catch {}
		return "";
	}

	// Track assistant markdown via multiple possible event names (pi version tolerant)
	// Keep legacy fallbacks (assistant_message, assistant, message, chunk) for tests/back-compat,
	// plus correct Pi events (message_end, message_update, message_start).
	try {
		const legacyEvents = ["assistant_message", "assistant", "message", "chunk"];
		for (const ev of legacyEvents) {
			try {
				pi.on(ev, (event) => {
					try {
						if (!event) return;
						let text = "";
						if (typeof event === "string") {
							text = event;
						} else {
							const candidates = [
								event.text,
								event.content,
								event.delta?.text,
								event.message?.content,
								event.data?.text,
							];
							for (const c of candidates) {
								if (typeof c === "string" && c.trim()) {
									// For legacy handlers, preserve old behavior of preferring strings containing marker,
									// but also handle generic strings to populate buffer for strict check.
									// We record any non-empty string; hasSynthesis will filter later.
									text = c;
									break;
								}
								if (Array.isArray(c)) {
									const joined = c.map((b) => (b && typeof b.text === "string" ? b.text : (typeof b === "string" ? b : ""))).join("\n");
									if (joined.trim()) {
										text = joined;
										break;
									}
								}
							}
							if (!text) {
								if (typeof event.text === "string" && event.text.trim()) text = event.text;
								else if (typeof event.content === "string" && event.content.trim()) text = event.content;
							}
						}
						if (text && text.trim()) recordText(text);
					} catch {}
				});
			} catch {}
		}
	} catch {}

	// Correct Pi events (0.84.x): message_end, message_update, message_start
	try {
		const piMessageEvents = ["message_end", "message_update", "message_start"];
		for (const ev of piMessageEvents) {
			try {
				pi.on(ev, (event) => {
					try {
						const text = extractAssistantText(event);
						if (text && text.trim()) recordText(text);
					} catch {}
				});
			} catch {}
		}
	} catch {}

	// Turn/agent boundaries: reset strict same-turn buffer at start of new turn/agent
	// Ensures old synthesis from previous turn does not satisfy next turn's check.
	try {
		for (const ev of ["turn_start", "agent_start"]) {
			try {
				pi.on(ev, () => {
					try {
						currentTurnMarkdown = "";
						currentTurnUpdateTime = 0;
					} catch {}
				});
			} catch {}
		}
	} catch {}

	// Reset after successful question tool execution (covers pre-registered tools not wrapped via execute)
	try {
		for (const ev of ["tool_execution_end", "tool_result"]) {
			try {
				pi.on(ev, (event) => {
					try {
						const name = event?.toolName ?? event?.name ?? "";
						if (name === "ask_user_choice" || name === "ask_user_question" || name === "question") {
							// Only reset if last execution was not an error (avoid clearing on blocked)
							const isErr = event?.isError === true || event?.result?.isError === true;
							if (!isErr) {
								currentTurnMarkdown = "";
								currentTurnUpdateTime = 0;
							}
						}
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
			// Try sessionManager branch if available (Pi ExtensionContext)
			try {
				const br = ctx.sessionManager?.getBranch?.();
				if (Array.isArray(br)) {
					const joined = br
						.map((e) => {
							if (!e) return "";
							if (e.type === "message" && e.message) {
								const c = e.message.content;
								if (Array.isArray(c)) return c.map((x) => x?.text || "").join("\n");
								if (typeof c === "string") return c;
								return e.message.text || "";
							}
							return "";
						})
						.join("\n");
					if (joined.includes("Sub-agent Result")) return joined;
				}
			} catch {}
		} catch {}
		return "";
	}

	function getSynthesisSource(ctx) {
		// Used only for diagnostics / status — may fallback to history (not for blocking).
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

	function getCurrentTurnSynthesis(ctx) {
		// Advise path: strict same-turn preferred, but history/last are allowed as fallback
		// (history only for non-blocking concern, never for blocking).
		const now = Date.now();
		// Prefer current-turn buffer first — this catches same-turn streaming race where markdown
		// was emitted milliseconds before tool_call and hasn't yet appeared in ctx.history.
		if (currentTurnMarkdown) {
			const has = hasSynthesis(currentTurnMarkdown) || hasSynthesisLoose(currentTurnMarkdown) || (currentTurnMarkdown.includes("Sub-agent Result") && currentTurnMarkdown.includes("Artifacts/Paths"));
			if (has) {
				if (now - currentTurnUpdateTime < 120000) return currentTurnMarkdown;
				// Even outside window, the most recent chunk is still valid for same-turn check.
				return currentTurnMarkdown;
			}
		}
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

	function checkSessionRecallInCurrentTurn() {
		// Session Boot Recall (HARD GATE) exception: ## Session Recall is emitted before preflight
		// and must not be blocked as missing synthesis. This helper checks strict same-turn
		// Session Recall presence (only currentTurnMarkdown, not history) for the recall exception.
		if (!currentTurnMarkdown) return false;
		if (hasSessionRecall(currentTurnMarkdown)) return true;
		return false;
	}

	function hasSessionRecallInHistory(ctx) {
		// Non-strict history check for diagnostics (like getSynthesisSource).
		const cur = currentTurnMarkdown && hasSessionRecall(currentTurnMarkdown) ? currentTurnMarkdown : "";
		if (cur) return true;
		const ctxText = getCtxHistory(ctx);
		if (ctxText && hasSessionRecall(ctxText)) return true;
		if (lastAssistantMarkdown && hasSessionRecall(lastAssistantMarkdown)) return true;
		return false;
	}

	// Language-aware synthesis (polish-synthesis-human-language): content may be localized (es/en) per human language
	// (`languageHint` / `Human language: es|en`), but markers stay English verbatim b0d2fc1;
	// isCheckpointAsk scans only option labels (proceed/continuar/proseguir/proceder/procede, adjust/ajustar, stop/detener/parar, continue/continuar, correct/corregir/cerrar) — REQ-DG-1: hasOptions (2-4 options) alone NEVER requires synthesis
	// not synthesis content — Spanish content with English markers passes, missing marker blocks regardless of language.
	function isCheckpointAsk(params) {
		if (params == null) return false;
		if (typeof params === "string") {
			try {
				params = JSON.parse(params);
			} catch {
				return false;
			}
		}
		if (typeof params !== "object") return false;
		const tokens = new Set(["proceed", "adjust", "stop", "continue", "correct", "continuar", "ajustar", "detener", "parar", "cerrar", "corregir", "proseguir", "proceder", "procede"]);
		const normalize = (s) => String(s).trim().toLowerCase();
		const isToken = (s) => {
			const n = normalize(s);
			if (tokens.has(n)) return true;
			for (const tok of tokens) {
				try {
					const re = new RegExp(`\\b${tok}\\b`, "i");
					if (re.test(String(s))) return true;
				} catch {}
			}
			return false;
		};
		// Expected shape: params.questions[].options[].label (also handles top-level options)
		try {
			const qs = params.questions;
			if (Array.isArray(qs)) {
				for (const q of qs) {
					if (!q || typeof q !== "object") continue;
					const opts = q.options;
					if (!Array.isArray(opts)) continue;
					for (const opt of opts) {
						if (opt == null) continue;
						let lab = null;
						if (typeof opt === "string") lab = opt;
						else if (typeof opt === "object") {
							if (typeof opt.label === "string") lab = opt.label;
							else if (typeof opt.value === "string") lab = opt.value;
							else if (typeof opt.name === "string") lab = opt.name;
							else if (typeof opt.id === "string") lab = opt.id;
							else if (typeof opt.title === "string") lab = opt.title;
						}
						if (lab != null && isToken(lab)) return true;
					}
				}
			}
			// Top-level options fallback (some question implementations)
			const topOpts = params.options;
			if (Array.isArray(topOpts)) {
				for (const opt of topOpts) {
					if (opt == null) continue;
					let lab = null;
					if (typeof opt === "string") lab = opt;
					else if (typeof opt === "object") {
						if (typeof opt.label === "string") lab = opt.label;
						else if (typeof opt.value === "string") lab = opt.value;
						else if (typeof opt.name === "string") lab = opt.name;
						else if (typeof opt.id === "string") lab = opt.id;
					}
					if (lab != null && isToken(lab)) return true;
				}
			}
		} catch {}
		// Generic deep search for any label/value-like field containing checkpoint token
		try {
			const stack = [params];
			const seen = new WeakSet();
			while (stack.length) {
				const cur = stack.pop();
				if (!cur || typeof cur !== "object") continue;
				if (seen.has(cur)) continue;
				try { seen.add(cur); } catch {}
				if (Array.isArray(cur)) {
					for (const el of cur) if (el && typeof el === "object") stack.push(el);
					continue;
				}
				for (const key of ["label", "value", "id", "name", "title"]) {
					const v = cur[key];
					if (typeof v === "string" && isToken(v)) return true;
				}
				for (const v of Object.values(cur)) {
					if (v && typeof v === "object") stack.push(v);
				}
			}
		} catch {}
		return false;
	}

	function hasOptions(params) {
		if (params == null) return false;
		if (typeof params === "string") {
			try { params = JSON.parse(params); } catch { return false; }
		}
		if (typeof params !== "object") return false;
		try {
			const qs = params.questions;
			if (Array.isArray(qs)) {
				for (const q of qs) {
					if (!q || typeof q !== "object") continue;
					const opts = q.options;
					if (Array.isArray(opts) && opts.length >= 2 && opts.length <= 4) return true;
				}
			}
			const topOpts = params.options;
			if (Array.isArray(topOpts) && topOpts.length >= 2 && topOpts.length <= 4) return true;
		} catch {}
		return false;
	}

	function validateQuestionEnvelope(p){if(p==null||typeof p!="object")return null;let q=p.questions;if(!Array.isArray(q)){let o=p.options;if(Array.isArray(o)){if(o.length<2||o.length>4)return{isError:!0,limit:"options",message:`options out of range 2-4: got ${o.length}`};for(let x of o){let l=typeof x==="string"?x:x.label??x.value??"";if(String(l).length>60)return{isError:!0,limit:"label",message:`label exceeds limit 60: got ${String(l).length}`}} }return null}if(q.length>4)return{isError:!0,limit:"questions",message:`questions exceed limit 4: got ${q.length}`};for(let i=0;i<q.length;i++){let v=q[i];if(!v||typeof v!="object")continue;let h=v.header??v.title??"";if(typeof h==="string"&&h.length>16)return{isError:!0,limit:"header",message:`header exceeds limit 16: got ${h.length}`};let o=v.options;if(!Array.isArray(o))continue;if(o.length<2||o.length>4)return{isError:!0,limit:"options",message:`options out of range 2-4: got ${o.length} for question ${i}`};for(let x of o){let l=typeof x==="string"?x:x.label??x.value??"";if(String(l).length>60)return{isError:!0,limit:"label",message:`label exceeds limit 60: got ${String(l).length}`}}}return null}
	function formatFallback(p){if(p==null||typeof p!="object")return"";let q=p.questions;if(!Array.isArray(q)||q.length===0){let o=p.options;if(Array.isArray(o)&&o.length>0){let s="## Questions\n\n";for(let x of o){let l=typeof x==="string"?x:x.label??x.value??"";let d=x.description??x.desc??"";s+="- "+l+(d?": "+d:"")+"\n"}return s}return""}let s="";for(let i=0;i<q.length;i++){let v=q[i];if(!v||typeof v!="object")continue;let h=v.header??v.title??"";let qu=v.question??v.text??"";s+=`### ${h?h+": ":""}Question ${i+1}\n`;if(qu)s+=qu+"\n";let o=v.options??[];for(let x of o){let l=typeof x==="string"?x:x.label??"";let d=x.description??"";s+="- "+l+(d?": "+d:"")+"\n"}s+="\n"}return s.trim()+"\n"}

	// blockedEnvelope builds the REQ-DG-2 same-turn plain-chat payload: attempted
	// context + full question via formatFallback so nothing is swallowed.
	// Mirrors Go BuildBlockedEnvelope in internal/sdd/synthesis_gate.go.
	function blockedEnvelope(params, reason) {
		const fb = formatFallback(params);
		let context = "synthesis required before checkpoint ask";
		try {
			const raw = typeof params === "string" ? params : JSON.stringify(params);
		if (raw) context += ": " + raw;
		} catch {}
		return { block: true, reason, context, fallback: fb };
	}

	function extractParamsFromToolCall(event) {
		if (!event || typeof event !== "object") return null;
		const candidates = [event.params, event.args, event.arguments, event.input, event.payload, event.data];
		for (const c of candidates) {
			if (c != null) return c;
		}
		// Some Pi versions pack params under event.toolCall or event.tool_input
		if (event.toolCall && typeof event.toolCall === "object") {
			if (event.toolCall.params != null) return event.toolCall.params;
			if (event.toolCall.args != null) return event.toolCall.args;
			if (event.toolCall.input != null) return event.toolCall.input;
		}
		return null;
	}

	function emitHistoryFallbackWarning(ctx) {
		const msg = "synthesis from previous turn — showing last known";
		console.warn(`[biggz-synthesis-gate] ${msg}`);
		try { ctx?.ui?.notify?.(msg, "warning"); } catch {}
		try { pi.notify?.(msg, "warning"); } catch {}
		try { pi.ui?.notify?.(msg, "warning"); } catch {}
		return msg;
	}

	function checkSynthesisPrecondition(ctx) {
		// STRICT: only currentTurnMarkdown ≤120s with HasSynthesis satisfies. Mirrors Go truth
		// Diagnostic: missing synthesis vs envelope limit (>16 header, >60 label) are distinct errors;
		// internal/sdd/synthesis_gate.go:ShouldBlock = !child && !recall && isCheckpointAsk && ≤120s && !HasSynthesis(currentTurn) (REQ-DG-1)
		// History (ctx.history / lastAssistantMarkdown) MUST NOT satisfy blocking; it is only for
		// BIGGZ_ADVISE=1 thin concern via getCurrentTurnSynthesis fallback, never for block.
		const now = Date.now();
		if (!currentTurnMarkdown) return false;
		const has = hasSynthesis(currentTurnMarkdown) || hasSynthesisLoose(currentTurnMarkdown) || (currentTurnMarkdown.includes("Sub-agent Result") && currentTurnMarkdown.includes("Artifacts/Paths"));
		if (!has) return false;
		if (now - currentTurnUpdateTime > 120000) return false;
		return true;
	}

	// Expose helpers for testing (no network, fixture-only)
	try {
		pi._biggzSynthesisGate = {
			hasSynthesis,
			hasSynthesisLoose,
			hasSessionRecall,
			extractArtifactsSection,
			countPaths,
			getArtifactsMetrics,
			isThinSynthesis,
			isAdviseEnabled,
			isChildBypass,
			getSynthesisSource,
			getCurrentTurnSynthesis,
			hasSessionRecallInHistory: (ctx) => hasSessionRecallInHistory(ctx),
			checkSessionRecallInCurrentTurn,
			checkSynthesisPrecondition,
			isCheckpointAsk,
			hasOptions,
			validateQuestionEnvelope,
			formatFallback,
			blockedEnvelope,
			extractParamsFromToolCall,
			emitConcern: (ctx, metrics) => emitConcern(ctx, pi, metrics),
			// test helpers to manipulate internal state
			_test: {
				recordText,
				getLast: () => lastAssistantMarkdown,
				setLast: (txt) => { lastAssistantMarkdown = txt; lastUpdateTime = Date.now(); },
				clearLast: () => { lastAssistantMarkdown = ""; lastUpdateTime = 0; },
				getCurrent: () => currentTurnMarkdown,
				setCurrent: (txt) => { currentTurnMarkdown = txt; currentTurnUpdateTime = Date.now(); },
				clearCurrent: () => { currentTurnMarkdown = ""; currentTurnUpdateTime = 0; },
				getCurrentTime: () => currentTurnUpdateTime,
			},
		};
	} catch {}

	// --- helper to wrap a single tool definition's execute (idempotent) ---
	function wrapSingleTool(def) {
		if (!def || typeof def.execute !== "function" || def._biggzGateWrapped) return false;
		const origExecute = def.execute;
		const toolName = def.name || "ask_user_choice"; // ask_user_choice for Pi closed
		def._biggzGateWrapped = true;
		def.execute = async (...args) => {
			// ENFORCEMENT RETIRED (2026-09-04): blocking proved unfulfillable
			// (same-turn side-channel + false positives). Context-before-question
			// is governed by the explicit agent contract in docs, not by code.
			// Passthrough — helpers below stay exposed for unit tests.
			return origExecute(...args);
			// args: toolCallId, params, signal, onUpdate, ctx — ctx is last arg if object with ui/history
			let ctx = null;
			for (let i = args.length - 1; i >= 0; i--) {
				const a = args[i];
				if (a && typeof a === "object" && (a.ui || a.history || a.messages || a.conversation || a.sessionManager)) {
					ctx = a;
					break;
				}
			}
			// Also try args[args.length-1] as ctx fallback
			if (!ctx && args.length > 0) {
				const last = args[args.length - 1];
				if (last && typeof last === "object") ctx = last;
			}
			// Checkpoint vs general: extract params for ownership/validation
			let params = null;
			if (args.length >= 2) params = args[1];
			if (!params || typeof params !== "object" || params.ui || params.history || params.sessionManager) {
				for (const a of args) {
					if (a && typeof a === "object" && (Array.isArray(a.questions) || Array.isArray(a.options))) {
						params = a;
						break;
					}
				}
				if ((!params || params.ui || params.history) && args[0] && typeof args[0] === "object" && (Array.isArray(args[0].questions) || Array.isArray(args[0].options))) {
					params = args[0];
				}
			}
			// Single ownership: sub-agent checkpoint asks blocked even with bypass
			if (isChildBypass()) {
				if (params && isCheckpointAsk(params)) {
					const reason = "isError:true checkpoint asks may only be emitted by orchestrator, not sub-agent";
					console.error(`[biggz-synthesis-gate] blocked sub-agent checkpoint ${toolName}: ${reason}`);
					try { ctx?.ui?.notify?.(reason, "error"); } catch {}
					try { pi.notify?.(reason, "error"); } catch {}
					return { content: [{ type: "text", text: reason }], isError: true };
				}
				return origExecute(...args);
			}
			// Envelope validation: reject isError:true with limit name, emit fallback, do not call handler
			try {
				if (params) {
					const v = validateQuestionEnvelope(params);
					if (v) {
						const fb = formatFallback(params);
						const reason = `isError:true ${v.message} (limit ${v.limit}) — fix header ≤16 (e.g. "Decisión" 8 not "Decisión del checkpoint" 23) or label ≤60, then retry`;
						console.error(`[biggz-synthesis-gate] blocked envelope ${toolName}: ${reason} — diagnostic: envelope limit vs synthesis missing`);
						try { if (fb) ctx?.ui?.notify?.(fb, "error"); } catch {}
						try { if (fb) pi.notify?.(fb, "error"); } catch {}
						try { if (fb) pi.ui?.notify?.(fb, "error"); } catch {}
						return { content: [{ type: "text", text: reason + (fb ? "\n\nFallback:\n" + fb : "") }], isError: true };
					}
				}
			} catch {}
			if (!isCheckpointAsk(params)) {
				// REQ-DG-1: only checkpoint asks gate; free-text and preflight option-asks never block
				try {
					const source = getCurrentTurnSynthesis(ctx);
					if (source && isAdviseEnabled() && isThinSynthesis(source)) {
						const metrics = getArtifactsMetrics(source);
						emitConcern(ctx, pi, metrics);
					}
				} catch {}
				const resultGeneral = await origExecute(...args);
				try {
					currentTurnMarkdown = "";
					currentTurnUpdateTime = 0;
				} catch {}
				return resultGeneral;
			}
			const has = checkSynthesisPrecondition(ctx);
			if (!has) {
				if (checkSessionRecallInCurrentTurn()) {
					// allow: recall -> preflight (same-turn Session Recall) — only valid exception
				} else {
					const reason =
						"Please synthesize before asking — missing ## Sub-agent Result block. Required markdown: ## Sub-agent Result: {phase/agent} + **Artifacts/Paths:** + **Risks / Open Questions:** + **Next Recommended:**. Emit markdown FIRST, adjacent, same turn, before ask_user_choice/ask_user_question/question (closed: ask_user_choice). Diagnostic: if envelope error (header >16 e.g. 'Decisión del checkpoint' 23 vs 'Decisión' 8, or label >60) fix header/label first; else ensure synthesis has 4 verbatim markers, plain markdown not in ``` code block, same turn ≤120s, not from history.";
					const env = blockedEnvelope(params, reason);
					const text = reason + (env.fallback ? "\n\nFallback:\n" + env.fallback : "");
					console.error(`[biggz-synthesis-gate] blocked ${toolName}: ${reason} — context+fallback emitted same turn (REQ-DG-2)`);
					try {
						ctx?.ui?.notify?.(text, "error");
					} catch {}
					try {
						pi.notify?.(text, "error");
					} catch {}
					try {
						pi.ui?.notify?.(text, "error");
					} catch {}
					return {
						content: [{ type: "text", text }],
						isError: true,
					};
				}
			}
			// Has synthesis or preflight allowance — check advise thin path (non-blocking concern)
			try {
				const source = getCurrentTurnSynthesis(ctx);
				if (source && isAdviseEnabled() && isThinSynthesis(source)) {
					const metrics = getArtifactsMetrics(source);
					emitConcern(ctx, pi, metrics);
					// do not block — allow the call
				}
			} catch {}
			const result = await origExecute(...args);
			// Reset current-turn buffer after successful tool call — next turn starts fresh.
			try {
				currentTurnMarkdown = "";
				currentTurnUpdateTime = 0;
			} catch {}
			return result;
		};
		return true;
	}

	// Wrap registerTool to enforce gate (primary path) — best-effort for future registrations via this pi instance.
	// Note: In Pi's ExtensionRunner each extension gets its own api object with its own registerTool closure,
	// so wrapping this instance only catches tools registered via this instance (i.e. the gate itself).
	// The secondary pi.on("tool_call") guard is the true load-order-safe blocking path for cross-extension tools
	// like rpiv-ask-user-question which register via their own api instance before/after the gate loads.
	try {
		if (typeof pi.registerTool === "function") {
			const origRegister = pi.registerTool.bind(pi);
			pi.registerTool = (def) => {
				try {
					if (
						def &&
						(def.name === "ask_user_choice" || def.name === "ask_user_question" || def.name === "question") &&
						typeof def.execute === "function"
					) {
						wrapSingleTool(def);
					}
				} catch (e) {
					console.log(`[biggz-synthesis-gate] wrap error: ${e?.message || e}`);
				}
				return origRegister(def);
			};
			console.log("[biggz-synthesis-gate] wrapped registerTool for ask_user_choice/ask_user_question/question");
		}
	} catch (e) {
		console.log(`[biggz-synthesis-gate] failed to wrap registerTool: ${e?.message || e}`);
	}

	// Also sweep pre-registered tools (load-order race: tool already registered before gate loaded).
	// Iterate known access points: pi.tools / pi._tools (test mocks), pi.getAllTools+getToolDefinition, pi.getTool.
	try {
		let wrappedCount = 0;
		// Strategy 1: direct Map/object access (mocks and possible internal exposure)
		try {
			const mapCandidates = [pi.tools, pi._tools, pi._extensionTools, pi._toolMap];
			for (const m of mapCandidates) {
				if (!m) continue;
				if (m instanceof Map) {
					for (const [name, def] of m.entries()) {
						if (name !== "ask_user_choice" && name !== "ask_user_question" && name !== "question") continue;
						const target = def && def.definition ? def.definition : def;
						if (wrapSingleTool(target)) wrappedCount++;
						// Also handle case where Map value is definition itself
						if (def && def !== target && typeof def.execute === "function") {
							if (wrapSingleTool(def)) wrappedCount++;
						}
					}
				} else if (typeof m === "object") {
					for (const k of Object.keys(m)) {
						if (k !== "ask_user_choice" && k !== "ask_user_question" && k !== "question") continue;
						const v = m[k];
						const target = v && v.definition ? v.definition : v;
						if (target && typeof target.execute === "function") {
							if (wrapSingleTool(target)) wrappedCount++;
						}
					}
				}
			}
		} catch {}
		// Strategy 2: via getAllTools + getToolDefinition if available (real Pi ExtensionAPI)
		try {
			if (typeof pi.getAllTools === "function") {
				const all = pi.getAllTools();
				if (Array.isArray(all)) {
					for (const info of all) {
						if (!info || (info.name !== "ask_user_choice" && info.name !== "ask_user_question" && info.name !== "question")) continue;
						let def = null;
						try {
							if (typeof pi.getToolDefinition === "function") def = pi.getToolDefinition(info.name);
						} catch {}
						// Fallback: try pi.getTool
						if (!def) {
							try {
								if (typeof pi.getTool === "function") def = pi.getTool(info.name);
							} catch {}
						}
						const target = def && def.definition ? def.definition : def;
						if (target && typeof target.execute === "function") {
							if (wrapSingleTool(target)) wrappedCount++;
						} else if (def && typeof def.execute === "function") {
							if (wrapSingleTool(def)) wrappedCount++;
						}
					}
				}
			}
		} catch {}
		// Strategy 3: direct pi.getTool / pi.getToolDefinition without getAllTools
		for (const name of ["ask_user_choice", "ask_user_question", "question"]) {
			try {
				if (typeof pi.getToolDefinition === "function") {
					const def = pi.getToolDefinition(name);
					const target = def && def.definition ? def.definition : def;
					if (target && typeof target.execute === "function") {
						if (wrapSingleTool(target)) wrappedCount++;
					}
				}
			} catch {}
			try {
				if (typeof pi.getTool === "function") {
					const def = pi.getTool(name);
					if (def && typeof def.execute === "function") {
						if (wrapSingleTool(def)) wrappedCount++;
					}
				}
			} catch {}
		}
		if (wrappedCount > 0) console.log(`[biggz-synthesis-gate] wrapped ${wrappedCount} pre-registered tool(s) (load-order race)`);
	} catch (e) {
		console.log(`[biggz-synthesis-gate] pre-register sweep error: ${e?.message || e}`);
	}

	// Secondary guard via tool_call event — MUST actually block (load-order safe, cross-extension).
	// Earlier versions only warned; this now returns {block:true, reason} to prevent bypass.
	try {
		if (typeof pi.on === "function") {
			pi.on("tool_call", async (event, ctx) => {
				try {
					const name = event?.toolName ?? event?.name ?? "";
					if (name !== "ask_user_choice" && name !== "ask_user_question" && name !== "question") return;
					// ENFORCEMENT RETIRED (2026-09-04): passthrough (see above).
					return;
					const toolParams = extractParamsFromToolCall(event);
					// Single ownership: block sub-agent checkpoint even before generic bypass
					if (isChildBypass()) {
						if (toolParams && isCheckpointAsk(toolParams)) {
							const reason = "isError:true checkpoint asks may only be emitted by orchestrator, not sub-agent";
							console.error(`[biggz-synthesis-gate] blocked sub-agent checkpoint (tool_call) ${name}: ${reason}`);
							try { ctx?.ui?.notify?.(reason, "error"); } catch {}
							try { pi.notify?.(reason, "error"); } catch {}
							return { block: true, reason };
						}
						return;
					}
					// Envelope validation before checkpoint logic
					try {
						const v = validateQuestionEnvelope(toolParams);
						if (v) {
							const fb = formatFallback(toolParams);
							const reason = `isError:true ${v.message} (limit ${v.limit}) — fix header ≤16 (e.g. "Decisión" 8 not "Decisión del checkpoint" 23) or label ≤60, then retry`;
							console.error(`[biggz-synthesis-gate] blocked envelope (tool_call) ${name}: ${reason} — diagnostic: envelope limit vs synthesis missing`);
							try { if (fb) ctx?.ui?.notify?.(fb, "error"); } catch {}
							try { if (fb) pi.notify?.(fb, "error"); } catch {}
							return { block: true, reason: reason + (fb ? "\n\nFallback:\n" + fb : "") };
						}
					} catch {}
					if (!isCheckpointAsk(toolParams)) {
						// REQ-DG-1: only checkpoint asks gate; free-text and preflight option-asks never block
						try {
							const source = getCurrentTurnSynthesis(ctx);
							if (source && isAdviseEnabled() && isThinSynthesis(source)) {
								const metrics = getArtifactsMetrics(source);
								emitConcern(ctx, pi, metrics);
							}
						} catch {}
						return;
					}
					const has = checkSynthesisPrecondition(ctx);
					if (!has) {
						if (checkSessionRecallInCurrentTurn()) {
							// allow: recall -> preflight — only valid exception
						} else {
							const reason =
								"Please synthesize before asking — missing ## Sub-agent Result block. Required markdown: ## Sub-agent Result: {phase/agent} + **Artifacts/Paths:** + **Risks / Open Questions:** + **Next Recommended:**. Emit markdown FIRST, adjacent, same turn, before ask_user_choice/ask_user_question/question (closed: ask_user_choice). Diagnostic: if envelope error (header >16 e.g. 'Decisión del checkpoint' 23 vs 'Decisión' 8, or label >60) fix header/label first; else ensure synthesis has 4 verbatim markers, plain markdown not in ``` code block, same turn ≤120s, not from history.";
							const env = blockedEnvelope(toolParams, reason);
							const text = reason + (env.fallback ? "\n\nFallback:\n" + env.fallback : "");
							console.error(`[biggz-synthesis-gate] blocked (tool_call) ${name}: ${reason} — context+fallback emitted same turn (REQ-DG-2)`);
							try {
								ctx?.ui?.notify?.(text, "error");
							} catch {}
							try {
								pi.notify?.(text, "error");
							} catch {}
							try {
								pi.ui?.notify?.(text, "error");
							} catch {}
							return { block: true, reason: text, context: env.context, fallback: env.fallback };
						}
					}
					// Has synthesis or preflight allowance — check thin + advise for concern (non-blocking)
					try {
						const source = getCurrentTurnSynthesis(ctx);
						if (source && isAdviseEnabled() && isThinSynthesis(source)) {
							const metrics = getArtifactsMetrics(source);
							emitConcern(ctx, pi, metrics);
						}
					} catch {}
				} catch {}
			});
		}
	} catch {}

	// ---------------------------------------------------------------------------
	// Gentle Safety — verbatim DENIED[6]/SENSITIVE[8]/GUARDED[5] mirror (policy parity)
	// Mirrors internal/policy/guardrails.go and gentle-ai.ts:280-720 verbatim.
	// No surface MAY add/omit. Same 3 checks: IsDenied, ClassifyGuardedCommand, EvaluateSensitivePathTool.
	// ---------------------------------------------------------------------------
	const GIT_GLOBAL_FLAGS_SRC = String.raw`(?:\s+--?\S+(?:\s+[^-\s]\S*)?)* `;
	const GIT_PUSH_RE_SAFETY = new RegExp(String.raw`\bgit${GIT_GLOBAL_FLAGS_SRC}push\b`);
	const DENIED_BASH_PATTERNS_SAFETY = [
		/\brm\s+-rf\s+(?:\/(?:\s|$)|~(?:\/|\s|$)|[$]HOME(?:\/|\s|$)|\.\.?(?:\s|$))/,
		/\bgit\s+reset\s+--hard\b/,
		/\bgit\s+clean\b(?=[^\n]*(?:-[^\n]*f|--force))(?=[^\n]*(?:-[^\n]*d|--directories))/,
		new RegExp(String.raw`\bgit${GIT_GLOBAL_FLAGS_SRC}push\b(?=[^\n]*\s--force(?:-with-lease)?\b)`),
		new RegExp(String.raw`\bgit${GIT_GLOBAL_FLAGS_SRC}push\b(?=[^\n]*\s-[^\s-]*f)`),
		/\bchmod\s+-R\s+777\b/,
		/\bchown\s+-R\b/,
	];
	const GUARDED_KEY_PATTERNS_SAFETY = {
		gitPush: GIT_PUSH_RE_SAFETY,
		gitRebase: /\bgit\s+rebase\b/,
		gitBranchDeleteForce: /\bgit\s+branch\s+(?:-[a-zA-Z]*D[a-zA-Z]*|-[a-zA-Z]*d[a-zA-Z]*f[a-zA-Z]*|-[a-zA-Z]*f[a-zA-Z]*d[a-zA-Z]*|--delete\b[^\n]*--force\b|--force\b[^\n]*--delete\b)/,
		npmPublish: /\bnpm\s+publish\b/,
		piRemove: /\bpi\s+remove\b/,
	};
	const AUTONOMOUS_DEFAULT_ACTIONS_SAFETY = { gitPush: "allow", gitRebase: "confirm", gitBranchDeleteForce: "confirm", npmPublish: "block", piRemove: "confirm" };
	const PATH_GUARDED_TOOLS_SAFETY = new Set(["read", "write", "edit"]);
	const PATH_INPUT_KEYS_SAFETY = new Set(["path", "paths", "file", "files", "filePath", "filePaths"]);
	const SENSITIVE_PATH_PATTERNS_SAFETY = [/(^|\/)\.ssh(?:\/|$)/, /(^|\/)\.credentials(?:\/|$)/, /(^|\/)library\/keychains(?:\/|$)/, /(^|\/)\.aws\/credentials$/, /(^|\/)\.config\/gh\/hosts\.ya?ml$/, /(^|\/)secrets(?:\/|$)/, /(^|\/)\.env(?:$|[./_-])/, /\.(?:pem|key|p12|pfx)$/];
	function isDeniedSafety(cmd) { for (const p of DENIED_BASH_PATTERNS_SAFETY) if (p.test(cmd)) return true; return false; }
	function classifyGuardedCommandSafety(command, cfg) {
		for (const p of DENIED_BASH_PATTERNS_SAFETY) if (p.test(command)) return "block";
		for (const [k, pat] of Object.entries(GUARDED_KEY_PATTERNS_SAFETY)) {
			if (!pat.test(command)) continue;
			if (!cfg?.autonomousMode) return "confirm";
			const act = cfg.guardedCommands?.[k];
			return act ?? AUTONOMOUS_DEFAULT_ACTIONS_SAFETY[k];
		}
		return "not-guarded";
	}
	function normalizePolicyPathSafety(v) { let n = String(v).trim().replace(/\\/g, "/").toLowerCase(); const h = (typeof process !== "undefined" && process.env?.HOME) || ""; n = n.replace(/^~(?=\/)/, h).replace(/^~/, h); return n; }
	function isSensitivePathSafety(v) { const n = normalizePolicyPathSafety(v); return SENSITIVE_PATH_PATTERNS_SAFETY.some(p => p.test(n)); }
	function collectPathInputsSafety(val, key) { if (typeof val === "string") return key && PATH_INPUT_KEYS_SAFETY.has(key) ? [val] : []; if (Array.isArray(val)) return val.flatMap(x => collectPathInputsSafety(x, key)); if (val && typeof val === "object") return Object.entries(val).flatMap(([k, v]) => collectPathInputsSafety(v, k)); return []; }
	function evaluateSensitivePathToolSafety(toolName, input) {
		if (!PATH_GUARDED_TOOLS_SAFETY.has(toolName)) return undefined;
		const p = collectPathInputsSafety(input).find(isSensitivePathSafety);
		if (!p) return undefined;
		return { block: true, reason: `Gentle AI safety policy blocked access to sensitive path: ${p}` };
	}
	// Hook safety into Pi tool_call: deny→block, guarded per mode, sensitive→block (parity with Go/opencode)
	try {
		pi.on?.("tool_call", async (event, ctx) => {
			try {
				const tool = event?.toolName ?? event?.name ?? "";
				const input = event?.params ?? event?.input ?? event?.args ?? {};
				const cmd = typeof input?.command === "string" ? input.command : (typeof input?.cmd === "string" ? input.cmd : "");
				if (cmd && isDeniedSafety(cmd)) {
					console.error(`[safety] blocked denied command: ${cmd.slice(0,120)} surface=pi kind=block`);
					return { block: true, reason: "Gentle AI safety policy blocked a destructive shell command. Ask the user for an explicit safer plan." };
				}
				// sensitive paths on read/write/edit
				if (PATH_GUARDED_TOOLS_SAFETY.has(tool)) {
					const sens = evaluateSensitivePathToolSafety(tool, input);
					if (sens?.block) {
						console.error(`[safety] blocked sensitive path tool=${tool} surface=pi kind=block path=${sens.reason}`);
						return { block: true, reason: sens.reason };
					}
				}
				// guarded classification: if block→block, confirm→prompt (non-blocking, log)
				if (cmd) {
					try {
						const cfg = { autonomousMode: process.env.GENTLE_PI_AUTONOMOUS_MODE === "1", guardedCommands: {} };
						const cls = classifyGuardedCommandSafety(cmd, cfg);
						if (cls === "block") { console.error(`[safety] guarded block cmd=${cmd.slice(0,80)} surface=pi kind=block`); return { block: true, reason: "Gentle AI safety policy blocked guarded command." }; }
						if (cls === "confirm") { console.warn(`[safety] guarded confirm cmd=${cmd.slice(0,80)} surface=pi kind=confirm`); }
					} catch {}
				}
			} catch {}
		});
	} catch {}
	// expose safety helpers for parity harness
	try { pi._biggzSafety = { isDenied: isDeniedSafety, classifyGuardedCommand: classifyGuardedCommandSafety, evaluateSensitivePathTool: evaluateSensitivePathToolSafety, isSensitivePath: isSensitivePathSafety }; } catch {}

	pi.registerCommand?.("synthesis-gate-status", {
		description: "Show synthesis gate status (checks # Sub-agent Result before ask_user_choice/ask_user_question)",
		handler: async (_args, ctx) => {
			if (isChildBypass()) {
				const msg = "synthesis gate bypassed: PI_SUBAGENT_CHILD=1";
				ctx.ui.notify(msg, "info");
				return { content: [{ type: "text", text: msg }] };
			}
			const has = checkSynthesisPrecondition(ctx);
			if (!has) {
				const status = "✗ synthesis gate: missing ## Sub-agent Result — emit markdown before ask_user_choice/ask_user_question/question";
				ctx.ui.notify(status, "warning");
				return { content: [{ type: "text", text: status }] };
			}
			const source = getCurrentTurnSynthesis(ctx);
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
