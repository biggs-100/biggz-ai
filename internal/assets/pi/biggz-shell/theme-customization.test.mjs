// Ported from gentle-pi 3.7.0 tests/theme-customization.test.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged; two portability
// deviations, both in fixtures only:
//  1. the palette document: the upstream suite read a pi-coding-agent dark.json
//     from node_modules, so this port reads the repository's own shipped fixture
//     at ../themes/dark.json; that fixture leaves the `text` color role empty, so
//     the test also supplies a valid text color. Every assert.match assertion is
//     kept unchanged;
//  2. the alias fixture: Windows without SeCreateSymbolicLinkPrivilege cannot
//     create a file symlink (EPERM), so when the OS denies it the test runs the
//     same palette assertions against the source path directly instead of
//     failing. Every assert.match assertion is kept unchanged; only the
//     realpath indirection is skipped on a platform that cannot make a symlink.
import assert from "node:assert/strict";
import { test } from "node:test";
import { mkdtempSync, readFileSync, readdirSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { sourcePalettePreview } from "./theme-customization.ts";

test("installed source palette resolves vars without activating a theme", (t) => {
	const root = mkdtempSync(join(tmpdir(), "theme-preview-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	const source = join(root, "theme.json");
	const document = JSON.parse(readFileSync(new URL("../themes/dark.json", import.meta.url), "utf8"));
	writeFileSync(source, JSON.stringify({ ...document, name: "dark", vars: { ...document.vars, previewAccent: "#123456" }, colors: { ...document.colors, accent: "previewAccent", text: "#c0c0c0" } }));
	const alias = join(root, "alias.json");
	let previewPath = source;
	try {
		symlinkSync(source, alias);
		previewPath = alias;
	} catch {
		// No symlink privilege on this platform: exercise the source directly.
	}
	const preview = sourcePalettePreview("dark", previewPath);
	assert.match(preview.title, /dark.*source palette/);
	assert.match(preview.sample, /48;2;18;52;86m/); // The accent swatch is a background; text is foreground.
	assert.match(preview.sample, /38;2;/);
	assert.match(preview.sample, /sample text/);
});

test("preview fails closed for missing, mismatched, malformed and oversized sources", (t) => {
	const root = mkdtempSync(join(tmpdir(), "theme-preview-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	const path = join(root, "theme.json");
	assert.throws(() => sourcePalettePreview("dark", undefined));
	writeFileSync(path, JSON.stringify({ name: "wrong", colors: { accent: "#abcdef", text: "#000000" } }));
	assert.throws(() => sourcePalettePreview("dark", path));
	writeFileSync(path, "not json");
	assert.throws(() => sourcePalettePreview("dark", path));
	writeFileSync(path, " ".repeat(256_001));
	assert.throws(() => sourcePalettePreview("dark", path));
});

test("source palette supports indexed colors and rejects unresolved references", (t) => {
	const root = mkdtempSync(join(tmpdir(), "theme-preview-"));
	t.after(() => rmSync(root, { recursive: true, force: true }));
	const path = join(root, "theme.json");
	writeFileSync(path, JSON.stringify({ name: "dark", colors: { accent: 42, text: 255 } }));
	assert.match(sourcePalettePreview("dark", path).sample, /48;5;42m.*38;5;255m/);
	writeFileSync(path, JSON.stringify({ name: "dark", vars: { a: "b", b: "a" }, colors: { accent: "a", text: "#ffffff" } }));
	assert.throws(() => sourcePalettePreview("dark", path));
});

// biggz coverage, task 2.9. Not upstream: the ported suite above overwrites the
// empty `text` role (see the header) because pi encodes the terminal's default
// foreground as "", and three of the themes this repository ships use exactly
// that. A preview that throws on them is unreachable with our own theme set, so
// this case previews every shipped theme as it ships: no doctored palette, no
// supplied color.
test("every shipped theme previews, terminal-default text role included", () => {
	const themesDir = new URL("../themes/", import.meta.url);
	const files = readdirSync(themesDir).filter((file) => file.endsWith(".json"));
	assert.ok(files.length >= 7, `expected the shipped theme set, found ${files.length} files`);
	const terminalDefaults = [];
	for (const file of files) {
		const path = fileURLToPath(new URL(file, themesDir));
		const document = JSON.parse(readFileSync(path, "utf8"));
		const preview = sourcePalettePreview(document.name, path);
		assert.match(preview.title, new RegExp(`${document.name}.*source palette`), `${file} must preview`);
		assert.match(preview.sample, /sample text/, `${file} must render the sample`);
		if (document.colors.text === "") {
			terminalDefaults.push(document.name);
			assert.match(
				preview.sample,
				/\x1b\[39m/,
				`${file} must render its empty text role as the terminal's default foreground, not as a palette color`,
			);
		}
	}
	assert.deepEqual(
		terminalDefaults.sort(),
		["dark", "light", "titanium"],
		"the empty text role must be exercised by the shipped themes, not avoided by a fixture",
	);
});
