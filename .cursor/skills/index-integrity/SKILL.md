---
name: index-integrity
description: Scan, validate, and enforce index.mdc compliance across .cursor/.
---

# Index Integrity

Scans every directory under `.cursor/`, validates its `index.mdc` against the index rules (`.cursor/meta.mdc`), and enforces compliance by creating missing indexes and fixing stale ones.

Unlike bootstrap (which only **creates** missing indexes and never touches existing ones), index-integrity **reads the filesystem and reconciles** — adding missing entries, removing stale entries, and creating absent indexes.

## When to invoke

- After adding, removing, or renaming files or directories under `.cursor/`.
- After bulk operations (new docs, new contracts, directory restructures).
- As a periodic hygiene pass.
- When bootstrap reports inconsistencies but doesn't fix them.

## Index rules (from meta.mdc)

1. Every `.cursor/` directory contains an `index.mdc`.
2. Indexes list **only** direct children — shallow, no nested trees.
3. Each entry = name + one-line description.
4. Empty directory → index with empty Directories and Files sections.
5. File add/remove/rename → parent `index.mdc` must reflect the change.

## Behavior

### Phase 1 — Scan

1. Recursively list every directory under `.cursor/`.
2. For each directory, list its direct children (files and subdirectories).
3. For each directory, read its `index.mdc` (if it exists).

### Phase 2 — Validate

For each directory, check: index exists; has appropriate structure (# Index, ## Directories, ## Files or equivalent); shallow; every direct child has an entry; every entry corresponds to an existing child; descriptions non-empty where expected.

### Phase 3 — Report

Print a summary before making changes.

### Phase 4 — Enforce

Missing index → create and populate. Malformed → rewrite preserving valid entries. Missing entry → add (generate description from frontmatter/heading). Stale entry → remove. Empty description → generate from content.

### Description heuristic

Frontmatter `description` → use it. First `#` heading or tagline → use it. Fallback → humanized filename. One short phrase; match existing terse style.

## Constraints

- **Never delete files or directories.** Only modify `index.mdc` files.
- **Preserve user-written descriptions** when valid.
- **Idempotent.** Second run = no changes.
- **Scope: `.cursor/` only.**
