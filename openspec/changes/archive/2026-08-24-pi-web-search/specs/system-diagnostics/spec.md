# Delta for system-diagnostics

## ADDED Requirements

### Requirement: REQ-DIAG-001 — Pi Web Search Health Check

The system MUST provide `PiWebSearchCheck` that verifies file and env prerequisites only — no live network probe. `ID` MUST be `pi-web-search`. The check MUST inspect `~/.pi/agent/extensions/biggz-web-search.js` existence and env `TAVILY_API_KEY`/`BRAVE_API_KEY`/`BIGGZ_DDG_FALLBACK`/`BIGGZ_WEB_FETCH_HEADLESS`. It MUST return `pass` when file present and at least one provider is configured, `warn` when file present but no provider, and `fail` when file is absent. Severity MUST map via `Report` buckets (fail→CRITICAL, warn→WARNING, pass→INFO). `Remedy` MUST be `biggz install --agent pi`.

#### Scenario: File missing — fail

- GIVEN `~/.pi/agent/extensions/biggz-web-search.js` does not exist
- WHEN `PiWebSearchCheck.Run()` executes
- THEN `Result.Status` MUST be `fail` with CRITICAL severity and message containing the expected path

#### Scenario: File present with Tavily key — pass

- GIVEN the extension file exists and `TAVILY_API_KEY` is set
- WHEN `PiWebSearchCheck.Run()` executes
- THEN `Result.Status` MUST be `pass` with INFO severity

#### Scenario: File present, no keys, DDG fallback off — warn

- GIVEN the extension file exists and no `TAVILY_API_KEY`/`BRAVE_API_KEY`/`BIGGZ_DDG_FALLBACK=1` is set
- WHEN `PiWebSearchCheck.Run()` executes
- THEN `Result.Status` MUST be `warn` with WARNING severity and message hinting `TAVILY_API_KEY` or `BIGGZ_DDG_FALLBACK=1`

#### Scenario: No live probe

- GIVEN network is unavailable but file and env are valid
- WHEN `PiWebSearchCheck.Run()` executes
- THEN it MUST NOT attempt HTTP calls and MUST return based solely on file+env

#### Scenario: Remedy executes atomically

- GIVEN `PiWebSearchCheck` declares `Remedy(ID=pi-web-search, Description=install pi web search)`
- WHEN `doctor --fix` iterates remedies and calls `Action()`
- THEN it MUST invoke `biggz install --agent pi` (via `DeployPiWebSearch`) atomically and return error on failure

#### Scenario: Runner panic isolation

- GIVEN `Runner` includes `PiWebSearchCheck` among other checks and one check panics
- WHEN `Runner.Run()` completes
- THEN `PiWebSearchCheck` result MUST still be present with correct status and other checks MUST be unaffected

#### Scenario: Headless flag visibility

- GIVEN `BIGGZ_WEB_FETCH_HEADLESS=1` is set
- WHEN `PiWebSearchCheck.Run()` executes
- THEN the result message SHOULD note headless tier is enabled (without affecting pass/warn threshold)

### Requirement: REQ-DIAG-002 — Doctor Runner Registration

The system MUST register `PiWebSearchCheck` in `cmd/biggz/cli_doctor_help.go` `doctorRun()` check slice alongside `PiSubagentsCheck` and `PiLastModelCheck`, respecting panic isolation and `--json`/`--fix` flags.

#### Scenario: Doctor lists pi-web-search

- GIVEN `biggz doctor` is invoked
- WHEN checks complete
- THEN output MUST include a row with ID `pi-web-search` and its status icon

#### Scenario: JSON output includes check

- GIVEN `biggz doctor --json` is invoked
- WHEN checks complete
- THEN JSON MUST contain `pi-web-search` entry with status and severity

#### Scenario: --fix invokes remedy then re-checks

- GIVEN `biggz doctor --fix` and `PiWebSearchCheck` is in warn/fail
- WHEN remedies execute
- THEN `PiWebSearchCheck` remedy MUST run before final Report serialization
