#!/usr/bin/env node
import {createHash} from "node:crypto";
import {existsSync,readFileSync,readdirSync,statSync} from "node:fs";
import {join,relative} from "node:path";
import {fileURLToPath} from "node:url";
const root=join(fileURLToPath(new URL("..",import.meta.url)));
const lock=join(root,"contracts/review-integration/provider-contract.lock.json");
function walk(d,o=[]){for(const e of readdirSync(d)){const p=join(d,e);const s=statSync(p);if(s.isDirectory())walk(p,o);else o.push(p)}return o}
function sha(p){return createHash("sha256").update(readFileSync(p)).digest("hex")}
if(!existsSync(lock)){console.error("missing lock");process.exit(1)}
let map;try{map=JSON.parse(readFileSync(lock,"utf8"))}catch(e){console.error(e.message);process.exit(1)}
const roots=[join(root,"contracts/review-integration/v1"),join(root,"contracts/review-integration/v2")].filter(existsSync);
const files=roots.flatMap(d=>walk(d));const act={};for(const f of files){const k=relative(root,f).replaceAll("\\","/");if(k.includes("provider-contract.lock.json"))continue;act[k]=sha(f)}
let drift=false;for(const k of Object.keys(map))if(!(k in act)||map[k]!==act[k]){console.error(`drift ${k}`);drift=true}for(const k of Object.keys(act))if(!(k in map)){console.error(`unlisted ${k}`);drift=true}
if(drift){console.error("provider contract drift - offline only");process.exit(1)}console.log(`check passed ${Object.keys(act).length} files`);
