#!/usr/bin/env node
import {existsSync,readFileSync,readdirSync,statSync} from "node:fs";
import {join,relative} from "node:path";
import {fileURLToPath} from "node:url";
const root=join(fileURLToPath(new URL("..",import.meta.url)));
const lock=join(root,"contracts/review-integration/provider-contract.lock.json");
function walk(d,o=[]){for(const e of readdirSync(d)){const p=join(d,e);const s=statSync(p);if(s.isDirectory())walk(p,o);else o.push(p)}return o}
if(!existsSync(lock)){console.error("missing manifest");process.exit(1)}
let map;try{map=JSON.parse(readFileSync(lock,"utf8"))}catch(e){console.error(e.message);process.exit(1)}
const roots=[join(root,"contracts/review-integration/v1"),join(root,"contracts/review-integration/v2")].filter(existsSync);
const files=roots.flatMap(d=>walk(d)).map(p=>relative(root,p).replaceAll("\\","/")).filter(k=>!k.includes("provider-contract.lock.json")).sort();
const listed=new Set(Object.keys(map));const walked=new Set(files);let miss=false;
for(const f of files)if(!listed.has(f)){console.error(`unlisted ${f}`);miss=true}
for(const k of listed)if(!walked.has(k)){console.error(`missing ${k}`);miss=true}
if(miss){console.error("manifest mismatch");process.exit(1)}console.log(`verify passed ${files.length} files`);
