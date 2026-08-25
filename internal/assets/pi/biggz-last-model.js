/**
 * biggz-last-model — remember last pi model across sessions.
 *
 * Pi always starts new sessions with `defaultModel`/`defaultProvider` from
 * `~/.pi/agent/settings.json`, ignoring the last `Ctrl+P` choice. This
 * extension and the Go installer sync the default to the most recent session's
 * model so `pi` reopens with the last used model.
 *
 * Strategy:
 * - Startup sync: read `last-model.json` cache if present, otherwise scan
 *   `~/.pi/agent/sessions` for the most recent `*.jsonl` (by mtime, recursive)
 *   and parse its last `model_change` or assistant `message` for
 *   `model`/`provider`. If found and different from `settings.json`, update
 *   `settings.json` atomically and also try `pi.setModel()` for the current
 *   session (best-effort, never crashes pi).
 * - Runtime persist: on `model_select` (Ctrl+P) write `last-model.json` and
 *   update `settings.json` atomically so the next `pi` launch inherits it.
 * - Fallback: also tries `session_shutdown` / `session_end` to flush.
 *
 * No external deps, uses `node:fs`, `node:os`, `node:path`.
 * Handles Windows paths via `path.join` and `os.homedir()`.
 * Writes atomically via temp+rename. All errors are swallowed.
 */

import fs from "node:fs";
import os from "node:os";
import path from "node:path";

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzLastModel(pi) {
	// Never run inside fresh/isolated subagent children — they have empty sessions and would mis-detect model + race settings.json writes.
	// Mirrors pi-subagents guard: if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	try {
		const agentDir = process.env.PI_CODING_AGENT_DIR
			? String(process.env.PI_CODING_AGENT_DIR).trim()
			: path.join(os.homedir(), ".pi", "agent");
		const sessionsDir = path.join(agentDir, "sessions");
		const settingsPath = path.join(agentDir, "settings.json");
		const cachePath = path.join(agentDir, "last-model.json");

		function readJsonSafe(p) {
			try {
				const raw = fs.readFileSync(p, "utf8");
				if (!raw || !raw.trim()) return null;
				return JSON.parse(raw);
			} catch {
				return null;
			}
		}

		function writeAtomic(target, data) {
			try {
				const dir = path.dirname(target);
				fs.mkdirSync(dir, { recursive: true });
				const tmp =
					target +
					".tmp-" +
					Date.now() +
					"-" +
					Math.random().toString(36).slice(2, 8);
				fs.writeFileSync(tmp, data, "utf8");
				fs.renameSync(tmp, target);
				return true;
			} catch {
				return false;
			}
		}

		function persist(model, provider) {
			if (!model || typeof model !== "string") return;
			model = String(model).trim();
			if (!model) return;
			if (provider) provider = String(provider).trim();
			try {
				const cache = {
					model,
					provider: provider || "",
					updatedAt: new Date().toISOString(),
				};
				writeAtomic(cachePath, JSON.stringify(cache, null, 2) + "\n");
			} catch {}
			try {
				const settings = readJsonSafe(settingsPath);
				if (!settings || typeof settings !== "object") return;
				if (
					settings.defaultModel === model &&
					(!provider || settings.defaultProvider === provider)
				)
					return;
				settings.defaultModel = model;
				if (provider) settings.defaultProvider = provider;
				writeAtomic(
					settingsPath,
					JSON.stringify(settings, null, 2) + "\n",
				);
			} catch {}
		}

		function findLastModelFromCache() {
			try {
				const c = readJsonSafe(cachePath);
				if (c && typeof c.model === "string" && c.model.trim()) {
					return {
						model: String(c.model).trim(),
						provider: c.provider ? String(c.provider).trim() : "",
					};
				}
			} catch {}
			return null;
		}

		function findLastModelFromSessions() {
			try {
				if (!fs.existsSync(sessionsDir)) return null;
				let latest = null; // { path, mtime }
				function walk(dir) {
					let entries;
					try {
						entries = fs.readdirSync(dir, { withFileTypes: true });
					} catch {
						return;
					}
					for (const e of entries) {
						const full = path.join(dir, e.name);
						if (e.isDirectory()) {
							walk(full);
						} else if (e.isFile() && e.name.endsWith(".jsonl")) {
							try {
								const stat = fs.statSync(full);
								const mtime = stat.mtimeMs || stat.mtime.getTime();
								if (!latest || mtime > latest.mtime) {
									latest = { path: full, mtime };
								}
							} catch {}
						}
					}
				}
				walk(sessionsDir);
				if (!latest) return null;
				let content;
				try {
					content = fs.readFileSync(latest.path, "utf8");
				} catch {
					return null;
				}
				const lines = content.split("\n");
				for (let i = lines.length - 1; i >= 0; i--) {
					const line = lines[i].trim();
					if (!line) continue;
					try {
						const obj = JSON.parse(line);
						if (
							obj.type === "model_change" &&
							typeof obj.modelId === "string" &&
							obj.modelId.trim()
						) {
							return {
								model: String(obj.modelId).trim(),
								provider: obj.provider ? String(obj.provider).trim() : "",
							};
						}
						if (obj.type === "message" && obj.message && typeof obj.message === "object") {
							const m = obj.message;
							// assistant message with provider/model (pi's session format)
							if (
								m.role === "assistant" &&
								typeof m.model === "string" &&
								m.model.trim()
							) {
								return {
									model: String(m.model).trim(),
									provider: m.provider
										? String(m.provider).trim()
										: m.modelProvider
											? String(m.modelProvider).trim()
											: "",
								};
							}
							// fallback: message carries provider + modelId
							if (
								typeof m.modelId === "string" &&
								m.modelId.trim() &&
								m.provider
							) {
								return {
									model: String(m.modelId).trim(),
									provider: String(m.provider).trim(),
								};
							}
							// some pi versions store model at top-level message fields
							if (
								m.api &&
								m.provider &&
								typeof m.model === "string" &&
								m.model.trim()
							) {
								return {
									model: String(m.model).trim(),
									provider: String(m.provider).trim(),
								};
							}
						}
					} catch {}
				}
				return null;
			} catch {
				return null;
			}
		}

		function findLastModel() {
			const fromCache = findLastModelFromCache();
			if (fromCache) return fromCache;
			return findLastModelFromSessions();
		}

		function syncStartup() {
			try {
				const found = findLastModel();
				if (!found || !found.model) return;
				const settings = readJsonSafe(settingsPath);
				if (!settings) return;
				const sameModel = settings.defaultModel === found.model;
				const sameProvider =
					!found.provider || settings.defaultProvider === found.provider;
				if (sameModel && sameProvider) return;
				// Best-effort: try to set current session's model via pi API.
				try {
					if (typeof pi.setModel === "function") {
						const candidate = {
							id: found.model,
							provider: found.provider || settings.defaultProvider || "opencode-go",
						};
						// Don't block startup; swallow rejection.
						Promise.resolve(pi.setModel(candidate)).catch(() => {});
					} else if (typeof pi.updateConfig === "function") {
						try {
							pi.updateConfig({
								defaultModel: found.model,
								defaultProvider: found.provider,
							});
						} catch {}
					}
				} catch {}
				persist(found.model, found.provider);
			} catch {}
		}

		// Immediate sync on extension load (covers non-TUI modes without session_start).
		try {
			syncStartup();
		} catch {}

		// Session start: primary hook for new TUI sessions.
		try {
			pi.on("session_start", async (_event, ctx) => {
				try {
					syncStartup();
				} catch {}
				// Try to align current session's model if it drifted from last.
				try {
					const found = findLastModel();
					if (found && found.model && ctx && ctx.model && ctx.model.id !== found.model) {
						if (typeof pi.setModel === "function") {
							const candidate = {
								id: found.model,
								provider:
									found.provider ||
									(ctx.model.provider ?? "opencode-go"),
							};
							await pi.setModel(candidate).catch(() => {});
						}
					}
				} catch {}
			});
		} catch {}

		// Persist whenever user picks a new model via Ctrl+P / /model.
		try {
			pi.on("model_select", async (event, _ctx) => {
				try {
					const m = event && event.model;
					if (m && typeof m.id === "string" && m.id.trim()) {
						persist(String(m.id).trim(), m.provider ? String(m.provider).trim() : "");
					}
				} catch {}
			});
		} catch {}

		// Flush on shutdown variants — best-effort. Names vary across pi versions.
		const flushFromCache = async () => {
			try {
				const c = findLastModelFromCache();
				if (c && c.model) persist(c.model, c.provider);
			} catch {}
		};
		try {
			pi.on("session_shutdown", flushFromCache);
		} catch {}
		try {
			// fallback name used in some docs; harmless if unknown
			pi.on("session_end", flushFromCache);
		} catch {}
	} catch {}
}
