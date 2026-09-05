/**
 * safety.ts — Opencode plugin mirroring gentle-ai.ts:280-720 verbatim.
 * Enforces identical 3 checks (IsDenied, ClassifyGuardedCommand, EvaluateSensitivePathTool)
 * at the opencode surface. Denied→block, guarded per mode, sensitive→block.
 * Follows model-variants.ts Plugin pattern.
 */
import type { Plugin } from "@opencode-ai/plugin"

const GIT_GLOBAL_FLAGS_SRC = String.raw`(?:\s+--?\S+(?:\s+[^-\\s]\S*)?)* `
const GIT_PUSH_RE = new RegExp(String.raw`\bgit${GIT_GLOBAL_FLAGS_SRC}push\b`)

const DENIED_BASH_PATTERNS: RegExp[] = [
  /\brm\s+-rf\s+(?:\/(?:\s|$)|~(?:\/|\s|$)|[$]HOME(?:\/|\s|$)|\.\.?(?:\s|$))/,
  /\bgit\s+reset\s+--hard\b/,
  /\bgit\s+clean\b(?=[^\n]*(?:-[^\n]*f|--force))(?=[^\n]*(?:-[^\n]*d|--directories))/,
  new RegExp(String.raw`\bgit${GIT_GLOBAL_FLAGS_SRC}push\b(?=[^\n]*\s--force(?:-with-lease)?\b)`),
  new RegExp(String.raw`\bgit${GIT_GLOBAL_FLAGS_SRC}push\b(?=[^\n]*\s-[^\s-]*f)`),
  /\bchmod\s+-R\s+777\b/,
  /\bchown\s+-R\b/,
]

const GUARDED_KEY_PATTERNS: Record<string, RegExp> = {
  gitPush: GIT_PUSH_RE,
  gitRebase: /\bgit\s+rebase\b/,
  gitBranchDeleteForce:
    /\bgit\s+branch\s+(?:-[a-zA-Z]*D[a-zA-Z]*|-[a-zA-Z]*d[a-zA-Z]*f[a-zA-Z]*|-[a-zA-Z]*f[a-zA-Z]*d[a-zA-Z]*|--delete\b[^\n]*--force\b|--force\b[^\n]*--delete\b)/,
  npmPublish: /\bnpm\s+publish\b/,
  piRemove: /\bpi\s+remove\b/,
}

const AUTONOMOUS_DEFAULT_ACTIONS: Record<string, string> = {
  gitPush: "allow",
  gitRebase: "confirm",
  gitBranchDeleteForce: "confirm",
  npmPublish: "block",
  piRemove: "confirm",
}

const PATH_GUARDED_TOOLS = new Set(["read", "write", "edit"])
const PATH_INPUT_KEYS = new Set(["path", "paths", "file", "files", "filePath", "filePaths"])
const SENSITIVE_PATH_PATTERNS: RegExp[] = [
  /(^|\/)\.ssh(?:\/|$)/,
  /(^|\/)\.credentials(?:\/|$)/,
  /(^|\/)library\/keychains(?:\/|$)/,
  /(^|\/)\.aws\/credentials$/,
  /(^|\/)\.config\/gh\/hosts\.ya?ml$/,
  /(^|\/)secrets(?:\/|$)/,
  /(^|\/)\.env(?:$|[./_-])/,
  /\.(?:pem|key|p12|pfx)$/,
]

function isDenied(cmd: string): boolean {
  for (const p of DENIED_BASH_PATTERNS) if (p.test(cmd)) return true
  return false
}

function classifyGuardedCommand(
  command: string,
  cfg: { autonomousMode: boolean; guardedCommands: Record<string, string> }
): string {
  for (const p of DENIED_BASH_PATTERNS) if (p.test(command)) return "block"
  for (const [k, pat] of Object.entries(GUARDED_KEY_PATTERNS)) {
    if (!pat.test(command)) continue
    if (!cfg.autonomousMode) return "confirm"
    return cfg.guardedCommands[k] ?? AUTONOMOUS_DEFAULT_ACTIONS[k] ?? "confirm"
  }
  return "not-guarded"
}

function normalizePolicyPath(value: string): string {
  let n = String(value).trim().replace(/\\/g, "/").toLowerCase()
  const home = (typeof process !== "undefined" && (process as any).env?.HOME) || ""
  n = n.replace(/^~(?=\/)/, home).replace(/^~/, home)
  return n
}

function isSensitivePath(value: string): boolean {
  const n = normalizePolicyPath(value)
  return SENSITIVE_PATH_PATTERNS.some((p) => p.test(n))
}

function collectPathInputs(value: unknown, key?: string): string[] {
  if (typeof value === "string") return key && PATH_INPUT_KEYS.has(key) ? [value] : []
  if (Array.isArray(value)) return (value as unknown[]).flatMap((x) => collectPathInputs(x, key))
  if (value && typeof value === "object")
    return Object.entries(value as Record<string, unknown>).flatMap(([k, v]) => collectPathInputs(v, k))
  return []
}

function evaluateSensitivePathTool(
  toolName: string,
  input: unknown
): { block: boolean; reason: string } | undefined {
  if (!PATH_GUARDED_TOOLS.has(toolName)) return undefined
  const p = collectPathInputs(input).find(isSensitivePath)
  if (!p) return undefined
  return { block: true, reason: `Gentle AI safety policy blocked access to sensitive path: ${p}` }
}

export const SafetyPlugin: Plugin = async () => {
  return {
    // opencode hook: intercept tool calls before execution
    "tool.execute.before": async (input: any, output: any) => {
      const tool = input?.tool ?? input?.name ?? ""
      const args = input?.args ?? input?.params ?? {}
      const cmd: string = typeof args?.command === "string" ? args.command : ""
      if (cmd && isDenied(cmd)) {
        console.error(`[safety] blocked denied command opencode surface=opencode kind=block cmd=${cmd.slice(0, 80)}`)
        throw new Error("Gentle AI safety policy blocked a destructive shell command.")
      }
      if (PATH_GUARDED_TOOLS.has(tool)) {
        const sens = evaluateSensitivePathTool(tool, args)
        if (sens?.block) {
          console.error(`[safety] blocked sensitive path opencode surface=opencode kind=block ${sens.reason}`)
          throw new Error(sens.reason)
        }
      }
      if (cmd) {
        const cfg = { autonomousMode: process.env.GENTLE_PI_AUTONOMOUS_MODE === "1", guardedCommands: {} as Record<string, string> }
        const cls = classifyGuardedCommand(cmd, cfg)
        if (cls === "block") {
          console.error(`[safety] guarded block opencode surface=opencode kind=block cmd=${cmd.slice(0, 80)}`)
          throw new Error("Gentle AI safety policy blocked guarded command.")
        }
        if (cls === "confirm") console.warn(`[safety] guarded confirm opencode surface=opencode kind=confirm cmd=${cmd.slice(0, 80)}`)
      }
    },
  } as any
}

export default SafetyPlugin
