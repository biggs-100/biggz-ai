import { execFileSync } from "node:child_process";

// ── Guard note: BIGGZ_PRETTY=0 disables pretty enhancements (checked inside exported fn) ──

// ── session-close summary guard (Cut 2: enforce-session-close-summary) ──
// APPLY-DECIDE Q2: timeout 1000ms. Measured 2026-09-07: `biggz session-close
// --check-only` cold-start 274-289ms on win32, so the 250ms draft would flap
// into degrade on the happy path and silently lose enforcement. 1000ms keeps
// close bounded (~3x headroom) while never trapping (timeout → allow + warn).
export const SESSION_STOP_TIMEOUT_MS = 1000;

// Test seam: module state, so extension-api's delegated call sees the same
// mock — that shared visibility is what makes the both-file parity test
// meaningful. Production always passes no opts (real execFileSync).
let _sessionStopExecForTest = null;
export function _setSessionStopExecForTest(fn) { _sessionStopExecForTest = fn || null; }

// checkSessionStop: single session-close guard shared by both pi extensions
// (extension-api delegates here; never reimplement pending/CLI logic there).
// Pending findings/lenses block first without invoking the CLI. Otherwise it
// invokes `biggz session-close --check-only` via execFileSync argv array (no
// shell, no injection): exit 0 → allow (undefined); exit 1 → { block: true, reason };
// timeout/crash → allow + degraded warning. Never throws.
export async function checkSessionStop(opts = {}) {
	try {
		const env = opts.env ?? process.env;
		const pending = parseInt(env?.BIGGZ_PENDING_FINDINGS || "0", 10);
		const lenses = parseInt(env?.BIGGZ_PENDING_LENSES || "0", 10);
		if (pending > 0 || lenses > 0) return { block: true, reason: "CanStopSession blocked: pending work" };
		const exec = opts.execFile ?? _sessionStopExecForTest ?? execFileSync;
		const cwd = opts.cwd ?? process.cwd();
		const bin = process.platform === "win32" ? "biggz.exe" : "biggz";
		exec(bin, ["session-close", "--check-only", "--cwd", cwd], {
			encoding: "utf8",
			timeout: SESSION_STOP_TIMEOUT_MS,
			windowsHide: true,
		});
		return undefined;
	} catch (err) {
		if (err && err.status === 1) {
			const toText = (v) => (Buffer.isBuffer(v) ? v.toString("utf8") : String(v ?? ""));
			const out = `${toText(err.stderr)}\n${toText(err.stdout)}`;
			// Only the gate token blocks. A stale binary without the
			// session-close verb also exits 1 (help text) — blocking on that
			// would trap every close with a nonsense reason. Degrade instead.
			if (/blocked\(session_summary_missing\)/.test(out)) {
				const first = out.split("\n").map((l) => l.trim()).find((l) => l.length > 0);
				return { block: true, reason: first || "blocked(session_summary_missing)" };
			}
		}
		try {
			console.warn(`session-close guard degraded (allowing close): ${err?.code || err?.message || String(err)}`);
		} catch {}
		return undefined;
	}
}
