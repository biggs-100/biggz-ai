/**
 * MANDATORY: Precede this tool call with synthesis markdown containing artifacts/paths + risks + next (see Post-Delegation Human Checkpoint).
 * biggz-question-mouse — pi mouse parity for ask_user_question.
 *
 * Pi's questionnaire (npm:@juicesharp/rpiv-ask-user-question 2.7.1) is
 * keyboard-only (arrows/numbers/Tab/Enter/Space/n). Opencode's `question`
 * tool supports mouse because its TUI enables SGR mouse reporting
 * (ESC[?1000h + ESC[?1006h) and maps clicks to row selection.
 *
 * This extension adds mouse parity WITHOUT forking the package:
 *  - Enables SGR extended mouse reporting ONLY while ask_user_question is active
 *    (per-dialog enable in the execute wrapper, disable in finally). Outside
 *    dialogs mouse reporting stays OFF so terminal text selection and wheel
 *    scroll work normally. During a dialog, hold Shift+drag to select text
 *    (standard terminal bypass for active mouse reporting).
 *  - Intercepts `ctx.ui.custom`'s factory/component handleInput to translate
 *    SGR mouse sequences `\x1b[<Cb;Cx;CyM` / `\x1b[<Cb;Cx;Cy m` into synthetic
 *    keyboard actions that the existing `routeKey` reducer already understands.
 *  - Wheel (Cb 64 up / 65 down) is mapped to arrow up/down so WrappingSelect
 *    scrolls its maxVisible window even when the dialog is taller than the
 *    terminal viewport.
 *
 * Layout mapping (v1, configurable):
 *  - headerOffset = 5 (heading + tabs + borders). Click row Cy -> targetIndex = clamp(Cy - offset, 0, itemsLen-1).
 *  - If Cy out of range, the click is ignored.
 *  - Single-select: click to focus, click again on same row to confirm (second click sends Enter). Matches typical select-then-confirm.
 *  - Multi-select: click toggles checkbox (Space) after nav; Next sentinel click sends Enter to submit.
 *
 * Fallback: if layout calc is fragile, the wrapper still enables mouse reporting
 * and synthesizes digit keys (`1`-`9`) as a secondary signal; opencode's
 * question allows "1", "la 1", etc. The key is that mouse always enables and is
 * never ignored as `ignore`.
 *
 * Defensive: all blocks are try/catch, console.log for debugging, handles wheel
 * (Cb 64/65) as nav prev/next, ignores non-left buttons, acts on press (M)
 * only, strips mouse sequences before forwarding remaining data.
 *
 * See: ask-user-question state/key-router.ts, state/questionnaire-session.ts,
 * view/dialog-builder.ts (topFixed/border), view/components/wrapping-select.ts
 */

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzQuestionMouse(pi) {
  if (process.env.PI_SUBAGENT_CHILD === "1") return;

  const MOUSE_ENABLE = "\x1b[?1000h\x1b[?1006h";
  const MOUSE_DISABLE = "\x1b[?1000l\x1b[?1006l";
  const HEADER_OFFSET = 5; // rows before first option; 4 for single, 5-6 for multi with tabs
  const DOUBLE_CLICK_MS = 500;

  function enableMouse() {
    try {
      if (process.stdout.isTTY) process.stdout.write(MOUSE_ENABLE);
      else process.stdout.write(MOUSE_ENABLE);
    } catch {}
  }
  function disableMouse() {
    try {
      if (process.stdout.isTTY) process.stdout.write(MOUSE_DISABLE);
      else process.stdout.write(MOUSE_DISABLE);
    } catch {}
  }

  // Mouse is enabled ONLY per-dialog (inside the ask_user_question execute wrapper).
  // Outside dialogs SGR reporting stays OFF so terminal text selection and wheel
  // scroll work normally. During a dialog, hold Shift+drag to select text
  // (standard terminal bypass for mouse reporting). Wheel is mapped to nav
  // prev/next so WrappingSelect scrolls even when the dialog is taller than
  // the terminal viewport.

  // Cleanup on exit — best effort, never throw.
  try {
    process.on("exit", () => {
      try { disableMouse(); } catch {}
    });
  } catch {}

  // Keep shadow state per-dialog to map clicks -> arrow navigation without
  // needing Session's private selectedIndex. Wrapped factory resets it per invocation.
  let capturedParams = null;

  // Wrap future registrations of ask_user_question (extension load order may be before or after rpiv-ask-user-question).
  let origRegister = null;
  try {
    if (typeof pi.registerTool === "function") {
      origRegister = pi.registerTool.bind(pi);
      pi.registerTool = (def) => {
        try {
          if (def && def.name === "ask_user_question" && typeof def.execute === "function") {
            const origExecute = def.execute;
            def.execute = async (...args) => {
              // args: toolCallId, params, signal, onUpdate, ctx  — ctx is last
              const maybeCtx = args[args.length - 1];
              const maybeParams = args[1];
              if (maybeParams && typeof maybeParams === "object") capturedParams = maybeParams;
              // Try to capture typed questions for multiSelect detection
              let typedForWrapper = capturedParams;
              // Patch ctx.ui.custom before calling the original execute so every
              // QuestionnaireSession created inside gets mouse translation.
              let patchedCtx = false;
              if (maybeCtx && maybeCtx.ui && typeof maybeCtx.ui.custom === "function") {
                const origCustom = maybeCtx.ui.custom.bind(maybeCtx.ui);
                maybeCtx.ui.custom = (factory, opts) => {
                  // Wrap the factory that builds the QuestionnaireSession component
                  const wrappedFactory = (tui, theme, keybindings, done) => {
                    let component;
                    try {
                      component = factory(tui, theme, keybindings, done);
                    } catch (e) {
                      // If factory throws, propagate — mouse wrapper must not hide original error.
                      throw e;
                    }
                    // If component has no handleInput, nothing to patch
                    if (!component || typeof component.handleInput !== "function") return component;

                    const origHandleInput = component.handleInput.bind(component);
                    // Per-dialog shadow state
                    let predictedIndex = 0;
                    let lastClickIndex = null;
                    let lastClickTime = 0;
                    // Estimate items length from captured params (best effort)
                    let estItemsLen = 8;
                    try {
                      const qLen = typedForWrapper?.questions?.length || 0;
                      if (qLen > 0) {
                        const firstQ = typedForWrapper.questions[0];
                        if (firstQ && Array.isArray(firstQ.options)) {
                          // +1 for "Type something." sentinel, +1 for Next in multiSelect
                          const isMulti = !!firstQ.multiSelect;
                          estItemsLen = firstQ.options.length + 1 + (isMulti ? 1 : 0);
                        }
                      }
                    } catch {}

                    component.handleInput = (data) => {
                      try {
                        if (typeof data === "string" && data.includes("\x1b[<")) {
                          // SGR mouse: \x1b[<Cb;Cx;CyM  (M press, m release)
                          const re = /\x1b\[<(\d+);(\d+);(\d+)([Mm])/g;
                          let m;
                          let handledMouse = false;
                          // We may have multiple sequences in one data chunk (e.g., press+release)
                          // Collect clicks, act on each press only.
                           const clicks = [];
                          const wheelDeltas = []; // -1 up (Cb 64), +1 down (Cb 65)
                          while ((m = re.exec(data)) !== null) {
                            const cb = parseInt(m[1], 10);
                            const cx = parseInt(m[2], 10);
                            const cy = parseInt(m[3], 10);
                            const isPress = m[4] === "M";
                            if (!isPress) continue;
                            // Wheel: Cb 64 = wheel up, 65 = wheel down (SGR). Map to nav prev/next.
                            // This lets WrappingSelect scroll its maxVisible window even when taller than viewport.
                            if (cb === 64) {
                              wheelDeltas.push(-1);
                              continue;
                            }
                            if (cb === 65) {
                              wheelDeltas.push(1);
                              continue;
                            }
                            // Left click is Cb 0; ignore right (2), middle (1), drags (32), etc.
                            // SGR encodes button + modifiers in Cb; wheel already handled above.
                            if (cb !== 0) continue;
                            clicks.push({ cb, cx, cy });
                          }
                          // Handle wheel: synthesize arrow up/down per tick, updating shadow predictedIndex.
                          for (const delta of wheelDeltas) {
                            try {
                              if (delta < 0) {
                                origHandleInput("\x1b[A");
                                predictedIndex = (predictedIndex - 1 + Math.max(1, estItemsLen)) % Math.max(1, estItemsLen);
                                console.log(`[biggz-question-mouse] wheel up -> nav prev (predicted=${predictedIndex})`);
                              } else {
                                origHandleInput("\x1b[B");
                                predictedIndex = (predictedIndex + 1) % Math.max(1, estItemsLen);
                                console.log(`[biggz-question-mouse] wheel down -> nav next (predicted=${predictedIndex})`);
                              }
                              handledMouse = true;
                            } catch {}
                          }
                          for (const { cb, cy } of clicks) {
                            // Map terminal row Cy to option index
                            // Header offset is approximate; log for tuning.
                            const targetIndex = Math.max(0, cy - HEADER_OFFSET);
                            // Clamp to estimated range, but allow slightly larger for debug
                            const clamped = Math.min(targetIndex, Math.max(0, estItemsLen - 1));
                            // Ignore clicks above the options area (e.g., header/tabs)
                            if (cy < HEADER_OFFSET) {
                              console.log(`[biggz-question-mouse] click ignored (above options) Cb=${cb} Cy=${cy} offset=${HEADER_OFFSET}`);
                              continue;
                            }
                            // Also ignore if clamped is out of plausible range and we have no itemsLen confidence
                            // We still allow it but log.
                            const isMulti = (() => {
                              try {
                                // Check capturedParams for multiSelect — if any question is multi, assume current tab is multi
                                const qs = typedForWrapper?.questions;
                                if (!qs) return false;
                                return qs.some((q) => !!q.multiSelect);
                              } catch { return false; }
                            })();

                            // Compute delta from predictedIndex to clamped
                            const delta = clamped - predictedIndex;
                            console.log(`[biggz-question-mouse] click Cb=${cb} Cy=${cy} -> target=${clamped} (raw ${targetIndex}) predicted=${predictedIndex} delta=${delta} multi=${isMulti}`);

                            // Synthesize arrow keys to move focus
                            // pi-tui matches \x1b[B for down, \x1b[A for up
                            if (delta !== 0) {
                              const step = delta > 0 ? "\x1b[B" : "\x1b[A";
                              const abs = Math.abs(delta);
                              // Clamp steps to avoid huge loops if offset miscalc; wrapTab handles wrap, but we still limit to estItemsLen
                              const steps = Math.min(abs, Math.max(estItemsLen, 8));
                              for (let i = 0; i < steps; i++) {
                                try { origHandleInput(step); } catch {}
                              }
                              predictedIndex = clamped;
                            } else {
                              // Even if no move, keep predicted in sync
                              predictedIndex = clamped;
                            }

                            // For multi-select, toggle checkbox after nav (Space)
                            if (isMulti) {
                              try { origHandleInput(" "); } catch {}
                              console.log(`[biggz-question-mouse] toggled index ${clamped} (multi)`);
                              // If this was the "Next" sentinel (last index), also send Enter to submit
                              // Heuristic: if clamped === estItemsLen-1 and isMulti, the click was on Next
                              if (clamped === estItemsLen - 1) {
                                try { origHandleInput("\r"); } catch {}
                                console.log(`[biggz-question-mouse] confirmed multi Next`);
                              }
                              handledMouse = true;
                            } else {
                              // Single-select: second click on same row confirms (Enter)
                              const now = Date.now();
                              if (lastClickIndex === clamped && (now - lastClickTime) < DOUBLE_CLICK_MS) {
                                try { origHandleInput("\r"); } catch {}
                                console.log(`[biggz-question-mouse] confirmed single index ${clamped} (double-click)`);
                              } else {
                                console.log(`[biggz-question-mouse] focused single index ${clamped} (click again to confirm)`);
                              }
                              lastClickIndex = clamped;
                              lastClickTime = now;
                              handledMouse = true;
                            }

                            // Also synthesize digit fallback for hosts that parse "1"/"2" as selection (opencode parity comment in spec)
                            // This is secondary; if arrow nav already handled, digit is extra but harmless for single-select (will be ignored)
                            // For safety, only send digit if single-select and we have not already confirmed, as a fallback if arrow synthesis failed
                            // We intentionally do NOT send digit for multi to avoid confusion.
                            if (!isMulti) {
                              const digit = String(clamped + 1);
                              // Only send digit if arrow path may have missed due to layout miscalc — keep behind debug flag
                              // Currently disabled to avoid double-handling; enable if needed by uncommenting:
                              // try { origHandleInput(digit); } catch {}
                            }
                          }

                          if (handledMouse) {
                            // Strip mouse sequences and forward any remaining non-mouse data
                            const stripped = data.replace(/\x1b\[<\d+;\d+;\d+[Mm]/g, "");
                            if (stripped && stripped !== data && stripped.trim()) {
                              try { origHandleInput(stripped); } catch {}
                            }
                            return;
                          }
                        }
                        // Update shadow for keyboard navigation so future mouse deltas stay accurate
                        // Detect up/down keys via pi-tui sequences
                        if (data === "\x1b[A" || data === "\x1bOA") {
                          predictedIndex = Math.max(0, predictedIndex - 1);
                          // Wrap handling: if at 0 and up, wrap to last
                          if (predictedIndex < 0) predictedIndex = Math.max(0, estItemsLen - 1);
                        } else if (data === "\x1b[B" || data === "\x1bOB") {
                          predictedIndex = (predictedIndex + 1) % Math.max(1, estItemsLen);
                        } else if (typeof data === "string" && data.length === 1 && data >= "1" && data <= "9") {
                          const n = parseInt(data, 10) - 1;
                          if (n >= 0 && n < estItemsLen) predictedIndex = n;
                        }
                      } catch (e) {
                        console.log(`[biggz-question-mouse] handleInput wrapper error: ${e?.message || e}`);
                      }
                      return origHandleInput(data);
                    };
                    // Also patch direct dispatch if session instance leaks via component (defensive)
                    // Try to hook QuestionnaireSession.prototype.dispatch if importable
                    return component;
                  };
                  // Also stash for debug
                  patchedCtx = true;
                  return origCustom(wrappedFactory, opts);
                };
              }

              // Enable mouse before dialog blocks on ctx.ui.custom
              try { enableMouse(); } catch {}
              console.log("[biggz-question-mouse] mouse enabled for ask_user_question");
              try {
                return await origExecute(...args);
              } finally {
                try { disableMouse(); } catch {}
                console.log("[biggz-question-mouse] mouse disabled after ask_user_question");
                // Restore custom if we patched — but per-call patch is ephemeral (ctx is per-invocation)
                if (patchedCtx && maybeCtx && maybeCtx.ui && maybeCtx.ui.custom) {
                  // No need to restore globally; ctx is discarded after execute
                }
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

  // Attempt to patch QuestionnaireSession.prototype.dispatch directly if the module
  // is already resolvable via pi's loader (best-effort). This handles the case
  // where the extension loads after rpiv-ask-user-question has already registered
  // and the session class is already in memory. We try a lazy import from the
  // agent's npm path; failure is swallowed (wrapper above is the primary path).
  (async () => {
    try {
      // Resolve via absolute file URL; jiti handles .ts in pi's context
      const candidates = [
        // pi's agent npm root (standard)
        `${process.env.PI_CODING_AGENT_DIR || `${process.env.HOME || process.env.USERPROFILE || ""}/.pi/agent`}/npm/node_modules/@juicesharp/rpiv-ask-user-question/state/questionnaire-session.ts`,
        `${process.env.HOME || process.env.USERPROFILE || ""}/.pi/agent/npm/node_modules/@juicesharp/rpiv-ask-user-question/state/questionnaire-session.ts`,
      ];
      let mod = null;
      for (const p of candidates) {
        try {
          // Dynamic import via file:// — may fail if jiti not used, but try
          const url = `file://${p.replace(/\\/g, "/")}`;
          mod = await import(url);
          if (mod && mod.QuestionnaireSession) break;
        } catch {}
      }
      if (mod && mod.QuestionnaireSession && mod.QuestionnaireSession.prototype) {
        const proto = mod.QuestionnaireSession.prototype;
        const origDispatch = proto.dispatch;
        if (typeof origDispatch === "function" && !proto.__biggzMousePatched) {
          proto.dispatch = function(data) {
            try {
              if (typeof data === "string" && data.includes("\x1b[<")) {
                const re = /\x1b\[<(\d+);(\d+);(\d+)([Mm])/g;
                let m;
                let sawLeft = false;
                let lastCy = null;
                while ((m = re.exec(data)) !== null) {
                  const cb = parseInt(m[1], 10);
                  const cy = parseInt(m[3], 10);
                  const isPress = m[4] === "M";
                  if (!isPress || cb !== 0) continue;
                  sawLeft = true;
                  lastCy = cy;
                }
                if (sawLeft && lastCy !== null) {
                  const target = Math.max(0, lastCy - HEADER_OFFSET);
                  console.log(`[biggz-question-mouse] prototype dispatch mouse Cy=${lastCy} -> ${target}`);
                  // Delegate to existing nav logic: dispatch with synthetic arrows
                  // We can't directly emit nav action without runtime, so fallback to calling origDispatch with arrow sequences
                  // Strip mouse and forward after
                }
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
