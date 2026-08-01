import type { Plugin } from "@opencode-ai/plugin"
import { spawn } from "node:child_process"
import { randomBytes } from "node:crypto"
import { mkdirSync, writeFileSync } from "node:fs"
import { join } from "node:path"

// biggz-ai review transport plugin.
//
// Port of gentle-ai's review-result-artifacts plugin adapted to the biggz
// native CLI. It owns the transport between the orchestrator, the reviewer
// sub-agent, and `biggz review capture-result`:
//
//   - tool.execute.before: validates the GENTLE_AI_REVIEW_BINDING literal,
//     rejects background tasks, runs `capture-result --preflight` to obtain
//     the artifact subject (base/candidate trees + ordered changed-path
//     manifest), injects it under GENTLE_AI_REVIEW_CONTEXT, and discards the
//     caller-authored task body — the reviewer receives only provider-owned
//     context and the exact instruction shape from the orchestrator's collect
//     input.
//   - tool.execute.after: extracts strict JSON from the task result (rejecting
//     empty and nested envelopes), runs `capture-result --input -` with the
//     binding flags, and replaces the task output with the captured artifact.
//   - Failure path: biggz has no `preserve-result` CLI verb, so the raw
//     payload is quarantined to a durable local file
//     .git/biggz/preserved-results/<lineage>-<lens>-<order>-<ts>.json
//     (exclusive write, never overwrite) and only safe typed diagnostics are
//     forwarded — never payload contents.
//
// GENTLE_AI_REVIEW_BINDING is adopted verbatim from gentle-ai: it is the
// de-facto standard literal across both projects, so a binding authored for
// one transport is recognizable in the other.

const REVIEW_AGENTS = new Set(["review-risk", "review-readability", "review-reliability", "review-resilience"])
const BINDING = /^GENTLE_AI_REVIEW_BINDING (\{[^\n]+\})(?:\n|$)/
const TASK_RESULT = /^<task id="[^"\r\n]+" state="completed">\n<task_result>\n([\s\S]*?)\n<\/task_result>\n<\/task>$/
const TASK_TAG = /<\/?task(?:\s|>)|<\/?task_result>/

const GIT_COMMIT_SHA = /^[a-f0-9]{40}(?:[a-f0-9]{24})?$/
const SHA256_HEX = /^[a-f0-9]{64}$/
const SHA256_IDENTITY = /^sha256:[a-f0-9]{64}$/

// Biggz specifics vs gentle-ai:
//   - target is the reviewed git commit SHA (no sha256: prefix)
//   - revision (expected-revision) is a bare 64-hex event revision (no
//     sha256: prefix)
//   - repository_context is a provider-issued JSON object string
//   - subject_hash is a sha256: identity
type ReviewBinding = {
  lineage: string
  target: string
  lens: string
  order: number
  revision: string
  repository_context?: string
  subject_hash?: string
}

interface ReviewArtifactSubject {
  schema: string
  subject_hash: string
  lineage_id: string
  authority_revision: string
  target_identity: string
  base_tree: string
  candidate_tree: string
  changed_path_manifest_sha256: string
  lens: string
  selected_order: number
}

interface ChangedPathManifestEntry {
  path: string
  status: string
  old_mode: string
  new_mode: string
  deleted: boolean
}

interface ReviewCapturePreflight {
  schema: string
  lineage_id: string
  target_identity: string
  lens: string
  selected_order: number
  expected_revision: string
  subject: ReviewArtifactSubject
  base_tree: string
  candidate_tree: string
  changed_path_manifest_sha256: string
  changed_path_manifest: ChangedPathManifestEntry[]
}

function parseBinding(prompt: unknown, lens: string): ReviewBinding {
  const match = BINDING.exec(typeof prompt === "string" ? prompt : "")
  if (!match) throw new Error("review task is missing GENTLE_AI_REVIEW_BINDING")

  let binding: unknown
  try {
    binding = JSON.parse(match[1])
  } catch {
    throw new Error("review task binding is malformed")
  }
  if (!binding || typeof binding !== "object" || Array.isArray(binding)) {
    throw new Error("review task binding must be an object")
  }
  const value = binding as Record<string, unknown>
  const fields = Object.keys(value).sort().join(",")
  const minimal = fields === "lens,lineage,order,revision,target"
  const withContext = fields === "lens,lineage,order,revision,repository_context,target"
  const withSubject = fields === "lens,lineage,order,revision,subject_hash,target"
  const current = fields === "lens,lineage,order,revision,repository_context,subject_hash,target"
  const validContext = (candidate: unknown) => {
    if (typeof candidate !== "string" || candidate === "") return false
    let parsed: unknown
    try {
      parsed = JSON.parse(candidate)
    } catch {
      return false
    }
    return Boolean(parsed && typeof parsed === "object" && !Array.isArray(parsed))
  }
  if ((!minimal && !withContext && !withSubject && !current) ||
      typeof value.lineage !== "string" || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(value.lineage) ||
      typeof value.target !== "string" || !GIT_COMMIT_SHA.test(value.target) ||
      ((withContext || current) && !validContext(value.repository_context)) ||
      typeof value.revision !== "string" || !SHA256_HEX.test(value.revision) ||
      ((withSubject || current) && (typeof value.subject_hash !== "string" || !SHA256_IDENTITY.test(value.subject_hash))) ||
      value.lens !== lens || !Number.isSafeInteger(value.order) || (value.order as number) < 0) {
    throw new Error("review task binding does not match the selected lens")
  }
  return value as ReviewBinding
}

function reviewerResult(output: unknown): string {
  if (typeof output !== "string" || output.trim() === "") throw new Error("reviewer output must not be empty")
  const trimmed = output.trim()
  const envelope = TASK_RESULT.exec(trimmed)
  if (!envelope) {
    if (TASK_TAG.test(trimmed)) throw new Error("reviewer output contains a malformed task result envelope")
    return trimmed
  }
  if (envelope[1].trim() === "") {
    throw Object.assign(new Error("reviewer task result is empty"), { reviewClass: "empty_result" })
  }
  if (TASK_TAG.test(envelope[1])) {
    throw Object.assign(new Error("reviewer task result contains a nested task envelope"), { reviewClass: "nested_envelope" })
  }
  return envelope[1]
}

function extractionClass(cause: unknown): string | undefined {
  const value = (cause as { reviewClass?: unknown } | null)?.reviewClass
  return typeof value === "string" ? value : undefined
}

function runNative(cwd: string, args: string[], stdin: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn("biggz", args, { cwd, stdio: ["pipe", "pipe", "pipe"] })
    const stdout: Buffer[] = []
    const stderr: Buffer[] = []
    child.stdout.on("data", (chunk: Buffer) => stdout.push(chunk))
    child.stderr.on("data", (chunk: Buffer) => stderr.push(chunk))
    child.stdin.on("error", reject)
    child.on("error", reject)
    child.on("close", (code) => {
      if (code === 0) {
        resolve(Buffer.concat(stdout).toString("utf8").trim())
        return
      }
      reject(new Error(`biggz ${args[0]} ${args[1]} failed (${code ?? "signal"}): ${Buffer.concat(stderr).toString("utf8").trim()}`))
    })
    child.stdin.end(stdin)
  })
}

function captureArgs(binding: ReviewBinding, preflight: boolean): string[] {
  const args = [
    "review", "capture-result",
    "--lineage", binding.lineage, "--target", binding.target,
    "--lens", binding.lens, "--order", String(binding.order),
    "--expected-revision", binding.revision,
  ]
  if (binding.repository_context) args.push("--repository-context", binding.repository_context)
  if (binding.subject_hash) args.push("--subject-hash", binding.subject_hash)
  if (preflight) args.push("--preflight")
  else args.push("--input", "-")
  return args
}

async function preflightCapture(cwd: string, binding: ReviewBinding): Promise<ReviewCapturePreflight> {
  try {
    const response = await runNative(cwd, captureArgs(binding, true), "")
    let parsed: unknown
    try {
      parsed = JSON.parse(response)
    } catch {
      throw new Error("review capture preflight returned malformed artifact-subject JSON")
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("review capture preflight returned malformed artifact-subject JSON")
    }
    const value = parsed as Record<string, unknown>
    const subject = value.subject as Record<string, unknown> | undefined
    if (!subject || subject.schema !== "biggz-ai.review-artifact-subject/v1" ||
        typeof subject.subject_hash !== "string" || !SHA256_IDENTITY.test(subject.subject_hash) ||
        typeof subject.authority_revision !== "string" || !SHA256_HEX.test(subject.authority_revision) ||
        typeof subject.target_identity !== "string" || !GIT_COMMIT_SHA.test(subject.target_identity) ||
        typeof subject.base_tree !== "string" || !GIT_COMMIT_SHA.test(subject.base_tree) ||
        typeof subject.candidate_tree !== "string" || !GIT_COMMIT_SHA.test(subject.candidate_tree) ||
        typeof subject.changed_path_manifest_sha256 !== "string" || !SHA256_IDENTITY.test(subject.changed_path_manifest_sha256) ||
        subject.lineage_id !== binding.lineage || subject.target_identity !== binding.target ||
        subject.authority_revision !== binding.revision ||
        subject.lens !== binding.lens || subject.selected_order !== binding.order ||
        value.schema !== "biggz-ai.review-capture-preflight/v1" ||
        value.lineage_id !== binding.lineage || value.target_identity !== binding.target ||
        value.lens !== binding.lens || value.selected_order !== binding.order ||
        value.expected_revision !== binding.revision ||
        value.base_tree !== subject.base_tree || value.candidate_tree !== subject.candidate_tree ||
        value.changed_path_manifest_sha256 !== subject.changed_path_manifest_sha256 ||
        !validManifest(value.changed_path_manifest)) {
      throw new Error("review capture preflight returned an incomplete artifact subject")
    }
    if (binding.subject_hash && subject.subject_hash !== binding.subject_hash) {
      throw new Error("review capture preflight returned a different artifact subject")
    }
    return value as unknown as ReviewCapturePreflight
  } catch (cause) {
    // Preflight runs before any reviewer is launched, so the native message
    // cannot embed reviewer payload content: forward it verbatim.
    const scope = binding.repository_context ? "the provider-issued repository context" : cwd
    throw new Error(
      `review capture preflight failed for lens ${binding.lens} under ${scope}: ` +
      `${errorMessage(cause)}. ` +
      `The reviewer was not launched, so its exactly-once invocation is preserved. ` +
      `Relaunch the lens from the repository that owns lineage ${binding.lineage} ` +
      `(biggz resolves the repository from the working directory).`,
    )
  }
}

function validManifest(value: unknown): value is ChangedPathManifestEntry[] {
  if (!Array.isArray(value)) return false
  let previous = ""
  for (const entry of value) {
    if (!validManifestEntry(entry) ||
        (previous !== "" && Buffer.compare(Buffer.from(previous, "utf8"), Buffer.from(entry.path, "utf8")) >= 0)) return false
    previous = entry.path
  }
  return true
}

function validManifestEntry(entry: unknown): entry is ChangedPathManifestEntry {
  if (!entry || typeof entry !== "object" || Array.isArray(entry)) return false
  const value = entry as Record<string, unknown>
  return Object.keys(value).sort().join(",") === "deleted,new_mode,old_mode,path,status" &&
    typeof value.path === "string" && value.path !== "" &&
    typeof value.status === "string" && /^[ADMT]$/.test(value.status) &&
    typeof value.old_mode === "string" && /^[0-7]{6}$/.test(value.old_mode) &&
    typeof value.new_mode === "string" && /^[0-7]{6}$/.test(value.new_mode) &&
    typeof value.deleted === "boolean"
}

async function injectReviewerContext(prompt: string, lens: string, cwd: string): Promise<string> {
  const binding = parseBinding(prompt, lens)
  const preflight = await preflightCapture(cwd, binding)
  const injectedBinding = { ...binding, subject_hash: preflight.subject.subject_hash }
  return `GENTLE_AI_REVIEW_BINDING ${JSON.stringify(injectedBinding)}\n` +
    `GENTLE_AI_REVIEW_CONTEXT ${JSON.stringify(preflight)}\n`
}

function errorMessage(cause: unknown): string {
  return cause instanceof Error ? cause.message : String(cause)
}

// ADMISSION_REJECTION matches the typed decision the native CLI emits when it
// refused the reviewer result itself (`reviewer artifact admission <decision>:`
// from cmd/biggz/main.go). Only the [a-z_]+ decision token is forwarded — never
// the native diagnostic, which can embed payload-derived text — so no payload
// contents ever reach the session transcript. Re-capturing identical bytes can
// never satisfy admission; only a relaunched reviewer can produce a corrected
// result.
const ADMISSION_REJECTION = /\breviewer artifact admission ([a-z_]+):/

function admissionDecision(cause: unknown): string | undefined {
  const match = ADMISSION_REJECTION.exec(errorMessage(cause))
  return match ? match[1] : undefined
}

// PRESERVE_MAX_ATTEMPTS_PER_SESSION bounds quarantine writes: after the budget
// is exhausted the plugin stops preserving and stops forwarding, surfacing a
// terminal typed error instead.
const PRESERVE_MAX_ATTEMPTS_PER_SESSION = 8

// writePreserved quarantines the raw payload under .git/biggz/preserved-results/
// with an exclusive (wx) write — the timestamped file is never overwritten.
// Returns the repository-relative path for the safe diagnostic reference.
function writePreserved(cwd: string, binding: ReviewBinding, raw: string): string {
  const dir = join(cwd, ".git", "biggz", "preserved-results")
  mkdirSync(dir, { recursive: true })
  const stamp = new Date().toISOString().replace(/[:.]/g, "-")
  const baseName = `${binding.lineage}-${binding.lens}-${binding.order}-${stamp}`
  for (let attempt = 0; attempt < 8; attempt++) {
    const suffix = attempt === 0 ? "" : `-${randomBytes(3).toString("hex")}`
    const fileName = `${baseName}${suffix}.json`
    try {
      writeFileSync(join(dir, fileName), raw, { flag: "wx" })
      return join(".git", "biggz", "preserved-results", fileName)
    } catch (cause) {
      const code = (cause as NodeJS.ErrnoException | null)?.code
      if (code !== "EEXIST" || attempt === 7) throw cause
    }
  }
  throw new Error("preserve failed: could not claim a quarantine file name")
}

async function preservedCaptureFailure(
  cwd: string, binding: ReviewBinding, raw: unknown, cause: unknown,
  preserveAttempts: Map<string, number>, sessionID: string,
): Promise<Error> {
  const attempts = preserveAttempts.get(sessionID) ?? 0
  if (attempts >= PRESERVE_MAX_ATTEMPTS_PER_SESSION) {
    return new Error(
      "reviewer_preserve_budget_exhausted: reviewer result capture failed repeatedly; " +
      "stop relaunching this lens and surface the terminal failure to the maintainer",
    )
  }
  preserveAttempts.set(sessionID, attempts + 1)
  const decision = admissionDecision(cause)
  const classLabel = extractionClass(cause)
  const typed = decision
    ? `reviewer artifact admission ${decision}`
    : classLabel
    ? `reviewer_artifact_extraction_${classLabel}`
    : "reviewer_capture_failed"
  const recovery = decision
    ? "re-capturing identical bytes cannot succeed; relaunch this lens reviewer exactly once to produce a corrected result"
    : classLabel
    ? "the reviewer produced no replayable strict JSON; relaunch this lens reviewer once after inspecting its last run"
    : "provider-owned review operation failed; refresh the exact native status for lineage " +
      binding.lineage + " and relaunch the lens only if the same bound slot is reoffered"
  if (typeof raw !== "string" || raw.trim() === "") {
    return new Error(`${typed}: no raw reviewer result was available to preserve; ${recovery}`)
  }
  try {
    const reference = writePreserved(cwd, binding, raw)
    return new Error(`${typed}: raw reviewer result preserved for manual recovery as ${reference}; ${recovery}`)
  } catch (preserveCause) {
    return new Error(
      `${typed}: raw reviewer result could not be preserved (${errorMessage(preserveCause)}); ` +
      `${recovery}; no payload contents were forwarded`,
    )
  }
}

const ReviewResultArtifactsPlugin: Plugin = async ({ directory, worktree }) => {
  const preserveAttempts: Map<string, number> = new Map()
  const cwd = worktree || directory
  return {
    dispose: async () => { preserveAttempts.clear() },
    event: async ({ event }) => {
      if (event.type === "session.deleted") preserveAttempts.delete(event.properties.info.id)
    },
    "tool.execute.before": async (input, output) => {
      if (input.tool !== "task" || typeof output.args?.subagent_type !== "string" ||
          !REVIEW_AGENTS.has(output.args.subagent_type)) return
      if (typeof output.args.prompt !== "string") {
        throw new Error("review task is missing GENTLE_AI_REVIEW_BINDING")
      }
      if (output.args.background === true) {
        throw new Error("bound review tasks must run in the foreground for native result capture")
      }
      output.args.prompt = await injectReviewerContext(
        output.args.prompt,
        output.args.subagent_type.slice("review-".length),
        cwd,
      )
    },
    "tool.execute.after": async (input, output) => {
      if (input.tool !== "task" || typeof input.args?.subagent_type !== "string" || !REVIEW_AGENTS.has(input.args.subagent_type)) return
      if (typeof input.args.prompt !== "string" || !BINDING.test(input.args.prompt)) return
      const lens = input.args.subagent_type.slice("review-".length)
      const binding = parseBinding(input.args.prompt, lens)
      // Extract the replayable payload exactly once, BEFORE capture: a capture
      // failure must quarantine the extracted strict JSON — never the
      // enveloped output.output.
      let result: string
      try {
        result = reviewerResult(output.output)
      } catch (cause) {
        throw await preservedCaptureFailure(cwd, binding, output.output, cause, preserveAttempts, input.sessionID)
      }
      try {
        output.output = await runNative(cwd, captureArgs(binding, false), result)
        preserveAttempts.delete(input.sessionID)
      } catch (cause) {
        throw await preservedCaptureFailure(cwd, binding, result, cause, preserveAttempts, input.sessionID)
      }
    },
  }
}

export default ReviewResultArtifactsPlugin
