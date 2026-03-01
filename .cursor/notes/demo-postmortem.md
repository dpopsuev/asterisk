# Demo Postmortem — Kabuki First Attempt

**Date:** 2026-02-26  
**Outcome:** Failed demo. Six root causes identified, all addressed in `improve-asterisk-kabuki-demo` contract.

## What happened

The first Asterisk Kabuki demo was presented using the initial `demo-presentation` contract deliverables: `PoliceStationKabuki` + `PoliceStationTheme`, `asterisk demo` CLI, and a static sample recording. The presentation fell flat.

## Six failures

### 1. No live action

The demo had no War Room — just a passive graph with an event log. No agent tabs, no TX/RX panels, no pause/resume controls. The audience couldn't see the multi-agent circuit in action.

**Fix:** Redesigned `LiveDemoSection` as a War Room: agent tabs (colored by element), TX/RX panels showing selected agent's prompt/response, pause/resume via WebSocket, RCA case tabs. Works in both replay and live mode.

### 2. Poor graph

`CircuitGraph` used flat horizontal layout (`x: 200*i, y: 100`). No auto-layout, no edge arrows, no hover tooltips. Nodes were uniform grey circles that conveyed no information about circuit topology.

**Fix:** Replaced with dagre auto-layout (`@dagrejs/dagre`), directional edge arrows, hover tooltips from theme `NodeDescriptions()`, element-colored active nodes, `@keyframes node-pulse` blink animation, dark grey unvisited nodes.

### 3. Language too allegory-heavy

The police metaphor was entertaining but opaque. Phrases like "Witness Interview" and "Jurisdiction Check" didn't convey what the system actually does. The audience needed domain context.

**Fix:** Every allegory now has its domain counterpart on the same line: "Witness Interview / Historical Failure Lookup", "Jurisdiction Check / Repository Selection", "Case Classification / Defect Type Classification". Applied across all sections, node descriptions, and agent intros.

### 4. Scaling issues

Sections didn't render well in Cursor's embedded web browser — content clipped or required horizontal scrolling. No responsive breakpoints.

**Fix:** Added responsive CSS breakpoints at 768px and 480px. Semantic design token layer with 50+ color tokens. Dark/light mode toggle. Per-act color harmony following Red Hat brand guidelines.

### 5. Missing section titles

Section pages had no visible headings. The audience couldn't tell which section they were looking at. The Agenda listed section names, but the sections themselves didn't display their own title.

**Fix:** Every section component now displays a visible `<h2>` title matching its Agenda label.

### 6. Wrong story order

The original 12-section layout (Hero → Problem → Solution → Demo → Results → Competitive → Architecture → Roadmap → Closing) lumped everything into a flat sequence. No narrative arc. The Origami engine reveal and deep science topics had no home.

**Fix:** Three-act structure with 26 sections: Act 1 (Asterisk: The Product, 8 sections), Act 2 (Origami: The Engine, 9 sections), Act 3 (Deep Science, 6 sections), plus bookends. `SectionOrder()` on `KabukiConfig` makes ordering consumer-configurable. New section types (`CodeShowcaseSection`, `ConceptSection`) support Act 2/3 content.

## Lessons learned

- **Show, don't tell.** A passive graph is a slide. The War Room with live agent activity is a demo.
- **Metaphors need anchoring.** Allegory without domain grounding is poetry, not a product pitch.
- **Test in the target viewport.** Cursor's web browser is narrower than a standard monitor — design for it.
- **Narrative arc matters.** Three acts with escalating depth (product → engine → science) keeps the audience engaged.

## Outstanding

- Canonical recording: current `testdata/demo/sample.jsonl` is a 19-line synthetic stub. Replace with real wet calibration recording (`ptp-real-ingest.jsonl`) when wet infrastructure is ready.
