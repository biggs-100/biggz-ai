import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

// Deployed-asset contract of fix-checkpoint-ask-context slice 2: ask-user-choice.ts
// invokes the Go-canonical check via argv array + stdin under a bounded timeout;
// ONLY explicit blocked(...) tokens block; every indeterminate failure (ENOENT,
// timeout, foreign non-zero, stale binary, BIGGZ_ASK_CHECK=0) degrades open with a
// visible notice — a broken check must never make asking impossible (D4/D6).
import askGuard, {
	checkCheckpointAsk,
	recordAskTurnText,
	getAskTurnMarkdown,
	ASK_CHECK_TIMEOUT_MS,
	_setAskCheckExecForTest,
} from './biggz-ask-guard.js';

function createMockPi() {
	const handlers = {};
	return {
		on: (ev, fn) => { (handlers[ev] ||= []).push(fn); },
		_handlers: handlers,
	};
}

function mockExecResult(returnValue) {
	const calls = [];
	const fn = (...args) => { calls.push(args); return returnValue; };
	fn.calls = calls;
	return fn;
}

function mockExecThrow(err) {
	const calls = [];
	const fn = (...args) => { calls.push(args); throw err; };
	fn.calls = calls;
	return fn;
}

function exitError(status, stdout) {
	const err = new Error(`Command failed with exit code ${status}`);
	err.status = status;
	err.stdout = stdout ?? '';
	err.stderr = '';
	return err;
}

function silentWarn(fn) {
	const warnings = [];
	const origWarn = console.warn;
	console.warn = (...a) => { warnings.push(a.join(' ')); };
	try { return { warnings, result: fn() }; } finally { console.warn = origWarn; }
}

describe('ask check invocation (slice 2)', () => {
	const savedEnv = {};

	beforeEach(() => {
		for (const k of ['BIGGZ_ASK_CHECK']) {
			savedEnv[k] = process.env[k];
			delete process.env[k];
		}
		_setAskCheckExecForTest(null);
		recordAskTurnText('', { reset: true });
	});

	afterEach(() => {
		for (const k of ['BIGGZ_ASK_CHECK']) {
			if (savedEnv[k] === undefined) delete process.env[k];
			else process.env[k] = savedEnv[k];
		}
		_setAskCheckExecForTest(null);
		recordAskTurnText('', { reset: true });
	});

	it('invokes `biggz sdd-ask-check` via argv array, no shell, bounded by ASK_CHECK_TIMEOUT_MS', async () => {
		const exec = mockExecResult('');
		_setAskCheckExecForTest(exec);
		recordAskTurnText('## Sub-agent Result: apply\n**Artifacts/Paths:** a.go');
		const params = { question: 'Ship it?', options: [{ label: 'proceed', description: 'a'.repeat(30) }] };
		const verdict = await checkCheckpointAsk(params);
		assert.equal(verdict, undefined, 'exit 0 must allow');
		assert.equal(exec.calls.length, 1);
		const [bin, argv, opts] = exec.calls[0];
		assert.match(bin, /biggz(\.exe)?$/);
		assert.deepEqual(argv, ['sdd-ask-check'], 'must pass verb as argv array');
		assert.ok(!opts?.shell, 'must not use a shell (no injection/quoting)');
		assert.equal(opts?.timeout, ASK_CHECK_TIMEOUT_MS);
		assert.equal(ASK_CHECK_TIMEOUT_MS, 1000);
		const payload = JSON.parse(opts.input);
		assert.equal(payload.question, JSON.stringify(params), 'raw params JSON must travel verbatim');
		assert.equal(payload.markdown, '## Sub-agent Result: apply\n**Artifacts/Paths:** a.go');
	});

	it('injection-shaped payload reaches the child verbatim on stdin', async () => {
		const exec = mockExecResult('');
		_setAskCheckExecForTest(exec);
		const params = {
			question: '"; rm -rf / #$(touch pwned) `id`',
			options: [{ label: 'proceed', description: 'x; $(curl evil) && echo' }],
		};
		const markdown = 'line1\n"; cat /etc/passwd #\n## Sub-agent Result';
		recordAskTurnText(markdown);
		await checkCheckpointAsk(params);
		const payload = JSON.parse(exec.calls[0][2].input);
		assert.equal(payload.question, JSON.stringify(params), 'injection text must not be interpreted or altered');
		assert.equal(payload.markdown, markdown);
	});

	it('exit 3 blocked(checkpoint_option_thin) blocks and surfaces the reason', async () => {
		const exec = mockExecThrow(exitError(3, 'blocked(checkpoint_option_thin): option "Yes, go ahead" (question 1) carries no decision context\n'));
		_setAskCheckExecForTest(exec);
		const verdict = await checkCheckpointAsk({ question: 'x', options: [] });
		assert.ok(verdict && verdict.block === true, `must block, got ${JSON.stringify(verdict)}`);
		assert.match(String(verdict.reason), /blocked\(checkpoint_option_thin\)/);
		assert.match(String(verdict.reason), /Yes, go ahead/);
	});

	it('exit 1 blocked(synthesis_required) blocks with reason', async () => {
		const exec = mockExecThrow(exitError(1, 'blocked(synthesis_required): synthesis required: missing ## Sub-agent Result\n'));
		_setAskCheckExecForTest(exec);
		const verdict = await checkCheckpointAsk({ question: 'x', options: [] });
		assert.equal(verdict?.block, true);
		assert.match(String(verdict.reason), /blocked\(synthesis_required\)/);
	});

	it('exit 2 envelope block names the limit (header 17 → limit 16) and flows through', async () => {
		const exec = mockExecThrow(exitError(2, 'blocked(envelope_invalid): header exceeds limit 16: got 17 for question 0\n'));
		_setAskCheckExecForTest(exec);
		const verdict = await checkCheckpointAsk({ question: 'x'.repeat(17), options: [] });
		assert.equal(verdict?.block, true);
		assert.match(String(verdict.reason), /limit 16/);
	});

	it('pass does not skip envelope limits: thin 1-option envelope is still rejected by the check', async () => {
		const exec = mockExecThrow(exitError(2, 'blocked(envelope_invalid): options out of range: got 1, want 2-4 for question 0\n'));
		_setAskCheckExecForTest(exec);
		const verdict = await checkCheckpointAsk({ question: 'one option', options: [{ label: 'a', description: 'b'.repeat(30) }] });
		assert.equal(verdict?.block, true);
		assert.match(String(verdict.reason), /2-4/);
	});

	it('valid 3-question envelope (check exit 0) allows presentation', async () => {
		const exec = mockExecResult('');
		_setAskCheckExecForTest(exec);
		const verdict = await checkCheckpointAsk({ questions: [{ header: 'Decision' }, { header: 'Second' }, { header: 'Third' }] });
		assert.equal(verdict, undefined);
	});

	describe('degrade-open table (never trap, always visible)', () => {
		const cases = [
			{ name: 'spawn ENOENT', exec: () => mockExecThrow(Object.assign(new Error('spawn biggz ENOENT'), { code: 'ENOENT' })) },
			{ name: 'timeout', exec: () => mockExecThrow(Object.assign(new Error('ETIMEDOUT'), { code: 'ETIMEDOUT', killed: true })) },
			{
				name: 'stale binary exit 1 with help text (no blocked token)',
				exec: () => mockExecThrow(exitError(1, 'Usage: biggz <command>\nbiggz tui requires both stdin and stdout to be terminals\n')),
			},
			{ name: 'foreign non-zero exit 42', exec: () => mockExecThrow(exitError(42, 'some unrelated failure\n')) },
		];
		for (const c of cases) {
			it(`${c.name} → degraded + notice, never block`, async () => {
				_setAskCheckExecForTest(c.exec());
				const { warnings, result } = silentWarn(() => checkCheckpointAsk({ question: 'q', options: [] }));
				const verdict = await result;
				assert.equal(verdict?.block, undefined, 'indeterminate must never block');
				assert.equal(verdict?.degraded, true, `must degrade, got ${JSON.stringify(verdict)}`);
				assert.ok(typeof verdict.notice === 'string' && verdict.notice.length > 0, 'must carry a visible notice');
				assert.ok(warnings.some((w) => /degraded/i.test(w)), 'must warn on stderr path');
			});
		}

		it('BIGGZ_ASK_CHECK=0 skips the check with a warning notice and does not exec', async () => {
			process.env.BIGGZ_ASK_CHECK = '0';
			const exec = mockExecResult('');
			_setAskCheckExecForTest(exec);
			const verdict = await checkCheckpointAsk({ question: 'q', options: [] });
			assert.equal(verdict?.degraded, true);
			assert.match(String(verdict.notice), /skipped/i);
			assert.equal(exec.calls.length, 0, 'escape hatch must not invoke the CLI');
		});

		it('never throws on unexpected exec failure', async () => {
			_setAskCheckExecForTest(() => { throw 'string failure'; });
			const verdict = await checkCheckpointAsk({ question: 'q', options: [] });
			assert.equal(verdict?.degraded, true);
		});
	});

	describe('current-turn buffer', () => {
		it('records message_end / message_update and resets on turn_start / agent_start', () => {
			const pi = createMockPi();
			askGuard(pi);
			assert.ok(pi._handlers.message_end?.length, 'factory must register message_end');
			assert.ok(pi._handlers.message_update?.length, 'factory must register message_update');
			assert.ok(pi._handlers.turn_start?.length, 'factory must register turn_start');
			assert.ok(pi._handlers.agent_start?.length, 'factory must register agent_start');
			pi._handlers.message_end[0]({ message: { role: 'assistant', content: '## Sub-agent Result: apply\n**Artifacts/Paths:** a.go' } });
			assert.match(getAskTurnMarkdown(), /## Sub-agent Result/, 'message_end must record turn text');
			pi._handlers.message_update[0]({ delta: { text: '| Topic | Decision |' } });
			assert.match(getAskTurnMarkdown(), /Topic/, 'message_update must feed the buffer');
			pi._handlers.turn_start[0]();
			assert.equal(getAskTurnMarkdown(), '', 'turn_start must reset the buffer');
			pi._handlers.message_end[0]({ message: { role: 'assistant', content: 'synthesis v2' } });
			pi._handlers.agent_start[0]();
			assert.equal(getAskTurnMarkdown(), '', 'agent_start must reset the buffer');
		});

		it('factory survives hostile/absent pi without throwing', () => {
			assert.doesNotThrow(() => askGuard(undefined));
			assert.doesNotThrow(() => askGuard({ on: () => { throw new Error('nope'); } }));
		});
	});
});

describe('ask-user-choice.ts wiring is live (source-scan)', () => {
	const ts = readFileSync(new URL('./ask-user-choice.ts', import.meta.url), 'utf8');

	it('statically imports checkCheckpointAsk from the shared guard module', () => {
		assert.match(
			ts,
			/import\s*\{[^}]*checkCheckpointAsk[^}]*\}\s*from\s*["']\.\/biggz-ask-guard\.js["']/,
			'ask-user-choice.ts must import checkCheckpointAsk from ./biggz-ask-guard.js',
		);
	});

	it('calls the check BEFORE rendering any UI', () => {
		const callIdx = ts.indexOf('checkCheckpointAsk(');
		const uiIdx = ts.indexOf('ctx.ui.custom');
		assert.ok(callIdx > -1, 'must call checkCheckpointAsk');
		assert.ok(uiIdx > -1, 'sanity: asset renders via ctx.ui.custom');
		assert.ok(callIdx < uiIdx, 'check must run before ctx.ui.custom (a rule nobody invokes is not a rule)');
	});

	it('refuses to present on a decided block (isError:true, reason surfaced)', () => {
		assert.match(ts, /verdict\?\.block/);
		assert.match(ts, /isError:\s*true/);
		assert.match(ts, /verdict\.reason/);
		assert.match(ts, /verdict\?\.degraded/);
		assert.match(ts, /ctx\.ui\.notify\(\s*verdict\.notice/);
		assert.match(ts, /"warning"/);
	});
});
