// S1a contract of pi-subagent-runtime: strict JSONL framing (LF-only, CR strip,
// 1 MiB cap, malformed skipped+logged), spawn authorization (`--tools` from
// frontmatter, read-only default, nested refusal, bounded unknown-agent error)
// and watchdog/kill (idle/total stall, abort → group SIGTERM → SIGKILL, Windows
// `taskkill /PID /T [/F]`, child+grandchild dead after cancel).
// S1b contract: registration gate (dual-registration vs j0k3r), `subagent`
// task-mode + companions over a bounded in-memory ring (no disk, no BigMem),
// one-line completion card via pi-tui width primitives, the
// `biggz-subagent-completion` renderer, and resolver fidelity (real pi-tui vs oracle).
// S2 contract: dialog relay round-trip + steering, background ids/completion
// delivery without polling, the `biggz-subagents` widget (max 2 rows + `… +N`,
// hidden when idle/child/pretty-off), cap 2 + FIFO queue with numeric
// `BIGGZ_BACKGROUND_SUBAGENTS` override, and the exact `subagent_wait` headline.
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

import { installPiTuiResolver, resolvePiTuiEntry, PI_TUI_SPECIFIER } from './test/pi-tui-resolver.mjs';

// The resolver hook must be installed before the runtime links its pi-tui import.
const resolution = installPiTuiResolver();
const tui = await import(PI_TUI_SPECIFIER);
const rt = await import('./biggz-subagent-runtime.js');
const runtime = rt.default;
const {
  JSONL_MAX_LINE_BYTES,
  createJsonlReader,
  discoverAgents,
  resolveAgent,
  nestedSpawnRefusal,
  buildChildArgs,
  buildChildEnv,
  createTask,
  killTree,
  subagentRegistrationGate,
  createTaskRegistry,
  createSubagentToolset,
  completionLine,
  renderCompletion,
  createCompletionRenderer,
  COMPLETION_MESSAGE_TYPE,
  resolveBackgroundCap,
  widgetRowsFor,
  createWidgetPublisher,
  renderWaitHeadline,
  presentUiRequest,
} = rt;

const SRC = fs.readFileSync(new URL('./biggz-subagent-runtime.js', import.meta.url), 'utf8');
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const isAlive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
};
const waitFor = async (fn, ms = 8000) => {
  const end = Date.now() + ms;
  while (Date.now() < end && !fn()) await sleep(25);
  return fn();
};
const LOG = { warn() {}, error() {} };
const readerOf = (onLine, warn = () => {}) => createJsonlReader({ onLine, logger: { warn } });

// Fake pi RPC child (inline; no fixture file): one JSONL ready line with pids +
// PI_SUBAGENT_CHILD, optional tick/settle/dialog, echoes stdin commands, answers
// `get_last_assistant_text`, never exits on `abort` (kills must escalate), exits
// on stdin end.
const FAKE = `
const { spawn } = require('node:child_process');
const out = (o) => process.stdout.write(JSON.stringify(o) + '\\n');
let grandchild = null;
if (process.env.FAKE_GRANDCHILD === '1') grandchild = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });
out({ type: 'fake_ready', child: process.pid, grandchild: grandchild && grandchild.pid, subagentChild: process.env.PI_SUBAGENT_CHILD ?? null });
if (process.env.FAKE_TICK_MS) setInterval(() => out({ type: 'tick' }), Number(process.env.FAKE_TICK_MS));
if (process.env.FAKE_SETTLED === '1') setTimeout(() => out({ type: 'agent_settled' }), 60);
if (process.env.FAKE_UI_REQUEST === '1') out({ type: 'extension_ui_request', id: 'q1', method: 'select', title: 'Pick one', options: ['Allow', 'Block'] });
process.stdin.on('data', (chunk) => {
  for (const raw of String(chunk).split('\\n')) {
    if (!raw) continue;
    let cmd = null;
    try { cmd = JSON.parse(raw); } catch {}
    if (!cmd) continue;
    out({ type: 'fake_command', command: cmd });
    if (cmd.type === 'get_last_assistant_text') out({ type: 'response', command: 'get_last_assistant_text', success: true, data: { text: 'final answer' } });
    if (cmd.type === 'extension_ui_response' && cmd.id === 'q1') {
      out({ type: 'dialog_answer', value: cmd.value ?? null, cancelled: cmd.cancelled === true });
      out({ type: 'agent_settled' });
    }
  }
});
process.stdin.on('end', () => process.exit(0));
setInterval(() => {}, 1000);
`;
const LAUNCH = { command: process.execPath, prefix: ['-e', FAKE], shell: false };
const AGENT = { id: 'probe', tools: ['read'], body: 'agent body', model: null };
const run = (env, extra = {}) =>
  createTask({
    agent: AGENT,
    task: 'do the thing',
    launch: LAUNCH,
    args: [], // fake child is `node -e <script>`; pi RPC argv is covered by buildChildArgs test
    env: { ...process.env, ...env },
    idleMs: 8000,
    totalMs: 20000,
    killGraceMs: 200,
    logger: LOG,
    ...extra,
  });

describe('strict JSONL framing', () => {
  it('parses partial lines, several records per chunk, and CRLF (CR stripped)', () => {
    const lines = [];
    const reader = readerOf((line) => lines.push(line));
    reader.push('{"a"');
    reader.push(':1}\r\n{"b":2}\n{"c"');
    assert.deepEqual(lines, [{ a: 1 }, { b: 2 }]);
    reader.push(':3}\n');
    assert.deepEqual(lines, [{ a: 1 }, { b: 2 }, { c: 3 }]);
  });

  it('keeps U+2028/U+2029 inside JSON strings; readline is never used', () => {
    const lines = [];
    readerOf((line) => lines.push(line)).push(`${JSON.stringify({ text: 'a\u2028b\u2029c' })}\n`);
    assert.equal(lines.length, 1, 'U+2028/U+2029 must not split a record');
    assert.equal(lines[0].text, 'a\u2028b\u2029c');
    assert.ok(!/node:readline|\.createInterface\(/.test(SRC), 'readline splits U+2028/U+2029 and is not RPC-compliant');
  });

  it('skips malformed lines with a warning and keeps the stream alive', () => {
    const lines = [];
    const warns = [];
    readerOf((line) => lines.push(line), (msg) => warns.push(msg)).push('{"ok":1}\nnot json\n{"ok":2}\n');
    assert.deepEqual(lines, [{ ok: 1 }, { ok: 2 }]);
    assert.equal(warns.length, 1);
    assert.match(warns[0], /malformed JSONL line skipped/);
  });

  it('drops oversized lines at the 1 MiB cap (with and without LF) and resumes', () => {
    const lines = [];
    const warns = [];
    const reader = readerOf((line) => lines.push(line), (msg) => warns.push(msg));
    reader.push(`${'x'.repeat(JSONL_MAX_LINE_BYTES + 16)}\n{"after":1}\n`);
    assert.deepEqual(lines, [{ after: 1 }], 'oversized record skipped, next record parsed');
    reader.push('y'.repeat(JSONL_MAX_LINE_BYTES + 16));
    assert.equal(reader.bufferedBytes(), 0, 'buffer must stay bounded below the cap');
    reader.push('\n{"afterNoLf":2}\n');
    assert.deepEqual(lines, [{ after: 1 }, { afterNoLf: 2 }], 'stream resumes after a dropped no-LF overflow');
    assert.equal(warns.filter((w) => /oversized JSONL line skipped/.test(w)).length, 2);
  });
});

describe('agent discovery + spawn authorization', () => {
  it('parses tools/model/body from frontmatter; missing frontmatter ⇒ read-only', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'biggz-agents-'));
    fs.writeFileSync(path.join(dir, 'sdd-apply.md'), '---\nname: sdd-apply\ntools:\n  - read\n  - edit\n  - bash\nmodel: anthropic/claude-sonnet-4-5\n---\n\nBody line\n');
    fs.writeFileSync(path.join(dir, 'inline.md'), '---\nname: inline\ntools: read, write\n---\nInline\n');
    fs.writeFileSync(path.join(dir, 'plain.md'), 'No frontmatter\n');
    const agents = discoverAgents({ dir });
    assert.deepEqual(agents.map((a) => a.id), ['inline', 'plain', 'sdd-apply']);
    assert.deepEqual(agents[2].tools, ['read', 'edit', 'bash']);
    assert.equal(agents[2].model, 'anthropic/claude-sonnet-4-5');
    assert.equal(agents[2].body, 'Body line');
    assert.deepEqual(agents[0].tools, ['read', 'write']);
    assert.deepEqual(agents[1].tools, ['read'], 'missing frontmatter ⇒ read-only default');
  });

  it('bounds unknown-agent errors, refuses nested spawn, builds argv+env without mutation', () => {
    const missing = resolveAgent('nope', [{ id: 'sdd-apply' }, { id: 'sdd-verify' }, { id: 'sdd-spec' }]);
    assert.equal(missing.ok, false);
    assert.match(missing.error, /unknown agent "nope"; available: sdd-apply, sdd-verify, sdd-spec/);
    const many = resolveAgent('nope', Array.from({ length: 20 }, (_, i) => ({ id: `a${i}` })));
    assert.match(many.error, /… \+12 more/);
    assert.ok(many.error.length < 200, `error must stay bounded, got ${many.error.length}`);
    assert.equal(nestedSpawnRefusal({ PI_SUBAGENT_CHILD: '1' })?.ok, false);
    assert.equal(nestedSpawnRefusal({}), null);
    assert.equal(resolveAgent('probe', [AGENT], { PI_SUBAGENT_CHILD: '1' }).ok, false, 'nested spawn refused');
    assert.deepEqual(buildChildArgs({ tools: [], model: null }), ['--mode', 'rpc', '--no-session', '--tools', 'read']);
    assert.deepEqual(buildChildArgs({ tools: ['read', 'edit'], model: 'openai/gpt-5' }), ['--mode', 'rpc', '--no-session', '--tools', 'read,edit', '--model', 'openai/gpt-5']);
    const base = { PATH: '/usr/bin' };
    assert.equal(buildChildEnv(base).PI_SUBAGENT_CHILD, '1');
    assert.equal('PI_SUBAGENT_CHILD' in base, false, 'base env object must not be mutated');
    assert.equal(process.env.PI_SUBAGENT_CHILD, undefined);
  });
});

describe('RPC child runner (fake child over JSONL)', () => {
  it('completes on agent_settled; child env PI_SUBAGENT_CHILD=1 is set on spawn only', async (t) => {
    const task = run({ FAKE_SETTLED: '1' });
    t.after(() => task.cancel());
    const result = await task.promise;
    assert.equal(result.state, 'completed');
    const ready = result.events.find((e) => e.type === 'fake_ready');
    assert.equal(String(ready?.subagentChild), '1', 'child env must carry PI_SUBAGENT_CHILD=1');
    const prompt = result.events.find((e) => e.type === 'fake_command' && e.command.type === 'prompt');
    assert.match(prompt?.command.message ?? '', /agent body[\s\S]*## delegated task\ndo the thing/);
    assert.equal(process.env.PI_SUBAGENT_CHILD, undefined, 'parent env must never gain PI_SUBAGENT_CHILD');
    await waitFor(() => !isAlive(task.pid));
  });

  it('idle watchdog stalls + kills a silent child; the runtime survives it', async (t) => {
    const task = run({}, { idleMs: 250 });
    t.after(() => task.cancel());
    const result = await task.promise;
    assert.equal(result.state, 'stalled');
    assert.equal(result.reason, 'idle');
    assert.ok(await waitFor(() => !isAlive(result.pid)), 'stalled child must be terminated');
    const next = run({ FAKE_SETTLED: '1' });
    t.after(() => next.cancel());
    assert.equal((await next.promise).state, 'completed', 'session must survive a prior stall');
  });

  it('total watchdog stalls a chatty child', async (t) => {
    const task = run({ FAKE_TICK_MS: '40' }, { totalMs: 300 });
    t.after(() => task.cancel());
    const result = await task.promise;
    assert.equal(result.state, 'stalled');
    assert.equal(result.reason, 'total');
    assert.ok(result.events.some((e) => e.type === 'tick'), 'chatty child was producing output');
  });

  it('cancel kills the child + grandchild tree and marks the task cancelled', async (t) => {
    const task = run({ FAKE_GRANDCHILD: '1' }, { killGraceMs: 300 });
    t.after(() => task.cancel());
    assert.ok(await waitFor(() => task.snapshot().events.some((e) => e.type === 'fake_ready' && e.grandchild)), 'fake child must report child + grandchild pids');
    const ready = task.snapshot().events.find((e) => e.type === 'fake_ready');
    await task.cancel();
    assert.equal((await task.promise).state, 'cancelled');
    assert.ok(await waitFor(() => !isAlive(ready.child) && !isAlive(ready.grandchild)), `tree must be dead (child ${ready.child}, grandchild ${ready.grandchild})`);
  });

  it('escalates abort → SIGTERM group → SIGKILL (posix) and taskkill /T → /T /F (win32)', async () => {
    let posixAlive = true;
    const signals = [];
    const killImpl = (_target, signal) => {
      if (signal === 0) {
        if (!posixAlive) throw Object.assign(new Error('gone'), { code: 'ESRCH' });
        return;
      }
      signals.push(signal);
      if (signal === 'SIGKILL') posixAlive = false;
    };
    const aborts = [];
    const posix = await killTree(4321, { platform: 'linux', graceMs: 0, pollMs: 1, killImpl, abort: () => aborts.push('abort'), logger: LOG });
    assert.deepEqual(posix.steps, ['abort', 'SIGTERM', 'SIGKILL'], 'escalation must be abort → group TERM → group KILL');
    assert.deepEqual(signals, ['SIGTERM', 'SIGKILL']);
    assert.equal(aborts.length, 1, 'cooperative abort RPC must be written first');

    let winAlive = true;
    const calls = [];
    const win = await killTree(9876, {
      platform: 'win32',
      graceMs: 0,
      pollMs: 1,
      abort: () => {},
      logger: LOG,
      execImpl: (cmd, argv) => {
        calls.push([cmd, ...argv]);
        if (argv.includes('/F')) winAlive = false;
      },
      killImpl: () => {
        if (!winAlive) throw Object.assign(new Error('gone'), { code: 'ESRCH' });
      },
    });
    assert.deepEqual(win.steps, ['abort', 'taskkill /T', 'taskkill /T /F']);
    assert.deepEqual(calls, [
      ['taskkill', '/PID', '9876', '/T'],
      ['taskkill', '/PID', '9876', '/T', '/F'],
    ]);
  });
});

describe('extension factory (inert)', () => {
  it('attaches internals for tests and no-ops inside a subagent child', () => {
    // Temp PI_CODING_AGENT_DIR keeps the gate off the developer's real settings.
    withAgentEnv(fakeAgentDir([]), () => {
      const pi = {};
      runtime(pi);
      assert.equal(typeof pi._biggzSubagentRuntime.createTask, 'function');
      assert.equal(pi.registered, undefined);
    });
    const saved = process.env.PI_SUBAGENT_CHILD;
    process.env.PI_SUBAGENT_CHILD = '1';
    try {
      const childPi = fakePi();
      runtime(childPi);
      assert.equal(childPi._biggzSubagentRuntime, undefined, 'child chrome must bypass the runtime factory');
      assert.equal(childPi.registered.length, 0, 'child chrome must not register tools');
    } finally {
      if (saved === undefined) delete process.env.PI_SUBAGENT_CHILD;
      else process.env.PI_SUBAGENT_CHILD = saved;
    }
  });
});

// ── S1b: registration gate, tool surface, completion card, resolver ──

// Fake agent dir (`PI_CODING_AGENT_DIR`): settings.json packages + optional agents.
const fakeAgentDir = (packages, agents = {}) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'biggz-pi-agent-'));
  fs.writeFileSync(path.join(dir, 'settings.json'), JSON.stringify({ packages }));
  fs.mkdirSync(path.join(dir, 'agents'), { recursive: true });
  for (const [name, body] of Object.entries(agents)) fs.writeFileSync(path.join(dir, 'agents', `${name}.md`), body);
  return dir;
};

// Fake ExtensionAPI: records registrations and answers the gate's chain.
const fakePi = (options = {}) => ({
  registered: [],
  renderers: [],
  getAllTools: options.getAllTools ?? (() => options.tools ?? []),
  ...(options.getToolDefinition ? { getToolDefinition: options.getToolDefinition } : {}),
  registerTool(definition) {
    this.registered.push(definition);
  },
  registerMessageRenderer(type, renderer) {
    this.renderers.push([type, renderer]);
  },
});

const withAgentEnv = (dir, fn) => {
  const saved = process.env.PI_CODING_AGENT_DIR;
  process.env.PI_CODING_AGENT_DIR = dir;
  try {
    return fn();
  } finally {
    if (saved === undefined) delete process.env.PI_CODING_AGENT_DIR;
    else process.env.PI_CODING_AGENT_DIR = saved;
  }
};

describe('registration gate (dual-registration vs j0k3r)', () => {
  it('refuses when getAllTools() lists subagent_run / subagent_list_* (zero registrations)', () => {
    const dir = fakeAgentDir(['npm:@heyhuynhgiabuu/pi-pretty']);
    const pi = fakePi({ tools: [{ name: 'read' }, { name: 'subagent_run' }, { name: 'subagent_list_running' }] });
    const warns = [];
    const gate = subagentRegistrationGate(pi, { env: { PI_CODING_AGENT_DIR: dir }, logger: { warn: (m) => warns.push(m), debug() {} } });
    assert.equal(gate.register, false);
    assert.match(gate.reason, /legacy j0k3r tool\(s\) present: subagent_run, subagent_list_running/);
    assert.match(warns[0], /registration skipped/);
    withAgentEnv(dir, () => runtime(pi));
    assert.equal(pi.registered.length, 0, 'dual registration is impossible by construction');
  });

  it('refuses when settings packages carry pi-subagents-j0k3r even with no tool clash', () => {
    const dir = fakeAgentDir(['npm:pi-subagents-j0k3r@1.6.1', 'npm:@heyhuynhgiabuu/pi-pretty']);
    const pi = fakePi({ tools: [{ name: 'subagent' }] });
    const gate = subagentRegistrationGate(pi, { env: { PI_CODING_AGENT_DIR: dir }, logger: LOG });
    assert.equal(gate.register, false);
    assert.match(gate.reason, /pi-subagents-j0k3r present in settings packages/);
    withAgentEnv(dir, () => runtime(pi));
    assert.equal(pi.registered.length, 0);
  });

  it('fails closed when the chain cannot prove absence (getAllTools throws + settings corrupt)', () => {
    const dir = fakeAgentDir([]);
    fs.writeFileSync(path.join(dir, 'settings.json'), '{ not json');
    const warns = [];
    const pi = fakePi({ getAllTools: () => { throw new Error('runtime not initialized'); } });
    const gate = subagentRegistrationGate(pi, { env: { PI_CODING_AGENT_DIR: dir }, logger: { warn: (m) => warns.push(m), debug() {} } });
    assert.equal(gate.register, false);
    assert.match(gate.reason, /cannot prove pi-subagents-j0k3r absent/);
    assert.match(warns[0], /registration skipped/);
    withAgentEnv(dir, () => runtime(pi));
    assert.equal(pi.registered.length, 0, 'unprovable state must not register');
  });

  it('registers the six tools + completion renderer when j0k3r is absent (load-time getAllTools throw tolerated)', () => {
    const dir = fakeAgentDir(['npm:@heyhuynhgiabuu/pi-pretty']);
    // 0.85.1 throws this exact message while extensions load — falling through, not failing closed.
    const pi = fakePi({ getAllTools: () => { throw new Error('Extension runtime not initialized. Action methods cannot be called during extension loading.'); } });
    const gate = subagentRegistrationGate(pi, { env: { PI_CODING_AGENT_DIR: dir }, logger: LOG });
    assert.equal(gate.register, true);
    withAgentEnv(dir, () => runtime(pi));
    assert.deepEqual(pi.registered.map((d) => d.name), ['subagent', 'subagent_wait', 'subagent_status', 'subagent_result', 'subagent_cancel', 'subagent_agents']);
    assert.equal(pi.renderers.length, 1);
    assert.equal(pi.renderers[0][0], COMPLETION_MESSAGE_TYPE);
    assert.equal(typeof pi.renderers[0][1], 'function');
    assert.ok(!/\.getTool\(/.test(SRC), 'pi.getTool is absent in 0.85.1 and must never be called');
  });
});

describe('bounded in-memory task ring', () => {
  it('keeps at most `limit` settled records, never evicts live work, and writes nothing', () => {
    const registry = createTaskRegistry({ limit: 3 });
    for (let i = 0; i < 6; i++) registry.add({ id: `t${i}`, state: 'completed' });
    assert.equal(registry.size, 3);
    assert.equal(registry.get('t0'), null, 'oldest settled record evicted');
    assert.equal(registry.get('t5')?.id, 't5', 'newest record kept');
    const live = createTaskRegistry({ limit: 2 });
    live.add({ id: 'run1', state: 'running' });
    live.add({ id: 'run2', state: 'running' });
    live.add({ id: 'done1', state: 'completed' });
    assert.deepEqual(live.all().map((t) => t.id), ['run1', 'run2', 'done1'], 'in-flight work is never evicted');
    assert.deepEqual(live.active().map((t) => t.id), ['run1', 'run2']);
    live.add({ id: 'done2', state: 'completed' });
    assert.deepEqual(live.all().map((t) => t.id), ['run1', 'run2', 'done2'], 'oldest settled record evicted, live kept');
    assert.ok(!/writeFileSync|appendFileSync|biggz_mem/i.test(SRC), 'ring is in-memory only (no disk, no BigMem)');
  });
});

describe('subagent tool surface (task mode, fake child)', () => {
  it('runs one fake RPC child per task and bounds context/mode/unknown-agent errors', async () => {
    const dir = fakeAgentDir([], { probe: '---\nname: probe\ntools: read\n---\nagent body\n' });
    const { tools, registry } = createSubagentToolset({
      env: { ...process.env, PI_CODING_AGENT_DIR: dir, FAKE_SETTLED: '1' },
      launch: LAUNCH,
      args: [],
      idleMs: 8000,
      totalMs: 20000,
      killGraceMs: 200,
      logger: LOG,
    });
    const byName = (name) => tools.find((tool) => tool.name === name);
    assert.deepEqual(tools.map((t) => t.name), ['subagent', 'subagent_wait', 'subagent_status', 'subagent_result', 'subagent_cancel', 'subagent_agents']);

    const run = await byName('subagent').execute('call-1', { agent: 'probe', task: 'do the thing' });
    assert.equal(run.isError, undefined);
    assert.match(run.content[0].text, /^subagent sub-[\w-]+ · completed · /);
    assert.equal(run.details.state, 'completed');
    assert.ok(registry.get(run.details.taskId), 'result is reachable from the in-memory ring only');

    const fork = await byName('subagent').execute('call-2', { agent: 'probe', task: 'x', context: 'fork' });
    assert.equal(fork.isError, true);
    assert.match(fork.content[0].text, /unsupported context "fork"; available: fresh/);
    const unknownMode = await byName('subagent').execute('call-3', { agent: 'probe', task: 'x', mode: 'chat' });
    assert.equal(unknownMode.isError, true);
    assert.match(unknownMode.content[0].text, /unsupported mode "chat"; available: task, background/);
    const missing = await byName('subagent').execute('call-4', { agent: 'nope', task: 'x' });
    assert.equal(missing.isError, true);
    assert.match(missing.content[0].text, /unknown agent "nope"; available: probe/);

    const agents = await byName('subagent_agents').execute('call-5', {});
    assert.match(agents.content[0].text, /^probe · .* · tools: read/);
    const unknown = await byName('subagent_status').execute('call-6', { task_id: 'gone' });
    assert.equal(unknown.isError, true);
    assert.match(unknown.content[0].text, /unknown or expired task "gone"/);
    const status = await byName('subagent_status').execute('call-7', { task_id: run.details.taskId });
    assert.match(status.content[0].text, /completed/);
    const tail = await byName('subagent_result').execute('call-8', { task_id: run.details.taskId, max_lines: 3 });
    assert.equal(tail.content[0].text.split('\n').length <= 3, true, 'subagent_result respects max_lines');
    const cancelled = await byName('subagent_cancel').execute('call-9', { task_id: run.details.taskId });
    assert.match(cancelled.content[0].text, /^cancelled sub-/);
  });
});

describe('completion card (pi-tui width primitives)', () => {
  // Exact crash shape: code-point math says 188 (= width) while true cells = 190.
  const crashRun = () => {
    for (let n = 0; n < 400; n++) {
      const run = { agent: 'sdd-apply', state: 'completed', elapsedMs: 23000, summary: `✅ ${'x'.repeat(n)}` };
      const line = completionLine(run);
      if ([...line].length === 188) return { run, line };
    }
    throw new Error('no 188-code-point candidate found');
  };

  it('renders one bounded line across widths 20-200; the w=190>188 two-✅ shape stays ≤ width', () => {
    const { run, line } = crashRun();
    assert.equal([...line].length, 188, 'code-point math would call the line 188');
    assert.equal(tui.visibleWidth(line), 190, 'two 2-cell ✅ make the true width 190 (w=190 > 188)');
    assert.equal((line.match(/✅/g) ?? []).length, 2, 'asset carries exactly two ✅');
    for (const width of [20, 40, 63, 80, 120, 160, 188, 190, 200]) {
      assert.ok(tui.visibleWidth(renderCompletion(run, width)) <= width, `line must fit width ${width}`);
    }
    const rendered = renderCompletion(run, 188);
    assert.match(rendered.replace(/\u001b\[[0-9;]*m/g, ''), /…$/, 'truncation keeps the pi-tui ellipsis');
    assert.ok(tui.visibleWidth(rendered) <= 188);
  });

  it('builds the biggz-subagent-completion renderer from the exported pure renderCompletion', () => {
    const render = createCompletionRenderer({ width: 188 });
    const component = render({
      customType: COMPLETION_MESSAGE_TYPE,
      content: 'done',
      details: { agent: 'sdd-apply', state: 'completed', elapsedMs: 23000, summary: '✅ ✅' },
    });
    const lines = component.render();
    assert.equal(lines.length, 1, 'exactly one bounded line');
    assert.ok(tui.visibleWidth(lines[0]) <= 188);
    assert.match(lines[0], /^✅ sdd-apply · completed · 23\.0s/);
    assert.equal(typeof component.invalidate, 'function');
    assert.ok(tui.visibleWidth(renderCompletion({ agent: 'x'.repeat(500), state: 'failed', elapsedMs: 1000 }, 20)) <= 20);
  });
});

describe('pi-tui resolver (install or test-only oracle)', () => {
  it('resolves the specifier and matches the oracle against the real pi-tui when installed', async (t) => {
    assert.match(resolution.mode, /^install:|^oracle$/);
    assert.equal(typeof tui.visibleWidth, 'function');
    assert.equal(typeof tui.truncateToWidth, 'function');
    assert.ok(typeof (await import('./test/pi-tui-oracle.mjs')).visibleWidth === 'function', 'oracle fallback exists');
    const real = resolvePiTuiEntry(process.env);
    if (!real) {
      t.skip('real @earendil-works/pi-tui not installed — oracle fallback in use');
      return;
    }
    const realTui = await import(pathToFileURL(real.file).href);
    const oracle = await import('./test/pi-tui-oracle.mjs');
    const fixtures = ['', 'plain ascii', '✅', '✅✅', '✅ ab ✅', '你好世界', '한글', '👨‍👩‍👧 family', '\u001b[31mred\u001b[0m ok', 'e\u0301 combining', 'x'.repeat(190)];
    for (const fixture of fixtures) {
      assert.equal(oracle.visibleWidth(fixture), realTui.visibleWidth(fixture), `visibleWidth mismatch: ${JSON.stringify(fixture)}`);
      for (const width of [1, 2, 5, 10, 20]) {
        const mine = oracle.truncateToWidth(fixture, width, '…');
        const theirs = realTui.truncateToWidth(fixture, width, '…');
        assert.equal(oracle.visibleWidth(mine), realTui.visibleWidth(theirs), `truncate width mismatch: ${JSON.stringify(fixture)} @ ${width}`);
      }
    }
  });
});

// ── S2: dialog relay, steering, background cap/queue, widget, wait headline ──

// Fake ExtensionContext: records setWidget/notify calls (options object only).
const fakeCtx = () => ({
  ui: {
    calls: [],
    notifies: [],
    setWidget(key, content, options) {
      this.calls.push([key, content, options]);
    },
    notify(message, type) {
      this.notifies.push([message, type]);
    },
  },
});

const probeDir = () => fakeAgentDir([], { probe: '---\nname: probe\ntools: read\n---\nagent body\n' });
const toolsetFor = (options = {}) => {
  const env = { ...process.env, PI_CODING_AGENT_DIR: probeDir(), FAKE_SETTLED: '1' };
  delete env.BIGGZ_BACKGROUND_SUBAGENTS; // the cap override is asserted directly; ambient env must not leak
  const tools = createSubagentToolset({
    env,
    launch: LAUNCH,
    args: [],
    idleMs: 8000,
    totalMs: 20000,
    killGraceMs: 200,
    logger: LOG,
    ...options,
  });
  return { ...tools, byName: (name) => tools.tools.find((tool) => tool.name === name) };
};

describe('S2 dialog relay + steering', () => {
  it('relays a child extension_ui_request, returns the answer, and the run continues', async (t) => {
    const presented = [];
    const task = run(
      { FAKE_UI_REQUEST: '1' },
      {
        onUiRequest: async (request) => {
          presented.push(request);
          return { value: 'Allow' };
        },
      },
    );
    t.after(() => task.cancel());
    const result = await task.promise;
    assert.equal(result.state, 'completed', 'the run must continue after the dialog is answered');
    assert.equal(presented.length, 1);
    assert.equal(presented[0].method, 'select');
    assert.deepEqual(presented[0].options, ['Allow', 'Block']);
    const answer = result.events.find((e) => e.type === 'fake_command' && e.command.type === 'extension_ui_response');
    assert.deepEqual(answer?.command, { type: 'extension_ui_response', id: 'q1', value: 'Allow' }, 'the answer must return to the child');
    assert.equal(result.events.find((e) => e.type === 'dialog_answer')?.value, 'Allow');
    assert.equal(result.finalText, 'final answer', 'get_last_assistant_text feeds the result');
    assert.equal(result.pendingUi, 0);
  });

  it('presents select/confirm/input through ctx.ui and normalizes the answer', async () => {
    const seen = [];
    const select = await presentUiRequest(
      {
        ui: {
          select: async (title, options) => {
            seen.push([title, options]);
            return 'Allow';
          },
        },
      },
      { method: 'select', title: 'Pick one', options: ['Allow', 'Block'] },
    );
    assert.deepEqual(seen, [['Pick one', ['Allow', 'Block']]]);
    assert.deepEqual(select, { value: 'Allow' });
    assert.deepEqual(await presentUiRequest({ ui: { confirm: async () => false } }, { method: 'confirm', title: 'ok?' }), { confirmed: false });
    assert.deepEqual(await presentUiRequest({ ui: { input: async () => undefined } }, { method: 'input', title: 't' }), { cancelled: true });
    assert.deepEqual(await presentUiRequest(undefined, { method: 'select', title: 't', options: [] }), { cancelled: true }, 'no UI ⇒ cancelled');
  });

  it('steers a running child mid-run', async (t) => {
    const task = run({ FAKE_TICK_MS: '30' });
    t.after(() => task.cancel());
    assert.ok(await waitFor(() => task.state === 'running'));
    assert.equal(task.steer('change of plan'), true);
    assert.ok(
      await waitFor(() => task.snapshot().events.some((e) => e.type === 'fake_command' && e.command.type === 'steer' && e.command.message === 'change of plan')),
      'the steering message must reach the child',
    );
  });
});

describe('S2 background mode, widget, cap and wait headline', () => {
  it('returns background ids immediately and delivers each completion without polling', async () => {
    const deliveries = [];
    const ctx = fakeCtx();
    const widget = createWidgetPublisher({ env: {}, width: 200 });
    // Mirrors the factory delivery: transcript card + notify + widget refresh.
    const deliverCompletion = (run) => {
      deliveries.push(run);
      widget.notify(completionLine(run), run.state === 'completed' ? 'info' : 'warning');
    };
    const { byName } = toolsetFor({ widget, deliverCompletion });
    const first = await byName('subagent').execute('c1', { agent: 'probe', task: 'a', mode: 'background' }, undefined, undefined, ctx);
    const second = await byName('subagent').execute('c2', { agent: 'probe', task: 'b', mode: 'background' }, undefined, undefined, ctx);
    assert.equal(first.isError, undefined);
    assert.match(first.content[0].text, /^background sub-\S+ · running · cap 2$/);
    assert.equal(deliveries.length, 0, 'ids must return before any completion (no polling, no awaiting)');
    assert.deepEqual(second.details.state, 'running');
    assert.deepEqual(ctx.ui.calls.at(-1)[1], ['◐ probe · running · 0s', '◐ probe · running · 0s'], 'widget rows for the active runs');
    assert.equal(ctx.ui.calls.at(-1)[0], 'biggz-subagents');
    assert.deepEqual(ctx.ui.calls.at(-1)[2], { placement: 'belowEditor' }, 'placement must ride the options object');
    await waitFor(() => deliveries.length === 2);
    assert.deepEqual(deliveries.map((run) => run.state), ['completed', 'completed']);
    assert.equal(deliveries[0].summary, 'final answer');
    assert.equal(ctx.ui.calls.at(-1)[1], undefined, 'widget is hidden when idle');
    assert.ok(ctx.ui.notifies.length >= 2, 'each completion is also a notification');
  });

  it('caps at 1, queues FIFO, auto-starts on a free slot, and cancels queued runs pre-spawn', async () => {
    const deliveries = [];
    const { tools, registry, byName } = toolsetFor({ cap: 1, deliverCompletion: (run) => deliveries.push(run) });
    const first = await byName('subagent').execute('c1', { agent: 'probe', task: 'a', mode: 'background' });
    const second = await byName('subagent').execute('c2', { agent: 'probe', task: 'b', mode: 'background' });
    const third = await byName('subagent').execute('c3', { agent: 'probe', task: 'c', mode: 'background' });
    assert.equal(first.details.state, 'running');
    assert.equal(second.details.state, 'queued');
    assert.equal(third.details.state, 'queued');
    assert.equal(registry.get(second.details.taskId).pid, null, 'queued runs must not spawn a child');
    const cancelled = await tools.find((tool) => tool.name === 'subagent_cancel').execute('c4', { task_id: third.details.taskId });
    assert.match(cancelled.content[0].text, /^cancelled sub-/);
    assert.equal(registry.get(third.details.taskId).state, 'cancelled', 'cancel of a queued run removes it pre-spawn');
    assert.equal(registry.get(third.details.taskId).pid, null);
    assert.ok(await waitFor(() => registry.get(second.details.taskId).pid !== null), 'queued run auto-starts when a slot frees');
    await waitFor(() => deliveries.length === 3);
    assert.deepEqual(
      deliveries.map((run) => [run.state, run.agent]),
      [['cancelled', 'probe'], ['completed', 'probe'], ['completed', 'probe']],
    );
    assert.equal(registry.get(second.details.taskId).state, 'completed');
  });

  it('resolves the cap from BIGGZ_BACKGROUND_SUBAGENTS (numeric override, on/off → 2, clamp ≥1)', () => {
    assert.equal(resolveBackgroundCap({}), 2);
    assert.equal(resolveBackgroundCap({ BIGGZ_BACKGROUND_SUBAGENTS: '1' }), 1);
    assert.equal(resolveBackgroundCap({ BIGGZ_BACKGROUND_SUBAGENTS: '4' }), 4);
    assert.equal(resolveBackgroundCap({ BIGGZ_BACKGROUND_SUBAGENTS: 'on' }), 2);
    assert.equal(resolveBackgroundCap({ BIGGZ_BACKGROUND_SUBAGENTS: 'off' }), 2);
    assert.equal(resolveBackgroundCap({ BIGGZ_BACKGROUND_SUBAGENTS: '0' }), 1);
    assert.equal(resolveBackgroundCap({ BIGGZ_BACKGROUND_SUBAGENTS: 'nope' }), 2);
  });

  it('renders max 2 widget rows + `… +N`, truncates via pi-tui, and hides when idle/child/pretty-off', () => {
    assert.deepEqual(widgetRowsFor([{ agent: 'sdd-apply', state: 'running', elapsedMs: 3200 }], { env: {}, width: 200 }), ['◐ sdd-apply · running · 3s']);
    assert.deepEqual(
      widgetRowsFor(
        [
          { agent: 'a', state: 'running', elapsedMs: 0 },
          { agent: 'b', state: 'queued', elapsedMs: 1000 },
          { agent: 'c', state: 'running', elapsedMs: 2000 },
        ],
        { env: {} },
      ),
      ['◐ a · running · 0s', '◌ b · queued · 1s', '… +1'],
    );
    assert.deepEqual(widgetRowsFor([{ agent: 'a', state: 'completed', elapsedMs: 0 }], { env: {} }), [], 'idle ⇒ hidden');
    assert.deepEqual(widgetRowsFor([{ agent: 'a', state: 'running', elapsedMs: 0 }], { env: { PI_SUBAGENT_CHILD: '1' } }), []);
    assert.deepEqual(widgetRowsFor([{ agent: 'a', state: 'running', elapsedMs: 0 }], { env: { BIGGZ_PRETTY: '0' } }), []);
    const narrow = widgetRowsFor([{ agent: 'x'.repeat(120), state: 'running', elapsedMs: 1000 }], { env: {}, width: 20 });
    assert.equal(narrow.length, 1);
    assert.ok(tui.visibleWidth(narrow[0]) <= 20, 'rows are truncateToWidth’d');
  });

  it('renders the exact wait headline ≤2 lines and never a run-list dump', () => {
    const runs = [
      { agent: 'sdd-apply', state: 'running' },
      { agent: 'sdd-verify', state: 'queued' },
    ];
    assert.deepEqual(renderWaitHeadline(runs, 23000), ['Wait 23s · 2 runs (sdd-apply running, sdd-verify queued)']);
    const hinted = renderWaitHeadline(runs, 23000, { hint: 'still running; completion notices arrive automatically' });
    assert.equal(hinted.length, 2, 'at most one hint line');
    assert.match(hinted[1], /^· /);
    const many = renderWaitHeadline(Array.from({ length: 9 }, (_, i) => ({ agent: `a${i}`, state: 'running' })), 1000);
    assert.equal(many.length, 1);
    assert.match(many[0], /^Wait 1s · 9 runs \(a0 running, a1 running, a2 running, a3 running, … \+5\)$/);
  });

  it('subagent_wait waits for the given runs and returns the bounded headline', async () => {
    const { byName } = toolsetFor({ deliverCompletion: () => {} });
    const first = await byName('subagent').execute('c1', { agent: 'probe', task: 'a', mode: 'background' });
    const second = await byName('subagent').execute('c2', { agent: 'probe', task: 'b', mode: 'background' });
    const waited = await byName('subagent_wait').execute('c3', { task_ids: [first.details.taskId, second.details.taskId], timeout_ms: 5000 });
    const lines = waited.content[0].text.split('\n');
    assert.ok(lines.length <= 2, '≤2 lines');
    assert.match(lines[0], /^Wait \d+s · 2 runs \(probe completed, probe completed\)$/);
    assert.equal(waited.details.count, 2);
    const unknown = await byName('subagent_wait').execute('c4', { task_ids: ['gone'] });
    assert.equal(unknown.isError, true);
    assert.match(unknown.content[0].text, /unknown or expired task "gone"/);
  });
});
