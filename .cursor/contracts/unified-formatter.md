# Unified Table Formatter

**Status:** complete
**Created:** 2026-02-17
**Rule:** `.cursor/rules/human-readable-output.mdc`

## Goal

Introduce a unified table formatter in `internal/format/` using the adapter pattern over `jedib0t/go-pretty/v6`, replacing all bespoke `strings.Builder` + `fmt.Sprintf` table construction.

## Library

**`jedib0t/go-pretty/v6`** (v6.7.8, MIT, pure Go, 3.4K stars).

Chosen over `olekukonko/tablewriter` (4.8K stars) and `charmbracelet/lipgloss` (terminal-only, no markdown) because:

- Same data, two formats: build once, call `.Render()` (ASCII) or `.RenderMarkdown()` (Markdown).
- Per-column alignment/width via `SetColumnConfigs()` replaces hardcoded `%-4s %-30s %6.2f` patterns.
- Built-in styles (`StyleLight`) eliminate manual `===` / `---` separators.
- `text` package available for future colored CLI output.

## Adapter Pattern

```
internal/format/
├── format.go       -- Mode enum (ASCII, Markdown), TableBuilder interface, go-pretty adapter
├── helpers.go      -- Shared helpers: FmtTokens, FmtDuration, Truncate, BoolMark
└── format_test.go  -- Unit tests (9 tests: ASCII, Markdown, footer, columns, dual-format, helpers)
```

### Interface

```go
type Mode int
const (
    ASCII    Mode = iota
    Markdown
)

type TableBuilder interface {
    Header(cols ...string)
    Row(vals ...any)
    Footer(vals ...any)
    Columns(cfgs ...ColumnConfig)
    String() string
}

func NewTable(m Mode) TableBuilder
```

### Helpers (moved from calibrate)

| Helper | From | To |
|--------|------|----|
| `fmtTokens` | `calibrate/tokimeter.go` | `format.FmtTokens` |
| `fmtDuration` | `calibrate/tokimeter.go` | `format.FmtDuration` |
| `truncate` | `calibrate/report.go` | `format.Truncate` |
| `boolMark` | `calibrate/report.go` | `format.BoolMark` |

## Refactored Call Sites

### Markdown tables (`format.Markdown`)

| File | Function | Tables |
|------|----------|--------|
| `internal/calibrate/tokimeter.go` | `FormatTokiMeter` | Summary (2-col), Per-case (9 cols), Per-step (6 cols + footer) |
| `internal/calibrate/briefing.go` | `GenerateBriefing` | Known symptoms (5 cols), Clusters (4 cols), Prior RCAs (4 cols) |

### ASCII tables (`format.ASCII`)

| File | Function | Tables |
|------|----------|--------|
| `internal/calibrate/report.go` | `FormatReport` | Metrics sections (6 cols), Per-case breakdown (8 cols) |
| `internal/calibrate/analysis.go` | `FormatAnalysisReport` | Per-case breakdown (7 cols) |
| `internal/calibrate/tokens.go` | `FormatTokenSummary` | Token & cost summary (2-col KV) |

### Out of scope

- `StdinDispatcher.Dispatch` banner (decorative box, not tabular).
- CLI status lines (`runPush`, `runSave`, `runStatus`) — single key-value prints.
- `[file-dispatch]` / `[responder]` log lines — operational logs.

## Completion Criteria

- [x] `go-pretty/v6` dependency added.
- [x] `internal/format/` package created with interface, adapter, helpers.
- [x] Unit tests for format package (9 tests, all pass).
- [x] All Markdown call sites refactored (tokimeter, briefing).
- [x] All ASCII call sites refactored (report, analysis, tokens).
- [x] All existing tests pass (`go test ./...` — 0 failures).
- [x] Old local helpers removed from `calibrate` package.
