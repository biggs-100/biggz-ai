/**
 * biggz-subagent-runtime — pi subagent delegation runtime.
 *
 * Ships inert until the S3 deploy cutover. S1a: agent discovery
 * (`~/.pi/agent/agents/*.md`), one `pi --mode rpc --no-session` child per task,
 * strict JSONL (LF-only split, CR stripped, 1 MiB line cap — `readline` also
 * splits U+2028/U+2029, legal inside JSON strings, so it is NOT RPC-protocol
 * compliant), stall watchdog (idle 4 min / total 30 min) and tree kill
 * (abort RPC → SIGTERM group → SIGKILL; Windows `taskkill /PID /T [/F]`).
 * S1b: registration gate (dual-registration vs j0k3r), `subagent*` tool surface
 * over a bounded in-memory ring (no disk, no BigMem), a one-line completion card
 * rendered only through pi-tui width primitives, and the
 * `biggz-subagent-completion` message renderer.
 * S2: RPC subset wiring (`steer`, `get_last_assistant_text`,
 * `extension_ui_response`; dialog relay for child `extension_ui_request`),
 * `mode:"background"` with completion delivery, the `biggz-subagents` widget
 * (max 2 rows + `… +N`), the concurrency cap with a FIFO queue, and the
 * `subagent_wait` headline.
 *
 * `@earendil-works/pi-tui` is the documented pi extension import (pi's jiti
 * alias maps it at runtime). Under `node --test` the specifier is resolved by
 * `test/pi-tui-resolver.mjs` (real install, else test-only oracle).
 */

import { execFileSync, spawn as nodeSpawn } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

import { truncateToWidth } from "@earendil-works/pi-tui";

export const JSONL_MAX_LINE_BYTES = 1024 * 1024;
export const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 1000;
export const DEFAULT_TOTAL_TIMEOUT_MS = 30 * 60 * 1000;
export const DEFAULT_FINAL_TEXT_MS = 1500;
export const DEFAULT_BACKGROUND_CAP = 2;
export const DEFAULT_WAIT_TIMEOUT_MS = 120 * 1000;
export const WIDGET_KEY = "biggz-subagents";
export const WIDGET_PLACEMENT = "belowEditor";
export const WIDGET_MAX_ROWS = 2;
export const WIDGET_INTERVAL_MS = 1000;
export const KILL_GRACE_MS = 5000;
export const KILL_POLL_MS = 50;
export const READ_ONLY_TOOLS = Object.freeze(["read"]);
const DIALOG_METHODS = new Set(["select", "confirm", "input", "editor"]);
const SAFE_TOKEN_RE = /^[A-Za-z0-9_.*\-/:]+$/;
const bounded = (value, limit = 200) => {
	const text = String(value ?? "");
	return text.length > limit ? `${text.slice(0, limit)}…` : text;
};
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// ── strict JSONL reader: LF-only split, CR stripped, bounded 1 MiB line ──

export function createJsonlReader(options = {}) {
	const maxBytes = options.maxLineBytes ?? JSONL_MAX_LINE_BYTES;
	const logger = options.logger ?? console;
	const stats = { lines: 0, skipped: 0 };
	let buffer = "";
	let dropping = false;
	const drop = (reason, detail) => {
		stats.skipped += 1;
		try {
			logger?.warn?.(`biggz-subagent-runtime: ${reason}: ${bounded(detail, 120)}`);
		} catch {}
	};
	function push(chunk) {
		if (chunk === undefined || chunk === null) return;
		let text = Buffer.isBuffer(chunk) ? chunk.toString("utf8") : String(chunk);
		if (!text) return;
		if (dropping) {
			const nl = text.indexOf("\n");
			if (nl === -1) return; // still inside a dropped oversized line
			text = text.slice(nl + 1);
			dropping = false;
		}
		buffer += text;
		for (let nl = buffer.indexOf("\n"); nl !== -1; nl = buffer.indexOf("\n")) {
			let line = buffer.slice(0, nl);
			buffer = buffer.slice(nl + 1);
			if (line.endsWith("\r")) line = line.slice(0, -1);
			if (!line) continue;
			if (Buffer.byteLength(line, "utf8") > maxBytes) {
				drop("oversized JSONL line skipped", line);
				continue;
			}
			let data;
			try {
				data = JSON.parse(line);
			} catch {
				drop("malformed JSONL line skipped", line);
				continue;
			}
			stats.lines += 1;
			try {
				options.onLine?.(data);
			} catch (err) {
				drop("JSONL handler error", err?.message ?? err);
			}
		}
		if (buffer && Buffer.byteLength(buffer, "utf8") > maxBytes) {
			buffer = "";
			dropping = true;
			drop("oversized JSONL line skipped (no LF within cap)", "");
		}
	}
	return { push, stats, bufferedBytes: () => Buffer.byteLength(buffer, "utf8") };
}

// ── agent discovery: ~/.pi/agent/agents/*.md, PI_CODING_AGENT_DIR honored ──

const unquote = (value) => {
	const text = String(value ?? "").trim();
	return /^".*"$|^'.*'$/.test(text) ? text.slice(1, -1) : text;
};
const splitList = (value) => String(value ?? "").split(",").map(unquote).filter(Boolean);

export function parseFrontmatter(text) {
	const raw = String(text ?? "").replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
	const end = raw.startsWith("---\n") ? raw.indexOf("\n---", 4) : -1;
	if (end === -1) return { data: {}, body: raw.trim() };
	const data = {};
	let listKey = null;
	for (const line of raw.slice(4, end).split("\n")) {
		const item = line.match(/^\s*-\s+(.*)$/);
		if (item && listKey) {
			data[listKey].push(unquote(item[1]));
			continue;
		}
		const pair = line.match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
		if (!pair) {
			listKey = null;
			continue;
		}
		const value = pair[2].trim();
		if (!value) {
			data[pair[1]] = []; // multiline YAML list follows
			listKey = pair[1];
			continue;
		}
		listKey = null;
		data[pair[1]] = /^\[.*\]$/.test(value) || pair[1] === "tools" ? splitList(value.replace(/^\[|\]$/g, "")) : unquote(value);
	}
	return { data, body: raw.slice(end + 4).replace(/^\n+/, "").trim() };
}

export function resolveAgentsDir(env = process.env) {
	const override = typeof env?.PI_CODING_AGENT_DIR === "string" ? env.PI_CODING_AGENT_DIR.trim() : "";
	return override ? path.join(override, "agents") : path.join(os.homedir(), ".pi", "agent", "agents");
}

export function discoverAgents(options = {}) {
	const dir = options.dir ?? resolveAgentsDir(options.env ?? process.env);
	const fsImpl = options.fsImpl ?? fs;
	let files = [];
	try {
		files = fsImpl.readdirSync(dir).filter((name) => name.endsWith(".md")).sort();
	} catch {
		return [];
	}
	const agents = [];
	for (const name of files) {
		try {
			const filePath = path.join(dir, name);
			const { data, body } = parseFrontmatter(fsImpl.readFileSync(filePath, "utf8"));
			const tools = (Array.isArray(data.tools) ? data.tools : []).map(unquote).filter(Boolean);
			agents.push({
				id: String(data.name || name.slice(0, -3)).trim().toLowerCase(),
				description: String(data.description || "").trim(),
				tools: tools.length ? tools : [...READ_ONLY_TOOLS], // no frontmatter tools ⇒ read-only `read`
				model: typeof data.model === "string" && data.model.trim() ? data.model.trim() : null,
				body,
				filePath,
			});
		} catch {}
	}
	return agents;
}

// ── spawn authorization: nested refusal, bounded unknown agent, argv + env ──

export function nestedSpawnRefusal(env = process.env) {
	return env?.PI_SUBAGENT_CHILD === "1" ? { ok: false, error: "subagent refused: nested subagent context (PI_SUBAGENT_CHILD=1)" } : null;
}

export function unknownAgentError(agentId, agents, limit = 8) {
	const names = (agents ?? []).map((agent) => agent.id);
	const more = names.length > limit ? ` … +${names.length - limit} more` : "";
	return `unknown agent "${bounded(agentId, 60)}"; available: ${names.slice(0, limit).join(", ") || "none"}${more}`;
}

export function resolveAgent(agentId, agents, env = process.env) {
	const refused = nestedSpawnRefusal(env);
	if (refused) return refused;
	const found = (agents ?? []).find((agent) => agent.id === String(agentId ?? "").trim().toLowerCase());
	return found ? { ok: true, agent: found } : { ok: false, error: unknownAgentError(agentId, agents) };
}

export function buildDelegatedPrompt(agent, task) {
	const body = String(agent?.body ?? "").trim();
	const delegated = `## delegated task\n${String(task ?? "").trim()}`;
	return body ? `${body}\n\n${delegated}` : delegated;
}

export function buildChildArgs(agent) {
	const tools = (Array.isArray(agent?.tools) ? agent.tools : []).map((token) => String(token).trim()).filter((token) => token && SAFE_TOKEN_RE.test(token));
	const args = ["--mode", "rpc", "--no-session", "--tools", (tools.length ? tools : READ_ONLY_TOOLS).join(",")];
	const model = typeof agent?.model === "string" ? agent.model.trim() : "";
	if (model && SAFE_TOKEN_RE.test(model)) args.push("--model", model);
	return args;
}

export const buildChildEnv = (baseEnv = process.env) => ({ ...baseEnv, PI_SUBAGENT_CHILD: "1" });

export function resolvePiLaunch(options = {}) {
	const env = options.env ?? process.env;
	const override = typeof env?.BIGGZ_PI_BIN === "string" ? env.BIGGZ_PI_BIN.trim() : "";
	if (override) return { command: override, prefix: [], shell: false };
	// Windows npm shims are .cmd files; safe because argv tokens are sanitized above.
	return (options.platform ?? process.platform) === "win32" ? { command: "pi.cmd", prefix: [], shell: true } : { command: "pi", prefix: [], shell: false };
}

// ── tree kill: abort RPC → SIGTERM group → SIGKILL (Windows taskkill /T [/F]) ──

export function isPidAlive(pid, killImpl = process.kill) {
	if (!pid) return false;
	try {
		killImpl(pid, 0);
		return true;
	} catch (err) {
		return err?.code === "EPERM";
	}
}

export async function killTree(pid, options = {}) {
	const steps = [];
	if (!pid) return { killed: true, steps };
	const platform = options.platform ?? process.platform;
	const killImpl = options.killImpl ?? process.kill;
	const execImpl = options.execImpl ?? execFileSync;
	const logger = options.logger ?? console;
	const killed = () => !isPidAlive(pid, killImpl);
	const step = (label, fn) => {
		steps.push(label);
		try {
			fn();
			return true;
		} catch (err) {
			if (err?.code !== "ESRCH") {
				try {
					logger?.warn?.(`biggz-subagent-runtime: kill step ${label} failed: ${bounded(err?.message ?? err, 120)}`);
				} catch {}
			}
			return false;
		}
	};
	if (typeof options.abort === "function") step("abort", options.abort);
	const graceful = platform === "win32"
		? step("taskkill /T", () => execImpl("taskkill", ["/PID", String(pid), "/T"], { stdio: "ignore", windowsHide: true }))
		: step("SIGTERM", () => killImpl(-pid, "SIGTERM"));
	if (!killed() && graceful) {
		const deadline = Date.now() + (options.graceMs ?? KILL_GRACE_MS);
		const pollMs = options.pollMs ?? KILL_POLL_MS;
		while (!killed() && Date.now() < deadline) await (options.sleep ?? sleep)(pollMs);
	}
	if (!killed()) {
		if (platform === "win32") step("taskkill /T /F", () => execImpl("taskkill", ["/PID", String(pid), "/T", "/F"], { stdio: "ignore", windowsHide: true }));
		else step("SIGKILL", () => killImpl(-pid, "SIGKILL"));
	}
	return { killed: killed(), steps };
}

// ── one RPC child per task, stall watchdog, bounded event capture ──

export function createTask(options = {}) {
	const platform = options.platform ?? process.platform;
	const logger = options.logger ?? console;
	const now = options.now ?? Date.now;
	const agent = options.agent ?? null;
	const env = options.env ?? process.env;
	const launch = options.launch ?? resolvePiLaunch({ env });
	const spawnImpl = options.spawnImpl ?? nodeSpawn;
	const args = options.args ?? buildChildArgs(agent);
	const idleMs = options.idleMs ?? DEFAULT_IDLE_TIMEOUT_MS;
	const totalMs = options.totalMs ?? DEFAULT_TOTAL_TIMEOUT_MS;
	const startedAt = now();
	const id = options.id ?? `sub-${startedAt.toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
	const events = [];
	const state = { value: options.autoStart === false ? "queued" : "spawning", reason: null, exitCode: null, error: null };
	let child = null;
	let idleTimer = null;
	let totalTimer = null;
	let finalTextTimer = null;
	let finalText = null;
	let pendingUi = 0;
	let started = false;
	let settled = false;
	let stderrTail = "";
	let resolveTask;
	const promise = new Promise((resolve) => {
		resolveTask = resolve;
	});
	const snapshot = () => ({ id, state: state.value, reason: state.reason, exitCode: state.exitCode, error: state.error, events, stderrTail, finalText, pendingUi, elapsedMs: now() - startedAt, pid: child?.pid ?? null });

	function clearTimers() {
		if (idleTimer) clearTimeout(idleTimer);
		if (totalTimer) clearTimeout(totalTimer);
		if (finalTextTimer) clearTimeout(finalTextTimer);
		idleTimer = null;
		totalTimer = null;
		finalTextTimer = null;
	}
	function settle(kind, reason, error) {
		if (settled) return;
		settled = true;
		clearTimers();
		state.value = kind;
		state.reason = reason ?? null;
		if (error !== undefined) state.error = error ? bounded(error?.message ?? error, 300) : null;
		resolveTask(snapshot());
	}
	function writeCommand(command) {
		try {
			if (child?.stdin?.writable !== false && typeof child.stdin.write === "function") {
				child.stdin.write(`${JSON.stringify(command)}\n`);
				return true;
			}
		} catch {}
		return false;
	}
	const stopTree = (reason) =>
		killTree(child?.pid, { platform, graceMs: options.killGraceMs, abort: () => writeCommand({ type: "abort" }), killImpl: options.killImpl, execImpl: options.execImpl, logger });
	const stalled = (reason) => {
		if (settled || state.value === "stalled" || state.value === "cancelled") return;
		state.value = "stalled";
		state.reason = reason;
		void stopTree(`watchdog ${reason}`).then(() => {
			if (state.value === "stalled") settle("stalled", reason);
		});
	};
	const armIdle = () => {
		if (idleTimer) clearTimeout(idleTimer);
		idleTimer = setTimeout(() => stalled("idle"), idleMs);
	};
	// agent_settled → get_last_assistant_text → completed; bounded so a silent
	// child can never hold the run open (deadline 0 skips the round-trip).
	const finishSettled = () => {
		if (settled) return;
		if (finalTextTimer) clearTimeout(finalTextTimer);
		finalTextTimer = null;
		settle("completed", "settled");
		void stopTree("settled");
	};
	const requestFinalText = () => {
		if (settled) return;
		const deadline = Math.max(0, Math.floor(Number(options.finalTextMs ?? DEFAULT_FINAL_TEXT_MS) || 0));
		if (deadline === 0 || !writeCommand({ id: `${id}-final`, type: "get_last_assistant_text" })) {
			finishSettled();
			return;
		}
		finalTextTimer = setTimeout(finishSettled, deadline);
	};
	// Dialog relay: a child `extension_ui_request` (select/confirm/input/editor) is
	// presented by the parent and answered on the child's stdin; fire-and-forget
	// methods and every other event type are ignored.
	async function relayUiRequest(request) {
		if (settled || !DIALOG_METHODS.has(request?.method)) return;
		pendingUi += 1;
		if (idleTimer) clearTimeout(idleTimer); // a dialog awaiting the user is not idle
		idleTimer = null;
		let response = null;
		try {
			response = await options.onUiRequest?.(request);
		} catch (err) {
			try {
				logger?.warn?.(`biggz-subagent-runtime: UI relay failed: ${bounded(err?.message ?? err, 120)}`);
			} catch {}
		}
		const answer = response == null || response?.cancelled === true
			? { cancelled: true }
			: request.method === "confirm"
				? { confirmed: response.confirmed === true }
				: { value: response.value ?? null };
		writeCommand({ type: "extension_ui_response", id: request?.id, ...answer });
		pendingUi = Math.max(0, pendingUi - 1);
		if (!settled) armIdle();
	}
	const reader = createJsonlReader({
		logger,
		onLine(data) {
			if (settled) return;
			events.push(data);
			if (events.length > 500) events.shift();
			const type = data?.type;
			if (type === "response") {
				if (data?.command === "get_last_assistant_text") {
					finalText = typeof data?.data?.text === "string" ? data.data.text : null;
					finishSettled();
				}
				return; // responses to other commands are not part of the subset
			}
			if (type === "agent_settled") {
				requestFinalText();
				return;
			}
			if (type === "extension_ui_request") {
				void relayUiRequest(data);
				return;
			}
			// Progress-only events: agent_start, message_update, tool_execution_start/end,
			// auto_retry_end, extension_error. Every other event type is ignored.
		},
	});

	function start() {
		if (started || settled) return false;
		started = true;
		state.value = "spawning";
		totalTimer = setTimeout(() => stalled("total"), totalMs);
		armIdle();
		try {
			child = spawnImpl(launch.command, [...(launch.prefix ?? []), ...args], {
				cwd: options.cwd,
				env: buildChildEnv(env),
				stdio: ["pipe", "pipe", "pipe"],
				windowsHide: true,
				detached: platform !== "win32", // own process group so kill(-pid) works
				shell: launch.shell === true,
			});
		} catch (err) {
			settle("failed", "spawn", err);
			return false;
		}
		if (child) {
			state.value = "running";
			try {
				child.stdout?.on?.("data", (chunk) => {
					armIdle();
					reader.push(chunk);
				});
				child.stderr?.on?.("data", (chunk) => {
					stderrTail = `${stderrTail}${String(chunk)}`.slice(-4096);
				});
				child.on?.("error", (err) => {
					if (!settled && state.value !== "stalled" && state.value !== "cancelled") settle("failed", "spawn", err);
				});
				child.on?.("exit", (code) => {
					state.exitCode = code;
					if (!settled && state.value !== "stalled" && state.value !== "cancelled") settle("failed", `exit:${code ?? "signal"}`);
				});
				writeCommand({ id: `${id}-prompt-1`, type: "prompt", message: options.firstMessage ?? buildDelegatedPrompt(agent, options.task) });
			} catch (err) {
				settle("failed", "child_setup", err);
			}
		}
		return true;
	}
	if (options.autoStart !== false) start();

	return {
		id,
		agent,
		promise,
		snapshot,
		start,
		get pid() {
			return child?.pid ?? null;
		},
		get state() {
			return state.value;
		},
		steer(message) {
			return !settled && writeCommand({ type: "steer", message: String(message ?? "") });
		},
		cancel(reason = "cancelled") {
			if (!settled && state.value !== "cancelled") {
				state.value = "cancelled";
				state.reason = reason;
				void stopTree(reason).then(() => {
					if (state.value === "cancelled") settle("cancelled", reason);
				});
			}
			return promise;
		},
	};
}

// ── registration gate: dual-registration vs j0k3r (tool scan → settings packages) ──

export const LEGACY_TOOL_RE = /^subagent_run$|^subagent_list_/;
export const J0K3R_PACKAGE_MARKER = "pi-subagents-j0k3r";

export function resolveSettingsPath(env = process.env) {
	const override = typeof env?.PI_CODING_AGENT_DIR === "string" ? env.PI_CODING_AGENT_DIR.trim() : "";
	return path.join(override || path.join(os.homedir(), ".pi", "agent"), "settings.json");
}

// Missing file ⇒ nothing installed; unreadable/corrupt ⇒ unprovable (the caller fails closed).
export function readSettingsPackages(options = {}) {
	const file = options.file ?? resolveSettingsPath(options.env ?? process.env);
	const fsImpl = options.fsImpl ?? fs;
	let raw;
	try {
		raw = fsImpl.readFileSync(file, "utf8");
	} catch (err) {
		return err?.code === "ENOENT" ? { ok: true, packages: [] } : { ok: false, error: bounded(err?.message ?? err, 120) };
	}
	try {
		const obj = JSON.parse(raw);
		return { ok: true, packages: Array.isArray(obj?.packages) ? obj.packages.map((entry) => String(entry)) : [] };
	} catch (err) {
		return { ok: false, error: `settings.json unreadable: ${bounded(err?.message ?? err, 100)}` };
	}
}

export const hasJ0k3rPackage = (packages) => (packages ?? []).some((entry) => String(entry).includes(J0K3R_PACKAGE_MARKER));

/**
 * One `typeof`-guarded chain; any link may only PROVE j0k3r presence, never absence
 * (on 0.85.1 `pi.getAllTools()` throws while extensions load — "runtime not
 * initialized" — and `pi.getToolDefinition` is runner-only). The load-time proof is
 * settings `packages`; when nothing can prove j0k3r absent the gate fails closed.
 * Never calls `pi.getTool` (absent in 0.85.1).
 */
export function subagentRegistrationGate(pi, options = {}) {
	const logger = options.logger ?? console;
	const refuse = (reason) => {
		try {
			logger?.warn?.(`biggz-subagent-runtime: registration skipped: ${reason}`);
		} catch {}
		return { register: false, reason };
	};
	try {
		if (typeof pi?.getAllTools === "function") {
			const tools = pi.getAllTools();
			if (Array.isArray(tools)) {
				const clash = tools.map((info) => info?.name).filter((name) => typeof name === "string" && LEGACY_TOOL_RE.test(name));
				if (clash.length) return refuse(`legacy j0k3r tool(s) present: ${bounded(clash.slice(0, 4).join(", "), 120)}`);
			}
		}
	} catch (err) {
		try {
			logger?.debug?.(`biggz-subagent-runtime: getAllTools() unavailable at load: ${bounded(err?.message ?? err, 100)}`);
		} catch {}
	}
	try {
		// typeof-guarded: absent from ExtensionAPI in 0.85.1 (runner.d.ts:119 only).
		if (typeof pi?.getToolDefinition === "function" && pi.getToolDefinition("subagent_run")) {
			return refuse('legacy tool definition "subagent_run" present');
		}
	} catch (err) {
		try {
			logger?.debug?.(`biggz-subagent-runtime: getToolDefinition() unavailable: ${bounded(err?.message ?? err, 100)}`);
		} catch {}
	}
	const settings = readSettingsPackages({ env: options.env, fsImpl: options.fsImpl });
	if (!settings.ok) return refuse(`cannot prove ${J0K3R_PACKAGE_MARKER} absent: ${settings.error}`);
	if (hasJ0k3rPackage(settings.packages)) return refuse(`${J0K3R_PACKAGE_MARKER} present in settings packages`);
	return { register: true, reason: `${J0K3R_PACKAGE_MARKER} absent` };
}

// ── bounded in-memory task ring (no disk, no BigMem) ──

export const TASK_RING_LIMIT = 50;
export const isActiveTaskState = (state) => state === "queued" || state === "spawning" || state === "running";

export function createTaskRegistry(options = {}) {
	const limit = Math.max(1, Math.floor(Number(options.limit) || TASK_RING_LIMIT));
	const order = [];
	const byId = new Map();
	const evict = (keepId) => {
		while (byId.size > limit) {
			// Oldest settled record goes first; the just-added record and in-flight work stay.
			const idx = order.findIndex((id) => id !== keepId && byId.has(id) && !isActiveTaskState(byId.get(id)?.state));
			if (idx === -1) return; // nothing evictable — bounded overflow of live work only
			byId.delete(order[idx]);
			order.splice(idx, 1);
		}
	};
	const all = () => order.filter((id) => byId.has(id)).map((id) => byId.get(id));
	return {
		limit,
		get size() {
			return byId.size;
		},
		add(task) {
			byId.set(task.id, task);
			order.push(task.id);
			evict(task.id);
			return task;
		},
		get: (id) => byId.get(String(id ?? "")) ?? null,
		all,
		active: () => all().filter((task) => isActiveTaskState(task?.state)),
	};
}

// ── S2 UI: cap, widget rows, wait headline, dialog presentation ──

export function resolveBackgroundCap(env = process.env) {
	const raw = String(env?.BIGGZ_BACKGROUND_SUBAGENTS ?? "").trim().toLowerCase();
	const parsed = Number.parseInt(raw, 10);
	if (raw === "on" || raw === "off" || !Number.isFinite(parsed)) return DEFAULT_BACKGROUND_CAP; // four-source policy owns on/off
	return Math.max(1, parsed); // clamp ≥1
}

/** Widget rows: `◐ <agent> · <state> · <elapsed>s`, max 2 + `… +N`, hidden when idle/child/pretty-off. */
export function widgetRowsFor(runs, options = {}) {
	const env = options.env ?? process.env;
	if (env?.PI_SUBAGENT_CHILD === "1" || String(env?.BIGGZ_PRETTY ?? "").trim() === "0") return [];
	const active = (runs ?? []).filter((run) => isActiveTaskState(run?.state));
	const max = Math.max(1, Math.floor(Number(options.maxRows) || WIDGET_MAX_ROWS));
	const rows = active.slice(0, max).map((run) => {
		const seconds = Math.max(0, Math.round((Number(run?.elapsedMs) || 0) / 1000));
		return `${COMPLETION_GLYPHS[run?.state] ?? "◐"} ${bounded(run?.agent ?? "subagent", 40)} · ${bounded(run?.state ?? "running", 16)} · ${seconds}s`;
	});
	if (active.length > max) rows.push(`… +${active.length - max}`);
	const width = Math.floor(Number(options.width) || 0);
	return width > 0 ? rows.map((row) => truncateToWidth(row, width, "…", false)) : rows;
}

export function createWidgetPublisher(options = {}) {
	const key = options.key ?? WIDGET_KEY;
	const placement = options.placement ?? WIDGET_PLACEMENT;
	const widthOf = () => (typeof options.width === "function" ? options.width() : options.width);
	let ctx = null;
	let timer = null;
	const publish = (runs) => {
		const rows = widgetRowsFor(runs ?? [], { env: options.env ?? process.env, width: widthOf(), maxRows: options.maxRows });
		try {
			ctx?.ui?.setWidget?.(key, rows.length ? rows : undefined, { placement });
		} catch {}
		if (timer && !rows.length) {
			clearInterval(timer);
			timer = null;
		}
		return rows;
	};
	return {
		publish,
		attach(next) {
			ctx = next;
			return this;
		},
		notify(text, type = "info") {
			try {
				ctx?.ui?.notify?.(text, type);
			} catch {}
		},
		watch(runsFor) {
			timer ??= setInterval(() => publish(runsFor()), options.intervalMs ?? WIDGET_INTERVAL_MS);
			timer?.unref?.();
		},
		stop() {
			if (timer) clearInterval(timer);
			timer = null;
		},
	};
}

/** One `Wait {s}s · {N} runs ({agent state, …})` line + at most one hint line. */
export function renderWaitHeadline(runs, elapsedMs, options = {}) {
	const all = runs ?? [];
	const max = Math.max(1, Math.floor(Number(options.maxRuns) || 4));
	const summaries = all.slice(0, max).map((run) => `${bounded(run?.agent ?? "subagent", 40)} ${bounded(run?.state ?? "unknown", 16)}`);
	if (all.length > max) summaries.push(`… +${all.length - max}`);
	const seconds = Math.max(0, Math.round((Number(elapsedMs) || 0) / 1000));
	const head = `Wait ${seconds}s · ${all.length} run${all.length === 1 ? "" : "s"} (${summaries.join(", ")})`;
	const lines = options.hint ? [head, `· ${bounded(options.hint, 120)}`] : [head];
	const width = Math.floor(Number(options.width) || 0);
	return width > 0 ? lines.map((line) => truncateToWidth(line, width, "…", false)) : lines;
}

/** Present a child dialog on the parent UI (`ctx.ui`) and normalize the answer for the child. */
export async function presentUiRequest(ctx, request) {
	const ui = ctx?.ui;
	const title = bounded(request?.title ?? "Subagent question", 120);
	try {
		if (request?.method === "select") {
			const value = await ui?.select?.(title, (request?.options ?? []).map((option) => String(option)));
			return value == null ? { cancelled: true } : { value };
		}
		if (request?.method === "confirm") {
			const confirmed = await ui?.confirm?.(title, bounded(request?.message ?? "", 200));
			return confirmed == null ? { cancelled: true } : { confirmed: confirmed === true };
		}
		if (request?.method === "input" || request?.method === "editor") {
			const value = await ui?.[request.method]?.(title, bounded(request?.placeholder ?? request?.prefill ?? "", 200));
			return value == null ? { cancelled: true } : { value };
		}
	} catch {}
	return { cancelled: true };
}

// ── tool surface: subagent + companions, results only in the ring ──

export function formatTaskResult(task, snapshot = task?.snapshot?.() ?? task) {
	const state = String(snapshot?.state ?? "unknown");
	const seconds = ((Number(snapshot?.elapsedMs) || 0) / 1000).toFixed(1);
	const detail = snapshot?.reason ? ` · ${bounded(snapshot.reason, 80)}` : "";
	return `subagent ${snapshot?.id ?? "?"} · ${state} · ${seconds}s${detail}`;
}

const textResult = (text, details) => ({ content: [{ type: "text", text }], details });
const toolError = (text) => ({ content: [{ type: "text", text }], isError: true });
const paramsOf = (properties, required = []) => ({ type: "object", properties, required });
const unknownTaskError = (id) => `unknown or expired task "${bounded(id, 60)}"`;
const withTimeout = (promises, ms) =>
	new Promise((resolve) => {
		const timer = setTimeout(resolve, Math.max(0, ms));
		timer?.unref?.();
		void Promise.all(promises).then(() => {
			clearTimeout(timer);
			resolve();
		});
	});

export function createSubagentToolset(options = {}) {
	const env = options.env ?? process.env;
	const logger = options.logger ?? console;
	const now = options.now ?? Date.now;
	const registry = options.registry ?? createTaskRegistry({ limit: options.registryLimit });
	const cap = Math.max(1, Math.floor(Number(options.cap) || resolveBackgroundCap(env)));
	const widget = options.widget ?? null;
	const deliverCompletion = options.deliverCompletion ?? null;
	const presentUi = options.presentUiRequest ?? presentUiRequest;
	const managed = []; // background tasks in launch order (FIFO queue)
	const discover = () => options.discoverAgents?.({ env }) ?? discoverAgents({ env });
	const spawnTask = (agent, task, extra = {}) =>
		createTask({
			agent,
			task,
			env,
			logger,
			launch: options.launch,
			args: options.args,
			spawnImpl: options.spawnImpl,
			idleMs: options.idleMs,
			totalMs: options.totalMs,
			finalTextMs: options.finalTextMs,
			onUiRequest: (request) => presentUi(extra.ctx, request),
			...extra,
		});
	const completionRecord = (task, snapshot) => ({
		id: task.id,
		agent: task.agent?.id ?? "subagent",
		state: snapshot?.state ?? task.state,
		elapsedMs: snapshot?.elapsedMs ?? 0,
		summary: bounded(String(snapshot?.finalText ?? "").split("\n")[0] ?? "", 80),
	});
	const activeRuns = () => registry.active().map((task) => ({ agent: task.agent?.id ?? "subagent", state: task.state, elapsedMs: task.snapshot().elapsedMs }));
	const pump = () => {
		let running = managed.filter((task) => task.state === "spawning" || task.state === "running").length;
		for (const task of managed) {
			if (running >= cap) return;
			if (task.state === "queued" && task.start()) running += 1;
		}
	};
	const launchBackground = (agent, taskText, ctx) => {
		const task = spawnTask(agent, taskText, { autoStart: false, ctx });
		registry.add(task);
		managed.push(task);
		void task.promise.then((snapshot) => {
			const index = managed.indexOf(task);
			if (index !== -1) managed.splice(index, 1); // settled tasks leave the FIFO queue
			pump();
			widget?.publish?.(activeRuns());
			try {
				deliverCompletion?.(completionRecord(task, snapshot));
			} catch (err) {
				try {
					logger?.warn?.(`biggz-subagent-runtime: completion delivery failed: ${bounded(err?.message ?? err, 120)}`);
				} catch {}
			}
		});
		widget?.attach?.(ctx);
		widget?.watch?.(activeRuns);
		pump();
		widget?.publish?.(activeRuns());
		return task;
	};
	const subagent = {
		name: "subagent",
		description: "Delegate one task to a markdown agent in a `pi --mode rpc` child (mode task) and return its result with the task id; mode background returns the id immediately",
		parameters: paramsOf(
			{
				agent: { type: "string", description: "Agent id from ~/.pi/agent/agents/*.md" },
				task: { type: "string", description: "Delegated task text" },
				context: { type: "string", description: "fresh (default); fork is unsupported" },
				mode: { type: "string", description: "task (default) or background" },
			},
			["agent", "task"],
		),
		async execute(_callId, params = {}, _signal, _onUpdate, ctx) {
			const nested = nestedSpawnRefusal(env);
			if (nested) return toolError(nested.error);
			const context = String(params.context ?? "fresh");
			if (context !== "fresh") return toolError(`unsupported context "${bounded(context, 40)}"; available: fresh`);
			const mode = String(params.mode ?? "task");
			if (mode !== "task" && mode !== "background") return toolError(`unsupported mode "${bounded(mode, 40)}"; available: task, background`);
			const found = resolveAgent(params.agent, discover(), env);
			if (!found.ok) return toolError(found.error);
			if (mode === "background") {
				const created = launchBackground(found.agent, params.task, ctx);
				return textResult(`background ${created.id} · ${created.state} · cap ${cap}`, { taskId: created.id, state: created.state, mode: "background" });
			}
			const task = spawnTask(found.agent, params.task, { ctx });
			registry.add(task);
			const result = await task.promise;
			return textResult(formatTaskResult(task, result), { taskId: result?.id ?? task.id, state: result?.state ?? task.state });
		},
	};
	const wait = {
		name: "subagent_wait",
		description: "Wait for background runs (task_ids, else every active run) and render a bounded headline (≤2 lines)",
		parameters: paramsOf({ task_ids: { type: "array", items: { type: "string" } }, timeout_ms: { type: "number" } }),
		async execute(_callId, params = {}) {
			const ids = Array.isArray(params.task_ids) ? params.task_ids.map((id) => String(id)) : [];
			const targets = ids.length ? ids.map((id) => registry.get(id)) : registry.active();
			const missing = ids.filter((_id, index) => !targets[index]);
			if (missing.length) return toolError(unknownTaskError(missing[0]));
			if (!targets.length) return textResult("no active subagent tasks", { count: 0 });
			const timeoutMs = Math.max(0, Math.min(600000, Math.floor(Number(params.timeout_ms) || DEFAULT_WAIT_TIMEOUT_MS)));
			const started = now();
			await withTimeout(targets.map((task) => task.promise), timeoutMs);
			const runs = targets.map((task) => ({ agent: task.agent?.id ?? "subagent", state: task.state, elapsedMs: task.snapshot().elapsedMs }));
			const activeLeft = runs.some((run) => isActiveTaskState(run.state));
			const lines = renderWaitHeadline(runs, now() - started, { hint: activeLeft ? "still running; completion notices arrive automatically" : null });
			return textResult(lines.join("\n"), { count: runs.length, states: runs.map((run) => run.state) });
		},
	};
	const status = {
		name: "subagent_status",
		description: "Show state, elapsed time and last activity for one task, or for the active ones",
		parameters: paramsOf({ task_id: { type: "string" } }),
		async execute(_callId, params = {}) {
			if (params.task_id) {
				const task = registry.get(params.task_id);
				if (!task) return toolError(unknownTaskError(params.task_id));
				return textResult(formatTaskResult(task), { taskId: task.id, state: task.state });
			}
			const rows = registry.active().map((task) => formatTaskResult(task));
			return textResult(rows.length ? rows.join("\n") : "no active subagent tasks", { count: rows.length });
		},
	};
	const resultTool = {
		name: "subagent_result",
		description: "Return the bounded output tail for a finished task (max_lines, default 20)",
		parameters: paramsOf({ task_id: { type: "string" }, max_lines: { type: "number" } }, ["task_id"]),
		async execute(_callId, params = {}) {
			const task = registry.get(params.task_id);
			if (!task) return toolError(unknownTaskError(params.task_id));
			const maxLines = Math.max(1, Math.min(200, Math.floor(Number(params.max_lines) || 20)));
			const snapshot = task.snapshot();
			const source = String(snapshot.finalText ?? "").trim() || String(snapshot.stderrTail ?? "");
			const tail = source
				.split("\n")
				.map((line) => line.trimEnd())
				.filter(Boolean)
				.slice(-maxLines);
			const events = tail.length ? tail : snapshot.events.slice(-maxLines).map((event) => `· ${bounded(event?.type ?? "event", 60)}`);
			return textResult([formatTaskResult(task, snapshot), ...events].slice(0, Math.max(1, maxLines)).join("\n"), { taskId: task.id, state: snapshot.state });
		},
	};
	const cancel = {
		name: "subagent_cancel",
		description: "Cancel one task (task_id) or every active task (all: true); kills the whole child tree",
		parameters: paramsOf({ task_id: { type: "string" }, all: { type: "boolean" } }),
		async execute(_callId, params = {}) {
			const targets = params.all === true ? registry.active() : [registry.get(params.task_id)].filter(Boolean);
			if (!targets.length) return toolError(params.all === true ? "no active subagent tasks" : unknownTaskError(params.task_id));
			for (const task of targets) await task.cancel("cancelled");
			return textResult(targets.map((task) => `cancelled ${task.id} · ${task.state}`).join("\n"), { cancelled: targets.length });
		},
	};
	const agentsTool = {
		name: "subagent_agents",
		description: "List discovered markdown agents (~/.pi/agent/agents/*.md) with tools and model",
		parameters: paramsOf({}),
		async execute() {
			const agents = discover();
			if (!agents.length) return textResult(`no agents found in ${resolveAgentsDir(env)}`);
			const rows = agents.map((agent) => `${agent.id} · ${bounded(agent.description || "no description", 80)} · tools: ${agent.tools.join(",")}${agent.model ? ` · model: ${agent.model}` : ""}`);
			return textResult(rows.join("\n"), { count: rows.length });
		},
	};
	return { registry, tools: [subagent, wait, status, resultTool, cancel, agentsTool] };
}

// ── completion card: one pi-tui-truncated line + its message renderer ──

export const COMPLETION_GLYPHS = Object.freeze({ completed: "✅", failed: "❌", stalled: "⏱", cancelled: "⊘", running: "◐", queued: "◌" });

export function completionLine(run) {
	const glyph = COMPLETION_GLYPHS[String(run?.state ?? "")] ?? "•";
	const seconds = ((Number(run?.elapsedMs) || 0) / 1000).toFixed(1);
	const summary = String(run?.summary ?? "")
		.replace(/\s+/g, " ")
		.trim();
	return `${glyph} ${bounded(run?.agent ?? "subagent", 60)} · ${bounded(run?.state ?? "completed", 24)} · ${seconds}s${summary ? ` · ${summary}` : ""}`;
}

export function renderCompletion(run, width) {
	return truncateToWidth(completionLine(run), Math.max(1, Math.floor(Number(width) || 0)), "…", false);
}

export function defaultCompletionWidth(columns = process.stdout?.columns) {
	return Math.min(200, Math.max(20, Math.floor(Number(columns) || 0) || 80));
}

export function createCompletionRenderer(options = {}) {
	return (message) => {
		const details = message?.details && typeof message.details === "object" ? message.details : {};
		const content = typeof message?.content === "string" ? message.content : Array.isArray(message?.content) ? message.content.map((block) => block?.text ?? "").join(" ") : "";
		const line = renderCompletion(
			{ agent: details.agent, state: details.state, elapsedMs: details.elapsedMs, summary: details.summary ?? content },
			options.width ?? defaultCompletionWidth(),
		);
		return { render: () => (line ? [line] : []), invalidate() {} };
	};
}

export const COMPLETION_MESSAGE_TYPE = "biggz-subagent-completion";

// ── pi extension factory (inert: gate + tools + background delivery land here) ──

export default function biggzSubagentRuntime(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (!pi || typeof pi !== "object") return;
	try {
		pi._biggzSubagentRuntime = {
			JSONL_MAX_LINE_BYTES,
			DEFAULT_IDLE_TIMEOUT_MS,
			DEFAULT_TOTAL_TIMEOUT_MS,
			DEFAULT_BACKGROUND_CAP,
			WIDGET_KEY,
			createJsonlReader,
			parseFrontmatter,
			resolveAgentsDir,
			discoverAgents,
			resolveAgent,
			buildDelegatedPrompt,
			buildChildArgs,
			buildChildEnv,
			resolvePiLaunch,
			isPidAlive,
			killTree,
			createTask,
			readSettingsPackages,
			subagentRegistrationGate,
			createTaskRegistry,
			createSubagentToolset,
			completionLine,
			renderCompletion,
			createCompletionRenderer,
			resolveBackgroundCap,
			widgetRowsFor,
			createWidgetPublisher,
			renderWaitHeadline,
			presentUiRequest,
		};
	} catch {}
	const gate = subagentRegistrationGate(pi, { env: process.env });
	if (!gate.register) return; // the gate already logged the reason
	try {
		const widget = createWidgetPublisher({ env: process.env, width: () => defaultCompletionWidth() });
		const deliverCompletion = (run) => {
			const line = completionLine(run);
			try {
				pi.sendMessage?.({ customType: COMPLETION_MESSAGE_TYPE, content: line, display: true, details: run });
			} catch {}
			widget.notify(line, run.state === "completed" ? "info" : "warning");
		};
		const { tools } = createSubagentToolset({ env: process.env, widget, deliverCompletion });
		for (const definition of tools) if (typeof pi.registerTool === "function") pi.registerTool(definition);
		if (typeof pi.registerMessageRenderer === "function") pi.registerMessageRenderer(COMPLETION_MESSAGE_TYPE, createCompletionRenderer());
	} catch (err) {
		try {
			console.warn?.(`biggz-subagent-runtime: registration failed: ${bounded(err?.message ?? err, 120)}`);
		} catch {}
	}
}
