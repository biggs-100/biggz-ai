// Ported from gentle-pi 3.7.0 tests/review-sidebar-state.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged. Unit 2.2a
// narrowed this suite because `shell-bar` did not exist yet; unit 2.3 restores
// the upstream render assertions against the real module.
// pi-tui is supplied by the repository's test oracle (see test/pi-tui-resolver.mjs).
import assert from "node:assert/strict";
import test from "node:test";
import { PI_TUI_SPECIFIER } from "../test/pi-tui-resolver.mjs";
import { createReviewSidebarPublisher, reviewSidebarSnapshot } from "./review-sidebar-state.ts";

const { visibleWidth } = await import(PI_TUI_SPECIFIER);
const { renderShellSidebarBar } = await import("./shell-bar.ts");

// Display contract only: lifecycle evidence must be normalized by the producer,
// never inferred from a successful tool execution by the renderer.
const theme = { fg: (_color, text) => text, bold: (text) => text };
function model(review) {
	return {
		cwd: "/repo",
		branch: "main",
		dirty: undefined,
		sessionName: undefined,
		modelId: "test-model",
		effort: undefined,
		contextPercent: null,
		contextWindow: 0,
		costTotal: 0,
		subscription: false,
		usage: undefined,
		statuses: [],
		review,
	};
}

test("RDD maps explicit native evidence conservatively without parsing opaque bindings", () => {
	const nativeStatus = (state, paths = ["src/app.ts", "test/app.test.ts", "README.md"]) => ({
		result: {
			schema: "gentle-ai.review-integration.status/v9",
			authority: { state }, projection: { paths }, next_transition: { kind: "collect" },
		},
	});
	assert.deepEqual(reviewSidebarSnapshot("status", nativeStatus("reviewing")), { state: "in_review", scope: "app.ts +2" });
	for (const [state, expected] of [["approved", "approved"], ["correction_required", "correction"], ["invalidated", "invalidated"], ["validating", "in_review"]]) {
		assert.equal(reviewSidebarSnapshot("status", nativeStatus(state)).state, expected);
	}
	const closure = { status: "closed", outcome: "native-last-event-closure", closure: { schema: "gentle-ai.review-last-event-closure/v1", state: "approved" } };
	assert.equal(reviewSidebarSnapshot("gentle_review_capture", closure).state, "approved");
	assert.equal(reviewSidebarSnapshot("gentle_review_capture_group", closure).state, "approved");
	assert.equal(reviewSidebarSnapshot("gentle_review_capture_group", { status: "blocked", outcome: "reviewer-model-run-forecast" }).state, "forecast");
	const burned = { operation: "acknowledge-approved", status: "closed", outcome: "native-approved-acknowledgement-completed", authority: "burned" };
	assert.equal(reviewSidebarSnapshot("acknowledge-approved", burned).state, "closed");
	for (const missing of ["operation", "status", "outcome", "authority"]) {
		const incomplete = { ...burned };
		delete incomplete[missing];
		assert.notEqual(reviewSidebarSnapshot("acknowledge-approved", incomplete).state, "closed");
	}
	assert.notEqual(reviewSidebarSnapshot("gentle_review_capture", burned).state, "closed");
	assert.equal(reviewSidebarSnapshot("answer-consent", { outcome: "consent-declined-this-candidate" }).state, "declined");
	assert.equal(reviewSidebarSnapshot("start", { outcome: "native-review-consent-required" }).state, "consent");
	assert.deepEqual(reviewSidebarSnapshot("inspect", {
		operation: "answer-consent", result: { action: "created", state: "reviewing" },
		actor_binding: { candidate_paths: ["lib/app.ts"] },
	}), { state: "in_review", scope: "app.ts" });
	assert.equal(reviewSidebarSnapshot("status", nativeStatus("reviewing", ["src\\app.ts"])).scope, "app.ts");
	assert.equal(reviewSidebarSnapshot("status", { ...nativeStatus("approved"), native_failure: {} }).state, "unavailable");
	assert.equal(reviewSidebarSnapshot("status", { result: { ...nativeStatus("approved").result, next_transition: { kind: "stop" } } }).state, "unavailable");
	for (const paths of [null, [], [null], ["/"]]) {
		assert.equal(reviewSidebarSnapshot("status", nativeStatus("reviewing", paths)).scope, "Candidate scope unavailable");
	}
	assert.deepEqual(reviewSidebarSnapshot("status", { status: "closed", collectBinding: "opaque", title: "Invented title" }), { state: "unknown", scope: "Candidate scope unavailable" });
});


for (const action of ["created", "resumed", "replayed"]) {
	test(`RDD treats ${action} START results as pending review, not active execution`, () => {
		for (const operation of ["start", "answer-consent", "select-intended-untracked"]) {
			for (const state of ["reviewing", "validating"]) {
				assert.deepEqual(reviewSidebarSnapshot(operation, {
					operation, result: { action, state },
					actor_binding: { candidate_paths: ["lib/app.ts"] },
				}), { state: "in_review", scope: "app.ts" });
			}
		}
	});
}

test("RDD completed STATUS evidence does not imply capture execution", () => {
	for (const state of ["reviewing", "validating"]) {
		for (const kind of ["collect", "execute"]) {
			const snapshot = reviewSidebarSnapshot("status", {
				result: {
					schema: "gentle-ai.review-integration.status/v9",
					authority: { state }, projection: { paths: ["lib/app.ts"] },
					next_transition: { kind },
				},
			});
			assert.deepEqual(snapshot, { state: "in_review", scope: "app.ts" });
			const pending = renderShellSidebarBar({ ...model(), review: snapshot }, theme, 46).join("\n");
			assert.match(pending, /In review/);
			assert.doesNotMatch(pending, /Reviewing/);
		}
	}
	// The future publisher owns execution evidence; this slice only renders its
	// explicit display state, never inferring it from a completed native result.
	const active = renderShellSidebarBar(model({ state: "reviewing", scope: "app.ts" }), theme, 46).join("\n");
	assert.match(active, /Reviewing/);
	assert.doesNotMatch(active, /In review/);
});

test("RDD stays hidden without current-session evidence and occupies one group between Changes and Integrations", () => {
	assert.doesNotMatch(renderShellSidebarBar(model(), theme, 46).join("\n"), /RDD/);
	const text = renderShellSidebarBar(model({ state: "reviewing", scope: "app.ts +2" }), theme, 46).join("\n");
	assert.equal((text.match(/RDD/g) ?? []).length, 1);
	assert.ok(text.indexOf("Changes") < text.indexOf("RDD"));
	assert.ok(text.indexOf("RDD") < text.indexOf("Integrations"));
	assert.match(text, /Reviewing/);
	assert.match(text, /app\.ts \+2/);
});

test("RDD distinguishes approval awaiting acknowledgement from confirmed closure and wraps narrow scopes", () => {
	const approved = renderShellSidebarBar(model({ state: "approved", scope: "Candidate scope unavailable" }), theme, 60).join("\n");
	assert.match(approved, /Approved.*awaiting acknowledgement/);
	assert.doesNotMatch(approved, /Closed/);
	const closed = renderShellSidebarBar(model({ state: "closed", scope: "app.ts" }), theme, 46).join("\n");
	assert.match(closed, /Closed/);
	for (const width of [20, 32, 46]) {
		const lines = renderShellSidebarBar(model({ state: "reviewing", scope: "長い候補ファイル-name-with-many-characters.ts +2" }), theme, width);
		assert.ok(lines.every((line) => visibleWidth(line) <= width), `RDD must fit ${width} columns`);
	}
});

// ---------------------------------------------------------------------------
// biggz coverage, task 2.11. Ported from gentle-pi 3.7.0
// tests/review-sidebar-state.test.ts, adapted to the observation API this port
// uses: upstream calls `publisher.tool(definition).execute(...)` because it owns
// the review tools, while this port is handed the same input and details by
// `tool_call` and `tool_result`, so every `run(...)` below is a `begin`
// followed by its `settle`. The mid-run assertions upstream makes inside a
// pending promise are made between the two calls here.
//
// One upstream case is deliberately NOT ported: "cancellation errors remain
// unknown while other failures are unavailable". It inspects the thrown error,
// which an observer never receives - only the `isError` flag - so this port
// maps every failure to `unavailable`. That limit is declared in the module
// header, and the module's own suite cannot test what its input cannot express.
// ---------------------------------------------------------------------------

function publisherFixture() {
	const events = [];
	const publisher = createReviewSidebarPublisher({ events: { emit: (_name, event) => events.push(event) } });
	const ctx = { cwd: "/repo", sessionManager: { getSessionId: () => "own" } };
	publisher.reset(ctx);
	// One call id per observation: a real session hands out a fresh id per tool call.
	let calls = 0;
	const run = (name, params, details, context = ctx, failed = false) => {
		const callId = `call-${++calls}`;
		publisher.begin(callId, name, params, context);
		publisher.settle(callId, name, details, context, failed);
	};
	const begin = (name, params, context = ctx) => {
		const callId = `call-${++calls}`;
		publisher.begin(callId, name, params, context);
		return callId;
	};
	const status = { result: { schema: "gentle-ai.review-integration.status/v9", authority: { state: "reviewing", lineage_id: "lineage" }, target_identity: "target", applicability: "current_target", projection: { paths: ["src/app.ts"] }, next_transition: { kind: "collect" } }, collectBindings: [{ collectBinding: "first" }, { collectBinding: "second" }] };
	const seed = () => run("gentle_review", { operation: "status", lineageId: "lineage" }, status);
	const snapshot = () => events.at(-1).snapshot;
	const closure = { tool: "gentle_review_capture", status: "closed", outcome: "native-last-event-closure", lineage_id: "lineage", closure: { schema: "gentle-ai.review-last-event-closure/v1", lineage_id: "lineage", target_identity: "target", state: "approved" } };
	const acknowledged = { operation: "acknowledge-approved", status: "closed", outcome: "native-approved-acknowledgement-completed", lineage_id: "lineage", target_identity: "target", authority: "burned" };
	const captured = { tool: "gentle_review_capture", status: "captured", outcome: "native-reviewer-result-captured", lineage_id: "lineage", host_relay: { transport: "pi_host_relay", lens: "review-risk", order: "0", subject_hash: "sha256:fixture", prompt_bytes: 10, result_bytes: 20, submission: '{"state":"approved","authority":"burned"}' } };
	return { publisher, ctx, events, run, begin, seed, snapshot, closure, acknowledged, captured };
}

test("the publisher announces the operation and keeps scope only for issued bindings", () => {
	const h = publisherFixture();
	h.seed();
	assert.deepEqual(h.snapshot(), { state: "in_review", scope: "app.ts" });
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "first" }, { outcome: "reviewer-model-run-forecast" });
	assert.deepEqual(h.snapshot(), { state: "forecast", scope: "app.ts" });
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "wrong" }, { outcome: "reviewer-model-run-forecast" });
	assert.equal(h.snapshot().scope, "Candidate scope unavailable", "an unissued binding cannot inherit the scope");
});

test("the publisher announces before the result and a nested workspaceRoot loses only correlation", () => {
	const h = publisherFixture();
	h.seed();
	const callId = h.begin("gentle_review", { operation: "status", lineageId: "lineage" });
	assert.deepEqual(h.snapshot(), { state: "checking", scope: "Candidate scope unavailable" }, "a status refresh announces itself before it answers");
	h.publisher.settle(callId, "gentle_review", { status: "blocked" }, h.ctx);
	h.seed();
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "first", workspaceRoot: "/repo/nested" }, { outcome: "reviewer-model-run-forecast" });
	assert.deepEqual(h.snapshot(), { state: "forecast", scope: "Candidate scope unavailable" }, "a nested workspace loses correlation instead of being canonicalized");
	h.seed();
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "first", workspaceRoot: "/repo" }, { outcome: "reviewer-model-run-forecast" });
	assert.deepEqual(h.snapshot(), { state: "forecast", scope: "app.ts" });
});

test("a superseded call and a reset both keep their late completion to themselves", () => {
	const h = publisherFixture();
	h.seed();
	const superseded = h.begin("gentle_review", { operation: "status", lineageId: "lineage" });
	h.seed();
	const afterSupersede = h.events.length;
	h.publisher.settle(superseded, "gentle_review", { status: "blocked" }, h.ctx);
	assert.equal(h.events.length, afterSupersede, "a superseded call publishes nothing when it finally answers");
	assert.deepEqual(h.snapshot(), { state: "in_review", scope: "app.ts" }, "the later call owns the display");
	h.publisher.reset(h.ctx);
	const stale = h.begin("gentle_review", { operation: "status", lineageId: "lineage" });
	h.publisher.reset(h.ctx);
	const afterReset = h.events.length;
	h.publisher.settle(stale, "gentle_review", h.closure, h.ctx);
	assert.equal(h.events.length, afterReset, "a reset discards the pending observation, so its answer never lands");
});

for (const name of ["gentle_review_capture", "gentle_review_capture_group"]) {
	test(`${name} retains issued scope through forecast, capture, closure and acknowledgement`, () => {
		const h = publisherFixture();
		h.seed();
		assert.deepEqual(h.snapshot(), { state: "in_review", scope: "app.ts" });
		const params = { lineageId: "lineage", ...(name.endsWith("group") ? { collectBindings: ["first", "second"] } : { collectBinding: "first" }) };
		h.run(name, params, { outcome: "reviewer-model-run-forecast" });
		assert.deepEqual(h.snapshot(), { state: "forecast", scope: "app.ts" });
		const running = h.begin(name, { ...params, reviewerRunAcknowledged: true });
		assert.deepEqual(h.snapshot(), { state: "reviewing", scope: "app.ts" }, "a bound capture shows the reviewers running");
		h.publisher.settle(running, name, { ...h.closure, tool: name }, h.ctx);
		assert.deepEqual(h.snapshot(), { state: "approved", scope: "app.ts" });
		h.run("gentle_review", { operation: "acknowledge-approved", lineageId: "lineage" }, h.acknowledged);
		assert.deepEqual(h.snapshot(), { state: "closed", scope: "app.ts" });
	});
}

test("separate issued captures retain only the remaining binding until terminal closure", () => {
	const h = publisherFixture();
	h.seed();
	for (const binding of ["first", "second"]) {
		h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: binding }, h.captured);
		assert.deepEqual(h.snapshot(), { state: "in_review", scope: "app.ts" }, "an opaque submission never implies closure");
	}
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "first" }, h.captured);
	assert.deepEqual(h.snapshot(), { state: "unknown", scope: "Candidate scope unavailable" }, "a consumed binding does not come back");
	h.seed();
	h.run("gentle_review_capture_group", { lineageId: "lineage", collectBindings: ["first", "second"] }, { ...h.closure, tool: "gentle_review_capture_group" });
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "second" }, h.captured);
	assert.deepEqual(h.snapshot(), { state: "unknown", scope: "Candidate scope unavailable" }, "a terminal closure ends the issued set");
});

test("a terminal closure needs its own matching identity, not just a matching wrapper", () => {
	const h = publisherFixture();
	const params = { lineageId: "lineage", collectBinding: "first" };
	for (const closure of [
		{ ...h.closure.closure, lineage_id: "other" },
		{ ...h.closure.closure, target_identity: "other" },
		{ ...h.closure.closure, lineage_id: undefined },
		{ ...h.closure.closure, target_identity: undefined },
	]) {
		h.seed();
		h.run("gentle_review_capture", params, { ...h.closure, closure });
		assert.deepEqual(h.snapshot(), { state: "approved", scope: "Candidate scope unavailable" });
	}
});

test("a nonterminal capture needs exact issued input and a well-formed matching output", () => {
	const h = publisherFixture();
	const bound = { lineageId: "lineage", collectBinding: "first", reviewerRunAcknowledged: true };
	for (const params of [
		{ lineageId: "lineage" }, { ...bound, collectBinding: "unissued" },
		{ ...bound, lineageId: "other" }, { ...bound, workspaceRoot: "/other" },
	]) {
		h.seed();
		h.run("gentle_review_capture", params, h.captured);
		assert.deepEqual(h.snapshot(), { state: "unknown", scope: "Candidate scope unavailable" });
	}
	for (const details of [
		{ ...h.captured, lineage_id: "other" }, { ...h.captured, target_identity: "other" },
		{ ...h.captured, target_identity: null }, { ...h.captured, lineage_id: undefined },
		{ ...h.captured, status: "closed" }, { ...h.captured, outcome: "unspecified" },
		{ ...h.captured, tool: "unrelated" }, { ...h.captured, host_relay: null },
		{ ...h.captured, host_relay: { ...h.captured.host_relay, transport: "other" } },
		{ ...h.captured, host_relay: { ...h.captured.host_relay, prompt_bytes: "10" } },
		{ ...h.captured, host_relay: { ...h.captured.host_relay, result_bytes: -1 } },
		{ ...h.captured, host_relay: { ...h.captured.host_relay, submission: {} } },
		{ ...h.captured, failure: { reason: "failed" } }, { status: "captured" },
	]) {
		h.seed();
		h.run("gentle_review_capture", bound, details);
		assert.equal(h.snapshot().scope, "Candidate scope unavailable");
		assert.ok(["unknown", "unavailable"].includes(h.snapshot().state));
	}
	h.seed();
	h.run("gentle_review_capture", bound, h.captured);
	assert.deepEqual(h.snapshot(), { state: "in_review", scope: "app.ts" }, "an opaque submission must not imply closure");
	h.run("gentle_review_capture", bound, h.captured);
	assert.deepEqual(h.snapshot(), { state: "unknown", scope: "Candidate scope unavailable" }, "a completed capture consumes its display binding");
});

test("a failed call reports unavailable, and a foreign session or reset publishes nothing", () => {
	const h = publisherFixture();
	h.seed();
	h.run("gentle_review_capture", { lineageId: "lineage", collectBinding: "first" }, undefined, h.ctx, true);
	assert.deepEqual(h.snapshot(), { state: "unavailable", scope: "Candidate scope unavailable" });
	h.seed();
	const foreign = { cwd: "/repo", sessionManager: { getSessionId: () => "foreign" } };
	const before = h.events.length;
	h.run("gentle_review", { operation: "status" }, { status: "blocked" }, foreign);
	assert.equal(h.events.length, before, "another session's review never paints this one");
	h.publisher.reset();
	h.run("gentle_review", { operation: "status" }, { status: "blocked" });
	assert.equal(h.events.length, before, "a disabled publisher answers nothing");
	const throwing = createReviewSidebarPublisher({ events: { emit: () => { throw new Error("display failed"); } } });
	throwing.reset(h.ctx);
	throwing.begin("call", "gentle_review", { operation: "status" }, h.ctx);
	throwing.settle("call", "gentle_review", { status: "blocked" }, h.ctx);
	assert.ok(true, "a throwing event bus is swallowed: display never changes an outcome");
});

// The two envelopes below are verbatim captures from this session's own review of
// unit 2.9 (lineage review-8c94163dee5a14e0): the STATUS the facade returned, and
// the closure the reviewer capture returned. The scope seed that precedes them is
// synthetic, because the real status envelope carries a 4 KB opaque binding that
// adds nothing to what this case checks.
test("the real STATUS and closure envelopes of this session's 2.9 review resolve as observed", () => {
	const h = publisherFixture();
	const status = { result: { schema: "gentle-ai.review-integration.status/v9", authority: { version: "compact-v2", lineage_id: "review-8c94163dee5a14e0", state: "reviewing", generation: 1, revision: "sha256:efdbf050c23f40b80b0ebd591393e24c61887fc886f40510ade87b973bc81367" }, action: "collect", frozen: { tier: "medium", original_changed_lines: 48, correction_budget: 24 }, target_identity: "sha256:6cac67fc311a39abc58268cd03c9e2be4c9464eb28e43530b6aaca83a43c2fcd", projection: { initial_review_tree: "5a7d2dd5778abbfc637975e2eb03806de503741c", current_candidate_tree: "5a7d2dd5778abbfc637975e2eb03806de503741c", paths: ["internal/assets/pi/biggz-shell/theme-customization.test.mjs", "internal/assets/pi/biggz-shell/theme-customization.ts"] }, next_transition: { kind: "collect", reason_code: "reviewer_results_required" } }, collectBindings: [{ collectBinding: "issued-by-the-facade" }] };
	h.run("gentle_review", { operation: "status", lineageId: "review-8c94163dee5a14e0" }, status);
	assert.deepEqual(h.snapshot(), { state: "in_review", scope: "theme-customization.test.mjs +1" });
	const closure = { operation: "review/capture-result", lineage_id: "review-8c94163dee5a14e0", state: "approved", store_revision: "sha256:27d480a868cd1f6e7c1c7c999c1e09ba1a37a0b8f77d9414c302cecccec0ef87", action: "the approved review completed on the last admitted event and awaits its exact acknowledgement" };
	const seed = { result: { schema: "gentle-ai.review-integration.status/v9", authority: { lineage_id: "review-8c94163dee5a14e0", state: "reviewing" }, target_identity: "sha256:6cac67fc311a39abc58268cd03c9e2be4c9464eb28e43530b6aaca83a43c2fcd", applicability: "current_target", projection: { paths: ["internal/assets/pi/biggz-shell/theme-customization.ts"] } }, collectBindings: [{ collectBinding: "issued-by-the-facade" }] };
	h.run("gentle_review", { operation: "status", lineageId: "review-8c94163dee5a14e0" }, seed);
	h.run("gentle_review_capture", { lineageId: "review-8c94163dee5a14e0", collectBinding: "issued-by-the-facade", reviewerRunAcknowledged: true }, { tool: "gentle_review_capture", status: "closed", outcome: "native-last-event-closure", lineage_id: "review-8c94163dee5a14e0", closure: { schema: "gentle-ai.review-last-event-closure/v1", operation: closure.operation, lineage_id: closure.lineage_id, state: closure.state, store_revision: closure.store_revision, action: closure.action } });
	// The real v1 closure carries no top-level `target_identity` - it nests it at
	// `acknowledgement.binding.target_identity` - while upstream's own fixture puts
	// it at the top. Upstream's closure-matching therefore fails against the real
	// envelope and the file scope is dropped once a capture closes. The state is
	// still resolved correctly; only the scope is lost, and this case pins that
	// observed behaviour instead of the fixture's.
	assert.deepEqual(h.snapshot(), { state: "approved", scope: "Candidate scope unavailable" });
	h.run("gentle_review", { operation: "acknowledge-approved", lineageId: "review-8c94163dee5a14e0" }, { operation: "acknowledge-approved", status: "closed", outcome: "native-approved-acknowledgement-completed", lineage_id: "review-8c94163dee5a14e0", target_identity: "sha256:6cac67fc311a39abc58268cd03c9e2be4c9464eb28e43530b6aaca83a43c2fcd", authority: "burned" });
	assert.deepEqual(h.snapshot(), { state: "closed", scope: "Candidate scope unavailable" }, "the burn this session observed resolves to the closed display state");
});
