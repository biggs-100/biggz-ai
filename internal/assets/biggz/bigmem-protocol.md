<!-- biggz:bigmem-protocol -->
## BigMem Persistent Memory — Protocol

You have access to BigMem, a persistent memory system that survives across sessions and compactions.
This protocol is MANDATORY and ALWAYS ACTIVE — not something you activate on demand.

NOTE: Content wrapped in <private>...</private> is redacted to [REDACTED] before storage. Search previews are 300 chars — call biggz_mem_get_observation for full content.

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

### After user confirmation or rejection
- User confirms a recommendation you made ("go with that", "let's do that", "sounds good", "agreed", "perfect", or the equivalent in the user's language)
- User rejects an option or approach ("no, better X", "not that one", or the equivalent in the user's language)
- User expresses a preference ("I prefer X over Y", "always do it this way", or the equivalent in the user's language)
- User makes a decision after you presented tradeoffs or options
- A discussion concludes with a clear direction chosen — even if the agent proposed it

Self-check after EVERY task:
> "Did I or the user just make a decision, confirm a recommendation, express a preference, fix a bug, learn something non-obvious, or establish a convention? If yes, call biggz_mem_save NOW."

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

Before architecture-sensitive work, call biggz_mem_review with action list; do NOT call mark_reviewed automatically — only after explicit user confirmation. Search results may show state: needs_review and supersedes:/conflicts: annotations.

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

## CONFLICT SURFACING

After biggz_mem_save: if judgment_required, iterate candidates[] and call biggz_mem_judge
once per entry using that entry's judgment_id; never reuse the top-level judgment_id.
Ask conversationally when confidence < 0.7 OR (relation in
{supersedes, conflicts_with} AND type in {architecture, policy, decision}); else
resolve with related | compatible | scoped | not_conflict. Pass evidence from user reply.

## PROJECT PINNING & CURRENT PROJECT

Project is pinned via the nearest .biggz/config.json (or legacy .engram/config.json) inside the enclosing git repo — this file overrides cwd detection and blocks $HOME leakage. Call biggz_mem_current_project first when starting a new session to confirm project_source and project_path before writing; it never errors and surfaces available_projects when ambiguous.
<!-- /biggz:bigmem-protocol -->
