---
name: _shared
description: Shared SDD references for installed skills. Not invokable.
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
---

# Shared SDD Refs

This directory contains reference documents shared across all SDD skills.

## Included

- `sdd-phase-common.md` — common protocol for all SDD phase skills

## Usage

Skills reference this directory via relative path `../_shared/<file>.md` or by orchestrator injection.

## Conventions

- All artifacts under `openspec/` use Markdown with YAML frontmatter.
- Change names are kebab-case (`my-change`).
- Every change has a directory at `openspec/changes/{change-name}/`.
- Phase artifacts follow the pattern `openspec/changes/{change-name}/{phase}.md`.
- Spec files live at `openspec/specs/{domain}/spec.md`.
- The orchestrator reads `openspec/config.yaml` to determine required phases.
