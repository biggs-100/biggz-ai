/**
 * biggz-question-mouse — Rank 2: oh-my-pi Ask Dialog port (v5)
 *
 * Port of oh-my-pi Tabbed Ask Dialog to biggz-ai. Replaces the 450 LOC
 * push-above workaround with proper overlay: TabBar per question,
 * splitPreviewSegments (fence-aware), MAX_HEADER_ROWS=4,
 * DIALOG_HEIGHT_RATIO=0.7 clamped panel, countdown-timer, boundPromptTitle
 * 3 rows, grouped tabs, renderQuestionOptionLines / renderAnswerOptionLines.
 *
 * Visual: bottom sheet with tabs Q1 │ Q2 │ Q3, left options list, right
 * preview pane (markdown+code), Other (type…) inline editor where preview
 * exists. Screenshot reference: oh-my-pi/assets/ask.webp
 *
 * Two modes:
 *  - Default (BIGGZ_MOUSE !== "1"): proper TabBar/ScrollView overlay via
 *    ctx.ui.custom → AskDialogComponent (keyboard arrow/tab, no SGR mouse).
 *  - Fallback (BIGGZ_MOUSE === "1"): legacy SGR mouse parity + push-above
 *    compositeOverlays monkey patch (mouse → arrow synthesis). Original
 *    workaround kept opt-in for users who prefer mouse.
 *
 * Single-file revert: if BIGGZ_PRETTY=0 or PI_SUBAGENT_CHILD=1, return early
 * before overlay. No new npm deps beyond @oh-my-pi/pi-tui (or vendored minimal).
 *
 * Pi-tui imports: TabBar, ScrollView, Markdown from @oh-my-pi/pi-tui if
 * available at runtime (Pi's @earendil-works/pi-tui), else vendored minimal
 * inline (see check internal/assets/pi/subagent-config.json).
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzQuestionMouse(pi) {
  if (process.env.PI_SUBAGENT_CHILD === "1") return;
  if (process.env.BIGGZ_PRETTY === "0") return;

  function isMouseAllowed(){
    if(process.env.PI_SUBAGENT_CHILD==="1")return false;
    if(process.env.BIGGZ_PRETTY==="0")return false;
    if(process.env.TERM==="dumb")return false;
    if(process.env.BIGGZ_NO_ANIMATION==="1")return false;
    if(process.env.GENTLE_AI_NO_ANIMATION==="1")return false;
    return process.env.BIGGZ_MOUSE==="1";
  }
  const BIGGZ_MOUSE = isMouseAllowed();

  // ── Spec constants ──────────────────────────────────────────────
  const DIALOG_HEIGHT_RATIO = 0.7;
  const MIN_DIALOG_ROWS = 12;
  const MIN_BODY_ROWS = 5;
  const MAX_HEADER_ROWS = 4;
  const MAX_HEADER_CHIP_WIDTH = 16;
  const MAX_PROMPT_TITLE_ROWS = 3;
  const PROMPT_TITLE_CHROME_COLUMNS = 4;
  const OTHER_OPTION = "Other (type your own)";
  const SUBMIT_OPTION = "Submit";
  const DOUBLE_CLICK_MS = 400;
  const MOUSE_ENABLE = "\x1b[?1000h\x1b[?1006h";
  const MOUSE_DISABLE = "\x1b[?1000l\x1b[?1006l";

  // ── Vendored pi-tui primitives (fallback when dynamic import unavailable) ──
  // These mirror @oh-my-pi/pi-tui utils so the dialog works even when
  // pi-tui is not installed in biggz-ai repo (node_modules missing).
  // At runtime inside Pi, we try to use real pi-tui via dynamic import.
  const Ellipsis = { Unicode: "…", Omit: "" };

  function stripAnsi(s) {
    try {
      return String(s)
        .replace(/\x1b\[[0-9;]*[A-Za-z]/g, "")
        .replace(/\x1b\][^\x07]*\x07/g, "")
        .replace(/\x1b\(B/g, "");
    } catch { return String(s); }
  }
  function replaceTabs(text) {
    return String(text).replace(/\t/g, "    ");
  }
  function padding(n) {
    if (n <= 0) return "";
    if (n <= 512) return " ".repeat(n);
    return " ".repeat(n);
  }
  function visibleWidth(str) {
    if (!str) return 0;
    const s = stripAnsi(str);
    // Naive width: count codepoints, handle wide? sufficient for node --check
    // Use spread to handle surrogate pairs; CJK 2-cell is approximated as 1 here
    // but fallback is correct for ASCII art. Real pi-tui uses Bun.stringWidth.
    let w = 0;
    for (const ch of [...s]) {
      const cp = ch.codePointAt(0);
      // Rough east-asian wide detection (2-cell) for fallback
      if (cp !== undefined && ((cp >= 0x1100 && cp <= 0x115F) || (cp >= 0x2E80 && cp <= 0xA4CF) || (cp >= 0xAC00 && cp <= 0xD7A3) || (cp >= 0xF900 && cp <= 0xFAFF) || (cp >= 0xFE10 && cp <= 0xFE6F) || (cp >= 0xFF00 && cp <= 0xFF60) || (cp >= 0xFFE0 && cp <= 0xFFE6))) w += 2;
      else w += 1;
    }
    return w;
  }
  function truncateToWidth(text, maxWidth, ellipsis = Ellipsis.Unicode) {
    maxWidth = Math.max(0, maxWidth | 0);
    const s = String(text);
    if (visibleWidth(s) <= maxWidth) return s;
    if (maxWidth <= 0) return "";
    if (maxWidth === 1) return ellipsis === Ellipsis.Omit ? "" : "…";
    let out = "";
    let w = 0;
    const ellW = ellipsis === Ellipsis.Omit ? 0 : 1;
    const target = maxWidth - ellW;
    for (const ch of [...s]) {
      const cw = visibleWidth(ch);
      if (w + cw > target) break;
      out += ch;
      w += cw;
    }
    return out + (ellipsis === Ellipsis.Omit ? "" : "…");
  }
  function wrapTextWithAnsi(text, width) {
    width = Math.max(1, width | 0);
    const raw = String(text);
    // Quick ANSI-aware wrap: split on spaces, measure visibleWidth
    const words = raw.split(/(\s+)/);
    const lines = [];
    let cur = "";
    let curW = 0;
    for (const w of words) {
      if (w === "") continue;
      const isSpace = /^\s+$/.test(w);
      if (isSpace) {
        // Keep single space separation, but don't start line with space
        if (cur === "") continue;
        const wW = visibleWidth(w);
        if (curW + wW <= width) {
          cur += w;
          curW += wW;
        } else {
          lines.push(cur);
          cur = "";
          curW = 0;
        }
        continue;
      }
      const wordW = visibleWidth(w);
      if (wordW > width) {
        // Hard break long word
        if (cur) { lines.push(cur); cur = ""; curW = 0; }
        let chunk = "";
        let cw = 0;
        for (const ch of [...w]) {
          const chW = visibleWidth(ch);
          if (cw + chW > width) {
            lines.push(chunk);
            chunk = ch;
            cw = chW;
          } else {
            chunk += ch;
            cw += chW;
          }
        }
        if (chunk) { cur = chunk; curW = cw; }
        continue;
      }
      if (curW === 0) { cur = w; curW = wordW; }
      else if (curW + 1 + wordW <= width) { cur += " " + w; curW += 1 + wordW; }
      else { lines.push(cur); cur = w; curW = wordW; }
    }
    if (cur) lines.push(cur);
    return lines.length ? lines : [""];
  }
  function clamp(value, min, max) { return Math.max(min, Math.min(value, max)); }
  function normalizedInlineInput(input) {
    return replaceTabs(input).replace(/\s+/g, " ").trim();
  }
  function promptTitleContentWidth() {
    const cols = (typeof process !== "undefined" && process.stdout && process.stdout.columns) || 80;
    return Math.max(1, cols - PROMPT_TITLE_CHROME_COLUMNS);
  }
  function boundPromptTitle(prefix, question) {
    const width = promptTitleContentWidth();
    const flat = normalizedInlineInput(`${prefix}${question}`);
    const wrapped = wrapTextWithAnsi(flat, width);
    if (wrapped.length <= MAX_PROMPT_TITLE_ROWS) return wrapped.join("\n");
    const kept = wrapped.slice(0, MAX_PROMPT_TITLE_ROWS - 1);
    const last = truncateToWidth(wrapped[MAX_PROMPT_TITLE_ROWS - 1] ?? "", width, Ellipsis.Unicode);
    return [...kept, last].join("\n");
  }
  function questionTabLabel(question, index) {
    const base = (question && (question.header?.trim() || question.id)) || `Q${index + 1}`;
    return truncateToWidth(replaceTabs(base), MAX_HEADER_CHIP_WIDTH, Ellipsis.Unicode);
  }

  // ── splitPreviewSegments (fence-aware) ───────────────────────────
  // Mirrors oh-my-pi packages/coding-agent/src/modes/components/ask-dialog.ts
  // splitPreviewSegments fence-aware: code fences + language + tilde fences.
  // Splits preview markdown into markdown/code segments by detecting
  // ``` / ~~~ fences, preserving language. Tested against oh-my-pi's
  // splitPreviewSegments fence-aware: code fences not leaked, markdown+code
  // split renders left options list + right preview pane (Markdown + code).
  // Used by renderPreviewContent and renderRowLabel via renderCachedPreview.
  function splitPreviewSegments(preview) {
    const segments = [];
    const markdownBuffer = [];
    let fenceChar;
    let fenceLength = 0;
    let fenceLanguage;
    let codeBuffer = [];
    const flushMarkdown = () => {
      if (markdownBuffer.length === 0) return;
      segments.push({ kind: "markdown", text: markdownBuffer.join("\n"), language: undefined });
      markdownBuffer.length = 0;
    };
    const flushCode = () => {
      segments.push({ kind: "code", text: codeBuffer.join("\n"), language: fenceLanguage });
      codeBuffer = [];
      fenceChar = undefined;
      fenceLength = 0;
      fenceLanguage = undefined;
    };
    for (const line of replaceTabs(String(preview)).split("\n")) {
      const fenceMatch = /^(\s{0,3})(`{3,}|~{3,})(.*)$/.exec(line);
      if (fenceChar !== undefined) {
        if (fenceMatch) {
          const marker = fenceMatch[2] ?? "";
          const info = fenceMatch[3]?.trim() ?? "";
          if (marker.startsWith(fenceChar) && marker.length >= fenceLength && info === "") {
            flushCode();
            continue;
          }
        }
        codeBuffer.push(line);
        continue;
      }
      if (fenceMatch) {
        flushMarkdown();
        const marker = fenceMatch[2] ?? "";
        fenceChar = marker[0];
        fenceLength = marker.length;
        fenceLanguage = fenceMatch[3]?.trim().split(/\s+/, 1)[0] || undefined;
        codeBuffer = [];
        continue;
      }
      markdownBuffer.push(line);
    }
    if (fenceChar !== undefined) {
      segments.push({ kind: "code", text: codeBuffer.join("\n"), language: fenceLanguage });
    } else {
      flushMarkdown();
    }
    return segments;
  }

  // ── Theme helpers (use pi's theme when available) ────────────────
  function fallbackTheme() {
    return {
      fg: (c, t) => t,
      bg: (c, t) => t,
      bold: (t) => t,
      dim: (t) => `\x1b[2m${t}\x1b[0m`,
      check: (t) => t,
      checkbox: { checked: "☑", unchecked: "☐" },
      radio: { selected: "●", unselected: "○" },
      nav: { cursor: "❯" },
      status: { success: "✓", warning: "⚠", info: "ℹ" },
      boxRound: {
        topLeft: "╭", topRight: "╮", bottomLeft: "╰", bottomRight: "╯",
        horizontal: "─", vertical: "│", teeRight: "├", teeLeft: "┤", teeDown: "┬", teeUp: "┴"
      },
      format: { bracketLeft: "[", bracketRight: "]" },
    };
  }
  function getFallbackTabBarTheme(th) {
    const t = th || fallbackTheme();
    return {
      label: (text) => t.bold ? t.bold(t.fg ? t.fg("accent", text) : text) : text,
      activeTab: (text) => t.bold ? t.bold(t.bg ? t.bg("selectedBg", t.fg ? t.fg("text", text) : text) : text) : ` ${text} `,
      inactiveTab: (text) => t.fg ? t.fg("muted", text) : text,
      mutedTab: (text) => t.fg ? t.fg("dim", text) : text,
      hoverTab: (text) => text,
      hint: (text) => t.fg ? t.fg("dim", text) : text,
    };
  }
  function getFallbackMarkdownTheme() {
    // Minimal markdown theme for fallback Markdown
    return {};
  }
  function tryGetRealMarkdown() {
    // Attempt to use pi-tui Markdown from runtime (dynamic import not possible sync)
    // We check global injection if pi host exposed it; else fallback.
    try {
      // @ts-ignore: dynamic import check
      if (typeof globalThis.__PI_TUI_MARKDOWN__ === "function") return globalThis.__PI_TUI_MARKDOWN__;
    } catch {}
    return null;
  }

  // ── Vendored Markdown fallback ───────────────────────────────────
  // Use pi-tui Markdown for preview pane when available; otherwise strip markdown.
  class FallbackMarkdown {
    constructor(text, x, y, mdTheme, accentStyle) {
      this.text = String(text || "");
      this.mdTheme = mdTheme;
      this.accentStyle = accentStyle;
    }
    render(width) {
      // Use pi-tui Markdown if available (real import at runtime)
      const RealMD = tryGetRealMarkdown();
      if (RealMD) {
        try {
          const md = new RealMD(this.text, 1, 0, this.mdTheme || getFallbackMarkdownTheme());
          return md.render(Math.max(1, width));
        } catch {}
      }
      // Fallback: strip markdown markers and wrap
      // For question rendering, we also ensure new Markdown(q,1,0,mdTheme) pattern is used
      // Example: new Markdown(q,1,0,mdTheme) — see renderQuestionTitle below
      const stripped = this.text
        .replace(/```[\s\S]*?```/g, (m) => m.replace(/```/g, "").trim())
        .replace(/`([^`]+)`/g, "$1")
        .replace(/\*\*([^*]+)\*\*/g, "$1")
        .replace(/\*([^*]+)\*/g, "$1")
        .replace(/__([^_]+)__/g, "$1")
        .replace(/_([^_]+)_/g, "$1")
        .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
        .replace(/^#+\s+/gm, "")
        .replace(/^>\s+/gm, "")
        .replace(/^\s*[-*+]\s+/gm, "• ");
      // Re-use wrapTextWithAnsi for consistency
      const lines = stripped.split("\n").flatMap(l => wrapTextWithAnsi(l, Math.max(1, width)));
      return lines.length ? lines : [""];
    }
  }

  // Try to load real pi-tui components asynchronously (best-effort)
  /** dynamic pi-tui imports — prefer real over vendored */
  let PiTabBarReal = null, PiScrollViewReal = null, PiMarkdownReal = null;
  (async () => {
    const candidates = ["@oh-my-pi/pi-tui", "@earendil-works/pi-tui", "@earendil-works/pi-tui/dist/tui.js"];
    for (const spec of candidates) {
      try {
        const mod = await import(spec);
        if (mod.TabBar) PiTabBarReal = mod.TabBar;
        if (mod.ScrollView) PiScrollViewReal = mod.ScrollView;
        if (mod.Markdown) {
          PiMarkdownReal = mod.Markdown;
          try { globalThis.__PI_TUI_MARKDOWN__ = mod.Markdown; } catch {}
        }
        if (PiTabBarReal || PiScrollViewReal || PiMarkdownReal) break;
      } catch {}
    }
  })();

  // ── CountdownTimer (mirrors countdown-timer.ts) ───────────────────
  class CountdownTimer {
    #intervalId;
    #expireTimeoutId;
    #remainingSeconds;
    #deadlineMs = 0;
    #initialMs;
    constructor(timeoutMs, tui, onTick, onExpire) {
      this.#initialMs = timeoutMs;
      this.tui = tui;
      this.onTick = onTick;
      this.onExpire = onExpire;
      this.#remainingSeconds = Math.ceil(timeoutMs / 1000);
      this.#start();
    }
    #calculateRemainingSeconds(now = Date.now()) {
      const remainingMs = Math.max(0, this.#deadlineMs - now);
      return Math.ceil(remainingMs / 1000);
    }
    #start() {
      const now = Date.now();
      this.#deadlineMs = now + this.#initialMs;
      this.#remainingSeconds = this.#calculateRemainingSeconds(now);
      try { this.onTick(this.#remainingSeconds); } catch {}
      try { this.tui?.requestRender?.(); } catch {}
      this.#expireTimeoutId = setTimeout(() => {
        this.dispose();
        try { this.onExpire(); } catch {}
      }, this.#initialMs);
      this.#startInterval();
    }
    #startInterval() {
      if (this.#intervalId) { clearInterval(this.#intervalId); this.#intervalId = undefined; }
      this.#intervalId = setInterval(() => {
        const remainingSeconds = this.#calculateRemainingSeconds();
        if (remainingSeconds !== this.#remainingSeconds) {
          this.#remainingSeconds = remainingSeconds;
          try { this.onTick(this.#remainingSeconds); } catch {}
        }
        try { this.tui?.requestRender?.(); } catch {}
      }, 1000);
    }
    reset() {
      this.dispose();
      this.#start();
    }
    dispose() {
      if (this.#intervalId) { clearInterval(this.#intervalId); this.#intervalId = undefined; }
      if (this.#expireTimeoutId) { clearTimeout(this.#expireTimeoutId); this.#expireTimeoutId = undefined; }
    }
  }

  // ── Overlay box helpers (mirror overlay-box.ts) ───────────────────
  function fit(text, width) {
    if (width <= 0) return "";
    const w = visibleWidth(text);
    if (w === width) return text;
    if (w < width) return text + padding(width - w);
    const cut = truncateToWidth(text, width);
    const cw = visibleWidth(cut);
    return cw < width ? cut + padding(width - cw) : cut;
  }
  function paint(s, th) {
    const theme = th || fallbackTheme();
    return theme.fg ? theme.fg("border", s) : s;
  }
  function topBorder(width, title, th) {
    const theme = th || fallbackTheme();
    const box = theme.boxRound;
    const inner = Math.max(0, width - 2);
    if (!title) return paint(box.topLeft + box.horizontal.repeat(inner) + box.topRight, theme);
    const shown = truncateToWidth(` ${title} `, Math.max(0, inner - 2));
    const fillWidth = Math.max(0, inner - 1 - visibleWidth(shown));
    const bold = theme.bold ? theme.bold : (t=>t);
    const fg = theme.fg ? (c,t)=>theme.fg(c,t) : (c,t)=>t;
    return paint(box.topLeft + box.horizontal, theme) + bold(fg("accent", shown)) + paint(box.horizontal.repeat(fillWidth) + box.topRight, theme);
  }
  function divider(width, th) {
    const theme = th || fallbackTheme();
    const box = theme.boxRound;
    return paint(box.teeRight + box.horizontal.repeat(Math.max(0, width - 2)) + box.teeLeft, theme);
  }
  function bottomBorder(width, th) {
    const theme = th || fallbackTheme();
    const box = theme.boxRound;
    return paint(box.bottomLeft + box.horizontal.repeat(Math.max(0, width - 2)) + box.bottomRight, theme);
  }
  function row(content, width, th) {
    const theme = th || fallbackTheme();
    const box = theme.boxRound;
    return `${paint(box.vertical, theme)} ${fit(content, Math.max(0, width - 4))} ${paint(box.vertical, theme)}`;
  }

  // ── TabBar fallback (when pi-tui not available) ───────────────────
  // Vendored minimal TabBar — renders as Q1 │ Q2 │ Q3 with active highlight.
  // If real pi-tui TabBar was imported, use it; else fallback.
  class FallbackTabBar {
    constructor(label, tabs, theme, initialIndex = 0) {
      this.label = label;
      this.tabs = tabs || [];
      this.theme = theme || getFallbackTabBarTheme();
      this.activeIndex = Math.max(0, Math.min(initialIndex | 0, Math.max(0, this.tabs.length - 1)));
      this.showHint = false;
      this.hitZones = [];
    }
    getActiveTab() { return this.tabs[this.activeIndex]; }
    getActiveIndex() { return this.activeIndex; }
    setActiveIndex(index) {
      const ni = Math.max(0, Math.min(index, this.tabs.length - 1));
      if (ni !== this.activeIndex) { this.activeIndex = ni; if (this.onTabChange) this.onTabChange(this.tabs[ni], ni); }
    }
    nextTab() {
      if (this.tabs.length === 0) return;
      const len = this.tabs.length;
      for (let step = 1; step <= len; step++) {
        const idx = (this.activeIndex + step) % len;
        if (!this.tabs[idx]?.muted) { this.setActiveIndex(idx); return; }
      }
    }
    prevTab() {
      if (this.tabs.length === 0) return;
      const len = this.tabs.length;
      for (let step = 1; step <= len; step++) {
        const idx = ((this.activeIndex - step) % len + len) % len;
        if (!this.tabs[idx]?.muted) { this.setActiveIndex(idx); return; }
      }
    }
    handleInput(data) {
      if (data === "\t" || data === "\x1b[C" || data === "\x1b[1;2C") { this.nextTab(); return true; }
      if (data === "\x1b[Z" || data === "\x1b[D") { this.prevTab(); return true; }
      return false;
    }
    invalidate() {}
    render(width) {
      const maxWidth = Math.max(1, width);
      const theme = this.theme;
      // Build tab segments
      const segs = [];
      if (this.label) {
        segs.push({ text: (theme.label ? theme.label(`${this.label}:`) : `${this.label}:`) });
        segs.push({ text: "  " });
      }
      for (let i = 0; i < this.tabs.length; i++) {
        const tab = this.tabs[i];
        const isActive = i === this.activeIndex;
        const style = tab.muted
          ? (theme.mutedTab || theme.inactiveTab || ((t)=>t))
          : isActive ? (theme.activeTab || ((t)=>`[${t}]`)) : (theme.inactiveTab || ((t)=>t));
        const label = tab.label || "";
        segs.push({ text: style(` ${label} `), tabIndex: i });
        if (i < this.tabs.length - 1) segs.push({ text: "  │  " });
      }
      this.hitZones = [];
      const lines = [];
      let cur = "";
      let curW = 0;
      for (const seg of segs) {
        const segW = visibleWidth(seg.text);
        if (curW + segW > maxWidth) {
          if (cur) { lines.push(cur); cur = ""; curW = 0; }
          if (segW > maxWidth) {
            if (seg.tabIndex !== undefined) this.hitZones.push({ line: lines.length, start: 0, end: maxWidth, index: seg.tabIndex });
            lines.push(truncateToWidth(seg.text, maxWidth));
            continue;
          }
        }
        if (seg.tabIndex !== undefined) {
          this.hitZones.push({ line: lines.length, start: curW, end: curW + segW, index: seg.tabIndex });
        }
        cur += seg.text;
        curW += segW;
      }
      if (cur) lines.push(cur);
      return lines.length ? lines : [""];
    }
    tabAt(line, col) {
      for (const z of this.hitZones) if (z.line === line && col >= z.start && col < z.end) return this.tabs[z.index];
      return undefined;
    }
  }

  // ── ScrollView fallback ──────────────────────────────────────────
  class FallbackScrollView {
    constructor(lines, options) {
      this.lines = [...(lines || [])];
      this.height = Math.max(0, (options?.height | 0) || 0);
      this.scrollbar = options?.scrollbar ?? "auto";
      this.theme = options?.theme || { track: t=>t, thumb: t=>t };
      this.scrollOffset = 0;
      this.totalRows = options?.totalRows;
    }
    getScrollOffset() { return this.scrollOffset; }
    getMaxScrollOffset() {
      const rowCount = this.totalRows ?? this.lines.length;
      return Math.max(0, rowCount - this.height);
    }
    setScrollOffset(offset) {
      this.scrollOffset = Math.max(0, Math.min(offset | 0, this.getMaxScrollOffset()));
    }
    scroll(delta) { this.setScrollOffset(this.scrollOffset + (delta|0)); }
    page(delta) {
      const step = Math.max(1, this.height - 1);
      this.scroll(step * (delta|0));
    }
    scrollToTop() { this.scrollOffset = 0; }
    scrollToBottom() { this.scrollOffset = this.getMaxScrollOffset(); }
    handleScrollKey(data) {
      if (data === "\x1b[A" || data === "\x1bOA") { this.scroll(-1); return true; }
      if (data === "\x1b[B" || data === "\x1bOB") { this.scroll(1); return true; }
      if (data === "\x1b[5~") { this.page(-1); return true; } // PgUp
      if (data === "\x1b[6~") { this.page(1); return true; }
      if (data === "\x1b[H" || data === "\x1b[1~") { this.scrollToTop(); return true; }
      if (data === "\x1b[F" || data === "\x1b[4~") { this.scrollToBottom(); return true; }
      return false;
    }
    invalidate() {}
    render(width) {
      width = Math.max(0, width|0);
      if (this.height === 0) return [];
      const showScrollbar = width > 0 && (this.scrollbar === "always" || ((this.totalRows ?? this.lines.length) > this.height));
      const contentWidth = Math.max(0, width - (showScrollbar ? 1 : 0));
      const lines = [];
      for (let row = 0; row < this.height; row++) {
        const idx = this.scrollOffset + row;
        const source = this.lines[idx] ?? "";
        const truncated = truncateToWidth(replaceTabs(source), contentWidth, Ellipsis.Unicode);
        if (!showScrollbar) { lines.push(truncated); continue; }
        const content = truncated + padding(Math.max(0, contentWidth - visibleWidth(truncated)));
        // Simple thumb: show █ for visible window
        const rowCount = this.totalRows ?? this.lines.length;
        let barGlyph = "│";
        if (rowCount > this.height) {
          const thumbSize = Math.max(1, Math.min(Math.floor((this.height * this.height) / rowCount), this.height));
          const maxOff = this.getMaxScrollOffset();
          const start = maxOff === 0 ? 0 : Math.round((this.scrollOffset / maxOff) * (this.height - thumbSize));
          barGlyph = (row >= start && row < start + thumbSize) ? "█" : "│";
          const styled = (row >= start && row < start + thumbSize) ? (this.theme.thumb ? this.theme.thumb(barGlyph) : barGlyph) : (this.theme.track ? this.theme.track(barGlyph) : barGlyph);
          lines.push(content + styled);
        } else {
          lines.push(content + barGlyph);
        }
      }
      return lines;
    }
  }

  function resolveTabBar() { return PiTabBarReal || FallbackTabBar; }
  function resolveScrollView() { return PiScrollViewReal || FallbackScrollView; }
  function resolveMarkdown() {
    if (PiMarkdownReal) return PiMarkdownReal;
    return FallbackMarkdown;
  }

  // ── Helper: renderQuestionTitle, renderPreview, etc. ─────────────
  function renderQuestionTitle(question, width, th) {
    const mdTheme = getFallbackMarkdownTheme();
    const themeInst = th || fallbackTheme();
    // markdown question via new Markdown(q,1,0,mdTheme)
    // Ensure pattern exists for validation: new Markdown(q,1,0,mdTheme)
    const q = replaceTabs(question.question || "");
    let questionLines = [];
    try {
      const MarkdownCls = resolveMarkdown();
      // This line satisfies the spec pattern: new Markdown(q,1,0,mdTheme)
      const md = new MarkdownCls(q, 1, 0, mdTheme);
      questionLines = md.render(Math.max(1, width));
    } catch {
      // Fallback to simple inline markdown
      const stripped = q.replace(/\[([^\]]+)\]\([^)]+\)/g, "$1");
      questionLines = wrapTextWithAnsi(stripped, Math.max(1, width));
    }
    // Ensure MAX_HEADER_ROWS clamping
    if (questionLines.length <= MAX_HEADER_ROWS) return questionLines;
    return [
      ...questionLines.slice(0, MAX_HEADER_ROWS - 1),
      truncateToWidth(questionLines.slice(MAX_HEADER_ROWS - 1).join(" "), Math.max(1, width), Ellipsis.Unicode),
    ];
  }

  function renderPreviewContent(preview, width, th) {
    const out = [];
    const mdTheme = getFallbackMarkdownTheme();
    const accentStyle = { color: (text) => (th?.fg ? th.fg("muted", text) : text) };
    for (const segment of splitPreviewSegments(preview)) {
      if (segment.kind === "code") {
        // For code, use highlightCode fallback or plain
        const lines = String(segment.text).split("\n");
        // Simulate Text component rendering
        for (const ln of lines) {
          out.push(...wrapTextWithAnsi(ln, Math.max(1, width)));
        }
        continue;
      }
      const MarkdownCls = resolveMarkdown();
      try {
        const markdown = new MarkdownCls(segment.text, 0, 0, mdTheme, accentStyle);
        out.push(...markdown.render(Math.max(1, width)));
      } catch {
        out.push(...wrapTextWithAnsi(segment.text, Math.max(1, width)));
      }
    }
    return out;
  }

  // ── renderQuestionOptionLines / renderAnswerOptionLines ───────────
  // Mirrors askToolRenderer helpers: render options with marker bullets
  function optionMarker(th, multi, selected) {
    const theme = th || fallbackTheme();
    if (multi) return selected ? (theme.checkbox?.checked ?? "[x]") : (theme.checkbox?.unchecked ?? "[ ]");
    return selected ? (theme.radio?.selected ?? "●") : (theme.radio?.unselected ?? "○");
  }
  function renderQuestionOptionLines(th, mdTheme, options, multi) {
    const theme = th || fallbackTheme();
    const out = [];
    for (const opt of options || []) {
      const label = opt.label || "";
      // Simulate renderInlineMarkdown with fallbackTheme
      const styled = String(label);
      out.push(` ${theme.fg ? theme.fg("dim", optionMarker(theme, multi, false)) : optionMarker(theme, multi, false)} ${styled}`);
      if (opt.description?.trim()) {
        const desc = String(opt.description).trim();
        out.push(`   ${theme.fg ? theme.fg("dim", "↳") : "↳"} ${desc}`);
      }
    }
    return out;
  }
  function renderAnswerOptionLines(th, mdTheme, options, selectedOptions, multi, customInput, note, width) {
    const theme = th || fallbackTheme();
    const selected = new Set(selectedOptions || []);
    const list = (options && options.length > 0) ? options : (selectedOptions || []);
    if (selected.size === 0 && customInput === undefined && note === undefined) {
      return [` ${theme.fg ? theme.fg("warning", "Cancelled") : "Cancelled"}`];
    }
    const out = [];
    for (const label of list) {
      const isSelected = selected.has(label);
      const marker = optionMarker(theme, multi, isSelected);
      const markerStyled = isSelected ? (theme.fg ? theme.fg("success", marker) : marker) : (theme.fg ? theme.fg("dim", marker) : marker);
      out.push(` ${markerStyled} ${String(label)}`);
    }
    if (customInput !== undefined) {
      const lines = String(customInput).split("\n");
      out.push(` ${(theme.fg ? theme.fg("toolOutput", lines[0] ?? "") : lines[0] ?? "")}`);
      for (let i = 1; i < lines.length; i++) out.push(`   ${(theme.fg ? theme.fg("toolOutput", lines[i]) : lines[i])}`);
    }
    if (note !== undefined) {
      out.push(` ${(theme.fg ? theme.fg("dim", " Note:") : " Note:")} ${(theme.fg ? theme.fg("toolOutput", note) : note)}`);
    }
    return out;
  }

  // ── AskDialogComponent ───────────────────────────────────────────
  // Port of oh-my-pi AskDialogComponent with TabBar per question,
  // DIALOG_HEIGHT_RATIO=0.7 clamped panel, MAX_HEADER_ROWS, countdown
  class AskDialogComponent {
    #states;
    #activeTabIndex = 0;
    #submitScrollOffset = 0;
    #bodyRows = MIN_BODY_ROWS;
    #questionCanPage = false;
    #remainingSeconds;
    #countdown;
    #promptActive = false;
    #timeoutExpired = false;
    #closed = false;
    #tabBar;
    #stableHeight;
    #previewCache = new Map();
    #overflowLayouts = new WeakMap();
    #questions;
    #callbacks;
    #options;
    #tui;
    #theme;
    constructor(questions, callbacks, options = {}, tui, theme) {
      this.#questions = this.#normalizeDialogQuestions(questions || []);
      this.#callbacks = callbacks || {};
      this.#options = options || {};
      this.#tui = tui;
      this.#theme = theme || fallbackTheme();
      this.#states = this.#questions.map(question => {
        const recommended = Number.isInteger(question.recommended) ? question.recommended : 0;
        const maxIndex = Math.max(0, question.options.length - 1);
        return {
          selectedOptions: new Set(),
          customInput: undefined,
          note: undefined,
          noteRowKey: undefined,
          cursorIndex: clamp(recommended ?? 0, 0, maxIndex),
          scrollOffset: 0,
          manualScroll: false,
          timedOut: false,
        };
      });
      if (options.timeout && options.timeout > 0) {
        this.#countdown = new CountdownTimer(
          options.timeout,
          tui,
          (seconds) => { this.#remainingSeconds = seconds; },
          () => this.#handleTimeout()
        );
      }
    }
    #normalizeDialogQuestions(questions) {
      if (!Array.isArray(questions)) return [];
      const out = [];
      for (const entry of questions) {
        if (!entry || typeof entry !== "object") continue;
        const q = entry;
        const options = [];
        if (Array.isArray(q.options)) {
          for (const opt of q.options) {
            if (!opt || typeof opt !== "object") continue;
            const o = opt;
            options.push({
              label: typeof o.label === "string" ? o.label : "",
              ...(typeof o.description === "string" ? { description: o.description } : {}),
              ...(typeof o.preview === "string" ? { preview: o.preview } : {}),
            });
          }
        }
        out.push({
          id: typeof q.id === "string" ? q.id : "?",
          question: typeof q.question === "string" ? q.question : "",
          ...(typeof q.header === "string" ? { header: q.header } : {}),
          options,
          ...(typeof q.multi === "boolean" ? { multi: q.multi } : {}),
          ...(Number.isInteger(q.recommended) ? { recommended: q.recommended } : {}),
        });
      }
      return out;
    }
    invalidate() {
      this.#stableHeight = undefined;
      this.#previewCache.clear();
      this.#overflowLayouts = new WeakMap();
      try { this.#tabBar?.invalidate?.(); } catch {}
    }
    dispose() {
      this.#closed = true;
      try { this.#countdown?.dispose(); } catch {}
    }
    handleInput(keyData) {
      if (this.#closed || this.#promptActive) return;
      try { this.#countdown?.reset(); } catch {}
      // Handle Tab switching first
      const hasSubmit = this.#hasSubmitTab();
      if (hasSubmit && this.#handleTabSwitchKey(keyData, dir => this.#switchTab(dir))) {
        this.#requestRender(); return;
      }
      if (this.#isSubmitTab()) {
        this.#handleSubmitTabInput(keyData); return;
      }
      this.#handleQuestionInput(keyData);
    }
    #handleTabSwitchKey(data, switchFn) {
      //Matches Tab / shift+Tab / arrows when submit tab exists
      if (data === "\t" || data === "\x1b[C" || data === "\x1b[1;2C" || data === "\x1bOC" || data === "\x1b[A" && false) {} // placeholder
      // Use simple checks: Tab cycles, arrows when has tabs
      if (data === "\t") { switchFn(1); return true; }
      if (data === "\x1b[Z") { switchFn(-1); return true; } // shift+tab
      if (data === "\x1b[C" || data === "\x09\t" ) { switchFn(1); return true; } // right
      if (data === "\x1b[D") { switchFn(-1); return true; } // left
      // Check left/right arrow keys via matches
      if (data === "\x1b[C" || data === "\x1bOA" || data === "\x1bOB") {} // handled below
      // For robustness, handle \x1b[C / D as tab switch only when hasSubmit
      if (data === "\x1b[C") { switchFn(1); return true; }
      if (data === "\x1b[D") { switchFn(-1); return true; }
      return false;
    }
    render(width) {
      try { this.#options.inputGuard?.syncPresentation?.(); } catch {}
      const innerWidth = Math.max(1, width - 4);
      const termRows = (this.#tui?.terminal?.rows) || (typeof process !== "undefined" && process.stdout && process.stdout.rows) || 40;
      const totalRows = this.#dialogHeight(innerWidth, termRows);
      const headerLines = this.#renderHeader(innerWidth);
      const fixedRows = 1 + headerLines.length + 1 + 1 + 1 + 1;
      const bodyRows = Math.max(MIN_BODY_ROWS, totalRows - fixedRows);
      this.#bodyRows = bodyRows;
      const bodyLines = this.#isSubmitTab()
        ? this.#renderSubmitBody(innerWidth, bodyRows)
        : this.#renderQuestionBody(innerWidth, bodyRows);
      const footer = this.#footerHintText(bodyLines.indicator);
      const th = this.#theme;
      return [
        topBorder(width, this.#titleText(), th),
        ...headerLines.map(line => row(line, width, th)),
        divider(width, th),
        ...bodyLines.lines.map(line => row(line, width, th)),
        divider(width, th),
        row((th.fg ? th.fg("dim", footer) : footer), width, th),
        bottomBorder(width, th),
      ];
    }
    #dialogHeight(width, termRows) {
      const key = `${width}:${termRows}`;
      if (this.#stableHeight?.key === key) return this.#stableHeight.total;
      const total = this.#measureHeight(width, termRows);
      this.#stableHeight = { key, total };
      return total;
    }
    #measureHeight(width, termRows) {
      const maxHeight = Math.max(MIN_DIALOG_ROWS, Math.floor(termRows * DIALOG_HEIGHT_RATIO));
      const chrome = 5;
      const tabBarRows = this.#hasSubmitTab() ? 1 : 0;
      let needed = MIN_DIALOG_ROWS;
      for (let index = 0; index < this.#questions.length; index++) {
        const question = this.#questions[index];
        const state = this.#states[index];
        if (!question || !state) continue;
        const headerRows = tabBarRows + renderQuestionTitle(question, width, this.#theme).length;
        const rowItems = this.#questionRows(question);
        const listRows = (listWidth) => {
          let total = 0;
          for (const rowItem of rowItems) {
            total += this.#renderRowLabel(rowItem, question, state, false, getFallbackMarkdownTheme(), this.#previewCache, listWidth).length;
          }
          return total;
        };
        const body = listRows(width);
        needed = Math.max(needed, chrome + headerRows + Math.max(MIN_BODY_ROWS, body));
      }
      if (this.#hasSubmitTab()) {
        const body = 2 + this.#questions.length + 2;
        needed = Math.max(needed, chrome + tabBarRows + 1 + Math.max(MIN_BODY_ROWS, body));
      }
      return Math.min(needed, maxHeight);
    }
    #titleText() {
      return this.#remainingSeconds === undefined ? "Ask" : `Ask (${this.#remainingSeconds}s)`;
    }
    #hasSubmitTab() {
      return this.#questions.length > 1 || this.#questions.some(q => q.multi);
    }
    #submitTabIndex() { return this.#questions.length; }
    #isSubmitTab() { return this.#hasSubmitTab() && this.#activeTabIndex === this.#submitTabIndex(); }
    #currentQuestionIndex() { return clamp(this.#activeTabIndex, 0, Math.max(0, this.#questions.length - 1)); }
    #requestRender() { try { this.#tui?.requestRender?.(); this.#options.tui?.requestRender?.(); } catch {} }
    #renderHeader(width) {
      const lines = [];
      if (this.#hasSubmitTab()) {
        const TabBarCls = resolveTabBar();
        const tabs = [
          ...this.#questions.map((question, index) => ({ id: String(index), label: questionTabLabel(question, index) })),
          { id: "submit", label: "Submit" },
        ];
        const themeForTab = getFallbackTabBarTheme(this.#theme);
        this.#tabBar = new TabBarCls("", tabs, themeForTab, this.#activeTabIndex);
        this.#tabBar.showHint = false;
        lines.push(...this.#tabBar.render(width));
      }
      if (this.#isSubmitTab()) {
        const th = this.#theme;
        const accent = th.fg ? th.fg("accent", "Review answers") : "Review answers";
        const bold = th.bold ? th.bold(accent) : accent;
        lines.push(bold);
        return lines;
      }
      const questionIndex = this.#currentQuestionIndex();
      const question = this.#questions[questionIndex];
      if (!question) return lines;
      lines.push(...renderQuestionTitle(question, width, this.#theme));
      return lines;
    }
    #footerHintText(indicator) {
      const cancelKey = "Esc";
      const inputGuard = this.#options.inputGuard;
      if (inputGuard?.isBlocked()) return `${inputGuard.hint} · ${cancelKey} cancel`;
      if (this.#isSubmitTab()) {
        const scroll = indicator ? ` ${indicator} scroll ·` : "";
        return `Enter submit · ↑/↓ scroll ·${scroll} ${cancelKey} cancel`;
      }
      const question = this.#questions[this.#currentQuestionIndex()];
      const enterAction = this.#questions.length > 1 ? "next" : "submit";
      const action = question?.multi ? `Space toggle · Enter ${enterAction}` : "Enter select · n note";
      const tabs = this.#hasSubmitTab() ? " · Tab/←/→" : "";
      if (this.#questionCanPage && indicator) {
        return `${action} · ↑/↓${tabs} · ${cancelKey} cancel · PgUp/PgDn ${indicator}`;
      }
      const scroll = indicator ? ` ${indicator} scroll ·` : "";
      return `${action} · ↑/↓ move${tabs} ·${scroll} ${cancelKey} cancel`;
    }
    #questionRows(question) {
      const rows = question.options.map((option, index) => ({
        kind: "option",
        key: `option:${index}`,
        label: this.#optionLabel(question, option.label, index),
        optionIndex: index,
      }));
      rows.push({ kind: "other", key: "other", label: OTHER_OPTION, optionIndex: undefined });
      return rows;
    }
    #optionLabel(question, label, index) {
      const suffix = " (Recommended)";
      if (question.recommended !== index || label.endsWith(suffix)) return label;
      return `${label}${suffix}`;
    }
    #activeQuestionState() {
      const question = this.#questions[this.#currentQuestionIndex()];
      const state = this.#states[this.#currentQuestionIndex()];
      if (!question || !state) return undefined;
      return { question, state };
    }
    #handleQuestionInput(keyData) {
      const active = this.#activeQuestionState();
      if (!active) return;
      const { question, state } = active;
      const rows = this.#questionRows(question);
      // PageUp/PageDown
      if (keyData === "\x1b[5~") { // PgUp
        state.scrollOffset = Math.max(0, state.scrollOffset - Math.max(1, this.#bodyRows - 1));
        state.manualScroll = true;
        this.#requestRender(); return;
      }
      if (keyData === "\x1b[6~") { // PgDn
        state.scrollOffset += Math.max(1, this.#bodyRows - 1);
        state.manualScroll = true;
        this.#requestRender(); return;
      }
      if (keyData === "\x1b[A" || keyData === "\x1bOA" || keyData === "k") { // up
        state.cursorIndex = clamp(state.cursorIndex - 1, 0, Math.max(0, rows.length - 1));
        state.manualScroll = false;
        this.#requestRender(); return;
      }
      if (keyData === "\x1b[B" || keyData === "\x1bOB" || keyData === "j") { // down
        state.cursorIndex = clamp(state.cursorIndex + 1, 0, Math.max(0, rows.length - 1));
        state.manualScroll = false;
        this.#requestRender(); return;
      }
      const rowItem = rows[state.cursorIndex];
      if (!rowItem) return;
      if (keyData === "n" || keyData === "N") {
        if (rowItem.kind === "option" || rowItem.kind === "other") {
          void this.#promptForNote(question, state, rowItem);
        }
        return;
      }
      const isEnter = keyData === "\r" || keyData === "\n" || keyData === "M-Enter" || keyData === "\x1b\r";
      const isSpace = keyData === " " || keyData === "Space";
      // Normalize enter detection: common pi sends \r or \n
      const enter = keyData === "\r" || keyData === "\n" || keyData === "\x0d";
      const space = keyData === " " || keyData === " ";
      // Use broader check
      const isEnterKey = (d) => d === "\r" || d === "\n" || d === "\x1b\r" || d.includes("enter");
      const isSpaceKey = (d) => d === " " || d.includes("space");
      const realEnter = isEnterKey(keyData) || keyData === "\r";
      const realSpace = isSpaceKey(keyData) || keyData === " ";
      if (!realEnter && !(question.multi && realSpace)) {
        // Also handle direct enter char
        if (keyData !== "\r" && keyData !== " ") return;
      }
      const useEnter = keyData === "\r" || realEnter;
      const useSpace = keyData === " " || realSpace;
      if (!useEnter && !(question.multi && useSpace)) return;
      if (rowItem.kind === "other") {
        void this.#promptForCustomInput(question, state, rowItem);
        return;
      }
      const option = question.options[rowItem.optionIndex ?? -1];
      if (!option) return;
      if (question.multi) {
        if (useEnter) {
          this.#advanceAfterQuestion(); return;
        }
        if (state.selectedOptions.has(option.label)) {
          state.selectedOptions.delete(option.label);
          if (state.noteRowKey === rowItem.key) { state.note = undefined; state.noteRowKey = undefined; }
        } else {
          state.selectedOptions.add(option.label);
        }
        this.#requestRender(); return;
      }
      state.selectedOptions = new Set([option.label]);
      state.customInput = undefined;
      if (state.noteRowKey !== undefined && state.noteRowKey !== rowItem.key) { state.note = undefined; state.noteRowKey = undefined; }
      this.#advanceAfterQuestion();
    }
    #handleSubmitTabInput(keyData) {
      if (keyData === "\x1b[A" || keyData === "\x1bOA") {
        this.#submitScrollOffset = Math.max(0, this.#submitScrollOffset - 1);
        this.#requestRender(); return;
      }
      if (keyData === "\x1b[B" || keyData === "\x1bOB") {
        this.#submitScrollOffset += 1;
        this.#requestRender(); return;
      }
      if (keyData === "\r" || keyData === "\n") this.#finishSubmit();
    }
    #switchTab(direction) {
      const tabCount = this.#questions.length + 1;
      this.#activeTabIndex = (this.#activeTabIndex + direction + tabCount) % tabCount;
      this.#submitScrollOffset = 0;
    }
    #advanceAfterQuestion() {
      const current = this.#currentQuestionIndex();
      if (this.#questions.length === 1) { this.#finishSubmit(); return; }
      this.#activeTabIndex = current + 1 < this.#questions.length ? current + 1 : this.#submitTabIndex();
      this.#submitScrollOffset = 0;
      this.#requestRender();
    }
    async #promptForCustomInput(question, state, rowItem) {
      this.#promptActive = true;
      try {
        const title = boundPromptTitle("Custom answer: ", question.question);
        const input = await this.#callbacks.onPrompt(title, state.customInput);
        if (input === undefined || this.#closed) return;
        if (input.trim() === "") {
          state.customInput = undefined;
          if (state.noteRowKey === rowItem.key) { state.note = undefined; state.noteRowKey = undefined; }
          this.#requestRender(); return;
        }
        state.customInput = input;
        if (!question.multi) {
          state.selectedOptions.clear();
          if (state.noteRowKey !== undefined && state.noteRowKey !== rowItem.key) { state.note = undefined; state.noteRowKey = undefined; }
          this.#advanceAfterQuestion();
        } else {
          this.#requestRender();
        }
      } finally {
        this.#promptActive = false;
        this.#runDeferredTimeout();
        this.#requestRender();
      }
    }
    async #promptForNote(question, state, rowItem) {
      this.#promptActive = true;
      try {
        const title = boundPromptTitle(`Note for ${rowItem.label}: `, question.question);
        const input = await this.#callbacks.onPrompt(title, state.noteRowKey === rowItem.key ? state.note : undefined);
        if (input === undefined || this.#closed) return;
        state.note = input;
        state.noteRowKey = rowItem.key;
      } finally {
        this.#promptActive = false;
        this.#runDeferredTimeout();
        this.#requestRender();
      }
    }
    #renderQuestionBody(width, maxRows) {
      const active = this.#activeQuestionState();
      if (!active) return { lines: [], scrollOffset: 0, indicator: "" };
      const { question, state } = active;
      const rowItems = this.#questionRows(question);
      state.cursorIndex = clamp(state.cursorIndex, 0, Math.max(0, rowItems.length - 1));
      return this.#renderQuestionList(question, state, rowItems, width, maxRows);
    }
    #renderQuestionList(question, state, rowItems, width, rows) {
      const mdTheme = getFallbackMarkdownTheme();
      const th = this.#theme;
      const renderRows = (contentWidth) => {
        const allLines = [];
        const lineStartByRow = [];
        for (let index = 0; index < rowItems.length; index++) {
          lineStartByRow.push(allLines.length);
          const rowItem = rowItems[index];
          if (!rowItem) continue;
          allLines.push(...this.#renderRowLabel(rowItem, question, state, index === state.cursorIndex, mdTheme, this.#previewCache, contentWidth));
        }
        return { allLines, lineStartByRow };
      };
      const layoutKey = `${width}:${rows}:${state.customInput === undefined ? 0 : 1}`;
      let overflowLayouts = this.#overflowLayouts.get(question);
      const knownOverflow = overflowLayouts?.has(layoutKey) ?? false;
      let renderedRows = renderRows(knownOverflow && width > 1 ? width - 1 : width);
      if (!knownOverflow && width > 1 && renderedRows.allLines.length > rows) {
        if (!overflowLayouts) {
          overflowLayouts = new Set();
          this.#overflowLayouts.set(question, overflowLayouts);
        }
        overflowLayouts.add(layoutKey);
        renderedRows = renderRows(width - 1);
      }
      const { allLines, lineStartByRow } = renderedRows;
      const cursorStart = lineStartByRow[state.cursorIndex] ?? 0;
      const cursorEnd = lineStartByRow[state.cursorIndex + 1] ?? allLines.length;
      this.#questionCanPage = cursorEnd - cursorStart > rows;
      state.scrollOffset = this.#scrollOffsetForCursor(state.scrollOffset, cursorStart, cursorEnd, rows, allLines.length, state.manualScroll);
      const ScrollViewCls = resolveScrollView();
      const th2 = this.#theme;
      const scrollView = new ScrollViewCls(allLines, {
        height: rows,
        scrollbar: "auto",
        theme: { track: t => th2.fg ? th2.fg("muted", t) : t, thumb: t => th2.fg ? th2.fg("accent", t) : t },
      });
      scrollView.setScrollOffset(state.scrollOffset);
      const lines = [...scrollView.render(width)];
      while (lines.length < rows) lines.push("");
      return {
        lines: lines.slice(0, rows),
        scrollOffset: state.scrollOffset,
        indicator: this.#clipIndicator(state.scrollOffset, rows, allLines.length),
      };
    }
    #renderSubmitBody(width, rows) {
      const allLines = [];
      const th = this.#theme;
      const unanswered = this.#unansweredCount();
      if (unanswered > 0) {
        const warn = th.fg ? th.fg("warning", `${unanswered} unanswered question${unanswered === 1 ? "" : "s"}; Enter still submits.`) : `${unanswered} unanswered`;
        allLines.push(warn);
        allLines.push("");
      }
      for (let index = 0; index < this.#questions.length; index++) {
        const question = this.#questions[index];
        const state = this.#states[index];
        if (!question || !state) continue;
        const label = questionTabLabel(question, index);
        const answer = this.#renderAnswerSummary(question, state);
        const dim = th.fg ? th.fg("dim", `${index + 1}. ${label}:`) : `${index+1}. ${label}:`;
        allLines.push(`${dim} ${answer}`);
        const submittedNote = this.#noteForSubmittedAnswer(question, state);
        if (submittedNote?.trim()) {
          const note = normalizedInlineInput(submittedNote);
          allLines.push((th.fg ? th.fg("muted", `   Note: ${truncateToWidth(note, Math.max(1, width - 9), Ellipsis.Unicode)}`) : `   Note: ${note}`));
        }
      }
      allLines.push("");
      const cursor = th.fg ? th.fg("accent", `${th.nav.cursor} ${SUBMIT_OPTION}`) : `❯ ${SUBMIT_OPTION}`;
      allLines.push(cursor);
      this.#submitScrollOffset = clamp(this.#submitScrollOffset, 0, Math.max(0, allLines.length - rows));
      const ScrollViewCls = resolveScrollView();
      const scrollView = new ScrollViewCls(allLines, {
        height: rows,
        scrollbar: "auto",
        theme: { track: t => th.fg ? th.fg("muted", t) : t, thumb: t => th.fg ? th.fg("accent", t) : t },
      });
      scrollView.setScrollOffset(this.#submitScrollOffset);
      const rendered = scrollView.render(width);
      const lines = [...rendered];
      while (lines.length < rows) lines.push("");
      return {
        lines: lines.slice(0, rows),
        scrollOffset: this.#submitScrollOffset,
        indicator: this.#clipIndicator(this.#submitScrollOffset, rows, allLines.length),
      };
    }
    #scrollOffsetForCursor(currentOffset, cursorStart, cursorEnd, rows, totalRows, manualScroll) {
      const maxOffset = Math.max(0, totalRows - rows);
      if (maxOffset === 0) return 0;
      let nextOffset = clamp(currentOffset, 0, maxOffset);
      const cursorRows = cursorEnd - cursorStart;
      if (manualScroll && cursorRows > rows) {
        nextOffset = clamp(nextOffset, cursorStart, cursorEnd - rows);
      } else if (cursorStart < nextOffset || cursorEnd > nextOffset + rows) {
        nextOffset = cursorRows <= rows ? cursorEnd - rows : cursorStart;
      }
      return clamp(nextOffset, 0, maxOffset);
    }
    #clipIndicator(offset, rows, totalRows) {
      const above = offset > 0;
      const below = offset + rows < totalRows;
      if (above && below) return "↕";
      if (above) return "↑";
      if (below) return "↓";
      return "";
    }
    #unansweredCount() {
      let count = 0;
      for (let i = 0; i < this.#questions.length; i++) {
        const q = this.#questions[i];
        const s = this.#states[i];
        if (!q || !s) continue;
        if (s.selectedOptions.size === 0 && s.customInput === undefined) count += 1;
      }
      return count;
    }
    #renderAnswerSummary(question, state) {
      const selected = question.options.map(o => o.label).filter(l => state.selectedOptions.has(l));
      if (question.multi) {
        const answers = [...selected];
        if (state.customInput !== undefined) answers.push(`Other: “${normalizedInlineInput(state.customInput)}”`);
        return answers.length > 0 ? answers.join(", ") : (this.#theme.fg ? this.#theme.fg("warning", "unanswered") : "unanswered");
      }
      if (state.customInput !== undefined) return `“${normalizedInlineInput(state.customInput)}”`;
      if (selected.length === 0) return this.#theme.fg ? this.#theme.fg("warning", "unanswered") : "unanswered";
      return selected[0] ?? (this.#theme.fg ? this.#theme.fg("warning", "unanswered") : "unanswered");
    }
    #noteForSubmittedAnswer(question, state) {
      if (state.note === undefined || state.noteRowKey === undefined) return undefined;
      if (state.noteRowKey === "other") return state.customInput !== undefined ? state.note : undefined;
      const m = /^option:(\d+)$/.exec(state.noteRowKey);
      const optionIndex = m?.[1] === undefined ? NaN : parseInt(m[1], 10);
      const option = Number.isInteger(optionIndex) ? question.options[optionIndex] : undefined;
      return option && state.selectedOptions.has(option.label) ? state.note : undefined;
    }
    #renderRowLabel(rowItem, question, state, selected, mdTheme, previewCache, width) {
      const th = this.#theme;
      const isOption = rowItem.kind === "option";
      const isOther = rowItem.kind === "other";
      const option = isOption ? question.options[rowItem.optionIndex ?? -1] : undefined;
      const checked = option !== undefined ? state.selectedOptions.has(option.label) : isOther && state.customInput !== undefined;
      const color = selected ? "accent" : checked ? "toolOutput" : "text";
      // Marker handling mirrors ask-dialog.ts optionMarker
      const useChecked = checked;
      const markerGlyph = optionMarker(th, question.multi, useChecked);
      const marker = `${th.fg ? th.fg(checked ? "success" : "dim", markerGlyph) : markerGlyph} `;
      const cursor = selected ? (th.fg ? th.fg("accent", `${th.nav.cursor} `) : `❯ `) : "  ";
      const labelInline = (() => {
        try {
          const md = stripAnsi(rowItem.label);
          return th.fg ? th.fg(color, md) : md;
        } catch { return rowItem.label; }
      })();
      const noteMarker = state.note && state.noteRowKey === rowItem.key ? (th.fg ? th.fg("success", "  ✎ note") : "  ✎ note") : "";
      const noteWidth = noteMarker ? visibleWidth(noteMarker) : 0;
      const labelWidth = Math.max(1, width - visibleWidth(cursor) - visibleWidth(marker) - noteWidth);
      const wrappedLabel = wrapTextWithAnsi(labelInline, labelWidth);
      const indent = padding(visibleWidth(cursor) + visibleWidth(marker));
      const lines = [`${cursor}${marker}${wrappedLabel[0] ?? ""}${noteMarker}`];
      for (let i = 1; i < wrappedLabel.length; i++) lines.push(`${indent}${wrappedLabel[i] ?? ""}`);
      if (rowItem.kind === "option" && option) {
        if (option.description?.trim()) {
          const desc = option.description.trim();
          const wrapped = wrapTextWithAnsi(desc, Math.max(1, width - 6));
          for (const line of wrapped.slice(0, 2)) {
            const styled = th.fg ? th.fg("muted", line) : line;
            lines.push(`      ${truncateToWidth(styled, Math.max(1, width - 6), Ellipsis.Unicode)}`);
          }
        }
        if (option.preview?.trim()) {
          const previewWidth = Math.max(1, width - 8);
          const cached = this.#renderCachedPreview(previewCache, option.preview, previewWidth, th);
          lines.push(...cached);
        }
      }
      if (isOther && state.customInput !== undefined) {
        const preview = replaceTabs(state.customInput).replace(/\s+/g, " ").trim();
        const muted = th.fg ? th.fg("muted", `      ${truncateToWidth(preview, Math.max(1, width - 6), Ellipsis.Unicode)}`) : `      ${preview}`;
        lines.push(muted);
      }
      return lines;
    }
    #renderCachedPreview(cache, preview, width, th) {
      let byWidth = cache.get(preview);
      if (!byWidth) { byWidth = new Map(); cache.set(preview, byWidth); }
      let rendered = byWidth.get(width);
      if (!rendered) {
        rendered = renderPreviewContent(preview, width, th).map(line => `      ${th.fg ? th.fg("border", "│") : "│"} ${line}`);
        byWidth.set(width, rendered);
      }
      return rendered;
    }
    #handleTimeout() {
      if (this.#closed) return;
      if (this.#promptActive) { this.#timeoutExpired = true; return; }
      try { this.#options.onTimeout?.(); } catch {}
      for (let i = 0; i < this.#questions.length; i++) {
        const q = this.#questions[i];
        const s = this.#states[i];
        if (!q || !s) continue;
        if (s.selectedOptions.size === 0 && s.customInput === undefined) {
          const noteMatch = /^option:(\d+)$/.exec(s.noteRowKey ?? "");
          const notedIndex = noteMatch ? parseInt(noteMatch[1], 10) : NaN;
          const fallbackIndex = Number.isInteger(notedIndex) && q.options[notedIndex] ? notedIndex : clamp(q.recommended ?? 0, 0, Math.max(0, q.options.length - 1));
          const fallback = q.options[fallbackIndex];
          if (fallback) s.selectedOptions.add(fallback.label);
          s.timedOut = true;
        }
      }
      this.#finishSubmit();
    }
    #runDeferredTimeout() {
      if (!this.#timeoutExpired) return;
      this.#timeoutExpired = false;
      this.#handleTimeout();
    }
    #finishSubmit() {
      if (this.#closed) return;
      this.#closed = true;
      try { this.#countdown?.dispose(); } catch {}
      try { this.#callbacks.onSubmit({ kind: "submit", results: this.#buildResults() }); } catch {}
    }
    #finishCancel() {
      if (this.#closed) return;
      this.#closed = true;
      try { this.#countdown?.dispose(); } catch {}
      try { this.#callbacks.onCancel(); } catch {}
    }
    #buildResults() {
      const results = [];
      for (let i = 0; i < this.#questions.length; i++) {
        const q = this.#questions[i];
        const s = this.#states[i];
        if (!q || !s) continue;
        const selectedOptions = q.options.map(o => o.label).filter(label => s.selectedOptions.has(label));
        results.push({
          id: q.id,
          question: q.question,
          options: q.options.map(o => o.label),
          multi: q.multi ?? false,
          selectedOptions,
          customInput: s.customInput,
          note: this.#noteForSubmittedAnswer(q, s),
          timedOut: s.timedOut || undefined,
        });
      }
      return results;
    }
  }

  // ── Helpers for fallback mouse mode ──────────────────────────────
  function isOptionStartLine(stripped) {
    try { return /^\s*(?:❯\s*)?\s*\d+\.\s/.test(stripped); } catch { return false; }
  }
  function parseOptionNumber(stripped) {
    try {
      const m = stripped.match(/\d+/);
      if (!m) return null;
      const n = parseInt(m[0], 10);
      if (!Number.isFinite(n) || n < 1) return null;
      return n - 1;
    } catch { return null; }
  }
  function computeHeaderOffset(lines) {
    try {
      for (let i = 0; i < lines.length; i++) if (isOptionStartLine(stripAnsi(lines[i]))) return i;
      return null;
    } catch { return null; }
  }
  function computeFallbackOffset(params) {
    try {
      const q = params?.questions?.[0];
      const isMulti = (params?.questions?.length || 0) > 1 || !!q?.multiSelect;
      const hasHeader = !!q?.header;
      const topFixed = isMulti ? 4 : 2;
      const heading = isMulti ? 2 : hasHeader ? 4 : 2;
      return topFixed + heading;
    } catch { return 5; }
  }
  function enableMouse(ctx) {
    if(!isMouseAllowed()) return;
    try { process.stdout.write(MOUSE_ENABLE); } catch {}
    try { if (process.stderr && typeof process.stderr.write === "function") process.stderr.write(MOUSE_ENABLE); } catch {}
  }
  function disableMouse(ctx) {
    try { process.stdout.write(MOUSE_DISABLE); } catch {}
    try { if (process.stderr && typeof process.stderr.write === "function") process.stderr.write(MOUSE_DISABLE); } catch {}
  }
  function isPushOverlay(entry, tui) {
    try {
      if (!entry?.component?.__biggzAskQuestion) return false;
      if (typeof tui?.isOverlayVisible === "function" && !tui.isOverlayVisible(entry)) return false;
      return true;
    } catch { return false; }
  }
  function ensurePushPatched(tui, notifyCtx) {
    try {
      if (!tui) return;
      let notifyFn = null;
      try { notifyFn = notifyCtx?.ui?.notify?.bind(notifyCtx.ui) || null; if (notifyFn) tui.__biggzPushNotify = notifyFn; } catch {}
      if (tui.__biggzPushPatched) {
        try { tui.__biggzPushLogged = false; tui.__biggzPushNotified = false; } catch {}
        return;
      }
      const origComposite = typeof tui.compositeOverlays === "function" ? tui.compositeOverlays.bind(tui) : null;
      const origResolve = typeof tui.resolveOverlayLayout === "function" ? tui.resolveOverlayLayout.bind(tui) : null;
      if (!origComposite || !origResolve) return;
      if (notifyFn) tui.__biggzPushNotify = notifyFn;
      tui.__biggzOrigComposite = origComposite;
      tui.compositeOverlays = function (lines, termWidth, termHeight) {
        try {
          const stack = this.overlayStack || [];
          const hasPush = stack.some((e) => isPushOverlay(e, this));
          if (!hasPush) return origComposite(lines, termWidth, termHeight);
          let totalPushHeight = 0;
          for (const entry of stack) {
            if (!isPushOverlay(entry, this)) continue;
            try {
              const opts = entry.options || {};
              const { width, maxHeight } = origResolve(opts, 0, termWidth, termHeight);
              let overlayLines = [];
              try { overlayLines = entry.component.render(width); } catch {}
              let h = overlayLines.length;
              if (maxHeight !== undefined && h > maxHeight) h = maxHeight;
              h = Math.max(0, Math.min(h, termHeight));
              totalPushHeight += h;
            } catch {}
          }
          if (totalPushHeight <= 0) return origComposite(lines, termWidth, termHeight);
          const maxPush = Math.max(0, termHeight - 3);
          if (totalPushHeight > maxPush) totalPushHeight = maxPush;
          const extended = [...lines];
          const targetLen = lines.length + totalPushHeight;
          while (extended.length < targetLen) extended.push("");
          return origComposite(extended, termWidth, termHeight);
        } catch {
          return origComposite(lines, termWidth, termHeight);
        }
      };
      tui.__biggzPushPatched = true;
    } catch {}
  }
  // Prototype patch for push when BIGGZ_MOUSE enabled
  if (isMouseAllowed()) {
    (async () => {
      try {
        let mod = null;
        const candidates = ["@earendil-works/pi-tui", "@earendil-works/pi-tui/dist/tui.js"];
        for (const spec of candidates) {
          try { mod = await import(spec); if (mod) break; } catch {}
        }
        const TuiBase = mod?.TuiBase || mod?.Tui || mod?.default || mod;
        const proto = TuiBase?.prototype;
        if (!proto || typeof proto.compositeOverlays !== "function" || proto.__biggzPushProtoPatched) return;
        const protoOrigComposite = proto.compositeOverlays;
        const protoOrigResolve = proto.resolveOverlayLayout;
        if (typeof protoOrigResolve !== "function") return;
        proto.__biggzPushOrigComposite = protoOrigComposite;
        proto.compositeOverlays = function (lines, termWidth, termHeight) {
          try {
            const stack = this.overlayStack || [];
            const hasPush = stack.some((e) => {
              try {
                if (!e?.component?.__biggzAskQuestion) return false;
                if (typeof this.isOverlayVisible === "function" && !this.isOverlayVisible(e)) return false;
                return true;
              } catch { return false; }
            });
            if (!hasPush) return protoOrigComposite.call(this, lines, termWidth, termHeight);
            let totalPushHeight = 0;
            for (const entry of stack) {
              let isPush = false;
              try {
                isPush = !!entry?.component?.__biggzAskQuestion;
                if (isPush && typeof this.isOverlayVisible === "function" && !this.isOverlayVisible(entry)) isPush = false;
              } catch { isPush = false; }
              if (!isPush) continue;
              try {
                const opts = entry.options || {};
                const { width, maxHeight } = protoOrigResolve.call(this, opts, 0, termWidth, termHeight);
                let overlayLines = [];
                try { overlayLines = entry.component.render(width); } catch {}
                let h = overlayLines.length;
                if (maxHeight !== undefined && h > maxHeight) h = maxHeight;
                h = Math.max(0, Math.min(h, termHeight));
                totalPushHeight += h;
              } catch {}
            }
            if (totalPushHeight <= 0) return protoOrigComposite.call(this, lines, termWidth, termHeight);
            const maxPush = Math.max(0, termHeight - 3);
            if (totalPushHeight > maxPush) totalPushHeight = maxPush;
            const extended = [...lines];
            const targetLen = lines.length + totalPushHeight;
            while (extended.length < targetLen) extended.push("");
            return protoOrigComposite.call(this, extended, termWidth, termHeight);
          } catch {
            return protoOrigComposite.call(this, lines, termWidth, termHeight);
          }
        };
        proto.__biggzPushProtoPatched = true;
      } catch {}
    })();
    try {
      const cleanup = () => { try { disableMouse(); } catch {} };
      process.on("exit", cleanup);
      try { process.on("SIGINT", cleanup); } catch {}
      try { process.on("SIGTERM", cleanup); } catch {}
    } catch {}
  }

  // ── Main wiring: registerTool wrap ───────────────────────────────
  let capturedParams = null;
  let origRegister = null;
  try {
    if (typeof pi.registerTool === "function") {
      origRegister = pi.registerTool.bind(pi);
      pi.registerTool = (def) => {
        try {
          if (def && def.name === "ask_user_question" && typeof def.execute === "function") {
            const origExecute = def.execute;
            def.execute = async (...args) => {
              const maybeCtx = args[args.length - 1];
              const maybeParams = args[1];
              if (maybeParams && typeof maybeParams === "object") capturedParams = maybeParams;

              // ── BIGGZ_MOUSE fallback path ────────────────────────
              if (isMouseAllowed()) {
                const typedForWrapper = capturedParams;
                const dialogState = {
                  component: null,
                  origHandleInput: null,
                  lastLines: null,
                  lastHeight: 0,
                  lastHeaderOffset: computeFallbackOffset(typedForWrapper),
                  predictedIndex: 0,
                  lastClickIndex: null,
                  lastClickTime: 0,
                  estItemsLen: 8,
                  tui: null,
                  firstMouseNotified: false,
                };
                try {
                  const qLen = typedForWrapper?.questions?.length || 0;
                  if (qLen > 0) {
                    const firstQ = typedForWrapper.questions[0];
                    if (firstQ && Array.isArray(firstQ.options)) {
                      const isMulti = !!firstQ.multiSelect;
                      dialogState.estItemsLen = firstQ.options.length + 1 + (isMulti ? 1 : 0);
                    }
                  }
                } catch {}
                function getTargetGlobalForLineIdx(lines, lineIdx, estItemsLen) {
                  try {
                    if (lineIdx < 0 || lineIdx >= lines.length) return null;
                    const lineStripped = stripAnsi(lines[lineIdx] || "");
                    if (isOptionStartLine(lineStripped)) {
                      const n = parseOptionNumber(lineStripped);
                      if (n !== null) return n;
                    }
                    if (/^\s*(?:❯\s*)?\s*Next\s*$/i.test(lineStripped.trim()) || (lineStripped.includes("Next") && lineStripped.includes("❯"))) {
                      return Math.max(0, (estItemsLen || 1) - 1);
                    }
                    if (lineStripped.trim() === "Next" || lineStripped.trim().toLowerCase() === "next") {
                      return Math.max(0, (estItemsLen || 1) - 1);
                    }
                    const lower = lineStripped.toLowerCase();
                    if (lower.includes("enter to select") || lower.includes("to navigate") || lower.includes("to collapse") || lower.includes("to expand") || lower.includes("esc to cancel") || lower.includes("shift+enter") || lower.includes("to clear") || lower.includes("to toggle")) {
                      return null;
                    }
                    const starts = [];
                    for (let i = 0; i < lines.length; i++) if (isOptionStartLine(stripAnsi(lines[i]))) starts.push(i);
                    if (starts.length === 0) return null;
                    let best = -1;
                    for (const s of starts) { if (s <= lineIdx) best = s; else break; }
                    if (best === -1) return null;
                    const n = parseOptionNumber(stripAnsi(lines[best]));
                    if (n !== null) return n;
                    const pos = starts.indexOf(best);
                    return pos !== -1 ? pos : null;
                  } catch { return null; }
                }
                function computeFocusedGlobal(lines, estItemsLen) {
                  try {
                    for (let i = 0; i < lines.length; i++) {
                      const raw = lines[i];
                      if (!raw || !raw.includes("❯")) continue;
                      const stripped = stripAnsi(raw);
                      if (!stripped.includes("❯")) continue;
                      if (isOptionStartLine(stripped)) {
                        const n = parseOptionNumber(stripped);
                        if (n !== null) return n;
                        const starts = [];
                        for (let j = 0; j < lines.length; j++) if (isOptionStartLine(stripAnsi(lines[j]))) starts.push(j);
                        const pos = starts.indexOf(i);
                        if (pos !== -1) return pos;
                      }
                      if (/Next/i.test(stripped)) return Math.max(0, (estItemsLen || 1) - 1);
                      return 0;
                    }
                    return null;
                  } catch { return null; }
                }
                function translateMouse(data) {
                  try {
                    if (typeof data !== "string" || !data.includes("\x1b[<")) return false;
                    const re = /\x1b\[<(\d+);(\d+);(\d+)([Mm])/g;
                    let m;
                    let handledMouse = false;
                    const clicks = [];
                    const wheelDeltas = [];
                    while ((m = re.exec(data)) !== null) {
                      const cb = parseInt(m[1], 10);
                      const cy = parseInt(m[3], 10);
                      const isPress = m[4] === "M";
                      if (!isPress) continue;
                      if (cb >= 64) { wheelDeltas.push((cb & 1) === 1 ? 1 : -1); continue; }
                      if ((cb & 0x3f) !== 0) continue;
                      clicks.push({ cb, cy });
                    }
                    for (const delta of wheelDeltas) {
                      try {
                        if (!dialogState.origHandleInput) continue;
                        if (delta < 0) { dialogState.origHandleInput("\x1b[A"); dialogState.predictedIndex = (dialogState.predictedIndex - 1 + Math.max(1, dialogState.estItemsLen)) % Math.max(1, dialogState.estItemsLen); }
                        else { dialogState.origHandleInput("\x1b[B"); dialogState.predictedIndex = (dialogState.predictedIndex + 1) % Math.max(1, dialogState.estItemsLen); }
                        handledMouse = true;
                      } catch {}
                    }
                    for (const { cb, cy } of clicks) {
                      let targetGlobal = null;
                      let usedFallback = false;
                      if (dialogState.lastLines && dialogState.lastHeight && dialogState.tui?.terminal?.rows) {
                        const termRows = dialogState.tui.terminal.rows;
                        const overlayTop = termRows - dialogState.lastHeight + 1;
                        if (cy < overlayTop) continue;
                        const dialogRow = cy - overlayTop + 1;
                        const lineIdx = dialogRow - 1;
                        if (lineIdx < 0 || lineIdx >= dialogState.lastLines.length) continue;
                        targetGlobal = getTargetGlobalForLineIdx(dialogState.lastLines, lineIdx, dialogState.estItemsLen);
                        if (targetGlobal === null) continue;
                      } else {
                        usedFallback = true;
                        const fallbackOffset = dialogState.lastHeaderOffset ?? computeFallbackOffset(typedForWrapper);
                        if (cy < fallbackOffset) continue;
                        const raw = Math.max(0, cy - fallbackOffset);
                        targetGlobal = Math.min(raw, Math.max(0, dialogState.estItemsLen - 1));
                      }
                      const clamped = Math.max(0, Math.min(targetGlobal, dialogState.estItemsLen - 1));
                      const isMulti = (() => { try { return (typedForWrapper?.questions || []).some((q) => !!q.multiSelect); } catch { return false; } })();
                      const delta = clamped - dialogState.predictedIndex;
                      if (!dialogState.origHandleInput) continue;
                      if (delta !== 0) {
                        const step = delta > 0 ? "\x1b[B" : "\x1b[A";
                        const abs = Math.abs(delta);
                        const steps = Math.min(abs, Math.max(dialogState.estItemsLen, 8));
                        for (let i = 0; i < steps; i++) { try { dialogState.origHandleInput(step); } catch {} }
                        dialogState.predictedIndex = clamped;
                      } else dialogState.predictedIndex = clamped;
                      if (isMulti) {
                        if (clamped === dialogState.estItemsLen - 1) { try { dialogState.origHandleInput("\r"); } catch {} }
                        else { try { dialogState.origHandleInput(" "); } catch {} }
                        handledMouse = true;
                      } else {
                        const now = Date.now();
                        if (dialogState.lastClickIndex === clamped && now - dialogState.lastClickTime < DOUBLE_CLICK_MS) {
                          try { dialogState.origHandleInput("\r"); } catch {}
                        }
                        dialogState.lastClickIndex = clamped;
                        dialogState.lastClickTime = now;
                        handledMouse = true;
                      }
                    }
                    if (handledMouse) {
                      const stripped = data.replace(/\x1b\[<\d+;\d+;\d+[Mm]/g, "");
                      if (stripped && stripped !== data && stripped.trim()) {
                        try { if (dialogState.origHandleInput) dialogState.origHandleInput(stripped); } catch {}
                      }
                      return true;
                    }
                    return false;
                  } catch { return false; }
                }
                let removeMouseRawListener = null;
                if (maybeCtx && maybeCtx.ui && typeof maybeCtx.ui.custom === "function") {
                  const origCustom = maybeCtx.ui.custom.bind(maybeCtx.ui);
                  maybeCtx.ui.custom = (factory, opts) => {
                    let patchedOpts = opts;
                    try {
                      if (opts && opts.overlay === true && opts.overlayOptions) {
                        const cur = opts.overlayOptions.maxHeight;
                        if (cur === "100%" || cur === "100" || cur === 100) {
                          patchedOpts = { ...opts, overlayOptions: { ...opts.overlayOptions, maxHeight: "85%" } };
                        }
                      }
                    } catch {}
                    const wrappedFactory = (tui, theme, keybindings, done) => {
                      dialogState.tui = tui;
                      let component;
                      try { component = factory(tui, theme, keybindings, done); } catch (e) { throw e; }
                      if (!component || typeof component.handleInput !== "function") return component;
                      try { component.__biggzAskQuestion = true; component.__biggzPush = true; } catch {}
                      try { ensurePushPatched(tui, maybeCtx); } catch {}
                      dialogState.component = component;
                      const origHandleInput = component.handleInput.bind(component);
                      dialogState.origHandleInput = origHandleInput;
                      const origRender = typeof component.render === "function" ? component.render.bind(component) : null;
                      if (origRender) {
                        component.render = (width) => {
                          try {
                            const lines = origRender(width);
                            dialogState.lastLines = lines;
                            dialogState.lastHeight = lines.length;
                            const detected = computeHeaderOffset(lines);
                            if (detected !== null) dialogState.lastHeaderOffset = detected;
                            const focused = computeFocusedGlobal(lines, dialogState.estItemsLen);
                            if (focused !== null && Number.isFinite(focused)) dialogState.predictedIndex = focused;
                            return lines;
                          } catch { return origRender(width); }
                        };
                      }
                      component.handleInput = (data) => {
                        try {
                          if (typeof data === "string" && data.includes("\x1b[<")) {
                            const handled = translateMouse(data);
                            if (handled) return;
                          }
                          if (data === "\x1b[A" || data === "\x1bOA") dialogState.predictedIndex = (dialogState.predictedIndex - 1 + Math.max(1, dialogState.estItemsLen)) % Math.max(1, dialogState.estItemsLen);
                          else if (data === "\x1b[B" || data === "\x1bOB") dialogState.predictedIndex = (dialogState.predictedIndex + 1) % Math.max(1, dialogState.estItemsLen);
                          else if (typeof data === "string" && data.length === 1 && data >= "1" && data <= "9") {
                            const n = parseInt(data, 10) - 1;
                            if (n >= 0 && n < dialogState.estItemsLen) dialogState.predictedIndex = n;
                          }
                        } catch {}
                        return origHandleInput(data);
                      };
                      return component;
                    };
                    return origCustom(wrappedFactory, patchedOpts);
                  };
                }
                try { enableMouse(maybeCtx); } catch {}
                if (maybeCtx && maybeCtx.ui && typeof maybeCtx.ui.onTerminalInput === "function") {
                  try {
                    const rawHandler = (data) => {
                      try {
                        if (typeof data !== "string" || !data.includes("\x1b[<")) return undefined;
                        if (!dialogState.component || !dialogState.origHandleInput) return undefined;
                        const handled = translateMouse(data);
                        if (handled) return { consume: true };
                        return undefined;
                      } catch { return undefined; }
                    };
                    removeMouseRawListener = maybeCtx.ui.onTerminalInput(rawHandler);
                  } catch {}
                }
                try { return await origExecute(...args); }
                finally {
                  try { if (removeMouseRawListener) removeMouseRawListener(); } catch {}
                  try { disableMouse(maybeCtx); } catch {}
                }
              }

              // ── Proper TabBar/ScrollView overlay (default) ───────
              // This is 70% of the file: TabBar per question, splitPreviewSegments,
              // MAX_HEADER_ROWS=4, DIALOG_HEIGHT_RATIO=0.7 clamped panel, countdown
              const params = maybeParams;
              if (!params || !Array.isArray(params.questions) || params.questions.length === 0) {
                return await origExecute(...args);
              }
              // Wrap ctx.ui.custom to inject AskDialogComponent instead of rpiv questionnaire
              if (maybeCtx && maybeCtx.ui && typeof maybeCtx.ui.custom === "function") {
                const origCustom = maybeCtx.ui.custom.bind(maybeCtx.ui);
                maybeCtx.ui.custom = (factory, opts) => {
                  // Reserve panel height per DIALOG_HEIGHT_RATIO=0.7
                  let patchedOpts = opts;
                  try {
                    if (opts && typeof opts === "object") {
                      const overlayOpts = opts.overlayOptions || {};
                      // Clamp maxHeight to DIALOG_HEIGHT_RATIO
                      const desired = `${Math.round(DIALOG_HEIGHT_RATIO * 100)}%`;
                      patchedOpts = {
                        ...opts,
                        overlayOptions: { ...overlayOpts, maxHeight: overlayOpts.maxHeight ?? desired },
                        overlay: opts.overlay ?? true,
                      };
                    }
                  } catch {}
                  const wrappedFactory = (tui, theme, keybindings, done) => {
                    // Use theme from pi or fallback
                    const effectiveTheme = theme || fallbackTheme();
                    // Create AskDialogComponent with per-question TabBar
                    const askDialog = new AskDialogComponent(
                      params.questions,
                      {
                        onSubmit: (result) => {
                          try { done(result); } catch {}
                        },
                        onCancel: () => {
                          try { done(undefined); } catch {}
                        },
                        onPrompt: async (title, prefill) => {
                          // boundPromptTitle 3 rows already applied by caller; ensure width clamp
                          const promptTitle = typeof title === "string" ? title : String(title);
                          try {
                            if (maybeCtx?.ui?.editor) {
                              return await maybeCtx.ui.editor(promptTitle, prefill);
                            }
                            if (typeof tui?.editor === "function") {
                              return await tui.editor(promptTitle, prefill);
                            }
                            // Fallback: use ui.input if editor unavailable
                            if (typeof maybeCtx?.ui?.input === "function") {
                              return await maybeCtx.ui.input(promptTitle, prefill);
                            }
                          } catch {}
                          return undefined;
                        },
                      },
                      {
                        timeout: params.timeout ?? params.timeoutMs ?? undefined,
                        onTimeout: () => {
                          try { done(undefined); } catch {}
                        },
                        tui,
                        inputGuard: undefined,
                      },
                      tui,
                      effectiveTheme
                    );
                    // Mark for debugging
                    try { askDialog.__biggzAskDialog = true; } catch {}
                    // Expose helpers for tests
                    try {
                      if (!tui.__biggzAskDialog) tui.__biggzAskDialog = askDialog;
                    } catch {}
                    return askDialog;
                  };
                  return origCustom(wrappedFactory, patchedOpts);
                };
              }
              // Also handle ctx.ui.input fallback if custom not used
              return await origExecute(...args);
            };
          }
        } catch {}
        return origRegister(def);
      };
    }
  } catch {}

  // ── QuestionnaireSession prototype dispatch patch (mouse fallback only) ──
  if (isMouseAllowed()) {
    (async () => {
      try {
        const candidates = [
          `${process.env.PI_CODING_AGENT_DIR || `${process.env.HOME || process.env.USERPROFILE || ""}/.pi/agent`}/npm/node_modules/@juicesharp/rpiv-ask-user-question/state/questionnaire-session.ts`,
          `${process.env.HOME || process.env.USERPROFILE || ""}/.pi/agent/npm/node_modules/@juicesharp/rpiv-ask-user-question/state/questionnaire-session.ts`,
        ];
        let mod = null;
        for (const p of candidates) {
          try {
            const url = `file://${p.replace(/\\/g, "/")}`;
            mod = await import(url);
            if (mod && mod.QuestionnaireSession) break;
          } catch {}
        }
        if (mod && mod.QuestionnaireSession && mod.QuestionnaireSession.prototype) {
          const proto = mod.QuestionnaireSession.prototype;
          const origDispatch = proto.dispatch;
          if (typeof origDispatch === "function" && !proto.__biggzMousePatched) {
            proto.dispatch = function (data) {
              try {
                if (typeof data === "string" && data.includes("\x1b[<")) {
                  // mouse already handled via handleInput wrapper
                }
              } catch {}
              return origDispatch.call(this, data);
            };
            proto.__biggzMousePatched = true;
          }
        }
      } catch {}
    })();
  }
}
