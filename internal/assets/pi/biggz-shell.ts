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
import {
	createReviewSidebarPublisher,
	isReviewSidebarSnapshot,
	REVIEW_SIDEBAR_EVENT,
	type ReviewSidebarSnapshot,
} from "./biggz-shell/review-sidebar-state.ts";
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
	/** Unit 2.11: the review display state, when this session has one. */
	review?: ReviewSidebarSnapshot;
}

type Presentation = ReturnType<typeof resolveVisualSettings>["settings"];

// Unit 2.15 (R3-001) containment. shell-sidebar-layout.ts binds to pi-tui's
// private ScrollView/VStack constructors (shell-sidebar-layout.ts:16). A static
// import of that module here would make a pi-tui release that drops either
// export fail at ESM link time and take the whole shell with it: the footer,
// the below-editor header row, /biggz-usage and its shortcut. The module is
// therefore imported lazily, and a module that cannot link leaves the rail
// absent while everything else installs and runs (D12: silently, with no
// user-visible warning and no thrown error out of the factory).
//
// The type is derived from the module (a type-only `typeof import`, erased at
// runtime, so it adds no static import) rather than hand-declared: a signature
// drift in installSidebar/invalidateSidebar is then a compile-time error at the
// call sites below instead of an unchecked runtime assumption.
type SidebarLayoutModule = typeof import("./biggz-shell/shell-sidebar-layout.ts");

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
		review: options.review,
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
	review: () => ReviewSidebarSnapshot | undefined = () => undefined,
	presentation?: () => Presentation,
): ShellBarComponent {
	const unsubscribe = footerData.onBranchChange(() => {
		host.invalidateSidebar?.();
		host.requestRender();
	});
	return {
		render(width: number) {
			return renderShellBar(buildShellBarModel(pi, ctx, footerData, { dirty: dirty(), usage: usage(), review: review() }), theme, width, presentation?.());
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

// One bounded line for a footer-dispose step whose failure was swallowed: the
// step names what could not be released, the capped message names the cause, and
// the stack is dropped because this is a stderr diagnostic, not a crash report.
// It runs at most once per session (the idempotent session_shutdown dispose).
function describeDisposeFailure(step: string, err: unknown): string {
	const detail = err instanceof Error ? err.message : String(err);
	return `[biggz-shell] footer dispose: ${step} failed: ${detail.slice(0, 160)}`;
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
	// R3-001: the rail module is loaded once and cached; a failed link resolves
	// to undefined so every dependent path degrades to the no-rail bar. A
	// session keeps its `tui` (see the footer callback below) so a module that
	// resolves after the footer callback still installs rather than being lost.
	let sidebarLayoutPromise: Promise<SidebarLayoutModule | undefined> | undefined;
	let sidebarLayout: SidebarLayoutModule | undefined;
	const loadSidebarLayout = (): Promise<SidebarLayoutModule | undefined> =>
		(sidebarLayoutPromise ??= import("./biggz-shell/shell-sidebar-layout.ts").then(
			(module) => (sidebarLayout = module),
			() => undefined,
		));

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

	// Unit 2.11. Upstream owns the review tool definitions and wraps them, so its
	// publisher reads every result as it passes through. This port does not own
	// them - the harness provides the review tools - and pi hands an extension the
	// same input and details through `tool_call` and `tool_result`, so the publisher
	// is driven by observation instead. It emits one snapshot per live session and
	// the bar paints it as its own group; nothing here grants or reports review
	// authority, and a session without a snapshot simply never shows the group.
	const reviewSidebar = createReviewSidebarPublisher(pi);
	let reviewSnapshot: ReviewSidebarSnapshot | undefined;
	const redrawReview = (): void => {
		renderHost?.invalidateSidebar?.();
		renderHost?.requestRender();
	};
	pi.events.on(REVIEW_SIDEBAR_EVENT, (value) => {
		const event = value as { sessionId?: unknown; snapshot?: unknown } | undefined;
		if (!currentContext || event?.sessionId !== currentContext.sessionManager.getSessionId()) return;
		if (!isReviewSidebarSnapshot(event?.snapshot)) return;
		const { state, scope } = event.snapshot;
		reviewSnapshot = { state, scope };
		redrawReview();
	});
	pi.on("tool_call", (event, ctx) => {
		reviewSidebar.begin(event.toolCallId, event.toolName, event.input, ctx);
	});
	pi.on("tool_result", (event, ctx) => {
		reviewSidebar.settle(event.toolCallId, event.toolName, event.details, ctx, event.isError === true);
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
		// The publisher holds display correlation per session too, and a snapshot
		// must not outlive the session that produced it.
		reviewSidebar.reset();
		reviewSnapshot = undefined;
	});

	pi.on("session_start", (_event, ctx) => {
		if (!ctx.hasUI) return;
		presentation = resolveVisualSettings().settings;
		currentContext = ctx;
		reviewSidebar.reset(ctx);
		ctx.ui.setFooter((tui, theme, footerData) => {
			// Unit 2.6 hands the sidebar hook through host.invalidateSidebar: the
			// fullscreen rail shares the branch/usage events the narrow footer
			// re-renders on and the cache it marks stale is the rail's.
			// R3-001: with the rail module unloadable there is no rail cache to
			// invalidate, so the hook is a no-op instead of a throw.
			const host: ShellRenderHost = { requestRender: () => tui.requestRender(), invalidateSidebar: () => sidebarLayout?.invalidateSidebar(tui) };
			renderHost = host;
			if (tui.terminal) sidebarState(tui).visibility = { todo: presentation?.visibility.todo };
			const usageFor = () => usage.get(ctx.model?.provider ?? "");
			const reviewFor = () => reviewSnapshot;
			const component = createShellBarComponent(pi, ctx, host, theme, footerData, undefined, usageFor, reviewFor, () => presentation);
			// The Status card paints live session state no event re-registers a rail
			// part for (model, effort, context, cost, session name, extension
			// statuses), so the rail declares a digest and the fullscreen memo
			// notices by itself. D10 still holds: no review lineage, lens or budget
			// reaches the rail. Unit 2.11 wires the display state and the candidate
			// scope the producer resolves, which upstream's Status card shows too.
			const footerModel = (): ShellBarModel => buildShellBarModel(pi, ctx, footerData, { usage: usageFor(), review: reviewFor() });
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
			// Install the rail on arrival. If the module is already loaded this is
			// synchronous; otherwise the shared load resolves later and installs
			// then, because this session still holds the `tui` the footer callback
			// received. A session_shutdown that beats the load marks `disposed`, so
			// a late resolution cannot bind a torn-down TUI. `presentation` is read
			// through closures, so the live values are the ones the rail sees.
			const rail = { disposed: false, uninstall: undefined as (() => void) | undefined };
			const installRail = (late: boolean) => {
				if (rail.disposed || rail.uninstall || !sidebarLayout) return;
				try {
					rail.uninstall = sidebarLayout.installSidebar(tui, theme, () => presentation!.statusPlacement, () => presentation!.headerPlacement, () => presentation!.density);
					// The synchronous caller is pi's footer callback, which runs inside the
					// render pass that is already painting this bar, so the rail it installs is
					// drawn on that same pass (the module's own attach also requests a render).
					// A late caller runs after that pass with nothing left to cover the
					// install, so without this request the first session's rail can stay
					// invisible until an unrelated render. Only the late path asks, so the
					// common, cached-module path never schedules a redundant frame.
					// The request lives inside this try on purpose: on the async path a
					// throwing requestRender would otherwise reject the promise that installs
					// the rail and surface as an unhandled rejection.
					if (late) tui.requestRender();
				} catch {
					// D12: a rail that throws on a shape change stays absent instead of
					// escaping the factory (the async path would otherwise surface as an
					// unhandled rejection and could take the process down). The same
					// containment covers a throwing render request.
					return;
				}
			};
			if (sidebarLayout) installRail(false);
			else void loadSidebarLayout().then(() => installRail(true));
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
					rail.disposed = true;
					// Each release runs independently: a step that throws must not skip the
					// rest of the chain or escape into session_shutdown, so a throwing
					// dispose cannot take the shutdown down (the widget clear, the header
					// slot release, the rail uninstall and the component's own dispose are
					// all best-effort teardown). The three non-quiet steps name themselves
					// on stderr because each releases state that would otherwise outlive
					// the session: `header widget clear` (a missed clear risks a stale
					// header row painting into a later session), `header slot release`
					// (the rail's header part would stay registered), and `footer component
					// dispose` (its branch subscription would stay live). `quiet` marks the
					// single step that needs no diagnostic: the rail's own uninstall, which
					// D12 already accepts as failing on a shape change.
					const releases: { step: string; run: () => void; quiet?: boolean }[] = [
						{ step: "header widget clear", run: () => ctx.ui.setWidget(HEADER_WIDGET_KEY, undefined) },
						{ step: "header slot release", run: disposeHeader },
						{ step: "rail uninstall", run: () => rail.uninstall?.(), quiet: true },
						{ step: "footer component dispose", run: () => part.dispose() },
					];
					for (const { step, run, quiet } of releases) {
						try {
							run();
						} catch (err) {
							if (quiet) continue;
							console.error(describeDisposeFailure(step, err));
						}
					}
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
