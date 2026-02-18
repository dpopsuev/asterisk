# Contract — PoC Scope Precision

**Status:** draft  
**Goal:** Deliver a zero-friction PoC experience: a simplified `asterisk analyze <LAUNCH_ID>` command (no flags needed), a `/asterisk-analyze` Cursor skill, RP token guidance, help system, and a CEO-ready README.md. Another engineer can clone, build, and run an RCA in under 5 minutes.  
**Serves:** PoC completion

## Contract rules

- Global rules only.
- No internal Red Hat URLs in committed files (README, code). Use placeholder URLs or env vars.
- Token (`.rp-api-key`) must never appear in logs, error messages, or skill output.
- Output artifacts written with `0600` permissions (SEC-004 mitigation).
- This contract is packaging, not new functionality. The pipeline, adapters, and calibration are unchanged.

## Context

- **Current UX pain:** `asterisk analyze` requires `--launch` (required), `-o` (required), `--rp-base-url`, `--rp-api-key`. A new user must know all four flags.
- **Target UX:** `asterisk analyze 33195` or `/asterisk-analyze 33195` — that's it.
- **RP instance:** URL provided via `ASTERISK_RP_URL` env var or `--rp-base-url` flag (not hardcoded in binary).
- **RP token:** `.rp-api-key` file, already in `.gitignore`. Tool checks existence and permissions.
- **Adapter:** `basic` (default, zero-LLM heuristic, M19 = 0.93).
- **Workspace:** embedded from `ptp-real-ingest` scenario (6 repos).
- **Existing skill:** `asterisk-investigate` (signal-based, for calibration). New skill is command-based (for ad-hoc analysis).

## Execution strategy

### Phase 1 — Simplify `analyze` CLI

**Files:** `cmd/asterisk/cmd_analyze.go`, `cmd/asterisk/helpers.go`

- Accept launch ID as positional arg: `asterisk analyze 33195` (keep `--launch` for backward compat)
- Default RP URL from `ASTERISK_RP_URL` env var; error with guidance if neither env var nor flag is set
- Default output path: `.asterisk/output/rca-{launch_id}.json` (auto-create dir); `-o` becomes optional
- Token permission check: warn if `.rp-api-key` is world-readable (`perm & 0044 != 0`)
- Input validation: reject non-numeric, non-filepath launch IDs before building output path (SEC-001 mitigation)
- Lightweight token validation: `GET /health` call before running pipeline (fail fast on bad token)

### Phase 2 — `/asterisk-analyze` Cursor skill

**Files:** `.cursor/skills/asterisk-analyze/SKILL.md`

Command-style skill:

1. Parse input: if empty, "help", or non-numeric -> show help
2. Check if `bin/asterisk` exists; if not, build it
3. Check if `.rp-api-key` exists; if not, print RP token guide and stop
4. Check if `ASTERISK_RP_URL` is set; if not, guide the user
5. Run: `bin/asterisk analyze <LAUNCH_ID>`
6. Read output JSON artifact from `.asterisk/output/rca-{launch_id}.json`
7. Present human-friendly summary: defect type, component, confidence, evidence, RCA message
8. Offer push: `bin/asterisk push -f <artifact_path> --rp-base-url $ASTERISK_RP_URL`

### Phase 3 — README overhaul

- Copy `README.md` -> `README.md.post` (verbatim archive of full technical docs)
- Write new `README.md`: executive summary, quick start (build, token, analyze), sample output, how it works (F0-F6), Cursor skill usage, calibration summary, security section, link to README.md.post
- Placeholder URLs only: `https://your-rp-instance.example.com`

### Phase 4 — FSC updates

- `contracts/index.mdc` — add `poc-scope-precision.md` to draft section
- `contracts/current-goal.mdc` — add as gate tier
- `.cursor/skills/index.mdc` — add `asterisk-analyze/` entry

## Tasks

- [ ] **Simplify CLI** — positional arg, env var default, output path default, token check, input validation
- [ ] **Create skill** — `.cursor/skills/asterisk-analyze/SKILL.md` with help mode and security guardrails
- [ ] **README overhaul** — copy to `.post`, write new CEO-ready README
- [ ] **Update FSC** — contracts index, goal manifest, skills index
- [ ] Validate (green) — `go build`, `go test`, `go vet` all pass; `asterisk analyze --help` shows updated help
- [ ] Tune (blue) — refine README tone, skill wording. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** a fresh clone with Go 1.24+ and a valid `.rp-api-key`,
- **When** the user sets `ASTERISK_RP_URL` and runs `asterisk analyze 33195`,
- **Then** an RCA artifact is produced at `.asterisk/output/rca-33195.json` with defect type, component, confidence, and evidence refs — no other flags needed.

- **Given** `.rp-api-key` does not exist,
- **When** the user runs `asterisk analyze 33195`,
- **Then** a clear guide is printed explaining how to get and save the RP token.

- **Given** `ASTERISK_RP_URL` is not set and `--rp-base-url` is not provided,
- **When** the user runs `asterisk analyze 33195`,
- **Then** a clear error explains how to set the env var or use the flag.

- **Given** the user types `/asterisk-analyze 33195` in Cursor,
- **When** the skill runs,
- **Then** it builds (if needed), checks the token, runs the analysis, and presents a human-readable summary.

- **Given** the user types `/asterisk-analyze help` or `/asterisk-analyze` with no args,
- **When** the skill runs,
- **Then** it prints usage instructions including the token setup guide.

## Security assessment

Implement these mitigations when executing this contract.

| OWASP | Finding | Severity | Mitigation |
|-------|---------|----------|------------|
| A01 | Positional arg `<LAUNCH_ID>` used in output path `.asterisk/output/rca-{launch_id}.json`. Path traversal if input contains `../`. Extends SEC-001. | High | Validate launch ID: numeric-only via `strconv.Atoi` before building output path. Reject non-numeric input. Use `filepath.Clean` + directory containment check. |
| A02 | `ASTERISK_RP_URL` env var replaces hardcoded URL. Config-based, not baked into binary. | Resolved | RP URL never compiled into binary. Read from env var or flag only. |
| A02 | `.rp-api-key` plaintext file permissions not checked (SEC-002). | Medium | Check `os.Stat().Mode().Perm()`. Warn if world-readable (`& 0044 != 0`). |
| A04 | README.md could leak internal RP instance URL, project name, or CI infrastructure details. | Medium | README uses placeholder URLs only: `https://your-rp-instance.example.com`. Real URLs in env vars or gitignored config. |
| A05 | Output artifacts at `0600` (already fixed in Phase 1 hardening). | Resolved | `os.WriteFile(..., 0600)` applied to all production writes. |
| A07 | No RP token validation before pipeline run. Bad token causes cryptic 401 mid-pipeline. | Low | Lightweight health check after reading token. Fail fast with "Token invalid or expired." |
| A09 | Token must never appear in error messages, logs, or Cursor skill output. | Low | Audit all `fmt.Fprintf` paths. Never interpolate token value. |
| A10 | `ASTERISK_RP_URL` env var could point to an internal service on shared systems (SSRF). | Low | Acceptable risk for PoC (CLI runs locally). For MVP: add URL allowlist. |

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `rp-e2e-launch.md` | active | E2E baseline proves the analyze command works end-to-end |
| `poc-tuning-loop.md` | draft | Tuning results validate BasicAdapter accuracy claims in README |

## Notes

(Running log, newest first.)

- 2026-02-18 — Contract created. Packages the PoC for external consumption: simplified CLI, Cursor skill, CEO-ready README. Security assessment includes 8 OWASP findings with mitigations.
