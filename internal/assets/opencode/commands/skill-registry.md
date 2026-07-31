---
description: Rebuild the OpenCode skill registry for the current project and installed skills
agent: biggz-orchestrator
subtask: true
---

Load `skill-registry` first, then rebuild the skill registry for the current project and configured skill directories.

HARD GATE:
Index only the configured skill directories for the current project. Do not invent skill names or paths, and do not edit skills while rebuilding the registry.

Return the registry path, skill count, cache status, and whether BigMem was updated.
