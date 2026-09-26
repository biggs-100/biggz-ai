// Ported from gentle-pi 3.7.0 extensions/gentle-shell.ts
// (MIT, Copyright (c) 2025 Mario Zechner). Behaviour unchanged apart from the
// biggz-ai brand, env and command names below (brand rename), and the
// registry-only usage refresh: the built-in Codex/NaN fetches are not ported
// (see refreshUsage).
//
// This file owns the shell's pi UI slots: the footer, the below-editor header
// widget and the full-table usage command. It paints the ported shell bar into
// pi's footer with ctx.ui.setFooter, registers the header row with
// ctx.ui.setWidget(HEADER_WIDGET_KEY, ..., { placement: "belowEditor" }) and
// registers the usage command (plus its optional shortcut) that the header
// row's usage segment points at, and (unit 2.6) installs the fullscreen sidebar
// rail: the Status card as the "footer" rail part and the live header row as its
// "header" part. Deliberately NOT ported here, with the unit that owns each:
//   - the changes tracker                            → unit 2.7
//   - the prompt editor / ownsPrompt                 → unit 2.8
//   - the provider-usage fetch                       → out of scope for this port
//   - the worktree registry                          → out of scope for this port
// The startup banner (ctx.ui.setHeader) is not part of this port yet: pi's
// header slot is a single last-writer-wins slot that nothing in biggz claims,
// and a deployed asset by the upstream banner's file name would be deleted by
// this installer's own retired-removal list.
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import type { TUI } from "@earendil-works/pi-tui";
import { homedir } from "node:os";
import { isPrettyEnabled } from "./biggz-extension-api.js";
import {
	buildShellHeaderModel,
	renderShellBar,
	renderShellHeaderBar,
	renderShellHeaderRule,
	renderShellSidebarBar,
	shellEnabled,
	type ShellBarModel,
	type ShellBarTheme,
} from "./biggz-shell/shell-bar.ts";
import { sidebarHeader, sidebarPart, sidebarState } from "./biggz-shell/shell-sidebar.ts";
import { installSidebar, invalidateSidebar } from "./biggz-shell/shell-sidebar-layout.ts";
import {
	parseProviderUsage,
	parseUsageSource,
	UsageSourceRegistry,
	UsageStore,
	USAGE_SOURCE_EVENT,
	type ProviderUsage,
	type UsageSource,
} from "./biggz-shell/shell-usage.ts";
import { UsageView } from "./biggz-shell/shell-usage-view.ts";
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
// this slice never needs it.
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

// pi scopes widgets by key, so the key names this one slot and nothing else:
// a second asset registering the same key would silently replace this row. The
// upstream value is "gentle-shell-below-input-header".
const HEADER_WIDGET_KEY = "biggz-shell-below-input-header";
// The full-table usage command. It follows this port's command convention,
// established by shell-changes.ts's "/biggz-changes" hint: upstream's
// "gentle:" namespace becomes "biggz-".
export const USAGE_COMMAND_NAME = "biggz-usage";
const USAGE_SHORTCUT_DEFAULT = "alt+u";
const USAGE_SHORTCUT_ENV = "BIGGZ_SHELL_USAGE_KEY";
// One provider's fetch interval. The 5-minute rule is per provider: one
// provider's fetch cannot leave the next one waiting for an interval it never
// used.
const USAGE_REFRESH_MS = 5 * 60_000;

// Upstream's GENTLE_PI_SHELL_USAGE_KEY. An unset value keeps the default; an
// empty string or "off" disables the shortcut.
export function usageShortcut(env: NodeJS.ProcessEnv = process.env): string | undefined {
	const value = env[USAGE_SHORTCUT_ENV]?.trim();
	if (value === undefined) return USAGE_SHORTCUT_DEFAULT;
	return value === "" || value.toLowerCase() === "off" ? undefined : value;
}

// A registered source's resolved value crosses the same trust boundary a parsed
// HTTP payload does: foreign code's own object, validated field by field and
// never recorded by reference.
async function fetchFromSource(source: UsageSource, apiKey: string | undefined, fetchFn: typeof fetch, now: number): Promise<ProviderUsage | undefined> {
	try {
		const result = await source.fetch(apiKey, fetchFn, now);
		return result === undefined ? undefined : parseProviderUsage(result, source.provider);
	} catch {
		return undefined;
	}
}

export default function biggzShell(pi: ExtensionAPI, env: NodeJS.ProcessEnv = process.env): void {
	// Upstream evaluates the shell kill switch at factory scope and returns
	// before registering anything (`extensions/gentle-shell.ts`:
	// `if (!shellEnabled(env)) return;`, ahead of both registerCommand and
	// registerShortcut). The same early return is used here because
	// registerCommand/registerShortcut and pi.events.on are factory-time APIs
	// that session_start cannot undo: checking the switches there would still
	// leave /biggz-usage, the global alt+u shortcut and the usage-source
	// subscription installed and reachable for a user who disabled the layer.
	// Two switches must both pass: isPrettyEnabled() is the repo-wide kill
	// switch (BIGGZ_PRETTY / PI_SUBAGENT_CHILD) that every deployed pretty asset
	// honours, so BIGGZ_PRETTY=0 keeps pi's built-in footer; shellEnabled() is the
	// shell bar's own switch (BIGGZ_SHELL / PI_SUBAGENT_CHILD). The ctx.hasUI
	// check stays inside session_start, the only place a context exists.
	if (!isPrettyEnabled() || !shellEnabled(env)) return;

	let footer: ShellBarComponent | undefined;
	let presentation: Presentation | undefined;
	let currentContext: ExtensionContext | undefined;
	let renderHost: ShellRenderHost | undefined;

	const usage = new UsageStore();
	// Providers this port has never heard of get a usage source too, when the
	// extension that owns them registers one on pi.events; see the
	// USAGE_SOURCE_EVENT subscription below.
	const usageSources = new UsageSourceRegistry();
	const usageFetchedAt = new Map<string, number>();

	// Registry-only: this port ships no built-in provider fetch, so a provider
	// with no registered source has nothing to fetch and the panel honestly
	// shows its pending note (providerNote / USAGE_EMPTY_MESSAGE say so).
	const refreshUsage = async (ctx: ExtensionContext, force: boolean): Promise<void> => {
		const provider = ctx.model?.provider;
		if (!provider) return;
		const source = usageSources.get(provider);
		if (!source) return;
		const now = Date.now();
		if (!force && now - (usageFetchedAt.get(provider) ?? 0) < USAGE_REFRESH_MS) return;
		usageFetchedAt.set(provider, now);
		const apiKey = await ctx.modelRegistry.getApiKeyForProvider(provider).catch(() => undefined);
		const fetched = await fetchFromSource(source, apiKey, fetch, Date.now());
		if (!fetched) return;
		// A registered source can be replaced while its own fetch is still in
		// flight; the identity captured above is this call's source, so a stale
		// answer that outlives its replacement is discarded instead of
		// overwriting whatever the replacement already recorded.
		if (usageSources.get(provider) !== source) return;
		usage.record(fetched);
		renderHost?.invalidateSidebar?.();
		renderHost?.requestRender();
	};

	// Subscribed once, for the life of the extension: a registration can arrive
	// before the first session_start (the owning extension's factory runs
	// first) or after it. Either order works: a registration for the currently
	// active provider forces exactly one refresh.
	pi.events.on(USAGE_SOURCE_EVENT, (payload) => {
		const source = parseUsageSource(payload);
		if (!source) return;
		usageSources.register(source);
		if (currentContext?.model?.provider === source.provider) void refreshUsage(currentContext, true);
	});

	const openUsage = async (ctx: ExtensionContext): Promise<void> => {
		await refreshUsage(ctx, true);
		await ctx.ui.custom<null>(
			(tui, theme, _keybindings, done) =>
				new UsageView(usage, {
					theme,
					now: () => Date.now(),
					active: () => (ctx.model ? { provider: ctx.model.provider } : undefined),
					registry: () => usageSources,
					onRefresh: () => refreshUsage(ctx, true),
					onClose: () => done(null),
					requestRender: () => tui.requestRender(),
				}),
			{ overlay: true, overlayOptions: { width: "70%", minWidth: 60, anchor: "center" } },
		);
	};

	pi.registerCommand(USAGE_COMMAND_NAME, {
		description: "Show subscription usage windows for the connected providers. Press r to refetch.",
		handler: async (_args, ctx) => openUsage(ctx),
	});
	const usageShortcutKey = usageShortcut(env);
	if (usageShortcutKey) {
		pi.registerShortcut(usageShortcutKey as Parameters<ExtensionAPI["registerShortcut"]>[0], {
			description: "Show subscription usage windows for the connected providers",
			handler: async (ctx) => openUsage(ctx),
		});
	}

	// pi requires session-scoped resources to be released from an idempotent
	// session_shutdown handler. The footer component disposes its branch
	// subscription and clears the header widget; clearing the reference makes a
	// second run a no-op.
	pi.on("session_shutdown", () => {
		footer?.dispose();
		footer = undefined;
		// The event subscription above outlives a session; dropping the context
		// keeps a late source registration from refreshing a dead session.
		currentContext = undefined;
	});

	pi.on("session_start", (_event, ctx) => {
		if (!ctx.hasUI) return;
		presentation = resolveVisualSettings().settings;
		currentContext = ctx;
		ctx.ui.setFooter((tui, theme, footerData) => {
			// Unit 2.6 hands the sidebar hook through host.invalidateSidebar: the
			// fullscreen rail shares the branch/usage events the narrow footer
			// re-renders on and the cache it marks stale is the rail's.
			const host: ShellRenderHost = { requestRender: () => tui.requestRender(), invalidateSidebar: () => invalidateSidebar(tui) };
			renderHost = host;
			if (tui.terminal) sidebarState(tui).visibility = { todo: presentation?.visibility.todo };
			const usageFor = () => usage.get(ctx.model?.provider ?? "");
			const component = createShellBarComponent(pi, ctx, host, theme, footerData, undefined, usageFor, () => presentation);
			// The Status card paints live session state no event re-registers a rail
			// part for (model, effort, context, cost, session name, extension
			// statuses), so the rail declares a digest and the fullscreen memo
			// notices by itself. D10: no review facts reach the rail — no lineage,
			// lens or budget — because the model never supplies them; unit 2.11 owns
			// the review protocol wiring.
			const footerModel = (): ShellBarModel => buildShellBarModel(pi, ctx, footerData, { usage: usageFor() });
			const part = sidebarPart(tui, "footer", component, {
				digest: () => JSON.stringify([footerModel(), presentation]),
				render: (width) => renderShellSidebarBar(footerModel(), theme, width, presentation),
				invalidate() {},
			});
			// Deliberate deviation from upstream: its showChanges() registers a
			// `sidebarPart(tui, "changes", ...)` (upstream extensions/gentle-shell.ts:1230),
			// but the rail's section list is literally ["footer", "agents", "todo"]
			// (upstream lib/shell-sidebar-layout.ts:199), so that part can never
			// render. It is not ported here; unit 2.7 owns the changes tracker and the
			// Status card above already carries its summary.
			// The header row carries everything that ticks every frame (model,
			// effort, context, cost, usage) plus session identity; it never sees
			// extension statuses or the working/thinking state. D11: this part owns
			// the rail's `top` case; the below-editor widget further down owns only
			// `below-input`, so the header is never painted twice in fullscreen.
			const headerBar = (width: number) =>
				renderShellHeaderBar(buildShellHeaderModel(footerModel()), theme, width, usageShortcutKey, presentation);
			const disposeHeader = sidebarHeader(tui, {
				digest: () => JSON.stringify([buildShellHeaderModel(footerModel()), presentation]),
				render: (width) => [headerBar(width).text, renderShellHeaderRule(theme, width)],
				invalidate() {},
				handleMouse(event) {
					if (event.type !== "click" || event.button !== "left") return undefined;
					if (event.y !== 0) return undefined; // the rule row under the status line is decorative, never clickable
					const { usageSpan } = headerBar(event.width);
					if (!usageSpan || event.x < usageSpan.start || event.x >= usageSpan.end) return undefined;
					void openUsage(ctx);
					return { handled: true, render: true };
				},
			});
			// presentation is assigned earlier in this same session_start handler,
			// before pi can invoke the footer factory, so the rail's live placement
			// readers can treat it as set (the biggz equivalent of upstream's
			// non-optional visualSettings).
			const uninstall = installSidebar(tui, theme, () => presentation!.statusPlacement, () => presentation!.headerPlacement, () => presentation!.density);
			// pi's public widget slot follows the editor even when the fullscreen
			// rail is absent. TUI.mode is "regular" unless the user opts into
			// fullscreen, so this row stays inert ([]) until headerPlacement is
			// "below-input" AND the terminal is fullscreen. That is expected: the
			// row cannot be forced on from here. The click handler shares the
			// gate, so an inert row never swallows a click.
			const belowHeader = () => presentation?.headerPlacement === "below-input" && (tui as TUI & { mode?: string }).mode === "fullscreen";
			ctx.ui.setWidget(HEADER_WIDGET_KEY, () => ({
				render(width: number) {
					return belowHeader() ? [headerBar(width).text, renderShellHeaderRule(theme, width)] : [];
				},
				invalidate() {},
				handleMouse(event) {
					if (!belowHeader() || event.type !== "click" || event.button !== "left" || event.y !== 0) return undefined;
					const span = headerBar(event.width).usageSpan;
					if (!span || event.x < span.start || event.x >= span.end) return undefined;
					void openUsage(ctx);
					return { handled: true, render: true };
				},
			}), { placement: "belowEditor" });
			// pi calls dispose() on session shutdown; clearing the widget here is
			// what keeps a second session from painting a stale header row. The
			// rail uninstalls and the header part releases its slot on the same
			// path, so a second session starts from the native layout.
			footer = {
				...part,
				dispose() {
					ctx.ui.setWidget(HEADER_WIDGET_KEY, undefined);
					disposeHeader();
					uninstall();
					part.dispose();
				},
			};
			return footer;
		});
		// Force one registry-only refresh per session: a source that registered
		// before this session_start (its owning extension's factory runs first)
		// has no context yet at registration time, so this is the only call that
		// notices it. With no registered source it is a no-op.
		void refreshUsage(ctx, true);
	});
}
