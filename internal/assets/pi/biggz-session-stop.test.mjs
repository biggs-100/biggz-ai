import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';

// Tests pin the Cut 2 contract: pending-first ordering, CLI exit mapping,
// degrade-without-trapping, argv-array (no shell), and both-file parity.
import {
	checkSessionStop,
	SESSION_STOP_TIMEOUT_MS,
	_setSessionStopExecForTest,
} from './biggz-session-guard.js';
import toolInterception from './biggz-tool-interception.js';
import extensionAPI from './biggz-extension-api.js';

function createMockPi() {
	const handlers = {};
	return {
		on: (ev, fn) => { handlers[ev] = fn; },
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

function exitError(status, stderr) {
	const err = new Error(`Command failed with exit code ${status}`);
	err.status = status;
	err.stderr = stderr;
	err.stdout = '';
	return err;
}

describe('session_stop summary guard (Cut 2)', () => {
	const savedEnv = {};

	beforeEach(() => {
		for (const k of ['BIGGZ_PENDING_FINDINGS', 'BIGGZ_PENDING_LENSES', 'BIGGZ_PRETTY', 'PI_SUBAGENT_CHILD']) {
			savedEnv[k] = process.env[k];
			delete process.env[k];
		}
		_setSessionStopExecForTest(null);
	});

	afterEach(() => {
		for (const k of ['BIGGZ_PENDING_FINDINGS', 'BIGGZ_PENDING_LENSES', 'BIGGZ_PRETTY', 'PI_SUBAGENT_CHILD']) {
			if (savedEnv[k] === undefined) delete process.env[k];
			else process.env[k] = savedEnv[k];
		}
		_setSessionStopExecForTest(null);
	});

	it('Q2: timeout covers measured biggz cold-start (~280ms)', () => {
		// Measured 2026-09-07: session-close --check-only cold-start 274-289ms
		// on win32. 250ms would flap into degrade on the happy path, losing
		// enforcement. 1000ms keeps close bounded while giving ~3x headroom.
		assert.equal(SESSION_STOP_TIMEOUT_MS, 1000);
	});

	it('verified summary (CLI exit 0) allows close', async () => {
		const exec = mockExecResult('');
		_setSessionStopExecForTest(exec);
		const verdict = await checkSessionStop();
		assert.equal(verdict, undefined);
		assert.equal(exec.calls.length, 1);
		const [file, argv, opts] = exec.calls[0];
		assert.match(file, /biggz(\.exe)?$/);
		assert.ok(Array.isArray(argv), 'must pass argv array (no shell)');
		assert.ok(argv.includes('session-close') && argv.includes('--check-only'));
		assert.ok(!opts?.shell, 'must not use shell (no injection)');
		assert.equal(opts?.timeout, SESSION_STOP_TIMEOUT_MS);
	});

	it('missing summary (CLI exit 1) blocks with reason', async () => {
		const exec = mockExecThrow(exitError(1, 'blocked(session_summary_missing): session_summary required before done\n'));
		_setSessionStopExecForTest(exec);
		const verdict = await checkSessionStop();
		assert.ok(verdict && verdict.block === true, `must block, got ${JSON.stringify(verdict)}`);
		assert.match(String(verdict.reason), /blocked\(session_summary_missing\)/);
	});

	it('stale binary (exit 1 without gate token) degrades, never traps', async () => {
		// E2E find 2026-09-07: a biggz without the session-close verb exits 1
		// with help text. Blocking on that would trap every close, so only
		// the blocked(session_summary_missing) token blocks.
		const exec = mockExecThrow(exitError(1, 'biggz tui requires both stdin and stdout to be terminals\nUsage: biggz <command>'));
		_setSessionStopExecForTest(exec);
		const warnings = [];
		const origWarn = console.warn;
		console.warn = (...a) => { warnings.push(a.join(' ')); };
		try {
			const verdict = await checkSessionStop();
			assert.equal(verdict, undefined, 'stale binary must allow, never trap');
		} finally {
			console.warn = origWarn;
		}
		assert.ok(warnings.some((w) => /degraded/i.test(w)));
	});

	it('pending findings block first without invoking CLI', async () => {
		process.env.BIGGZ_PENDING_FINDINGS = '2';
		const exec = mockExecResult('');
		_setSessionStopExecForTest(exec);
		const verdict = await checkSessionStop();
		assert.ok(verdict && verdict.block === true);
		assert.match(String(verdict.reason), /pending work/);
		assert.equal(exec.calls.length, 0, 'CLI must not run when pending work blocks first');
	});

	it('pending lenses block first without invoking CLI', async () => {
		process.env.BIGGZ_PENDING_LENSES = '1';
		const exec = mockExecResult('');
		_setSessionStopExecForTest(exec);
		const verdict = await checkSessionStop();
		assert.ok(verdict && verdict.block === true);
		assert.equal(exec.calls.length, 0);
	});

	it('CLI timeout degrades to allow with warning (never traps)', async () => {
		const timeoutErr = new Error('ETIMEDOUT');
		timeoutErr.code = 'ETIMEDOUT';
		const exec = mockExecThrow(timeoutErr);
		_setSessionStopExecForTest(exec);
		const warnings = [];
		const origWarn = console.warn;
		console.warn = (...a) => { warnings.push(a.join(' ')); };
		try {
			const verdict = await checkSessionStop();
			assert.equal(verdict, undefined, 'timeout must allow, never trap');
		} finally {
			console.warn = origWarn;
		}
		assert.ok(warnings.some((w) => /degraded/i.test(w)), `must warn degraded, got ${JSON.stringify(warnings)}`);
	});

	it('CLI crash (ENOENT) degrades to allow with warning', async () => {
		const crash = new Error('spawn biggz ENOENT');
		crash.code = 'ENOENT';
		const exec = mockExecThrow(crash);
		_setSessionStopExecForTest(exec);
		const warnings = [];
		const origWarn = console.warn;
		console.warn = (...a) => { warnings.push(a.join(' ')); };
		try {
			const verdict = await checkSessionStop();
			assert.equal(verdict, undefined);
		} finally {
			console.warn = origWarn;
		}
		assert.ok(warnings.some((w) => /degraded/i.test(w)));
	});

	it('never throws on unexpected exec failure', async () => {
		_setSessionStopExecForTest(() => { throw 'string failure'; });
		const verdict = await checkSessionStop();
		assert.equal(verdict, undefined);
	});

	it('both files return identical verdicts (single guard, one delegation)', async () => {
		const pi1 = createMockPi();
		const pi2 = createMockPi();
		toolInterception(pi1);
		extensionAPI(pi2);
		const stop1 = pi1._handlers['session_stop'];
		const stop2 = pi2._handlers['session_stop'];
		assert.ok(stop1, 'tool-interception must register session_stop');
		assert.ok(stop2, 'extension-api must register session_stop');

		const cases = [
			{ name: 'allow', exec: mockExecResult(''), env: {} },
			{ name: 'block', exec: mockExecThrow(exitError(1, 'blocked(session_summary_missing)')), env: {} },
			{ name: 'pending', exec: mockExecResult(''), env: { BIGGZ_PENDING_FINDINGS: '1' } },
			{ name: 'timeout-degrade', exec: mockExecThrow(Object.assign(new Error('ETIMEDOUT'), { code: 'ETIMEDOUT' })), env: {} },
		];
		for (const c of cases) {
			delete process.env.BIGGZ_PENDING_FINDINGS;
			delete process.env.BIGGZ_PENDING_LENSES;
			Object.assign(process.env, c.env);
			_setSessionStopExecForTest(c.exec);
			const [v1, v2] = await Promise.all([stop1(), stop2()]);
			assert.deepEqual(v2, v1, `${c.name}: extension-api must delegate, not diverge`);
		}
	});
});
