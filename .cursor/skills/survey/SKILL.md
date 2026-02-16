---
name: survey
description: Scan repo and spill key concepts into .cursor.
disable-model-invocation: true
---

# /survey

## Purpose
Scan the repo, identify core concepts, and spill them into `.cursor` context.

## Baseline behavior
- Detect core domains, architecture, and entry points.
- Identify key services/modules and their responsibilities.
- Extract build/test/run commands if discoverable.
- Populate `.cursor/notes` for summaries and `.cursor/docs` for deep references.
- Add new terms to `.cursor/glossary` if needed.
- Update all relevant `index.mdc`.

## Update behavior (git-aware)
- Read stored survey marker from `.cursor/notes/survey-state.mdc`.
- If `HEAD` unchanged, do nothing.
- If `HEAD` changed, compute diff size and update only impacted notes/docs.
- If no marker exists, perform full scan and write marker.
