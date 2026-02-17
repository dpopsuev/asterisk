# Contract — batch-dispatch-protocol

**Status:** complete (2026-02-17)  
**Goal:** Define the batch signal protocol (manifest, briefing, concurrent semantics) that enables multi-subagent communication between the Go CLI and Cursor's Task-based parallel agents.

## Contract rules

- This is a **design contract** — deliverables are schemas and protocol documentation, not Go code.
- The protocol must be backward-compatible: single-agent mode (no manifest, no briefing) must continue to work exactly as today.
- All new file schemas must be JSON with documented fields, types, and lifecycle states.
- The protocol must not assume any specific number of subagents; it works with 1–N.
- Follow the generic-vs-scenario rule: the protocol is generic; Cursor Task tool constraints (max 4 subagents) are documented as a current-environment note, not baked into the protocol.

## Context

- **Current single-signal protocol**: `notes/dispatcher-protocol.mdc` — Go writes one `signal.json` per case dir, external agent reads prompt, writes artifact, Go polls. Per-case dirs already provide isolation (`.asterisk/calibrate/{suiteID}/{caseID}/signal.json`).
- **Current parallel Go infra**: `internal/calibrate/parallel.go` — worker pools, token semaphore, clustering. All workers share a single `ModelAdapter`. The adapter calls `Dispatch()` which blocks until one artifact appears.
- **Cursor Task tool**: launches up to 4 subagents in parallel within one conversation. Each subagent starts fresh (no shared memory), can read/write files, and returns a result to the parent. One level deep (subagents cannot spawn subagents).
- **FileDispatcher**: `internal/calibrate/dispatcher.go` — writes `signal.json`, polls for artifact with `dispatch_id` matching. One dispatcher instance per dispatch call.
- **Gap**: No coordination layer tells the skill "here are N cases waiting simultaneously." No shared context mechanism for subagents. No batch lifecycle tracking.

## Design

### Batch manifest

The **batch manifest** is the coordination file between the Go CLI and the Cursor skill. It lists all signals that are ready for processing in a single batch.

**Location**: `.asterisk/calibrate/{suiteID}/batch-manifest.json`

**Schema**:

```json
{
  "batch_id": 1,
  "status": "pending",
  "phase": "triage",
  "created_at": "2026-02-17T10:00:00Z",
  "updated_at": "2026-02-17T10:00:00Z",
  "total": 4,
  "briefing_path": ".asterisk/calibrate/1001/briefing.md",
  "signals": [
    {
      "case_id": "C1",
      "signal_path": ".asterisk/calibrate/1001/101/signal.json",
      "status": "pending"
    },
    {
      "case_id": "C2",
      "signal_path": ".asterisk/calibrate/1001/102/signal.json",
      "status": "pending"
    }
  ]
}
```

**Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `batch_id` | int64 | Monotonic batch counter within a calibration run |
| `status` | string | Batch lifecycle: `pending`, `in_progress`, `done`, `error` |
| `phase` | string | Pipeline phase: `triage` (F0+F1) or `investigation` (F2-F6) |
| `created_at` | string | ISO 8601 timestamp of batch creation |
| `updated_at` | string | ISO 8601 timestamp of last status change |
| `total` | int | Number of signals in this batch |
| `briefing_path` | string | Path to the shared briefing file for this batch |
| `signals` | array | Per-case signal entries |
| `signals[].case_id` | string | Ground-truth case ID |
| `signals[].signal_path` | string | Path to the case's `signal.json` |
| `signals[].status` | string | Per-signal status: `pending`, `claimed`, `done`, `error` |

**Lifecycle**:

```
pending → in_progress → done
pending → error (timeout, all signals failed)
```

- **pending**: Go CLI has written all signals and the manifest. Waiting for skill.
- **in_progress**: Skill has read the manifest and is spawning subagents.
- **done**: All signals in the batch have artifacts. Go CLI can proceed.
- **error**: Batch-level failure (timeout, unrecoverable errors).

### Briefing file

The **briefing** provides shared context that all subagents in a batch can read. It eliminates redundant reasoning by summarizing what is already known.

**Location**: `.asterisk/calibrate/{suiteID}/briefing.md`

**Content** (markdown, human-readable for agent consumption):

```markdown
# Calibration Briefing — Batch {batch_id}

## Run context
- Scenario: {scenario_name}
- Suite ID: {suite_id}
- Phase: {triage|investigation}
- Cases in this batch: {count}
- Total cases in run: {total}
- Completed so far: {completed_count}

## Known symptoms (from prior batches)
| Case | Category | Component | Defect Hypothesis | Severity |
|------|----------|-----------|-------------------|----------|
| C1   | product  | linuxptp-daemon | clock sync failure | high |
| C2   | environment | ptp-operator | deployment timeout | medium |

## Cluster assignments (investigation phase only)
| Cluster | Representative | Members | Key |
|---------|---------------|---------|-----|
| K1 | C1 | C1, C5, C9 | product|linuxptp-daemon|clock_sync |
| K2 | C4 | C4, C7 | environment|ptp-operator|deploy |

## Prior RCAs (from completed investigations)
| RCA ID | Component | Defect Type | Summary |
|--------|-----------|-------------|---------|
| R1 | linuxptp-daemon | pb001 | Clock servo fails to converge under load |

## Common error patterns
- `ptp4l timeout` — seen in 8/30 cases, associated with linuxptp-daemon
- `deployment failed` — seen in 6/30 cases, associated with ptp-operator
```

The briefing is regenerated by the Go CLI before each batch. It grows as more batches complete (later batches have richer context from earlier results).

### Concurrent signal semantics

Each case already has an isolated signal directory. The batch manifest adds a coordination layer on top:

1. **Go CLI writes N signals** to their respective case dirs (existing protocol, no change).
2. **Go CLI writes batch-manifest.json** listing all N signals.
3. **Skill parent reads manifest**, identifies pending signals.
4. **Skill spawns up to K subagents** (K <= min(N, 4) in current Cursor). Each subagent receives:
   - The briefing file path
   - One signal file path
   - The pipeline step and case context
5. **Each subagent reads its signal** (existing protocol), reads the prompt, reads the briefing, analyzes, writes the artifact (existing protocol).
6. **Skill parent waits** for all K subagents to return.
7. **Skill parent updates manifest** — marks processed signals as `done` or `error`.
8. **Go CLI polls** each artifact path independently (existing protocol). When all N artifacts are present, the batch is complete.
9. **Go CLI updates manifest status** to `done` and proceeds to the next batch (or next phase).

### Subagent response contract

Each Task subagent receives a self-contained prompt with:

```
You are analyzing case {case_id} at pipeline step {step}.

1. Read the briefing at {briefing_path} for shared context.
2. Read the signal at {signal_path} to find the prompt and artifact paths.
3. Read the prompt file at the signal's prompt_path.
4. Analyze the failure data in the prompt.
5. Write your JSON artifact to the signal's artifact_path.
   Wrap it: {"dispatch_id": {dispatch_id}, "data": {your artifact}}.

Do NOT read files under internal/calibrate/scenarios/, *_test.go, or .cursor/contracts/.
```

The subagent writes the artifact in the existing `artifactWrapper` format:

```json
{
  "dispatch_id": 7,
  "data": { ... step-specific artifact ... }
}
```

No changes to the artifact wrapper schema. The subagent follows the same contract as the single-agent watcher — the only difference is that N subagents run simultaneously instead of one agent processing sequentially.

### Backward compatibility

When `batch-manifest.json` does not exist:
- The skill falls back to single-signal watching (current behavior).
- The Go CLI falls back to sequential dispatch (current behavior).

This means the protocol is opt-in: `--dispatch=batch-file` enables it; `--dispatch=file` continues to work as before.

## Tasks

- [ ] **T1** Write `batch-manifest.json` schema documentation — fields, types, lifecycle states, example.
- [ ] **T2** Write briefing file content specification — sections, what each contains, how it grows across batches.
- [ ] **T3** Document concurrent signal semantics — step-by-step flow for Go CLI + skill + subagents.
- [ ] **T4** Document subagent response contract — what each Task subagent receives and must produce.
- [ ] **T5** Document backward compatibility — fallback behavior when manifest is absent.
- [ ] **T6** Add protocol to `notes/dispatcher-protocol.mdc` as a new "Batch dispatch" section.
- [ ] **T7** Add protocol to `.cursor/skills/asterisk-investigate/signal-protocol.md` as a new "Batch mode" section.
- [ ] Validate (green) — all schemas are internally consistent; manifest lifecycle covers all transitions; subagent contract matches existing artifact wrapper; backward compatibility is explicit.
- [ ] Tune (blue) — refine wording, add edge cases (partial batch failure, subagent timeout, stale manifest).
- [ ] Validate (green) — protocol review complete.

## Acceptance criteria

- **Given** the batch-manifest.json schema,
- **When** compared against the existing `signal.json` and `artifactWrapper` schemas,
- **Then** the batch manifest references existing signal paths (no duplication), and the subagent response uses the existing `artifactWrapper` format (no schema changes).

- **Given** the briefing file specification,
- **When** populated with data from a 30-case scenario after triage,
- **Then** it contains: run context, known symptoms, cluster assignments (if investigation phase), prior RCAs (if any), and common error patterns.

- **Given** a system running without `batch-manifest.json`,
- **When** the skill starts,
- **Then** it falls back to single-signal sequential watching (no errors, no behavior change).

## Dependencies

| Contract | Status | Required for |
|----------|--------|--------------|
| `fs-dispatcher.md` | Complete | Existing signal.json protocol to extend |
| `parallel-investigation.md` | Complete | Worker pool + clustering architecture |
| `cursor-skill.md` | Active | Existing skill to upgrade |
| `dispatcher-protocol.mdc` | Reference | Existing protocol docs |

## Notes

(Running log, newest first.)

- 2026-02-17 23:00 — Contract complete. Protocol documented in `notes/dispatcher-protocol.mdc` (Batch dispatch section) and `skills/asterisk-investigate/signal-protocol.md` (Batch mode section). All schemas, lifecycles, and concurrent semantics defined.
- 2026-02-17 22:00 — Contract created. Design-only contract defining the batch signal protocol for multi-subagent Cursor skill communication. Extends the existing per-case signal.json protocol with a coordination manifest and shared briefing file.
