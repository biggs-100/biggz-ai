import { execFileSync } from "node:child_process";

// ── checkpoint-ask guard (fix-checkpoint-ask-context, slice 2) ──
// The ONLY live invocation path of the checkpoint-ask rule: the deployed
// ask-user-choice.ts asks this module, which invokes `biggz sdd-ask-check`
// (Go-canonical: synthesis precondition + envelope + substance). Only explicit
// `blocked(...)` tokens block; indeterminate failures degrade open visibly (D4).

// Bounded so a hung or slow binary can never freeze the ask UI. The Go verb is
// a one-shot stdin→exit process, well under this on any measured path.
export const ASK_CHECK_TIMEOUT_MS = 1000;

// Test seam: module state (session-guard parity) so tests observe the exact
// argv/opts the production path uses. Production passes no opts → real execFileSync.
let _askCheckExecForTest = null;
export function _setAskCheckExecForTest(fn) { _askCheckExecForTest = fn || null; }

// Current-turn buffer: the CLI seeds SetCurrentTurnMarkdown once from whatever
// the asset sends (D3); reset on turn_start/agent_start so a previous turn's
// synthesis never satisfies the current ask.
let askTurnMarkdown = "";
export function recordAskTurnText(text, opts = {}) {
	if (opts && opts.reset) { askTurnMarkdown = ""; return askTurnMarkdown; }
	if (typeof text === "string" && text.trim()) {
		if (!askTurnMarkdown) askTurnMarkdown = text;
		else if (text.includes(askTurnMarkdown)) askTurnMarkdown = text;
		else if (!askTurnMarkdown.includes(text)) askTurnMarkdown += "\n" + text;
	}
	return askTurnMarkdown;
}

export function getAskTurnMarkdown() { return askTurnMarkdown; }

function extractAskText(event) {
	if (!event) return "";
	if (typeof event === "string") return event;
	try {
		const msg = event.message;
		if (msg) {
			const role = msg.role;
			if (role && role !== "assistant" && role !== "custom") return "";
			if (typeof msg.text === "string" && msg.text.trim()) return msg.text;
			const content = msg.content;
			if (typeof content === "string") return content;
			if (Array.isArray(content)) {
				return content.map((b) => (typeof b === "string" ? b : b?.text ?? "")).filter(Boolean).join("\n");
			}
		}
	} catch {}
	if (typeof event.text === "string") return event.text;
	if (typeof event.content === "string") return event.content;
	try { if (typeof event.delta?.text === "string") return event.delta.text; } catch {}
	return "";
}

// checkCheckpointAsk(params): single check shared by the deployed ask tool.
// Returns undefined (allow) | { block: true, reason } | { degraded: true, notice }.
// Never throws — the caller must be able to present even when the check is broken.
export async function checkCheckpointAsk(params, opts = {}) {
	try {
		const env = opts.env ?? process.env;
		if (env?.BIGGZ_ASK_CHECK === "0") {
			return {
				degraded: true,
				notice: "biggz ask check skipped (BIGGZ_ASK_CHECK=0): the checkpoint-ask rule is NOT enforced for this ask",
			};
		}
		const exec = opts.execFile ?? _askCheckExecForTest ?? execFileSync;
		const bin = process.platform === "win32" ? "biggz.exe" : "biggz";
		exec(bin, ["sdd-ask-check"], {
			encoding: "utf8",
			timeout: ASK_CHECK_TIMEOUT_MS,
			windowsHide: true,
			input: JSON.stringify({
				question: JSON.stringify(params ?? {}),
				markdown: askTurnMarkdown,
			}),
		});
		return undefined;
	} catch (err) {
		try {
			if (err && typeof err.status === "number" && err.status !== 0) {
				const toText = (v) => (Buffer.isBuffer(v) ? v.toString("utf8") : String(v ?? ""));
				const out = `${toText(err.stdout)}\n${toText(err.stderr)}`;
				// Only the gate token blocks. A stale binary without the verb
				// also exits non-zero (help text) — blocking on that would trap
				// every ask with a nonsense reason. Degrade instead.
				const match = out.match(/blocked\((synthesis_required|envelope_invalid|checkpoint_option_thin)\)[^\n]*/);
				if (match) return { block: true, reason: match[0].trim() };
			}
			const why = err?.code ? String(err.code) : err?.status != null ? `exit ${err.status}` : (err?.message || String(err));
			const notice = `biggz ask check degraded (${why}): proceeding WITHOUT the checkpoint-ask rule — fix 'biggz sdd-ask-check' to restore enforcement`;
			try { console.warn(`biggz ask check degraded (proceeding): ${why}`); } catch {}
			return { degraded: true, notice };
		} catch {
			return { degraded: true, notice: "biggz ask check degraded: proceeding WITHOUT the checkpoint-ask rule" };
		}
	}
}

/**
 * pi ExtensionAPI factory — required so pi treats this file as a valid extension
 * (validatePiExtensionsFactory: `export default function`). Named guard API is
 * imported as a library by ask-user-choice.ts; the factory feeds the turn buffer.
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */
export default function biggzAskGuard(pi) {
	try {
		if (typeof pi?.on !== "function") return;
		for (const ev of ["message_end", "message_update", "message_start"]) {
			try {
				pi.on(ev, (event) => {
					try {
						const text = extractAskText(event);
						if (text && text.trim()) recordAskTurnText(text);
					} catch {}
				});
			} catch {}
		}
		for (const ev of ["turn_start", "agent_start"]) {
			try { pi.on(ev, () => recordAskTurnText("", { reset: true })); } catch {}
		}
	} catch {}
}
