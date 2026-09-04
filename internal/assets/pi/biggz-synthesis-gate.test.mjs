import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const gatePath = path.resolve('internal/assets/pi/biggz-synthesis-gate.js');
const gateUrl = pathToFileURL(gatePath).href;

// helper to create fresh mockPi with tracking — supports multiple tool_call handlers (gate + safety)
function createMockPi() {
  const tools = new Map();
  const toolCallHandlers = [];
  const notifyCalls = [];
  const onHandlers = {};
  const mock = {
    registerTool: (def) => {
      tools.set(def.name, def);
    },
    registerCommand: () => {},
    on: (ev, handler) => {
      // keep last for onHandlers for message_end etc, but accumulate tool_call handlers
      onHandlers[ev] = handler;
      if (ev === 'tool_call') toolCallHandlers.push(handler);
    },
    notify: (msg, level) => {
      notifyCalls.push({ msg, level });
    },
    settings: {},
    // expose for test
    _tools: tools,
    _notifyCalls: notifyCalls,
    _onHandlers: onHandlers,
    _getToolCallHandler: () => async (event, ctx) => {
      // emulate Pi calling handlers sequentially; return first block result if any
      for (const h of toolCallHandlers) {
        const r = await h(event, ctx);
        if (r && (r.block === true || r.isError === true)) return r;
      }
      return undefined;
    },
    _toolCallHandlers: toolCallHandlers,
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

// Registers a dummy ask tool and returns the wrapped def plus a call counter.
function registerDummy(mock, name = 'ask_user_question') {
  const calls = { count: 0 };
  mock.registerTool({
    name,
    description: 'test',
    parameters: { type: 'object', properties: {} },
    execute: async () => {
      calls.count += 1;
      return { content: [{ type: 'text', text: 'ok' }] };
    },
  });
  return { wrapped: mock._tools.get(name), calls };
}

describe('biggz-synthesis-gate retired passthrough — fixtures no network', () => {
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

  const checkpointParams = { questions: [{ question: 'Next?', header: 'Checkpoint', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };

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

  it('passthrough: checkpoint without synthesis allows (enforcement retired)', async () => {
    for (const advise of [undefined, '1']) {
      if (advise) process.env.BIGGZ_ADVISE = advise;
      else delete process.env.BIGGZ_ADVISE;
      const mock = createMockPi();
      gateFn(mock);
      const { wrapped, calls } = registerDummy(mock);
      assert.ok(wrapped, 'wrapped tool exists');
      const h = mock._biggzSynthesisGate;
      h._test.clearLast();
      h._test.clearCurrent();
      h._test.setLast(richMarkdown);
      const ctxNotify = [];
      const result = await wrapped.execute('id1', checkpointParams, null, null, makeCtx(missingMarkdown, ctxNotify));
      assert.equal(result.isError, undefined, `checkpoint without current synthesis must allow (advise=${advise})`);
      assert.equal(calls.count, 1, 'original must be called');
      const blockNotify = [...ctxNotify, ...mock._notifyCalls].some((n) => String(n.msg).includes('Please synthesize before asking'));
      assert.equal(blockNotify, false, 'must never notify block');
      h._test.clearLast();
    }
  });

  it('passthrough: tool_call handler never blocks question tools', async () => {
    const mock = createMockPi();
    gateFn(mock);
    const handler = mock._getToolCallHandler();
    assert.ok(handler, 'tool_call handler registered');
    const h = mock._biggzSynthesisGate;
    h._test.clearCurrent();
    h._test.clearLast();
    h._test.setLast(richMarkdown);
    const ret = await handler({ toolName: 'ask_user_question', params: checkpointParams }, makeCtx(missingMarkdown, []));
    assert.equal(ret, undefined, 'tool_call checkpoint must allow');
    const retGeneral = await handler({ toolName: 'ask_user_question', params: { questions: [{ question: '¿por dónde empezamos?' }] } }, makeCtx(missingMarkdown, []));
    assert.equal(retGeneral, undefined, 'tool_call general must allow');
    const retEmpty = await handler({ toolName: 'ask_user_question', params: {} }, makeCtx(missingMarkdown, []));
    assert.equal(retEmpty, undefined, 'empty params must allow');
    const retOther = await handler({ toolName: 'bash' }, makeCtx(missingMarkdown, []));
    assert.equal(retOther, undefined, 'non-question tools untouched');
  });

  it('passthrough: history-only, expired and empty states all allow', async () => {
    const mock = createMockPi();
    gateFn(mock);
    const { wrapped, calls } = registerDummy(mock);
    const h = mock._biggzSynthesisGate;
    // history-only synthesis (old strict rule blocked here)
    h._test.clearCurrent();
    h._test.clearLast();
    h._test.setLast(richMarkdown);
    const r1 = await wrapped.execute('id-hist', checkpointParams, null, null, makeCtx(richMarkdown, []));
    assert.equal(r1.isError, undefined, 'history-only synthesis must allow');
    // expired window (old strict rule blocked here)
    const originalNow = Date.now;
    try {
      Date.now = () => originalNow() + 121000;
      h._test.setCurrent(richMarkdown);
      const r2 = await wrapped.execute('id-exp', checkpointParams, null, null, makeCtx('', []));
      assert.equal(r2.isError, undefined, 'expired window must allow');
    } finally {
      Date.now = originalNow;
    }
    // no synthesis anywhere (old hard gate blocked checkpoints here)
    h._test.clearCurrent();
    h._test.clearLast();
    const r3 = await wrapped.execute('id-none', checkpointParams, null, null, makeCtx(missingMarkdown, []));
    assert.equal(r3.isError, undefined, 'checkpoint with no synthesis anywhere must allow');
    assert.equal(calls.count, 3, 'all three calls reach original');
  });

  it('advise retired: no concern emitted even with BIGGZ_ADVISE=1', async () => {
    process.env.BIGGZ_ADVISE = '1';
    const mock = createMockPi();
    let modelCalled = false;
    mock.callModel = () => { modelCalled = true; };
    gateFn(mock);
    assert.equal(mock._biggzSynthesisGate.isAdviseEnabled(), true, 'advise flag helper still reports enabled');
    const { wrapped, calls } = registerDummy(mock);
    const h = mock._biggzSynthesisGate;
    h._test.clearLast();
    h._test.clearCurrent();
    h._test.setCurrent(thinMarkdown);
    const ctxNotify = [];
    const result = await wrapped.execute('id1', {}, null, null, makeCtx(thinMarkdown, ctxNotify));
    assert.equal(result.isError, undefined);
    assert.equal(calls.count, 1, 'original called');
    assert.equal(modelCalled, false, 'must not call model');
    const concern = [...ctxNotify, ...mock._notifyCalls].find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern, undefined, 'retired advise must stay silent');
    const handler = mock._getToolCallHandler();
    const ctx2Notify = [];
    await handler({ toolName: 'ask_user_question' }, makeCtx(thinMarkdown, ctx2Notify));
    const concern2 = ctx2Notify.find((n) => String(n.msg).toLowerCase().includes('concern'));
    assert.equal(concern2, undefined, 'tool_call handler stays silent');
    h._test.clearLast();
  });

  it('settings advise flag is inert for enforcement', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    mock.settings = { advise: true };
    gateFn(mock);
    assert.equal(mock._biggzSynthesisGate.isAdviseEnabled(), true, 'settings advise true still reported');
    const { wrapped, calls } = registerDummy(mock, 'question');
    const h = mock._biggzSynthesisGate;
    h._test.clearLast();
    h._test.clearCurrent();
    h._test.setCurrent(thinMarkdown);
    const ctxNotify = [];
    await wrapped.execute('id1', {}, null, null, makeCtx(thinMarkdown, ctxNotify));
    assert.equal(calls.count, 1);
    const concern = [...ctxNotify, ...mock._notifyCalls].find((n) => String(n.msg).includes('concern'));
    assert.equal(concern, undefined, 'settings flag must not emit concern (retired)');
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

  it('envelope helpers: validateQuestionEnvelope limits + formatFallback verbatim', async () => {
    const mock = createMockPi(); gateFn(mock); const h = mock._biggzSynthesisGate;
    const badH = { questions: [{ header: 'a'.repeat(17), question: 'Q?', options: [{ label: 'proceed' }, { label: 'adjust' }] }] };
    const vH = h.validateQuestionEnvelope(badH); assert.ok(vH && vH.isError && String(vH.limit + vH.message).toLowerCase().includes('header'));
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
    // Enforcement retired: even an over-limit envelope reaches the tool —
    // the tool itself owns format validation (single source of truth).
    const { wrapped, calls } = registerDummy(mock);
    h._test.setCurrent(richMarkdown);
    const r = await wrapped.execute('id-bad', badH, null, null, makeCtx(richMarkdown, []));
    assert.equal(r.isError, undefined, 'over-limit envelope must reach the tool (retired)');
    assert.equal(calls.count, 1);
    const handler = mock._getToolCallHandler();
    const ret = await handler({ toolName: 'ask_user_question', params: badH }, makeCtx(richMarkdown, []));
    assert.equal(ret, undefined, 'tool_call must not validate envelope (retired)');
  });

  it('single ownership retired: sub-agent checkpoint executes', async () => {
    const mock = createMockPi(); gateFn(mock);
    const { wrapped } = registerDummy(mock);
    const checkpoint = { questions: [{ question: 'Next?', options: [{ label: 'proceed' }, { label: 'adjust' }, { label: 'stop' }] }] };
    const general = { questions: [{ question: 'general q', options: [{ label: 'opt A' }, { label: 'opt B' }] }] };
    process.env.PI_SUBAGENT_CHILD = '1'; mock._biggzSynthesisGate._test.setCurrent(richMarkdown);
    const ctx = makeCtx(richMarkdown, []);
    assert.equal((await wrapped.execute('sub-cp', checkpoint, null, null, ctx)).isError, undefined, 'retired ownership: child checkpoint executes');
    assert.equal((await wrapped.execute('sub-gen', general, null, null, ctx)).isError, undefined);
    delete process.env.PI_SUBAGENT_CHILD;
    assert.equal(mock._biggzSynthesisGate.isThinSynthesis(thinMarkdown), true, 'thin helper intact');
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
    const bodyTokenNeutralLabels = { questions: [{ header: 'Ritmo', question: '¿Qué ritmo usamos para continuar con el change?', options: [{ label: 'Interactivo' }, { label: 'Automático' }] }] };
    assert.equal(h.isCheckpointAsk(bodyTokenNeutralLabels), false, 'token in question body with neutral labels must be false (labels-only)');
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

  it('hasOptions helper intact — REQ-DG-1 verdicts now passthrough', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi();
    gateFn(mock);
    const h = mock._biggzSynthesisGate;
    // hasOptions helper
    assert.equal(h.hasOptions({ questions: [{ question: 'Q?', options: [{ label: 'Opción A' }, { label: 'Opción B' }] }] }), true, '2 options should be hasOptions true');
    assert.equal(h.hasOptions({ questions: [{ question: 'Q?', options: [{ label: 'A' }, { label: 'B' }, { label: 'C' }, { label: 'D' }] }] }), true, '4 options true');
    assert.equal(h.hasOptions({ questions: [{ question: 'Q?', options: [{ label: 'only' }] }] }), false, '1 option false');
    assert.equal(h.hasOptions({ questions: [{ question: 'Q?' }] }), false, 'no options false');
    assert.equal(h.hasOptions({}), false, 'empty false');
    assert.equal(h.hasOptions({ options: [{ label: 'A' }, { label: 'B' }] }), true, 'top-level 2 options true');
    // every ask shape reaches the tool (retired enforcement)
    const { wrapped, calls } = registerDummy(mock);
    h._test.clearCurrent(); h._test.clearLast(); h._test.setLast(richMarkdown);
    const nonCheckpointWithOptions = { questions: [{ question: 'Elige?', options: [{ label: 'Opción A' }, { label: 'Opción B' }] }] };
    const resAllowedFirst = await wrapped.execute('id-hasopts-allow', nonCheckpointWithOptions, null, null, makeCtx(missingMarkdown, []));
    assert.equal(resAllowedFirst.isError, undefined, 'non-checkpoint 2-option ask must allow');
    const resCp = await wrapped.execute('id-hasopts-cp', checkpointParams, null, null, makeCtx(missingMarkdown, []));
    assert.equal(resCp.isError, undefined, 'checkpoint 2-option ask must allow (retired)');
    assert.equal(calls.count, 2);
    const handler = mock._getToolCallHandler();
    const ret = await handler({ toolName: 'ask_user_question', params: nonCheckpointWithOptions }, makeCtx(missingMarkdown, []));
    assert.equal(ret, undefined, 'tool_call must allow');
    h._test.clearCurrent(); h._test.clearLast();
  });

  it('proceder/procede tokens are checkpoint labels (case-insensitive)', async () => {
    const mock = createMockPi(); gateFn(mock);
    const h = mock._biggzSynthesisGate;
    const proc = { questions: [{ question: 'Next?', options: [{ label: 'Proceder' }, { label: 'Ajustar' }, { label: 'Detener' }] }] };
    const procede = { questions: [{ question: 'Q?', options: [{ label: 'procede' }, { label: 'otro' }] }] };
    const procUpper = { questions: [{ question: 'Q?', options: [{ label: 'PROCEDER' }] }] };
    const jsonStr = JSON.stringify({ questions: [{ question: 'Q?', options: [{ label: 'Proceder' }] }] });
    assert.equal(h.isCheckpointAsk(proc), true, 'Proceder should be checkpoint');
    assert.equal(h.isCheckpointAsk(procede), true, 'procede should be checkpoint');
    assert.equal(h.isCheckpointAsk(procUpper), true, 'PROCEDER upper should be checkpoint');
    assert.equal(h.isCheckpointAsk(JSON.parse(jsonStr)), true);
    // Enforcement retired: labels still classify, but the ask always executes.
    delete process.env.BIGGZ_ADVISE;
    const { wrapped, calls } = registerDummy(mock, 'question');
    h._test.clearCurrent(); h._test.clearLast(); h._test.setLast(richMarkdown);
    const res = await wrapped.execute('id-proc', proc, null, null, makeCtx(missingMarkdown, []));
    assert.equal(res.isError, undefined, 'Proceder checkpoint executes (retired)');
    assert.equal(calls.count, 1);
    h._test.clearCurrent(); h._test.clearLast();
  });

  it('parity vs Go fixtures — detection verdicts match, enforcement retired both sides', async () => {
    delete process.env.BIGGZ_ADVISE;
    const mock = createMockPi(); gateFn(mock);
    const h = mock._biggzSynthesisGate;
    // Same fixtures as Go TestIsCheckpointAskEnvelopeLabelsOnly — JS verdicts must match Go.
    const checkpoint = { questions: [{ question: 'Proceed with plan?', header: 'Decisión', options: [{ label: 'Proceed' }, { label: 'Adjust' }] }] };
    const freeText = { question: 'How are you doing today?' };
    const preflight = { questions: [{ question: 'Pick pace', options: [{ label: 'Relaxed' }, { label: 'Fast' }] }] };
    const bodyToken = { questions: [{ header: 'Ritmo', question: '¿Qué ritmo usamos para continuar con el change?', options: [{ label: 'Interactivo' }, { label: 'Automático' }] }] };
    assert.equal(h.isCheckpointAsk(checkpoint), true, 'checkpoint fixture must be checkpoint');
    assert.equal(h.isCheckpointAsk(freeText), false, 'free-text fixture must not be checkpoint');
    assert.equal(h.isCheckpointAsk(preflight), false, 'preflight option-ask must not be checkpoint');
    assert.equal(h.isCheckpointAsk(bodyToken), false, 'body-token fixture must not be checkpoint (labels-only parity)');
    // All verdicts allow (both sides retired).
    const { wrapped, calls } = registerDummy(mock);
    h._test.clearCurrent(); h._test.clearLast();
    for (const [id, params] of [['p-cp', checkpoint], ['p-free', freeText], ['p-pre', preflight], ['p-body', bodyToken]]) {
      const res = await wrapped.execute(id, params, null, null, makeCtx(missingMarkdown, []));
      assert.equal(res.isError, undefined, `${id} must allow (retired)`);
    }
    assert.equal(calls.count, 4);
    const handler = mock._getToolCallHandler();
    const retCp = await handler({ toolName: 'ask_user_question', params: checkpoint }, makeCtx(missingMarkdown, []));
    assert.equal(retCp, undefined, 'tool_call checkpoint must allow (retired)');
    const retPre = await handler({ toolName: 'ask_user_question', params: preflight }, makeCtx(missingMarkdown, []));
    assert.equal(retPre, undefined, 'tool_call preflight must allow');
    // formatFallback still renders the envelope verbatim for the plain-chat fallback path.
    const fb = h.formatFallback(checkpoint);
    for (const want of ['Proceed with plan?', 'Proceed', 'Adjust']) {
      assert.ok(fb && fb.includes(want), `fallback must contain ${want} verbatim`);
    }
    h._test.clearCurrent(); h._test.clearLast();
  });

});
