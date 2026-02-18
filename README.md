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

## Quick Start

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

*License information to be added.*
