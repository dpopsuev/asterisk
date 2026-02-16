---
name: bootstrap
description: Create/verify .cursor structure (project-agnostic).
disable-model-invocation: true
---

# /bootstrap

## Purpose
Create or verify the agreed `.cursor` directory structure and shallow indexes.

## Scope
- Create missing `.cursor` subdirectories (per `.cursor/meta.mdc`: rules, guide, docs, notes, contracts, security-cases, prompts, strategy, tactics, taxonomy, glossary, goals, skills).
- Create missing `index.mdc` files (shallow, direct children only).
- Add minimal `meta.mdc` and rules scaffolding if missing.
- Do **not** scan the repo or infer domain knowledge.
- Do **not** update existing indexes — use index-integrity skill for that.

## When run after files were added (idempotent, non-destructive)
- Create only **missing** structure (directories and indexes). Never overwrite existing files.
- Do **not** modify existing meta, rules, notes, docs, or other content.
- Do **not** update indexes for new files (that is index-integrity / survey scope).
- Optionally **report** missing or inconsistent structure; do not auto-fix content.
- Safe to run repeatedly; no destructive changes.
