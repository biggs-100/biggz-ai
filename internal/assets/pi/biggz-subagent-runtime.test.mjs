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
import { EventEmitter } from 'node:events';
import { PassThrough } from 'node:stream';
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
  resolvePiLaunch,
  createTask,
  killTree,
  subagentRegistrationGate,
  LEGACY_TOOL_RE,
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
  activityLabel,
  formatTaskResult,
  formatTokens,
  formatCost,
  WIDGET_FINISHED_TTL_MS,
  runRow,
  registryRuns,
  createAgentsView,
  AGENTS_COMMAND,
  resolveSubagentSessionsDir,
  pruneSubagentSessions,
  transcriptEntries,
  transcriptEntry,
  DEFAULT_DIALOG_TIMEOUT_MS,
  resolveDialogTimeout,
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
if (process.env.FAKE_TOOL_EVENT === '1') out({ type: 'tool_execution_start', toolCallId: 't1', toolName: 'read', args: { path: 'internal/install/steps/pi_extensions.go' } });
if (process.env.FAKE_USAGE === '1') out({ type: 'message_end', message: { role: 'assistant', content: [{ type: 'text', text: 'hi' }], model: 'fake-model', usage: { totalTokens: 1234, cost: { total: 0.0042 } } } });
process.stdin.on('data', (chunk) => {
  for (const raw of String(chunk).split('\\n')) {
    if (!raw) continue;
    let cmd = null;
    try { cmd = JSON.parse(raw); } catch {}
    if (!cmd) continue;
    out({ type: 'fake_command', command: cmd });
    if (cmd.type === 'get_state') out({ type: 'response', command: 'get_state', success: true, data: { sessionFile: process.env.FAKE_SESSION_FILE ?? null, model: { provider: 'fake', id: 'fake-model' }, thinkingLevel: 'high' } });
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

// Scripted in-process child: full runtime path (listeners, JSONL reader, watchdog)
// without a real process, so tool-event timing is deterministic.
const scriptedChild = (pid = 987656) => {
  const child = new EventEmitter();
  child.pid = pid;
  child.stdout = new EventEmitter();
  child.stderr = new EventEmitter();
  child.stdin = { writable: true, write: () => true };
  return child;
};
const emitLine = (child, event) => child.stdout.emit('data', Buffer.from(`${JSON.stringify(event)}\n`));
const killStub = () => {
  throw Object.assign(new Error('gone'), { code: 'ESRCH' });
};

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
    assert.deepEqual(buildChildArgs({ tools: [], model: null }, { sessionDir: '/tmp/child-sessions' }), ['--mode', 'rpc', '--session-dir', '/tmp/child-sessions', '--tools', 'read']);
    assert.deepEqual(
      buildChildArgs({ tools: [], model: null }, { env: { PI_CODING_AGENT_DIR: '/tmp/agent' } }),
      ['--mode', 'rpc', '--session-dir', path.join('/tmp/agent', 'sessions', 'biggz-subagents'), '--tools', 'read'],
      'children keep a transcript under the agent dir by default',
    );
    assert.deepEqual(buildChildArgs({ tools: ['read', 'edit'], model: 'openai/gpt-5' }, { sessionDir: null }), ['--mode', 'rpc', '--no-session', '--tools', 'read,edit', '--model', 'openai/gpt-5']);
    const base = { PATH: '/usr/bin' };
    assert.equal(buildChildEnv(base).PI_SUBAGENT_CHILD, '1');
    assert.equal('PI_SUBAGENT_CHILD' in base, false, 'base env object must not be mutated');
    assert.equal(process.env.PI_SUBAGENT_CHILD, undefined);
  });
});

// A6: inside pi, re-exec the running package entry (`dist/bundle/cli.js`) through
// `process.execPath` instead of the Windows `pi.cmd` shim; outside pi keep the
// portable fallback; `BIGGZ_PI_BIN` always wins.
describe('resolvePiLaunch (A6 runtime parity)', () => {
  const PI_ENTRY = 'C:\\npm\\node_modules\\@earendil-works\\pi-coding-agent\\dist\\bundle\\cli.js';
  const TEST_ARGV1 = path.join(os.tmpdir(), 'biggz-subagent-runtime.test.mjs');

  it('lets BIGGZ_PI_BIN win on every platform', () => {
    for (const platform of ['win32', 'linux']) {
      assert.deepEqual(
        resolvePiLaunch({ env: { BIGGZ_PI_BIN: ' /opt/custom-pi ' }, platform, execPath: 'node', argv1: PI_ENTRY }),
        { command: '/opt/custom-pi', prefix: [], shell: false },
      );
    }
  });

  it('spawns node + the pi cli entry directly when the runtime runs inside pi', () => {
    assert.deepEqual(
      resolvePiLaunch({ env: {}, platform: 'win32', execPath: 'C:\\node\\node.exe', argv1: PI_ENTRY }),
      { command: 'C:\\node\\node.exe', prefix: [PI_ENTRY], shell: false },
    );
    const posixEntry = '/usr/lib/node_modules/@earendil-works/pi-coding-agent/dist/bundle/cli.js';
    assert.deepEqual(
      resolvePiLaunch({ env: {}, platform: 'linux', execPath: '/usr/bin/node', argv1: posixEntry }),
      { command: '/usr/bin/node', prefix: [posixEntry], shell: false },
    );
  });

  it('falls back to the portable launcher outside pi (win32 pi.cmd+shell, posix pi)', () => {
    assert.deepEqual(
      resolvePiLaunch({ env: {}, platform: 'win32', execPath: 'node', argv1: TEST_ARGV1 }),
      { command: 'pi.cmd', prefix: [], shell: true },
    );
    assert.deepEqual(
      resolvePiLaunch({ env: {}, platform: 'linux', execPath: 'node', argv1: TEST_ARGV1 }),
      { command: 'pi', prefix: [], shell: false },
    );
    // A bare cli.js outside the pi package must not switch modes (conservative detection).
    assert.deepEqual(
      resolvePiLaunch({ env: {}, platform: 'win32', execPath: 'node', argv1: 'C:\\somewhere\\cli.js' }),
      { command: 'pi.cmd', prefix: [], shell: true },
    );
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

  // #135: a silent tool call (e.g. a >4 min bash command) is work, not a stall.
  // pi 0.85.1 emits tool_execution_start/end with toolCallId + toolName; while a
  // tool is in flight the idle bound is suspended, the total bound still governs.
  it('suspends the idle bound while a tool is in flight and re-arms after tool_execution_end', async (t) => {
    const child = scriptedChild();
    const task = run({}, { idleMs: 60, spawnImpl: () => child, killGraceMs: 0, execImpl: () => {}, killImpl: killStub });
    t.after(() => task.cancel());
    emitLine(child, { type: 'tool_execution_start', toolCallId: 'call_1', toolName: 'bash', args: { command: 'sleep 600' } });
    await sleep(180); // > 2× idleMs of silence with a tool in flight
    assert.equal(task.state, 'running', 'an announced in-flight tool must suspend the idle bound');
    assert.equal(task.snapshot().reason, null);
    emitLine(child, { type: 'tool_execution_end', toolCallId: 'call_1', toolName: 'bash', result: { content: [] }, isError: false });
    const result = await Promise.race([task.promise, sleep(3000).then(() => null)]);
    assert.ok(result, 'silence after the last tool end must stall');
    assert.equal(result.state, 'stalled');
    assert.equal(result.reason, 'idle');
    assert.ok(await waitFor(() => !isAlive(result.pid)), 'stalled child must be terminated');
  });

  it('still stalls a silent child with no announced tool (control)', async (t) => {
    const child = scriptedChild(987657);
    const task = run({}, { idleMs: 60, spawnImpl: () => child, killGraceMs: 0, execImpl: () => {}, killImpl: killStub });
    t.after(() => task.cancel());
    const result = await Promise.race([task.promise, sleep(3000).then(() => null)]);
    assert.ok(result, 'a silent child with no tool event must stall as before');
    assert.equal(result.state, 'stalled');
    assert.equal(result.reason, 'idle');
  });

  it('keeps the idle bound suspended until the last of several in-flight tools ends', async (t) => {
    const child = scriptedChild(987658);
    const task = run({}, { idleMs: 60, spawnImpl: () => child, killGraceMs: 0, execImpl: () => {}, killImpl: killStub });
    t.after(() => task.cancel());
    emitLine(child, { type: 'tool_execution_start', toolCallId: 'call_a', toolName: 'bash' });
    emitLine(child, { type: 'tool_execution_start', toolCallId: 'call_b', toolName: 'read' });
    await sleep(180);
    assert.equal(task.state, 'running');
    emitLine(child, { type: 'tool_execution_end', toolCallId: 'call_a', toolName: 'bash', isError: false });
    await sleep(180);
    assert.equal(task.state, 'running', 'one tool still in flight must keep the idle bound suspended');
    emitLine(child, { type: 'tool_execution_end', toolCallId: 'call_b', toolName: 'read', isError: false });
    const result = await Promise.race([task.promise, sleep(3000).then(() => null)]);
    assert.equal(result?.state, 'stalled');
    assert.equal(result?.reason, 'idle');
  });

  it('keeps the total bound in effect while the idle bound is tool-suspended', async (t) => {
    const child = scriptedChild(987659);
    const task = run({}, { idleMs: 5000, totalMs: 120, spawnImpl: () => child, killGraceMs: 0, execImpl: () => {}, killImpl: killStub });
    t.after(() => task.cancel());
    emitLine(child, { type: 'tool_execution_start', toolCallId: 'long', toolName: 'bash' });
    const result = await Promise.race([task.promise, sleep(3000).then(() => null)]);
    assert.ok(result, 'the total bound must settle a tool-suspended task');
    assert.equal(result.state, 'stalled');
    assert.equal(result.reason, 'total');
  });

  it('degrades safely on malformed tool events (missing keys fall back, never throw)', async (t) => {
    const warns = [];
    const child = scriptedChild(987660);
    const task = run(
      {},
      { idleMs: 60, spawnImpl: () => child, killGraceMs: 0, execImpl: () => {}, killImpl: killStub, logger: { warn: (m) => warns.push(m), error() {} } },
    );
    t.after(() => task.cancel());
    emitLine(child, { type: 'tool_execution_start' }); // no toolCallId/toolName
    emitLine(child, { type: 'tool_execution_start', toolName: 'bash' }); // name-only fallback
    await sleep(180);
    assert.equal(task.state, 'running', 'malformed starts must still suspend the idle bound');
    emitLine(child, { type: 'tool_execution_end' }); // clears the unnamed fallback key only
    await sleep(180);
    assert.equal(task.state, 'running', 'the name-keyed tool is still in flight');
    emitLine(child, { type: 'tool_execution_end', toolName: 'bash' });
    const result = await Promise.race([task.promise, sleep(3000).then(() => null)]);
    assert.equal(result?.state, 'stalled');
    assert.equal(result?.reason, 'idle');
    assert.deepEqual(warns, [], 'malformed tool events must not surface a handler error');
  });

  // F1: an oversized `tool_execution_end` line is dropped by the 1 MiB cap; its
  // key must not stay in flight forever turning a 4 min idle bound into 30 min.
  it('re-arms the idle bound when a dropped line may have been tool_execution_end', async (t) => {
    const warns = [];
    const child = scriptedChild(987661);
    const task = run(
      {},
      { idleMs: 60, totalMs: 1200, spawnImpl: () => child, killGraceMs: 0, execImpl: () => {}, killImpl: killStub, logger: { warn: (m) => warns.push(m), error() {} } },
    );
    t.after(() => task.cancel());
    emitLine(child, { type: 'tool_execution_start', toolCallId: 'call_1', toolName: 'bash' });
    const oversized = `${JSON.stringify({ type: 'tool_execution_end', toolCallId: 'call_1', toolName: 'bash', result: { content: [{ type: 'text', text: 'x'.repeat(JSONL_MAX_LINE_BYTES + 64) }] }, isError: false })}\n`;
    child.stdout.emit('data', Buffer.from(oversized));
    const result = await Promise.race([task.promise, sleep(3000).then(() => null)]);
    assert.ok(result, 'a dropped end line must not suspend the idle bound forever');
    assert.equal(result.state, 'stalled');
    assert.equal(result.reason, 'idle', 'the plain idle bound must re-arm after a framing drop');
    assert.ok(warns.some((w) => /oversized JSONL line skipped/.test(w)), 'the drop must be logged');
    assert.ok(await waitFor(() => !isAlive(result.pid)), 'stalled child must be terminated');
  });

  // Regression for #132: settle() → clearTimers() used to race a late stdout
  // chunk, whose handler re-armed a fresh REFERENCED 240 s idle timer and kept
  // `pi -p` alive ~4 min after the tool result. Observable contract: after
  // settle, late chunks arm nothing, mutate nothing, and no watchdog holds a ref.
  it('keeps a late stdout/stderr chunk after settle inert (no re-armed referenced idle timer)', async () => {
    const IDLE_MS = 240000;
    const child = new EventEmitter();
    child.pid = 987654;
    child.stdout = new EventEmitter();
    child.stderr = new EventEmitter();
    child.stdin = { writable: true, write: () => true };
    const armed = [];
    const realSetTimeout = globalThis.setTimeout;
    globalThis.setTimeout = (fn, ms, ...rest) => {
      const timer = realSetTimeout(fn, ms, ...rest);
      if (ms === IDLE_MS) armed.push(timer);
      return timer;
    };
    let task;
    try {
      task = run(
        {},
        {
          spawnImpl: () => child,
          idleMs: IDLE_MS,
          finalTextMs: 0, // agent_settled ⇒ sync settle, no timer round-trip
          killGraceMs: 0,
          execImpl: () => {},
          killImpl: () => {
            throw Object.assign(new Error('gone'), { code: 'ESRCH' });
          },
        },
      );
      child.stdout.emit('data', Buffer.from('{"type":"tick"}\n'));
      assert.equal(armed.length, 2, 'spawn arms the idle watchdog; a running chunk re-arms it');
      child.stdout.emit('data', Buffer.from('{"type":"agent_settled"}\n'));
      assert.equal((await task.promise).state, 'completed');
      const armsAfterSettle = armed.length;
      const before = task.snapshot();
      child.stdout.emit('data', Buffer.from('{"type":"message_update"}\n'));
      child.stderr.emit('data', Buffer.from('late stderr noise'));
      const after = task.snapshot();
      assert.equal(armed.length, armsAfterSettle, 'a late stdout chunk must not arm another idle watchdog');
      assert.equal(after.events.length, before.events.length, 'late chunks must not be processed after settle');
      assert.equal(after.stderrTail, before.stderrTail, 'stderr tail must stay frozen at settle');
      assert.ok(armed.length > 0, 'the spy must have observed the idle watchdog arms');
      assert.ok(armed.every((timer) => timer.hasRef?.() === false), 'idle watchdogs must be unref’d');
    } finally {
      globalThis.setTimeout = realSetTimeout;
    }
  });

  // A3: a multibyte char split across two stdout/stderr chunks must decode intact
  // (stream setEncoding), not corrupt to U+FFFD through per-chunk Buffer.toString.
  it('decodes multibyte chars split across stdout/stderr writes (no U+FFFD)', async (t) => {
    const child = new EventEmitter();
    child.pid = 987655;
    child.stdout = new PassThrough();
    child.stderr = new PassThrough();
    child.stdin = { writable: true, write: () => true };
    const task = run(
      {},
      {
        spawnImpl: () => child,
        finalTextMs: 0, // agent_settled ⇒ sync settle
        killGraceMs: 0,
        execImpl: () => {},
        killImpl: () => {
          throw Object.assign(new Error('gone'), { code: 'ESRCH' });
        },
      },
    );
    t.after(() => task.cancel());
    const line = Buffer.from(`${JSON.stringify({ type: 'note', text: '✅ done' })}\n`, 'utf8');
    const cut = line.indexOf(0xe2) + 1; // split inside ✅ (E2 9C 85)
    assert.ok(cut > 0 && cut < line.length, 'fixture must split inside the ✅ bytes');
    child.stdout.write(line.subarray(0, cut));
    child.stdout.write(line.subarray(cut));
    child.stderr.write(Buffer.from([0xe2]));
    child.stderr.write(Buffer.from([0x9c, 0x85]));
    const note = await waitFor(() => task.snapshot().events.find((event) => event.type === 'note'));
    assert.equal(note?.text, '✅ done', 'split multibyte stdout must decode intact');
    assert.ok(!JSON.stringify(note).includes('\uFFFD'), 'no replacement char may survive decoding');
    child.stdout.write('{"type":"agent_settled"}\n');
    const result = await task.promise;
    assert.equal(result.state, 'completed');
    assert.equal(result.stderrTail, '✅', 'split multibyte stderr must decode intact');
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
  commands: [],
  getAllTools: options.getAllTools ?? (() => options.tools ?? []),
  ...(options.getToolDefinition ? { getToolDefinition: options.getToolDefinition } : {}),
  registerTool(definition) {
    this.registered.push(definition);
  },
  registerMessageRenderer(type, renderer) {
    this.renderers.push([type, renderer]);
  },
  registerCommand(name, definition) {
    this.commands.push([name, definition]);
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
  it('classifies only the exact j0k3r names as legacy', () => {
    for (const legacy of ['subagent_run', 'subagent_list_running']) assert.equal(LEGACY_TOOL_RE.test(legacy), true, `${legacy} must stay legacy`);
    for (const own of ['subagent', 'subagent_status', 'subagent_list_tasks', 'subagent_send_message']) {
      assert.equal(LEGACY_TOOL_RE.test(own), false, `${own} must not classify as legacy`);
    }
  });

  it('registers when the scan returns only the runtime’s own 8-name surface', () => {
    const dir = fakeAgentDir(['npm:@heyhuynhgiabuu/pi-pretty']);
    const own = ['subagent', 'subagent_wait', 'subagent_status', 'subagent_result', 'subagent_cancel', 'subagent_agents', 'subagent_list_tasks', 'subagent_send_message'];
    const pi = fakePi({ tools: own.map((name) => ({ name })) });
    const gate = subagentRegistrationGate(pi, { env: { PI_CODING_AGENT_DIR: dir }, logger: LOG });
    assert.equal(gate.register, true, 'the runtime’s own list tool must not cause registration refusal');
    withAgentEnv(dir, () => runtime(pi));
    assert.deepEqual(pi.registered.map((d) => d.name), own);
  });

  it('refuses genuine j0k3r names (subagent_run + subagent_list_running) with zero registrations', () => {
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

  it('registers the eight tools + completion renderer when j0k3r is absent (load-time getAllTools throw tolerated)', () => {
    const dir = fakeAgentDir(['npm:@heyhuynhgiabuu/pi-pretty']);
    // 0.85.1 throws this exact message while extensions load — falling through, not failing closed.
    const pi = fakePi({ getAllTools: () => { throw new Error('Extension runtime not initialized. Action methods cannot be called during extension loading.'); } });
    const gate = subagentRegistrationGate(pi, { env: { PI_CODING_AGENT_DIR: dir }, logger: LOG });
    assert.equal(gate.register, true);
    withAgentEnv(dir, () => runtime(pi));
    assert.deepEqual(pi.registered.map((d) => d.name), ['subagent', 'subagent_wait', 'subagent_status', 'subagent_result', 'subagent_cancel', 'subagent_agents', 'subagent_list_tasks', 'subagent_send_message']);
    assert.equal(pi.renderers.length, 1);
    assert.equal(pi.renderers[0][0], COMPLETION_MESSAGE_TYPE);
    assert.equal(typeof pi.renderers[0][1], 'function');
    assert.ok(!/\.getTool\(/.test(SRC), 'pi.getTool is absent in 0.85.1 and must never be called');
  });
});

describe('S2 agents panel (/biggz-agents)', () => {
  const theme = { fg: (_role, text) => String(text), bold: (text) => String(text) };
  const runs = [
    { id: 'a', agent: 'sdd-apply', state: 'running', elapsedMs: 4200, lastStep: 'read x.go', tokens: 12345, cost: 0.0042 },
    { id: 'b', agent: 'sdd-verify', state: 'completed', elapsedMs: 9000, settledAt: Date.now(), summary: 'verify spec' },
  ];

  it('renders one row per run and closes on q/escape', () => {
    const closed = [];
    const view = createAgentsView({ theme, runs, close: (value) => closed.push(value) });
    const lines = view.render(80);
    assert.match(lines[0], /Subagent runs/);
    assert.deepEqual(
      lines.slice(2, 4),
      ['→ ◐ sdd-apply · read x.go · 4s · 12.3k tok · $0.0042', '  ✅ sdd-verify · verify spec · 9s'],
      'one row per run, the selected one marked, spend included',
    );
    assert.match(lines.at(-1), /↑↓ move · s stop · o transcript · q close/);
    view.handleInput('q');
    view.handleInput('escape');
    assert.deepEqual(closed, [null, null]);
  });

  it('moves the selection and stops the selected run', () => {
    const stopped = [];
    const view = createAgentsView({ theme, runs, stop: (run) => { stopped.push(run.id); return true; } });
    view.handleInput('j');
    view.handleInput('s');
    assert.deepEqual(stopped, ['b'], 'after ↓ the second row is the selected one');
    assert.match(view.render(80).at(-1), /stopping sdd-verify/);
    const refused = createAgentsView({ theme, runs, stop: () => false });
    refused.handleInput('s');
    assert.match(refused.render(80).at(-1), /nothing to stop/);
    const empty = createAgentsView({ theme, runs: [] });
    assert.match(empty.render(80)[2], /no subagent runs in this session/);
  });

  it('lists live work first and keeps only fresh completions', () => {
    const now = 1_000_000;
    const fake = (id, state, settledAt) => ({
      id,
      agent: id,
      state,
      task: `task ${id}`,
      snapshot: () => ({ elapsedMs: 1000, settledAt, lastStep: '', tokens: 0, cost: 0 }),
    });
    const registry = {
      all: () => [fake('old', 'completed', now - WIDGET_FINISHED_TTL_MS - 1), fake('run', 'running', 0), fake('fresh', 'completed', now - 500)],
    };
    assert.deepEqual(registryRuns(registry, now).map((run) => run.id), ['run', 'fresh'], 'active first, then the fresh completion, never the stale one');
    assert.equal(runRow({ agent: 'x', state: 'running', elapsedMs: 1000 }, 0), '◐ x · running · 1s');
  });

  it('registers the command and opens the overlay with the session runs', async () => {
    const dir = fakeAgentDir(['npm:@heyhuynhgiabuu/pi-pretty']);
    const pi = fakePi({ getAllTools: () => [] });
    withAgentEnv(dir, () => runtime(pi));
    assert.equal(pi.commands.length, 1);
    assert.equal(pi.commands[0][0], AGENTS_COMMAND);
    const custom = [];
    const ctx = { mode: 'tui', ui: { custom: async (factory, opts) => { custom.push([factory, opts]); return null; }, notify() {} } };
    await pi.commands[0][1].handler([], ctx);
    assert.equal(custom.length, 1, 'the handler must open one overlay');
    assert.equal(custom[0][1].overlay, true);
    const view = custom[0][0]({ requestRender() {} }, theme, {}, () => {});
    assert.equal(typeof view.render, 'function');
    assert.equal(typeof view.handleInput, 'function');
    const notified = [];
    await pi.commands[0][1].handler([], { mode: 'rpc', ui: { notify: (m) => notified.push(m) } });
    assert.deepEqual(notified, ['the subagents panel needs TUI mode']);
  });
});

describe('S3 child transcripts', () => {
  const theme = { fg: (_role, text) => String(text), bold: (text) => String(text) };

  it('formats a session file into bounded one-line entries', () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'biggz-transcript-'));
    const file = path.join(dir, 'session.jsonl');
    const records = [
      { type: 'session', timestamp: '2026-09-19T20:31:05.000Z' },
      { type: 'message', timestamp: '2026-09-19T20:31:06.000Z', message: { role: 'user', content: [{ type: 'text', text: 'read the file' }] } },
      { type: 'message', timestamp: '2026-09-19T20:31:07.000Z', message: { role: 'assistant', content: [{ type: 'thinking', thinking: 'hmm' }, { type: 'toolCall', name: 'read', arguments: { path: 'a.go' } }] } },
      { type: 'message', timestamp: '2026-09-19T20:31:08.000Z', message: { role: 'assistant', content: [{ type: 'text', text: 'done' }] } },
    ];
    fs.writeFileSync(file, records.map((record) => JSON.stringify(record)).join('\n') + '\n');
    const entries = transcriptEntries(file);
    assert.equal(entries.length, 4);
    assert.match(entries[0], /── session start/);
    assert.match(entries[1], /20:31:06 user\s+read the file/);
    assert.match(entries[2], /assistant\s+⚙ read a\.go/);
    assert.match(entries[3], /assistant\s+done/);
    assert.equal(transcriptEntry(null), '');
    assert.equal(transcriptEntry({ type: 'model_change' }), '', 'noise records stay out');
    assert.deepEqual(transcriptEntries(path.join(dir, 'missing.jsonl')), [], 'a missing file degrades to empty');
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('resolves the transcript directory and prunes all but the newest', () => {
    assert.equal(resolveSubagentSessionsDir({ PI_CODING_AGENT_DIR: '/tmp/agent' }), path.join('/tmp/agent', 'sessions', 'biggz-subagents'));
    assert.equal(resolveSubagentSessionsDir({ BIGGZ_SUBAGENT_SESSION_DIR: '/tmp/elsewhere' }), '/tmp/elsewhere');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'biggz-prune-'));
    for (const [name, ageMs] of [['newest.jsonl', 0], ['mid.jsonl', 60_000], ['oldest.jsonl', 120_000]]) {
      const file = path.join(dir, name);
      fs.writeFileSync(file, '{}\n');
      const when = new Date(Date.now() - ageMs);
      fs.utimesSync(file, when, when);
    }
    fs.writeFileSync(path.join(dir, 'keep.txt'), 'not a transcript');
    assert.equal(pruneSubagentSessions(dir, 2), 1, 'only the oldest transcript goes');
    assert.deepEqual(fs.readdirSync(dir).sort(), ['keep.txt', 'mid.jsonl', 'newest.jsonl']);
    assert.equal(pruneSubagentSessions(dir, 0), 2);
    assert.equal(pruneSubagentSessions(null), 0);
    assert.equal(pruneSubagentSessions(path.join(dir, 'gone'), 5), 0, 'a missing dir is not a crash');
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('exposes the child transcript path, model and effort from get_state', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'biggz-session-'));
    const file = path.join(dir, 'child.jsonl');
    const task = run({ FAKE_SETTLED: '1', FAKE_SESSION_FILE: file }, { sessionDir: dir });
    const result = await task.promise;
    assert.equal(result.state, 'completed');
    assert.equal(result.transcriptPath, file, 'the child reports where its transcript lives');
    assert.equal(result.model, 'fake-model');
    assert.equal(result.thinking, 'high');
    fs.rmSync(dir, { recursive: true, force: true });
  });

  it('reads a run transcript inside the panel and returns to the list', () => {
    const runs = [{ id: 'a', agent: 'sdd-apply', state: 'running', elapsedMs: 1200, lastStep: 'read a.go' }];
    const view = createAgentsView({ theme, runs, transcript: () => ({ path: '/tmp/child.jsonl', lines: ['first line', 'second line'] }) });
    view.handleInput('o');
    const lines = view.render(80);
    assert.match(lines[0], /Transcript · sdd-apply/);
    assert.match(lines[1], /\/tmp\/child\.jsonl/);
    assert.ok(lines.includes('first line') && lines.includes('second line'), 'the transcript body is rendered');
    view.handleInput('q');
    assert.match(view.render(80)[0], /Subagent runs/, 'q returns to the run list');
    const missing = createAgentsView({ theme, runs, transcript: () => null });
    missing.handleInput('o');
    assert.match(missing.render(80).join('\n'), /no session file/);
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

describe('subagent_list_tasks (session ring only, newest first)', () => {
  const record = (id, state) => ({ id, state, snapshot: () => ({ id, state, elapsedMs: 0 }) });
  const listToolFor = (registry) => createSubagentToolset({ registry }).tools.find((tool) => tool.name === 'subagent_list_tasks');

  it('lists settled and running records newest first', async () => {
    const registry = createTaskRegistry({ limit: 50 });
    registry.add(record('t1', 'completed'));
    registry.add(record('t2', 'completed'));
    registry.add(record('t3', 'running'));
    const result = await listToolFor(registry).execute('c1', {});
    assert.equal(result.isError, undefined);
    assert.deepEqual(result.content[0].text.split('\n'), [
      'subagent t3 · running · 0.0s',
      'subagent t2 · completed · 0.0s',
      'subagent t1 · completed · 0.0s',
    ]);
    assert.equal(result.details.count, 3);
  });

  it('returns a bounded empty state for an empty ring, not an error', async () => {
    const result = await listToolFor(createTaskRegistry()).execute('c2', {});
    assert.equal(result.isError, undefined);
    assert.equal(result.content[0].text, 'no subagent tasks this session');
    assert.equal(result.details.count, 0);
  });

  it('caps rows at the ring limit over bounded live-work overflow (ring-only read)', async () => {
    const registry = createTaskRegistry({ limit: 2 });
    for (const id of ['t1', 't2', 't3']) registry.add(record(id, 'running'));
    assert.equal(registry.all().length, 3, 'live work may exceed the ring bound (nothing evictable)');
    const result = await listToolFor(registry).execute('c3', {});
    assert.equal(result.isError, undefined);
    assert.deepEqual(result.content[0].text.split('\n').map((row) => row.split(' · ')[0]), ['subagent t3', 'subagent t2']);
    assert.equal(result.details.count, 2);
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
    assert.deepEqual(tools.map((t) => t.name), ['subagent', 'subagent_wait', 'subagent_status', 'subagent_result', 'subagent_cancel', 'subagent_agents', 'subagent_list_tasks', 'subagent_send_message']);

    const run = await byName('subagent').execute('call-1', { agent: 'probe', task: 'do the thing' });
    assert.equal(run.isError, undefined);
    // A1: foreground task mode returns the settlement line AND the child’s bounded final text.
    assert.match(run.content[0].text, /^subagent sub-[\w-]+ · completed · [\d.]+s · settled\nfinal answer$/);
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

  it('names the asking subagent in the relayed question', async () => {
    const seen = [];
    const answer = await presentUiRequest(
      { ui: { select: async (title, options) => { seen.push([title, options]); return 'Allow'; } } },
      { method: 'select', title: 'Pick one', options: ['Allow'] },
      { timeout: 1000, agentLabel: 'sdd-propose' },
    );
    assert.deepEqual(seen, [['sdd-propose: Pick one', ['Allow']]], 'the human must see who is asking');
    assert.deepEqual(answer, { value: 'Allow' });
  });

  it('answers `cancelled` at the dialog bound when the parent never answers, and the run continues', async (t) => {
    // The deployed failure: a child asks (sdd-* agents ship `ask_user_question`),
    // the parent relays it, nobody answers — the child blocks until the 30-min
    // total watchdog and the user sees a silent hang. The relay must bound it.
    const warns = [];
    let resolvePresentation;
    const task = run(
      { FAKE_UI_REQUEST: '1' },
      {
        dialogMs: 150,
        logger: { warn: (m) => warns.push(m), error() {} },
        onUiRequest: () => new Promise((resolve) => { resolvePresentation = resolve; }), // never answered by a human
      },
    );
    t.after(() => task.cancel());
    const result = await task.promise;
    assert.equal(result.state, 'completed', 'the run must continue instead of blocking on the unanswered dialog');
    assert.equal(result.dialogRelayed, 1);
    assert.equal(result.dialogMissed, 1, 'the missed question must be recorded on the snapshot');
    assert.equal(result.pendingUi, 0);
    assert.equal(result.pendingDialog, null);
    const answers = result.events.filter((e) => e.type === 'fake_command' && e.command.type === 'extension_ui_response');
    assert.deepEqual(answers.map((e) => e.command), [{ type: 'extension_ui_response', id: 'q1', cancelled: true }], 'the child must receive `cancelled` so it can degrade and continue');
    assert.equal(result.events.find((e) => e.type === 'dialog_answer')?.cancelled, true);
    assert.ok(warns.some((m) => m.includes('unanswered')), `the miss must be logged; got ${JSON.stringify(warns)}`);
    // A late human answer after the bound must not answer the child twice.
    resolvePresentation?.({ value: 'Allow' });
    await sleep(60);
    const after = task.snapshot().events.filter((e) => e.type === 'fake_command' && e.command.type === 'extension_ui_response');
    assert.equal(after.length, 1, 'exactly one answer per relayed question');
  });

  it('keeps the idle bound honest: an unanswered dialog cannot hold the run open', async (t) => {
    const child = scriptedChild();
    const writes = [];
    child.stdin = { writable: true, write: (line) => { writes.push(String(line)); return true; } };
    const task = run(
      {},
      {
        dialogMs: 120,
        idleMs: 100000, // the total bound would be the only escape without the dialog bound
        totalMs: 100000,
        spawnImpl: () => child,
        killGraceMs: 0,
        execImpl: () => {},
        killImpl: () => {},
        onUiRequest: () => new Promise(() => {}),
      },
    );
    t.after(() => task.cancel());
    emitLine(child, { type: 'extension_ui_request', id: 'q1', method: 'select', title: 'Pick one', options: ['A', 'B'] });
    await waitFor(() => task.snapshot().pendingUi === 1, 2000);
    assert.equal(task.snapshot().pendingDialog?.title, 'Pick one');
    await sleep(220);
    assert.ok(writes.some((line) => line.includes('"extension_ui_response"') && line.includes('"cancelled":true')), `the child must be unblocked at the bound; got ${JSON.stringify(writes)}`);
    assert.equal(task.snapshot().pendingUi, 0, 'a missed dialog must not stay pending');
    assert.equal(task.snapshot().pendingDialog, null);
  });

  it('passes the dialog budget to the parent UI and announces the question', async () => {
    const seen = [];
    const notifies = [];
    const answer = await presentUiRequest(
      {
        ui: {
          notify: (message, type) => notifies.push([message, type]),
          select: async (title, options, opts) => {
            seen.push([title, options, opts]);
            return 'Allow';
          },
        },
      },
      { method: 'select', title: 'Pick one', options: ['Allow', 'Block'] },
      { timeout: 5000 },
    );
    assert.deepEqual(answer, { value: 'Allow' });
    assert.deepEqual(seen, [['Pick one', ['Allow', 'Block'], { timeout: 5000 }]], 'the TUI must own the countdown for the same budget the runtime enforces');
    assert.deepEqual(notifies, [['Subagent question: Pick one', 'info']]);
    const noBudget = [];
    await presentUiRequest({ ui: { select: async (...args) => { noBudget.push(args[2]); return 'A'; } } }, { method: 'select', title: 't', options: ['A'] });
    assert.deepEqual(noBudget, [undefined], 'no configured budget ⇒ no artificial dialog option');
  });

  it('resolves the dialog budget from BIGGZ_SUBAGENT_DIALOG_MS', () => {
    assert.equal(resolveDialogTimeout({}), DEFAULT_DIALOG_TIMEOUT_MS);
    assert.equal(resolveDialogTimeout({ BIGGZ_SUBAGENT_DIALOG_MS: '45000' }), 45000);
    assert.equal(resolveDialogTimeout({ BIGGZ_SUBAGENT_DIALOG_MS: '0' }), 0);
    assert.equal(resolveDialogTimeout({ BIGGZ_SUBAGENT_DIALOG_MS: 'nonsense' }), DEFAULT_DIALOG_TIMEOUT_MS);
    assert.equal(resolveDialogTimeout({ BIGGZ_SUBAGENT_DIALOG_MS: '-5' }), DEFAULT_DIALOG_TIMEOUT_MS);
  });

  it('surfaces the unanswered question in the subagent tool result', async () => {
    const toolset = toolsetFor({
      dialogMs: 120,
      presentUiRequest: () => new Promise(() => {}),
      // The fake child asks at start and only settles once it is answered.
      env: { ...process.env, PI_CODING_AGENT_DIR: probeDir(), FAKE_UI_REQUEST: '1' },
    });
    const tool = toolset.byName('subagent');
    const result = await tool.execute('call1', { agent: 'probe', task: 'do it', mode: 'task' }, undefined, undefined, fakeCtx());
    assert.equal(result.isError, undefined);
    const text = result.content[0].text;
    assert.match(text, /1 child question not answered within 1s/, `the caller must see why the child degraded; got ${text}`);
    assert.equal(result.details.dialogsMissed, 1);
  });

  it('dismisses a background child question instead of asking a human who is not waiting', async (t) => {
    // gentle-shell parity: nobody waits on a background child, so its question is answered
    // `cancelled` at once and the parent UI is never touched (it used to steal the human's
    // Enter and answer option 0 in their name).
    const env = { ...process.env, PI_CODING_AGENT_DIR: probeDir(), FAKE_SETTLED: '1', FAKE_UI_REQUEST: '1' };
    delete env.BIGGZ_BACKGROUND_SUBAGENTS;
    const presented = [];
    const warns = [];
    const { registry, byName } = toolsetFor({
      env,
      logger: { warn: (m) => warns.push(m), error() {} },
      presentUiRequest: async (request) => { presented.push(request); return { value: 'Allow' }; },
    });
    const launched = await byName('subagent').execute('b1', { agent: 'probe', task: 'do the thing', mode: 'background' });
    const id = launched.details.taskId;
    t.after(() => registry.get(id)?.cancel());
    assert.ok(await waitFor(() => registry.get(id)?.state === 'completed'), 'the child must continue and settle on its own');
    assert.deepEqual(presented, [], 'a background question must never reach the parent UI');
    const snap = registry.get(id).snapshot();
    assert.equal(snap.dialogsDismissed, 1, 'the dismissal must be recorded on the snapshot');
    assert.equal(snap.dialogRelayed, 0);
    assert.equal(snap.dialogMissed, 0);
    assert.deepEqual(
      snap.events.filter((e) => e.type === 'fake_command' && e.command.type === 'extension_ui_response').map((e) => e.command),
      [{ type: 'extension_ui_response', id: 'q1', cancelled: true }],
      'the child must receive `cancelled` immediately',
    );
    assert.ok(warns.some((m) => m.includes('dismissed without asking')), `the dismissal must be logged; got ${JSON.stringify(warns)}`);
  });

  it('carries the asking agent id on a foreground relay', async () => {
    const env = { ...process.env, PI_CODING_AGENT_DIR: probeDir(), FAKE_SETTLED: '1', FAKE_UI_REQUEST: '1' };
    delete env.BIGGZ_BACKGROUND_SUBAGENTS;
    const labels = [];
    const { byName } = toolsetFor({
      env,
      presentUiRequest: async (_ctx, _request, dialogOpts) => { labels.push(dialogOpts?.agentLabel); return { value: 'Allow' }; },
    });
    const result = await byName('subagent').execute('f1', { agent: 'probe', task: 'do the thing' });
    assert.equal(result.isError, undefined);
    assert.deepEqual(labels, ['probe'], 'a foreground relay must identify the asking subagent');
    assert.equal(result.details.dialogsMissed, 0);
    assert.equal(result.details.dialogsDismissed, 0);
  });

  it('Esc aborts a running foreground delegation instead of waiting forever', async () => {
    // pi passes an AbortSignal to tool.execute and documents it as the way Esc cancels work.
    // The runtime ignored it, so a slow child kept running while the TUI showed `Working…`
    // forever and the only exit was killing pi.
    const env = { ...process.env, PI_CODING_AGENT_DIR: probeDir(), FAKE_TICK_MS: '30' };
    delete env.BIGGZ_BACKGROUND_SUBAGENTS;
    const { registry, byName } = toolsetFor({ env });
    const controller = new AbortController();
    const pending = byName('subagent').execute('a1', { agent: 'probe', task: 'do the thing' }, controller.signal);
    assert.ok(await waitFor(() => registry.active().length === 1), 'the child must be running before the abort');
    controller.abort();
    const result = await pending; // a hang here IS the bug: the tool must settle
    assert.equal(result.details.state, 'cancelled');
    assert.match(result.content[0].text, /cancelled/, 'the caller must see the cancellation');
    assert.match(result.content[0].text, /cancelled by user \(Esc\)/);
    assert.equal(registry.active().length, 0, 'the child tree must be gone');
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

  it('subagent_send_message steers a running background child through the tool', async () => {
    const env = { ...process.env, PI_CODING_AGENT_DIR: probeDir(), FAKE_TICK_MS: '30', FAKE_SETTLED: '1' };
    delete env.BIGGZ_BACKGROUND_SUBAGENTS;
    const { registry, byName } = toolsetFor({ cap: 1, env });
    const launched = await byName('subagent').execute('c1', { agent: 'probe', task: 'a', mode: 'background' });
    const id = launched.details.taskId;
    const steered = await byName('subagent_send_message').execute('c2', { task_id: id, message: 'change of plan' });
    assert.equal(steered.isError, undefined);
    assert.match(steered.content[0].text, new RegExp(`^steered ${id} · running$`));
    assert.ok(
      await waitFor(() => registry.get(id).snapshot().events.some((e) => e.type === 'fake_command' && e.command.type === 'steer' && e.command.message === 'change of plan')),
      'the steer command must reach the fake child',
    );
    assert.ok(await waitFor(() => registry.get(id)?.state === 'completed'), 'the child settles on its own; no kill needed');
  });

  it('subagent_send_message bounds unknown/expired ids without throwing', async () => {
    const { byName } = toolsetFor({});
    const result = await byName('subagent_send_message').execute('c3', { task_id: 'sub-gone', message: 'hi' });
    assert.equal(result.isError, true);
    assert.match(result.content[0].text, /^unknown or expired task "sub-gone"$/);
  });

  it('subagent_send_message refuses a queued task with the bounded state error', async () => {
    const { registry, byName } = toolsetFor({ cap: 1 }); // default env: FAKE_SETTLED=1
    const first = await byName('subagent').execute('c4', { agent: 'probe', task: 'a', mode: 'background' });
    const second = await byName('subagent').execute('c5', { agent: 'probe', task: 'b', mode: 'background' });
    assert.equal(first.details.state, 'running');
    assert.equal(second.details.state, 'queued');
    assert.equal(registry.get(second.details.taskId).pid, null, 'queued runs have no live child');
    const queued = await byName('subagent_send_message').execute('c6', { task_id: second.details.taskId, message: 'x' });
    assert.equal(queued.isError, true);
    assert.match(queued.content[0].text, new RegExp(`^task "${second.details.taskId}" is not running \\(queued\\); steer not delivered$`));
    assert.ok(await waitFor(() => registry.get(second.details.taskId)?.state === 'completed'), 'both runs settle on their own; no kill needed');
  });

  it('subagent_send_message refuses a settled task with the bounded state error', async () => {
    const { registry, byName } = toolsetFor({ cap: 1 }); // default env: FAKE_SETTLED=1
    const launched = await byName('subagent').execute('c7', { agent: 'probe', task: 'a', mode: 'background' });
    const id = launched.details.taskId;
    assert.ok(await waitFor(() => registry.get(id)?.state === 'completed'));
    const settled = await byName('subagent_send_message').execute('c8', { task_id: id, message: 'too late' });
    assert.equal(settled.isError, true);
    assert.match(settled.content[0].text, new RegExp(`^task "${id}" is not running \\(completed\\); steer not delivered$`));
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
    const rows = ctx.ui.calls.at(-1)[1];
    assert.deepEqual(rows.map((row) => row.replace(/ · \d+s$/, '')), ['◐ probe · a', '◐ probe · b'], 'widget rows name each active run by its task summary');
    assert.equal(ctx.ui.calls.at(-1)[0], 'biggz-subagents');
    assert.deepEqual(ctx.ui.calls.at(-1)[2], { placement: 'belowEditor' }, 'placement must ride the options object');
    await waitFor(() => deliveries.length === 2);
    assert.deepEqual(deliveries.map((run) => run.state), ['completed', 'completed']);
    assert.equal(deliveries[0].summary, 'final answer');
    const rowsAfter = ctx.ui.calls.at(-1)[1];
    assert.deepEqual(
      rowsAfter.map((row) => row.replace(/ · \d+s.*$/, '')),
      ['✅ probe · a', '✅ probe · b'],
      'completions stay visible for a minute before fading (gentle-shell parity)',
    );
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

  it('derives a bounded activity label from the child’s own events', () => {
    assert.equal(activityLabel({ type: 'tool_execution_start', toolName: 'grep', args: { pattern: 'dialogPolicy' } }), 'grep dialogPolicy');
    assert.equal(activityLabel({ type: 'tool_execution_start', toolName: 'bash', args: { command: 'go test ./...\nmore' } }), 'bash go test ./...');
    assert.equal(activityLabel({ type: 'tool_execution_start', toolName: 'read' }), 'read');
    assert.equal(activityLabel({ type: 'message_start', message: { role: 'assistant' } }), 'thinking');
    assert.equal(activityLabel({ type: 'message_update' }), '', 'chatty events must not flicker the label');
    assert.equal(activityLabel({ type: 'tool_execution_end', toolName: 'read' }), '');
    assert.equal(activityLabel(null), '');
  });

  it('shows what a running child is doing in the widget row and the status line', async (t) => {
    // "How is it working?" must be answerable while it runs: the glyph carries the state,
    // so the row carries the live activity (or the task summary before the first tool).
    const task = run({ FAKE_TICK_MS: '40', FAKE_TOOL_EVENT: '1' });
    t.after(() => task.cancel());
    assert.ok(
      await waitFor(() => task.snapshot().lastStep === 'read internal/install/steps/pi_extensions.go'),
      `the announced tool must become the live step; got ${JSON.stringify(task.snapshot().lastStep)}`,
    );
    const snapshot = task.snapshot();
    assert.match(formatTaskResult(task, snapshot), /· read internal\/install\/steps\/pi_extensions\.go$/);
    assert.deepEqual(
      widgetRowsFor([{ agent: 'sdd-explore', state: 'running', elapsedMs: 4200, lastStep: snapshot.lastStep }], { env: {}, width: 200 }),
      ['◐ sdd-explore · read internal/install/steps/pi_extensions.go · 4s'],
    );
    assert.deepEqual(
      widgetRowsFor([{ agent: 'sdd-explore', state: 'running', elapsedMs: 1000, summary: 'Read pi_extensions.go' }], { env: {}, width: 200 }),
      ['◐ sdd-explore · Read pi_extensions.go · 1s'],
      'before the first tool the task summary is the honest label',
    );
  });

  it('formats spend compactly and keeps a finished run visible for a minute', () => {
    assert.equal(formatTokens(940), '940');
    assert.equal(formatTokens(12345), '12.3k');
    assert.equal(formatTokens(1234567), '1.2M');
    assert.equal(formatCost(0), '', 'nothing spent ⇒ no metric');
    assert.equal(formatCost(0.0042), '$0.0042');
    assert.equal(formatCost(0.42), '$0.42');
    assert.deepEqual(
      widgetRowsFor([{ agent: 'a', state: 'running', elapsedMs: 3000, lastStep: 'grep x', tokens: 12345, cost: 0.0042 }], { env: {}, width: 200 }),
      ['◐ a · grep x · 3s · 12.3k tok · $0.0042'],
    );
    const settled = { agent: 'sdd-apply', state: 'completed', elapsedMs: 12000, settledAt: 1000, summary: 'read x' };
    assert.deepEqual(widgetRowsFor([settled], { env: {}, now: 2000, width: 200 }), ['✅ sdd-apply · read x · 12s'], 'a fresh completion stays visible');
    assert.deepEqual(widgetRowsFor([settled], { env: {}, now: 1000 + WIDGET_FINISHED_TTL_MS + 1 }), [], 'and fades after the TTL');
  });

  it('accumulates the child’s spend and freezes the reported time at settle', async () => {
    const task = run({ FAKE_SETTLED: '1', FAKE_USAGE: '1' });
    const result = await task.promise;
    assert.equal(result.state, 'completed');
    assert.equal(result.tokens, 1234, 'assistant usage must accumulate on the snapshot');
    assert.ok(Math.abs(result.cost - 0.0042) < 1e-9);
    assert.equal(result.model, 'fake-model');
    assert.match(formatTaskResult(task), /· 1\.2k tok · \$0\.0042$/);
    const frozen = result.elapsedMs;
    await sleep(250);
    assert.equal(task.snapshot().elapsedMs, frozen, 'a finished run must not report a growing time');
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
