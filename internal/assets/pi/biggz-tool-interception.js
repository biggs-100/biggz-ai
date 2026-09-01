/**
 * biggz-tool-interception — minimal parity with oh-my-pi ExtensionAPI
 * ToolCallInterceptor Before/After + session_stop guard via CanStopSession
 * Keeps registerFileWriteFallback intact; user_bash/python via Runner override
 * Pi parity slice: SafeToolRenderer try/catch fallback + stabilizeStreamingPreviews
 */
/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */

// ── Guard note: BIGGZ_PRETTY=0 disables pretty enhancements (checked inside exported fn) ──

// ── SafeToolRenderer (mirrors tool-execution.ts SafeToolRendererComponent) ──
export function SafeToolRenderer(renderFn, fallbackText = "✗ render error") {
	return (...args) => {
		try {
			return renderFn(...args);
		} catch (e) {
			return `\x1b[2m${fallbackText}\x1b[22m`;
		}
	};
}

export class SafeToolRendererComponent {
	constructor(toolName, component, fallback) {
		this.toolName = toolName;
		this.component = component;
		this.fallback = fallback;
		this._warned = false;
	}
	render(width) {
		try {
			if (this.component && typeof this.component.render === "function") return this.component.render(width);
			if (typeof this.component === "function") return this.component(width);
			return this.component;
		} catch (err) {
			if (!this._warned) {
				this._warned = true;
				try { console.warn(`Tool renderer failed ${this.toolName}: ${String(err)}`); } catch {}
			}
			try {
				if (typeof this.fallback === "function") {
					const fb = this.fallback();
					if (fb && typeof fb.render === "function") return fb.render(width);
					return fb;
				}
				if (this.fallback) return [String(this.fallback)];
			} catch {}
			return ["\x1b[2m✗ render error\x1b[22m"];
		}
	}
}

// ── stabilizeStreamingPreviews: strip trailing `+` and trailing whitespace before capPreviewLines ──
export function stabilizeStreamingPreviews(input) {
	if (typeof input === "string") {
		let s = input.replace(/\s+$/g, "");
		if (s.endsWith("+")) s = s.slice(0, -1).replace(/\s+$/g, "");
		return s;
	}
	if (Array.isArray(input)) {
		let changed = false;
		const out = input.map((v) => {
			const nv = stabilizeStreamingPreviews(v);
			if (nv !== v) changed = true;
			return nv;
		});
		return changed ? out : input;
	}
	if (input && typeof input === "object" && typeof input.diff === "string") {
		const st = stabilizeStreamingPreviews(input.diff);
		if (st !== input.diff) return { ...input, diff: st };
	}
	return input;
}

// ── capPreviewLines wrapper that stabilizes before capping (mirrors biggz-pi-pretty capPreviewLines) ──
const PREVIEW_LIMITS = Object.freeze({ COLLAPSED_LINES: 3, COLLAPSED_ITEMS: 8, EXPANDED_LINES: 12 });
export function capPreviewLines(lines, opts = {}) {
	const stabilized = stabilizeStreamingPreviews(lines);
	if (!Array.isArray(stabilized)) return [];
	if (opts.expanded) return stabilized;
	const max = opts.maxRows ?? opts.max ?? PREVIEW_LIMITS.COLLAPSED_LINES;
	if (stabilized.length <= max) return stabilized;
	const visible = max <= 1 ? [] : stabilized.slice(stabilized.length - (max - 1));
	const hidden = stabilized.length - visible.length;
	return [`… ${hidden} earlier ${hidden === 1 ? "line" : "lines"}`, ...visible];
}

export default function biggzToolInterception(pi) {
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	const blocked = ["rm -rf", "mkfs", ":(){:|:&};:"];
	// expose helpers for tests
	try {
		pi._biggzToolInterception = {
			SafeToolRenderer,
			SafeToolRendererComponent,
			stabilizeStreamingPreviews,
			capPreviewLines,
		};
		// also surface through _biggzExtension for single-writer parity checks
		if (pi._biggzExtension) {
			pi._biggzExtension.SafeToolRenderer = SafeToolRenderer;
			pi._biggzExtension.SafeToolRendererComponent = SafeToolRendererComponent;
			pi._biggzExtension.stabilizeStreamingPreviews = stabilizeStreamingPreviews;
			pi._biggzExtension.capPreviewLines = capPreviewLines;
		}
	} catch {}
	if (typeof pi.on === "function") {
		try {
			pi.on("tool_call", async (event, ctx) => {
				const name = event?.toolName ?? event?.name ?? "";
				const args = event?.args ?? event?.params ?? {};
				// stabilize streamed preview fields early (strip trailing +/whitespace) so capPreviewLines never shows `+` artefact
				if (args && typeof args.preview === "string") {
					try { args.preview = stabilizeStreamingPreviews(args.preview); } catch {}
				}
				if (event && typeof event.preview === "string") {
					try { event.preview = stabilizeStreamingPreviews(event.preview); } catch {}
				}
				const raw = JSON.stringify(args);
				for (const p of blocked) if (raw.includes(p)) return { block: true, reason: `blocked by policy: ${p}` };
				const mode = process.env.BIGGZ_APPROVAL_MODE || "auto";
				if (mode === "ask" && name === "user_bash") {
					const resolved = process.env.BIGGZ_TOOL_CONSENT;
					if (resolved === "deny") return { block: true, reason: "consent denied" };
					if (resolved === "allow") { try { ctx?.ui?.setStatus?.("tool", `tool_execution_start ${name}`); } catch {} return undefined; }
					return { block: true, reason: "awaiting consent" };
				}
				// Wrap any rendering that might throw: ensure stream not crashed, fallback dim error instead
				try { ctx?.ui?.setStatus?.("tool", `tool_execution_start ${name}`); } catch {}
				return undefined;
			});
			// Wrap tool_result rendering path as well — if custom renderer throws, show dim error
			pi.on("tool_result", async (event, ctx) => {
				// stabilize preview in result details if present
				try {
					const details = event?.details;
					if (details && typeof details.preview === "string") details.preview = stabilizeStreamingPreviews(details.preview);
					if (details && Array.isArray(details.previews)) details.previews = stabilizeStreamingPreviews(details.previews);
				} catch {}
				// observability-only; SafeToolRendererComponent will handle render exceptions at TUI layer
			});
			pi.on("session_stop", async () => {
				const pending = parseInt(process.env.BIGGZ_PENDING_FINDINGS || "0", 10);
				const lenses = parseInt(process.env.BIGGZ_PENDING_LENSES || "0", 10);
				if (pending > 0 || lenses > 0) return { block: true, reason: "CanStopSession blocked: pending work" };
				return undefined;
			});
		} catch {}
	}
}
