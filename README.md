<p align="center">
  <img src="asterisk-rh.png" alt="Asterisk" width="200" />
</p>

<h1 align="center">Asterisk</h1>

<p align="center">
  Evidence-based root cause analysis for ReportPortal test failures.
</p>

---

**Asterisk** is a standalone CLI that takes a ReportPortal launch, runs every failure through a six-stage investigation pipeline, and produces a structured RCA report — defect type, suspected component, confidence score, and evidence links.

No AI subscription needed. The default adapter (`basic`) is a zero-LLM heuristic engine that achieves **93% accuracy** on Jira-verified ground truth.

---

## Try It Now (No ReportPortal Needed)

Want to see the engine without any setup? The framework playground demonstrates every major concept in 30 seconds:

```bash
make playground
```

Or run a full analysis against a bundled example envelope:

```bash
make build
bin/asterisk analyze examples/pre-investigation-33195-4.21/envelope_33195_4.21.json -o /tmp/rca.json
```

Stub calibration (20 metrics, zero external dependencies):

```bash
bin/asterisk calibrate --scenario=ptp-mock --adapter=stub
```

---

## Quick Start (with ReportPortal)

### 1. Build

```bash
go build -o bin/asterisk ./cmd/asterisk/
```

Requires **Go 1.24+**.

### 2. Configure RP Access

Set your ReportPortal instance URL:

```bash
export ASTERISK_RP_URL=https://your-rp-instance.example.com
```

Save your RP API token (find it in RP > User Profile > API token):

```bash
echo 'YOUR_TOKEN_HERE' > .rp-api-key
chmod 600 .rp-api-key
```

### 3. Analyze a Launch

```bash
bin/asterisk analyze 33195
```

That's it. The tool fetches the launch from RP, runs the pipeline, and writes the RCA artifact to `.asterisk/output/rca-33195.json`.

### 4. Review Results

The output is a JSON report with one entry per failed test:

```json
{
  "launch_name": "ptp-operator-4.21",
  "cases": [
    {
      "test_name": "[test_id:74895] ptp holdover failure",
      "defect_type": "PB001",
      "component": "linuxptp-daemon",
      "convergence_score": 0.85,
      "rca_message": "Broken pipe in linuxptp-daemon holdover path...",
      "evidence_refs": ["OCPBUGS-74895", "linuxptp-daemon:pkg/daemon/process.go"]
    }
  ]
}
```

### 5. Push to ReportPortal (optional)

Update the defect classifications back in RP:

```bash
bin/asterisk push -f .asterisk/output/rca-33195.json --rp-base-url $ASTERISK_RP_URL
```

---

## Cursor Skill

If you use [Cursor](https://cursor.sh/), Asterisk includes a built-in skill:

```
/asterisk-analyze 33195
```

The skill builds the binary (if needed), checks your token, runs the analysis, and presents a human-friendly summary — all without leaving the IDE.

Type `/asterisk-analyze help` for setup instructions.

---

## The Framework

Underneath the RCA product is a generic **agentic pipeline framework** — a graph-based agent orchestration engine with zero domain dependencies.

```
internal/framework/    ~1,500 lines of source, ~2,400 lines of tests
pipelines/*.yaml       Declarative pipeline definitions (YAML → Graph → Walk)
examples/framework/    Interactive playground
docs/framework-guide.md   Design document
```

**Key concepts:**

- **Elements** — 6 behavioral archetypes (Fire, Lightning, Earth, Diamond, Water, Air) with quantified traits governing speed, persistence, convergence, and failure modes.
- **Personas** — 8 named agent identities (4 Light + 4 Shadow), each with a color, element, court position, and prompt preamble.
- **Pipeline DSL** — YAML pipelines compiled to executable directed graphs with conditional edges, zones, and loop control.
- **Graph Walk** — A Walker (agent) traverses nodes, producing artifacts that drive edge evaluation. First matching edge fires. Definition-order evaluation ensures determinism.
- **Masks** — Detachable middleware that injects capabilities at specific nodes without changing the node or agent.
- **Shadow Court** — An adversarial D0-D4 pipeline where prosecution, defense, and judge deliberate over uncertain classifications. Same interfaces as the Light pipeline.
- **Element Cycles** — Generative and destructive interactions between elements (inspired by Wu Xing) that govern agent routing.

The framework is the engine; the RCA pipeline is one car built on it. See the [Framework Developer Guide](docs/framework-guide.md) for the full design document, or run `make playground` to see it in action.

---

## How It Works

Every failed test flows through a six-stage pipeline (F0-F6):

```
Recall (F0) → Triage (F1) → Resolve (F2) → Investigate (F3) → Correlate (F4) → Review (F5) → Report (F6)
```

| Stage | What it does |
|-------|-------------|
| **Recall** | Checks if this failure matches a known root cause |
| **Triage** | Classifies the failure type and severity |
| **Resolve** | Selects candidate repos for investigation |
| **Investigate** | Deep analysis — logs, commits, code paths |
| **Correlate** | Detects duplicates across cases |
| **Review** | Validates the conclusion |
| **Report** | Generates the final RCA artifact |

Heuristic rules (H1-H18) route each case through the optimal path — short-circuiting when possible (e.g., recall hit skips to review), looping when evidence is insufficient.

---

## Calibration

Asterisk includes a calibration framework with 20 metrics and 4 scenarios to validate accuracy:

| Metric | Current | Target |
|--------|---------|--------|
| Overall Accuracy (M19) | **0.93** | 0.95 |

Run calibration yourself:

```bash
bin/asterisk calibrate --scenario=ptp-real-ingest --adapter=basic
```

---

## Security

- **Token handling**: `.rp-api-key` must be `chmod 600`. The CLI warns if world-readable.
- **Output permissions**: All artifacts written with owner-only permissions (`0600`).
- **No hardcoded URLs**: RP instance configured via environment variable, never embedded in the binary.
- **API timeouts**: All RP API calls have 30-second timeouts to prevent hangs.
- **Input validation**: Launch IDs validated before use in file paths.
- See [security cases](.cursor/security-cases/) for the full OWASP assessment.

---

## Full Documentation

For architecture details, data model, CLI reference, project structure, calibration metrics, and development methodology, see [README.md.post](README.md.post).

---

## License

Apache License 2.0. See [LICENSE](LICENSE).
