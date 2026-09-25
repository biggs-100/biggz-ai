// Ported from gentle-pi 3.7.0 extensions/gentle-shell.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged apart from the
// biggz-ai brand, env and command names below (brand rename).
//
// This file owns exactly one slice of the upstream extension: the footer slot.
// It paints the ported shell bar into pi's footer with ctx.ui.setFooter and
// nothing else. Deliberately NOT ported here, with the unit that owns each:
//   - the fullscreen sidebar part / installSidebar   → unit 2.6
//   - the header widget + HEADER_WIDGET_KEY          → unit 2.5
//   - the changes tracker                            → unit 2.7
//   - the prompt editor / ownsPrompt                 → unit 2.8
//   - the provider-usage fetch                       → out of scope for this port
//   - the worktree registry                          → out of scope for this port
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import { homedir } from "node:os";
import { isPrettyEnabled } from "./biggz-extension-api.js";
import { renderShellBar, shellEnabled, type ShellBarModel, type ShellBarTheme } from "./biggz-shell/shell-bar.ts";
import type { ProviderUsage } from "./biggz-shell/shell-usage.ts";
import { resolveVisualSettings } from "./biggz-shell/visual-customization-policy.ts";

// The subset of pi's ReadonlyFooterDataProvider the bar reads. Declared here so
// the port does not depend on pi's provider class, and so tests can supply a
// small fake.
interface ShellFooterData {
	getGitBranch(): string | null;
	getExtensionStatuses(): ReadonlyMap<string, string>;
	getAvailableProviderCount(): number;
	onBranchChange(callback: () => void): () => void;
}

// invalidateSidebar is the sidebar hook unit 2.6 supplies. It stays optional so
// this footer-only slice never needs it.
interface ShellRenderHost {
	requestRender(): void;
	invalidateSidebar?(): void;
}

interface ShellBarComponent {
	render(width: number): string[];
	invalidate(): void;
	dispose(): void;
}

interface BuildOptions {
	profile?: string;
	home?: string;
	dirty?: number;
	usage?: ProviderUsage;
}

type Presentation = ReturnType<typeof resolveVisualSettings>["settings"];

interface AssistantUsageEntry {
	type: string;
	message?: {
		role?: string;
		usage?: {
			cost?: { total?: number };
		};
	};
}

function shortenHome(cwd: string, home: string | undefined): string {
	if (home && cwd.startsWith(home)) return `~${cwd.slice(home.length)}`;
	return cwd;
}

function sessionCost(ctx: ExtensionContext): number {
	let total = 0;
	for (const entry of ctx.sessionManager.getEntries() as AssistantUsageEntry[]) {
		if (entry.type !== "message" || entry.message?.role !== "assistant") continue;
		total += entry.message.usage?.cost?.total ?? 0;
	}
	return total;
}

export function buildShellBarModel(
	pi: ExtensionAPI,
	ctx: ExtensionContext,
	footerData: ShellFooterData,
	options: BuildOptions = {},
): ShellBarModel {
	const home = options.home ?? homedir();
	const usage = ctx.getContextUsage();
	const model = ctx.model;
	const statuses = Array.from(footerData.getExtensionStatuses().entries())
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([, text]) => text);
	return {
		cwd: shortenHome(ctx.sessionManager.getCwd(), home),
		profile: options.profile,
		branch: footerData.getGitBranch(),
		dirty: options.dirty,
		sessionName: ctx.sessionManager.getSessionName(),
		modelId: model?.id ?? "no-model",
		effort: model?.reasoning ? pi.getThinkingLevel() : undefined,
		contextPercent: usage?.percent ?? null,
		contextWindow: usage?.contextWindow ?? model?.contextWindow ?? 0,
		costTotal: sessionCost(ctx),
		subscription: model ? ctx.modelRegistry.isUsingOAuth(model) : false,
		usage: options.usage,
		statuses,
	};
}

export function createShellBarComponent(
	pi: ExtensionAPI,
	ctx: ExtensionContext,
	host: ShellRenderHost,
	theme: ShellBarTheme,
	footerData: ShellFooterData,
	dirty: () => number | undefined = () => undefined,
	usage: () => ProviderUsage | undefined = () => undefined,
	presentation?: () => Presentation,
): ShellBarComponent {
	const unsubscribe = footerData.onBranchChange(() => {
		host.invalidateSidebar?.();
		host.requestRender();
	});
	return {
		render(width: number) {
			return renderShellBar(buildShellBarModel(pi, ctx, footerData, { dirty: dirty(), usage: usage() }), theme, width, presentation?.());
		},
		invalidate() {},
		dispose() {
			unsubscribe();
		},
	};
}

export default function biggzShell(pi: ExtensionAPI): void {
	let footer: ShellBarComponent | undefined;
	let presentation: Presentation | undefined;

	// pi requires session-scoped resources to be released from an idempotent
	// session_shutdown handler. The footer component is the only resource this
	// slice installs, and clearing the reference makes a second run a no-op.
	pi.on("session_shutdown", () => {
		footer?.dispose();
		footer = undefined;
	});

	pi.on("session_start", (_event, ctx) => {
		if (!ctx.hasUI) return;
		// Two gates before the UI is touched. isPrettyEnabled() is the
		// repo-wide kill switch (BIGGZ_PRETTY / PI_SUBAGENT_CHILD) that every
		// deployed pretty asset honours, so BIGGZ_PRETTY=0 keeps pi's built-in
		// footer; shellEnabled() is the shell bar's own switch (BIGGZ_SHELL /
		// PI_SUBAGENT_CHILD).
		if (!isPrettyEnabled() || !shellEnabled()) return;
		presentation = resolveVisualSettings().settings;
		ctx.ui.setFooter((tui, theme, footerData) => {
			// Unit 2.6 supplies the sidebar hook through host.invalidateSidebar;
			// this slice only needs requestRender.
			const host: ShellRenderHost = { requestRender: () => tui.requestRender() };
			const component = createShellBarComponent(pi, ctx, host, theme, footerData, undefined, undefined, () => presentation);
			footer = component;
			return component;
		});
	});
}
