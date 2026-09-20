/**
 * biggz-subagent-runtime — pi subagent delegation runtime.
 *
 * Ships inert until the S3 deploy cutover. S1a: agent discovery
 * (`~/.pi/agent/agents/*.md`), one `pi --mode rpc --no-session` child per task,
 * strict JSONL (LF-only split, CR stripped, 1 MiB line cap — `readline` also
 * splits U+2028/U+2029, legal inside JSON strings, so it is NOT RPC-protocol
 * compliant), stall watchdog (idle 4 min while no tool is in flight / total 30 min) and tree kill
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

import { truncateToWidth, visibleWidth } from "@earendil-works/pi-tui";

export const JSONL_MAX_LINE_BYTES = 1024 * 1024;
export const DEFAULT_IDLE_TIMEOUT_MS = 4 * 60 * 1000;
export const DEFAULT_TOTAL_TIMEOUT_MS = 30 * 60 * 1000;
export const DEFAULT_FINAL_TEXT_MS = 1500;
// A child question is relayed to the user, but never at the cost of the run. An
// unanswered dialog (no UI, unattended session, a modal the user never sees) is
// answered `cancelled` at this bound so the child degrades and continues instead
// of blocking the delegation until the total watchdog.
export const DEFAULT_DIALOG_TIMEOUT_MS = 2 * 60 * 1000;
export const DEFAULT_BACKGROUND_CAP = 2;
// Child transcripts are a debugging aid, not a record: keep the newest N and prune the rest.
export const DEFAULT_SUBAGENT_SESSION_KEEP = 50;
// Child ↔ parent question channel (issue #139 / gentle-shell parity): a child asks the
// orchestrator, never the human, and the answer comes back without a dialog stealing the TUI.
export const PARENT_MESSAGE_TOOL = "subagent_parent_message";
export const PARENT_REPLY_TOOL = "subagent_reply";
export const QUERY_FRAME_TYPE = "biggz_parent_message";
export const QUERY_DIR_ENV = "BIGGZ_SUBAGENT_QUERY_DIR";
export const DEFAULT_QUERY_TIMEOUT_MS = 5 * 60 * 1000;
export const DEFAULT_QUERY_POLL_MS = 250;
export const DEFAULT_QUERY_BUDGET = 5;
// How long a finished run stays visible in the widget so its ✓/✗ is seen before it fades.
export const WIDGET_FINISHED_TTL_MS = 60 * 1000;
export const WIDGET_FINISHED_MAX = 3;
// Row budget for the card above the editor: never fewer than 3, never more than 8, and never
// more than a quarter of the terminal (gentle-shell's numbers).
export const WIDGET_MIN_ROWS = 3;
export const WIDGET_MAX_ROWS = 8;
export const WIDGET_ROWS_RATIO = 0.25;
export const DEFAULT_WAIT_TIMEOUT_MS = 120 * 1000;
export const WIDGET_KEY = "biggz-subagents";
export const WIDGET_PLACEMENT = "belowEditor";
export const WIDGET_INTERVAL_MS = 1000;
export const KILL_GRACE_MS = 5000;
export const KILL_POLL_MS = 50;
export const READ_ONLY_TOOLS = Object.freeze(["read"]);
const DIALOG_METHODS = new Set(["select", "confirm", "input", "editor"]);
const SAFE_TOKEN_RE = /^[A-Za-z0-9_.*\-/:]+$/;
// 0 answers an unanswered question immediately (headless/unattended runs).
export function resolveDialogTimeout(env = process.env) {
	const raw = String(env?.BIGGZ_SUBAGENT_DIALOG_MS ?? "").trim();
	const parsed = Number.parseInt(raw, 10);
	return Number.isFinite(parsed) && parsed >= 0 ? parsed : DEFAULT_DIALOG_TIMEOUT_MS;
}
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
		try {
			options.onDrop?.(reason, detail);
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

/**
 * Where a child writes its session transcript. Defaults under the pi agent dir so every
 * delegation leaves a readable JSONL of what it did; `BIGGZ_SUBAGENT_SESSION_DIR` overrides it.
 */
export function resolveSubagentSessionsDir(env = process.env) {
	const override = typeof env?.BIGGZ_SUBAGENT_SESSION_DIR === "string" ? env.BIGGZ_SUBAGENT_SESSION_DIR.trim() : "";
	if (override) return override;
	const agentDir = typeof env?.PI_CODING_AGENT_DIR === "string" && env.PI_CODING_AGENT_DIR.trim() ? env.PI_CODING_AGENT_DIR.trim() : path.join(os.homedir(), ".pi", "agent");
	return path.join(agentDir, "sessions", "biggz-subagents");
}

/** Keep the newest `keep` transcripts (non-jsonl files untouched); best-effort, never throws. */
export function pruneSubagentSessions(dir, keep = DEFAULT_SUBAGENT_SESSION_KEEP) {
	if (!dir) return 0;
	try {
		const files = fs
			.readdirSync(dir)
			.filter((name) => name.endsWith(".jsonl"))
			.map((name) => {
				const full = path.join(dir, name);
				let mtime = 0;
				try {
					mtime = fs.statSync(full).mtimeMs;
				} catch {}
				return { full, mtime };
			})
			.sort((a, b) => b.mtime - a.mtime);
		let removed = 0;
		for (const file of files.slice(Math.max(0, Math.floor(Number(keep) || 0)))) {
			try {
				fs.rmSync(file.full, { force: true });
				removed += 1;
			} catch {}
		}
		return removed;
	} catch {
		return 0;
	}
}

export function buildChildArgs(agent, options = {}) {
	const tools = (Array.isArray(agent?.tools) ? agent.tools : []).map((token) => String(token).trim()).filter((token) => token && SAFE_TOKEN_RE.test(token));
	const args = ["--mode", "rpc"];
	// A child keeps a session transcript unless the caller asks for an ephemeral run: the file is
	// what makes a finished delegation inspectable (`transcriptPath`) and resumable (`--session`).
	const sessionDir = options.sessionDir === null || options.ephemeral === true ? "" : (options.sessionDir ?? resolveSubagentSessionsDir(options.env ?? process.env));
	if (sessionDir) args.push("--session-dir", sessionDir);
	else args.push("--no-session");
	args.push("--tools", [...(tools.length ? tools : READ_ONLY_TOOLS), PARENT_MESSAGE_TOOL].join(","));
	const model = typeof agent?.model === "string" ? agent.model.trim() : "";
	if (model && SAFE_TOKEN_RE.test(model)) args.push("--model", model);
	return args;
}

export const buildChildEnv = (baseEnv = process.env, options = {}) => {
	const env = { ...baseEnv, PI_SUBAGENT_CHILD: "1" };
	// The child needs to know where to post its questions (and look for the answers).
	if (options.queryDir) env[QUERY_DIR_ENV] = options.queryDir;
	return env;
};

// Inside pi, `process.argv[1]` is the package entry being executed — the same
// `dist/bundle/cli.js` that `pi.cmd`/`pi` launch (`"bin": {"pi": "dist/bundle/cli.js"}`),
// so re-executing it via `process.execPath` skips the Windows .cmd shim entirely.
// Detection is conservative: only that exact installed-package shape counts, so
// anything doubtful keeps the portable fallback. `BIGGZ_PI_BIN` still wins.
const PI_CLI_ENTRY_RE = /\/node_modules\/@earendil-works\/pi-coding-agent\/dist\/bundle\/cli\.c?js$/;

export function resolvePiLaunch(options = {}) {
	const env = options.env ?? process.env;
	const platform = options.platform ?? process.platform;
	const override = typeof env?.BIGGZ_PI_BIN === "string" ? env.BIGGZ_PI_BIN.trim() : "";
	if (override) return { command: override, prefix: [], shell: false };
	const execPath = options.execPath ?? process.execPath;
	const argv1 = String(options.argv1 ?? process.argv?.[1] ?? "");
	if (execPath && PI_CLI_ENTRY_RE.test(argv1.replace(/\\/g, "/"))) return { command: execPath, prefix: [argv1], shell: false };
	// Windows npm shims are .cmd files; safe because argv tokens are sanitized above.
	return platform === "win32" ? { command: "pi.cmd", prefix: [], shell: true } : { command: "pi", prefix: [], shell: false };
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
	const launch = options.launch ?? resolvePiLaunch({ env, platform });
	const spawnImpl = options.spawnImpl ?? nodeSpawn;
	const args = options.args ?? buildChildArgs(agent, { env, sessionDir: options.sessionDir, ephemeral: options.ephemeral });
	const idleMs = options.idleMs ?? DEFAULT_IDLE_TIMEOUT_MS;
	const totalMs = options.totalMs ?? DEFAULT_TOTAL_TIMEOUT_MS;
	const dialogMs = options.dialogMs ?? DEFAULT_DIALOG_TIMEOUT_MS;
	// `relay` (foreground) presents a child question on the parent UI; `dismiss` (background)
	// answers it `cancelled` at once, because nobody is waiting on a background child and a
	// question nobody answers is exactly what used to hang the run (gentle-shell parity).
	const dialogPolicy = options.dialogPolicy === "dismiss" ? "dismiss" : "relay";
	// Where this child writes its transcript (null ⇒ ephemeral `--no-session`).
	const sessionDir = options.ephemeral === true || options.sessionDir === null ? null : (options.sessionDir ?? resolveSubagentSessionsDir(env));
	const queryDir = sessionDir ? resolveQueryDir(sessionDir) : "";
	const queryBudget = Number.isFinite(Number(options.queryBudget)) ? Math.max(0, Math.floor(Number(options.queryBudget))) : DEFAULT_QUERY_BUDGET;
	const startedAt = now();
	const id = options.id ?? `sub-${startedAt.toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
	const events = [];
	const state = { value: options.autoStart === false ? "queued" : "spawning", reason: null, exitCode: null, error: null };
	const inFlightTools = new Set(); // announced tool starts awaiting their end (pi 0.85.1 tool_execution_*)
	let child = null;
	let idleTimer = null;
	let totalTimer = null;
	let finalTextTimer = null;
	let dialogTimer = null;
	let queryTimer = null;
	const queryPollMs = Math.max(200, Math.floor(Number(options.queryPollMs) || DEFAULT_QUERY_POLL_MS * 4));
	let finalText = null;
	let pendingUi = 0;
	let dialogRelayed = 0;
	let dialogMissed = 0;
	let dialogsDismissed = 0;
	let pendingDialog = null;
	let lastStep = "";
	let settledAt = 0;
	let transcriptPath = "";
	let thinking = "";
	const usageTotals = { tokens: 0, cost: 0, model: "" };
	const pendingQueries = new Map();
	let queriesAnswered = 0;
	let started = false;
	let settled = false;
	let stderrTail = "";
	let resolveTask;
	const promise = new Promise((resolve) => {
		resolveTask = resolve;
	});
	const snapshot = () => ({ id, state: state.value, reason: state.reason, exitCode: state.exitCode, error: state.error, events, stderrTail, finalText, pendingUi, dialogMs, dialogRelayed, dialogMissed, dialogsDismissed, lastStep, pendingDialog, tokens: usageTotals.tokens, cost: usageTotals.cost, model: usageTotals.model, thinking, transcriptPath, pendingQueries: pendingQueries.size, pendingQuery: pendingQueries.size ? [...pendingQueries.values()][0] : null, queriesAnswered, settledAt, elapsedMs: (settledAt || now()) - startedAt, pid: child?.pid ?? null });

	function clearTimers() {
		if (idleTimer) clearTimeout(idleTimer);
		if (queryTimer) clearInterval(queryTimer);
		queryTimer = null;
		if (totalTimer) clearTimeout(totalTimer);
		if (finalTextTimer) clearTimeout(finalTextTimer);
		if (dialogTimer) clearTimeout(dialogTimer);
		idleTimer = null;
		totalTimer = null;
		finalTextTimer = null;
		dialogTimer = null;
	}
	function settle(kind, reason, error) {
		if (settled) return;
		settled = true;
		clearTimers();
		detachChildListeners();
		state.value = kind;
		state.reason = reason ?? null;
		if (error !== undefined) state.error = error ? bounded(error?.message ?? error, 300) : null;
		settledAt = now(); // freezes elapsedMs: a finished run must not report a growing time
		if (sessionDir) pruneSubagentSessions(sessionDir);
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
	// pi 0.85.1 `tool_execution_start` / `tool_execution_end` carry
	// `toolCallId` + `toolName` (dist/core/agent-session.js:528-555); correlate on
	// the id and fall back to the tool name (then a constant) so a malformed
	// event degrades conservatively instead of throwing.
	const toolKeyOf = (data) => {
		const callId = typeof data?.toolCallId === "string" ? data.toolCallId.trim() : "";
		if (callId) return callId;
		const toolName = typeof data?.toolName === "string" ? data.toolName.trim() : "";
		return toolName || "__unnamed_tool__";
	};
	const armIdle = () => {
		if (settled) return; // a chunk racing settle() must not resurrect the watchdog
		if (idleTimer) clearTimeout(idleTimer);
		idleTimer = null;
		if (inFlightTools.size > 0) return; // an announced in-flight tool owns the budget; the total bound still governs
		idleTimer = setTimeout(() => stalled("idle"), idleMs);
		idleTimer?.unref?.();
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
		finalTextTimer?.unref?.();
	};
	// Dialog relay: a child `extension_ui_request` (select/confirm/input/editor) is
	// presented by the parent and answered on the child's stdin; fire-and-forget
	// methods and every other event type are ignored. Every relayed question is
	// BOUNDED by `dialogMs` so a parent that never answers cannot leave the child
	// blocked: at the bound the child receives `cancelled` and the run continues.
	// A child question is answered by the orchestrator, never by a dialog: a bounded registry with
	// a budget, so a chatty child degrades to its own judgement instead of blocking the run.
	const answerQuery = async (requestId, message) => {
		const key = bounded(String(requestId ?? ""), 60);
		if (!key) return false;
		const answer = bounded(String(message ?? "").trim(), 4000) || "no answer given";
		if (queryDir) {
			try {
				fs.mkdirSync(queryDir, { recursive: true });
				fs.writeFileSync(path.join(queryDir, `${key}.reply.json`), JSON.stringify({ id: key, message: answer, at: now() }), "utf8");
			} catch {}
		}
		if (pendingQueries.delete(key)) queriesAnswered += 1;
		return true;
	};
	// The child posts questions as files in its query directory (stdout belongs to pi's RPC
	// stream), so the parent sweeps that directory while the task lives and records what it finds.
	const scanQueryFiles = () => {
		if (!queryDir) return 0;
		let names = [];
		try {
			names = fs.readdirSync(queryDir).filter((name) => name.endsWith(".query.json"));
		} catch {
			return 0;
		}
		let seen = 0;
		for (const name of names) {
			const full = path.join(queryDir, name);
			let frame = null;
			try {
				frame = JSON.parse(fs.readFileSync(full, "utf8"));
			} catch {
				frame = null;
			}
			try {
				fs.rmSync(full, { force: true });
			} catch {}
			if (frame) {
				seen += 1;
				recordQuery(frame);
			}
		}
		return seen;
	};
	const recordQuery = (frame) => {
		const requestId = bounded(String(frame?.requestId ?? frame?.id ?? ""), 60);
		const message = bounded(String(frame?.message ?? "").trim(), 800);
		if (!requestId || !message) return;
		const used = queriesAnswered + pendingQueries.size;
		if (used >= queryBudget) {
			void answerQuery(requestId, "question budget for this run is exhausted; continue with your best judgement and state the assumption in your report");
			return;
		}
		pendingQueries.set(requestId, { id: requestId, message, at: now() });
		try {
			options.onQuery?.({ id, agent: agent?.id ?? "subagent", requestId, message, budget: queryBudget, used: used + 1 });
		} catch {}
	};

	async function relayUiRequest(request) {
		if (settled || !DIALOG_METHODS.has(request?.method)) return;
		if (dialogPolicy === "dismiss") {
			dialogsDismissed += 1;
			try {
				logger?.warn?.(`biggz-subagent-runtime: child question dismissed without asking (background ${agent?.id ?? "subagent"}): ${bounded(request?.title ?? "", 80) || "untitled"}`);
			} catch {}
			writeCommand({ type: "extension_ui_response", id: request?.id, cancelled: true });
			return;
		}
		pendingUi += 1;
		if (idleTimer) clearTimeout(idleTimer); // a dialog awaiting the user is not idle
		idleTimer = null;
		dialogRelayed += 1;
		const dialog = { id: request?.id, method: String(request?.method), title: bounded(request?.title ?? "", 120), deadlineAt: now() + dialogMs };
		pendingDialog = dialog;
		let replied = false;
		let closed = false;
		const reply = (payload) => {
			if (replied) return;
			replied = true;
			writeCommand({ type: "extension_ui_response", id: request?.id, ...payload });
		};
		const close = () => {
			if (closed) return;
			closed = true;
			if (dialogTimer) clearTimeout(dialogTimer);
			dialogTimer = null;
			if (pendingDialog === dialog) pendingDialog = null;
			pendingUi = Math.max(0, pendingUi - 1);
			if (!settled) armIdle();
		};
		dialogTimer = setTimeout(() => {
			if (settled || closed) return;
			dialogMissed += 1;
			try {
				logger?.warn?.(`biggz-subagent-runtime: child question unanswered after ${dialogMs}ms (${dialog.title || "untitled"}); answering cancelled so the run continues`);
			} catch {}
			reply({ cancelled: true });
			close();
		}, dialogMs);
		dialogTimer?.unref?.();
		let response = null;
		try {
			response = await options.onUiRequest?.(request, { timeout: dialogMs });
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
		reply(answer);
		close();
	}
	const reader = createJsonlReader({
		logger,
		onLine(data) {
			if (settled) return;
			events.push(data);
			if (events.length > 500) events.shift();
			const step = activityLabel(data);
			if (step) lastStep = step;
			const type = data?.type;
			if (type === "response") {
				if (data?.command === "get_last_assistant_text") {
					finalText = typeof data?.data?.text === "string" ? data.data.text : null;
					finishSettled();
				}
				if (data?.command === "get_state") {
					// The child reports where it writes its transcript (and its model/effort) once it is up.
					const info = data?.data ?? {};
					if (typeof info.sessionFile === "string" && info.sessionFile) transcriptPath = info.sessionFile;
					if (typeof info.thinkingLevel === "string" && info.thinkingLevel) thinking = info.thinkingLevel;
					const modelId = typeof info.model?.id === "string" ? info.model.id : "";
					if (modelId && !usageTotals.model) usageTotals.model = bounded(modelId, 60);
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
			if (type === QUERY_FRAME_TYPE) {
				recordQuery(data);
				return;
			}
			if (type === "tool_execution_start") {
				inFlightTools.add(toolKeyOf(data));
				armIdle(); // suspend the idle bound while the announced tool runs
				return;
			}
			if (type === "tool_execution_end") {
				inFlightTools.delete(toolKeyOf(data));
				armIdle(); // last in-flight tool done ⇒ the idle bound re-arms
				return;
			}
			if (type === "message_end") {
				// Cumulative spend for the row: each assistant message reports its own usage once.
				const message = data?.message;
				if (message?.role === "assistant") {
					const usage = message.usage ?? null;
					const tokens = Number(usage?.totalTokens) || (Number(usage?.input) || 0) + (Number(usage?.output) || 0);
					if (tokens > 0) usageTotals.tokens += tokens;
					const cost = Number(usage?.cost?.total);
					if (cost > 0) usageTotals.cost += cost;
					if (typeof message.model === "string" && message.model) usageTotals.model = message.model;
				}
				return;
			}
			// Progress-only events: agent_start, message_update, tool_execution_update,
			// auto_retry_end, extension_error. Every other event type is ignored.
		},
		// A dropped/malformed line may have been a `tool_execution_end`; when
		// framing integrity is lost, the conservative fallback is the plain idle
		// bound re-arming. Trade-off: an oversized *unrelated* line can re-arm
		// idle while a silent tool runs — strictly rarer than the stuck-key case.
		onDrop() {
			if (inFlightTools.size) {
				inFlightTools.clear();
				armIdle();
			}
		},
	});

	// Named so settle() can detach them: a chunk racing the settle must not
	// re-arm the idle watchdog, grow events/stderrTail, or queue more work.
	function onStdout(chunk) {
		armIdle();
		reader.push(chunk);
	}
	function onStderr(chunk) {
		stderrTail = `${stderrTail}${String(chunk)}`.slice(-4096);
	}
	function detachChildListeners() {
		try {
			child?.stdout?.off?.("data", onStdout);
			child?.stderr?.off?.("data", onStderr);
		} catch {}
	}

	function start() {
		if (started || settled) return false;
		started = true;
		state.value = "spawning";
		totalTimer = setTimeout(() => stalled("total"), totalMs);
		totalTimer?.unref?.();
		armIdle();
		try {
			child = spawnImpl(launch.command, [...(launch.prefix ?? []), ...args], {
				cwd: options.cwd,
				env: buildChildEnv(env, { queryDir }),
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
				child.stdout?.on?.("data", onStdout);
				child.stderr?.on?.("data", onStderr);
				// A killed child's stdin is a dead pipe: without a listener the EPIPE surfaces as an
				// uncaughtException inside the parent (pi's TUI) and the delegation never reports.
				child.stdin?.on?.("error", (err) => {
					try {
						logger?.debug?.(`biggz-subagent-runtime: child stdin closed (${bounded(err?.message ?? err, 80)})`);
					} catch {}
				});
				// Decode at the stream boundary so a multibyte char split across chunks
				// stays intact; the JSONL reader keeps its Buffer branch for direct pushes.
				child.stdout?.setEncoding?.("utf8");
				child.stderr?.setEncoding?.("utf8");
				child.on?.("error", (err) => {
					if (!settled && state.value !== "stalled" && state.value !== "cancelled") settle("failed", "spawn", err);
				});
				child.on?.("exit", (code) => {
					state.exitCode = code;
					if (!settled && state.value !== "stalled" && state.value !== "cancelled") settle("failed", `exit:${code ?? "signal"}`);
				});
				writeCommand({ id: `${id}-prompt-1`, type: "prompt", message: options.firstMessage ?? buildDelegatedPrompt(agent, options.task) });
				if (sessionDir) {
					try {
						fs.mkdirSync(sessionDir, { recursive: true });
					} catch {}
					writeCommand({ id: `${id}-state-1`, type: "get_state" });
				}
				if (queryDir) {
					queryTimer = setInterval(scanQueryFiles, queryPollMs);
					queryTimer?.unref?.();
				}
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
		task: options.task ?? "",
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
		pendingQueries: () => [...pendingQueries.values()],
		scanQueries: () => scanQueryFiles(),
		answerQuery: (requestId, message) => answerQuery(requestId, message),
		queryBudget,
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

// Exact j0k3r-era names only: the runtime's own `subagent_list_tasks` must
// never classify as legacy (delta spec `Runtime's own list tool is not legacy`).
export const LEGACY_TOOL_RE = /^subagent_run$|^subagent_list_running$/;
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

/** `12345` → `12.3k`, `1234567` → `1.2M`: keeps a widget row short. */
export function formatTokens(value) {
	const total = Math.max(0, Math.round(Number(value) || 0));
	if (total < 1000) return String(total);
	if (total < 1000000) return `${(total / 1000).toFixed(1)}k`;
	return `${(total / 1000000).toFixed(1)}M`;
}

/** Sub-cent costs keep four decimals, anything else two; empty when nothing was spent. */
export function formatCost(value) {
	const total = Number(value) || 0;
	if (total <= 0) return "";
	return total < 0.01 ? `$${total.toFixed(4)}` : `$${total.toFixed(2)}`;
}

/**
 * One-line "what is this child doing" label derived from its own RPC events: the tool it
 * announced (plus the target it was given) while a tool runs, `thinking` while it answers.
 * Empty for chatty or irrelevant events so the label never flickers mid-stream.
 */
export function activityLabel(event) {
	if (!event || typeof event !== "object") return "";
	if (event.type === "tool_execution_start") {
		const tool = bounded(String(event.toolName ?? ""), 20);
		const args = event.args && typeof event.args === "object" ? event.args : {};
		const target = String(args.path ?? args.pattern ?? args.command ?? args.query ?? args.url ?? "").trim().split("\n")[0];
		return bounded(target ? `${tool} ${target}` : tool || "tool", 60);
	}
	if (event.type === "message_start" && event.message?.role === "assistant") return "thinking";
	return "";
}

/** One widget/panel row for a run, degrading gracefully when the width runs out. */
export function runRow(run, width = 0) {
	const seconds = Math.max(0, Math.round((Number(run?.elapsedMs) || 0) / 1000));
	// What the child is DOING beats which state it is in: the glyph already carries the state.
	const detail = bounded(String(run?.lastStep || run?.summary || run?.state || "running"), 60);
	const model = [String(run?.model ?? "").trim(), String(run?.thinking ?? "").trim()].filter(Boolean).join(" · ");
	const tokens = Number(run?.tokens) || 0;
	const cost = Number(run?.cost) || 0;
	const spend = [tokens ? `${formatTokens(tokens)} tok` : "", cost ? formatCost(cost) : ""].filter(Boolean).join(" · ");
	const head = `${COMPLETION_GLYPHS[run?.state] ?? "◐"} ${bounded(run?.agent ?? "subagent", 40)} · ${detail}`;
	// Drop from the least important end first (spend, then model) so the activity and the
	// elapsed time — the two things you read at a glance — survive a narrow terminal.
	const variants = [[model, spend, `${seconds}s`], [spend, `${seconds}s`], [`${seconds}s`]];
	let row = "";
	for (const parts of variants) {
		row = [head, ...parts.filter(Boolean)].join(" · ");
		if (width <= 0 || visibleWidth(row) <= width) break;
	}
	return width > 0 ? truncateToWidth(row, width, "…", false) : row;
}

/** Row budget: min 3, max 8, else a quarter of the terminal; falls back to the minimum. */
export function widgetRowBudget(terminalRows, options = {}) {
	const min = Math.max(1, Math.floor(Number(options.min) || WIDGET_MIN_ROWS));
	const max = Math.max(min, Math.floor(Number(options.max) || WIDGET_MAX_ROWS));
	const rows = Math.floor(Number(terminalRows) || 0);
	if (rows <= 0) return min;
	return Math.max(min, Math.min(max, Math.floor(rows * WIDGET_ROWS_RATIO)));
}

/** Widget rows: `<glyph> <agent> · <live activity|task summary|state> · <elapsed>s · <tokens · cost>`, folded with `… +N`; finished runs stay for `WIDGET_FINISHED_TTL_MS`. */
export function widgetRowsFor(runs, options = {}) {
	const env = options.env ?? process.env;
	if (env?.PI_SUBAGENT_CHILD === "1" || String(env?.BIGGZ_PRETTY ?? "").trim() === "0") return [];
	const nowMs = Number(options.now) || Date.now();
	const ttl = Number.isFinite(Number(options.finishedTtlMs)) ? Number(options.finishedTtlMs) : WIDGET_FINISHED_TTL_MS;
	// Running runs plus the ones that just finished, so a completion is seen before it fades.
	const visible = (runs ?? []).filter((run) => isActiveTaskState(run?.state) || (Number(run?.settledAt) || 0) > nowMs - ttl);
	const max = Math.max(1, Math.floor(Number(options.maxRows) || widgetRowBudget(options.terminalRows)));
	const width = Math.floor(Number(options.width) || 0);
	// Each row sheds what it can before the final truncation: the width is the real budget.
	const rows = visible.slice(0, max).map((run) => runRow(run, width));
	if (visible.length > max) rows.push(`… +${visible.length - max}`);
	return width > 0 ? rows.map((row) => truncateToWidth(row, width, "…", false)) : rows;
}

export function createWidgetPublisher(options = {}) {
	const key = options.key ?? WIDGET_KEY;
	const placement = options.placement ?? WIDGET_PLACEMENT;
	const widthOf = () => (typeof options.width === "function" ? options.width() : options.width);
	let ctx = null;
	let timer = null;
	const publish = (runs) => {
		const rows = widgetRowsFor(runs ?? [], {
			env: options.env ?? process.env,
			width: widthOf(),
			maxRows: options.maxRows,
			terminalRows: typeof options.terminalRows === "function" ? options.terminalRows() : options.terminalRows,
			now: Date.now(),
		});
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

/** Runs for the widget and the panel: live work first, then the last few completions. */
export function registryRuns(registry, nowMs = Date.now()) {
	const runs = (registry?.all?.() ?? []).map((task) => {
		const snapshot = typeof task?.snapshot === "function" ? task.snapshot() : {};
		return {
			id: task?.id ?? "?",
			agent: task?.agent?.id ?? "subagent",
			state: task?.state ?? "unknown",
			elapsedMs: snapshot.elapsedMs ?? 0,
			settledAt: snapshot.settledAt ?? 0,
			lastStep: snapshot.lastStep ?? "",
			model: snapshot.model ?? "",
			thinking: snapshot.thinking ?? "",
			summary: bounded(String(task?.task ?? "").split("\n")[0] ?? "", 60),
			tokens: snapshot.tokens ?? 0,
			cost: snapshot.cost ?? 0,
		};
	});
	const active = runs.filter((run) => isActiveTaskState(run.state));
	// Keep the last few completions visible for a moment, so a ✓/✗ is seen before it fades.
	const finished = runs.filter((run) => !isActiveTaskState(run.state) && (Number(run.settledAt) || 0) > nowMs - WIDGET_FINISHED_TTL_MS).slice(-WIDGET_FINISHED_MAX);
	return [...active, ...finished];
}

/** One compact line for a child session record (raw JSONL is unreadable); empty when noise. */
export function transcriptEntry(record) {
	const message = record?.message ?? null;
	const stamp = typeof record?.timestamp === "string" ? record.timestamp.slice(11, 19) : "";
	const when = stamp ? `${stamp} ` : "";
	if (!message) return record?.type === "session" ? `${when}── session start` : "";
	const parts = Array.isArray(message.content) ? message.content : [];
	const text = parts
		.filter((part) => part?.type === "text")
		.map((part) => String(part.text ?? "").replace(/\s+/g, " ").trim())
		.join(" ");
	const tools = parts
		.filter((part) => part?.type === "toolCall")
		.map((part) => `⚙ ${String(part.name ?? "tool")} ${String(part.arguments?.path ?? part.arguments?.command ?? part.arguments?.pattern ?? "").trim()}`.trim());
	const bits = [...tools];
	if (text) bits.push(text);
	if (!bits.length && parts.some((part) => part?.type === "thinking")) bits.push("(thinking)");
	if (!bits.length) return "";
	return bounded(`${when}${bounded(String(message.role ?? "?"), 10).padEnd(10)} ${bits.join(" · ")}`, 400);
}

/** Bounded read of a child transcript into compact lines (tail-first); never throws. */
export function transcriptEntries(file, options = {}) {
	const maxBytes = Math.max(4096, Math.floor(Number(options.maxBytes) || 256 * 1024));
	const maxLines = Math.max(1, Math.floor(Number(options.maxLines) || 400));
	if (!file) return [];
	try {
		const size = fs.statSync(file).size;
		const start = Math.max(0, size - maxBytes);
		const length = size - start;
		if (length <= 0) return [];
		const fd = fs.openSync(file, "r");
		let raw = "";
		try {
			const buffer = Buffer.alloc(length);
			fs.readSync(fd, buffer, 0, length, start);
			raw = buffer.toString("utf8");
		} finally {
			fs.closeSync(fd);
		}
		if (start > 0) raw = raw.slice(raw.indexOf("\n") + 1); // drop the partial first line
		const out = [];
		for (const line of raw.split("\n").filter(Boolean).slice(-maxLines)) {
			let record = null;
			try {
				record = JSON.parse(line);
			} catch {
				continue;
			}
			const entry = transcriptEntry(record);
			if (entry) out.push(entry);
		}
		return out;
	} catch {
		return [];
	}
}

export const AGENTS_COMMAND = "biggz-agents";
// `alt+a` mirrors gentle-shell; `BIGGZ_AGENTS_KEY` overrides it, and off/none/disabled turns it off.
export const AGENTS_SHORTCUT = "alt+a";

/** Open the runs panel as a pi overlay: `/biggz-agents` and the shortcut share exactly this. */
export async function openSubagentPanel(ctx, options = {}) {
	const ui = ctx?.ui;
	if (typeof ui?.custom !== "function" || ctx?.mode !== "tui") {
		try {
			ui?.notify?.("the subagents panel needs TUI mode", "warning");
		} catch {}
		return null;
	}
	const registry = options.registry;
	return ui.custom(
		(tui, theme, _keybindings, done) => {
			const tick = setInterval(() => {
				try {
					tui?.requestRender?.();
				} catch {}
			}, WIDGET_INTERVAL_MS);
			tick?.unref?.();
			return createAgentsView({
				theme,
				runs: () => registryRuns(registry),
				stop: (run) => {
					const task = registry?.get?.(run?.id);
					if (!task || !isActiveTaskState(task.state)) return false;
					void task.cancel("stopped from the agents panel");
					return true;
				},
				transcript: (run) => {
					const task = registry?.get?.(run?.id);
					const file = task?.snapshot?.()?.transcriptPath ?? "";
					if (!file) return null;
					return { path: file, lines: transcriptEntries(file) };
				},
				close: (value) => {
					clearInterval(tick);
					done(value);
				},
				requestRender: () => {
					try {
						tui?.requestRender?.();
					} catch {}
				},
			});
		},
		{ overlay: true, overlayOptions: { anchor: "top-right", width: "60%", margin: 1 } },
	);
}

/**
 * Body of the `/biggz-agents` panel: one row per run (live activity + spend), `s` stops the
 * selected run, `q`/Esc closes. Deliberately tiny and total: a panel bug must never be able
 * to freeze the TUI it is drawn in.
 */
export function createAgentsView(options = {}) {
	const theme = options.theme ?? { fg: (_role, text) => String(text), bold: (text) => String(text) };
	const runsFor = typeof options.runs === "function" ? options.runs : () => options.runs ?? [];
	const stop = typeof options.stop === "function" ? options.stop : () => false;
	const readTranscript = typeof options.transcript === "function" ? options.transcript : () => null;
	const close = typeof options.close === "function" ? options.close : () => {};
	const repaint = typeof options.requestRender === "function" ? options.requestRender : () => {};
	let selected = 0;
	let status = "";
	// `list` is the run table; `transcript` reads one child's own session file inside the panel.
	const view = { mode: "list", agent: "", path: "", lines: [], scroll: 0, missing: "", run: null };
	const TRANSCRIPT_PAGE = 14;
	const rows = () => {
		try {
			return runsFor() ?? [];
		} catch {
			return [];
		}
	};
	return {
		render(width) {
			try {
				const inner = Math.max(12, Math.floor(Number(width) || 0) - 3);
				if (view.mode === "transcript") {
					const header = theme.fg("accent", theme.bold(`Transcript · ${bounded(view.agent, 24)}`));
					const lines = [header, theme.fg("dim", bounded(view.path || "(no session file yet)", inner)), ""];
					if (view.missing) lines.push(theme.fg("warning", view.missing));
					const start = Math.max(0, Math.min(view.scroll, Math.max(0, view.lines.length - TRANSCRIPT_PAGE)));
					const page = view.lines.slice(start, start + TRANSCRIPT_PAGE);
					if (!page.length && !view.missing) lines.push(theme.fg("dim", "(transcript is empty so far)"));
					for (const line of page) lines.push(truncateToWidth(line, inner, "…", false));
					lines.push("");
					lines.push(theme.fg("dim", `↑↓ scroll · f follow · q back (${start + page.length}/${view.lines.length})`));
					return lines;
				}
				const runs = rows();
				if (selected > runs.length - 1) selected = Math.max(0, runs.length - 1);
				const lines = [theme.fg("accent", theme.bold("Subagent runs")), ""];
				if (!runs.length) lines.push(theme.fg("dim", "no subagent runs in this session"));
				for (const [index, run] of runs.entries()) {
					const marker = index === selected ? theme.fg("accent", "→ ") : "  ";
					lines.push(`${marker}${runRow(run, inner)}`);
				}
				lines.push("");
				lines.push(theme.fg("dim", "↑↓ move · s stop · o transcript · q close"));
				if (status) lines.push(theme.fg("warning", status));
				return lines;
			} catch (err) {
				return [theme.fg("dim", `subagents panel error: ${bounded(err?.message ?? err, 80)}`)];
			}
		},
		handleInput(keyData) {
			const key = String(keyData ?? "");
			if (view.mode === "transcript") {
				if (key === "q" || key === "escape" || key === "\u0003") {
					view.mode = "list";
					repaint();
					return;
				}
				if (key === "j" || key === "down" || key === "\u001b[B") {
					view.scroll += 1;
					repaint();
					return;
				}
				if (key === "k" || key === "up" || key === "\u001b[A") {
					view.scroll = Math.max(0, view.scroll - 1);
					repaint();
					return;
				}
				if (key === "f") {
					// Refresh the transcript and jump to its end (the file keeps growing while the child runs).
					let fresh = null;
					try {
						fresh = readTranscript(view.run ?? { id: view.id, agent: view.agent });
					} catch {
						fresh = null;
					}
					if (Array.isArray(fresh?.lines)) {
						view.lines = fresh.lines;
						view.path = String(fresh.path ?? view.path);
						view.missing = "";
					}
					view.scroll = Math.max(0, view.lines.length - TRANSCRIPT_PAGE);
					repaint();
					return;
				}
				if (key === "pagedown" || key === "\u001b[6~") {
					view.scroll += TRANSCRIPT_PAGE;
					repaint();
					return;
				}
				if (key === "pageup" || key === "\u001b[5~") {
					view.scroll = Math.max(0, view.scroll - TRANSCRIPT_PAGE);
					repaint();
				}
				return;
			}
			if (key === "q" || key === "escape" || key === "\u0003") {
				close(null);
				return;
			}
			if (key === "j" || key === "down" || key === "\u001b[B") {
				selected = Math.min(Math.max(0, rows().length - 1), selected + 1);
				repaint();
				return;
			}
			if (key === "k" || key === "up" || key === "\u001b[A") {
				selected = Math.max(0, selected - 1);
				repaint();
				return;
			}
			if (key === "o") {
				const run = rows()[selected];
				if (!run) return;
				let opened = null;
				try {
					opened = readTranscript(run);
				} catch {
					opened = null;
				}
				view.mode = "transcript";
				view.run = run;
				view.agent = String(run.agent ?? "subagent");
				view.path = String(opened?.path ?? opened?.transcriptPath ?? "");
				view.lines = Array.isArray(opened?.lines) ? opened.lines : [];
				view.missing = view.path ? "" : "this run has no session file (ephemeral or still starting)";
				view.scroll = Math.max(0, view.lines.length - TRANSCRIPT_PAGE); // follow the tail first
				repaint();
				return;
			}
			if (key === "s") {
				const run = rows()[selected];
				status = run && stop(run) ? `stopping ${run.agent}…` : "nothing to stop";
				repaint();
			}
		},
		invalidate() {},
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
export async function presentUiRequest(ctx, request, dialogOpts = {}) {
	const ui = ctx?.ui;
	const base = bounded(request?.title ?? "Subagent question", 120);
	// Name the asker: a subagent question must be identifiable on sight, so the human knows
	// who is waiting on the answer before touching the keyboard.
	const label = bounded(String(dialogOpts?.agentLabel ?? ""), 40);
	const title = label ? `${label}: ${base}` : base;
	// The parent dialog carries the same budget the runtime enforces, so the TUI
	// shows a countdown and auto-dismisses instead of waiting on the user forever.
	const timeout = Math.max(0, Math.floor(Number(dialogOpts?.timeout) || 0));
	const opts = timeout > 0 ? { timeout } : undefined;
	try {
		// A child question the user never notices is the difference between a
		// degraded answer and a hung delegation; announce it before presenting.
		ui?.notify?.(`Subagent question: ${title}`, "info");
	} catch {}
	try {
		if (request?.method === "select") {
			const value = await ui?.select?.(title, (request?.options ?? []).map((option) => String(option)), opts);
			return value == null ? { cancelled: true } : { value };
		}
		if (request?.method === "confirm") {
			const confirmed = await ui?.confirm?.(title, bounded(request?.message ?? "", 200), opts);
			return confirmed == null ? { cancelled: true } : { confirmed: confirmed === true };
		}
		if (request?.method === "input" || request?.method === "editor") {
			const value = await ui?.[request.method]?.(title, bounded(request?.placeholder ?? request?.prefill ?? "", 200), opts);
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
	const waiting = snapshot?.pendingDialog ? ` · awaiting answer: ${bounded(snapshot.pendingDialog.title || "question", 60)}` : "";
	// Live activity while it runs, so a status call answers "what is it doing?" and not just
	// "is it alive?" — the glyph/state already carries the latter.
	const step = !waiting && isActiveTaskState(state) && snapshot?.lastStep ? ` · ${bounded(snapshot.lastStep, 60)}` : "";
	const query = waiting || !snapshot?.pendingQuery?.message ? "" : ` · awaiting parent: ${bounded(snapshot.pendingQuery.message, 60)}`;
	const tokens = Number(snapshot?.tokens) || 0;
	const cost = Number(snapshot?.cost) || 0;
	const spend = `${tokens ? ` · ${formatTokens(tokens)} tok` : ""}${cost ? ` · ${formatCost(cost)}` : ""}`;
	return `subagent ${snapshot?.id ?? "?"} · ${state} · ${seconds}s${detail}${waiting}${query}${step}${spend}`;
}

/** Bounded result tail: trimmed `finalText` else `stderrTail`, blank lines collapsed; event types when no text. */
export function boundedResultTail(snapshot, maxLines = 20) {
	const limit = Math.max(1, Math.floor(Number(maxLines) || 20));
	const source = String(snapshot?.finalText ?? "").trim() || String(snapshot?.stderrTail ?? "");
	const tail = source
		.split("\n")
		.map((line) => line.trimEnd())
		.filter(Boolean)
		.slice(-limit);
	return tail.length ? tail : (snapshot?.events ?? []).slice(-limit).map((event) => `· ${bounded(event?.type ?? "event", 60)}`);
}

const textResult = (text, details) => ({ content: [{ type: "text", text }], details });
const toolError = (text) => ({ content: [{ type: "text", text }], isError: true });
const paramsOf = (properties, required = []) => ({ type: "object", properties, required });

/** `.../<session dir>/queries` — where a child posts questions and reads its answers. */
export const resolveQueryDir = (sessionDir) => (sessionDir ? path.join(sessionDir, "queries") : "");

/**
 * Child side: ask the run's parent a question and wait for the answer file it writes back.
 * A file (not stdin/stdout framing) carries the answer, so nothing can hijack pi's RPC stream;
 * the question itself rides stdout as one extra JSONL line the parent's reader recognises.
 */
export function createParentMessageTool(options = {}) {
	const env = options.env ?? process.env;
	const dir = String(options.queryDir ?? env[QUERY_DIR_ENV] ?? "").trim();
	const timeoutMs = Math.max(1000, Math.floor(Number(options.timeoutMs) || DEFAULT_QUERY_TIMEOUT_MS));
	const pollMs = Math.max(25, Math.floor(Number(options.pollMs) || DEFAULT_QUERY_POLL_MS));
	const writeFrame = typeof options.writeFrame === "function" ? options.writeFrame : () => {};
	const sleep = typeof options.sleep === "function" ? options.sleep : (ms) => new Promise((resolve) => setTimeout(resolve, ms));
	return {
		name: PARENT_MESSAGE_TOOL,
		description: "Ask this run's parent (the orchestrator) a question when a decision blocks the task; it can answer with context you cannot see. Use it sparingly — prefer deciding and stating your assumption.",
		parameters: paramsOf({ message: { type: "string" } }, ["message"]),
		async execute(_callId, params = {}) {
			const message = bounded(String(params.message ?? "").trim(), 800);
			if (!message) return toolError("message is required");
			if (!dir) return textResult("no parent channel is available for this run; continue with your best judgement and state the assumption in your report");
			const requestId = `q-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
			const frame = { type: QUERY_FRAME_TYPE, id: requestId, requestId, message, at: Date.now() };
			// The parent polls this directory: writing to stdout is NOT an option inside an RPC child —
			// pi owns that stream and an extension line never reaches the client (verified live).
			try {
				fs.mkdirSync(dir, { recursive: true });
				fs.writeFileSync(path.join(dir, `${requestId}.query.json`), JSON.stringify(frame), "utf8");
			} catch {}
			try {
				writeFrame(frame);
			} catch {}
			const replyPath = path.join(dir, `${requestId}.reply.json`);
			const deadline = Date.now() + timeoutMs;
			while (Date.now() < deadline) {
				let raw = "";
				try {
					raw = fs.readFileSync(replyPath, "utf8");
				} catch {
					raw = "";
				}
				if (raw) {
					try {
						fs.rmSync(replyPath, { force: true });
					} catch {}
					let answer = "";
					try {
						answer = String(JSON.parse(raw)?.message ?? "");
					} catch {
						answer = raw;
					}
					const text = bounded(answer.trim(), 4000);
					return textResult(text ? `answer from the parent: ${text}` : "the parent replied with an empty answer; continue with your best judgement");
				}
				await sleep(pollMs);
			}
			return textResult(`no answer from the parent within ${Math.round(timeoutMs / 1000)}s; continue with your best judgement and state the assumption in your report`);
		},
	};
}

/** In a child process: register the question tool only — the delegation surface belongs to the parent. */
export function registerChildChannel(pi, options = {}) {
	if (!pi || typeof pi.registerTool !== "function") return false;
	try {
		pi.registerTool(createParentMessageTool(options));
		return true;
	} catch {
		return false;
	}
}
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
	const spawnTask = (agent, task, extra = {}) => {
		const agentLabel = bounded(String(agent?.id ?? agent?.name ?? "subagent"), 40);
		return createTask({
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
			onQuery: options.onQuery,
			dialogMs: options.dialogMs ?? resolveDialogTimeout(env),
			onUiRequest: (request, dialogOpts) => presentUi(extra.ctx, request, { ...dialogOpts, agentLabel }),
			...extra,
		});
	};
	const completionRecord = (task, snapshot) => ({
		id: task.id,
		agent: task.agent?.id ?? "subagent",
		state: snapshot?.state ?? task.state,
		elapsedMs: snapshot?.elapsedMs ?? 0,
		summary: bounded(String(snapshot?.finalText ?? "").split("\n")[0] ?? "", 80),
	});
	const activeRuns = () => registryRuns(registry, now());
	const pump = () => {
		let running = managed.filter((task) => task.state === "spawning" || task.state === "running").length;
		for (const task of managed) {
			if (running >= cap) return;
			if (task.state === "queued" && task.start()) running += 1;
		}
	};
	const launchBackground = (agent, taskText, ctx) => {
		const task = spawnTask(agent, taskText, { autoStart: false, ctx, dialogPolicy: "dismiss" });
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
		async execute(_callId, params = {}, signal, _onUpdate, ctx) {
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
			// A foreground delegation must show progress in the TUI: a slow child used to look
			// like a frozen screen with nothing to cancel.
			widget?.attach?.(ctx);
			widget?.watch?.(activeRuns);
			widget?.publish?.(activeRuns());
			// Esc must end a delegation. pi aborts the turn through this signal; ignoring it left
			// the child running while the UI waited forever, so killing pi was the only exit.
			let aborted = false;
			const onAbort = () => {
				aborted = true;
				void task.cancel("cancelled");
			};
			if (signal?.aborted) onAbort();
			else signal?.addEventListener?.("abort", onAbort, { once: true });
			let result;
			try {
				result = await task.promise;
			} finally {
				signal?.removeEventListener?.("abort", onAbort);
				widget?.publish?.(activeRuns());
			}
			// The caller gets the settlement line plus the child's bounded final text.
			const tail = boundedResultTail(result ?? task.snapshot());
			const missed = Math.max(0, Number(result?.dialogMissed) || 0);
			const dismissed = Math.max(0, Number(result?.dialogsDismissed) || 0);
			const notice = [];
			if (aborted) notice.push("· cancelled by user (Esc) — the child tree was killed");
			if (missed) notice.push(`· ${missed} child question${missed === 1 ? "" : "s"} not answered within ${Math.max(1, Math.round((Number(result?.dialogMs) || DEFAULT_DIALOG_TIMEOUT_MS) / 1000))}s — the child was told to continue without it`);
			if (dismissed) notice.push(`· ${dismissed} child question${dismissed === 1 ? "" : "s"} dismissed without asking — the child was told to continue without it`);
			// The child's own session file: the only way to read a finished run in full.
			if (result?.transcriptPath) notice.push(`· transcript: ${result.transcriptPath}`);
			return textResult([formatTaskResult(task, result), ...tail, ...notice].join("\n"), { taskId: result?.id ?? task.id, state: result?.state ?? task.state, dialogsMissed: missed, dialogsDismissed: dismissed });
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
			const tail = boundedResultTail(snapshot, maxLines);
			return textResult([formatTaskResult(task, snapshot), ...tail].slice(0, Math.max(1, maxLines)).join("\n"), { taskId: task.id, state: snapshot.state });
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
	const reply = {
		name: PARENT_REPLY_TOOL,
		description: "Answer a subagent's question (task_id + request_id from the query message); the answer reaches the child",
		parameters: paramsOf({ task_id: { type: "string" }, request_id: { type: "string" }, message: { type: "string" } }, ["task_id", "request_id", "message"]),
		async execute(_callId, params = {}) {
			const task = registry.get(params.task_id);
			if (!task) return toolError(unknownTaskError(params.task_id));
			const requestId = String(params.request_id ?? "").trim();
			const message = String(params.message ?? "").trim();
			if (!requestId) return toolError("request_id is required");
			if (!message) return toolError("message is required");
			if (typeof task.answerQuery !== "function") return toolError(`task ${task.id} has no question channel`);
			const known = task.pendingQueries().some((query) => query.id === requestId);
			await task.answerQuery(requestId, message);
			return textResult(
				`answered ${task.id} · ${requestId}${known ? "" : " (no pending question with that id — the child may have timed out or answered already)"}`,
				{ taskId: task.id, requestId, delivered: known },
			);
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
	const listTasks = {
		name: "subagent_list_tasks",
		description: "List this session's subagent task records, newest first (settled included, bounded to the ring limit)",
		parameters: paramsOf({}),
		async execute() {
			const rows = registry.all().slice(-registry.limit).reverse().map((task) => formatTaskResult(task));
			return textResult(rows.length ? rows.join("\n") : "no subagent tasks this session", { count: rows.length });
		},
	};
	const sendMessage = {
		name: "subagent_send_message",
		description: "Send a steering message to a running subagent task (task_id, message)",
		parameters: paramsOf({ task_id: { type: "string" }, message: { type: "string" } }, ["task_id", "message"]),
		async execute(_callId, params = {}) {
			const task = registry.get(params.task_id);
			if (!task) return toolError(unknownTaskError(params.task_id));
			const state = String(task.state);
			if ((state !== "running" && state !== "spawning") || task.steer(params.message) !== true) {
				return toolError(`task "${bounded(params.task_id, 60)}" is not running (${bounded(String(task.state), 20)}); steer not delivered`);
			}
			return textResult(`steered ${task.id} · ${task.state}`, { taskId: task.id, state: task.state });
		},
	};
	return { registry, tools: [subagent, wait, status, resultTool, cancel, agentsTool, listTasks, sendMessage, reply] };
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
// A child's question arrives in the parent conversation under its own type (issue #139).
export const QUERY_MESSAGE_TYPE = "biggz-subagent-query";

// ── pi extension factory (inert: gate + tools + background delivery land here) ──

export default function biggzSubagentRuntime(pi) {
	if (!pi || typeof pi !== "object") return;
	// Inside a delegation child the runtime registers ONE thing: the question tool. The delegation
	// surface (and the human) belongs to the parent, so a child can ask its orchestrator, not a dialog.
	if (process.env.PI_SUBAGENT_CHILD === "1") {
		registerChildChannel(pi, { env: process.env });
		return;
	}
	try {
		pi._biggzSubagentRuntime = {
			JSONL_MAX_LINE_BYTES,
			DEFAULT_IDLE_TIMEOUT_MS,
			DEFAULT_TOTAL_TIMEOUT_MS,
			DEFAULT_DIALOG_TIMEOUT_MS,
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
			resolveDialogTimeout,
			widgetRowsFor,
			runRow,
			widgetRowBudget,
			registryRuns,
			createAgentsView,
			openSubagentPanel,
			AGENTS_COMMAND,
			AGENTS_SHORTCUT,
			resolveSubagentSessionsDir,
			pruneSubagentSessions,
			transcriptEntries,
			transcriptEntry,
			DEFAULT_SUBAGENT_SESSION_KEEP,
			PARENT_MESSAGE_TOOL,
			PARENT_REPLY_TOOL,
			QUERY_FRAME_TYPE,
			QUERY_DIR_ENV,
			DEFAULT_QUERY_TIMEOUT_MS,
			DEFAULT_QUERY_BUDGET,
			resolveQueryDir,
			createParentMessageTool,
			registerChildChannel,
			createWidgetPublisher,
			renderWaitHeadline,
			presentUiRequest,
			boundedResultTail,
		};
	} catch {}
	const gate = subagentRegistrationGate(pi, { env: process.env });
	if (!gate.register) return; // the gate already logged the reason
	try {
		const widget = createWidgetPublisher({
			env: process.env,
			width: () => defaultCompletionWidth(),
			terminalRows: () => Number(process.stdout?.rows) || 0,
		});
		const deliverCompletion = (run) => {
			const line = completionLine(run);
			try {
				pi.sendMessage?.({ customType: COMPLETION_MESSAGE_TYPE, content: line, display: true, details: run });
			} catch {}
			widget.notify(line, run.state === "completed" ? "info" : "warning");
		};
		const onQuery = (query) => {
			// The child's question becomes a message in the parent conversation: the orchestrator
			// answers it with `subagent_reply`, and no modal ever steals the keyboard.
			try {
				pi.sendMessage?.(
					{
						customType: QUERY_MESSAGE_TYPE,
						content: `Subagent ${query.agent} asks:\nTask ID: ${query.id}\nRequest ID: ${query.requestId}\nQuestion: ${query.message}`,
						display: true,
						details: query,
					},
					{ deliverAs: "followUp", triggerTurn: true },
				);
			} catch {}
		};
		const { tools, registry } = createSubagentToolset({ env: process.env, widget, deliverCompletion, onQuery });
		for (const definition of tools) if (typeof pi.registerTool === "function") pi.registerTool(definition);
		if (typeof pi.registerMessageRenderer === "function") pi.registerMessageRenderer(COMPLETION_MESSAGE_TYPE, createCompletionRenderer());
		const openAgentsPanel = (ctx) => openSubagentPanel(ctx, { registry });
		if (typeof pi.registerCommand === "function") {
			// `/biggz-agents`: the live panel — what each run is doing, its spend, and `s` to stop it.
			pi.registerCommand(AGENTS_COMMAND, {
				description: "Show this session's subagent runs (live activity, spend) and stop one",
				handler: async (_args, ctx) => openAgentsPanel(ctx),
			});
		}
		const shortcut = String(process.env.BIGGZ_AGENTS_KEY ?? AGENTS_SHORTCUT).trim();
		if (shortcut && !/^(off|none|disabled)$/i.test(shortcut) && typeof pi.registerShortcut === "function") {
			try {
				pi.registerShortcut(shortcut, { description: "Show the subagent runs panel", handler: async (ctx) => openAgentsPanel(ctx) });
			} catch (err) {
				try {
					console.warn?.(`biggz-subagent-runtime: shortcut "${bounded(shortcut, 24)}" rejected: ${bounded(err?.message ?? err, 80)}`);
				} catch {}
			}
		}
	} catch (err) {
		try {
			console.warn?.(`biggz-subagent-runtime: registration failed: ${bounded(err?.message ?? err, 120)}`);
		} catch {}
	}
}
