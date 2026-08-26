# Delta for pi-web-search

## ADDED Requirements

### Requirement: REQ-001 - web_search Provider Fallback

The system MUST expose `web_search(query, limit?)` in order `Tavily (TAVILY_API_KEY) -> Brave (BRAVE_API_KEY) -> DuckDuckGo`, MUST log order, and mark DDG as `publisher: DuckDuckGo (no-key fallback)`.

#### Scenario: Tavily first

- GIVEN `TAVILY_API_KEY` set
- WHEN `web_search("sdd research")` is called
- THEN it MUST call Tavily and not call Brave/DDG

#### Scenario: Fallback to DDG

- GIVEN no keys and `BIGGZ_DDG_FALLBACK=1`
- WHEN `web_search` is called
- THEN it MUST use DuckDuckGo with fallback publisher

#### Scenario: No provider

- GIVEN no keys and `BIGGZ_DDG_FALLBACK` unset
- WHEN `web_search` is called
- THEN it MUST return `blocked` with `Gaps: missing TAVILY_API_KEY`

### Requirement: REQ-002 - web_fetch Three-Tier + TLS

The system MUST expose `web_fetch(url)` via T1 `net/http`+headers, T2 `tls-client`/`utls` (`chrome124`/`safari17`), T3 headless gated by `BIGGZ_WEB_FETCH_HEADLESS=1`; T1 403 MUST trigger T2.

#### Scenario: 403 escalates

- GIVEN T1 returns 403
- WHEN T1 completes
- THEN it MUST invoke T2

#### Scenario: T3 gated off

- GIVEN `BIGGZ_WEB_FETCH_HEADLESS` unset
- WHEN T1 and T2 fail
- THEN it MUST skip headless and return `FetchBlocked`

#### Scenario: T3 gated on

- GIVEN `BIGGZ_WEB_FETCH_HEADLESS=1` and T1/T2 403
- WHEN fetch continues
- THEN it MAY invoke headless before `FetchBlocked`

### Requirement: REQ-003 - Markdown Extract and Caps

The system MUST extract HTML->Markdown via `go-readability`/`html-to-markdown`/`turndown`, enforce 10s/1MB caps, and emit `publisher`, `URL`, `accessed_at`, `excerpt`.

#### Scenario: Extract success

- GIVEN HTML with article body
- WHEN extract finishes within caps
- THEN it MUST return Markdown excerpt with source fields

#### Scenario: Cap exceeded

- GIVEN Markdown exceeds 1MB
- WHEN extract finalizes
- THEN it MUST truncate to 1MB and annotate truncation

#### Scenario: Timeout

- GIVEN fetch exceeds 10s
- WHEN timer fires
- THEN it MUST abort with `FetchBlocked` and timeout evidence

### Requirement: REQ-004 - Backoff, Retry-After, FetchBlocked

The system MUST retry 429/5xx with exponential backoff, respect `Retry-After`, and return loud `FetchBlocked`; MUST NOT silently return partial.

#### Scenario: Retry-After honored

- GIVEN 429 with `Retry-After: 2`
- WHEN scheduling retry
- THEN it MUST wait >=2s

#### Scenario: Exhausted

- GIVEN all retries and tiers fail
- WHEN no tier remains
- THEN it MUST return `FetchBlocked` with status, URL, tiers

### Requirement: REQ-005 - Gating to sdd-research

Tools MUST appear only for `sdd-research` with `biggz-ai.sdd-research-capability/v1` `open-web` and (`TAVILY_API_KEY` or `BRAVE_API_KEY` or `BIGGZ_DDG_FALLBACK=1`); others MUST NOT see them.

#### Scenario: sdd-research allowed

- GIVEN `sdd-research` admitted with `open-web` and key set
- WHEN Pi resolves tools
- THEN `web_search`/`web_fetch` MUST be in allowlist

#### Scenario: Other lane denied

- GIVEN agent is `sdd-explore` or `sdd-apply`
- WHEN tools resolve
- THEN `web_search`/`web_fetch` MUST be absent

#### Scenario: No grant

- GIVEN `open-web` not granted
- WHEN `sdd-research` requests web tools
- THEN it MUST return `blocked` with no claims

### Requirement: REQ-006 - SSRF and Secret Handling

The system MUST block `localhost`, `127.0.0.0/8`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `169.254.0.0/16`, `::1`, `fe80::/10`, `fc00::/7` and schemes `file:`, `data:`, `ftp:`, `gopher:`. Keys MUST come from env only and MUST NOT be logged.

#### Scenario: Private IP blocked

- GIVEN `web_fetch("http://192.168.1.10/docs")`
- WHEN validation runs
- THEN it MUST reject with SSRF error and no fetch

#### Scenario: No key leak

- GIVEN keys are set
- WHEN provider order is logged
- THEN logs MUST NOT contain key values

### Requirement: REQ-007 - Evidence Observability

The system MUST log provider order and persist `biggz-ai.sdd-research/v1` claims mapped to IDs; `partial`/`blocked` MUST list `Gaps` and MUST NOT hallucinate.

#### Scenario: Auditable

- GIVEN `web_search` then `web_fetch` succeed
- WHEN research persists
- THEN BigMem and `research.md` MUST match on `publisher/URL/accessed_at`

#### Scenario: Partial gap

- GIVEN 1 of 3 questions has sources after `FetchBlocked`
- WHEN research finalizes
- THEN outcome MUST be `partial` with `Gaps` and no unvalidated claims

### Requirement: Anchor-Preserving Markdown Fetch

The system MUST preserve heading `id` anchors and heading hierarchy when converting HTML to Markdown in `internal/assets/pi/biggz-web-search.js`. Headings MUST be emitted as `# heading {#anchor}` (ATX style with `{#id}` suffix) when the source element carries an `id`. The readable-article extraction MUST retain anchor order as a document index (preservar índice). Both `web_search` result fetch and direct `web_fetch` MUST use the same anchor-preserving readability path. The system MUST enforce existing 10s timeout and 1MB caps; when truncation occurs it MUST annotate the output with truncation offset and the nearest preceding anchor (e.g., `[truncated: 1MB cap — offset at {#section-id}]`). The system MUST NOT throw on malformed HTML; it MUST return best-effort Markdown and MUST be covered by fixture-HTML tests (no network).

#### Scenario: Fixture HTML preserves anchors

- GIVEN fixture HTML with `<h2 id="install">Install</h2>` and `<h3 id="usage">Usage</h3>`
- WHEN fetch extraction completes
- THEN Markdown MUST contain `## Install {#install}` and `### Usage {#usage}` in original order

#### Scenario: Truncation annotates anchor offset

- GIVEN extracted Markdown exceeds 1MB and nearest anchor before cut is `{#api}`
- WHEN truncation is applied
- THEN output MUST be capped at 1MB and end with annotation containing `{#api}`

#### Scenario: Malformed HTML does not throw

- GIVEN HTML with unclosed tags and duplicate ids
- WHEN extraction runs
- THEN it MUST return Markdown without throwing and preserve at least one anchor

#### Scenario: Shared path for web_search and web_fetch

- GIVEN same HTML fixture fetched via `web_search` follow-up and via `web_fetch`
- WHEN both complete extraction
- THEN both outputs MUST contain identical anchor-preserved headings

#### Scenario: Relative links resolved with baseUrl

- GIVEN Markdown extraction with `baseUrl: https://example.com/docs`
- WHEN content has `<a href="/guide">`
- THEN output MUST contain `[text](https://example.com/guide)`

