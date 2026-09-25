// Ported from gentle-pi 3.7.0 tests/animation-policy.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged; the schema id
// and config-home option are biggz-ai (brand rename).
import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { ANIMATION_SCHEMA, parseAnimationPolicyFile, resolveAnimationPolicy, writeAnimationPolicy } from "./animation-policy.ts";

test("animation policy strictly accepts only the v1 schema and three modes", () => {
	for (const policy of ["quality", "performance", "potato"]) {
		assert.equal(parseAnimationPolicyFile(JSON.stringify({ schema: ANIMATION_SCHEMA, policy })), policy);
	}
	for (const raw of ["bad", "null", "[]", '{}', '{"policy":"potato"}', ...[
		{ schema: "biggz-ai.animations/v2", policy: "potato" },
		{ schema: ANIMATION_SCHEMA, policy: "POTATO" },
		{ schema: ANIMATION_SCHEMA, policy: false },
		{ schema: ANIMATION_SCHEMA, policy: "quality", extra: true },
	].map((value) => JSON.stringify(value))]) assert.equal(parseAnimationPolicyFile(raw), undefined);
});

test("global policy defaults to quality, persists atomically, and attributes malformed input without rewriting", () => {
	const home = mkdtempSync(join(tmpdir(), "gp-animations-"));
	try {
		const options = { configHome: home };
		assert.equal(resolveAnimationPolicy(options).policy, "quality");
		assert.equal(resolveAnimationPolicy(options).source, "default");
		for (const policy of ["potato", "performance", "quality"]) {
			const path = writeAnimationPolicy(policy, options);
			assert.deepEqual(JSON.parse(readFileSync(path, "utf8")), { schema: ANIMATION_SCHEMA, policy });
			assert.equal(resolveAnimationPolicy(options).policy, policy);
			assert.equal(resolveAnimationPolicy(options).source, "global_file");
			assert.deepEqual(readdirSync(home), ["animations.json"]);
		}
		const path = join(home, "animations.json");
		writeFileSync(path, "broken");
		const result = resolveAnimationPolicy(options);
		assert.equal(result.policy, "quality");
		assert.equal(result.source, "global_file");
		assert.equal(result.malformed, true);
		assert.equal(result.globalFile, path);
		assert.equal(readFileSync(path, "utf8"), "broken");
	} finally { rmSync(home, { recursive: true, force: true }); }
});
