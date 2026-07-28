---
name: _shared
description: Shared SDD references for installed skills. Not invokable.
---

# Shared SDD Refs

This directory contains reference documents shared across all SDD skills.
These files are loaded by skills that need phase-common protocols, persistence
contracts, or OpenSpec conventions.

## Included

- `sdd-phase-common.md` — common protocol for all SDD phase skills

## Usage

Skills reference this directory via relative path `../_shared/<file>.md`
or by convention in the agent's overlay configuration.
