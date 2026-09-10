/**
 * biggz-quiet-tools — quiet collapsed rendering for Pi built-in tools.
 *
 * Port of gentle-pi extensions/quiet-tools.ts collapsed-result strategy, adapted
 * to biggz-ai's string-template chrome style (no @earendil-works/pi-tui import:
 * deployed extensions resolve only relative paths + node: builtins).
 *
 * Strategy (zero behavior risk):
 * - Re-register read/bash/powershell/grep/find/ls/edit/write spreading the
 *   CURRENT definition, so execute/schemas/prompts stay identical. Only
 *   renderResult is replaced.
 * - Collapsed (expanded=false): one-line count summary + max 3 preview lines,
 *   errors show last 3 meaningful lines, empty results render nothing.
 * - Expanded: delegates to the ORIGINAL renderResult (full native fidelity).
 *
 * Pure helpers (quietResultText + counters) are exported for node tests.
 * Opt-out via BIGGZ_QUIET_TOOLS=0 (gentle parity: GENTLE_PI_QUIET_TOOLS).
 * Disabled entirely with BIGGZ_PRETTY=0 or in subagent children.
 */

import { sanitizeTerminalText, semanticJsonPreview } from "./biggz-memory-chrome.js";

export const QUIET_TOOLS = Object.freeze([
  "read",
  "bash",
  "powershell",
  "grep",
  "find",
  "ls",
  "edit",
  "write",
]);

export const QUIET_PREVIEW_LINES = 3;
const GIT_TAIL_LINES = 10;

const EMPTY_RESULT_MESSAGES = Object.freeze({
  grep: ["No matches found"],
  find: ["No files found matching pattern"],
  ls: ["Directory is empty", "(empty directory)"],
  bash: ["(no output)"],
  powershell: ["(no output)"],
});

const COUNT_LABELS = Object.freeze({
  grep: "matches",
  find: "files",
  ls: "entries",
});

export function isQuietEnabled(env = process.env) {
  return env?.BIGGZ_QUIET_TOOLS !== "0";
}

export function extractText(result) {
  const content = result?.content;
  if (!Array.isArray(content)) return "";
  return content
    .filter((entry) => entry?.type === "text" && typeof entry.text === "string")
    .map((entry) => entry.text)
    .join("\n");
}

function outputLines(text) {
  const normalized = String(text ?? "").replace(/\r\n/g, "\n").replace(/\n$/, "");
  return normalized.length > 0 ? normalized.split("\n") : [];
}

export function countNonEmptyLines(text) {
  return outputLines(text).filter((line) => line.trim().length > 0).length;
}

function lastLines(text, limit) {
  const lines = outputLines(text);
  return lines.slice(Math.max(0, lines.length - limit)).join("\n");
}

function lastMeaningfulLines(text, limit) {
  return outputLines(text)
    .filter((line) => line.trim().length > 0)
    .slice(-limit)
    .join("\n");
}

function isEmptyResult(toolName, text) {
  const normalized = String(text ?? "").trim();
  return (EMPTY_RESULT_MESSAGES[toolName] ?? []).some((message) => normalized === message);
}

function grepMatchCount(text, args) {
  const context = args?.context;
  if (typeof context !== "number" || context <= 0) return countNonEmptyLines(text);
  return outputLines(text).filter((line) => /^\s*.+:\d+:\s?/.test(line)).length;
}

function isGitCommand(args) {
  const command = typeof args?.command === "string" ? args.command.trim() : "";
  return /^(?:env\s+\S+=\S+\s+|command\s+|\w+=\S+\s+)*git(?:\s|$)/.test(command);
}

function diffStats(diff) {
  let additions = 0;
  let removals = 0;
  for (const line of String(diff ?? "").split("\n")) {
    if (line.startsWith("+") && !line.startsWith("+++")) additions++;
    if (line.startsWith("-") && !line.startsWith("---")) removals++;
  }
  return { additions, removals };
}

function writeSummary(text) {
  const bytes = String(text ?? "").match(/(?:Successfully )?wrote\s+(\d+)\s+bytes/i)?.[1];
  return bytes ? `wrote ${bytes} bytes` : "written";
}

/**
 * Collapsed quiet summary for a built-in tool result. Returns "" when there
 * is nothing worth showing (empty results). Never truncates data — the
 * expanded view keeps the original renderer.
 */
export function quietResultText(toolName, result, opts = {}) {
  const text = sanitizeTerminalText(extractText(result));
  const args = opts.args ?? {};
  if (opts.isError) {
    return lastMeaningfulLines(text, QUIET_PREVIEW_LINES);
  }
  switch (toolName) {
    case "grep":
    case "find":
    case "ls": {
      if (!text.trim() || isEmptyResult(toolName, text)) return "";
      const count = toolName === "grep" ? grepMatchCount(text, args) : countNonEmptyLines(text);
      if (count <= 0) return "";
      const preview = lastLines(text, QUIET_PREVIEW_LINES);
      return `→ ${count} ${COUNT_LABELS[toolName]}\n${preview}`;
    }
    case "read": {
      if (!text.trim()) return "";
      return outputLines(text).slice(0, QUIET_PREVIEW_LINES).join("\n");
    }
    case "bash":
    case "powershell": {
      if (!text.trim() || isEmptyResult(toolName, text)) return "";
      if (isGitCommand(args)) return lastLines(text, GIT_TAIL_LINES);
      return semanticJsonPreview(text, QUIET_PREVIEW_LINES) ?? lastLines(text, QUIET_PREVIEW_LINES);
    }
    case "edit": {
      const diff = result?.details?.diff;
      if (typeof diff === "string" && diff.length > 0) {
        const stats = diffStats(diff);
        return `✓ +${stats.additions} / -${stats.removals}`;
      }
      return "✓ applied";
    }
    case "write":
      return `✓ ${writeSummary(text)}`;
    default:
      return lastLines(text, QUIET_PREVIEW_LINES);
  }
}

function asComponent(text) {
  const lines = String(text ?? "")
    .split("\n")
    .filter((line, index, all) => !(index === 0 && line === "" && all.length === 1));
  return {
    render() {
      return text === "" ? [] : lines;
    },
    invalidate() {},
  };
}

function currentDefinition(pi, name) {
  try {
    if (typeof pi.getToolDefinition === "function") {
      const def = pi.getToolDefinition(name);
      if (def) return def.definition ?? def;
    }
  } catch {}
  try {
    if (typeof pi.getTool === "function") {
      const tool = pi.getTool(name);
      if (tool) return tool.definition ?? tool;
    }
  } catch {}
  return undefined;
}

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzQuietTools(pi) {
  if (process.env.PI_SUBAGENT_CHILD === "1") return;
  if (process.env.BIGGZ_PRETTY === "0") return;
  if (!isQuietEnabled()) return;
  if (typeof pi.registerTool !== "function") return;
  for (const name of QUIET_TOOLS) {
    let def;
    try {
      def = currentDefinition(pi, name);
    } catch {
      continue;
    }
    if (!def || typeof def.execute !== "function") continue;
    const originalRenderResult = typeof def.renderResult === "function" ? def.renderResult.bind(def) : null;
    const toolName = name;
    try {
      pi.registerTool({
        ...def,
        name: toolName,
        renderResult(result, options = {}, theme, context) {
          if (options.expanded && originalRenderResult) {
            return originalRenderResult(result, options, theme, context);
          }
          const summary = quietResultText(toolName, result, {
            isError: options.isError ?? context?.isError ?? false,
            args: context?.args,
          });
          if (theme && typeof theme.fg === "function" && summary) {
            const color = options.isError || context?.isError ? "error" : "muted";
            return asComponent(summary.split("\n").map((line) => theme.fg(color, line)).join("\n"));
          }
          return asComponent(summary);
        },
      });
    } catch {}
  }
}
