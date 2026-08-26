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

  it('scenario 1: blocking still enforced on missing markers (advise off and on)', async () => {
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
      // ensure internal last is clear and ctx has missing
      mock._biggzSynthesisGate._test.clearLast();
      const ctx = makeCtx(missingMarkdown, ctxNotify);
      const result = await wrapped.execute('id1', {}, null, null, ctx);
      assert.equal(result.isError, true, `should block when missing (advise=${advise})`);
      assert.ok(String(result.content[0].text).includes('Please synthesize'), 'error instructs synthesis');
      assert.equal(originalCalled, false, 'original should not be called when blocked');
      // notify should contain error
      const hasErrorNotify = ctxNotify.some((n) => String(n.msg).includes('Please synthesize')) || mock._notifyCalls.some((n) => String(n.msg).includes('Please synthesize'));
      assert.ok(hasErrorNotify, 'should notify error');
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

    // also verify tool_call secondary handler emits concern
    const handler = mock._getToolCallHandler();
    assert.ok(handler, 'tool_call handler registered');
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
    const ctx = makeCtx(thinMarkdown, ctxNotify);
    const result = await wrapped.execute('id1', {}, null, null, ctx);
    assert.equal(originalCalled, true, 'should allow when advise off');
    assert.equal(result.isError, undefined);
    const allNotifies = [...ctxNotify, ...mock._notifyCalls];
    const concern = allNotifies.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern, undefined, `should not emit concern when advise off, got ${JSON.stringify(allNotifies)}`);
    // tool_call handler also silent
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
    // sanity: rich is not thin
    assert.equal(mock._biggzSynthesisGate.isThinSynthesis(richMarkdown), false);
    const ctx = makeCtx(richMarkdown, ctxNotify);
    const result = await wrapped.execute('id1', {}, null, null, ctx);
    assert.equal(originalCalled, true, 'rich should allow');
    assert.equal(result.isError, undefined);
    const allNotifies = [...ctxNotify, ...mock._notifyCalls];
    const concern = allNotifies.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern, undefined, `rich should not emit concern, got ${JSON.stringify(allNotifies)}`);
    // tool_call also silent
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
});
