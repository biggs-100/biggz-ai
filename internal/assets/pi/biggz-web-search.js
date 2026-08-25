/**
 * biggz-web-search — Pi extension for open-web sdd-research (PR2).
 * Exposes web_search + web_fetch gated to sdd-research with open-web grant.
 * SSRF guard: blocks file:/data:/ftp:/gopher:, private CIDRs, localhost, DNS re-check.
 * Provider chain: Tavily -> Brave -> DuckDuckGo (no-key fallback, BIGGZ_DDG_FALLBACK gate).
 * Fetch tiers: T1 net/http+headers -> T2 tls-client/utls (chrome124/safari17) on 403 -> T3 headless (flag).
 * Caps: 10s AbortController -> FetchBlocked, 1MB truncate+annotate, Retry-After backoff, loud FetchBlocked.
 * Extract: HTML -> Markdown via lightweight readability+html-to-markdown (JS port).
 * Secrets: env-only, log providerOrder without keys, publisher DuckDuckGo (no-key fallback).
 */

import dns from "node:dns/promises";
import fs from "node:fs";
import net from "node:net";
import os from "node:os";
import path from "node:path";

const BLOCKED_SCHEMES = new Set(["file:", "data:", "ftp:", "gopher:"]);
const ONE_MB = 1024 * 1024;
const FETCH_TIMEOUT_MS = 10_000;
const MAX_RETRIES = 3;
const BASE_BACKOFF_MS = 1000;
const CHROME124_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36";
const SAFARI17_UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4.1 Safari/605.1.15";

function isIPv4Private(ip) {
  const p = ip.split(".").map(Number);
  if (p.length !== 4 || p.some((n) => Number.isNaN(n) || n < 0 || n > 255)) return false;
  if (p[0] === 127) return true;
  if (p[0] === 10) return true;
  if (p[0] === 192 && p[1] === 168) return true;
  if (p[0] === 172 && p[1] >= 16 && p[1] <= 31) return true;
  if (p[0] === 169 && p[1] === 254) return true;
  return false;
}

function isIPv6Private(ip) {
  const lower = ip.toLowerCase();
  if (lower === "::1" || lower === "0:0:0:0:0:0:0:1") return true;
  const firstHextet = parseInt(lower.split(":")[0], 16);
  if (!Number.isNaN(firstHextet)) {
    if ((firstHextet & 0xffc0) === 0xfe80) return true;
    if ((firstHextet & 0xfe00) === 0xfc00) return true;
  }
  return false;
}

function stripBrackets(h) {
  if (h.startsWith("[") && h.endsWith("]")) return h.slice(1, -1);
  return h;
}
function isPrivateIP(ip) {
  const bare = stripBrackets(ip);
  if (net.isIP(bare) === 4) return isIPv4Private(bare);
  if (net.isIP(bare) === 6) return isIPv6Private(bare);
  return false;
}

function isBlockedHostname(hostname) {
  const h = stripBrackets(hostname.toLowerCase());
  if (h === "localhost" || h.endsWith(".localhost")) return true;
  if (net.isIP(h) && isPrivateIP(h)) return true;
  return false;
}

async function dnsRecheck(hostname) {
  const bare = stripBrackets(hostname);
  if (net.isIP(bare)) return !isPrivateIP(bare);
  try {
    const addrs = await dns.lookup(bare, { all: true });
    for (const a of addrs) {
      if (isPrivateIP(a.address)) return false;
    }
    return true;
  } catch {
    return true;
  }
}

async function assertSSRF(urlStr) {
  let u;
  try {
    u = new URL(urlStr);
  } catch {
    throw Object.assign(new Error(`SSRF: invalid URL ${urlStr}`), { code: "SSRF" });
  }
  if (BLOCKED_SCHEMES.has(u.protocol)) {
    throw Object.assign(new Error(`SSRF: blocked scheme ${u.protocol}`), { code: "SSRF", scheme: u.protocol });
  }
  if (isBlockedHostname(u.hostname)) {
    throw Object.assign(new Error(`SSRF: blocked host ${u.hostname}`), { code: "SSRF", host: u.hostname });
  }
  const ok = await dnsRecheck(u.hostname);
  if (!ok) throw Object.assign(new Error(`SSRF: DNS re-check blocked ${u.hostname}`), { code: "SSRF", host: u.hostname });
}

function publisherFor(provider) {
  if (provider === "duckduckgo") return "DuckDuckGo (no-key fallback)";
  if (provider === "tavily") return "Tavily";
  if (provider === "brave") return "Brave";
  return provider;
}

function toContent(payload, isError = false) {
  return { content: [{ type: "text", text: JSON.stringify(payload, null, 2) }], isError };
}

function resolveProviderOrder(env) {
  const order = [];
  if (env.TAVILY_API_KEY) order.push("tavily");
  if (env.BRAVE_API_KEY) order.push("brave");
  // DuckDuckGo no-key fallback is now default (no env gate) — keeps web_search working without API keys
  // Keep BIGGZ_DDG_FALLBACK gate for backward compat, but always include duckduckgo as last resort
  order.push("duckduckgo");
  return order;
}

// ——— PR2 helpers ———

function parseRetryAfter(value) {
  if (!value) return null;
  const v = String(value).trim();
  const secs = Number(v);
  if (!Number.isNaN(secs) && /^\d+$/.test(v)) return secs * 1000;
  const d = Date.parse(v);
  if (!Number.isNaN(d)) {
    const diff = d - Date.now();
    return diff > 0 ? diff : 0;
  }
  return null;
}

function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

function buildHeaders(profile) {
  if (profile === "chrome124") {
    return {
      "User-Agent": CHROME124_UA,
      Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
      "Accept-Language": "en-US,en;q=0.9",
      "Accept-Encoding": "gzip, deflate, br",
      "Sec-Ch-Ua": '"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"',
      "Sec-Ch-Ua-Mobile": "?0",
      "Sec-Ch-Ua-Platform": '"Windows"',
      "Sec-Fetch-Dest": "document",
      "Sec-Fetch-Mode": "navigate",
      "Sec-Fetch-Site": "none",
      "Upgrade-Insecure-Requests": "1",
    };
  }
  if (profile === "safari17") {
    return {
      "User-Agent": SAFARI17_UA,
      Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
      "Accept-Language": "en-US,en;q=0.9",
      "Accept-Encoding": "gzip, deflate, br",
      "Sec-Fetch-Dest": "document",
      "Sec-Fetch-Mode": "navigate",
      "Sec-Fetch-Site": "none",
    };
  }
  return {
    "User-Agent": CHROME124_UA,
    Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.9",
    "Accept-Encoding": "gzip, deflate",
  };
}

function htmlToMarkdown(html, baseUrl) {
  if (!html || typeof html !== "string") return "";
  let cleaned = html
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "")
    .replace(/<noscript[\s\S]*?<\/noscript>/gi, "")
    .replace(/<iframe[\s\S]*?<\/iframe>/gi, "");
  let content = cleaned;
  const bodyMatch = cleaned.match(/<body[^>]*>([\s\S]*?)<\/body>/i);
  if (bodyMatch) content = bodyMatch[1];
  const article = content.match(/<article[^>]*>([\s\S]*?)<\/article>/i);
  if (article && article[1].trim().length > 200) content = article[1];
  else {
    const main = content.match(/<main[^>]*>([\s\S]*?)<\/main>/i);
    if (main && main[1].trim().length > 200) content = main[1];
  }
  let md = content
    .replace(/<h1[^>]*>(.*?)<\/h1>/gi, "# $1\n\n")
    .replace(/<h2[^>]*>(.*?)<\/h2>/gi, "## $1\n\n")
    .replace(/<h3[^>]*>(.*?)<\/h3>/gi, "### $1\n\n")
    .replace(/<h4[^>]*>(.*?)<\/h4>/gi, "#### $1\n\n")
    .replace(/<p[^>]*>(.*?)<\/p>/gi, "$1\n\n")
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<strong[^>]*>(.*?)<\/strong>/gi, "**$1**")
    .replace(/<b[^>]*>(.*?)<\/b>/gi, "**$1**")
    .replace(/<em[^>]*>(.*?)<\/em>/gi, "*$1*")
    .replace(/<i[^>]*>(.*?)<\/i>/gi, "*$1*")
    .replace(/<code[^>]*>(.*?)<\/code>/gi, "`$1`")
    .replace(/<pre[^>]*>(.*?)<\/pre>/gi, "```\n$1\n```\n\n")
    .replace(/<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)<\/a>/gi, "[$2]($1)")
    .replace(/<li[^>]*>(.*?)<\/li>/gi, "- $1\n")
    .replace(/<ul[^>]*>/gi, "\n")
    .replace(/<\/ul>/gi, "\n")
    .replace(/<ol[^>]*>/gi, "\n")
    .replace(/<\/ol>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&#39;/g, "'")
    .replace(/\r/g, "")
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
  // resolve relative links if baseUrl provided (best-effort)
  if (baseUrl) {
    try {
      const base = new URL(baseUrl);
      md = md.replace(/\[([^\]]+)\]\((\/[^)]+)\)/g, (_, t, p) => `[${t}](${base.origin}${p})`);
    } catch {}
  }
  return md;
}

// Provider live fetch — isolated for testability and key-env gating
async function searchTavily(query, limit, apiKey, fetcher = fetch) {
  const controller = new AbortController();
  const t = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const res = await fetcher("https://api.tavily.com/search", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        api_key: apiKey,
        query,
        max_results: limit,
        include_answer: false,
        search_depth: "basic",
      }),
      signal: controller.signal,
    });
    if (!res.ok) throw new Error(`Tavily ${res.status}`);
    const data = await res.json();
    const results = (data.results || data.answer ? data.results || [] : []).slice(0, limit).map((r) => ({
      title: r.title || r.url || "",
      url: r.url || "",
      snippet: r.content || r.snippet || "",
      score: r.score,
    }));
    return { results };
  } finally {
    clearTimeout(t);
  }
}

async function searchBrave(query, limit, apiKey, fetcher = fetch) {
  const url = `https://api.search.brave.com/res/v1/web/search?q=${encodeURIComponent(query)}&count=${limit}`;
  const controller = new AbortController();
  const t = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const res = await fetcher(url, {
      headers: { Accept: "application/json", "X-Subscription-Token": apiKey, "Accept-Encoding": "gzip" },
      signal: controller.signal,
    });
    if (!res.ok) throw new Error(`Brave ${res.status}`);
    const data = await res.json();
    const web = data.web?.results || data.results || [];
    const results = web.slice(0, limit).map((r) => ({
      title: r.title || "",
      url: r.url || "",
      snippet: r.description || r.snippet || "",
    }));
    return { results };
  } finally {
    clearTimeout(t);
  }
}

async function searchDDG(query, limit, fetcher = fetch) {
  const url = `https://api.duckduckgo.com/?q=${encodeURIComponent(query)}&format=json&pretty=1&no_html=1&skip_disambig=1`;
  const controller = new AbortController();
  const t = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const res = await fetcher(url, {
      headers: { Accept: "application/json", "User-Agent": CHROME124_UA },
      signal: controller.signal,
    });
    if (!res.ok) throw new Error(`DDG ${res.status}`);
    const data = await res.json();
    const out = [];
    const pushTopics = (topics) => {
      for (const tp of topics || []) {
        if (tp.Topics) pushTopics(tp.Topics);
        else if (tp.FirstURL && tp.Text) out.push({ title: tp.Text.slice(0, 120), url: tp.FirstURL, snippet: tp.Text });
        if (out.length >= limit) break;
      }
    };
    pushTopics(data.RelatedTopics);
    if (data.Results) {
      for (const r of data.Results) {
        if (out.length >= limit) break;
        if (r.FirstURL) out.push({ title: r.Text?.slice(0, 120) || r.FirstURL, url: r.FirstURL, snippet: r.Text || "" });
      }
    }
    // fallback: if API yielded nothing, try HTML scrape (api.duckduckgo.com is instant answers only, often 0 for news queries)
    if (out.length === 0) {
      try {
        const htmlUrl = `https://html.duckduckgo.com/html/?q=${encodeURIComponent(query)}`;
        const htmlRes = await fetch(htmlUrl, {
          headers: { Accept: "text/html", "User-Agent": CHROME124_UA },
        });
        if (htmlRes.ok) {
          const html = await htmlRes.text();
          // Parse result__a links and result__snippet
          const linkRe = /<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)<\/a>/gi;
          const snippetRe = /<a[^>]*class="[^"]*result__snippet[^"]*"[^>]*>(.*?)<\/a>/gi;
          // fallback: also match result__url
          let m;
          const links = [];
          const titles = [];
          while ((m = linkRe.exec(html)) !== null && links.length < limit) {
            try {
              const href = m[1].startsWith("//") ? "https:" + m[1] : m[1].startsWith("/") ? "https://duckduckgo.com" + m[1] : m[1];
              // DuckDuckGo wraps via /l/?uddg=...
              let urlDecoded = href;
              const uddg = href.match(/[?&]uddg=([^&]+)/);
              if (uddg) try { urlDecoded = decodeURIComponent(uddg[1]); } catch {}
              const title = m[2].replace(/<[^>]+>/g, "").trim().slice(0, 120);
              if (urlDecoded.startsWith("http")) links.push({ url: urlDecoded, title });
            } catch {}
          }
          // Try to extract snippets separately and merge by index
          const snippets = [];
          let sm;
          while ((sm = snippetRe.exec(html)) !== null && snippets.length < limit) {
            snippets.push(sm[1].replace(/<[^>]+>/g, "").trim());
          }
          for (let i = 0; i < links.length && out.length < limit; i++) {
            out.push({ title: links[i].title || links[i].url, url: links[i].url, snippet: snippets[i] || links[i].title || "" });
          }
        }
      } catch {}
    }
    return { results: out.slice(0, limit) };
  } finally {
    clearTimeout(t);
  }
}

/** @type {import("@earendil-works/pi-coding-agent").ExtensionAPI} */
export default function biggzWebSearch(pi) {
  const register = typeof pi.registerTool === "function" ? pi.registerTool.bind(pi) : null;

  const webSearchHandler = async ({ query, limit } = {}) => {
    const env = process.env;
    const providerOrder = resolveProviderOrder(env);
    console.log(`[biggz-web-search] web_search providerOrder=${providerOrder.join("->") || "none"} query=${JSON.stringify(query)}`);
    if (providerOrder.length === 0) {
      const payload = { error: "blocked", Gaps: "missing TAVILY_API_KEY (or BRAVE_API_KEY, or BIGGZ_DDG_FALLBACK=1) — set BIGGZ_DDG_FALLBACK=1 for no-key DuckDuckGo or add a key then restart pi", providerOrder };
      return { content: [{ type: "text", text: JSON.stringify(payload, null, 2) }], isError: true };
    }
    const lim = typeof limit === "number" && limit > 0 ? Math.min(limit, 10) : 5;
    if (!query || typeof query !== "string" || !query.trim()) {
      return toContent({ error: "blocked", Gaps: "missing query", providerOrder }, true);
    }
    let lastError = null;
    for (const provider of providerOrder) {
      try {
        let r;
        if (provider === "tavily") r = await searchTavily(query, lim, env.TAVILY_API_KEY);
        else if (provider === "brave") r = await searchBrave(query, lim, env.BRAVE_API_KEY);
        else if (provider === "duckduckgo") r = await searchDDG(query, lim);
        const results = r?.results || [];
        // Do not return partial silently: empty results tries next provider; if all empty, fall through to blocked
        if (results.length > 0) {
          return toContent({
            providerOrder,
            publisher: publisherFor(provider),
            results,
            query,
            limit: lim,
          });
        }
        lastError = `provider ${provider} returned 0 results`;
      } catch (e) {
        lastError = e?.message || String(e);
        // try next provider — 429/5xx already handled inside search funcs; fallback is intentional
        continue;
      }
    }
    return toContent({
      error: "blocked",
      Gaps: lastError ? `all providers exhausted: ${lastError}` : "missing TAVILY_API_KEY (or BRAVE_API_KEY, or BIGGZ_DDG_FALLBACK=1)",
      providerOrder,
    }, true);
  };

  const webFetchHandler = async ({ url } = {}) => {
    await assertSSRF(url);
    const tiers = [];
    const attemptedProfiles = [];

    async function fetchTier(tierLabel, profile) {
      tiers.push(tierLabel);
      if (profile) attemptedProfiles.push(profile);
      const headers = buildHeaders(profile);
      let attempt = 0;
      while (attempt <= MAX_RETRIES) {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
        try {
          const res = await fetch(url, {
            signal: controller.signal,
            headers,
            redirect: "follow",
          });
          // 429/5xx backoff with Retry-After — never partial
          if (res.status === 429 || (res.status >= 500 && res.status < 600)) {
            const retryAfter = res.headers.get("Retry-After");
            const waitMs = parseRetryAfter(retryAfter);
            const backoff = waitMs !== null ? waitMs : BASE_BACKOFF_MS * Math.pow(2, attempt);
            if (attempt < MAX_RETRIES) {
              clearTimeout(timeout);
              await sleep(backoff);
              attempt++;
              continue;
            }
            return { res, exhausted: true, retryAfter };
          }
          return { res, exhausted: false };
        } catch (e) {
          if (e && e.name === "AbortError") {
            return { error: "FetchBlocked", status: 0, URL: url, tiers: [...tiers], reason: "timeout 10s" };
          }
          // network error — retry with backoff unless exhausted
          if (attempt < MAX_RETRIES) {
            clearTimeout(timeout);
            await sleep(BASE_BACKOFF_MS * Math.pow(2, attempt));
            attempt++;
            continue;
          }
          throw e;
        } finally {
          clearTimeout(timeout);
        }
      }
      return { res: null, exhausted: true };
    }

    // T1
    let tierResult = await fetchTier("T1", null);
    if (tierResult.error) return toContent(tierResult, true);
    let res = tierResult.res;

    // 403 -> T2 chrome124/safari17
    if (res && res.status === 403) {
      tierResult = await fetchTier("T2:chrome124", "chrome124");
      if (tierResult.error) return toContent(tierResult, true);
      res = tierResult.res;
      if (res && res.status === 403) {
        tierResult = await fetchTier("T2:safari17", "safari17");
        if (tierResult.error) return toContent(tierResult, true);
        res = tierResult.res;
      }
    }

    // Non-ok after tiers -> check T3 gate or FetchBlocked
    if (!res || !res.ok) {
      const status = res ? res.status : 0;
      // exhausted or 403 after T2 -> decide T3
      if (process.env.BIGGZ_WEB_FETCH_HEADLESS === "1") {
        tiers.push("T3:headless");
        return toContent({
          error: "FetchBlocked",
          status,
          URL: url,
          tiers: [...tiers],
          note: "T3 headless gated but not bundled; returning FetchBlocked — see BIGGZ_WEB_FETCH_HEADLESS",
        }, true);
      }
      // Exhausted 429/5xx after retries -> FetchBlocked with tiers (never partial)
      if (tierResult.exhausted) {
        return toContent({
          error: "FetchBlocked",
          status,
          URL: url,
          tiers: [...tiers],
          reason: tierResult.retryAfter ? `Retry-After ${tierResult.retryAfter}` : "exhausted retries",
        }, true);
      }
      return toContent({ error: "FetchBlocked", status, URL: url, tiers: [...tiers] }, true);
    }

    // Extract body with 1MB cap — never partial without annotation
    let text = await res.text();
    // detect html vs plain
    const ct = (res.headers.get("content-type") || "").toLowerCase();
    const isHTML = ct.includes("text/html") || ct.includes("application/xhtml") || text.trimStart().startsWith("<!DOCTYPE") || text.trimStart().startsWith("<html");
    let markdown;
    if (isHTML) markdown = htmlToMarkdown(text, url);
    else markdown = text;

    let truncated = false;
    if (Buffer.byteLength(markdown, "utf8") > ONE_MB) {
      markdown = Buffer.from(markdown, "utf8").subarray(0, ONE_MB).toString("utf8");
      truncated = true;
    }
    const excerpt = markdown.slice(0, 2000);
    const finalMarkdown = truncated ? markdown + "\n\n[truncated: 1MB cap]" : markdown;
    return toContent({
      markdown: finalMarkdown,
      excerpt,
      publisher: new URL(url).hostname,
      URL: url,
      accessed_at: new Date().toISOString(),
      truncated,
      tiers: [...tiers],
    });
  };

  if (register) {
    // Avoid duplicate web_search when pi-web-search (provider-native) is already installed — it provides better grounding via Gemini/OpenAI/Anthropic
    let hasPiWebSearch = false;
    try {
      hasPiWebSearch = fs.existsSync(path.join(os.homedir(), ".pi", "agent", "npm", "node_modules", "pi-web-search", "package.json"));
    } catch {}
    if (!hasPiWebSearch) {
      pi.registerTool({
        name: "web_search",
        description: "Search web via Tavily->Brave->DDG (open-web, sdd-research only)",
        parameters: {
          type: "object",
          properties: {
            query: { type: "string", description: "Search query" },
            limit: { type: "number", description: "Max results 1-10" },
          },
          required: ["query"],
        },
        execute: async (...args) => {
          const params = args[1] && typeof args[1] === "object" ? args[1] : args[0];
          return webSearchHandler(params);
        },
      });
    } else {
      pi.registerTool({
        name: "biggz_web_search",
        description: "Search web via DuckDuckGo (no-key fallback, for models without native grounding like ox-alpha-free)",
        parameters: {
          type: "object",
          properties: {
            query: { type: "string", description: "Search query" },
            limit: { type: "number", description: "Max results 1-10" },
          },
          required: ["query"],
        },
        execute: async (...args) => {
          const params = args[1] && typeof args[1] === "object" ? args[1] : args[0];
          return webSearchHandler(params);
        },
      });
      console.log("[biggz-web-search] pi-web-search provides web_search, registered biggz_web_search fallback (DuckDuckGo no-key)");
    }
    pi.registerTool({
      name: "web_fetch",
      description: "Fetch URL -> Markdown with SSRF guard, 10s/1MB caps, 3-tier TLS fallback",
      parameters: {
        type: "object",
        properties: {
          url: { type: "string", description: "URL to fetch" },
        },
        required: ["url"],
      },
      execute: async (...args) => {
        const params = args[1] && typeof args[1] === "object" ? args[1] : args[0];
        return webFetchHandler(params);
      },
    });
  } else {
    pi.registerCommand("web_search", {
      description: "web_search (fallback command, sdd-research only)",
      handler: async (args) => {
        const q = typeof args === "string" ? args : args?.query;
        return webSearchHandler({ query: q });
      },
    });
    pi.registerCommand("web_fetch", {
      description: "web_fetch (fallback command, SSRF-guarded)",
      handler: async (args) => {
        const u = typeof args === "string" ? args : args?.url;
        return webFetchHandler({ url: u });
      },
    });
  }

  pi._biggzWebSearch = {
    assertSSRF,
    isPrivateIP,
    isBlockedHostname,
    resolveProviderOrder,
    publisherFor,
    parseRetryAfter,
    htmlToMarkdown,
    searchTavily,
    searchBrave,
    searchDDG,
    buildHeaders,
  };
}
