# Spec: engram-bigmem-import

## Purpose

One-way Engram -> BigMem migration. `biggz bigmem sync import --from-engram [--engram-dir PATH] [--project NAME]` reads `.engram/manifest.json` + `chunks/*.jsonl.gz`, maps `sync_id->ID`, filters, dedups, inserts into `bigmem.db`.

## Requirements — bigmem (NEW)

### Requirement: REQ-1 — Dispatch via --from-engram

System MUST import from Engram when `--from-engram` set; else MUST use default BigMem transport.

#### Scenario: Engram source

- GIVEN valid `.engram/manifest.json`
- WHEN `biggz bigmem sync import --from-engram` runs
- THEN manifest + `chunks/*.jsonl.gz` MUST be imported into `bigmem.db`

#### Scenario: Default unchanged

- GIVEN no `--from-engram`
- WHEN import runs
- THEN existing BigMem transport MUST be used

### Requirement: REQ-2 — --engram-dir

MUST accept `--engram-dir <PATH>` overriding default `.engram/`; else MUST resolve default.

#### Scenario: Custom dir

- GIVEN `--from-engram --engram-dir /tmp/.engram`
- WHEN import runs
- THEN chunks MUST be read from `/tmp/.engram`

### Requirement: REQ-3 — --project filter

With `--project NAME` MUST import only entities where `project==NAME` post-gunzip.

#### Scenario: Filtered

- GIVEN chunks with `biggz-ai` and `other`
- WHEN `--project biggz-ai` runs
- THEN only `biggz-ai` entities MUST be inserted

### Requirement: REQ-4 — sync_id -> ID

MUST map `engram.sync_id -> bigmem.ID`; ignore int64 `id`; empty `sync_id` MUST yield `engram-<sha256[0:12]>`.

#### Scenario: sync_id preserved

- GIVEN obs `sync_id="obs-abc123"`, `id=42`
- WHEN imported
- THEN `bigmem.ID` MUST be `"obs-abc123"`

#### Scenario: Fallback

- GIVEN empty `sync_id`
- WHEN imported
- THEN deterministic `engram-<hex>` MUST be used

### Requirement: REQ-5 — Idempotent dedup

Re-import MUST be no-op via `sync_chunks('engram:'+chunkID)`; inserts MUST use `ON CONFLICT DO NOTHING`.

#### Scenario: Re-import no-op

- GIVEN chunk already in `sync_chunks`
- WHEN re-imported
- THEN it MUST be skipped with no duplicates

### Requirement: REQ-6 — Missing manifest

Missing `manifest.json` MUST stderr + exit 1, zero DB mutations.

#### Scenario: Missing manifest

- GIVEN no `manifest.json` at dir
- WHEN import runs
- THEN stderr MUST mention `manifest.json` and exit 1

### Requirement: REQ-7 — Corrupt chunk

Corrupt gzip/JSON MUST warn per chunk, skip, continue others; exit 0 if any succeeds.

#### Scenario: Corrupt gzip

- GIVEN `chunks/bad.jsonl.gz` invalid gzip
- WHEN import runs
- THEN warn with chunk ID; other chunks MUST still import

### Requirement: REQ-8 — Pi untouched

MUST NOT modify `pi/` or TUI; `.engram/` MUST stay read-only; tracking only in `bigmem.db`.

#### Scenario: Pi unchanged

- GIVEN change applied
- WHEN `git diff -- pi/` checked
- THEN output MUST be empty

## Requirements — cli (MODIFIED)

### Requirement: REQ-CLI-1 — --from-engram flag

CLI MUST add boolean `--from-engram` to `biggz bigmem sync import`; routes to Engram path when set.

#### Scenario: Flag routes to Engram

- GIVEN `biggz bigmem sync import --from-engram`
- WHEN parsed
- THEN Engram handler MUST be invoked

### Requirement: REQ-CLI-2 — --engram-dir / --project flags

CLI MUST add `--engram-dir <path>` and `--project <name>`; forward to handler; help MUST list all three.

#### Scenario: Help lists flags

- GIVEN `biggz bigmem sync import --help`
- WHEN help renders
- THEN `--from-engram`, `--engram-dir`, `--project` MUST appear
