/**
 * pi-tui-resolver — `--import` specifier hook for `@earendil-works/pi-tui` (S1b, task 3.7).
 *
 *   node --import ./internal/assets/pi/test/pi-tui-resolver.mjs --test internal/assets/pi/biggz-subagent-runtime.test.mjs
 *
 * Importing this module installs synchronous module hooks (idempotent) that resolve
 * `@earendil-works/pi-tui` to, in priority order:
 *   1. `PI_TUI_DIR` — explicit package-root override (always trusted).
 *   2. Node's own resolution — the repo, or pi's jiti alias at runtime.
 *   3. The pi install(s) discovered below (`@earendil-works/pi-coding-agent/node_modules`, `~/.pi/agent/npm`),
 *      but only when their source mentions every symbol in `PI_TUI_SHELL_SYMBOLS`.
 *   4. `pi-tui-oracle.mjs` — the test-only fallback when no real pi-tui is installed
 *      or the installed one cannot satisfy the ported shell modules' imports.
 * A test file can also `import "./test/pi-tui-resolver.mjs"` for the side effect before
 * dynamically importing the runtime (static imports of the bare specifier would link first).
 */
import { registerHooks } from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

export const PI_TUI_SPECIFIER = "@earendil-works/pi-tui";
export const PI_TUI_ORACLE_URL = new URL("./pi-tui-oracle.mjs", import.meta.url);

// Symbols the ported shell modules import from `@earendil-works/pi-tui`. The
// oracle implements them all, so a discovered install must mention them in its
// source for the shell suites to link against it instead of the fallback.
export const PI_TUI_SHELL_SYMBOLS = Object.freeze(["truncateToWidth", "visibleWidth", "wrapTextWithAnsi"]);

const join = (...parts) => path.join(...parts.filter(Boolean));

// Package roots to probe, in order: Windows npm global, then ~/.pi/agent/npm trees.
export function piTuiCandidateDirs(env = process.env) {
	const home = env.HOME || env.USERPROFILE || os.homedir();
	const appData = env.APPDATA || (env.USERPROFILE ? join(env.USERPROFILE, "AppData", "Roaming") : "");
	return [
		join(appData, "npm", "node_modules", "@earendil-works", "pi-coding-agent", "node_modules", "@earendil-works", "pi-tui"),
		join(home, ".pi", "agent", "npm", "node_modules", "@earendil-works", "pi-tui"),
		join(home, ".pi", "agent", "npm", "node_modules", "@earendil-works", "pi-coding-agent", "node_modules", "@earendil-works", "pi-tui"),
	];
}

function entryIn(dir, requireSymbols = false) {
	if (!dir) return null;
	try {
		const pkg = JSON.parse(fs.readFileSync(join(dir, "package.json"), "utf8"));
		if (pkg?.name !== PI_TUI_SPECIFIER) return null;
		const main = typeof pkg.main === "string" && pkg.main ? pkg.main : "dist/index.js";
		const file = join(dir, main);
		if (!fs.existsSync(file)) return null;
		if (requireSymbols && !packageMentionsSymbols(dir, PI_TUI_SHELL_SYMBOLS)) return null;
		return { dir, file };
	} catch {
		return null;
	}
}

// True when some `.js` file under the package root mentions every symbol. The
// probe reads source text instead of importing, so it stays synchronous and
// side-effect free; a union across files survives `export * from "./utils.js"`.
function packageMentionsSymbols(dir, symbols) {
	const found = new Set();
	const walk = (current, depth) => {
		if (depth > 4) return;
		let entries;
		try {
			entries = fs.readdirSync(current, { withFileTypes: true });
		} catch {
			return;
		}
		for (const entry of entries) {
			const full = join(current, entry.name);
			if (entry.isDirectory()) {
				walk(full, depth + 1);
				continue;
			}
			if (!entry.name.endsWith(".js")) continue;
			let source;
			try {
				source = fs.readFileSync(full, "utf8");
			} catch {
				continue;
			}
			for (const symbol of symbols) if (!found.has(symbol) && source.includes(symbol)) found.add(symbol);
		}
	};
	walk(dir, 0);
	return found.size === symbols.length;
}

// PI_TUI_DIR override first, then the installed-pi candidates — null when absent.
export function resolvePiTuiEntry(env = process.env) {
	return entryIn(String(env.PI_TUI_DIR ?? "").trim()) ?? piTuiCandidateDirs(env).map((dir) => entryIn(dir, true)).find(Boolean) ?? null;
}

let installed = false;
let mode = "pending";

export function installPiTuiResolver(env = process.env) {
	if (installed) return { installed: true, mode, reason: "already-installed" };
	installed = true;
	const explicit = entryIn(String(env.PI_TUI_DIR ?? "").trim());
	const discovered = explicit ?? piTuiCandidateDirs(env).map((dir) => entryIn(dir, true)).find(Boolean) ?? null;
	registerHooks({
		resolve(specifier, context, nextResolve) {
			if (specifier !== PI_TUI_SPECIFIER) return nextResolve(specifier, context);
			if (explicit) return { url: pathToFileURL(explicit.file).href, shortCircuit: true };
			try {
				return nextResolve(specifier, context);
			} catch {}
			if (discovered) return { url: pathToFileURL(discovered.file).href, shortCircuit: true };
			return { url: PI_TUI_ORACLE_URL.href, shortCircuit: true };
		},
	});
	mode = explicit ? `install:${explicit.dir}` : discovered ? `install:${discovered.dir}` : "oracle";
	return { installed: true, mode };
}

installPiTuiResolver();
