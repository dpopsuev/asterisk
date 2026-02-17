# Contract: Cobra CLI Migration

**Priority:** HIGH — developer experience  
**Status:** Complete  
**Depends on:** None (can proceed independently; ideally after calibrate-decomposition)

## Goal

Replace the hand-rolled 880-LOC `cmd/asterisk/main.go` (flag + switch routing) with `spf13/cobra` for structured subcommands, automatic help, shell completion, and flag validation. Eliminate `orchestrate.BasePath` mutable global.

## Current State

- `cmd/asterisk/main.go`: 880 LOC
- Manual `os.Args` parsing with `switch` dispatch
- `flag.NewFlagSet` per subcommand
- `printUsage()` hand-written
- Business logic embedded: `createAnalysisScaffolding` (~50 LOC), adapter wiring, envelope loading
- `orchestrate.BasePath` set as global, read across the `orchestrate` package

## Target State

```
cmd/asterisk/
├── main.go           # cobra root command + Execute()
├── analyze.go        # cobra analyze subcommand
├── push.go           # cobra push subcommand
├── cursor.go         # cobra cursor subcommand
├── save.go           # cobra save subcommand
├── status.go         # cobra status subcommand
├── calibrate.go      # cobra calibrate subcommand
└── helpers.go        # shared flag binding, store opener, envelope loader
```

## Tasks

### Phase 1: Add Cobra dependency
1. `go get github.com/spf13/cobra@latest`
2. Create `cmd/asterisk/root.go` with root command and version info

### Phase 2: Migrate subcommands (one at a time)
For each of `analyze`, `push`, `cursor`, `save`, `status`, `calibrate`:
1. Create `cmd/asterisk/{name}.go` with `cobra.Command`
2. Move flag definitions from `flag.NewFlagSet` to `cmd.Flags()`
3. Move `RunE` logic, extracting business logic into packages
4. Wire into root command via `rootCmd.AddCommand()`
5. Test: `asterisk {name} --help` works, manual run produces same output

### Phase 3: Extract business logic
1. Move `createAnalysisScaffolding` into `preinvest` or a new `bootstrap` package
2. Move adapter wiring into `calibrate/adapt/` (after decomposition) or `wiring/`
3. Pass `basePath` as config parameter instead of setting `orchestrate.BasePath`
4. Eliminate all mutable globals from `orchestrate`

### Phase 4: Polish
1. Add shell completion: `asterisk completion bash/zsh/fish`
2. Add `--version` flag
3. Add `--verbose` / `--quiet` flags for log level control
4. Remove old `printUsage()` function
5. Run full test suite

## Estimated Impact

| Metric | Before | After |
|--------|--------|-------|
| `main.go` LOC | 880 | ~50 (just main + Execute) |
| Total `cmd/asterisk/` LOC | 880 | ~280 (split across 8 files) |
| Subcommand files | 1 | 8 |
| Manual flag parsing | Yes | No (Cobra handles) |
| Shell completion | No | Yes |
| Global state | `orchestrate.BasePath` | None |

## Completion Criteria
- All 6 subcommands work identically to current behavior
- `asterisk --help` shows proper usage
- Shell completion available
- `orchestrate.BasePath` global eliminated
- Total `cmd/asterisk/` LOC <= 350
- All tests pass

## Cross-Reference
- Architecture: `.cursor/docs/architecture.md`
- Off-the-shelf rule: `.cursor/rules/off-the-shelf.mdc`
- Calibrate decomposition: `.cursor/contracts/calibrate-decomposition.md` (adapter wiring moves)
