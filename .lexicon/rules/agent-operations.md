---
id: agent-operations
title: Agent Operations
description: Asterisk-specific circuit stages, commands, and SLAs
labels: [asterisk, domain]
---

# Agent Operations

Reference global `agentic-workflow` for universal principles. Asterisk-specific:

## Local circuit

| Stage | Command | SLA | On failure |
|-------|---------|-----|------------|
| Build | `just build` (runs `origami fold`) | 30 s | Fix immediately |
| Circuit lint | `origami lint --profile strict` on changed YAMLs | 5 s | Fix; use `--fix` where available |
| Lint | `ReadLints` on changed files | 10 s | Fix introduced; ignore pre-existing |
| Unit test | `go test ./...` in Origami repo (Asterisk has no Go tests) | 60 s | Stop; fix before proceeding |
| Integration | `just calibrate-stub` or scenario-specific | 2 min | Triage metric regressions |
| Wet validation | `just calibrate-wet` or equivalent | 10 min | Monitor; apply abort protocol |

Never skip. Build → test → integration → wet.

## Timeout thresholds

| Scope | Silence budget | Action |
|-------|----------------|--------|
| `asterisk` binary | 30 s | Kill and diagnose |
| `origami fold`, `origami lint`, linters | 60 s cold / 30 s incremental | Kill and diagnose |
| Dispatcher polling | Configured timeout | Inspect, kill, diagnose |

## Calibration SLAs

| Run type | Budget | If exceeded |
|----------|--------|-------------|
| Stub (no LLM) | 10 s | Bottleneck in our code |
| Full 30-case wet (`--parallel=1`) | 10 min | Investigate per-step timing |

Record elapsed time after each run. Flag regressions immediately.
