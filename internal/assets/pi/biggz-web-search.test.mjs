import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const mockPi = { registerTool: () => {}, registerCommand: () => {}, on: () => {} };
const mod = await import(pathToFileURL(path.resolve('internal/assets/pi/biggz-web-search.js')).href);
mod.default(mockPi);
const { extractWithAnchors, truncateWithAnchor, htmlToMarkdown } = mockPi._biggzWebSearch;

describe('extractWithAnchors anchor-preserving', () => {
  it('preserves anchors h2 and h3 in order with hierarchy', () => {
    const html = `<html><body><h2 id="install">Install</h2><p>foo</p><h3 id="usage">Usage</h3><p>bar</p></body></html>`;
    const { markdown, anchors } = extractWithAnchors(html);
    assert.ok(markdown.includes('## Install {#install}'), `expected ## Install {#install} got ${markdown}`);
    assert.ok(markdown.includes('### Usage {#usage}'), `expected ### Usage {#usage} got ${markdown}`);
    assert.ok(markdown.indexOf('{#install}') < markdown.indexOf('{#usage}'), 'order must be preserved');
    assert.deepEqual(anchors, ['install', 'usage']);
    // htmlToMarkdown delegates and preserves same
    const md2 = htmlToMarkdown(html);
    assert.ok(md2.includes('## Install {#install}'));
    assert.ok(md2.includes('### Usage {#usage}'));
  });

  it('resolves relative /href via baseUrl', () => {
    const html = `<p>See <a href="/guide">guide</a></p>`;
    const { markdown } = extractWithAnchors(html, 'https://example.com/docs');
    assert.ok(markdown.includes('[guide](https://example.com/guide)'), `expected resolved link got ${markdown}`);
  });

  it('truncate annotates with nearest preceding anchor', () => {
    let big = '';
    for (let i = 0; i < 8000; i++) {
      big += `<h2 id="sec${i}">Section ${i}</h2><p>${'x'.repeat(500)}</p>\n`;
    }
    const { markdown, anchors } = extractWithAnchors(big);
    assert.ok(Buffer.byteLength(markdown, 'utf8') > 1024 * 1024, 'big markdown should exceed 1MB');
    const t = truncateWithAnchor(markdown, anchors);
    assert.equal(t.truncated, true);
    assert.ok(t.nearest, 'nearest anchor should be found');
    assert.ok(t.annotation.includes(`{#${t.nearest}}`), `annotation must contain nearest anchor, got ${t.annotation}`);
    assert.ok(t.annotation.includes('truncated'), 'annotation must contain truncated');
    assert.ok(t.annotation.includes('1MB'), 'annotation must contain 1MB');
    assert.ok(t.annotation.includes('offset at'), 'annotation must contain offset at');
    assert.ok(t.markdown.includes(`{#${t.nearest}}`), 'truncated markdown must contain nearest anchor before cut');
    assert.ok(Buffer.byteLength(t.markdown, 'utf8') <= 1024 * 1024, 'truncated markdown must be capped at 1MB');
    // full pipeline via extractWithAnchors + truncate should produce same annotation when used in web_fetch
    const finalMarkdown = t.truncated ? t.markdown + t.annotation : t.markdown;
    assert.ok(finalMarkdown.includes(`{#${t.nearest}}`));
  });

  it('does not throw on malformed HTML and preserves at least one anchor', () => {
    const malformed = `<div><h2 id="a">A<h2 id="b">B</h3><p>unclosed<div>`;
    assert.doesNotThrow(() => extractWithAnchors(malformed));
    const { markdown } = extractWithAnchors(malformed);
    // at least one anchor preserved
    assert.ok(markdown.includes('{#a}') || markdown.includes('{#b}'), `expected at least one anchor in malformed result got ${markdown}`);
  });

  it('handles duplicate ids and preserves them', () => {
    const html = `<h2 id="dup">First</h2><h2 id="dup">Second</h2>`;
    const { markdown, anchors } = extractWithAnchors(html);
    assert.ok(markdown.includes('## First {#dup}'));
    assert.ok(markdown.includes('## Second {#dup}'));
    assert.deepEqual(anchors, ['dup', 'dup']);
  });

  it('preserves hierarchy h1..h6 and order for mixed levels', () => {
    const html = `<h1 id="a">A</h1><h2 id="b">B</h2><h3 id="c">C</h3><h2 id="d">D</h2>`;
    const { markdown, anchors } = extractWithAnchors(html);
    assert.ok(markdown.includes('# A {#a}'));
    assert.ok(markdown.includes('## B {#b}'));
    assert.ok(markdown.includes('### C {#c}'));
    assert.ok(markdown.includes('## D {#d}'));
    assert.deepEqual(anchors, ['a', 'b', 'c', 'd']);
    assert.ok(markdown.indexOf('{#a}') < markdown.indexOf('{#b}'));
    assert.ok(markdown.indexOf('{#b}') < markdown.indexOf('{#c}'));
    assert.ok(markdown.indexOf('{#c}') < markdown.indexOf('{#d}'));
  });

  it('htmlToMarkdown and extractWithAnchors share same path (parity)', () => {
    const html = `<h2 id="install">Install</h2><p>Text <a href="/guide">guide</a></p>`;
    const a = htmlToMarkdown(html, 'https://example.com');
    const b = extractWithAnchors(html, 'https://example.com').markdown;
    assert.equal(a, b);
  });

  it('span inside heading handled', () => {
    const html = `<h2 id="install"><span>Install</span></h2>`;
    const { markdown } = extractWithAnchors(html);
    assert.ok(markdown.includes('## Install {#install}'));
  });

  it('headings without id emit without anchor', () => {
    const html = `<h2>Title</h2>`;
    const { markdown, anchors } = extractWithAnchors(html);
    assert.ok(markdown.includes('## Title'));
    assert.ok(!markdown.includes('{#'));
    assert.deepEqual(anchors, []);
  });
});
