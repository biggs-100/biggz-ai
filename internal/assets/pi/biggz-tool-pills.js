/**
 * biggz-tool-pills — compact colored pill labels for Pi tools
 * Ported from tomsej/pi-ext tool-pills (MIT) as offline biggz-native extension.
 * No Shiki — uses theme tokens + ANSI for lightweight pills and collapsed output.
 * @param {import("@earendil-works/pi-coding-agent").ExtensionAPI} pi
 */

// ── Pill definitions: tool → { label, bg, fg } using theme color keys ──
export const TOOL_PILL_MAP = Object.freeze({
	read: { label: "READ", bg: "toolPendingBg", fg: "toolTitle" },
	write: { label: "WRITE", bg: "toolSuccessBg", fg: "success" },
	edit: { label: "EDIT", bg: "toolSuccessBg", fg: "warning" },
	bash: { label: "BASH", bg: "toolPendingBg", fg: "bashMode" },
	grep: { label: "GREP", bg: "toolPendingBg", fg: "muted" },
	find: { label: "FIND", bg: "toolPendingBg", fg: "muted" },
	task: { label: "TASK", bg: "toolPendingBg", fg: "accent" },
	question: { label: "ASK", bg: "toolPendingBg", fg: "accent" },
	ask_user_question: { label: "ASK", bg: "toolPendingBg", fg: "accent" },
	subagent: { label: "AGENT", bg: "toolPendingBg", fg: "accent" },
});

export function getToolPill(toolName) {
	if (!toolName) return null;
	const key = String(toolName).toLowerCase();
	return TOOL_PILL_MAP[key] || { label: key.toUpperCase().slice(0, 6), bg: "toolPendingBg", fg: "muted" };
}

export function isPrettyEnabled(){return process.env.BIGGZ_PRETTY!=="0"&&process.env.PI_SUBAGENT_CHILD!=="1"}
export function isDumbTerm(){return process.env.TERM==="dumb"}
export function isAnimationEnabled(){return isPrettyEnabled()&&!isDumbTerm()&&process.env.BIGGZ_NO_ANIMATION!=="1"&&process.env.GENTLE_AI_NO_ANIMATION!=="1"}
export function stripAnsi(s){try{return String(s??"").replace(/\x1b\[[0-9;]*[A-Za-z]/g,"").replace(/\x1b\][^\x07]*\x07/g,"")}catch{return String(s??"")}}
export const PILL_STATE_STYLES=Object.freeze({running:{icon:"⠋",bg:"toolPendingBg",fg:"accent",spinner:true},queued:{icon:"◷",bg:"toolPendingBg",fg:"muted"},complete:{icon:"✓",bg:"toolSuccessBg",fg:"success"},failed:{icon:"✗",bg:"toolErrorBg",fg:"error"},pending:{icon:"⠋",bg:"toolPendingBg",fg:"accent",spinner:true},success:{icon:"✓",bg:"toolSuccessBg",fg:"success"},error:{icon:"✗",bg:"toolErrorBg",fg:"error"}});
export const SPINNER_FRAMES=Object.freeze(["⠋","⠙","⠹","⠸","⠼","⠴","⠦","⠧","⠇","⠏"]);
function getSpinnerFrame(){return isAnimationEnabled()?SPINNER_FRAMES[0]:"·"}

// Minimal ANSI pill: [ LABEL ] with theme-aware colors fallback to ANSI codes
export function ansiPill(label, opts = {}) {
	const raw=String(label??""); if(!isPrettyEnabled()||isDumbTerm()) return raw;
	const state=opts.state?String(opts.state).toLowerCase():""; const st=state?PILL_STATE_STYLES[state]:null;
	let display=raw; if(st&&st.spinner){display=`${getSpinnerFrame()} ${raw}`}else if(st&&st.icon&&opts.withIcon!==false&&!raw.includes(st.icon)){display=`${st.icon} ${raw}`}
	const bgKey=(st&&st.bg)||opts.bg||"toolPendingBg", fgKey=(st&&st.fg)||opts.fg||"muted";
	const bgMap={toolPendingBg:"\x1b[48;5;235m",toolSuccessBg:"\x1b[48;5;22m",toolErrorBg:"\x1b[48;5;52m"};
	const fgMap={accent:"\x1b[38;5;39m",success:"\x1b[38;5;84m",warning:"\x1b[38;5;214m",error:"\x1b[38;5;196m",muted:"\x1b[38;5;244m",bashMode:"\x1b[38;5;84m",toolTitle:"\x1b[38;5;39m"};
	return `${bgMap[bgKey]||"\x1b[48;5;235m"}${fgMap[fgKey]||"\x1b[38;5;244m"} ${display} \x1b[0m`;
}
// Collapsed output helper: show first 3 lines + hidden count (now … +N hidden, guards)
export function collapseOutput(text, maxLines = 3) {
	if (typeof text !== "string") return ""; const lines=text.split("\n");
	if(lines.length<=maxLines) return (!isPrettyEnabled()||isDumbTerm())?stripAnsi(text):text;
	const visible=lines.slice(0,maxLines), hidden=lines.length-maxLines;
	return (!isPrettyEnabled()||isDumbTerm())?`${visible.join("\n")}\n… +${hidden} hidden`:`${visible.join("\n")}\n\x1b[2m… +${hidden} hidden\x1b[22m`;
}
export function collapsePills(pills,limit=3){if(!Array.isArray(pills))return{visible:[],hidden:0,suffix:""}; if(pills.length<=limit)return{visible:pills.slice(),hidden:0,suffix:""}; return{visible:pills.slice(0,limit),hidden:pills.length-limit,suffix:`… +${pills.length-limit} hidden`}}
export function renderPills(pills,limit=3){if(!Array.isArray(pills)||pills.length===0)return""; if(!isPrettyEnabled()||isDumbTerm()){const{visible,suffix}=collapsePills(pills,limit); const labels=visible.map(p=>p==null?"":typeof p==="string"?p:String(p.label??p.name??p.tool??p)); let s=labels.filter(Boolean).join(" "); if(suffix)s+=(s?" ":"")+suffix; return stripAnsi(s)} const{visible,suffix}=collapsePills(pills,limit); const rendered=visible.map(p=>{if(p==null)return""; if(typeof p==="string")return ansiPill(p,{}); const label=String(p.label??p.name??p.tool??""), state=p.state??p.status??"", base=getToolPill(p.tool??p.name??label); const opts={bg:base?.bg,fg:base?.fg,state:state||undefined,withIcon:true}; return state?ansiPill(label||base.label,opts):ansiPill(label||base.label,{bg:base.bg,fg:base.fg})}); let s=rendered.filter(Boolean).join(" "); if(suffix)s+=` \x1b[2m${suffix}\x1b[22m`; return s}
export function renderPill(label,state){return renderPills([{label,state}],3)}
export function syntaxHighlight(text){if(typeof text!=="string")return text; if(!isPrettyEnabled()||isDumbTerm())return stripAnsi(text); return text.replace(/\b(function|const|let|var|if|else|return|import|export|class|async|await)\b/g,"\x1b[38;5;39m$1\x1b[39m").replace(/("[^"]*"|'[^']*')/g,"\x1b[38;5;84m$1\x1b[39m")} 

export default function biggzToolPills(pi) {
	try {
		const api={TOOL_PILL_MAP,PILL_STATE_STYLES,SPINNER_FRAMES,getToolPill,ansiPill,collapseOutput,collapsePills,renderPills,renderPill,syntaxHighlight,isPrettyEnabled,isAnimationEnabled,isDumbTerm,stripAnsi,getSpinnerFrame};
		if(pi){pi._biggzToolPills=api; if(pi._biggzExtension)pi._biggzExtension.getToolPill=getToolPill}
		if(typeof globalThis!=="undefined")globalThis._biggzToolPills=api;
	} catch {}
	if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if (process.env.BIGGZ_PRETTY === "0") return;
	if (!pi||typeof pi.on !== "function") return;

	// Hook tool_call to inject pill metadata (non-blocking, observability)
	try {
		pi.on("tool_call", async (event) => {
			const name = event?.toolName ?? event?.name ?? "";
			const pill = getToolPill(name);
			// Attach pill info to event for downstream renderers (if pi supports event.pill)
			try {
				if (event && pill) event._pill = pill;
			} catch {}
		});
	} catch {}

	// Hook tool_result to provide pill-aware rendering helper
	try {
		pi.on("tool_result", async (event) => {
			// No block, just ensure pill info persists
			try {
				const name = event?.toolName ?? event?.name ?? "";
				if (name && event && !event._pill) event._pill = getToolPill(name);
			} catch {}
		});
	} catch {}
}
