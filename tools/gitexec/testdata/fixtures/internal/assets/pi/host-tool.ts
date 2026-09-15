import { execFileSync } from "node:child_process";

const run = process.env.RUNNER ?? execFileSync;

export function toplevel(dir: string): string {
	const direct = execFileSync("git", ["rev-parse", "--show-toplevel"], { cwd: dir });
	const aliased = run("git", ["status", "--porcelain=v1"], { cwd: dir });
	return String(direct || aliased);
}
