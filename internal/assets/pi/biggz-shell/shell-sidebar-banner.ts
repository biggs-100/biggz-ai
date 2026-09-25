// Ported from gentle-pi 3.7.0 lib/shell-sidebar-banner.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged apart from the
// biggz-ai product label below (brand rename).
import { visibleWidth } from "@earendil-works/pi-tui";
import type { ShellBarTheme } from "./shell-bar.ts";

export function renderSidebarBanner(theme: ShellBarTheme, width: number): string[] {
	const label = "✿ biggz ✿";
	const space = width - visibleWidth(label);
	if (space < 0) return [];
	// Gentleman's accent is pink; card titles use a separate champagne role.
	const title = theme.fg("accent", "✿") + " " + theme.fg("text", "biggz") + " " + theme.fg("accent", "✿");
	return [" ".repeat(Math.floor(space / 2)) + title + " ".repeat(Math.ceil(space / 2))];
}
