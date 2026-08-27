/**
 * biggz-extension-api — pi ExtensionAPI wiring for Runner
 * Mirrors biggz-tool-interception but delegates to Runner reusing PolicyInterceptor.
 * Handles PI_SUBAGENT_CHILD=1 bypass, block/revise short-circuit, tool_result no-mutate.
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */
export default function biggzExtensionAPI(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (typeof pi.on !== "function") return;
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
