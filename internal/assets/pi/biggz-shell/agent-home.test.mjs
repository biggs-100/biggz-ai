// Ported from gentle-pi 3.7.0 tests/agent-home.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged; the env names,
// config-home path and helper name are biggz-ai (brand rename).
import assert from "node:assert/strict";
import { mkdtempSync, rmSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { biggzConfigHome } from "./agent-home.ts";

// The two Pi homes: `~/.pi/agent` holds Pi's own agent definitions and
// `~/.biggz` holds everything the biggz commands own. Both are resolved
// once and shared, because the launch-time profile pin resolver reads the same
// profiles store the `/biggz:profiles` panel writes: two spellings of the override
// would silently read two different stores.

function isolatedHome(t) {
	const home = mkdtempSync(join(tmpdir(), "biggz-config-home-"));
	const previousHome = process.env.HOME;
	const previousUserProfile = process.env.USERPROFILE;
	// Isolation must cover POSIX and Windows, the way homedir() resolves each.
	process.env.HOME = home;
	process.env.USERPROFILE = home;
	t.after(() => {
		if (previousHome === undefined) delete process.env.HOME;
		else process.env.HOME = previousHome;
		if (previousUserProfile === undefined) delete process.env.USERPROFILE;
		else process.env.USERPROFILE = previousUserProfile;
		rmSync(home, { recursive: true, force: true });
	});
	return home;
}

test("biggzConfigHome honours BIGGZ_CONFIG_HOME and otherwise falls back to ~/.biggz", (t) => {
	const home = isolatedHome(t);
	assert.equal(biggzConfigHome({ BIGGZ_CONFIG_HOME: "/custom/biggz" }), "/custom/biggz");
	assert.equal(biggzConfigHome({ BIGGZ_CONFIG_HOME: "" }), join(homedir(), ".biggz"));
	assert.equal(biggzConfigHome({}), join(homedir(), ".biggz"));
	assert.equal(biggzConfigHome(), join(homedir(), ".biggz"));
	assert.equal(
		join(biggzConfigHome(), "profiles.json"),
		join(home, ".biggz", "profiles.json"),
		"the store path is derived from the config home, not from the agent home",
	);
});

test("the config home ignores the Pi agent-home overrides", (t) => {
	const home = isolatedHome(t);
	// PI_CODING_AGENT_DIR and BIGGZ_AGENT_HOME move Pi's agent directory only.
	// Reusing that resolver here would point the profiles store at the agent home.
	assert.equal(
		biggzConfigHome({ PI_CODING_AGENT_DIR: "/pi/agent", BIGGZ_AGENT_HOME: "/biggz/agent" }),
		join(home, ".biggz"),
	);
});
