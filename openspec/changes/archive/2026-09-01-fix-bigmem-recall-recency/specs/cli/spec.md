# Delta for cli

## ADDED Requirements

### Requirement: REQ-RR1-CLI — Recall/Recent Dispatch

CLI MUST dispatch `biggz recall` / `biggz bigmem recent` to `Search("", opts)`; forwards `--json|--limit|--type|--project|--scope`; caps `--limit` 50.

#### Scenario: Alias works

- GIVEN `biggz recall --json --limit 5`
- WHEN dispatched
- THEN returns `updated_at DESC`

#### Scenario: Flags forwarded

- GIVEN `recent --type session_summary --project biggz-ai --limit 10 --json`
- WHEN run
- THEN forwards all to `Search`

#### Scenario: Help documents

- GIVEN `recall --help` / `recent --help`
- WHEN rendered
- THEN lists `--json --limit --type --project` and recency note
