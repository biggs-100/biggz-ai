// Ported from gentle-pi 3.7.0 tests/review-sidebar-state.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged for the module
// this unit ships.
//
// Narrowing note: the upstream file also asserted against `renderShellSidebarBar`
// from `../lib/shell-bar.ts`. `shell-bar` is owned by a later work unit (2.3) and
// is not part of this one, so those render-only assertions are omitted here rather
// than tested against an invented stub. The snapshot contract they exercised is
// still asserted below.
import assert from "node:assert/strict";
import test from "node:test";
import { reviewSidebarSnapshot } from "./review-sidebar-state.ts";

// Display contract only: lifecycle evidence must be normalized by the producer,
// never inferred from a successful tool execution by the renderer.

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
		}
	}
});
