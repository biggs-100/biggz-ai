import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Mirrors internal/install/steps/pi_extensions.go deploy list.
// 11 JS extensions + 3 TS (skill-registry family). TS are not pi ExtensionAPI factories.
const DEPLOY_LIST = [
  { asset: 'pi/biggz-thinking-wrap.js', target: 'biggz-thinking-wrap.js' },
  { asset: 'pi/biggz-memory-chrome.js', target: 'biggz-memory-chrome.js' },
  { asset: 'pi/biggz-tool-interception.js', target: 'biggz-tool-interception.js' },
  { asset: 'pi/biggz-extension-api.js', target: 'biggz-extension-api.js' },
  { asset: 'pi/biggz-session-guard.js', target: 'biggz-session-guard.js' },
  { asset: 'pi/biggz-last-model.js', target: 'biggz-last-model.js' },
  { asset: 'pi/biggz-wait-pretty.js', target: 'biggz-wait-pretty.js' },
  { asset: 'pi/biggz-footer.js', target: 'biggz-footer.js' },
  { asset: 'pi/biggz-tool-pills.js', target: 'biggz-tool-pills.js' },
  { asset: 'pi/biggz-web-search.js', target: 'biggz-web-search.js' },
  { asset: 'pi/biggz-question-mouse.js', target: 'biggz-question-mouse.js' },
  // TS — skill-registry family, not pi extension factories (no factory check)
  { asset: 'pi/ask-user-choice.ts', target: 'ask-user-choice.ts' },
  { asset: 'pi/codegraph-tools.ts', target: 'codegraph-tools.ts' },
  { asset: 'pi/skill-registry.ts', target: 'skill-registry.ts' },
];

// pi loader expects every .js in ~/.pi/agent/extensions to export a valid factory.
// Regression guard for biggz-session-guard.js crash: "Extension does not export a valid factory function".
const FACTORY_RE = /export\s+default\s+function/;

describe('pi extensions must export valid factory', () => {
  for (const entry of DEPLOY_LIST) {
    if (!entry.target.endsWith('.js')) continue;
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

  it('deploy list covers all expected js extensions', () => {
    const jsCount = DEPLOY_LIST.filter((e) => e.target.endsWith('.js')).length;
    // Guard against deploy list drift: if pi_extensions.go adds a new JS, this test must be updated.
    assert.equal(jsCount, 11, `expected 11 JS extensions in deploy list, got ${jsCount} — sync with pi_extensions.go`);
  });
});
