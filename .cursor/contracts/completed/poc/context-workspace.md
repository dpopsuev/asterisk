# Contract — Context workspace (load and use)

**Status:** complete  
**Goal:** Load context workspace from FS (YAML/JSON/TOML), pass repo list and purpose into the analyze path, and document the format so CLI and templates stay in sync.

## Contract rules

- Global rules only. Format and semantics follow `docs/context-workspace.mdc` (repos with path/url, optional name, purpose, branch override).
- No Cursor API; workspace is config on FS only.

## Context

- **Purpose:** Context workspace lists repos (and optional purpose) for RCA; ref from envelope for branch/commit when no override. `docs/context-workspace.mdc`, `notes/pre-dev-decisions.mdc`.
- **PoC:** One external context source (e.g. GitHub); workspace = config file. Analyze takes workspace path and uses it to know which repos to reference in prompts.
- **Current:** No loading of workspace in code; only data structure documented.

## Execution strategy

1. Define a Go type (or use existing spec from docs) for workspace (repos: path, url, name, purpose, branch).
2. Implement loader: read from path, support YAML/JSON/TOML; return parsed workspace or error.
3. Pass workspace into analyze (or into the layer that builds prompt context): so artifact/prompt generation can list repos and purposes.
4. Add tests (fixture workspace file); document format in FSC if not already.
5. Validate; blue if needed.

## Tasks

- [x] **Define workspace type** — Struct matching context-workspace.mdc (repos with path/url, name, purpose, branch). Prefer single type for all formats.
- [x] **Loader** — LoadFromPath(path string) (Workspace, error); detect format by extension or content; support at least YAML and JSON for PoC.
- [x] **Wire into analyze** — Analyze (or caller) accepts optional workspace path; load and pass workspace into the flow so downstream can use repo list and purpose.
- [x] **Tests** — Unit test with fixture workspace file(s); test load and parse.
- [x] **Document** — Ensure docs/context-workspace.mdc (or data-io) describes format and example; link from CLI help or docs.
- [x] **Validate (green)** — Loader tests pass; analyze path receives workspace when path provided.
- [x] **Tune (blue)** — Clear boundaries; no behavior change.
- [x] **Validate (green)** — Tests still pass.

## Acceptance criteria

- **Given** a workspace file (YAML/JSON) at a path,
- **When** the loader is called with that path,
- **Then** a workspace value is returned with repos (path/url, optional name, purpose, branch) and can be passed into the analyze flow.
- **And** the format is documented and versionable.

## Notes

(Running log, newest first. YYYY-MM-DD HH:MM — decision or finding.)

- 2026-02-17 — Completed. Added `internal/workspace`: Workspace and Repo types; LoadFromPath(path) and Load(data, ext) with YAML/JSON; fixture tests. Wired into analyze via AnalyzeWithWorkspace(src, launchID, artifactPath, ws); Analyze calls it with nil. Updated docs/context-workspace.mdc with loader reference and Go type. All tests pass.
