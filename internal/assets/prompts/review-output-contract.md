# Reviewer Output Contract

The materialized review task follows below. Everything above it is operator material — the shared guidance, your lens role, and these rules. Read all of it, then reply with the single JSON object described here and nothing else.

Your entire response must be exactly one JSON object: no prose before or after it, no Markdown code fences, no `<task_result>` wrapper, and no second copy of the shape. Exactly one object is accepted; any other response is a failed run that cannot be captured.

## Top-level keys

- `subject_hash`: copy the `subject_hash` value from the `GENTLE_AI_REVIEW_BINDING` line of the task verbatim. Never recompute, reformat, or omit it.
- `inspection`: an object with `status` and `paths`:
  - `status`: the literal `completed`.
  - `paths`: every `path` from `GENTLE_AI_REVIEW_CONTEXT.changed_path_manifest`, each exactly once, in ascending order, with no paths added. Covering only part of the frozen manifest fails the inspection.
- `lens`: the selected lens name (`risk`, `readability`, `reliability`, `resilience`). It must match the binding.
- `findings`: an explicit array of finding objects. When the candidate is clean, send `[]` — never omit the key.
- `evidence`: a non-empty array of concrete inspection lines. Each line states what was inspected and what was observed (a file and line, or the sweep that supports an all-clear). Placeholders such as `none`, `n/a`, `tbd` or `pass`, claims that inspection was unavailable, and vague summaries are all rejected.

Unknown keys anywhere — top level, inside `inspection`, or inside a finding — are rejected outright. Add nothing.

## Findings

Each finding object uses exactly these keys:

- `id`: a stable unique identifier like `R1-001`: the selected lens prefix (`risk`→`R1`, `readability`→`R2`, `reliability`→`R3`, `resilience`→`R4`) plus a three-digit sequence. Never repeat an id.
- `lens`: the selected lens name.
- `location`: `path:line`, where `path` is one frozen manifest path exactly as written (repository-relative, forward slashes) and `line` is a positive integer. A location outside the frozen manifest is rejected.
- `severity`: one of `BLOCKER`, `CRITICAL`, `WARNING`, `SUGGESTION` (uppercase).
- `claim`: a non-empty description of the defect and its consequence.
- `proof_refs`: optional array of concrete references into the frozen patch sections that support the claim.
- `evidence_class`: `deterministic`, `inferential`, or `insufficient`. Required on every `BLOCKER` or `CRITICAL` finding.
- `causal_disposition`: `introduced`, `behavior-activated`, `worsened`, `pre-existing`, `base-only`, or `unknown`. Required on every `BLOCKER` or `CRITICAL` finding.

## Example shape

{"subject_hash":"sha256:...","inspection":{"status":"completed","paths":["internal/auth/token.go"]},"lens":"risk","findings":[{"id":"R1-001","lens":"risk","location":"internal/auth/token.go:42","severity":"CRITICAL","claim":"Token is read before the guard validates the caller.","proof_refs":["GENTLE_AI_REVIEW_PATCH internal/auth/token.go"],"evidence_class":"inferential","causal_disposition":"introduced"}],"evidence":["internal/auth/token.go:42 reads Token before the guard validates the caller"]}

The example illustrates the shape only — never repeat, quote, or copy its values.

## Clean result

When the frozen candidate has no findings, answer with `"findings":[]` plus at least one concrete evidence line describing the sweep. An all-clear still claims the candidate was inspected, so empty or placeholder evidence is rejected.

Respond with exactly one JSON object — nothing else.
