#!/usr/bin/env node
import {readdirSync,readFileSync,statSync} from "node:fs";import {join} from "node:path";
const ROOT=process.cwd();
function countTokens(b){return b.trim()?b.trim().split(/\s+/).length:0}
function extractFrontmatter(c){
 if(!c.startsWith("---\n")&&!c.startsWith("---\r\n"))return{error:"missing frontmatter",body:c,fm:""};
 const rest=c.slice(4);const idx=rest.indexOf("\n---");if(idx===-1)return{error:"missing frontmatter closing ---",body:c,fm:""};
 return{fm:rest.slice(0,idx),body:rest.slice(idx+4).replace(/^\r?\n/,""),error:""}
}
function validateFrontmatter(fm){
 let found=false;
 for(const raw of fm.split("\n")){
  const trim=raw.trim();if(!trim.startsWith("description:"))continue;
  found=true;const val=trim.slice("description:".length).trim();
  if(!val)return"description missing value (multi-line not allowed)";
  if(!((val.startsWith('"')&&val.endsWith('"'))||(val.startsWith("'")&&val.endsWith("'"))))return"description must be single-line quoted";
  const inner=val.slice(1,-1);if(inner.length>250)return`description exceeds 250 chars (${inner.length})`;
  if(!inner.includes("Trigger:")&&!inner.includes("trigger:"))return"description missing trigger keyword";break;
 }
 if(!found)return"missing description in frontmatter";return"";
}
function lintFile(p){
 const c=readFileSync(p,"utf8");const{fm,body,error}=extractFrontmatter(c);const diags=[];
 if(error)diags.push(`FAIL: ${error}`);else{const fe=validateFrontmatter(fm);if(fe)diags.push(`FAIL: ${fe}`)}
 const tokens=countTokens(body);
 if(tokens>1000)diags.push(`FAIL: token count ${tokens} exceeds hard limit 1000`);
 else if(tokens>450)diags.push(`WARN: token count ${tokens} exceeds ideal 450 (warn until 1000)`);
 else if(tokens>0&&tokens<180)diags.push(`WARN: token count ${tokens} below ideal 180`);
 return{tokens,diags,hasFail:diags.some(d=>d.startsWith("FAIL:")),hasWarn:diags.some(d=>d.startsWith("WARN:"))}
}
function findSkills(dir,out){let e=[];try{e=readdirSync(dir)}catch{return}for(const n of e){const p=join(dir,n);let s;try{s=statSync(p)}catch{continue}if(s.isDirectory())findSkills(p,out);else if(n==="SKILL.md")out.push(p)}}
const paths=[];for(const base of["skills","internal/assets/skills"])findSkills(join(ROOT,base),paths);
if(!paths.length){console.log("check:skill-lint: no SKILL.md found");process.exit(0)}
let failed=false,warned=false;
for(const p of paths){const{tokens,diags,hasFail,hasWarn}=lintFile(p);const rel=p.replace(ROOT+"\\","").replace(ROOT+"/","");if(hasFail){failed=true;console.error(`FAIL ${rel} (${tokens} tokens): ${diags.join("; ")}`)}else if(hasWarn){warned=true;console.warn(`WARN ${rel} (${tokens} tokens): ${diags.join("; ")}`)}else console.log(`OK ${rel} (${tokens} tokens)`)}
if(failed)process.exit(1);if(warned)process.exit(2);process.exit(0);
