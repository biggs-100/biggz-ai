/**
 * biggz-pi-pretty — FleetView (pi-subagents) + pretty rendering.
 *
 * Port of gentle-pi's extensions/pi-pretty.ts: resolves pnpm symlinks via
 * realpathSync and delegates to @heyhuynhgiabuu/pi-pretty so pi's
 * subagent_run tool renders as FleetView instead of single-column native task.
 *
 * pi's loader only scans ~/.pi/agent/node_modules (pnpm symlinks), not the
 * global npm prefix. realpathSync ensures the require base is the real package
 * path so the pnpm symlink for pi-pretty is correctly resolved, matching
 * gentle-pi's createRequire(realpathSync(packageJsonPath)) pattern.
 *
 * Keep alongside biggz-thinking-wrap.js (Ctrl+T wrap); do not replace it.
 * Both extensions coexist: thinking-wrap hints wrap, pretty renders FleetView.
 */

import { realpathSync } from "node:fs";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

export default async function biggzPiPrettyExtension(pi, deps) {
	try {
		// Resolve the real file path to handle pnpm symlink installs
		// (like gentle-pi's realpathSync(packageJsonPath) -> createRequire).
		const extensionRealPath = realpathSync(fileURLToPath(import.meta.url));
		const require = createRequire(extensionRealPath);
		const mod = require("@heyhuynhgiabuu/pi-pretty");
		const piPretty = typeof mod === "function" ? mod : mod?.default;
		if (typeof piPretty === "function") {
			return piPretty(pi, deps);
		}
		if (pi?.logger?.warn) {
			pi.logger.warn("[biggz-pi-pretty] pi-pretty module has no callable export");
		}
	} catch (err) {
		// pi-pretty not installed yet — `pi install` pending. Don't crash pi;
		// pi will fall back to native task (single column) until installed.
		const msg = err?.message || String(err);
		if (pi?.logger?.warn) {
			pi.logger.warn(`[biggz-pi-pretty] pi-pretty not ready (run pi install): ${msg}`);
		} else {
			console.error(`[biggz-pi-pretty] pi-pretty not ready: ${msg}`);
		}
	}
}
