/**
 * biggz-memory-chrome — Pi memory tool chrome for BigMem (biggz-mcp).
 *
 * Port of engram's plugin/pi/memory-tool-chrome.js (164 LOC) with
 * adaptations for biggz-ai's 22-tool BigMem surface and biggz_mem_* prefix.
 *
 * Pi's MCP renderer collapses tool calls to a single line (Ctrl+O expands).
 * Without chrome the collapsed line shows only "biggz_mem_save" + hidden JSON.
 * This extension provides compact human labels and status so the stream
 * shows rendered saved text: "🧠 save \"title\" … → ✓ saved #id" collapsed,
 * and the full content when expanded.
 *
 * Pure UI — no DB semantics. Handles both textResult and jsonResult via
 * firstTextContent + resultData duality.
 */

const TOOL_LABELS = {
  mem_save: "save",
  mem_search: "search",
  mem_get_observation: "get observation",
  mem_get: "get observation",
  mem_update: "update",
  mem_delete: "delete",
  mem_context: "context",
  mem_session_summary: "session summary",
  mem_session_start: "start session",
  mem_session_end: "end session",
  mem_save_prompt: "save prompt",
  mem_current_project: "current project",
  mem_suggest_topic_key: "suggest topic",
  mem_timeline: "timeline",
  mem_stats: "stats",
  mem_pin: "pin",
  mem_unpin: "unpin",
  mem_doctor: "doctor",
  mem_compare: "compare",
  mem_judge: "judge",
  mem_capture_passive: "capture passive",
  mem_merge_projects: "merge projects",
  mem_review: "review",
};

// BigMem keys coverage: title, content, type, topic_key, project, scope, session_id, tool_name, pinned
const _BIGGZ_KEYS = ["title", "content", "type", "topic_key", "project", "scope", "session_id", "tool_name", "pinned"];

const ARG_KEYS = {
  mem_save: ["title", "type"],
  mem_search: ["query"],
  mem_get_observation: ["id"],
  mem_get: ["id"],
  mem_update: ["id", "title"],
  mem_delete: ["id"],
  mem_context: ["project", "scope"],
  mem_session_summary: ["content"],
  mem_session_start: ["id"],
  mem_session_end: ["id"],
  mem_save_prompt: ["content"],
  mem_current_project: ["cwd"],
  mem_suggest_topic_key: ["title", "type"],
  mem_timeline: ["observation_id"],
  mem_stats: ["project"],
  mem_pin: ["id"],
  mem_unpin: ["id"],
  mem_doctor: ["check", "project"],
  mem_compare: ["memory_id_a", "memory_id_b"],
  mem_judge: ["judgment_id", "relation"],
  mem_capture_passive: ["source", "content"],
  mem_merge_projects: ["source_project", "target_project"],
  mem_review: ["action", "project", "limit", "observation_id", "id"],
};

export const SUPPORTED_MEMORY_TOOLS = Object.freeze(Object.keys(TOOL_LABELS));

function normalizeToolName(toolName) {
  // Strip biggz_ prefix so biggz_mem_save → mem_save maps to same label
  const base = String(toolName ?? "").replace(/^biggz_/, "");
  // mem_get is alias for mem_get_observation
  return base;
}

export function humanToolName(toolName) {
  const base = normalizeToolName(toolName);
  return TOOL_LABELS[base] ?? base.replace(/^mem_/, "").replace(/_/g, " ");
}

export function truncateText(value, max = 48) {
  const text = String(value ?? "").replace(/\s+/g, " ").trim();
  if (text.length <= max) return text;
  return `${text.slice(0, Math.max(0, max - 1))}…`;
}

function quote(value) {
  const text = truncateText(value);
  return text ? `“${text}”` : "";
}

export function compactToolArg(toolName, args = {}) {
  const base = normalizeToolName(toolName);
  if (base === "mem_review") return compactReviewArg(args);
  const keys = ARG_KEYS[base] ?? [];
  for (const key of keys) {
    const value = args?.[key];
    if (value === undefined || value === null || value === "") continue;
    if (key === "id" || key === "observation_id" || key === "memory_id_a" || key === "memory_id_b") return `#${value}`;
    if (key === "source_project" || key === "target_project") return quote(value);
    return quote(value);
  }
  return "";
}

function compactReviewArg(args = {}) {
  const parts = [];
  if (args.action !== undefined && args.action !== null && args.action !== "") parts.push(String(args.action));
  const id = args.observation_id ?? args.id;
  if (id !== undefined && id !== null && id !== "") parts.push(`#${id}`);
  if (args.project !== undefined && args.project !== null && args.project !== "") parts.push(quote(args.project));
  if (args.limit !== undefined && args.limit !== null && args.limit !== "") parts.push(`limit ${args.limit}`);
  return parts.join(" ");
}

function firstTextContent(result) {
  const block = result?.content?.find?.((entry) => entry?.type === "text" && typeof entry.text === "string");
  return block?.text ?? "";
}

function resultData(result) {
  return result?.details?.data ?? result?.details ?? result;
}

function countItems(value) {
  if (Array.isArray(value)) return value.length;
  if (Array.isArray(value?.results)) return value.results.length;
  if (Array.isArray(value?.observations)) return value.observations.length;
  if (Array.isArray(value?.sessions)) return value.sessions.length;
  if (Array.isArray(value?.prompts)) return value.prompts.length;
  if (typeof value?.count === "number") return value.count;
  return undefined;
}

export function compactResultStatus(toolName, result, options = {}) {
  const base = normalizeToolName(toolName);
  if (options.isPartial) return `${humanToolName(toolName)}…`;
  if (options.isError || result?.isError) {
    const text = truncateText(firstTextContent(result) || result?.details?.error || "error", 64);
    return `✗ ${text}`;
  }
  const data = resultData(result);
  const count = countItems(data);
  if (base === "mem_search") return `✓ ${count ?? 0} result${count === 1 ? "" : "s"}`;
  if (base === "mem_context") return `✓ ${firstTextContent(result) || data?.context ? "loaded" : "empty"}`;
  if (base === "mem_stats") return "✓ loaded";
  if (base === "mem_timeline") return `✓ ${count ?? "timeline"}`;
  if (base === "mem_get_observation" || base === "mem_get") return data?.id ? `✓ observation #${data.id}` : "✓ loaded";
  if (base === "mem_save" || base === "mem_session_summary") return data?.id ? `✓ saved #${data.id}` : firstTextContent(result)?.startsWith("Saved:") ? `✓ ${truncateText(firstTextContent(result), 32)}` : "✓ saved";
  if (base === "mem_update") return data?.id ? `✓ updated #${data.id}` : firstTextContent(result)?.startsWith("Updated:") ? `✓ ${truncateText(firstTextContent(result), 32)}` : "✓ updated";
  if (base === "mem_delete") return data?.id ? `✓ deleted #${data.id}` : "✓ deleted";
  if (base === "mem_suggest_topic_key") return data?.topic_key ? `✓ ${data.topic_key}` : firstTextContent(result) ? `✓ ${truncateText(firstTextContent(result), 32)}` : "✓ suggested";
  if (base === "mem_save_prompt") return data?.id ? `✓ prompt #${data.id}` : firstTextContent(result) ? `✓ ${truncateText(firstTextContent(result), 32)}` : "✓ prompt saved";
  if (base === "mem_session_start") return "✓ started";
  if (base === "mem_session_end") return "✓ ended";
  if (base === "mem_current_project") return data?.project ? `✓ ${data.project}` : "✓ detected";
  if (base === "mem_doctor") return data?.status ? `✓ ${data.status}` : "✓ checked";
  if (base === "mem_capture_passive") return `✓ captured ${data?.saved ?? count ?? 0}`;
  if (base === "mem_judge") return data?.relation?.sync_id ? `✓ judged ${data.relation.sync_id}` : data?.sync_id ? `✓ judged ${data.sync_id}` : "✓ judged";
  if (base === "mem_compare") return data?.sync_id ? `✓ ${data.sync_id}` : "✓ compared";
  if (base === "mem_merge_projects") return firstTextContent(result)?.startsWith("Merged") ? `✓ ${truncateText(firstTextContent(result), 40)}` : "✓ merged";
  if (base === "mem_pin") return "✓ pinned";
  if (base === "mem_unpin") return "✓ unpinned";
  if (base === "mem_review") {
    if (count !== undefined) return `✓ ${count} need${count === 1 ? "s" : ""} review`;
    const id = data?.id ?? data?.observation_id ?? data?.observation?.id;
    return id ? `✓ reviewed #${id}` : "✓ reviewed";
  }
  return "✓ done";
}

export function renderCallText(toolName, args = {}) {
  const arg = compactToolArg(toolName, args);
  return `🧠 ${humanToolName(toolName)}${arg ? ` ${arg}` : ""} …`;
}

export function renderResultText(toolName, result, options = {}) {
  const status = compactResultStatus(toolName, result, options);
  if (!options.expanded || options.isPartial) return `↳ ${status}`;
  const text = firstTextContent(result);
  if (text) return `↳ ${status}\n\n${text}`;
  const data = resultData(result);
  // For json results (search etc.), show JSON when expanded
  const json = (() => {
    try { return JSON.stringify(data, null, 2); } catch { return String(data); }
  })();
  return `↳ ${status}\n\n${truncateText(json, 2000)}`;
}

// Pi extension wrapper — lightweight, mirrors biggz-thinking-wrap pattern.
// Registers no new tools (BigMem tools are MCP via biggz-mcp), but hooks
// tool events to show compact status in the TUI status bar and to provide
// rendering helpers for any future native re-registration.
/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzMemoryChrome(pi) {
  if (process.env.PI_SUBAGENT_CHILD === "1") return;
  // Normalize check for MCP prefix: pi may emit "biggz_mem_save" or "mem_save"
  const isMemoryTool = (name) => {
    const base = normalizeToolName(name);
    return SUPPORTED_MEMORY_TOOLS.includes(base);
  };

  // Status bar feedback mirrors engram's memory-tool-chrome usage:
  // on call show "🧠 save “title” …", on result show "✓ saved #id"
  if (typeof pi.on === "function") {
    try {
      pi.on("tool_call", async (event, ctx) => {
        const name = event?.toolName ?? event?.name ?? "";
        if (!isMemoryTool(name)) return;
        const args = event?.args ?? event?.params ?? {};
        try {
          ctx?.ui?.setStatus?.("biggz-memory", renderCallText(name, args));
        } catch {}
      });
      pi.on("tool_result", async (event, ctx) => {
        const name = event?.toolName ?? event?.name ?? "";
        if (!isMemoryTool(name)) return;
        const result = event?.result ?? event;
        try {
          const status = compactResultStatus(name, result, { isError: event?.isError });
          ctx?.ui?.setStatus?.("biggz-memory", `🧠 ${status}`);
          setTimeout(() => ctx?.ui?.setStatus?.("biggz-memory", undefined), 3000);
        } catch {}
      });
    } catch {}
  }
}
