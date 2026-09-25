import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Mirrors internal/install/steps/pi_extensions.go deploy list.
// This mirror deliberately covers the FACTORY entries only: pi loads every direct
// extensions/*.js|*.ts as an extension and crashes on one that does not export
// `export default function(pi)`. That includes the three TypeScript factories
// (ask-user-choice.ts, codegraph-tools.ts, skill-registry.ts) an earlier
// extension-suffix guard silently skipped, so this guard checks all 16. The
// sixteen `pi/biggz-shell/*.ts` helpers are factory:false and are NOT listed
// here: they are covered by the Go layout guard (validatePiExtensionsFactory/
// Layout, which require every helper to sit under biggz-shell/ and forbid a
// factory export) plus their own co-located `biggz-shell/*.test.mjs` suites. Do
// not add fake entries to pad a count. Neither mirror is a manifest of that
// helper set, so a helper rename or drop is caught by nothing today (task 2.12).
const DEPLOY_LIST = [
  { asset: 'pi/biggz-thinking-wrap.js', target: 'biggz-thinking-wrap.js', factory: true },
  { asset: 'pi/biggz-memory-chrome.js', target: 'biggz-memory-chrome.js', factory: true },
  { asset: 'pi/biggz-tool-interception.js', target: 'biggz-tool-interception.js', factory: true },
  { asset: 'pi/biggz-extension-api.js', target: 'biggz-extension-api.js', factory: true },
  { asset: 'pi/biggz-session-guard.js', target: 'biggz-session-guard.js', factory: true },
  { asset: 'pi/biggz-ask-guard.js', target: 'biggz-ask-guard.js', factory: true },
  { asset: 'pi/biggz-last-model.js', target: 'biggz-last-model.js', factory: true },
  { asset: 'pi/biggz-subagent-runtime.js', target: 'biggz-subagent-runtime.js', factory: true },
  { asset: 'pi/biggz-shell.ts', target: 'biggz-shell.ts', factory: true },
  { asset: 'pi/biggz-tool-pills.js', target: 'biggz-tool-pills.js', factory: true },
  { asset: 'pi/biggz-web-search.js', target: 'biggz-web-search.js', factory: true },
  { asset: 'pi/biggz-question-mouse.js', target: 'biggz-question-mouse.js', factory: true },
  { asset: 'pi/biggz-quiet-tools.js', target: 'biggz-quiet-tools.js', factory: true },
  { asset: 'pi/ask-user-choice.ts', target: 'ask-user-choice.ts', factory: true },
  { asset: 'pi/codegraph-tools.ts', target: 'codegraph-tools.ts', factory: true },
  { asset: 'pi/skill-registry.ts', target: 'skill-registry.ts', factory: true },
];

// pi loader expects every direct extensions/*.js|*.ts to export a valid factory.
// Regression guard for biggz-session-guard.js crash: "Extension does not export a valid factory function".
const FACTORY_RE = /export\s+default\s+function/;

describe('pi extensions must export valid factory', () => {
  for (const entry of DEPLOY_LIST) {
    if (!entry.factory) continue;
    it(`${entry.asset} exports default factory`, () => {
      const abs = path.join(__dirname, path.basename(entry.asset));
      let content;
      try {
        content = readFileSync(abs, 'utf8');
      } catch (e) {
        assert.fail(`missing pi extension asset ${entry.asset} at ${abs}: ${e.message}`);
      }
      assert.ok(
        FACTORY_RE.test(content),
        `pi extension ${entry.asset} must contain 'export default function' (pi requires valid factory) — got first 200 chars: ${content.slice(0, 200).replace(/\n/g, '\\n')}`,
      );
      // Extra strictness: ensure the factory takes pi param (common shape)
      assert.ok(
        /export\s+default\s+function\s+\w*\s*\(\s*pi/.test(content),
        `pi extension ${entry.asset} factory should accept pi param: export default function <name>(pi)`,
      );
    });
  }

  it('deploy list covers all expected pi extension factories', () => {
    const factoryCount = DEPLOY_LIST.filter((e) => e.factory).length;
    // Guard against deploy list drift: if pi_extensions.go adds a factory, this test must be updated.
    // 16, not 13: the three TS factories were deployed and unchecked before this change, so the guard is stricter, not looser.
    assert.equal(
      factoryCount,
      16,
      `expected 16 pi extension factories in deploy list, got ${factoryCount} — the three TS factories were deployed and unchecked before this change, so the guard is stricter, not looser; sync with pi_extensions.go`,
    );
  });
});
