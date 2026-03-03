# Contract — sumi-refining

**Status:** draft
**Goal:** Sumi TUI renders correctly: ANSI-safe panel composition, newest-first events, case lifecycle in Workers panel, graph-quality rendering, and tick-driven animations for walker movement and node activity.
**Serves:** 100% DSL — Zero Go

## Contract rules

Global rules only.

## Context

After the first successful wet calibration run through Sumi, four visual and data issues were observed:

1. **Workers panel shows 4 walkers, not 12 cases.** The panel reads `m.snap.Walkers` which contains only currently-active case walkers (C01-C04 at any moment), not the full case lifecycle. The label "Workers" is misleading — these are case walkers, not worker agents (w1-w4).
2. **Events panel shows oldest first; newest get clipped.** `TimelineRingBuffer.All()` returns oldest-first. `RenderPanelFrame` clips to `innerH` lines from the top, so newest events are invisible.
3. **TUI visually broken.** `blitString` writes lipgloss ANSI-rendered strings onto a rune canvas. ANSI escape sequences consume rune positions but have zero display width, shifting all content rightward and clipping panels.
4. **Graph could be more graph-like.** Layout and edge routing work but node centering, panel-aware scaling, and edge aesthetics are basic compared to mermaid-style output.
5. **No animations.** Walker movement between nodes is instantaneous — the `●` marker jumps without showing traversal. Active nodes show a static `▶` with no indication of ongoing processing. No visual feedback on state transitions.
6. **Generic iconography.** Nodes are plain rectangles with text labels. The Origami Symbol Standard (OSS) defines a rich pictogram vocabulary rooted in electronic engineering (IEC 60617 / IEEE 315) that should be applied: D/S badges, element-colored borders, typed edge styles, and tiered detail levels.

### Design references

Two existing case studies and one UX analysis govern the visual direction:

- **`origami/.cursor/docs/case-studies/electronic-symbols-washi-pictograms.md`** — The Origami Symbol Standard (OSS). Defines tiered pictograms for every Origami primitive, modeled on IEC 60617 electronic schematic symbols. Key principles: iconic not literal, distinguishable at small scale, composable with standard edges, annotatable, variant-aware (D/S badges as variant markers).
- **`origami/.cursor/docs/case-studies/electronic-circuit-theory.md`** — Component-level mapping (transistor=Node, op-amp=Dialectic, diode=Shortcut, capacitor=Context, wire=Edge). Establishes the conceptual foundation for why circuit-style rendering is appropriate.
- **`origami/.cursor/docs/laws-of-ux-kabuki.md`** — 10 Laws of UX applied to Origami. Directly applicable to Sumi TUI:
  - **Von Restorff Effect**: Active node must stand out through multiple channels (color + animation + border weight), not color alone.
  - **Doherty Threshold**: All animations 200-300ms. No perceptible lag on state transitions.
  - **Law of Uniform Connectedness**: Directional edge arrows required for graph readability. Edge color matches source node element.
  - **Aesthetic-Usability Effect**: A polished TUI builds trust in the product before functionality is evaluated.
  - **Peak-End Rule**: The War Room (circuit graph) is the visual peak — invest disproportionate polish.

### OSS pictograms for terminal rendering

The OSS defines SVG pictograms for Washi (web). For Sumi (terminal), these translate to Unicode box-drawing equivalents:

| Origami Primitive | OSS Pictogram | Sumi Terminal Symbol |
|-------------------|---------------|---------------------|
| Node (deterministic) | Rectangle + gear badge | `[D]` badge (existing), add `⚙` in high-detail mode |
| Node (stochastic) | Rectangle + sparkle badge | `[S]` badge (existing), add `✦` in high-detail mode |
| Node (dialectic) | Triangle (op-amp) | `[Δ]` badge (existing) |
| Edge (normal) | Solid line + arrowhead | `─▸` (existing) |
| Edge (shortcut) | Dashed line + triangle | `╌▸` (existing) |
| Edge (loop) | Curved return + circular arrow | `◀` return (existing) |
| Walker | Element-colored moving dot | `●` (existing), animate along edge path |
| Zone | Rounded rectangle + label | Border + label (existing), add element background tint |
| Start (`_start`) | Filled circle + play icon | `▶` (existing) |
| Done (`_done`) | Filled circle + stop icon | `■` |
| Extractor | Funnel badge | `[E]` or `↓` |
| Transformer | Hexagonal badge | `⬡` |
| Breakpoint | | `◉` (existing) |

Key files:
- `sumi/model.go` — `viewWarRoom`, `blitString`, `renderWorkersContent`, `renderTimelineContent`
- `sumi/panels.go` — `RenderPanelFrame`, `padOrTruncate`, `ComputeLayout`
- `sumi/graph.go` — `RenderGraphWithHitMap`, `drawEdges`, `drawNodes`
- `sumi/timeline.go` — `TimelineRingBuffer.All()`, `Filtered()`
- `sumi/workers.go` — `WorkersPanel`
- `view/store.go` — `CircuitStore`, `CircuitSnapshot.Walkers`
- `view/grid.go` — `GridLayout.Layout`

### Current architecture

`viewWarRoom` allocates a `[][]rune` canvas, renders each panel with lipgloss (ANSI codes), then `blitString` writes rune-by-rune — ANSI sequences corrupt positions. Events are oldest-first. Workers panel shows only active walkers.

### Desired architecture

`viewWarRoom` composes panels via lipgloss layout primitives (`JoinHorizontal`, `JoinVertical`, `Place`) which handle ANSI-aware widths internally. Events are newest-first. Cases panel shows full lifecycle (pending/active/completed). Graph is centered and scaled within its panel. Tick-driven animations provide visual feedback: active node spinners, edge traversal highlights on walker movement, node completion flash, and connection status pulse. Node rendering uses OSS-derived iconography: D/S badges with unicode symbols, element-colored borders, typed edge styles — applied through the Von Restorff effect (multiple visual channels for active nodes) and Doherty threshold (200-300ms animation timing).

## FSC artifacts

Code only — no FSC artifacts.

## Execution strategy

1. **Fix visual misalignment (issue 3)** — Replace rune-canvas `blitString` composition with lipgloss layout primitives. Foundation for all other visual work.
2. **Fix events ordering (issue 2)** — Reverse iteration in `renderTimelineContent()`. Smallest, highest-impact UX fix.
3. **Rework Workers panel to Cases (issue 1)** — Track case lifecycle (pending/active/completed) in store, rename panel, display all cases with status.
4. **Improve graph rendering (issue 4)** — Center graph within panel, improve edge routing aesthetics.
5. **Add animations (issue 5)** — Tick-driven animation system: active node spinners, walker edge traversal highlights, node completion flash, connection status pulse.
6. **Apply OSS iconography (issue 6)** — Upgrade node and edge rendering with electronic-engineering-inspired symbols per the Origami Symbol Standard. Apply Laws of UX (Von Restorff, Doherty, Uniform Connectedness) to all visual decisions.

Each issue gets a failing reproduction test before the fix (Red-Green methodology).

### Animation catalog

| Animation | Trigger | Visual | Duration |
|-----------|---------|--------|----------|
| Active node spinner | Node in `active` state | `▶` replaced by rotating braille (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) | Continuous while active |
| Edge traversal highlight | Walker moves between nodes | Connecting edge `─` becomes `━` (or color shift) | ~300ms (3-4 frames) |
| Node completion flash | Node transitions to `completed` | Border briefly flashes green | ~200ms (2 frames) |
| New event highlight | Event added to timeline | Newest entry renders bold/bright | ~500ms (5 frames) |
| Connection pulse | SSE reconnecting | Status indicator animates (`◌` `◔` `◑` `◕` `●`) | Continuous while reconnecting |

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Panel rendering, event ordering, display-width calculations, case lifecycle |
| **Integration** | yes | Full War Room rendering with realistic circuit data at various terminal sizes |
| **Contract** | no | No public API schema changes |
| **E2E** | yes | Sumi TUI rendering with populated circuit state |
| **Concurrency** | yes | Timeline ring buffer concurrent access, store walker updates |
| **Security** | no | No trust boundaries affected |

## Tasks

- [ ] Test: `blitString` with ANSI-styled content misaligns subsequent panels (reproduction)
- [ ] Fix: replace rune-canvas composition with lipgloss layout in `viewWarRoom`
- [ ] Fix: make `padOrTruncate` display-width-aware (use `lipgloss.Width` or `runewidth`)
- [ ] Test: `renderTimelineContent()` returns oldest-first, newest events clipped by panel height
- [ ] Fix: reverse event iteration in `renderTimelineContent()` — newest at top
- [ ] Test: Workers panel shows only active walkers, not full case lifecycle
- [ ] Fix: track case lifecycle in `CircuitStore` (pending/active/completed); rename panel to Cases
- [ ] Test: graph rendering not centered within panel; verify edge routing quality
- [ ] Fix: center graph within circuit panel; improve edge routing aesthetics
- [ ] Add tick-based animation system to Model (frame counter, `tea.Tick` subscription)
- [ ] Test: active node shows spinner frames advancing on tick
- [ ] Fix: replace static `▶` with rotating braille spinner on active nodes
- [ ] Test: walker movement triggers edge highlight that decays after N frames
- [ ] Fix: edge traversal highlight on walker `DiffWalkerMoved` events
- [ ] Fix: node completion flash on `DiffNodeState` completed transition
- [ ] Fix: connection status pulse animation while reconnecting
- [ ] Upgrade node rendering: D/S badges use `⚙`/`✦`/`Δ` unicode symbols per OSS
- [ ] Upgrade node rendering: element-colored borders (fire=red, water=blue, earth=purple, etc.)
- [ ] Upgrade active node: multiple visual channels per Von Restorff (color + spinner + border weight)
- [ ] Upgrade done/start terminals: `■` for done, `▶` for start per OSS
- [ ] Test: node symbols render correctly at all layout tiers; D/S badges distinguishable
- [ ] Validate (green) — all tests pass, acceptance criteria met.
- [ ] Tune (blue) — refactor for quality. No behavior changes.
- [ ] Validate (green) — all tests still pass after tuning.

## Acceptance criteria

Given a War Room layout with colored (ANSI-styled) panels
When rendered at 140x40 (TierFull)
Then all panels are correctly positioned with no ANSI-induced misalignment

Given a timeline with 50+ events in a panel with height for 8 lines
When the Events panel renders
Then the 8 newest events are visible (newest at top)

Given a 4-parallel calibration with 12 cases
When the Cases panel renders during execution
Then all 12 cases are listed with their status (pending, active at node, or completed)

Given a circuit with 6+ nodes and edges
When the graph renders in the center panel
Then the graph is centered horizontally and vertically within the available space

Given a node in `active` state and the animation tick fires
When the graph renders
Then the node shows a rotating spinner character that advances each frame

Given a walker moves from node A to node B
When the graph renders within 300ms of the move
Then the edge connecting A to B is rendered with a highlight style

Given the SSE connection is in reconnecting state
When the status bar renders
Then the connection indicator animates through a pulse sequence

Given a circuit with deterministic and stochastic nodes
When the graph renders
Then deterministic nodes show `⚙` badge, stochastic nodes show `✦` badge, dialectic nodes show `Δ` badge

Given a circuit with element-typed nodes (fire, water, earth, etc.)
When the graph renders with colors enabled
Then each node border uses its element color, and the active node stands out via multiple visual channels (color + animation + heavier border) per Von Restorff

## Security assessment

No trust boundaries affected.

## Notes

2026-03-03 — Added OSS iconography (issue 6): electronic engineering pictogram vocabulary for terminal rendering, element-colored borders, multi-channel Von Restorff active node treatment. Referenced three Origami case studies: electronic-symbols-washi-pictograms, electronic-circuit-theory, laws-of-ux-kabuki.
2026-03-03 — Added animation system (issue 5): active node spinners, edge traversal highlights, node completion flash, connection pulse. Animation catalog with trigger/visual/duration spec.
2026-03-03 — Contract created from wet calibration observation. Four issues identified: ANSI misalignment, event ordering, worker/case confusion, graph quality.
