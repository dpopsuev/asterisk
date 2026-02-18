---
name: asterisk-analyze
description: >
  Run evidence-based RCA on a ReportPortal launch. Accepts a launch ID,
  builds the binary if needed, checks RP token and URL, runs the analysis,
  and presents a human-friendly summary. Use when the user types
  "/asterisk-analyze <LAUNCH_ID>" or asks to analyze an RP launch.
---

# Asterisk Analyze

Run `asterisk analyze` on a ReportPortal launch and present results.

## Trigger

The user types one of:

- `/asterisk-analyze 33195` — analyze launch 33195
- `/asterisk-analyze help` — show usage guide
- `/asterisk-analyze` — (no arg) show usage guide

## Flow

### 1. Parse input

Extract `LAUNCH_ID` from the user's input.

- If the input is empty, "help", or non-numeric → go to **Help mode**.
- If the input is a positive integer → proceed.

### 2. Check binary

```bash
ls bin/asterisk
```

If missing, build it:

```bash
go build -o bin/asterisk ./cmd/asterisk/
```

If the build fails, report the error and stop.

### 3. Check RP URL

```bash
echo $ASTERISK_RP_URL
```

If empty, print this guide and **stop**:

> **RP base URL is not configured.**
>
> Set the environment variable for your RP instance:
>
> ```bash
> export ASTERISK_RP_URL=https://your-rp-instance.example.com
> ```
>
> Then re-run `/asterisk-analyze <LAUNCH_ID>`.

### 4. Check RP token

```bash
test -f .rp-api-key && echo "exists" || echo "missing"
```

If missing, print this guide and **stop**:

> **RP API token not found.** The file `.rp-api-key` is required.
>
> To get your token:
>
> 1. Log in to your ReportPortal instance
> 2. Go to **User Profile** (top-right avatar icon)
> 3. Copy the **API token** (UUID format)
> 4. Save it locally:
>
> ```bash
> echo 'YOUR_TOKEN_HERE' > .rp-api-key
> chmod 600 .rp-api-key
> ```
>
> Then re-run `/asterisk-analyze <LAUNCH_ID>`.

### 5. Run analysis

```bash
bin/asterisk analyze LAUNCH_ID
```

The command uses `$ASTERISK_RP_URL` automatically and writes the artifact to `.asterisk/output/rca-LAUNCH_ID.json`.

### 6. Present results

Read the output artifact:

```bash
cat .asterisk/output/rca-LAUNCH_ID.json
```

Parse the JSON and present a human-friendly summary:

```
## RCA Summary: Launch LAUNCH_ID

| # | Test | Defect Type | Component | Confidence | RCA |
|---|------|-------------|-----------|------------|-----|
| 1 | test_name | PB001 | component | 0.85 | Brief explanation... |

Artifact: .asterisk/output/rca-LAUNCH_ID.json
```

### 7. Offer push (optional)

Ask the user if they want to push the results back to RP:

> Push these results to ReportPortal? Run:
>
> ```bash
> bin/asterisk push -f .asterisk/output/rca-LAUNCH_ID.json --rp-base-url $ASTERISK_RP_URL
> ```

## Help mode

When triggered with no args, "help", or non-numeric input, print:

> **Asterisk Analyze** — Evidence-based Root Cause Analysis for ReportPortal
>
> **Usage:** `/asterisk-analyze <LAUNCH_ID>`
>
> **Example:** `/asterisk-analyze 33195`
>
> **Prerequisites:**
>
> 1. **Go 1.24+** installed
> 2. **RP URL** — `export ASTERISK_RP_URL=https://your-rp-instance.example.com`
> 3. **RP token** — save your API token to `.rp-api-key`:
>
>    ```bash
>    echo 'YOUR_TOKEN' > .rp-api-key && chmod 600 .rp-api-key
>    ```
>
> **What it does:**
>
> Fetches failures from the specified RP launch, runs the F0-F6 evidence pipeline
> with the BasicAdapter (heuristic, zero-LLM), and produces a structured RCA
> artifact with defect classifications, suspected components, and confidence scores.

## Security guardrails

- **Never** echo or log the contents of `.rp-api-key`.
- **Validate** that `LAUNCH_ID` is numeric before use.
- **Never** interpolate the token value into any output or error message.
- Output artifacts are written with `0600` permissions (owner-only read/write).
