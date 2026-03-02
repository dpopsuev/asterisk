# Contract — kabuki-demo-revamp

**Status:** draft  
**Goal:** Rethink the Kabuki demo for the post-zero-Go era: new narrative arc showcasing the YAML-only achievement, canonical wet calibration recording, and updated section content.  
**Serves:** Presentation; external visibility; Red Hat telco QE mission

## Contract rules

- The demo must tell the "zero code, just YAML" story — this is the milestone achievement.
- Canonical recording must come from a real wet calibration run, not synthetic data.
- All demo content lives in Origami's `modules/rca/` (the `PoliceStationKabuki` + `PoliceStationTheme` structs). Asterisk provides only the recording file.
- The three-act structure (Product, Engine, Science) is preserved but content is fully refreshed.

## Context

The current demo was built during the PoC era when Asterisk had 11,771 LOC of Go. Two contracts completed the initial demo:

- `demo-presentation` — Phase 1-2 done (theme + CLI). Phase 3 (canonical recording) deferred.
- `improve-asterisk-kabuki-demo` — 26-section three-act structure, War Room, dagre graph, responsive scaling. Six postmortem issues fixed.

The achievement is different now. The star of the show is the YAML-only architecture — an entire RCA tool defined in three YAML files and compiled to a binary by `origami fold`. The demo needs a new narrative that showcases this.

### Dependencies

- `naming-taxonomy` — Deterministic/Stochastic classification for D/S boundary visualization in the demo.
- `origami-autodoc` (optional) — generated Mermaid diagrams from circuit YAML could be showcased live.
- `housekeeping` + `yaml-playbook-docs` — repo must be clean and documented before demoing.

### Current demo state

- `testdata/demo/sample.jsonl` — 19-line synthetic recording (covers integration tests only).
- `PoliceStationTheme` + `PoliceStationKabuki` live in Origami `modules/rca/cmd/` (moved from Asterisk during `origami-fold`).
- `asterisk demo --replay` CLI works with synthetic data.
- Demo postmortem documented and all 6 issues resolved.

## FSC artifacts

| Artifact | Target | Compartment |
|----------|--------|-------------|
| Canonical demo recording | `testdata/demo/ptp-real-ingest.jsonl` | domain |
| Updated demo narrative doc | `.cursor/docs/demo-narrative.md` | domain |

## Execution strategy

### Phase 1: Narrative design

Document the new three-act narrative:

**Act 1 — "The Crime Scene"** (Asterisk: The Product)
- Hero: Evidence-based RCA for CI failures
- Problem: 10 people x 10 days per failure batch
- Solution: $1 AI cost saves $50K labor (50,000x ROI)
- Live Demo: War Room with real wet calibration walk
- Results: M19 metrics, accuracy comparison (BasicAdapter 0.83, CursorAdapter 0.58 baseline)

**Act 2 — "The Lab"** (Origami: The Engine)
- Zero Go achievement: YAML-only playbook model (the Ansible analogy)
- `origami fold`: show how 3 YAML files compile to a binary (live `origami fold` execution?)
- Circuit DSL: `asterisk-rca.yaml` as the complete application definition
- Deterministic/Stochastic boundary: which nodes need AI, which are match rules
- Autodoc: Mermaid diagram generated from the same YAML (if `origami-autodoc` is ready)
- Elements, Personas, Adversarial Dialectic

**Act 3 — "The Academy"** (Deep Science)
- Ouroboros metacalibration
- Evidence SNR, convergence trajectory, thermal budget
- GBWP, offset compensator
- Multi-operator vision: PTP Operator today, N operators tomorrow

### Phase 2: Record canonical demo

- Run a real wet calibration session with Kami recording enabled.
- Trim to a compelling 3-5 minute segment showing the full F0-F6 circuit walk.
- Commit as `testdata/demo/ptp-real-ingest.jsonl`.
- Verify replay runs cleanly with `asterisk demo --replay testdata/demo/ptp-real-ingest.jsonl`.

### Phase 3: Update section content

- Refresh `PoliceStationKabuki` sections in Origami `modules/rca/cmd/`:
  - `Hero()` — add "100% YAML — Zero Go" subtitle
  - `Problem()` — updated CI failure stats if available
  - `Solution()` — emphasize YAML-only architecture
  - New: "The Lab" sections for Act 2 (CodeShowcase for `origami fold`, ConceptSection for D/S boundary)
  - `Competitive()` — refresh landscape analysis
  - `Roadmap()` — update milestones (PoC done, multi-operator next)
- Update agent intro lines and node descriptions for current circuit structure.
- Update `SectionOrder()` to reflect the refreshed three-act structure.

### Phase 4: Integration and polish

- Test full replay end-to-end with canonical recording.
- Verify all 26 sections render correctly.
- Test responsive scaling in Cursor's embedded browser and standard browser.
- Verify War Room agent tabs, TX/RX panels, graph visualization all work with real data.

## Coverage matrix

| Layer | Applies | Rationale |
|-------|---------|-----------|
| **Unit** | yes | Section data generation, theme completeness |
| **Integration** | yes | `asterisk demo --replay` end-to-end with canonical recording |
| **Contract** | yes | `kami.KabukiConfig` interface compliance |
| **E2E** | yes | Full replay demo runs, all sections render |
| **Concurrency** | no | Single-user demo |
| **Security** | no | Localhost demo |

## Tasks

- [ ] Phase 1 — Design and document new three-act narrative
- [ ] Phase 2a — Run wet calibration with Kami recording
- [ ] Phase 2b — Trim and commit canonical recording
- [ ] Phase 3a — Refresh Act 1 section content (Crime Scene)
- [ ] Phase 3b — Create Act 2 section content (The Lab — zero Go, fold, DSL, D/S)
- [ ] Phase 3c — Refresh Act 3 section content (Academy — science)
- [ ] Phase 3d — Update agent intros, node descriptions, cooperation dialogs
- [ ] Phase 4 — Integration test: full replay e2e
- [ ] Validate (green) — all tests pass, demo replays cleanly
- [ ] Tune (blue) — polish language, timing, visual flow
- [ ] Validate (green) — all tests still pass after tuning

## Acceptance criteria

- **Given** `asterisk demo --replay testdata/demo/ptp-real-ingest.jsonl`, **when** a browser opens `http://localhost:3000`, **then** all 26 sections render with the updated three-act narrative.
- **Given** Act 2 sections, **when** viewing "The Lab", **then** the zero-Go achievement, `origami fold`, and circuit DSL are prominently showcased.
- **Given** the canonical recording, **when** replaying, **then** it shows a real wet calibration walk (not synthetic data) with actual RCA evidence accumulation.
- **Given** the War Room, **when** the demo reaches the Live Demo section, **then** agent tabs show real agent activity, TX/RX panels show actual prompts/responses, and the graph animates the circuit walk.
- **Given** a D/S boundary visualization (post `naming-taxonomy`), **when** viewing the circuit graph, **then** deterministic and stochastic nodes are visually distinct.

## Security assessment

No trust boundaries affected. The demo runs on localhost, serves static content, and reads a local JSONL file. The canonical recording must be scrubbed of any sensitive data (internal hostnames, credentials) per `data-hygiene.mdc`.

## Notes

2026-03-02 00:00 — Contract drafted. Full demo revamp for the post-zero-Go era. The current demo tells the PoC story; the new demo tells the "zero code, just YAML" story. Three acts: Crime Scene (product), Lab (engine), Academy (science).
