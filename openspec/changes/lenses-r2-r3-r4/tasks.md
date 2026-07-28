# Tasks: Lenses R2-R4 (Readability, Reliability, Resilience)

## Review Workload Forecast

Decision needed before apply: No
400-line budget risk: Low
Chain strategy: stacked-to-main

## Tasks

### Step 1: Create internal/lens/gitdiff/
- [x] 1.1 Create `internal/lens/gitdiff/types.go` — DiffFile type (extracted from risk)
- [x] 1.2 Create `internal/lens/gitdiff/parse.go` — ParseDiffStat, DetectModeChanges (extracted from risk)
- [x] 1.3 Create `internal/lens/gitdiff/parse_test.go` — 9 tests
- [x] 1.4 Create `internal/lens/gitdiff/git.go` — GetDiffStat, HasModeChanges (extracted from risk)
- [x] 1.5 Update `internal/lens/risk/types.go` — use type alias for gitdiff.DiffFile
- [x] 1.6 Update `internal/lens/risk/lens.go` — import and use gitdiff package
- [x] 1.7 Update `internal/lens/risk/lens_test.go` — remove moved tests, keep risk-specific tests

### Step 2: Create internal/lens/readability/
- [x] 2.1 Create `internal/lens/readability/lens.go` — ReadabilityLens (ID: "readability", Name: "Readability Review")
- [x] 2.2 Implement file-length heuristics (>500 additions = warning, >200 = info)
- [x] 2.3 Implement naming convention checks (mixed case + underscores)
- [x] 2.4 Create `internal/lens/readability/lens_test.go` — 15 tests

### Step 3: Create internal/lens/reliability/
- [x] 3.1 Create `internal/lens/reliability/lens.go` — ReliabilityLens (ID: "reliability", Name: "Reliability Review")
- [x] 3.2 Implement missing test detection (_test.go coverage)
- [x] 3.3 Implement large change set detection and error-handling file path heuristics
- [x] 3.4 Create `internal/lens/reliability/lens_test.go` — 16 tests

### Step 4: Create internal/lens/resilience/
- [x] 4.1 Create `internal/lens/resilience/lens.go` — ResilienceLens (ID: "resilience", Name: "Resilience Review")
- [x] 4.2 Implement timeout detection (HTTP clients, transport layers)
- [x] 4.3 Implement context cancellation checks (db, repo, store paths)
- [x] 4.4 Implement concurrency checks (worker, pool, goroutine paths)
- [x] 4.5 Implement resource cleanup detection (file, stream, reader, writer, conn paths)
- [x] 4.6 Create `internal/lens/resilience/lens_test.go` — 15 tests

### Step 5: Wire in cmd/biggz/main.go
- [x] 5.1 Import all three new lens packages
- [x] 5.2 Register lenses in registry
- [x] 5.3 Add lens stages to pipeline (risk → readability → reliability → resilience → dummy)
