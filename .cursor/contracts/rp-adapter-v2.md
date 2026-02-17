# Contract — RP Adapter v2 (reference: report-portal-cli)

**Status:** active  
**Goal:** Unify `rpfetch` + `rppush` into a single `rp` package with a scope-based client, proper error types, context support, structured logging, and richer type coverage — using `report-portal-cli` (`/home/dpopsuev/Repositories/report-portal-cli/`) as the architectural reference. No external dependency on the CLI module; patterns and types are adapted, not imported.

## Contract rules

- Global rules only.
- RP API version locked to **5.11** (RP 24.1). All endpoints and response shapes must match the OpenAPI spec at `report-portal-cli/pkg/internal/openapi/openapi.yaml`.
- Do **not** add `report-portal-cli` as a Go module dependency. Asterisk owns its own RP client. The CLI repo is a **read-only reference** for patterns, types, and API correctness.
- Existing consumers (`cmd/asterisk/main.go`, `internal/calibrate/*`) must continue to work. Migration is incremental: new package `internal/rp` coexists with old `rpfetch`/`rppush` until callers are migrated, then old packages are removed.
- All external HTTP calls must have `context.Context`, timeouts, and retry with backoff per `rules/project-standards.mdc`.

## Context

- **Reference repo:** `report-portal-cli` at `/home/dpopsuev/Repositories/report-portal-cli/`
  - Module: `github.com/klaskosk/report-portal-cli`
  - Key files: `pkg/client/client.go` (scope-based client + functional options), `pkg/client/launch.go` (LaunchScope), `pkg/client/errors.go` (APIError + predicates), `pkg/types/types.gen.go` (generated RP types), `pkg/types/epochmillis.go` (timestamp handling), `pkg/internal/openapi/openapi.yaml` (5.11 spec).
- **Current state (Asterisk):**
  - `internal/rpfetch/client.go` — hand-rolled HTTP, minimal `rpLaunch`/`rpItem` structs, no context, no error types, manual URL building, no logging.
  - `internal/rppush/client.go` — separate client, duplicated `Config`, manual HTTP, no structured errors.
  - `internal/preinvest/envelope.go` — minimal `Envelope` and `FailureItem` (6 fields each).
- **Completed contracts:** `rp-api-completion.md` (research), `rp-fetch.md` (basic fetch), `rp-push.md` (basic push). All complete — this contract upgrades the implementation quality.
- **FSC:** `notes/rp-api-usage.mdc` (endpoints, auth, mapping).

## Gap analysis: current vs reference

| Concern | Current (Asterisk) | Reference (report-portal-cli) | Gap |
|---------|-------------------|-------------------------------|-----|
| **Client structure** | Two separate packages (`rpfetch`, `rppush`), each with own `Config`/`Client` | Single client, scope-based: `Client → ProjectScope → LaunchScope` | Unify into single `internal/rp` package |
| **Construction** | `NewClient(Config)` with flat struct | `New(baseURL, token, opts...)` with functional options | Adopt functional options pattern |
| **Error handling** | `fmt.Errorf` with status body string | `APIError` type with `StatusCode()`, `ErrorCode()`, `Message()`; predicates `IsNotFound`, `IsUnauthorized`, etc. | Add structured error type |
| **Context** | None (`http.NewRequest` without context) | `context.Context` on all operations | Add context to all methods |
| **Logging** | None | `log/slog` with operation/project context | Add structured logging |
| **Types** | Minimal hand-rolled (`rpLaunch` 8 fields, `rpItem` 10 fields) | Full generated types (`LaunchResource` 20+ fields, `TestItemResource` 25+ fields, `Issue`, `IssueDefinition`) | Expand types to match RP response shapes |
| **Envelope mapping** | Basic: `id`, `uuid`, `name`, `type`, `status`, `path` | N/A (CLI doesn't map to envelope) | Enrich `FailureItem` with `codeRef`, `issue`, `parent`, `pathNames`, `description` |
| **Test items** | List by launchId, filter by status | Generated endpoint: `GetTestItemsV2UsingGET`, `GetNestedItemsUsingGET` + filter params | Add nested items, item-by-ID |
| **Logs** | Not implemented | Generated endpoint: `GetLogsUsingGET`, `SearchLogsUsingPOST` | Add log fetching for RCA context |
| **Defect update** | Single item PUT | Generated: bulk `PUT /item` with `IssueDefinition[]` | Add bulk defect update |
| **Timestamps** | `int64` (raw) | `EpochMillis` custom type (auto-detects ms vs μs) | Adopt EpochMillis pattern |
| **HTTP client** | `http.DefaultClient` hardcoded | Configurable via `WithHTTPClient` option | Allow injection for testing |
| **Pagination** | Manual loop in `fetchItems` | Filter params as functional options (`WithPageSize`, `WithPageNumber`, `WithSort`) | Adopt functional option pagination |

## Design

### Package layout

```
internal/rp/
├── client.go        # Client, New(), functional options, auth
├── project.go       # ProjectScope
├── launch.go        # LaunchScope: Get, List
├── item.go          # ItemScope: List, Get, UpdateDefect (single + bulk)
├── log.go           # LogScope: List, Search (stretch goal for PoC)
├── errors.go        # APIError, IsNotFound, IsUnauthorized, etc.
├── types.go         # RP response types (hand-written, aligned with generated)
├── envelope.go      # MapToEnvelope: LaunchResource + []TestItemResource → preinvest.Envelope
├── client_test.go   # Unit tests with httptest.Server
└── doc.go           # Package documentation
```

### Client API (modeled after report-portal-cli)

```go
// Construction
client, err := rp.New(baseURL, token,
    rp.WithHTTPClient(customClient),
    rp.WithLogger(slog.Default()),
    rp.WithTimeout(30 * time.Second),
)

// Scoped operations
launch, err := client.Project("ecosystem-qe").Launches().Get(ctx, 33195)
items, err := client.Project("ecosystem-qe").Items().List(ctx,
    rp.WithLaunchID(33195),
    rp.WithStatus("FAILED"),
    rp.WithPageSize(200),
)
err = client.Project("ecosystem-qe").Items().UpdateDefect(ctx, itemID, "pb001")
err = client.Project("ecosystem-qe").Items().UpdateDefectBulk(ctx, definitions)

// Convenience: fetch + map to envelope (replaces rpfetch.Client.FetchEnvelope)
env, err := client.Project("ecosystem-qe").FetchEnvelope(ctx, 33195)
```

### Error type (adapted from report-portal-cli)

```go
type APIError struct {
    operation  string
    statusCode int
    errorCode  int
    message    string
}

func IsNotFound(err error) bool       // 404
func IsUnauthorized(err error) bool   // 401
func IsForbidden(err error) bool      // 403
func HasStatusCode(err error, code int) bool
```

### Types (hand-written, aligned with generated)

Not importing `report-portal-cli/pkg/types` to avoid the dependency. Instead, define Asterisk-owned types that match the RP 5.11 response shapes needed for fetch, investigate, and push. Reference `types.gen.go` for field names and JSON tags.

**Core types needed:**

| Type | Fields from reference | Used by |
|------|----------------------|---------|
| `LaunchResource` | Id, Uuid, Name, Number, Status, StartTime, EndTime, Attributes, Description, Owner, Statistics | FetchEnvelope, Launch.Get |
| `TestItemResource` | Id, Uuid, Name, Type, Status, LaunchId, CodeRef, Issue, Parent, Path, PathNames, StartTime, EndTime, Description, Attributes, Statistics | Items.List, FetchEnvelope |
| `Issue` | IssueType, Comment, AutoAnalyzed, ExternalSystemIssues | Items.UpdateDefect, envelope mapping |
| `IssueDefinition` | Issue, TestItemId | Items.UpdateDefectBulk |
| `ItemAttributeResource` | Key, Value, System | Launch/item attributes |
| `StatisticsResource` | Defects, Executions | Launch/item statistics |
| `EpochMillis` | (custom time type) | All timestamps |

### Enriched envelope

```go
type FailureItem struct {
    // existing
    ID     int    `json:"id"`
    UUID   string `json:"uuid"`
    Name   string `json:"name"`
    Type   string `json:"type"`
    Status string `json:"status"`
    Path   string `json:"path"`

    // new (from TestItemResource)
    CodeRef      string `json:"code_ref,omitempty"`       // source file reference
    Description  string `json:"description,omitempty"`    // item description/error message
    ParentID     int    `json:"parent_id,omitempty"`      // parent item ID (suite/story)
    IssueType    string `json:"issue_type,omitempty"`     // current defect type in RP
    IssueComment string `json:"issue_comment,omitempty"`  // current defect comment
}
```

### Migration path

1. Create `internal/rp` alongside existing `rpfetch`/`rppush`.
2. Add a `FetchEnvelope(ctx, launchID)` convenience method that replaces `rpfetch.Client.FetchEnvelope`.
3. Add `UpdateDefect(ctx, itemID, defectType)` that replaces `rppush.Client.UpdateItemDefectType`.
4. Update `cmd/asterisk/main.go` callers to use `internal/rp`.
5. Remove `internal/rpfetch` and `internal/rppush`.

## Execution strategy

1. Create `internal/rp` package with client, errors, and types — no callers yet.
2. Implement LaunchScope (Get, List) and ItemScope (List, Get, UpdateDefect, UpdateDefectBulk).
3. Add FetchEnvelope convenience method with enriched FailureItem mapping.
4. Write tests against httptest.Server (port existing tests from rpfetch/rppush).
5. Migrate callers in main.go.
6. Remove old rpfetch/rppush packages.
7. (Stretch) Add LogScope for fetching logs per item.

## Tasks

- [ ] **Package scaffold** — Create `internal/rp/` with `doc.go`, `client.go` (New + functional options), `errors.go` (APIError + predicates), `types.go` (LaunchResource, TestItemResource, Issue, IssueDefinition, EpochMillis, ItemAttributeResource, StatisticsResource).
- [ ] **LaunchScope** — `project.go` (ProjectScope), `launch.go` (LaunchScope: Get by ID, List with functional option filters). Reference `report-portal-cli/pkg/client/launch.go` for pattern.
- [ ] **ItemScope** — `item.go` (ItemScope: List with filters [launchId, status, pageSize], Get by ID, UpdateDefect single, UpdateDefectBulk). Reference `report-portal-cli/pkg/types/types.gen.go` for `IssueDefinition` shape.
- [ ] **FetchEnvelope** — `envelope.go`: `ProjectScope.FetchEnvelope(ctx, launchID)` → `*preinvest.Envelope`. Maps LaunchResource + []TestItemResource to enriched Envelope/FailureItem. Replaces `rpfetch.Client.FetchEnvelope`.
- [ ] **Enriched FailureItem** — Add `CodeRef`, `Description`, `ParentID`, `IssueType`, `IssueComment` to `preinvest.FailureItem`. Update envelope mapping.
- [ ] **Tests** — Unit tests with `httptest.Server`: launch get/list, item list/get/update, envelope mapping, error predicates, pagination. Port and extend existing tests from `rpfetch/client_test.go` and `rppush/client_test.go`.
- [ ] **Migrate callers** — Update `cmd/asterisk/main.go` (`runAnalyze`, `runPush`, `runCursor`, `loadEnvelopeForAnalyze`, `loadEnvelopeForCursor`) to use `internal/rp`. Update `rpfetch.ReadAPIKey` → `rp.ReadAPIKey`.
- [ ] **Remove old packages** — Delete `internal/rpfetch/` and `internal/rppush/` once all callers are migrated.
- [ ] **LogScope (stretch)** — `log.go`: `ItemScope.Logs(ctx, itemID, opts...)` → log entries for RCA context. Reference `report-portal-cli/pkg/internal/api/api.gen.go` for log endpoints. Not required for PoC but valuable for enriched investigation prompts.
- [ ] Validate (green) — all tests pass, `asterisk analyze`, `asterisk push`, `asterisk cursor` work with new client, calibration still passes 20/20.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

- **Given** the new `internal/rp` package,
- **When** `asterisk analyze --launch=33195` is run,
- **Then** it fetches the launch and items via the new scope-based client with proper `context.Context`, structured error handling, and returns an enriched envelope.

- **Given** a network error or 404 from RP,
- **When** the client receives the error,
- **Then** it returns an `APIError` that passes `rp.IsNotFound(err)` or wraps the original error, and the caller can handle it without string matching.

- **Given** the old `rpfetch` and `rppush` packages,
- **When** migration is complete,
- **Then** they are deleted and no imports reference them.

- **Given** existing calibration tests,
- **When** the RP adapter is refactored,
- **Then** all calibration tests (stub 20/20 on all three scenarios) continue to pass unchanged, because calibration uses `MemStore` and `ModelAdapter`, not the RP client.

- **Given** the `report-portal-cli` repo in the workspace,
- **When** an implementer works on this contract,
- **Then** they reference `report-portal-cli/pkg/client/` for patterns (scope API, options, errors) and `report-portal-cli/pkg/types/types.gen.go` for RP response shapes — but do not import or depend on the module.

## Reference files (report-portal-cli)

| File | What to reference |
|------|------------------|
| `pkg/client/client.go` | `New()` construction, functional options (`Option`, `config`), auth via request editor, `WithHTTPClient`, `WithLogger` |
| `pkg/client/project.go` | `ProjectScope` pattern, scope chaining |
| `pkg/client/launch.go` | `LaunchScope`, `GetByID`, `GetByUUID`, `List`, `ListLaunchesOption` functional options for filters |
| `pkg/client/errors.go` | `APIError` type, `StatusCode()`, `ErrorCode()`, `Message()`, `Operation()`; predicate functions `IsNotFound`, `IsUnauthorized`, `IsForbidden`, `HasStatusCode`, `HasErrorCode` |
| `pkg/types/types.gen.go` | `LaunchResource`, `TestItemResource`, `Issue`, `IssueDefinition`, `IssueSubTypeResource`, `ItemAttributeResource`, `StatisticsResource`, `ErrorRS`, `IterableLaunchResource` — field names and JSON tags |
| `pkg/types/epochmillis.go` | `EpochMillis` type — auto-detect ms vs μs on unmarshal |
| `pkg/internal/openapi/openapi.yaml` | Canonical RP 5.11 OpenAPI spec — endpoint paths, query params, request/response schemas |

## Notes

(Running log, newest first. Use `YYYY-MM-DD HH:MM` — e.g. `2026-02-16 14:32 — Decision or finding.`)

- 2026-02-16 23:00 — Contract created. Gap analysis between Asterisk's `rpfetch`/`rppush` and `report-portal-cli` patterns. Decision: adapt patterns (scope API, functional options, APIError, EpochMillis) without importing the module. Enriched FailureItem adds codeRef, description, parentID, issueType, issueComment. LogScope is stretch goal.
