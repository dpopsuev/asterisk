# Contract — Data Swipe

**Status:** complete  
**Goal:** All internal infrastructure hostnames are scrubbed from the entire git history so the repo is safe to remain public.  
**Serves:** PoC completion (public repo hygiene)

## Contract rules

- The pattern file with actual strings lives only in `.dev/` (gitignored). This contract must never reproduce actual internal hostnames.
- `git filter-repo --replace-text` rewrites all commits in one pass. All commit hashes change.
- Work on a fresh clone. Verify before force-pushing. Backup first.

## Context

An audit of the 71-commit, 301-file repo identified 8 pattern categories of internal infrastructure hostnames spread across ~12 files and ~9 commits. The contamination starts at the second commit in the repo history, so the entire history requires rewriting.

### What gets replaced

| Category | Replacement | Scope |
|----------|-------------|-------|
| RP instance hostname | Generic example domain | ~10 files, 8 commits |
| Jenkins CI hostname | Generic example domain | 1 file, 1 commit |
| Lab registry hostname | Generic example domain | 1 file, 1 commit |
| Lab cluster identifiers (spoke, hub) | Generic names (`spoke-cluster`, `hub-cluster`) | 4 files, 4 commits |
| CI notification email | Generic example email | 2 files, 2 commits |
| URL allowlist glob | Generic domain glob | 1 file, 1 commit |

### What is NOT sensitive (no action needed)

- Jira ticket IDs (`OCPBUGS-*`, `CNF-*`) — public Red Hat Jira
- GitHub PR references (`openshift/*`, `redhat-cne/*`) — public repos
- OCP versions, operator versions — public release data
- RP service git hashes — from public repos

## Execution strategy

1. Prepare pattern file and verification script in `.dev/` (gitignored)
2. Baseline verify — confirm detection of all affected files
3. Create backup mirror clone
4. Make a fresh clone for the rewrite operation
5. Run `git filter-repo --replace-text` with the pattern file
6. Verify: zero pattern matches in the rewritten history
7. Spot-check affected files for semantic correctness
8. Build and test (`go build ./...`, `go test ./...`)
9. Force-push rewritten master to origin
10. Fresh clone from origin, final verification

## Tasks

- [x] Install `git-filter-repo`
- [x] Create `.dev/data-swipe-patterns.txt` — replacement rules (gitignored)
- [x] Create `.dev/verify-swipe.sh` — verification script (gitignored)
- [x] Baseline verify — all 8 patterns detected in 12 files
- [x] Create this contract
- [x] Create backup mirror clone (`/tmp/asterisk-backup-20260219.git`)
- [x] Fresh clone, run filter-repo, verify clean (9 patterns, 0 leaks)
- [x] Build and test on rewritten repo (`go build` + `go test` — all pass)
- [x] Force-push rewritten history to origin/master (`2fcc3f3...9f5097c`)
- [x] Final verify on fresh clone (zero matches in tracked files + full history)
- [x] Add pre-commit guard rule (`.cursor/rules/data-hygiene.mdc` + `.git/hooks/pre-commit`)

## Acceptance criteria

- **Given** a fresh clone of the repo from origin,
- **When** `.dev/verify-swipe.sh` runs against it,
- **Then** exit code is 0 (zero matches for all 8 patterns).

- **Given** the rewritten repo,
- **When** `go build ./...` and `go test ./...` run,
- **Then** both pass with zero errors.

## Notes

- 2026-02-19 00:00 — Contract created. 8 patterns across 12 files, 9 commits. Pattern file and verify script in `.dev/` (gitignored). Baseline verification passes (all patterns detected).
- 2026-02-19 00:30 — Execution complete. 9 patterns (added minio-s3 during first pass QA), 72 commits rewritten, zero leaks in tracked files and full history. `go build` + `go test` pass. Force-pushed to origin. Pre-commit hook and data-hygiene rule installed. Backup mirror at `/tmp/asterisk-backup-20260219.git`.
