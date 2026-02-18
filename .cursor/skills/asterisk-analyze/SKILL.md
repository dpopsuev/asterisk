---
name: asterisk-analyze
description: >
  Run AI-driven RCA on a ReportPortal launch using the Cursor agent as the
  reasoning engine (--adapter=cursor). Accepts a launch ID, builds binaries
  if needed, checks RP token and URL, launches analysis with file-based
  dispatch, then acts as the AI investigator via the asterisk-investigate
  protocol. Use when the user types "/asterisk-analyze <LAUNCH_ID>" or asks
  to analyze an RP launch.
---

# Asterisk Analyze (Cursor Adapter)

Run `asterisk-analyze-rp-cursor` on a ReportPortal launch. This binary bakes in
`--adapter=cursor --dispatch=file --report` with RP configuration from environment
variables. The Cursor agent IS the reasoning engine — after launching the command,
you switch to the `asterisk-investigate` signal protocol to produce F0-F6 artifacts.

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

### 2. Check binaries

```bash
ls bin/asterisk bin/asterisk-analyze-rp-cursor
```

If missing, build both:

```bash
go build -o bin/asterisk ./cmd/asterisk/ && go build -o bin/asterisk-analyze-rp-cursor ./cmd/asterisk-analyze-rp-cursor/
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

### 4. Check RP project

```bash
echo $ASTERISK_RP_PROJECT
```

If empty, print this guide and **stop**:

> **RP project is not configured.**
>
> ```bash
> export ASTERISK_RP_PROJECT=your-project-name
> ```

### 5. Check RP token

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

### 6. Launch analysis

```bash
bin/asterisk-analyze-rp-cursor LAUNCH_ID
```

This runs `asterisk analyze LAUNCH_ID --adapter=cursor --dispatch=file --report`
with RP URL and project from environment. The command:

- Fetches failures from the RP launch
- Creates signal.json files in `.asterisk/analyze/` for each pipeline step
- Waits for the Cursor agent to produce artifacts via the signal protocol

### 7. Act as the AI investigator

Once the command is running and producing signal.json files, follow the
**asterisk-investigate** skill protocol:

1. Watch for `signal.json` with `status: "waiting"` in `.asterisk/analyze/`
2. Read the prompt at `prompt_path`
3. Analyze the failure data
4. Write the JSON artifact to `artifact_path`
5. Repeat until the pipeline completes

The asterisk-investigate skill has full details on the signal protocol,
artifact schemas for each F0-F6 step, and worked examples.

### 8. Present the RCA report

After the pipeline completes, the `--report` flag produces a Markdown report.
**Read** the `.md` file and **present its contents to the user verbatim**.

```bash
cat .asterisk/output/rca-LAUNCH_ID.md
```

Read the file content and present it directly in your response to the user.
Do not summarize or reformat it — relay it as-is.

### 9. Offer push (optional)

Ask the user if they want to push the results back to RP:

> Push these results to ReportPortal? Run:
>
> ```bash
> bin/asterisk push -f .asterisk/output/rca-LAUNCH_ID.json --rp-base-url $ASTERISK_RP_URL
> ```

## Help mode

When triggered with no args, "help", or non-numeric input, print:

> **Asterisk Analyze** — AI-driven Root Cause Analysis for ReportPortal
>
> **Usage:** `/asterisk-analyze <LAUNCH_ID>`
>
> **Example:** `/asterisk-analyze 33195`
>
> **Prerequisites:**
>
> 1. **Go 1.24+** installed
> 2. **RP URL** — `export ASTERISK_RP_URL=https://your-rp-instance.example.com`
> 3. **RP project** — `export ASTERISK_RP_PROJECT=your-project-name`
> 4. **RP token** — save your API token to `.rp-api-key`:
>
>    ```bash
>    echo 'YOUR_TOKEN' > .rp-api-key && chmod 600 .rp-api-key
>    ```
>
> **What it does:**
>
> Fetches failures from the specified RP launch, runs the F0-F6 evidence pipeline
> with the Cursor agent as the AI reasoning engine (--adapter=cursor), and produces
> a structured RCA artifact with defect classifications, suspected components, and
> confidence scores. The agent reads prompts via signal.json and writes artifacts
> back — no manual intervention required.

## Security guardrails

- **Never** echo or log the contents of `.rp-api-key`.
- **Validate** that `LAUNCH_ID` is numeric before use.
- **Never** interpolate the token value into any output or error message.
- Output artifacts are written with `0600` permissions (owner-only read/write).
