---
id: workspace-context
title: Workspace Context
description: Local repos as source of truth; version alignment
labels: [asterisk, domain]
---

# Workspace Context

## Local-first

- All repos cloned in Cursor workspace. Zero-latency access, full-text search, consistent version pinning.
- Workspace is source of truth. Read from disk; do not guess.

## Scope

- Workspace includes Asterisk + RP API research. Repos/paths in `notes/cursor-workspace-structure.mdc`.

## Agent behavior

- Reference workspace content for structure, endpoints, behavior.
- Use workspace folders for locked version (RP 5.11 / 24.1).

## Version alignment

- `report-portal/service-api` at **5.11.x**. See `contracts/rp-api-research.md`, `notes/poc-constraints.mdc`.
