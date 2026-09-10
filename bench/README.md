# biggz-ceremony-bench

Measures workflow **friction** so a "before" binary and an "after" binary can
be compared and ceremony changes can be shown rather than asserted. Port of
gentle-ai's `bench/` pattern, scoped to biggz-ai ceremony behavior.

The corpus is a **black box**: it drives a `biggz` binary given by `--binary`
as a subprocess and never instruments the product, so it works against any
build including old releases. It is **deterministic and offline**: no model is
ever called. Every journey runs in a fresh temp directory with its own `HOME`
and never touches real config or repositories.

## Its own module, on purpose

`bench/` declares its own `go.mod` (stdlib only), so the root module's
`go build ./...`, `go vet ./...` and `go test ./...` do not see it. Build it
from inside this directory:

```
cd bench
go build -o biggz-ceremony-bench .
```

## Usage

```
bench run --binary /path/to/biggz --out results-after.json
bench run --binary /path/to/old-biggz --out results-before.json
bench compare --before results-before.json --after results-after.json
```

`run --only j01-settle-admits-without-acquire` runs a subset. `run` fails
closed on failed journeys; `compare` fails on any pass-to-fail regression.

## Journeys

| ID | Asserts |
|----|---------|
| j01 | settle with unknown token exits 0 with warning (admissible ledger) |
| j02 | settle `--strict` exits 1 with invalid_continuation (lock preserved) |
| j03 | session-close --check-only without summary exits 1 with gate token (fail-closed intact) |
| j04 | install deploys quiet-ceremony prompt markers |
