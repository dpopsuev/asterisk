# Contract — Demo Presentation

**Status:** draft  
**Goal:** Build the "Inside Out"-themed PoC presentation using Origami's Kami debugger — police station theme, agent intros, interactive pipeline visualization, and a recorded replay for repeatable demos.  
**Serves:** Polishing & Presentation (should)

## Contract rules

- This contract depends on Origami's `kami-live-debugger` contract. Do not start Phase 2+ until the `kami.Theme` interface is defined.
- Theme content (intro lines, node descriptions, cooperation dialogs) must be funny and personality-driven. Each persona's voice matches its element.
- The recorded replay `.jsonl` is committed to the repo as a demo artifact. It must be reproducible from a real calibration run.
- No hardcoded agent identities — the theme provides personality content, Kami provides the model identity (provider/model/version) from the actual run.

## Context

- **Red Hat Presentation DNA** (`origami/.cursor/docs/rh-presentation-dna.md`): Color system (4 collections), web section patterns (12 types), design constraints, accessibility. The demo web app uses RH brand colors and layout patterns — not as a static slide deck, but as the design language for an interactive presentation SPA.
- **Kami** (Origami): Live agentic debugger with triple-homed architecture (MCP + HTTP/SSE + WS). Provides `Theme` interface for domain-specific visualization content. See `origami/.cursor/contracts/draft/kami-live-debugger.md`.
- **Pipeline RCA** (`internal/orchestrate/pipeline_rca.yaml`): Asterisk's 7-node pipeline with 3 zones (Backcourt, Frontcourt, Paint). This is the graph visualized in the demo.
- **Origami Personas** (`persona.go`): 8 personas (Herald, Seeker, Sentinel, Weaver + Shadow counterparts) with element affinities and personality traits.
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

Phase 0 scaffolds the RH-branded presentation web app — a single-page application that IS the demo, with section-based navigation mapping storyboard acts to RH web section patterns. The Kami live graph visualization is embedded directly in the "Live Demo" section. Phase 1 implements the `kami.Theme` interface with Asterisk's police station personality. Phase 2 wires the `asterisk demo` CLI command to serve the presentation SPA. Phase 3 records a canonical calibration run for repeatable demos. Phase 4 validates the full replay experience.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Theme struct field completeness, intro line generation |
| **Integration** | yes | `asterisk demo --replay` starts Kami, serves SPA, plays events |
| **Contract** | yes | `kami.Theme` interface compliance |
| **E2E** | yes | Full replay demo runs without errors |
| **Concurrency** | no | Single-user demo, no shared state |
| **Security** | no | Localhost demo, no trust boundaries |

## Tasks

### Phase 0 — Presentation Web App (RH-branded interactive SPA)

- [ ] **S1** Create web section structure document `docs/demo-web-sections.md` mapping storyboard acts to RH web section patterns per `origami/.cursor/docs/rh-presentation-dna.md` Section 5.1:
  - Hero: **Title pattern** — full-viewport hero with animated Origami logo, "Asterisk: AI-Driven Root-Cause Analysis", presenter info
  - Agenda: **Navigator pattern** — interactive section navigator with `▸` markers, click-to-jump between sections
  - Problem: **SplitPane pattern** — CI failure stats left, animated counter right
  - Solution: **IconGrid pattern** — pipeline graph preview (static Mermaid render), 7 nodes, 3 zones
  - Agent Intros: **CardCarousel pattern** — 3D CSS polyhedra per agent, name, element, personality tags, model identity
  - Transition: **Divider pattern** — full-screen animated text: "Time to investigate some crimes against CI"
  - Live Demo: **EmbeddedKami pattern** — Kami graph visualization embedded directly, SSE-driven animation (the centerpiece)
  - Results: **MetricCard pattern** — animated M19 bar comparison, metric cards, live-updating if `--live`
  - Competitive: **InteractiveTable pattern** — Origami vs CrewAI vs OmO with hover highlights
  - Architecture: **ImagePane pattern** — Mermaid diagram rendered client-side
  - Roadmap: **HorizontalTimeline pattern** — animated milestone dots for Sprint 1-6
  - Closing: **Closing pattern** — RH boilerplate, social links, CTA
- [ ] **S2** Scaffold presentation SPA (React + Vite + TypeScript + Tailwind) in `internal/demo/frontend/` with section-based scroll navigation
- [ ] **S3** Configure Tailwind theme with RH Color Collection 1 tokens (red-50 `#ee0000`, purple-50 `#5e40be`, teal-50 `#37a3a3`, neutrals) per `origami/.cursor/docs/rh-presentation-dna.md` Section 1
- [ ] **S4** Verify accessibility: WCAG contrast ratios, keyboard navigation between sections (arrow keys / Page Up/Down), ARIA landmarks per section, no color-alone data differentiation

### Phase 1 — Police Station Theme

- [ ] **T1** Define `PoliceStationTheme` struct implementing `kami.Theme` interface in `internal/demo/theme.go`
- [ ] **T2** Agent intro lines — one per persona, personality-driven, funny. Fire=impatient detective, Water=forensic analyst, Earth=desk sergeant, Air=undercover, Diamond=internal affairs, Lightning=dispatch
- [ ] **T3** Node descriptions — map each pipeline node to a police metaphor (recall="Witness Interview", triage="Case Classification", investigate="Crime Scene Analysis", etc.)
- [ ] **T4** Cooperation dialogs — funny argument templates for agent pairs working on the same case ("Herald: I already solved it. Seeker: You haven't even read the logs.")
- [ ] **T5** Costume assets — police hat SVG overlay for agent shapes, badge icon for the domain header
- [ ] **T6** Unit tests: theme implements interface, all nodes have descriptions, all personas have intros

### Phase 2 — CLI command

- [ ] **D1** Implement `cmd_demo.go` — `asterisk demo` Cobra command
- [ ] **D2** Flags: `--port` (default 3000), `--replay <path>` (JSONL file), `--speed` (default 1.0), `--live` (connect to running pipeline)
- [ ] **D3** Wiring: load `pipeline_rca.yaml` graph, create `PoliceStationTheme`, pass both to `kami.NewKamiServer()`
- [ ] **D4** Integration test: `asterisk demo --replay testdata/demo/sample.jsonl` starts and serves without error

### Phase 3 — Record canonical demo

- [ ] **C1** Run a real calibration session (`just calibrate-wet` or equivalent) with Kami recording enabled
- [ ] **C2** Trim the recording to a compelling 3-5 minute segment showing agent intros, graph traversal, cooperation, and RCA evidence
- [ ] **C3** Commit recording as `testdata/demo/ptp-real-ingest.jsonl`
- [ ] **C4** Verify replay: `asterisk demo --replay testdata/demo/ptp-real-ingest.jsonl --speed 2.0` runs cleanly

### Phase 4 — Validate and tune

- [ ] **V1** Validate (green) — `go build ./...`, `go test ./...` all pass. Demo replay runs end-to-end.
- [ ] **V2** Tune (blue) — Polish intro lines, adjust timing, improve cooperation dialog variety.
- [ ] **V3** Validate (green) — all tests still pass after tuning.

## Acceptance criteria

**Given** `asterisk demo --replay testdata/demo/ptp-real-ingest.jsonl` is executed,  
**When** a browser navigates to `http://localhost:3000`,  
**Then** the web app presents 12 RH-branded sections in scroll order: Hero, Agenda, Problem, Solution, Agent Intros, Transition, Live Demo (embedded Kami graph with SSE-driven animation), Results, Competitive, Architecture, Roadmap, Closing.

**Given** the presentation web app is loaded,  
**When** the user scrolls or uses keyboard navigation (arrow keys, Page Up/Down),  
**Then** each section transitions smoothly, the Agenda navigator highlights the current section, and all ARIA landmarks are present.

**Given** the `PoliceStationTheme` struct,  
**When** it is passed to `kami.NewKamiServer()`,  
**Then** it satisfies the `kami.Theme` interface: all 7 pipeline nodes have descriptions, all active personas have intro lines, at least 4 cooperation dialog templates exist.

**Given** a live calibration run with `--live` flag,  
**When** `asterisk demo --live --port 3000` is started alongside `asterisk calibrate`,  
**Then** the Live Demo section shows real-time pipeline execution with the police station theme applied, embedded within the presentation SPA.

## Security assessment

No trust boundaries affected. The demo runs on localhost, serves static content, and reads a local JSONL file. No external API calls, no user input beyond CLI flags.

## Notes

2026-02-25 — Contract created as companion to Origami's `kami-live-debugger` contract. This is the "We are done with the PoC" presentation. The Inside Out + Police Station theme emerged from the conversation: Asterisk is the police station investigating crimes against CI. Agents wear police hats, have personality-driven banter, and accumulate evidence (RCA findings) as the demo progresses.
