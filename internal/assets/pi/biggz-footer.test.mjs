import {describe,it,beforeEach,afterEach} from 'node:test';import assert from 'node:assert/strict';import path from 'node:path';import{pathToFileURL}from 'node:url';
const strip=s=>String(s).replace(/\x1b\[[0-9;]*[A-Za-z]/g,"").replace(/\x1b\][^\x07]*\x07/g,"");
const footerUrl=pathToFileURL(path.resolve('internal/assets/pi/biggz-footer.js')).href;
const extUrl=pathToFileURL(path.resolve('internal/assets/pi/biggz-extension-api.js')).href;
const pillsUrl=pathToFileURL(path.resolve('internal/assets/pi/biggz-tool-pills.js')).href;
// ── helpers ───────────────────────────────────────────────────────────────────
const WIDTHS=[120,100,80,60,40,30,20];
const FIXTURE_BRANCH='chore/agent-lesson-guards'; // 25c crash fixture
const FIXTURE_MODEL_ID='muse-spark-1.3-contributor';
const FIXTURE_PROVIDER='opencode-go';
const FIXTURE_THINKING='xhigh';
const LONG_BRANCH_60='feature/'+'a'.repeat(52); // 8+52=60
const LONG_MODEL_80='a'.repeat(80);
const LONG_PATH_100='/home/user/projects/'+'a'.repeat(80); // 20+80=100
const LONG_BRANCH_60_RAW='b'.repeat(60);
const themeColored={fg:(_,t)=>`\x1b[38;5;1m${t}\x1b[39m`,symbolPreset:"unicode"};
const themePlain={fg:(_,t)=>t,symbolPreset:"unicode"};
function legacyCtx({branch=FIXTURE_BRANCH,cwd='/home/user/projects/biggz-ai/with/very/long/path/that/exceeds/forty/chars/project',modelId=FIXTURE_MODEL_ID,provider=FIXTURE_PROVIDER}={}){
  return {branch,cwd,model:{id:modelId,provider},getContextUsage:()=>({percent:42,contextWindow:200000}),usage:{input:18000,output:266,cost:{total:0.42}}};
}
function newCtx({branch=FIXTURE_BRANCH,change=FIXTURE_BRANCH,lineage=2,lens='1/4',budget='1/1'}={}){
  return {branch,change,lineage,lens,budget};
}
describe('footer PR3',()=>{
let footer,ext,pills;
beforeEach(async()=>{footer=await import(`${footerUrl}?r=${Date.now()}${Math.random()}`);ext=await import(`${extUrl}?r=${Date.now()}${Math.random()}`);pills=await import(`${pillsUrl}?r=${Date.now()}${Math.random()}`);});
afterEach(()=>{for(const k of["BIGGZ_PRETTY","BIGGZ_NO_ANIMATION","GENTLE_AI_NO_ANIMATION","TERM","PI_SUBAGENT_CHILD","BIGGZ_NERDFONT"])delete process.env[k];try{ext._resetPillThrottleForTest?.();pills._resetPillThrottleForTest?.();}catch{}});
it('order branch▕change▕lineage▕lens▕budget',()=>{process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';const theme={fg:(_,t)=>t,symbolPreset:"unicode"};const ctx={branch:"main",change:"ui-pi-pretty-v2",lineage:2,lens:"1/4",budget:"1/1",hasNerdFont:false};const segs=footer.buildFooterSegments(theme,{},ctx,null);assert.equal(segs.raw.branch,"main");assert.equal(segs.raw.change,"ui-pi-pretty-v2");assert.equal(segs.raw.lineage,"lineage 2");assert.equal(segs.raw.lens,"lens 1/4");assert.equal(segs.raw.budget,"budget 1/1");const line=strip(footer.renderFooterLine(200,theme,segs,ctx));const a=line.indexOf("main"),b=line.indexOf("ui-pi-pretty-v2"),c=line.indexOf("lineage 2"),d=line.indexOf("lens 1/4"),e=line.indexOf("budget 1/1");assert.ok(a>=0&&b>=0&&c>=0&&d>=0&&e>=0);assert.ok(a<b&&b<c&&c<d&&d<e);assert.ok(line.includes("▕")||line.includes("/"));});
it('nerd fallback ▕/ zero nerd glyphs',()=>{process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';process.env.BIGGZ_NERDFONT='0';const theme={fg:(_,t)=>t,symbolPreset:"unicode"};const ctx={branch:"main",change:"c",lineage:1,lens:"1/4",budget:"1/1",hasNerdFont:false};const segs=footer.buildFooterSegments(theme,{},ctx,null);const line=strip(footer.renderFooterLine(200,theme,segs,ctx));assert.ok(line.includes("▕"));assert.equal(line.includes("\uE0B0"),false);assert.equal(line.includes("\uE0B1"),false);assert.equal(ext.getSeparator("powerline-thin",{hasNerdFont:false}).left.includes("\uE0B0"),false);process.env.BIGGZ_NERDFONT='1';assert.ok(["›","\uE0B1","\uE0B0"].includes(ext.getSeparator("powerline-thin",{hasNerdFont:true}).left));});
it('kill-switch BIGGZ_PRETTY=0 no injection',async()=>{process.env.BIGGZ_PRETTY='0';const m1=await import(`${footerUrl}?a=${Date.now()}${Math.random()}`);const pi={on:()=>{}};m1.default(pi);assert.equal(pi._biggzFooter,undefined);const m2=await import(`${extUrl}?b=${Date.now()}${Math.random()}`);const pi2={on:()=>{}};m2.default(pi2);assert.equal(pi2._biggzExtension,undefined);});
it('TERM=dumb strips ANSI ascii only',()=>{process.env.BIGGZ_PRETTY='1';process.env.TERM='dumb';const theme={fg:(t,v)=>`\x1b[38;5;1m${v}\x1b[39m`,symbolPreset:"unicode"};const ctx={branch:"main",change:"ui-pi-pretty-v2",lineage:2,lens:"1/4",budget:"1/1"};const segs=footer.buildFooterSegments(theme,{},ctx,null);const line=footer.renderFooterLine(200,theme,segs,ctx);assert.equal(line.includes("\x1b["),false);assert.ok(strip(line).includes("▕")||strip(line).includes("/"));assert.equal(line.includes("\uE0B0"),false);assert.equal(line.includes("›"),false);});
it('PI_SUBAGENT_CHILD bypass + throttle coalesce',async()=>{process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';process.env.PI_SUBAGENT_CHILD='1';let f=null;ext.schedulePillUpdate([{label:"a"}],p=>{f=p});assert.equal(f,null);assert.equal(ext.isPillThrottled(),false);delete process.env.PI_SUBAGENT_CHILD;ext._resetPillThrottleForTest();let c=0,last=null;const fn=p=>{c++;last=p;};ext.schedulePillUpdate([{label:"a"}],fn);ext.schedulePillUpdate([{label:"a"},{label:"b"}],fn);assert.equal(c,0);assert.equal(ext.isPillThrottled(),true);await new Promise(r=>setTimeout(r,30));assert.equal(c,1);assert.deepEqual(last,[{label:"a"},{label:"b"}]);});
it('collapsible preserves order 4->3+1 hidden + registry guards',()=>{process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';const arr=[{label:"a"},{label:"b"},{label:"c"},{label:"d"}];const {visible,hidden,suffix}=ext.collapsePillsForStream(arr,3);assert.deepEqual(visible.map(p=>p.label),["a","b","c"]);assert.equal(hidden,1);assert.equal(suffix,"… +1 hidden");const r=ext.renderPillsForStream(arr,3);assert.ok(r.includes("a")&&r.includes("b")&&r.includes("c"));assert.ok(r.includes("… +1 hidden"));assert.ok(strip(pills.renderPills(arr,3)).includes("… +1 hidden"));assert.ok(ext.SEPARATORS["powerline-thin"]);assert.ok(typeof ext.isPrettyEnabled==="function");process.env.BIGGZ_PRETTY='0';assert.equal(ext.isSyncSupported(),false);});
});

// ── regression: 121>120 never exceed width ───────────────────────────────────
describe('regression: 121>120 crash fixture widths',()=>{
let footer;
beforeEach(async()=>{footer=await import(`${footerUrl}?crash=${Date.now()}${Math.random()}`);});
afterEach(()=>{for(const k of["BIGGZ_PRETTY","TERM","PI_SUBAGENT_CHILD","BIGGZ_NERDFONT"])delete process.env[k];});
for(const width of WIDTHS){
it(`legacy fixture branch 25c + model xhigh at width ${width} never exceeds ${width}`,()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
const ctx=legacyCtx();
const pi={getThinkingLevel:()=>FIXTURE_THINKING};
const segs=footer.buildFooterSegments(themeColored,{},ctx,pi);
const line=footer.renderFooterLine(width,themeColored,segs,ctx);
const vw=footer.visibleWidth(line);
assert.ok(vw<=width,`legacy fixture vw=${vw} > width=${width} line=${JSON.stringify(strip(line).slice(0,120))}`);
const line2=footer.renderFooter(width,themeColored,{},ctx,pi)[0];
assert.ok(footer.visibleWidth(line2)<=width,`renderFooter vw=${footer.visibleWidth(line2)} > ${width}`);
});
it(`new contract branch/change/lineage/lens/budget at width ${width} never exceeds ${width}`,()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
const ctx=newCtx();
const segs=footer.buildFooterSegments(themeColored,{},ctx,null);
const line=footer.renderFooterLine(width,themeColored,segs,ctx);
const vw=footer.visibleWidth(line);
assert.ok(vw<=width,`new contract vw=${vw} > width=${width} line=${JSON.stringify(strip(line).slice(0,120))}`);
const line2=footer.renderFooter(width,themeColored,{},ctx,null)[0];
assert.ok(footer.visibleWidth(line2)<=width,`renderFooter new vw=${footer.visibleWidth(line2)} > ${width}`);
});
}
it('legacy fixture plain theme also never exceeds at all widths',()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
for(const w of WIDTHS){
const ctx=legacyCtx();
const pi={getThinkingLevel:()=>FIXTURE_THINKING};
const segs=footer.buildFooterSegments(themePlain,{},ctx,pi);
const line=footer.renderFooterLine(w,themePlain,segs,ctx);
assert.ok(footer.visibleWidth(line)<=w,`plain legacy w=${w} vw=${footer.visibleWidth(line)}`);
}
});
it('new contract plain theme also never exceeds at all widths',()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
for(const w of WIDTHS){
const ctx=newCtx();
const segs=footer.buildFooterSegments(themePlain,{},ctx,null);
const line=footer.renderFooterLine(w,themePlain,segs,ctx);
assert.ok(footer.visibleWidth(line)<=w,`plain new w=${w} vw=${footer.visibleWidth(line)}`);
}
});
it('renderFooter dumb term also never exceeds',()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='dumb';
for(const w of WIDTHS){
const ctx=legacyCtx();
const pi={getThinkingLevel:()=>FIXTURE_THINKING};
const line=footer.renderFooter(w,themeColored,{},ctx,pi)[0];
assert.equal(line.includes("\x1b["),false,`dumb should strip ANSI at w=${w}`);
assert.ok(footer.visibleWidth(line)<=w,`dumb vw ${footer.visibleWidth(line)} > ${w}`);
}
});
});

// ── visibleWidth correctness ─────────────────────────────────────────────────
describe('visibleWidth correctness',()=>{
let footer;
beforeEach(async()=>{footer=await import(`${footerUrl}?vw=${Date.now()}${Math.random()}`);});
it('⚡ is width 2 (wide emoji)',()=>{assert.equal(footer.visibleWidth('⚡'),2);});
it(' ▕  is width 3 (space + ▕ + space)',()=>{assert.equal(footer.visibleWidth(' ▕ '),3);});
it('⑂ test is width 7 (⑂=2)',()=>{assert.equal(footer.visibleWidth('⑂ test'),7);});
it('↑18k ↓266 is width 11 (arrows wide)',()=>{assert.equal(footer.visibleWidth('↑18k ↓266'),11);});
it('ANSI stripped: \\x1b[31mhello\\x1b[0m is 5',()=>{assert.equal(footer.visibleWidth('\x1b[31mhello\x1b[0m'),5);});
it('ANSI with wide char: colored ⚡ is still 2',()=>{assert.equal(footer.visibleWidth('\x1b[38;5;1m⚡\x1b[39m'),2);});
it('empty and null yields 0',()=>{assert.equal(footer.visibleWidth(''),0);assert.equal(footer.visibleWidth(null),0);assert.equal(footer.visibleWidth(undefined),0);});
it('CJK 中 is width 2',()=>{assert.equal(footer.visibleWidth('a中b'),4);});
it('branch icon + branch 25c counts correctly',()=>{const s=`⑂ ${FIXTURE_BRANCH}`;assert.equal(footer.visibleWidth(s),2+1+25);});
it('model raw width matches visibleWidth(raw) from renderModelInfo',()=>{
const info=footer.renderModelInfo(FIXTURE_MODEL_ID,FIXTURE_PROVIDER,FIXTURE_THINKING,themePlain);
assert.equal(info.rawWidth,footer.visibleWidth(info.raw));
assert.equal(footer.visibleWidth('⚡ '+FIXTURE_MODEL_ID+' ('+FIXTURE_PROVIDER+') • '+FIXTURE_THINKING),info.rawWidth);
});
});

// ── truncateToWidth ANSI ─────────────────────────────────────────────────────
describe('truncateToWidth ANSI',()=>{
let footer;
beforeEach(async()=>{footer=await import(`${footerUrl}?trunc=${Date.now()}${Math.random()}`);});
it('truncated colored string never exceeds width',()=>{
const colored=themeColored.fg('x','Hello World This Is A Very Long String For Truncation Test');
for(const w of [20,10,5,2,1]){
const t=footer.truncateToWidth(colored,w);
assert.ok(footer.visibleWidth(t)<=w,`w=${w} vw=${footer.visibleWidth(t)} t=${JSON.stringify(t)}`);
}
});
it('truncated colored still preserves ANSI prefix when width>1',()=>{
const colored=themeColored.fg('x','Hello World Long');
const t=footer.truncateToWidth(colored,10);
assert.ok(footer.visibleWidth(t)<=10);
assert.ok(t.includes('\x1b['),`should preserve ANSI, got ${JSON.stringify(t)}`);
assert.ok(t.endsWith('…'),`should end with ellipsis when truncated`);
});
it('truncate model raw with thinking still <= width and preserves ANSI when colored theme',()=>{
const pi={getThinkingLevel:()=>FIXTURE_THINKING};
const ctx=legacyCtx();
const segs=footer.buildFooterSegments(themeColored,{},ctx,pi);
const modelSeg=segs.modelSeg;
assert.ok(modelSeg.length>0);
for(const w of [30,20,10]){
const t=footer.truncateToWidth(modelSeg,w);
assert.ok(footer.visibleWidth(t)<=w,`model truncate w=${w} vw=${footer.visibleWidth(t)}`);
}
});
it('truncate with empty ellipsis respects width',()=>{
const s='hello world';
const t=footer.truncateToWidth(s,5,'');
assert.ok(footer.visibleWidth(t)<=5);
assert.equal(t.length<=5,true);
});
it('CJK not split: truncate a中b中c at w=4 keeps width<=4 and no half char',()=>{
const s='a中b中c'; // widths: a1 中2 b1 中2 c1 => 7
const t=footer.truncateToWidth(s,4);
assert.ok(footer.visibleWidth(t)<=4,`vw=${footer.visibleWidth(t)} t=${t}`);
assert.ok(!t.includes('�'));
});
it('width 0 returns empty, width 1 returns ellipsis',()=>{
assert.equal(footer.truncateToWidth('hello',0),'');
assert.equal(footer.truncateToWidth('hello',1),'…');
});
});

// ── fuzz/stress longest combos ───────────────────────────────────────────────
describe('fuzz: longest combos never exceed width 120..20',()=>{
let footer;
beforeEach(async()=>{footer=await import(`${footerUrl}?fuzz=${Date.now()}${Math.random()}`);});
afterEach(()=>{for(const k of["BIGGZ_PRETTY","TERM","PI_SUBAGENT_CHILD"])delete process.env[k];});
const combos=[
{id:'branch60/path100/model80 legacy',mkCtx:()=>legacyCtx({branch:LONG_BRANCH_60,cwd:LONG_PATH_100,modelId:LONG_MODEL_80}),pi:{getThinkingLevel:()=>'xhigh'}},
{id:'branch60 alone legacy',mkCtx:()=>legacyCtx({branch:LONG_BRANCH_60_RAW,cwd:'/tmp',modelId:'short'}),pi:{getThinkingLevel:()=>'off'}},
{id:'new contract long branch+change 60',mkCtx:()=>newCtx({branch:LONG_BRANCH_60,change:LONG_BRANCH_60_RAW,lineage:999,lens:'99/99',budget:'99/99'}),pi:null},
];
for(const {id,mkCtx,pi} of combos){
for(const w of WIDTHS){
it(`${id} at width ${w} never exceeds`,()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
const ctx=mkCtx();
const segs=footer.buildFooterSegments(themeColored,{},ctx,pi);
const line=footer.renderFooterLine(w,themeColored,segs,ctx);
assert.ok(footer.visibleWidth(line)<=w,`${id} vw=${footer.visibleWidth(line)} > ${w} line=${JSON.stringify(strip(line).slice(0,80))}`);
const line2=footer.renderFooter(w,themeColored,{},ctx,pi)[0];
assert.ok(footer.visibleWidth(line2)<=w,`renderFooter ${id} vw=${footer.visibleWidth(line2)} > ${w}`);
});
}
}
it('stress: 60c branch + 80c model + 100c path combo exhaustive widths 120..20 with both themes',()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
for(const w of WIDTHS){
for(const theme of [themeColored,themePlain]){
const ctx=legacyCtx({branch:LONG_BRANCH_60,cwd:LONG_PATH_100,modelId:LONG_MODEL_80,provider:'opencode-go'});
const pi={getThinkingLevel:()=>'xhigh'};
const segs=footer.buildFooterSegments(theme,{},ctx,pi);
const line=footer.renderFooterLine(w,theme,segs,ctx);
assert.ok(footer.visibleWidth(line)<=w,`theme=${theme===themeColored?'colored':'plain'} w=${w} vw=${footer.visibleWidth(line)}`);
}
}
});
it('extreme narrow 20 always single segment truncated not empty',()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
const ctx=legacyCtx({branch:LONG_BRANCH_60,cwd:LONG_PATH_100,modelId:LONG_MODEL_80});
const pi={getThinkingLevel:()=>'xhigh'};
const line=footer.renderFooterLine(20,themeColored,footer.buildFooterSegments(themeColored,{},ctx,pi),ctx);
assert.ok(footer.visibleWidth(line)<=20);
assert.ok(footer.visibleWidth(line)>0,`should not be empty at 20, got ${JSON.stringify(line)}`);
});
});

// ── guards: isPrettyEnabled / BIGGZ_PRETTY=0 / TERM=dumb ───────────────────
describe('guards regression',()=>{
let footer;
beforeEach(async()=>{footer=await import(`${footerUrl}?guard=${Date.now()}${Math.random()}`);});
afterEach(()=>{for(const k of["BIGGZ_PRETTY","TERM","PI_SUBAGENT_CHILD","BIGGZ_NERDFONT"])delete process.env[k];});
it('isPrettyEnabled false when BIGGZ_PRETTY=0',()=>{
process.env.BIGGZ_PRETTY='0';
assert.equal(footer.isPrettyEnabled(),false);
});
it('isPrettyEnabled false when PI_SUBAGENT_CHILD=1',()=>{
process.env.PI_SUBAGENT_CHILD='1';
assert.equal(footer.isPrettyEnabled(),false);
});
it('renderFooter returns [""] when BIGGZ_PRETTY=0 (bypass)',()=>{
process.env.BIGGZ_PRETTY='0';
const ctx=legacyCtx();
const pi={getThinkingLevel:()=>FIXTURE_THINKING};
const out=footer.renderFooter(120,themeColored,{},ctx,pi);
assert.deepEqual(out,[""]);
});
it('renderFooter returns [""] when PI_SUBAGENT_CHILD=1',()=>{
process.env.PI_SUBAGENT_CHILD='1';
const ctx=legacyCtx();
const pi={getThinkingLevel:()=>FIXTURE_THINKING};
const out=footer.renderFooter(120,themeColored,{},ctx,pi);
assert.deepEqual(out,[""]);
});
it('isDumbTerm true when TERM=dumb',()=>{
process.env.TERM='dumb';
assert.equal(footer.isDumbTerm(),true);
});
it('export default bypasses injection when BIGGZ_PRETTY=0',async()=>{
process.env.BIGGZ_PRETTY='0';
const m=await import(`${footerUrl}?bypass=${Date.now()}${Math.random()}`);
const pi={on:()=>{}};
m.default(pi);
assert.equal(pi._biggzFooter,undefined);
});
});

// ── git worktree counts (gentle parity: parsePorcelain) ─────────────────────
describe('git counts',()=>{
let footer;
const NL = String.fromCharCode(10);
beforeEach(async()=>{footer=await import(`${footerUrl}?git=${Date.now()}${Math.random()}`);footer._resetGitCountsForTest();});
afterEach(()=>{delete process.env.BIGGZ_GIT_STATUS;try{footer._resetGitCountsForTest();}catch{}});
it('parsePorcelain staged/unstaged/untracked/ignored',()=>{
const out = ["M  a.js"," M b.js","A  c.js","R  o.js -> n.js","?? u.js","!! ign.js",""].join(NL);
assert.deepEqual(footer.parsePorcelain(out),{staged:3,unstaged:1,untracked:1});
assert.deepEqual(footer.parsePorcelain(""),{staged:0,unstaged:0,untracked:0});
assert.deepEqual(footer.parsePorcelain(null),{staged:0,unstaged:0,untracked:0});
});
it('formatGitCounts only non-zero as *U +S ?T',()=>{
assert.equal(footer.formatGitCounts({staged:0,unstaged:0,untracked:0}),"");
assert.equal(footer.formatGitCounts({staged:1,unstaged:2,untracked:3})," *2 +1 ?3");
assert.equal(footer.formatGitCounts({staged:2})," +2");
});
it('getGitCounts injectable runner + null on throw + TTL cache',()=>{
let calls=0;
const run=(...a)=>{calls++;return [" M a.js",""].join(NL);};
assert.deepEqual(footer.getGitCounts("/repo",run),{staged:0,unstaged:1,untracked:0});
assert.deepEqual(footer.getGitCounts("/repo",()=>{throw new Error("boom");}),{staged:0,unstaged:1,untracked:0});
assert.equal(calls,1);
assert.equal(footer.getGitCounts("/nope",()=>{throw new Error("x");}),null);
});
it('BIGGZ_GIT_STATUS=0 never shells out',()=>{
process.env.BIGGZ_GIT_STATUS="0";
let calls=0;
assert.equal(footer.getGitCounts("/repo",()=>{calls++;return "";}),null);
assert.equal(calls,0);
});
it('branch segment carries counts, raw stays clean',()=>{
process.env.BIGGZ_PRETTY='1';process.env.TERM='xterm-256color';
const theme={fg:(_,t)=>t,symbolPreset:"unicode"};
const ctx={branch:"main",cwd:"/definitely-not-a-repo-xyz"};
const segs=footer.buildFooterSegments(theme,{},ctx,null);
assert.ok(String(segs.raw.branch).includes("main"),"raw keeps branch");
assert.ok(!String(segs.branchSeg).includes("*"),"no counts outside repo");
});
});
