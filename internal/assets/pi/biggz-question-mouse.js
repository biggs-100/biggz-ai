/**
 * biggz-question-mouse — pi mouse parity + push-above for ask_user_question (v4).
 *
 * Pi's questionnaire (npm:@juicesharp/rpiv-ask-user-question 2.7.1) is
 * keyboard-only (arrows/numbers/Tab/Enter/Space/n). Opencode's `question`
 * tool supports mouse because its TUI enables SGR mouse reporting
 * (ESC[?1000h + ESC[?1006h) and maps clicks to row selection.
 *
 * This extension adds mouse parity WITHOUT forking the package and fixes the
 * overlay-covering-chat problem by pushing chat above the questionnaire
 * instead of compositing over it.
 *
 * v4 — push-above: instead of pi-tui's compositeOverlays overwriting chat lines
 * via compositeLineAt (result[idx] = compositeLineAt(...)), we reserve vertical
 * space so chat is pushed above. The overlay (anchor: bottom-center, width:100%)
 * is rendered over empty padding, not over chat. Implemented by monkey-patching
 * the TUI instance's compositeOverlays at runtime to extend the base lines array
 * by the overlay height before delegating to the original composite. This keeps
 * chat visible without forking pi-tui or the questionnaire package and persists
 * via `biggz install --agent pi` / `biggz sync` because the extension is copied
 * from the embedded asset (internal/assets/pi/biggz-question-mouse.js) to
 * ~/.pi/agent/extensions/biggz-question-mouse.js via DeployPiQuestionMouse.
 *
 * Two-level push defense:
 *  1) Prototype patch at extension load (best-effort) — tries to import
 *     @earendil-works/pi-tui and wrap TuiBase.prototype.compositeOverlays so
 *     every TUI instance pushes. If the import fails (different loader layout),
 *     the instance patch below still guarantees push for the active dialog.
 *  2) Instance patch inside ctx.ui.custom factory — captures the live tui
 *     object, wraps its compositeOverlays bound method, and tags the
 *     questionnaire component with __biggzAskQuestion so only that overlay
 *     pushes; other overlays (e.g. /btw) keep original composite behavior.
 *     Both layers are defensive, idempotent, and fallback to composite on error.
 *
 * v3 mouse parity (still active, benefits from push because overlayTop math
 * stays bottom-anchored but chat is no longer occluded):
 *  1) Raw onTerminalInput listener (primary) — intercepts SGR mouse sequences
 *     \x1b[<Cb;Cx;CyM  / \x1b[<Cb;Cx;Cym  BEFORE Pi's TUI input parser can filter
 *     them. Translates to synthetic handleInput calls (arrows/Space/Enter) on the
 *     captured component and returns {consume:true} so Pi doesn't double-process.
 *  2) component.handleInput patch (fallback) — same translation inside the
 *     component's handleInput wrapper, kept for when raw listener isn't available
 *     (e.g. RPC hosts where onTerminalInput is a no-op) or for sequences that
 *     arrive already routed to the component.
 *
 * Mouse reporting is enabled ONLY while ask_user_question is active
 * (per-dialog enable in the execute wrapper, BEFORE ctx.ui.custom blocks,
 *  disable in finally). Outside dialogs mouse stays OFF so text selection
 *  and wheel scroll work normally. During a dialog hold Shift+drag to select.
 *
 * Layout mapping (robust):
 *  - Wraps component.render() to capture lastLines/lastHeight and live header
 *    offset via scan for first option line (pointer + number). Survives multi
 *    vs single, header presence, preview side-by-side, etc.
 *  - Absolute Cy (1-indexed SGR) -> dialog-relative via
 *    overlayTop = termRows - lastHeight + 1 (bottom-anchored overlay).
 *    termRows is captured from tui.terminal.rows inside the factory closure
 *    so the raw listener — registered at ctx level — can still read it.
 *  - If tui.terminal is undefined, falls back to headerOffset scan.
 *  - Wrapped descriptions map to nearest preceding option start.
 *  - Clicks on footer hints (enter to select / to collapse / etc.) are ignored.
 *  - Clicks with cy < overlayTop (above overlay) are ignored.
 *
 * Wheel: Cb>=64 is wheel (bit0=direction). Modifier bits (shift/alt/ctrl) are
 * preserved in upper bits, so we test cb>=64 and decode direction via (cb&1).
 * Mapped to \x1b[A / \x1b[B.
 *
 * Clicks: (Cb & 0x3f)===0 is left click. Compute targetGlobal via
 * getTargetGlobalForLineIdx when render captured, else fallback cy-headerOffset.
 * delta = clamped - predictedIndex -> synthesize arrow steps, then
 * single: double-click within 400ms => \r else focus; multi: Space or \r for Next.
 *
 * Defensive: all blocks try/catch, handles wheel, ignores non-left, acts on press
 * (M) only, strips mouse sequences before forwarding remainder.
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzQuestionMouse(pi) {
  if (process.env.PI_SUBAGENT_CHILD === "1") return;

  const MOUSE_ENABLE = "\x1b[?1000h\x1b[?1006h";
  const MOUSE_DISABLE = "\x1b[?1000l\x1b[?1006l";
  const DOUBLE_CLICK_MS = 400;

  function stripAnsi(s) {
    try {
      return String(s)
        .replace(/\x1b\[[0-9;]*[A-Za-z]/g, "")
        .replace(/\x1b\][^\x07]*\x07/g, "")
        .replace(/\x1b\(B/g, "");
    } catch {
      return String(s);
    }
  }

  function isOptionStartLine(stripped) {
    try {
      return /^\s*(?:\u276F\s*)?\s*\d+\.\s/.test(stripped);
    } catch {
      return false;
    }
  }

  function parseOptionNumber(stripped) {
    try {
      const m = stripped.match(/\d+/);
      if (!m) return null;
      const n = parseInt(m[0], 10);
      if (!Number.isFinite(n) || n < 1) return null;
      return n - 1;
    } catch {
      return null;
    }
  }

  function computeHeaderOffset(lines) {
    try {
      for (let i = 0; i < lines.length; i++) {
        if (isOptionStartLine(stripAnsi(lines[i]))) return i;
      }
      return null;
    } catch {
      return null;
    }
  }

  function computeFocusedGlobal(lines, estItemsLen) {
    try {
      for (let i = 0; i < lines.length; i++) {
        const raw = lines[i];
        if (!raw || !raw.includes("\u276F")) continue;
        const stripped = stripAnsi(raw);
        if (!stripped.includes("\u276F")) continue;
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
    } catch {
      return null;
    }
  }

  function getTargetGlobalForLineIdx(lines, lineIdx, estItemsLen) {
    try {
      if (lineIdx < 0 || lineIdx >= lines.length) return null;
      const lineStripped = stripAnsi(lines[lineIdx] || "");
      if (isOptionStartLine(lineStripped)) {
        const n = parseOptionNumber(lineStripped);
        if (n !== null) return n;
      }
      if (/^\s*(?:\u276F\s*)?\s*Next\s*$/i.test(lineStripped.trim()) || (lineStripped.includes("Next") && lineStripped.includes("\u276F"))) {
        return Math.max(0, (estItemsLen || 1) - 1);
      }
      if (lineStripped.trim() === "Next" || lineStripped.trim().toLowerCase() === "next") {
        return Math.max(0, (estItemsLen || 1) - 1);
      }
      const lower = lineStripped.toLowerCase();
      if (
        lower.includes("enter to select") ||
        lower.includes("to navigate") ||
        lower.includes("to collapse") ||
        lower.includes("to expand") ||
        lower.includes("esc to cancel") ||
        lower.includes("shift+enter") ||
        lower.includes("to clear") ||
        lower.includes("to toggle")
      ) {
        return null;
      }
      const starts = [];
      for (let i = 0; i < lines.length; i++) if (isOptionStartLine(stripAnsi(lines[i]))) starts.push(i);
      if (starts.length === 0) return null;
      let best = -1;
      for (const s of starts) {
        if (s <= lineIdx) best = s;
        else break;
      }
      if (best === -1) return null;
      const n = parseOptionNumber(stripAnsi(lines[best]));
      if (n !== null) return n;
      const pos = starts.indexOf(best);
      return pos !== -1 ? pos : null;
    } catch {
      return null;
    }
  }

  function computeFallbackOffset(params) {
    try {
      const q = params?.questions?.[0];
      const isMulti = (params?.questions?.length || 0) > 1 || !!q?.multiSelect;
      const hasHeader = !!q?.header;
      const topFixed = isMulti ? 4 : 2;
      const heading = isMulti ? 2 : hasHeader ? 4 : 2;
      return topFixed + heading;
    } catch {
      return 5;
    }
  }

  function enableMouse(ctx) {
    try {
      process.stdout.write(MOUSE_ENABLE);
    } catch {}
    try {
      if (process.stderr && typeof process.stderr.write === "function") process.stderr.write(MOUSE_ENABLE);
    } catch {}
    try {
      console.log("[biggz-question-mouse] mouse enabled");
    } catch {}
    try {
      console.log("[biggz-question-mouse] mouse enabled for ask_user_question (SGR 1000/1006, wheel \u2192 arrows, Shift+drag to select text, Ctrl+] to collapse)");
    } catch {}
    try {
      ctx?.ui?.notify?.("[biggz-question-mouse] mouse enabled", "info");
    } catch {}
  }

  function disableMouse(ctx) {
    try {
      process.stdout.write(MOUSE_DISABLE);
    } catch {}
    try {
      if (process.stderr && typeof process.stderr.write === "function") process.stderr.write(MOUSE_DISABLE);
    } catch {}
    try {
      console.log("[biggz-question-mouse] mouse disabled");
    } catch {}
    try {
      ctx?.ui?.notify?.("[biggz-question-mouse] mouse disabled", "info");
    } catch {}
  }

  // ── Push-above helpers ──────────────────────────────────────────────
  // Why push vs overlay: pi-tui's compositeOverlays does result[idx] =
  // compositeLineAt(base, overlay, ...) which overwrites chat pixels.
  // With push we extend the base lines array by the overlay height so the
  // original composite renders over empty padding, not chat, effectively
  // shifting chat up. The TUI viewport (last termHeight lines) then shows
  // chat above + questionnaire at the bottom, matching opencode's push.
  function isPushOverlay(entry, tui) {
    try {
      if (!entry?.component?.__biggzAskQuestion) return false;
      if (typeof tui?.isOverlayVisible === "function" && !tui.isOverlayVisible(entry)) return false;
      return true;
    } catch {
      return false;
    }
  }

  function ensurePushPatched(tui, notifyCtx) {
    try {
      if (!tui) return;
      // Capture notify for visible push confirmation; update even if already patched so each dialog's ctx is used.
      let notifyFn = null;
      try {
        notifyFn = notifyCtx?.ui?.notify?.bind(notifyCtx.ui) || null;
        if (notifyFn) tui.__biggzPushNotify = notifyFn;
      } catch {}
      if (tui.__biggzPushPatched) {
        // Reset per-dialog flags so next dialog's push is notified again.
        try {
          tui.__biggzPushLogged = false;
          tui.__biggzPushNotified = false;
        } catch {}
        return;
      }
      const origComposite = typeof tui.compositeOverlays === "function" ? tui.compositeOverlays.bind(tui) : null;
      const origResolve = typeof tui.resolveOverlayLayout === "function" ? tui.resolveOverlayLayout.bind(tui) : null;
      if (!origComposite || !origResolve) {
        try {
          console.log("[biggz-question-push] tui missing compositeOverlays/resolveOverlayLayout, skip push patch");
        } catch {}
        try {
          notifyCtx?.ui?.notify?.("[biggz-question-push] push unavailable: TUI missing composite", "warning");
        } catch {}
        return;
      }
      if (notifyFn) tui.__biggzPushNotify = notifyFn;
      tui.__biggzOrigComposite = origComposite;
      tui.compositeOverlays = function (lines, termWidth, termHeight) {
        try {
          const stack = this.overlayStack || [];
          const hasPush = stack.some((e) => isPushOverlay(e, this));
          if (!hasPush) return origComposite(lines, termWidth, termHeight);

          // Compute total height to reserve for all push overlays visible.
          let totalPushHeight = 0;
          for (const entry of stack) {
            if (!isPushOverlay(entry, this)) continue;
            try {
              const opts = entry.options || {};
              const { width, maxHeight } = origResolve(opts, 0, termWidth, termHeight);
              let overlayLines = [];
              try {
                overlayLines = entry.component.render(width);
              } catch {}
              let h = overlayLines.length;
              if (maxHeight !== undefined && h > maxHeight) h = maxHeight;
              h = Math.max(0, Math.min(h, termHeight));
              totalPushHeight += h;
            } catch {}
          }
          if (totalPushHeight <= 0) return origComposite(lines, termWidth, termHeight);
          // Keep at least 3 chat rows visible above the questionnaire.
          const maxPush = Math.max(0, termHeight - 3);
          if (totalPushHeight > maxPush) totalPushHeight = maxPush;

          const extended = [...lines];
          const targetLen = lines.length + totalPushHeight;
          while (extended.length < targetLen) extended.push("");

          if (!this.__biggzPushLogged) {
            this.__biggzPushLogged = true;
            try {
              console.log(`[biggz-question-push] push active: reserved ${totalPushHeight} rows (term ${termWidth}x${termHeight})`);
            } catch {}
            try {
              this.terminal?.write?.("");
            } catch {}
          }
          try {
            console.log(`[biggz-question-push] push mode active — chat pushed ${totalPushHeight} rows above questionnaire`);
          } catch {}
          try {
            this.terminal?.write?.("");
          } catch {}

          const result = origComposite(extended, termWidth, termHeight);
          // Visible confirmation when push triggers (idempotent per dialog via __biggzPushNotified).
          try {
            if (!this.__biggzPushNotified) {
              this.__biggzPushNotified = true;
              const n = this.__biggzPushNotify;
              if (typeof n === "function") n(`push active — chat pushed ${totalPushHeight} rows above questionnaire`, "info");
              else if (typeof notifyFn === "function") notifyFn(`push active — chat pushed ${totalPushHeight} rows above questionnaire`, "info");
            }
          } catch {}
          return result;
        } catch (e) {
          try {
            console.log(`[biggz-question-push] push wrapper error: ${e?.message || e}, fallback to composite`);
          } catch {}
          return origComposite(lines, termWidth, termHeight);
        }
      };
      tui.__biggzPushPatched = true;
      try {
        console.log("[biggz-question-push] push patch installed on TUI instance (chat will be pushed above questionnaire)");
      } catch {}
      try {
        tui.terminal?.write?.("");
      } catch {}
    } catch (e) {
      try {
        console.log(`[biggz-question-push] ensurePushPatched failed: ${e?.message || e}`);
      } catch {}
    }
  }

  // Prototype-level push patch (best-effort, runs once at extension load).
  // Covers TUI instances created before the first questionnaire; the instance
  // patch above is the guaranteed path for the dialog's own tui.
  (async () => {
    try {
      let mod = null;
      const candidates = ["@earendil-works/pi-tui", "@earendil-works/pi-tui/dist/tui.js"];
      for (const spec of candidates) {
        try {
          mod = await import(spec);
          if (mod) break;
        } catch {}
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
            } catch {
              return false;
            }
          });
          if (!hasPush) return protoOrigComposite.call(this, lines, termWidth, termHeight);
          let totalPushHeight = 0;
          for (const entry of stack) {
            let isPush = false;
            try {
              isPush = !!entry?.component?.__biggzAskQuestion;
              if (isPush && typeof this.isOverlayVisible === "function" && !this.isOverlayVisible(entry)) isPush = false;
            } catch {
              isPush = false;
            }
            if (!isPush) continue;
            try {
              const opts = entry.options || {};
              const { width, maxHeight } = protoOrigResolve.call(this, opts, 0, termWidth, termHeight);
              let overlayLines = [];
              try {
                overlayLines = entry.component.render(width);
              } catch {}
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
          if (!this.__biggzPushLogged) {
            this.__biggzPushLogged = true;
            try {
              console.log(`[biggz-question-push] prototype push active: reserved ${totalPushHeight} rows`);
            } catch {}
          }
          return protoOrigComposite.call(this, extended, termWidth, termHeight);
        } catch (e) {
          try {
            console.log(`[biggz-question-push] prototype push error: ${e?.message || e}`);
          } catch {}
          return protoOrigComposite.call(this, lines, termWidth, termHeight);
        }
      };
      proto.__biggzPushProtoPatched = true;
      try {
        console.log("[biggz-question-push] prototype push patch installed (TuiBase.prototype.compositeOverlays)");
      } catch {}
    } catch {}
  })();

  try {
    const cleanup = () => {
      try {
        disableMouse();
      } catch {}
    };
    process.on("exit", cleanup);
    try {
      process.on("SIGINT", cleanup);
    } catch {}
    try {
      process.on("SIGTERM", cleanup);
    } catch {}
  } catch {}

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
              const typedForWrapper = capturedParams;

              // Per-dialog shared state — accessible to both factory and raw listener.
              // This is the bridge that lets the ctx-level onTerminalInput handler
              // call synthetic handleInput on the component created inside
              // ctx.ui.custom's factory.
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

              // Shared mouse translation — used by both handleInput wrapper and raw listener.
              // Calls origHandleInput to move focus/toggle. Returns true if any mouse was handled.
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
                    if (cb >= 64) {
                      const isDown = (cb & 1) === 1;
                      wheelDeltas.push(isDown ? 1 : -1);
                      continue;
                    }
                    if ((cb & 0x3f) !== 0) continue;
                    clicks.push({ cb, cy });
                  }

                  // Diagnostic on first mouse event (either wheel or click).
                  if ((wheelDeltas.length > 0 || clicks.length > 0) && !dialogState.firstMouseNotified) {
                    dialogState.firstMouseNotified = true;
                    try {
                      console.log("[biggz-question-mouse] mouse event captured (raw listener active)");
                    } catch {}
                    try {
                      maybeCtx?.ui?.notify?.("mouse event captured", "info");
                    } catch {}
                  }

                  for (const delta of wheelDeltas) {
                    try {
                      if (!dialogState.origHandleInput) continue;
                      if (delta < 0) {
                        dialogState.origHandleInput("\x1b[A");
                        dialogState.predictedIndex =
                          (dialogState.predictedIndex - 1 + Math.max(1, dialogState.estItemsLen)) %
                          Math.max(1, dialogState.estItemsLen);
                        console.log(`[biggz-question-mouse] wheel up -> nav prev (predicted=${dialogState.predictedIndex})`);
                      } else {
                        dialogState.origHandleInput("\x1b[B");
                        dialogState.predictedIndex = (dialogState.predictedIndex + 1) % Math.max(1, dialogState.estItemsLen);
                        console.log(`[biggz-question-mouse] wheel down -> nav next (predicted=${dialogState.predictedIndex})`);
                      }
                      handledMouse = true;
                    } catch {}
                  }

                  for (const { cb, cy } of clicks) {
                    let targetGlobal = null;
                    let usedFallback = false;

                    if (dialogState.lastLines && dialogState.lastHeight && dialogState.tui?.terminal?.rows) {
                      const termRows = dialogState.tui.terminal.rows;
                      const overlayTop = termRows - dialogState.lastHeight + 1; // 1-indexed
                      if (cy < overlayTop) {
                        console.log(
                          `[biggz-question-mouse] click ignored (above overlay) Cb=${cb} Cy=${cy} overlayTop=${overlayTop} termRows=${termRows} h=${dialogState.lastHeight}`,
                        );
                        continue;
                      }
                      const dialogRow = cy - overlayTop + 1; // 1..h
                      const lineIdx = dialogRow - 1; // 0-based
                      if (lineIdx < 0 || lineIdx >= dialogState.lastLines.length) {
                        console.log(
                          `[biggz-question-mouse] click ignored (out of dialog bounds) lineIdx=${lineIdx} h=${dialogState.lastHeight} Cy=${cy}`,
                        );
                        continue;
                      }
                      targetGlobal = getTargetGlobalForLineIdx(dialogState.lastLines, lineIdx, dialogState.estItemsLen);
                      if (targetGlobal === null) {
                        console.log(
                          `[biggz-question-mouse] click ignored (no mappable option at lineIdx=${lineIdx} Cy=${cy} headerOffset=${dialogState.lastHeaderOffset})`,
                        );
                        continue;
                      }
                    } else {
                      usedFallback = true;
                      const fallbackOffset = dialogState.lastHeaderOffset ?? computeFallbackOffset(typedForWrapper);
                      if (cy < fallbackOffset) {
                        console.log(
                          `[biggz-question-mouse] click ignored (fallback above options) Cb=${cb} Cy=${cy} offset=${fallbackOffset}`,
                        );
                        continue;
                      }
                      const raw = Math.max(0, cy - fallbackOffset);
                      const clampedFallback = Math.min(raw, Math.max(0, dialogState.estItemsLen - 1));
                      targetGlobal = clampedFallback;
                      console.log(`[biggz-question-mouse] fallback mapping Cy=${cy} offset=${fallbackOffset} -> ${targetGlobal}`);
                    }

                    const clamped = Math.max(0, Math.min(targetGlobal, dialogState.estItemsLen - 1));

                    const isMulti = (() => {
                      try {
                        const qs = typedForWrapper?.questions;
                        if (!qs) return false;
                        return qs.some((q) => !!q.multiSelect);
                      } catch {
                        return false;
                      }
                    })();

                    const delta = clamped - dialogState.predictedIndex;
                    console.log(
                      `[biggz-question-mouse] click Cb=${cb} Cy=${cy} -> target=${clamped} predicted=${dialogState.predictedIndex} delta=${delta} multi=${isMulti} fallback=${usedFallback} headerOffset=${dialogState.lastHeaderOffset} h=${dialogState.lastHeight}`,
                    );

                    if (!dialogState.origHandleInput) continue;

                    if (delta !== 0) {
                      const step = delta > 0 ? "\x1b[B" : "\x1b[A";
                      const abs = Math.abs(delta);
                      const steps = Math.min(abs, Math.max(dialogState.estItemsLen, 8));
                      for (let i = 0; i < steps; i++) {
                        try {
                          dialogState.origHandleInput(step);
                        } catch {}
                      }
                      dialogState.predictedIndex = clamped;
                    } else {
                      dialogState.predictedIndex = clamped;
                    }

                    if (isMulti) {
                      if (clamped === dialogState.estItemsLen - 1) {
                        try {
                          dialogState.origHandleInput("\r");
                        } catch {}
                        console.log(`[biggz-question-mouse] confirmed multi Next (index ${clamped})`);
                      } else {
                        try {
                          dialogState.origHandleInput(" ");
                        } catch {}
                        console.log(`[biggz-question-mouse] toggled index ${clamped} (multi)`);
                      }
                      handledMouse = true;
                    } else {
                      const now = Date.now();
                      if (dialogState.lastClickIndex === clamped && now - dialogState.lastClickTime < DOUBLE_CLICK_MS) {
                        try {
                          dialogState.origHandleInput("\r");
                        } catch {}
                        console.log(`[biggz-question-mouse] confirmed single index ${clamped} (double-click)`);
                      } else {
                        console.log(`[biggz-question-mouse] focused single index ${clamped} (click again to confirm)`);
                      }
                      dialogState.lastClickIndex = clamped;
                      dialogState.lastClickTime = now;
                      handledMouse = true;
                    }
                  }

                  if (handledMouse) {
                    const stripped = data.replace(/\x1b\[<\d+;\d+;\d+[Mm]/g, "");
                    if (stripped && stripped !== data && stripped.trim()) {
                      try {
                        if (dialogState.origHandleInput) dialogState.origHandleInput(stripped);
                      } catch {}
                    }
                    return true;
                  }
                  return false;
                } catch (e) {
                  console.log(`[biggz-question-mouse] translateMouse error: ${e?.message || e}`);
                  return false;
                }
              }

              let removeMouseRawListener = null;

              // Patch ctx.ui.custom — captures component + tui and keeps handleInput fallback.
              // Also installs the push-above patch so chat is not covered.
              if (maybeCtx && maybeCtx.ui && typeof maybeCtx.ui.custom === "function") {
                const origCustom = maybeCtx.ui.custom.bind(maybeCtx.ui);
                maybeCtx.ui.custom = (factory, opts) => {
                  let patchedOpts = opts;
                  try {
                    if (opts && opts.overlay === true && opts.overlayOptions) {
                      const cur = opts.overlayOptions.maxHeight;
                      if (cur === "100%" || cur === "100" || cur === 100) {
                        patchedOpts = {
                          ...opts,
                          overlayOptions: { ...opts.overlayOptions, maxHeight: "85%" },
                        };
                        console.log(
                          "[biggz-question-mouse] overlay maxHeight 100% -> 85% for transcript visibility (overlay:true preserved, push will keep chat above)",
                        );
                      }
                    }
                  } catch (e) {
                    console.log(`[biggz-question-mouse] overlay patch error: ${e?.message || e}`);
                  }

                  const wrappedFactory = (tui, theme, keybindings, done) => {
                    dialogState.tui = tui;
                    let component;
                    try {
                      component = factory(tui, theme, keybindings, done);
                    } catch (e) {
                      throw e;
                    }
                    if (!component || typeof component.handleInput !== "function") return component;

                    // Tag for push detection — only this questionnaire should push.
                    // Must happen BEFORE ensurePushPatched so first composite sees hasPush=true.
                    try {
                      component.__biggzAskQuestion = true;
                      component.__biggzPush = true;
                    } catch {}
                    // Install push-above patch on this TUI instance before the overlay renders.
                    // This reserves space so the bottom-anchored questionnaire does not
                    // composite over chat lines. Idempotent per tui (notify updated per dialog).
                    try {
                      ensurePushPatched(tui, maybeCtx);
                    } catch (e) {
                      try {
                        console.log(`[biggz-question-push] ensurePushPatched in factory failed: ${e?.message || e}`);
                      } catch {}
                    }

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
                          try {
                            const starts = [];
                            for (let i = 0; i < lines.length; i++) if (isOptionStartLine(stripAnsi(lines[i]))) starts.push(i);
                            if (starts.length > 0) {
                              const isMulti = (() => {
                                try {
                                  return (typedForWrapper?.questions || []).some((q) => !!q.multiSelect);
                                } catch {
                                  return false;
                                }
                              })();
                              const inferred = starts.length + (isMulti && lines.some((l) => /Next/i.test(stripAnsi(l))) ? 1 : 0);
                              if (inferred > dialogState.estItemsLen) dialogState.estItemsLen = inferred;
                            }
                          } catch {}
                          try {
                            const footerText = lines
                              .slice(-3)
                              .map((l) => stripAnsi(l))
                              .join(" | ")
                              .toLowerCase();
                            const hasCollapseHint = footerText.includes("to collapse") || footerText.includes("to expand");
                            if (!hasCollapseHint && lines.length > 1) {
                              const allText = lines
                                .map((l) => stripAnsi(l))
                                .join(" ")
                                .toLowerCase();
                              if (!allText.includes("to collapse") && !allText.includes("to expand")) {
                                console.log("[biggz-question-mouse] note: collapse hint not detected in render (expected 'Ctrl+] to collapse')");
                              }
                            }
                          } catch {}
                          return lines;
                        } catch (e) {
                          console.log(`[biggz-question-mouse] render wrap error: ${e?.message || e}`);
                          return origRender(width);
                        }
                      };
                    }

                    // Fallback handleInput path — handles mouse if raw listener didn't consume it
                    // (e.g. hosts where onTerminalInput is unavailable, or sequences routed directly).
                    component.handleInput = (data) => {
                      try {
                        if (typeof data === "string" && data.includes("\x1b[<")) {
                          const handled = translateMouse(data);
                          if (handled) return;
                        }
                        if (data === "\x1b[A" || data === "\x1bOA") {
                          dialogState.predictedIndex =
                            (dialogState.predictedIndex - 1 + Math.max(1, dialogState.estItemsLen)) %
                            Math.max(1, dialogState.estItemsLen);
                        } else if (data === "\x1b[B" || data === "\x1bOB") {
                          dialogState.predictedIndex = (dialogState.predictedIndex + 1) % Math.max(1, dialogState.estItemsLen);
                        } else if (typeof data === "string" && data.length === 1 && data >= "1" && data <= "9") {
                          const n = parseInt(data, 10) - 1;
                          if (n >= 0 && n < dialogState.estItemsLen) dialogState.predictedIndex = n;
                        }
                      } catch (e) {
                        console.log(`[biggz-question-mouse] handleInput wrapper error: ${e?.message || e}`);
                      }
                      return origHandleInput(data);
                    };

                    return component;
                  };
                  return origCustom(wrappedFactory, patchedOpts);
                };
              }

              // Enable mouse BEFORE the custom overlay blocks — per spec this must happen
              // before ctx.ui.custom is awaited inside origExecute.
              try {
                enableMouse(maybeCtx);
              } catch {}

              // Register raw onTerminalInput listener that captures SGR at TUI level.
              // Must run after maybeCtx is captured and before origExecute blocks.
              // Keep remover to unregister + disable in finally.
              if (maybeCtx && maybeCtx.ui && typeof maybeCtx.ui.onTerminalInput === "function") {
                try {
                  const rawHandler = (data) => {
                    try {
                      if (typeof data !== "string" || !data.includes("\x1b[<")) return undefined;
                      // If component not yet mounted (very early input before factory), fallback
                      // to handleInput handling later — don't consume yet.
                      if (!dialogState.component || !dialogState.origHandleInput) return undefined;
                      const handled = translateMouse(data);
                      if (handled) return { consume: true };
                      return undefined;
                    } catch (e) {
                      console.log(`[biggz-question-mouse] raw handler error: ${e?.message || e}`);
                      return undefined;
                    }
                  };
                  removeMouseRawListener = maybeCtx.ui.onTerminalInput(rawHandler);
                  console.log("[biggz-question-mouse] raw onTerminalInput listener registered");
                } catch (e) {
                  console.log(`[biggz-question-mouse] onTerminalInput register failed: ${e?.message || e}`);
                }
              } else {
                console.log("[biggz-question-mouse] onTerminalInput unavailable, falling back to handleInput-only");
              }

              try {
                return await origExecute(...args);
              } finally {
                try {
                  if (removeMouseRawListener) removeMouseRawListener();
                } catch {}
                try {
                  disableMouse(maybeCtx);
                } catch {}
                console.log("[biggz-question-mouse] mouse disabled after ask_user_question");
              }
            };
          }
        } catch (e) {
          console.log(`[biggz-question-mouse] registerTool wrapper error: ${e?.message || e}`);
        }
        return origRegister(def);
      };
    }
  } catch (e) {
    console.log(`[biggz-question-mouse] failed to wrap registerTool: ${e?.message || e}`);
  }

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
                console.log("[biggz-question-mouse] prototype dispatch saw mouse sequence (handled via handleInput wrapper)");
              }
            } catch {}
            return origDispatch.call(this, data);
          };
          proto.__biggzMousePatched = true;
          console.log("[biggz-question-mouse] patched QuestionnaireSession.prototype.dispatch");
        }
      }
    } catch {}
  })();
}

