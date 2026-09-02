import { describe, it, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
const url = pathToFileURL(path.resolve('internal/assets/pi/biggz-tool-pills.js')).href;
let mod; beforeEach(async()=>{mod=await import(url)}); afterEach(()=>{for(const k of["BIGGZ_PRETTY","BIGGZ_NO_ANIMATION","GENTLE_AI_NO_ANIMATION","TERM","PI_SUBAGENT_CHILD"])delete process.env[k]});
function strip(s){return String(s).replace(/\x1b\[[0-9;]*[A-Za-z]/g,"").replace(/\x1b\][^\x07]*\x07/g,"")}
describe('pills PR2',()=>{
  it('collapse 5->3+2 hidden order',async()=>{
    const{pillCollapse, collapsePills, renderPills}=mod; // collapsePills may be named collapsePills
    const c = mod.collapsePills || mod.collapsePills; assert.ok(typeof mod.collapsePills==='function');
    const pills=[{label:'a'},{label:'b'},{label:'c'},{label:'d'},{label:'e'}];
    const{visible,hidden,suffix}=mod.collapsePills(pills,3);
    assert.equal(visible.length,3); assert.deepEqual(visible.map(p=>p.label),['a','b','c']); assert.equal(hidden,2); assert.equal(suffix,'… +2 hidden');
    process.env.BIGGZ_PRETTY=''; process.env.TERM='xterm-256color';
    const rendered=mod.renderPills(pills,3); const plain=strip(rendered);
    assert.ok(plain.includes('a')&&plain.includes('b')&&plain.includes('c')); assert.ok(plain.includes('… +2 hidden')); assert.ok(plain.indexOf('a')<plain.indexOf('b'));
  });
  it('spinner frozen on NO_ANIMATION',async()=>{
    process.env.BIGGZ_PRETTY=''; process.env.TERM='xterm-256color'; process.env.BIGGZ_NO_ANIMATION='1';
    assert.equal(mod.isAnimationEnabled(),false);
    const r=mod.ansiPill('READ',{state:'running'}); assert.ok(strip(r).includes('·')); assert.equal(strip(r),strip(mod.ansiPill('READ',{state:'running'})));
    delete process.env.BIGGZ_NO_ANIMATION; assert.equal(mod.isAnimationEnabled(),true);
    assert.ok(mod.ansiPill('READ',{state:'running'}).includes('⠋'));
  });
  it('pretty off plain',async()=>{
    process.env.BIGGZ_PRETTY='0'; process.env.TERM='xterm-256color';
    const p=mod.ansiPill('READ',{bg:'toolPendingBg',fg:'accent'}); assert.equal(p,'READ'); assert.ok(!p.includes('\x1b['));
    const rendered=mod.renderPills([{label:'READ',state:'running'},{label:'WRITE',state:'complete'}]); assert.ok(!rendered.includes('\x1b['));
    const collapsed=mod.collapseOutput('a\nb\nc\nd\ne',3); assert.ok(!collapsed.includes('\x1b[')); assert.ok(collapsed.includes('… +2 hidden'));
  });
  it('dumb strips ANSI',async()=>{
    process.env.BIGGZ_PRETTY=''; process.env.TERM='dumb';
    const p=mod.ansiPill('READ',{state:'running'}); assert.equal(p,'READ'); assert.ok(!p.includes('\x1b['));
    const rendered=mod.renderPills([{label:'a'},{label:'b'},{label:'c'},{label:'d'}]); assert.ok(!rendered.includes('\x1b[')); assert.ok(rendered.includes('… +1 hidden'));
  });
  it('maps and gentle compat',async()=>{
    assert.ok(mod.TOOL_PILL_MAP.read); assert.equal(mod.getToolPill('read').label,'READ');
    assert.ok(mod.PILL_STATE_STYLES.running); assert.ok(mod.PILL_STATE_STYLES.queued); assert.ok(Array.isArray(mod.SPINNER_FRAMES));
    process.env.BIGGZ_PRETTY=''; process.env.TERM='xterm-256color'; process.env.GENTLE_AI_NO_ANIMATION='1';
    assert.equal(mod.isAnimationEnabled(),false); assert.ok(strip(mod.ansiPill('READ',{state:'running'})).includes('·'));
    const code='function foo(){return "bar"}'; process.env.BIGGZ_PRETTY=''; process.env.TERM='xterm-256color'; delete process.env.GENTLE_AI_NO_ANIMATION;
    assert.ok(mod.syntaxHighlight(code).includes('\x1b[')); process.env.BIGGZ_PRETTY='0'; assert.ok(!mod.syntaxHighlight(code).includes('\x1b['));
  });
});
