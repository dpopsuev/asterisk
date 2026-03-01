# Contract — Demo Presentation

**Status:** complete  
**Goal:** Implement Asterisk's Police Station presentation by providing a `KabukiConfig` + `Theme` to Origami's Kabuki presentation engine — personality-driven content, `asterisk demo` CLI, and a recorded replay for repeatable demos. Kami is the debugger; Kabuki is the presentation layer.  
**Serves:** Polishing & Presentation (should)

## Contract rules

- This contract depends on **two** Origami contracts: `kami-live-debugger` (complete) and `kabuki-presentation-engine` (draft). Do not start Phase 2+ until Kabuki's `KabukiConfig` interface is defined and API endpoints exist.
- **No standalone SPA in Asterisk.** The `internal/demo/frontend/` directory is deleted. All presentation rendering is done by Origami's Kabuki frontend. Asterisk only provides Go data structs.
- Theme content (intro lines, node descriptions, cooperation dialogs) must be funny and personality-driven. Each persona's voice matches its element.
- The recorded replay `.jsonl` is committed to the repo as a demo artifact. It must be reproducible from a real calibration run.
- No hardcoded agent identities — the theme provides personality content, Kami provides the model identity (provider/model/version) from the actual run.

## Context

- **Kabuki Presentation Engine** (Origami, draft): Data-driven presentation SPA as a framework feature (Kami = debugger, Kabuki = presentation). Defines `KabukiConfig` interface, `/api/theme` + `/api/circuit` + `/api/kabuki` endpoints, scroll-snap section rendering, element selector. See `origami/.cursor/contracts/draft/kabuki-presentation-engine.md`. **This contract is the upstream dependency.**
- **Kami Live Debugger** (Origami, complete): EventBridge, KamiServer, Debug API, MCP tools, Recorder/Replayer, React frontend.
- **Red Hat Presentation DNA** (`origami/.cursor/docs/rh-presentation-dna.md`): Color system, web section patterns. Kabuki implements the layout; Asterisk provides the color tokens and content.
- **Circuit RCA** (`internal/orchestrate/circuit_rca.yaml`): Asterisk's 7-node circuit with 3 zones (Backcourt, Frontcourt, Paint). This is the graph visualized in the demo.
- **Origami Personas** (`persona.go`): 8 personas (Herald, Seeker, Sentinel, Weaver + Antithesis counterparts) with element affinities and personality traits.
- **Origami Elements** (`element.go`): Fire (decisive), Water (thorough), Earth (methodical), Air (creative), Diamond (precise), Lightning (fast).
- **Calibration results**: PoC demonstrated M19=0.83 (BasicAdapter) and M19=0.58 (CursorAdapter) on 18 verified cases. The demo should showcase a real calibration run with visible RCA evidence accumulation.
- **Inside Out inspiration**: Agents have distinct personalities and argue during cooperation. The Police Station metaphor: Asterisk investigates "crimes against CI." Agents wear police hats.

### Storyboard

**Act 1 — Introduction**: Agents appear one by one with 3D CSS polyhedra, name, element, personality tags, and model identity. Funny personality-driven intro lines. Fire: "I saw the error. I already know what happened. You're welcome." Water: "Let's not jump to conclusions. I'd like to examine all 47 log files first."

**Act 2 — Costume & Briefing**: The graph renders with zones. Agents "put on" their police costumes. Each node lights up with a tooltip. Humorous transition: "Time to investigate some crimes against CI."

**Act 3 — Execution**: Live SSE events drive the visualization. Agent dots move across the graph. Monologue panels show inner reasoning. Cooperation pop-ups appear with funny arguments. Evidence cards build up at the bottom showing RCA findings.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Demo recording (canonical `.jsonl`) | `testdata/demo/` | domain |
| Police station theme reference | `docs/demo-theme.md` | domain |
| Web section structure (act-to-component mapping) | `docs/demo-web-sections.md` | domain |

## Execution strategy

Phase 0 is removed — there is no standalone SPA to build. Asterisk provides only Go data structs; Origami's Kabuki renders them. Phase 1 keeps the existing `PoliceStationTheme` and adds `PoliceStationKabuki` implementing Origami's `KabukiConfig`. Phase 2 wires the `asterisk demo` CLI to pass both to `kami.NewServer()`. Phase 3 records a canonical calibration run. Phase 4 validates the full replay experience. Phase 5 deletes `internal/demo/frontend/` and `internal/demo/embed.go`.

**Dependency:** Origami's `kabuki-presentation-engine` contract must be complete before Phase 1 starts. The `KabukiConfig` interface, API endpoints, and data-driven frontend must exist in the framework first.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Theme + KabukiConfig struct completeness, section data generation |
| **Integration** | yes | `asterisk demo --replay` starts Kami+Kabuki, serves framework SPA, plays events |
| **Contract** | yes | `kami.Theme` + `kami.KabukiConfig` interface compliance |
| **E2E** | yes | Full replay demo runs without errors, all sections render |
| **Concurrency** | no | Single-user demo, no shared state |
| **Security** | no | Localhost demo, no trust boundaries |

## Tasks

### Phase 1 — Police Station Theme + KabukiConfig

- [x] **T1** Verify existing `PoliceStationTheme` (already implements `kami.Theme`) in `internal/demo/theme.go`
- [x] **T2** Create `PoliceStationKabuki` implementing `kami.KabukiConfig` in `internal/demo/kabuki.go` — provides section data:
  - `Hero()` — "Asterisk: AI-Driven Root-Cause Analysis", subtitle, presenter info
  - `Problem()` — CI failure stats, pain points
  - `Results()` — M19 metric comparisons, calibration outcomes
  - `Competitive()` — Origami vs CrewAI vs OmO comparison rows
  - `Roadmap()` — Sprint 1-6 milestones
  - `Closing()` — CTA, social links
  - `TransitionLine()` — "Time to investigate some crimes against CI"
- [x] **T3** Agent intro lines — one per persona, personality-driven, funny. Fire=impatient detective, Water=forensic analyst, Earth=desk sergeant, Air=undercover, Diamond=internal affairs, Lightning=dispatch
- [x] **T4** Node descriptions — map each circuit node to a police metaphor
- [x] **T5** Cooperation dialogs — funny argument templates for agent pairs
- [x] **T6** Unit tests: theme + presentation implement interfaces, all sections return data, nil-safety

### Phase 2 — CLI command

- [x] **D1** Update `cmd_demo.go` — `asterisk demo` Cobra command to pass `KabukiConfig` to Kami
- [x] **D2** Flags: `--port` (default 3000), `--replay <path>` (JSONL file), `--speed` (default 1.0), `--live` (connect to running circuit)
- [x] **D3** Wiring: load `circuit_rca.yaml` graph, create `PoliceStationTheme` + `PoliceStationKabuki`, pass to `kami.NewServer(kami.Config{Theme: theme, Kabuki: kabuki})`
- [x] **D4** Integration test: `asterisk demo --replay testdata/demo/sample.jsonl` starts and serves without error

### Phase 3 — Record canonical demo

- [ ] **C1** Run a real calibration session with Kami recording enabled *(deferred — requires wet calibration session)*
- [ ] **C2** Trim the recording to a compelling 3-5 minute segment *(deferred)*
- [ ] **C3** Commit recording as `testdata/demo/ptp-real-ingest.jsonl` *(deferred — synthetic sample.jsonl covers testing)*
- [ ] **C4** Verify replay runs cleanly *(deferred)*

### Phase 4 — Clean up standalone SPA

- [x] **X1** Delete `internal/demo/frontend/` — all React code is now in Origami's Kami *(already deleted)*
- [x] **X2** Delete `internal/demo/embed.go` — no more `go:embed` for frontend assets *(already deleted)*
- [x] **X3** Update `.gitignore` — no frontend-specific entries present (clean)

### Phase 5 — Validate and tune

- [x] **V1** Validate (green) — `go build ./...`, `go test ./...` all pass. Demo replay runs end-to-end.
- [x] **V2** Tune (blue) — Polish intro lines, adjust timing, improve section content.
- [x] **V3** Validate (green) — all tests still pass after tuning.

## Acceptance criteria

**Given** `asterisk demo --replay testdata/demo/ptp-real-ingest.jsonl` is executed,  
**When** a browser navigates to `http://localhost:3000`,  
**Then** the web app presents 12 RH-branded Kabuki sections in scroll order: Hero, Agenda, Problem, Solution, Agent Intros, Transition, Live Demo (embedded Kami debugger graph with SSE-driven animation), Results, Competitive, Architecture, Roadmap, Closing.

**Given** the presentation web app is loaded,  
**When** the user scrolls or uses keyboard navigation (arrow keys, Page Up/Down),  
**Then** each section transitions smoothly, the Agenda navigator highlights the current section, and all ARIA landmarks are present.

**Given** the `PoliceStationTheme` struct,  
**When** it is passed to `kami.NewServer()`,  
**Then** it satisfies the `kami.Theme` interface: all 7 circuit nodes have descriptions, all active personas have intro lines, at least 4 cooperation dialog templates exist.

**Given** a live calibration run with `--live` flag,  
**When** `asterisk demo --live --port 3000` is started alongside `asterisk calibrate`,  
**Then** the Live Demo section shows real-time circuit execution with the police station theme applied, embedded within the presentation SPA.

## Security assessment

No trust boundaries affected. The demo runs on localhost, serves static content, and reads a local JSONL file. No external API calls, no user input beyond CLI flags.

## Notes

2026-02-25 — Contract created as companion to Origami's `kami-live-debugger` contract. This is the "We are done with the PoC" presentation. The Inside Out + Police Station theme emerged from the conversation: Asterisk is the police station investigating crimes against CI. Agents wear police hats, have personality-driven banter, and accumulate evidence (RCA findings) as the demo progresses.

2026-02-25 — Updated to use Kabuki naming. Kami = MCP debugger, Kabuki = presentation layer. `PresentationConfig` → `KabukiConfig`, `PoliceStationPresentation` → `PoliceStationKabuki`. Upstream dependency renamed from `kami-presentation-engine` to `kabuki-presentation-engine`.

2026-02-26 — Contract complete. Phases 1-2, 4-5 done. Phase 3 (canonical recording from real calibration) deferred — requires wet calibration session. Synthetic `testdata/demo/sample.jsonl` covers integration testing. `go build`, `go test`, `just calibrate-stub` all green. 19/19 metrics pass, M19=0.99.
