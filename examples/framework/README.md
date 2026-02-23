# Framework Playground

A self-contained demo of the Asterisk agentic pipeline framework. No ReportPortal, no AI subscriptions, no external services.

## Run It

```bash
go run ./examples/framework/
```

## What You'll See

The playground walks through every major framework concept:

| Section | What it demonstrates |
|---------|---------------------|
| **Elements** | 6 behavioral archetypes (Fire, Lightning, Earth, Diamond, Water, Air) with quantified traits |
| **Personas** | 8 pre-configured agent identities — 4 Light (investigation) + 4 Shadow (adversarial dialectic) |
| **Pipeline DSL** | YAML pipeline loading, validation, node/edge/zone structure |
| **Mermaid** | Pipeline rendered as a diagram you can paste into [mermaid.live](https://mermaid.live) |
| **Graph Walk** | A Herald (Fire/PG) walker traverses a 4-node pipeline with conditional edges |
| **Masks** | Middleware capabilities that inject context at specific nodes |
| **Cycles** | Generative and destructive element interactions (inspired by Wu Xing) |
| **Adversarial Dialectic** | D0-D4 thesis-antithesis-synthesis pipeline with 5 synthesis decisions |

## The Demo Pipeline

`triage.yaml` defines a simple 4-node "bug triage" pipeline:

```
classify --> investigate --> decide --> close
    |              ^           |
    +--(shortcut)--+           +--(reassess)--+
         decide                  investigate
```

- **classify** (Fire) — initial assessment with confidence score
- **investigate** (Water) — deep analysis with convergence tracking and retry loops
- **decide** (Diamond) — approve or reassess
- **close** (Air) — finalize and exit

This is NOT the production RCA pipeline — it's a teaching example using the same framework.

## Source

The framework lives in `internal/framework/` (~1,500 lines of source, ~2,400 lines of tests, zero external dependencies beyond stdlib + YAML parser). Production pipelines are in `pipelines/*.yaml`.
