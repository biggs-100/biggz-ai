// Ported from gentle-pi 3.7.0 lib/agent-home.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged; the env names,
// config-home path and export names are biggz-ai (brand rename).
import { homedir } from "node:os";
import { join } from "node:path";

// Pi Subagents resolves its global directory as `PI_CODING_AGENT_DIR || ~/.pi/agent`,
// so an empty value must fall through here too or the two homes diverge again.
// BIGGZ_AGENT_HOME is biggz's own override; PI_CODING_AGENT_DIR is Pi's variable,
// not upstream branding, and biggz's Go side already honours it, so it stays as
// the second choice.
export function resolveBiggzAgentHome(env: NodeJS.ProcessEnv = process.env): string {
	return env.BIGGZ_AGENT_HOME || env.PI_CODING_AGENT_DIR || join(homedir(), ".pi", "agent");
}

// biggz's own config directory outside `~/.pi/agent`: everything the extension
// commands own lives here. It is defined once because the launch-time profile pin
// resolver reads the same profiles store the `/biggz:profiles` panel writes, and
// two spellings of the override would silently read two different stores. The
// home is `~/.biggz`, which the Go side already uses in
// internal/agents/pi/adapter.go, internal/backup and internal/bigmem, so
// extension-owned config belongs there and not in another product's directory.
export function biggzConfigHome(env: NodeJS.ProcessEnv = process.env): string {
	return env.BIGGZ_CONFIG_HOME || join(homedir(), ".biggz");
}
