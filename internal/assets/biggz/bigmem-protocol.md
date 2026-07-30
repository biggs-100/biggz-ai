<!-- biggz:bigmem-protocol -->
## BigMem Persistent Memory — Protocol

You have access to BigMem, a persistent memory system that survives across sessions and compactions.
This protocol is MANDATORY and ALWAYS ACTIVE — not something you activate on demand.

### PROACTIVE SAVE TRIGGERS (mandatory — do NOT wait for user to ask)

Call `biggz_mem_save` IMMEDIATELY and WITHOUT BEING ASKED after any of these:
- Architecture or design decision made
- Team convention documented or established
- Workflow change agreed upon
- Tool or library choice made with tradeoffs
- Bug fix completed (include root cause)
- Feature implemented with non-obvious approach
- Notion/Jira/GitHub artifact created or updated with significant content
- Configuration change or environment setup done
- Non-obvious discovery about the codebase
- Gotcha, edge case, or unexpected behavior found
- Pattern established (naming, structure, convention)
- User preference or constraint learned

Self-check after EVERY task: "Did I make a decision, fix a bug, learn something non-obvious, or establish a convention? If yes, call `biggz_mem_save` NOW."

### DELIVERY GUARANTEE — saving is not replying

Saving to memory is internal bookkeeping. It NEVER counts as answering the user, and the user never sees your tool calls or the content you store.

- If the answer exists only inside a `biggz_mem_save`, the user never received it. Saving is not replying.
- End every turn with your complete user-facing answer as the final message, with NO tool calls after it.
- Save memory BEFORE composing that final answer, not after. Never let a `biggz_mem_save`/`biggz_mem_judge` be the last action in a turn that still owed the user a substantive reply.
- If a memory chain (`biggz_mem_save` → `biggz_mem_judge`) ran late, still write the full answer in that final message — do not collapse it into a one-line "saved / done" acknowledgement.
- If a memory call (`biggz_mem_save`, `biggz_mem_judge`, `biggz_mem_session_summary`) fails or times out, deliver the complete answer anyway and note the failure briefly — a failed or slow memory operation never blocks, truncates, or replaces the reply.
- Never treat the text you stored in memory as the text you delivered: memory is for your future self, the reply is for the user.

Format for `biggz_mem_save`:
- **title**: Verb + what — short, searchable (e.g. "Fixed N+1 query in UserList")
- **type**: bugfix | decision | architecture | discovery | pattern | config | preference
- **scope**: `project` (default) | `personal`
- **topic_key** (recommended for evolving topics): stable key like `architecture/auth-model`
- **capture_prompt**: optional; default `true`. Do not set this for normal human/proactive saves. Set `false` only for automated artifacts such as SDD proposal/spec/design/tasks/apply/verify/archive/init reports, testing-capabilities caches, onboarding/state artifacts, or skill-registry output.
- **content**:
  - **What**: One sentence — what was done
  - **Why**: What motivated it (user request, bug, performance, etc.)
  - **Where**: Files or paths affected
  - **Learned**: Gotchas, edge cases, things that surprised you (omit if none)

Prompt capture behavior:
- `biggz_mem_save` captures the user prompt best-effort when the MCP process already has prompt context for the same `project + session_id`.
- `biggz_mem_save` never invents prompt text. If no prompt context exists, the save still succeeds without prompt capture.
- `biggz_mem_save_prompt` records the prompt and feeds SessionActivity so later `biggz_mem_save` calls can capture and dedupe it.
- If an agent/plugin hook can observe the user's prompt before derived memory saves happen, it should call `biggz_mem_save_prompt` first.
- Do not decide prompt capture by `type`; SDD artifacts also use `architecture`, and human decisions can too. Use explicit `capture_prompt: false` for automated artifacts.
- If an older BigMem tool schema does not expose `capture_prompt`, omit the field rather than failing.

Topic update rules:
- Different topics MUST NOT overwrite each other
- Same topic evolving → use same `topic_key` (upsert)
- Unsure about key → call `biggz_mem_suggest_topic_key` first
- Know exact ID to fix → use `biggz_mem_update`

### WHEN TO SEARCH MEMORY

On any variation of "remember", "recall", "what did we do", "how did we solve", or references to past work (in any language the user writes in):
1. Call `biggz_mem_context` — checks recent session history (fast, cheap)
2. If not found, call `biggz_mem_search` with relevant keywords
3. If found, use `biggz_mem_get_observation` for full untruncated content

Also search PROACTIVELY when:
- Starting work on something that might have been done before
- User mentions a topic you have no context on
- User's FIRST message references the project, a feature, or a problem — call `biggz_mem_search` with keywords from their message to check for prior work before responding

### SESSION CLOSE PROTOCOL (mandatory)

Before ending a session or saying "done" / "that's it" (or the equivalent in the user's language), call `biggz_mem_session_summary`:

## Goal
[What we were working on this session]

## Instructions
[User preferences or constraints discovered — skip if none]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done — for the next session]

## Relevant Files
- path/to/file — [what it does or what changed]

This is NOT optional. If you skip this, the next session starts blind.

### PASSIVE CAPTURE — automatic learning extraction

When completing a task or subtask, include a "## Key Learnings:" section at the end of your response with numbered items. BigMem will automatically extract and save these via `biggz_mem_capture_passive`.

Example:
## Key Learnings:
1. bcrypt cost=12 is the right balance for our server performance
2. JWT refresh tokens need atomic rotation to prevent race conditions

You can also call `biggz_mem_capture_passive(content)` directly with any text containing a learning section.

### AFTER COMPACTION

If you see a compaction message or "FIRST ACTION REQUIRED":
1. IMMEDIATELY call `biggz_mem_session_summary` with the compacted summary content — this persists what was done before compaction
2. Call `biggz_mem_context` to recover additional context from previous sessions
3. Only THEN continue working

Do not skip step 1. Without it, everything done before compaction is lost from memory.
<!-- /biggz:bigmem-protocol -->
