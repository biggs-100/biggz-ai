// Ported from gentle-pi 3.7.0 lib/review-sidebar-state.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged apart from the
// adaptation declared here, made by task 2.11.
//
// Upstream owns the review tool definitions, so its publisher wraps them
// (`publisher.tool(definition)`) and reads each result as it passes through.
// This port does not own them: the review tools belong to the harness. Pi hands
// an extension the same information through two handlers instead, so the
// publisher is driven by observation and keeps upstream's resolution logic
// unchanged:
//   `tool_call`   carries toolCallId, toolName and the input, which is where
//                 upstream reads the operation and the correlation it holds;
//   `tool_result` carries the same input plus the result's `details`.
// Later calls supersede earlier ones exactly as upstream's generation counter
// does. One fidelity limit: upstream distinguishes a cancellation from a
// failure by inspecting the thrown error, which an observer does not receive, so
// both land on `unavailable` here instead of `unknown` for a cancellation.
import { sanitizeTerminalText } from "./terminal-theme.ts";

/** Ephemeral display data only; this event grants no review authority. */
export const REVIEW_SIDEBAR_EVENT = "gentle-ai:review-sidebar";
export const REVIEW_SCOPE_UNAVAILABLE = "Candidate scope unavailable";

export const REVIEW_SIDEBAR_LABELS = {
	reviewing: "Reviewing",
	in_review: "In review",
	checking: "Checking review",
	approved: "Approved · awaiting acknowledgement",
	closed: "Closed",
	correction: "Correction required",
	declined: "Declined",
	invalidated: "Invalidated",
	unavailable: "Unavailable",
	unknown: "Unknown",
	ready: "Ready for review",
	consent: "Awaiting consent",
	forecast: "Awaiting reviewer run",
} as const;

export interface ReviewSidebarSnapshot {
	state: keyof typeof REVIEW_SIDEBAR_LABELS;
	scope: string;
}

const record = (value: unknown): Record<string, unknown> =>
	value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};

function candidateScope(paths: unknown): string {
	if (!Array.isArray(paths) || paths.length === 0 || !paths.every((path) => typeof path === "string" && path.length > 0)) return REVIEW_SCOPE_UNAVAILABLE;
	const unique = [...new Set(paths as string[])];
	const first = sanitizeTerminalText(unique[0]!.replaceAll("\\", "/").split("/").pop() ?? "").trim();
	if (!first || first === "." || first === "..") return REVIEW_SCOPE_UNAVAILABLE;
	return `${first}${unique.length > 1 ? ` +${unique.length - 1}` : ""}`;
}

/** Interpret only the facade's explicit evidence, not tool success or opaque bindings. */
export function reviewSidebarSnapshot(operation: string, details: unknown): ReviewSidebarSnapshot {
	const data = record(details);
	const result = record(data.result);
	const closure = record(data.closure);
	const transition = record(result.next_transition);
	const scope = candidateScope(record(result.projection).paths ?? record(data.actor_binding).candidate_paths);
	const snapshot = (state: ReviewSidebarSnapshot["state"]): ReviewSidebarSnapshot => ({ state, scope });
	if (operation === "acknowledge-approved" && data.operation === operation &&
		data.outcome === "native-approved-acknowledgement-completed" &&
		data.status === "closed" && data.authority === "burned") return snapshot("closed");
	if (data.outcome === "consent-declined-this-candidate") return snapshot("declined");
	if (data.outcome === "native-review-consent-required") return snapshot("consent");
	if (data.native_failure || data.failure || data.reconciliation_failure) return snapshot("unavailable");
	if (data.outcome === "reviewer-model-run-forecast") return snapshot("forecast");
	// A terminal capture reports status=closed even when acknowledgement is still
	// pending. Its explicit closure state, not that wrapper status, is evidence.
	const capture = data.outcome === "native-last-event-closure" && closure.schema === "gentle-ai.review-last-event-closure/v1";
	const status = typeof result.schema === "string" && /^gentle-ai\.review-integration\.status\/v[3-9]$/.test(result.schema);
	// An inspect/status call can finish host-mediated consent and return an
	// answer-consent result. Interpret the returned operation, not the input.
	const start = ["start", "answer-consent", "select-intended-untracked"].includes(String(data.operation)) &&
		(result.action === "created" || result.action === "resumed" || result.action === "replayed");
	if (!capture && !status && !start) return snapshot(data.status === "blocked" ? "unavailable" : "unknown");
	const state = capture ? closure.state : status ? record(result.authority).state : result.state;
	if (state === "invalidated") return snapshot("invalidated");
	if (state === "correction_required") return snapshot("correction");
	if (status && (result.action === "recover" || transition.kind === "stop")) return snapshot("unavailable");
	if (status && result.action === "start") return snapshot("ready");
	if (state === "approved") return snapshot("approved");
	// Completed native results show lifecycle state, not active capture execution.
	if (state === "reviewing" || state === "validating") return snapshot("in_review");
	return snapshot("unknown");
}

export function isReviewSidebarSnapshot(value: unknown): value is ReviewSidebarSnapshot {
	if (!value || typeof value !== "object") return false;
	const snapshot = value as Partial<ReviewSidebarSnapshot>;
	return typeof snapshot.state === "string" && Object.hasOwn(REVIEW_SIDEBAR_LABELS, snapshot.state)
		&& typeof snapshot.scope === "string" && snapshot.scope.length > 0;
}

// ---------------------------------------------------------------------------
// The publisher: local display correlation only, never review authority.
// ---------------------------------------------------------------------------

/** Correlation the publisher keeps between calls; nothing here is authority. */
interface ReviewScope {
	workspace: string;
	lineage: string;
	target: string;
	scope: string;
	bindings: string[];
}

function issuedBindings(data: Record<string, unknown>): string[] {
	return Array.isArray(data.collectBindings)
		? data.collectBindings.map((entry) => record(entry).collectBinding).filter((binding): binding is string => typeof binding === "string")
		: [];
}

function matchesCapture(name: string, input: Record<string, unknown>, prior: ReviewScope | undefined): boolean {
	if (!prior || input.lineageId !== prior.lineage) return false;
	if (name === "gentle_review_capture") return typeof input.collectBinding === "string" && prior.bindings.includes(input.collectBinding);
	return name === "gentle_review_capture_group" && Array.isArray(input.collectBindings) &&
		prior.bindings.length > 0 && input.collectBindings.length === prior.bindings.length &&
		input.collectBindings.every((binding, index) => binding === prior.bindings[index]);
}

// This pathless single-capture envelope is not a status or terminal closure.
// It can describe idle continuity only with the publisher's issued correlation.
function isNonterminalReviewerCapture(data: Record<string, unknown>): boolean {
	const relay = record(data.host_relay);
	return data.tool === "gentle_review_capture" && data.status === "captured" &&
		data.outcome === "native-reviewer-result-captured" &&
		typeof data.lineage_id === "string" && data.lineage_id.length > 0 &&
		data.result === undefined && data.closure === undefined &&
		data.native_failure === undefined && data.failure === undefined && data.reconciliation_failure === undefined &&
		relay.transport === "pi_host_relay" &&
		[relay.prompt_bytes, relay.result_bytes].every((value) => typeof value === "number" && Number.isSafeInteger(value) && value >= 0) &&
		typeof relay.submission === "string" && relay.submission.length > 0 &&
		[relay.lens, relay.order, relay.subject_hash, relay.role].every((value) => value === undefined || (typeof value === "string" && value.length > 0));
}

// Resolve completion using only local display correlation; never interpret bindings.
function resolveCompletion(operation: string, name: string, input: Record<string, unknown>, details: unknown,
	prior: ReviewScope | undefined, boundCapture: boolean, workspace: string): { snapshot: ReviewSidebarSnapshot; scope?: ReviewScope } {
	const data = record(details);
	const native = record(data.result);
	const closure = record(data.closure);
	const snapshot = reviewSidebarSnapshot(operation, data);
	const lineage = record(native.authority).lineage_id ?? native.lineage_id ?? data.lineage_id ?? closure.lineage_id;
	const target = native.target_identity ?? data.target_identity ?? closure.target_identity;
	const terminalClosure = data.outcome === "native-last-event-closure";
	const closureMatches = !terminalClosure || (closure.schema === "gentle-ai.review-last-event-closure/v1" &&
		closure.lineage_id === prior?.lineage && closure.target_identity === prior?.target);
	const sameCapture = boundCapture && closureMatches &&
		(lineage === undefined || lineage === prior!.lineage) && (target === undefined || target === prior!.target);
	const nonterminalSingle = name === "gentle_review_capture" && sameCapture && isNonterminalReviewerCapture(data) &&
		data.lineage_id === prior!.lineage && (data.target_identity === undefined || data.target_identity === prior!.target);
	if (nonterminalSingle) {
		snapshot.state = "in_review";
		snapshot.scope = prior!.scope;
	}
	const healthy = !["unknown", "unavailable", "invalidated", "declined"].includes(snapshot.state);
	const sameAcknowledgement = operation === "acknowledge-approved" && snapshot.state === "closed" &&
		prior !== undefined && input.lineageId === prior.lineage && lineage === prior.lineage && target === prior.target;
	let nextScope: ReviewScope | undefined;
	if (healthy && (sameCapture || sameAcknowledgement)) {
		if (snapshot.scope === REVIEW_SCOPE_UNAVAILABLE) snapshot.scope = prior!.scope;
		nextScope = { ...prior!, scope: snapshot.scope, bindings: snapshot.state === "forecast" ? prior!.bindings :
			nonterminalSingle ? prior!.bindings.filter((binding) => binding !== input.collectBinding) : [] };
	}
	// A fresh native projection replaces correlation, even for the same lineage.
	if (healthy && native.applicability === "current_target" && snapshot.scope !== REVIEW_SCOPE_UNAVAILABLE && typeof lineage === "string" && lineage && typeof target === "string" && target && native.projection) {
		nextScope = { workspace, lineage, target, scope: snapshot.scope, bindings: issuedBindings(data) };
	}
	return { snapshot, scope: nextScope };
}

interface PendingObservation {
	ticket: number;
	input: Record<string, unknown>;
	prior: ReviewScope | undefined;
	boundCapture: boolean;
	workspace: string;
	id: string;
}

/** One in-memory snapshot per live runtime; observation cannot affect tool outcomes. */
export function createReviewSidebarPublisher(pi: ExtensionAPI) {
	let active = true;
	let sessionId: string | undefined;
	let generation = 0;
	let scope: ReviewScope | undefined;
	// Upstream reads the input and the result inside its own wrapper. An observer
	// sees them in two events, so the correlation between them is held here, keyed
	// by the tool call pi assigns.
	const pending = new Map<string, PendingObservation>();
	const publish = (id: string, snapshot: ReviewSidebarSnapshot): void => {
		try { pi.events?.emit(REVIEW_SIDEBAR_EVENT, { sessionId: id, snapshot }); } catch { /* Display only. */ }
	};
	return {
		reset(ctx?: ExtensionContext): void {
			generation += 1;
			scope = undefined;
			pending.clear();
			active = ctx !== undefined;
			sessionId = ctx?.sessionManager?.getSessionId();
		},
		/** The review tool is about to run: announce the operation, keep its correlation. */
		begin(toolCallId: string, name: string, input: unknown, ctx: ExtensionContext): void {
			const params = record(input);
			const operation = String(params.operation ?? name);
			if (operation === "assess") return;
			const id = ctx.sessionManager?.getSessionId();
			if (!active || !id || (sessionId !== undefined && sessionId !== id)) return;
			sessionId = id;
			const ticket = ++generation;
			const workspace = typeof params.workspaceRoot === "string" ? params.workspaceRoot : ctx.cwd;
			// A raw nested workspaceRoot can differ from canonical ctx.cwd: lose display
			// correlation rather than canonicalizing paths or inferring authority.
			const prior = scope?.workspace === workspace ? scope : undefined;
			const boundCapture = matchesCapture(name, params, prior);
			// Unbound refreshes and new candidates discard the previous display.
			scope = undefined;
			pending.set(toolCallId, { ticket, input: params, prior, boundCapture, workspace, id });
			publish(id, { state: boundCapture ? "reviewing" : "checking", scope: boundCapture ? prior!.scope : REVIEW_SCOPE_UNAVAILABLE });
		},
		/** The review tool finished: resolve the display state, or report the failure. */
		settle(toolCallId: string, name: string, details: unknown, ctx: ExtensionContext, failed = false): void {
			const observation = pending.get(toolCallId);
			if (!observation) return;
			pending.delete(toolCallId);
			const id = ctx.sessionManager?.getSessionId();
			// A superseded call, a reset or another session keeps its snapshot to itself.
			if (!(active && generation === observation.ticket && sessionId === observation.id && id === observation.id)) return;
			if (failed) {
				scope = undefined;
				publish(observation.id, { state: "unavailable", scope: REVIEW_SCOPE_UNAVAILABLE });
				return;
			}
			const operation = String(observation.input.operation ?? name);
			const resolved = resolveCompletion(operation, name, observation.input, details, observation.prior, observation.boundCapture, observation.workspace);
			scope = resolved.scope;
			publish(observation.id, resolved.snapshot);
		},
	};
}
