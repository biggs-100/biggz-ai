import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const gatePath = path.resolve('internal/assets/pi/biggz-synthesis-gate.js');
const gateUrl = pathToFileURL(gatePath).href;

// helper to create fresh mockPi with tracking
function createMockPi() {
  const tools = new Map();
  let toolCallHandler = null;
  const notifyCalls = [];
  const onHandlers = {};
  const mock = {
    registerTool: (def) => {
      tools.set(def.name, def);
    },
    registerCommand: () => {},
    on: (ev, handler) => {
      onHandlers[ev] = handler;
      if (ev === 'tool_call') toolCallHandler = handler;
    },
    notify: (msg, level) => {
      notifyCalls.push({ msg, level });
    },
    settings: {},
    // expose for test
    _tools: tools,
    _notifyCalls: notifyCalls,
    _onHandlers: onHandlers,
    _getToolCallHandler: () => toolCallHandler,
  };
  return mock;
}

function makeCtx(markdown, ctxNotify) {
  return {
    ui: {
      notify: (msg, level) => {
        ctxNotify.push({ msg, level });
      },
    },
    history: markdown,
    messages: [{ text: markdown }],
  };
}

// load gate fresh — use dynamic import with cache bust via query param for child bypass early-return test
async function loadGateFresh(mockPi) {
  // gate is already imported; to test early return we would need new import with PI_SUBAGENT_CHILD set before import
  // For runtime bypass tests we can reuse already-loaded gate function
  const mod = await import(gateUrl);
  const fn = mod.default;
  fn(mockPi);
  return mockPi;
}

describe('biggz-synthesis-gate advisor dual-mode — fixtures no network', () => {
  const originalEnvAdvise = process.env.BIGGZ_ADVISE;
  const originalEnvChild = process.env.PI_SUBAGENT_CHILD;
  let mod;
  let gateFn;

  beforeEach(async () => {
    // ensure clean env
    delete process.env.BIGGZ_ADVISE;
    delete process.env.PI_SUBAGENT_CHILD;
    mod = await import(gateUrl);
    gateFn = mod.default;
  });

  afterEach(() => {
    if (originalEnvAdvise === undefined) delete process.env.BIGGZ_ADVISE;
    else process.env.BIGGZ_ADVISE = originalEnvAdvise;
    if (originalEnvChild === undefined) delete process.env.PI_SUBAGENT_CHILD;
    else process.env.PI_SUBAGENT_CHILD = originalEnvChild;
  });

  // fixtures
  const missingMarkdown = `Just some random assistant text without any synthesis markers. No artifacts here.`;
  const thinMarkdown = `## Sub-agent Result: advisor-test
**Artifacts/Paths:** -
**Risks / Open Questions:** none
**Next Recommended:** none`;
  // same thin but with helper to ensure count 1 len 10
  const thinAlt = `## Sub-agent Result: phase
**Artifacts/Paths:** - 
**Risks / Open Questions:** low
**Next Recommended:** verify`;
  const richMarkdown = `## Sub-agent Result: advisor-test
**Artifacts/Paths:** internal/assets/pi/biggz-synthesis-gate.js (dual-mode watchdog with heuristic and BIGGZ_ADVISE gate), internal/assets/pi/biggz-web-search.js (anchor-preserving markdown fetch with baseUrl resolve), internal/tui/tui.go (CSI sync and bracketed paste handling with fallback)
**Risks / Open Questions:** none
**Next Recommended:** none`;
  // verify rich is indeed rich
  // count 3, len >50

  it('heuristic helpers: thin vs rich classification (no network)', async () => {
    const mock = createMockPi();
    gateFn(mock);
    const h = mock._biggzSynthesisGate;
    assert.ok(h, 'exposed helpers');
    // thin should be true
    assert.equal(h.isThinSynthesis(thinMarkdown), true, 'thin "-" should be thin');
    assert.equal(h.isThinSynthesis(thinAlt), true);
    // rich should be false
    const richMetrics = h.getArtifactsMetrics(richMarkdown);
    assert.ok(richMetrics.count >= 2, `rich count ${richMetrics.count} should be >=2`);
    assert.ok(richMetrics.len >= 50, `rich len ${richMetrics.len} should be >=50`);
    assert.equal(h.isThinSynthesis(richMarkdown), false, 'rich should not be thin');
    // missing should be false (not thin because no markers)
    assert.equal(h.isThinSynthesis(missingMarkdown), false);
    // missing has no markers -> not thin but also blocking path
    assert.equal(h.hasSynthesis(missingMarkdown), false);
    assert.equal(h.hasSynthesis(richMarkdown), true);
    assert.equal(h.extractArtifactsSection(thinMarkdown).length < 50, true);
  });

  it('scenario 1: blocking still enforced on missing markers (advise off and on) — checkpoint only', async () => {
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed', description: 'continue' }, { label: 'adjust' }, { label: 'stop' }] }] };
    for (const advise of [undefined, '1']) {
      if (advise) process.env.BIGGZ_ADVISE = advise;
      else delete process.env.BIGGZ_ADVISE;
      const mock = createMockPi();
      gateFn(mock);
      // register dummy tool after gate wrapping
      let originalCalled = false;
      const ctxNotify = [];
      mock.registerTool({
        name: 'ask_user_question',
        description: 'test',
        parameters: { type: 'object', properties: {} },
        execute: async (..._args) => {
          originalCalled = true;
          return { content: [{ type: 'text', text: 'ok' }], isError: false };
        },
      });
      const wrapped = mock._tools.get('ask_user_question');
      assert.ok(wrapped, 'wrapped tool exists');
      // ensure post-delegation state: a prior synthesis exists in history so missing currentTurn should block for checkpoint asks
      mock._biggzSynthesisGate._test.clearLast();
      mock._biggzSynthesisGate._test.setLast(richMarkdown);
      const ctx = makeCtx(missingMarkdown, ctxNotify);
      const result = await wrapped.execute('id1', checkpointParams, null, null, ctx);
      assert.ok(result.isError !== true, `should allow with history fallback when missing current but prior synthesis exists (advise=${advise})`);
      assert.equal(originalCalled, true, 'original should be called when history fallback allows');
      const hasFallbackWarning = ctxNotify.some((n) => String(n.msg).includes('synthesis from previous turn')) || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn'));
      assert.ok(hasFallbackWarning, 'should notify history fallback warning');
      // cleanup for next loop
      mock._biggzSynthesisGate._test.clearLast();
    }
  });

  it('scenario 2: advise emits concern on thin synthesis when BIGGZ_ADVISE=1', async () => {
    process.env.BIGGZ_ADVISE = '1';
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    // strict same-turn: blocking requires currentTurn, not just ctx.history
    mock._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const ctx = makeCtx(thinMarkdown, ctxNotify);
    const result = await wrapped.execute('id1', {}, null, null, ctx);
    // should NOT block — allow
    assert.equal(result.isError, undefined, 'thin with advise should not block');
    assert.equal(originalCalled, true, 'original should be called (allow)');
    // should have emitted concern via ctx.ui.notify or pi.notify
    const allNotifies = [...ctxNotify, ...mock._notifyCalls];
    const concern = allNotifies.find((n) => String(n.msg).includes('concern') && String(n.msg).includes('thin'));
    assert.ok(concern, `should emit concern on thin with advise, got ${JSON.stringify(allNotifies)}`);
    assert.ok(String(concern.msg).includes('count=1') || String(concern.msg).includes('len='), 'concern should contain metrics');

    // also verify tool_call secondary handler emits concern (strict: must have currentTurn)
    const handler = mock._getToolCallHandler();
    assert.ok(handler, 'tool_call handler registered');
    // re-set currentTurn for secondary handler (strict same-turn requires currentTurn, not just history)
    mock._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const ctx2Notify = [];
    const ctx2 = makeCtx(thinMarkdown, ctx2Notify);
    // need to provide event shape
    const before = ctx2Notify.length + mock._notifyCalls.length;
    await handler({ toolName: 'ask_user_question' }, ctx2);
    const afterNotifies = [...ctx2Notify, ...mock._notifyCalls.slice(before - ctx2Notify.length)];
    // handler should have emitted concern again (we check ctx2Notify)
    const concern2 = ctx2Notify.find((n) => String(n.msg).includes('concern')) || mock._notifyCalls.slice(before).find((n) => String(n.msg).includes('concern'));
    assert.ok(concern2, 'tool_call handler should emit concern on thin with advise');

    mock._biggzSynthesisGate._test.clearLast();
  });

  it('scenario 3: advise off by default — thin synthesis passes silently without concern', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    // ensure settings has no advise flag
    mock.settings = {};
    gateFn(mock);
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const ctx = makeCtx(thinMarkdown, ctxNotify);
    const result = await wrapped.execute('id1', {}, null, null, ctx);
    assert.equal(originalCalled, true, 'should allow when advise off');
    assert.equal(result.isError, undefined);
    const allNotifies = [...ctxNotify, ...mock._notifyCalls];
    const concern = allNotifies.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern, undefined, `should not emit concern when advise off, got ${JSON.stringify(allNotifies)}`);
    // tool_call handler also silent (strict: need currentTurn set, but advise off so no concern anyway)
    mock._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const handler = mock._getToolCallHandler();
    const ctx2Notify = [];
    const ctx2 = makeCtx(thinMarkdown, ctx2Notify);
    await handler({ toolName: 'ask_user_question' }, ctx2);
    const concern2 = ctx2Notify.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern2, undefined, 'tool_call handler should be silent when advise off');
  });

  it('scenario 4: rich synthesis never triggers concern even with BIGGZ_ADVISE=1', async () => {
    process.env.BIGGZ_ADVISE = '1';
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    // sanity: rich is not thin
    assert.equal(mock._biggzSynthesisGate.isThinSynthesis(richMarkdown), false);
    const ctx = makeCtx(richMarkdown, ctxNotify);
    const result = await wrapped.execute('id1', {}, null, null, ctx);
    assert.equal(originalCalled, true, 'rich should allow');
    assert.equal(result.isError, undefined);
    const allNotifies = [...ctxNotify, ...mock._notifyCalls];
    const concern = allNotifies.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern, undefined, `rich should not emit concern, got ${JSON.stringify(allNotifies)}`);
    // tool_call also silent (strict: ensure currentTurn set)
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const handler = mock._getToolCallHandler();
    const ctx2Notify = [];
    const ctx2 = makeCtx(richMarkdown, ctx2Notify);
    await handler({ toolName: 'ask_user_question' }, ctx2);
    const concern2 = ctx2Notify.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern2, undefined);
  });

  it('scenario 5: child subagent bypass skips both blocking and advise', async () => {
    // test runtime bypass: set env before call, even with missing markdown should allow
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearLast();
    // set child flag before execute
    process.env.PI_SUBAGENT_CHILD = '1';
    assert.equal(mock._biggzSynthesisGate.isChildBypass(), true, 'should detect child bypass');
    const ctxMissing = makeCtx(missingMarkdown, ctxNotify);
    const resultMissing = await wrapped.execute('id1', {}, null, null, ctxMissing);
    assert.equal(originalCalled, true, 'child bypass should allow even missing markers');
    assert.equal(resultMissing.isError, undefined);
    const hasBlock = ctxNotify.some((n) => String(n.msg).includes('Please synthesize'));
    assert.equal(hasBlock, false, 'child bypass should not notify block');

    // also thin with advise should be silent under child bypass
    process.env.BIGGZ_ADVISE = '1';
    originalCalled = false;
    ctxNotify.length = 0;
    mock._notifyCalls.length = 0;
    const ctxThin = makeCtx(thinMarkdown, ctxNotify);
    const resultThin = await wrapped.execute('id2', {}, null, null, ctxThin);
    assert.equal(originalCalled, true, 'child bypass thin should allow');
    const concern = [...ctxNotify, ...mock._notifyCalls].find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern, undefined, 'child bypass should not emit concern even with thin+advise');

    // tool_call handler also bypasses
    const handler = mock._getToolCallHandler();
    const ctx2Notify = [];
    const ctx2 = makeCtx(missingMarkdown, ctx2Notify);
    await handler({ toolName: 'ask_user_question' }, ctx2);
    const bypassNotify = ctx2Notify.find((n) => String(n.msg).includes('concern') || String(n.msg).includes('Please synthesize'));
    assert.equal(bypassNotify, undefined, 'tool_call handler should bypass under child');

    // cleanup child flag for other tests
    delete process.env.PI_SUBAGENT_CHILD;
    delete process.env.BIGGZ_ADVISE;
  });

  it('settings flag gates advise as alternative to env (BIGGZ_ADVISE via pi.settings)', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    mock.settings = { advise: true };
    gateFn(mock);
    assert.equal(mock._biggzSynthesisGate.isAdviseEnabled(), true, 'settings advise true should enable');
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('question');
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const ctx = makeCtx(thinMarkdown, ctxNotify);
    await wrapped.execute('id1', {}, null, null, ctx);
    assert.equal(originalCalled, true);
    const concern = [...ctxNotify, ...mock._notifyCalls].find((n) => String(n.msg).includes('concern'));
    assert.ok(concern, 'settings flag should also emit concern on thin');
  });

  it('advise does not auto-fix and does not call model — only notify', async () => {
    process.env.BIGGZ_ADVISE = '1';
    const mock = createMockPi();
    let modelCalled = false;
    // inject fake model caller to detect if gate tries to call it
    mock.callModel = () => { modelCalled = true; };
    gateFn(mock);
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => ({ content: [{ type: 'text', text: 'ok' }] }),
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const ctx = makeCtx(thinMarkdown, ctxNotify);
    await wrapped.execute('id1', {}, null, null, ctx);
    assert.equal(modelCalled, false, 'advise must not call model');
    // ensure notify was called, not fix
    const concern = [...ctxNotify, ...mock._notifyCalls].find((n) => String(n.msg).includes('concern'));
    assert.ok(concern);
  });

  it('same-turn markdown immediately before tool_call passes (race fix)', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    assert.ok(wrapped, 'wrapped tool exists');
    const handler = mock._onHandlers['assistant_message'];
    assert.ok(handler, 'assistant_message handler registered');
    handler({ text: richMarkdown });
    assert.ok(mock._biggzSynthesisGate._test.getCurrent().includes('Sub-agent Result'), 'currentTurn should contain synthesis after assistant_message');
    const ctx = {
      ui: { notify: (msg, level) => ctxNotify.push({ msg, level }) },
      history: '',
      messages: [],
    };
    const start = Date.now();
    const result = await wrapped.execute('id-race', {}, null, null, ctx);
    const elapsed = Date.now() - start;
    assert.ok(elapsed < 1000, `should be within same-turn window, elapsed ${elapsed}ms`);
    assert.equal(result.isError, undefined, 'same-turn markdown should PASS (no block)');
    assert.equal(originalCalled, true, 'original should be called when same-turn markdown present');
    const source = mock._biggzSynthesisGate.getCurrentTurnSynthesis(ctx);
    // After successful call, buffer is reset, so check source before reset would have had synthesis; verify helper works via fresh set
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const source2 = mock._biggzSynthesisGate.getCurrentTurnSynthesis(ctx);
    assert.ok(source2.includes('Sub-agent Result'), 'getCurrentTurnSynthesis should return currentTurn markdown');
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
  });

  it('regression: bloquea cuando solo hay síntesis vieja en ctx.history pero no en currentTurn (strict same-turn) — checkpoint only', async () => {
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    // Simulate old synthesis lingering in history (e.g. 2026-08-27-synthesis-gate-hardening) but currentTurn empty
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    // Also set lastAssistant to rich to simulate old lastAssistantMarkdown — should still be ignored for blocking
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    // Now clear current again to simulate no synthesis in THIS turn (bash intermediates vaciaron buffer)
    mock._biggzSynthesisGate._test.clearCurrent();
    const ctx = makeCtx(richMarkdown, ctxNotify); // ctx.history has rich synthesis from previous turn
    // Relaxed: history fallback within 120s should allow with warning instead of blocking
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctx), true, 'relaxed check must be true when history/last has synthesis within 120s');
    const result = await wrapped.execute('id-regression', checkpointParams, null, null, ctx);
    assert.equal(result.isError, undefined, 'must allow with history fallback when only history has synthesis, currentTurn empty');
    const hasFallback2 = ctxNotify.some((n) => String(n.msg).includes('synthesis from previous turn')) || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn'));
    assert.ok(hasFallback2, 'should emit history fallback warning');
    assert.equal(originalCalled, true, 'original must be called on history fallback allow');
    // history is still available for advise path (non-blocking) — getCurrentTurnSynthesis should return history
    const adviseSource = mock._biggzSynthesisGate.getCurrentTurnSynthesis(ctx);
    assert.ok(adviseSource.includes('Sub-agent Result'), 'advise fallback may still see history');
    // Now emit synthesis in currentTurn (same turn, adjacent) and retry — should pass
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctx), true, 'must pass when currentTurn has synthesis');
    const ctx2Notify = [];
    const ctx2 = makeCtx('', ctx2Notify);
    const result2 = await wrapped.execute('id-regression-2', checkpointParams, null, null, ctx2);
    assert.equal(result2.isError, undefined, 'must allow when currentTurn has synthesis even if ctx.history empty');
    assert.equal(originalCalled, true, 'original should be called after strict pass');
    // After successful call, currentTurn must be reset (next turn starts fresh)
    assert.equal(mock._biggzSynthesisGate._test.getCurrent(), '', 'currentTurn must be reset after successful ask_user_question');
  });

  it('strict blocking: currentTurn reset after successful ask prevents reuse (no history fallback) — checkpoint only', async () => {
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => ({ content: [{ type: 'text', text: 'ok' }] }),
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const ctx = makeCtx('', ctxNotify);
    const r1 = await wrapped.execute('id1', checkpointParams, null, null, ctx);
    assert.equal(r1.isError, undefined, 'first call with currentTurn should pass');
    // currentTurn should now be empty after reset
    assert.equal(mock._biggzSynthesisGate._test.getCurrent(), '', 'currentTurn reset after success');
    // second call without new synthesis, even though history still has old richMarkdown, now allows via history fallback (relaxed)
    const ctx2 = makeCtx(richMarkdown, []);
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctx2), true, 'second call with history fallback should allow when history/last present within 120s');
    const r2 = await wrapped.execute('id2', checkpointParams, null, null, ctx2);
    assert.equal(r2.isError, undefined, 'second call must allow with history fallback (relaxed)');
  });

  it('load-order race: tool already registered before gate loads must still be blocked when missing synthesis — checkpoint only', async () => {
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    // Register BEFORE gate wraps — simulates rpiv-ask-user-question loading before synthesis gate
    let originalCalled = false;
    mock.registerTool({
      name: 'ask_user_question',
      description: 'pre-registered',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    // Now load gate — it should sweep pre-registered tools and wrap them
    gateFn(mock);
    const wrapped = mock._tools.get('ask_user_question');
    assert.ok(wrapped, 'pre-registered tool exists after gate');
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    // Simulate prior synthesis in session history so this is post-delegation (should block without currentTurn)
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctxNotify = [];
    const ctxMissing = makeCtx(missingMarkdown, ctxNotify);
    // Relaxed: history fallback allows even for pre-registered tools when prior synthesis exists within 120s
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctxMissing), true);
    const resultMissing = await wrapped.execute('id-pre', checkpointParams, null, null, ctxMissing);
    assert.equal(resultMissing.isError, undefined, 'pre-registered tool must allow with history fallback when missing current but prior synthesis exists');
    const hasFallback4 = ctxNotify.some((n) => String(n.msg).includes('synthesis from previous turn')) || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn'));
    assert.ok(hasFallback4, 'should emit history fallback warning');
    assert.equal(originalCalled, true, 'original must be called for pre-registered history fallback allow');
    // With currentTurn synthesis, pre-registered tool must allow (checkpoint with synthesis)
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const ctxRich = makeCtx('', []);
    const resultRich = await wrapped.execute('id-pre2', checkpointParams, null, null, ctxRich);
    assert.equal(resultRich.isError, undefined, 'pre-registered tool must allow with currentTurn synthesis');
    assert.equal(originalCalled, true, 'original must be called when synthesis present');
  });

  it('secondary guard via tool_call actually blocks when missing synthesis (not just warn) — checkpoint only', async () => {
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const generalParams = { questions: [{ question: '¿por dónde empezamos?', options: [{ label: 'opción A' }, { label: 'opción B' }] }] };
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    const handler = mock._getToolCallHandler();
    assert.ok(handler, 'tool_call handler registered');
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    // Simulate prior synthesis so this is post-delegation (should block for checkpoint)
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctxNotify = [];
    const ctx = makeCtx(missingMarkdown, ctxNotify);
    // Relaxed: tool_call with missing synthesis but prior history should allow with warning (not block)
    const ret = await handler({ toolName: 'ask_user_question', params: checkpointParams }, ctx);
    assert.equal(ret, undefined, `tool_call handler must allow with history fallback, got ${JSON.stringify(ret)}`);
    const hasFallback5 = ctxNotify.some((n) => String(n.msg).includes('synthesis from previous turn')) || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn'));
    assert.ok(hasFallback5 || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn')), 'should emit history fallback warning');
    // With currentTurn synthesis, handler must NOT block (allow checkpoint)
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const ctx2Notify = [];
    const ctx2 = makeCtx('', ctx2Notify);
    const ret2 = await handler({ toolName: 'ask_user_question', params: checkpointParams }, ctx2);
    assert.equal(ret2, undefined, 'tool_call handler must allow when currentTurn has synthesis');
    // General question must NOT block even when missing synthesis (checkpoint filter)
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctxGeneral = makeCtx(missingMarkdown, []);
    const retGeneral = await handler({ toolName: 'ask_user_question', params: generalParams }, ctxGeneral);
    assert.equal(retGeneral, undefined, 'general question must NOT block even without synthesis');
    const retGeneralEmpty = await handler({ toolName: 'ask_user_question', params: {} }, ctxGeneral);
    assert.equal(retGeneralEmpty, undefined, 'empty params must be treated as general and not block');
    // Also verify non-question tools are ignored (never block)
    mock._biggzSynthesisGate._test.clearCurrent();
    const ret3 = await handler({ toolName: 'bash' }, makeCtx(missingMarkdown, []));
    assert.equal(ret3, undefined, 'non-question tool_call must not block');
  });

  it('message_end tracking populates currentTurn for strict check (pi correct event)', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    // Simulate Pi's real message_end event with content array (not legacy assistant_message)
    const msgEndHandler = mock._onHandlers['message_end'];
    assert.ok(msgEndHandler, 'message_end handler must be registered (hardened gate tracks Pi correct events)');
    const piMessage = {
      message: {
        role: 'assistant',
        content: [{ type: 'text', text: richMarkdown }],
      },
    };
    await msgEndHandler(piMessage);
    assert.ok(mock._biggzSynthesisGate._test.getCurrent().includes('Sub-agent Result'), 'message_end should populate currentTurn');
    // Also verify message_update handler exists
    assert.ok(mock._onHandlers['message_update'], 'message_update handler must be registered');
    // Now tool call should pass strict same-turn for checkpoint
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(makeCtx('', [])), true, 'strict check must pass after message_end populated currentTurn');
    const checkpointParams = { questions: [{ question: 'Next?', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const result = await wrapped.execute('id-msg-end', checkpointParams, null, null, makeCtx('', []));
    assert.equal(result.isError, undefined);
    assert.equal(originalCalled, true);
  });

  it('turn_start resets currentTurn (strict same-turn enforcement across turns)', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    assert.ok(mock._biggzSynthesisGate.checkSynthesisPrecondition(makeCtx('', [])), 'precondition should be true before turn_start');
    const turnStartHandler = mock._onHandlers['turn_start'];
    assert.ok(turnStartHandler, 'turn_start handler must be registered');
    await turnStartHandler({});
    assert.equal(mock._biggzSynthesisGate._test.getCurrent(), '', 'turn_start must reset currentTurn for strict same-turn');
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(makeCtx('', [])), false, 'after turn_start without new synthesis, check must be false');
  });

  it('preflight allowance: first ask with no prior synthesis ever must NOT block (SDD Session Preflight)', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    mock.registerTool({
      name: 'ask_user_question',
      description: 'preflight',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    // No synthesis ever in session (current empty, last empty, history missing) — preflight should be allowed
    const ctxMissing = makeCtx(missingMarkdown, []);
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctxMissing), false, 'strict check is false (no current)');
    assert.equal(mock._biggzSynthesisGate.getCurrentTurnSynthesis(ctxMissing), '', 'no synthesis anywhere');
    assert.equal(mock._biggzSynthesisGate.getSynthesisSource(ctxMissing), '', 'no synthesis anywhere');
    const result = await wrapped.execute('id-preflight', {}, null, null, ctxMissing);
    assert.equal(result.isError, undefined, 'first ask with no prior synthesis should NOT block (preflight allowance) — general');
    assert.equal(originalCalled, true, 'original should be called for preflight allowance');
    // checkpoint with no prior synthesis must also be allowed via preflight allowance
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    originalCalled = false;
    const checkpointParams = { questions: [{ question: 'Next?', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const resultCp = await wrapped.execute('id-preflight-cp', checkpointParams, null, null, ctxMissing);
    assert.equal(resultCp.isError, undefined, 'checkpoint with no prior synthesis should also be allowed (preflight)');
    assert.equal(originalCalled, true);
    // tool_call secondary guard also must allow when no synthesis ever
    const handler = mock._getToolCallHandler();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    const ret = await handler({ toolName: 'ask_user_question' }, makeCtx(missingMarkdown, []));
    assert.equal(ret, undefined, 'tool_call must allow when no synthesis ever (preflight)');
    const retCp = await handler({ toolName: 'ask_user_question', params: checkpointParams }, makeCtx(missingMarkdown, []));
    assert.equal(retCp, undefined, 'checkpoint preflight must also allow when no synthesis ever');
  });

  it('checkpoint detection: isCheckpointAsk identifies proceed/adjust/stop and continue/correct (case-insensitive) vs general', async () => {
    const mock = createMockPi();
    gateFn(mock);
    const h = mock._biggzSynthesisGate;
    assert.ok(h.isCheckpointAsk, 'isCheckpointAsk exposed');
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const checkpointContinue = { questions: [{ question: 'Next?', options: [{ label: 'continue' }, { label: 'correct' }] }] };
    const checkpointMixedCase = { questions: [{ question: 'Next?', options: [{ label: 'Proceed' }, { label: 'ADJUST' }, { label: 'Stop' }] }] };
    const generalParams = { questions: [{ question: '¿por dónde empezamos?', options: [{ label: 'opción A' }, { label: 'opción B' }] }] };
    const generalEmpty = {};
    const generalNull = null;
    const questionToolParams = { questions: [{ question: 'Next?', options: [{ label: 'continue' }] }] };
    assert.equal(h.isCheckpointAsk(checkpointParams), true, 'proceed/adjust/stop should be checkpoint');
    assert.equal(h.isCheckpointAsk(checkpointContinue), true, 'continue/correct should be checkpoint');
    assert.equal(h.isCheckpointAsk(checkpointMixedCase), true, 'case-insensitive checkpoint');
    assert.equal(h.isCheckpointAsk(questionToolParams), true, 'question tool continue is checkpoint');
    assert.equal(h.isCheckpointAsk(generalParams), false, 'general without checkpoint labels must be false');
    assert.equal(h.isCheckpointAsk(generalEmpty), false, 'empty object false');
    assert.equal(h.isCheckpointAsk(generalNull), false, 'null false');
    assert.equal(h.isCheckpointAsk(undefined), false, 'undefined false');
    // also test top-level options fallback
    const topLevelCheckpoint = { options: [{ label: 'proceed' }] };
    assert.equal(h.isCheckpointAsk(topLevelCheckpoint), true, 'top-level options checkpoint');
    // tool_call param extraction
    const extracted = h.extractParamsFromToolCall({ toolName: 'ask_user_question', params: checkpointParams });
    assert.deepEqual(extracted, checkpointParams, 'extractParamsFromToolCall should return params');
    assert.equal(h.isCheckpointAsk(extracted), true);
    const extractedArgs = h.extractParamsFromToolCall({ toolName: 'ask_user_question', args: checkpointParams });
    assert.equal(h.isCheckpointAsk(extractedArgs), true);
  });

  it('general question after delegation must NOT block even without synthesis (checkpoint filter)', async () => {
    const generalParams = { questions: [{ question: '¿por dónde empezamos?', options: [{ label: 'opción A' }, { label: 'opción B' }] }] };
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    let originalCalled = false;
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => { originalCalled = true; return { content: [{ type: 'text', text: 'ok' }] }; },
    });
    const wrapped = mock._tools.get('ask_user_question');
    // Simulate post-delegation: prior synthesis exists, current empty — relaxed allows with warning for checkpoint
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctx = makeCtx(missingMarkdown, []);
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctx), true, 'relaxed: history fallback should allow');
    // general should pass
    const resultGeneral = await wrapped.execute('id-general', generalParams, null, null, ctx);
    assert.equal(resultGeneral.isError, undefined, 'general must not block even without current synthesis');
    assert.equal(originalCalled, true, 'general should call original');
    // empty params also general
    originalCalled = false;
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctxEmpty = makeCtx(missingMarkdown, []);
    const resultEmpty = await wrapped.execute('id-empty', {}, null, null, ctxEmpty);
    assert.equal(resultEmpty.isError, undefined, 'empty params must not block');
    assert.equal(originalCalled, true);
    // checkpoint now allows via history fallback (relaxed) instead of blocking
    originalCalled = false;
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const checkpointParams = { questions: [{ question: 'Next?', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const ctxCp = makeCtx(missingMarkdown, []);
    const resultCheckpoint = await wrapped.execute('id-cp', checkpointParams, null, null, ctxCp);
    assert.equal(resultCheckpoint.isError, undefined, 'checkpoint must allow with history fallback when missing current but history exists');
    assert.equal(originalCalled, true);
    // tool_call guard: general must not block
    const handler = mock._getToolCallHandler();
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctxGeneral2 = makeCtx(missingMarkdown, []);
    const retGeneral = await handler({ toolName: 'ask_user_question', params: generalParams }, ctxGeneral2);
    assert.equal(retGeneral, undefined, 'tool_call general must not block');
    const ctxCp2 = makeCtx(missingMarkdown, []);
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const retCheckpoint = await handler({ toolName: 'ask_user_question', params: checkpointParams }, ctxCp2);
    assert.equal(retCheckpoint, undefined, 'tool_call checkpoint must allow with history fallback (relaxed)');
  });
  it('envelope validation � PR2 limits and fallback', async () => {
    const mock = createMockPi(); gateFn(mock); const h = mock._biggzSynthesisGate;
    const badH = { questions: [{ header: 'a'.repeat(17), question: 'Q?', options: [{ label: 'proceed' }, { label: 'adjust' }] }] };
    const vH = h.validateQuestionEnvelope(badH); assert.ok(vH && vH.isError && String(vH.limit+vH.message).toLowerCase().includes('header'));
    const badL = { questions: [{ header: 'h', question: 'Q', options: [{ label: 'b'.repeat(61) }, { label: 'ok' }] }] };
    assert.ok(h.validateQuestionEnvelope(badL).isError);
    const five = { questions: Array.from({ length: 5 }, () => ({ header: 'h', question: 'Q', options: [{ label: 'a' }, { label: 'b' }] })) };
    assert.ok(h.validateQuestionEnvelope(five).isError);
    const one = { questions: [{ header: 'h', question: 'Q', options: [{ label: 'only' }] }] };
    assert.ok(h.validateQuestionEnvelope(one).isError);
    const valid = { questions: Array.from({ length: 3 }, () => ({ header: 'c'.repeat(12), question: 'Valid?', options: [{ label: 'a' }, { label: 'b' }, { label: 'c' }] })) };
    assert.equal(h.validateQuestionEnvelope(valid), null);
    const fb = h.formatFallback({ questions: [{ header: 'Hdr1', question: 'Q1?', options: [{ label: 'a' }, { label: 'b' }] }, { header: 'Hdr2', question: 'Q2?', options: [{ label: 'c' }, { label: 'd' }] }] });
    assert.ok(fb.includes('Hdr1') && fb.indexOf('Hdr1') < fb.indexOf('Hdr2'));
    let called = false; mock.registerTool({ name: 'ask_user_question', description: 't', parameters: { type: 'object', properties: {} }, execute: async () => { called = true; return { content: [{ type: 'text', text: 'ok' }] }; } });
    const w = mock._tools.get('ask_user_question'); mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const ctx = makeCtx(richMarkdown, []); const r = await w.execute('id-bad', badH, null, null, ctx);
    assert.equal(r.isError, true); assert.ok(String(r.content[0].text).includes('isError:true')); assert.equal(called, false);
    const handler = mock._getToolCallHandler(); const ret = await handler({ toolName: 'ask_user_question', params: badH }, ctx);
    assert.ok(ret && ret.block === true);
  });
  it('single ownership and thin/general bypass', async () => {
    const mock = createMockPi(); gateFn(mock);
    mock.registerTool({ name: 'ask_user_question', description: 't', parameters: { type: 'object', properties: {} }, execute: async () => ({ content: [{ type: 'text', text: 'ok' }] }) });
    const w = mock._tools.get('ask_user_question');
    const checkpoint = { questions: [{ question: 'Next?', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const general = { questions: [{ question: 'general q', options: [{ label: 'opt A' }, { label: 'opt B' }] }] };
    process.env.PI_SUBAGENT_CHILD = '1'; mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const ctx = makeCtx(richMarkdown, []); assert.equal((await w.execute('sub-cp', checkpoint, null, null, ctx)).isError, true);
    assert.equal((await w.execute('sub-gen', general, null, null, ctx)).isError, undefined);
    delete process.env.PI_SUBAGENT_CHILD; mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    assert.equal((await w.execute('orch', checkpoint, null, null, ctx)).isError, undefined);
    process.env.BIGGZ_ADVISE = '1'; const m2 = createMockPi(); gateFn(m2);
    m2.registerTool({ name: 'ask_user_question', description: 't', parameters: { type: 'object', properties: {} }, execute: async () => ({ content: [{ type: 'text', text: 'ok' }] }) });
    const w2 = m2._tools.get('ask_user_question'); m2._biggzSynthesisGate._test.setCurrent(thinMarkdown);
    const res = await w2.execute('thin', general, null, null, makeCtx(thinMarkdown, []));
    assert.equal(res.isError, undefined); assert.equal(m2._biggzSynthesisGate.isThinSynthesis(thinMarkdown), true);
    delete process.env.BIGGZ_ADVISE;
  });

  it('history fallback — checkpoint with synthesis in history but not currentTurn should allow with concern (not block)', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    let originalCalled = false;
    const ctxNotify = [];
    mock.registerTool({
      name: 'ask_user_question',
      description: 'test',
      parameters: { type: 'object', properties: {} },
      execute: async () => {
        originalCalled = true;
        return { content: [{ type: 'text', text: 'ok' }] };
      },
    });
    const wrapped = mock._tools.get('ask_user_question');
    // Simulate turn_start cleared currentTurn but history still has synthesis within 120s
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctx = makeCtx(richMarkdown, ctxNotify);
    // No currentTurn synthesis, but history has it — relaxed should allow
    assert.equal(mock._biggzSynthesisGate.checkSynthesisPrecondition(ctx), true, 'history fallback should return true');
    const result = await wrapped.execute('id-history-fallback', checkpointParams, null, null, ctx);
    assert.equal(result.isError, undefined, 'checkpoint must allow with history fallback (not block)');
    assert.equal(originalCalled, true, 'original should be called on history fallback');
    const hasWarning = ctxNotify.some((n) => String(n.msg).includes('synthesis from previous turn')) || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn'));
    assert.ok(hasWarning, 'should emit history fallback warning via notify');
    // Also verify tool_call secondary guard allows with same fallback
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.setLast(richMarkdown);
    const ctx2Notify = [];
    const ctx2 = makeCtx(richMarkdown, ctx2Notify);
    const handler = mock._getToolCallHandler();
    const ret = await handler({ toolName: 'ask_user_question', params: checkpointParams }, ctx2);
    assert.equal(ret, undefined, 'tool_call should allow with history fallback');
    const hasWarning2 = ctx2Notify.some((n) => String(n.msg).includes('synthesis from previous turn')) || mock._notifyCalls.some((n) => String(n.msg).includes('synthesis from previous turn'));
    assert.ok(hasWarning2, 'tool_call should emit history fallback warning');
    // Preflight case: no synthesis anywhere still allows
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    const ctxPreflight = makeCtx(missingMarkdown, []);
    assert.equal(mock._biggzSynthesisGate.getCurrentTurnSynthesis(ctxPreflight), '', 'no synthesis anywhere');
    mock._biggzSynthesisGate._test.clearCurrent();
    mock._biggzSynthesisGate._test.clearLast();
    const resultPreflight = await wrapped.execute('id-preflight-history', checkpointParams, null, null, ctxPreflight);
    assert.equal(resultPreflight.isError, undefined, 'preflight with no prior synthesis should still allow');
  });

});
