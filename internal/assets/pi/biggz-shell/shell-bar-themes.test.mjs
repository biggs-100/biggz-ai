// biggz-ai role-palette guard (unit 2.3, not ported from upstream).
//
// The shell bar paints through theme role names, and a real pi theme throws
// from `theme.fg` when asked for a role it does not define. This suite binds
// that contract to the seven themes this repository actually ships: for each
// theme it reads the `colors` keys and renders the bar through a strict stub
// that throws on any role the theme file does not define. A role the bar names
// but no theme defines therefore fails here instead of crashing on a user's
// terminal.
//
// pi-tui is supplied by the repository's test oracle (see test/pi-tui-resolver.mjs).
import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { PI_TUI_SPECIFIER } from "../test/pi-tui-resolver.mjs";

const { visibleWidth } = await import(PI_TUI_SPECIFIER);
const {
	buildShellHeaderModel,
	renderShellBar,
	renderShellHeaderBar,
	renderShellHeaderRule,
	renderShellSidebarBar,
} = await import("./shell-bar.ts");

const themesDir = fileURLToPath(new URL("../themes/", import.meta.url));
const SHIPPED_THEMES = ["Gentle.json", "Gentleman-Cute.json", "Gentleman-Sexy.json", "dark.json", "light.json", "solarized-osaka.json", "titanium.json"];
const WIDTHS = [20, 60, 80, 120, 200];

// A fixture that reaches the roles the sidebar card only paints conditionally:
// the Changes counts (+success/−error) and notice (warning), an RDD review
// snapshot (muted/text in the card), a status (muted) and a usage window
// (dim/label). Without all of them a theme missing a role could slip through
// by rendering nothing that used it.
const FIXTURE = {
	cwd: "~/work/biggz-ai",
	branch: "feat/shell-visual-parity",
	dirty: 3,
	sessionName: "role palette",
	modelId: "gpt-5.5",
	effort: "high",
	profile: "team",
	contextPercent: 82,
	contextWindow: 272_000,
	costTotal: 9.49,
	subscription: true,
	usage: {
		provider: "openai-codex",
		plan: "pro",
		fetchedAt: 0,
		limits: [{ name: "codex", limitReached: false, windows: [
			{ label: "5h", usedPercent: 62, windowSeconds: 18_000, resetAt: null },
			{ label: "week", usedPercent: 31, windowSeconds: 604_800, resetAt: null },
		] }],
	},
	statuses: ["MCP: 3 servers"],
	changes: { files: 2, added: 7, deleted: 3, notice: "capture warning" },
	review: { state: "reviewing", scope: "app.ts +2" },
};

function strictTheme(name, keys) {
	return {
		fg(color, text) {
			if (!keys.has(color)) {
				throw new Error(`theme ${name} does not define color role "${color}" (defines: ${[...keys].join(", ")})`);
			}
			return text;
		},
		bold(text) {
			return text;
		},
	};
}

let checkedLines = 0;

test("the repository ships exactly the seven themes this guard binds", () => {
	const found = readdirSync(themesDir).filter((name) => name.endsWith(".json")).sort();
	assert.deepEqual(found, [...SHIPPED_THEMES].sort());
});

for (const file of SHIPPED_THEMES) {
	test(`${file} defines every role the shell bar paints`, (t) => {
		const json = JSON.parse(readFileSync(join(themesDir, file), "utf8"));
		const keys = new Set(Object.keys(json.colors ?? {}));
		const theme = strictTheme(file, keys);
		const header = buildShellHeaderModel(FIXTURE);
		for (const width of WIDTHS) {
			const rendered = [
				...renderShellBar(FIXTURE, theme, width),
				renderShellHeaderBar(header, theme, width, "alt+u").text,
				renderShellHeaderRule(theme, width),
				...renderShellSidebarBar(FIXTURE, theme, width),
			];
			for (const line of rendered) {
				// No try/catch: a role the theme does not define must fail this
				// test through strictTheme's throw, not be swallowed.
				assert.ok(visibleWidth(line) <= width, `${file} @ ${width}: visible width ${visibleWidth(line)} > ${width}: ${line}`);
				checkedLines++;
			}
		}
		t.diagnostic(`${file}: roles ${keys.size}`);
	});
}

test("every shipped theme rendered at every width", (t) => {
	assert.ok(checkedLines > 0, "the palette guard must actually render lines");
	t.diagnostic(`shell-bar role-palette width assertions: ${checkedLines}`);
});
