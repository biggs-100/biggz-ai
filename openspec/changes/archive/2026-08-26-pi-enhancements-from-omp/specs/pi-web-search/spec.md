# Delta for pi-web-search

## ADDED Requirements

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
