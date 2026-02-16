# Asterik Roadmap (TDD‑BDD AI)

## Minimal roles
- [QA] Test execution quality and failure analysis
- [Dev] Component owner and code changes
- [Triage] On-call triage and routing
- [Platform] CI/CD and infra pipelines
- [Security] Access control and compliance

## User stories (labeled)
- [QA] As a test engineer, I want Asterik to ingest Report Portal failures for a run so that RCA starts with the exact failing tests and metadata (suite, labels, env, timestamps).
- [Dev][Triage] As a maintainer, I want Asterik to map failures to relevant code repositories (operators, infra, pipeline, manifests) so that investigation scope is correct.
- [Dev] As a maintainer, I want Asterik to pull commit history and diffs around the failure window so that suspected regressions are surfaced.
- [QA][Triage] As a QA analyst, I want Asterik to correlate failures across runs (not just previous run) so that flaky vs. persistent issues are distinguished.
- [Dev][Platform] As a release manager, I want Asterik to identify recently changed components (operators, OLM, ZTP, IPI configs) that align with the failure signature.
- [Dev][QA] As a developer, I want Asterik to link failures to suspected code paths or manifests using pattern matching or log signatures.
- [Triage] As a triager, I want Asterik to assign a probable root cause category (infra, test, operator, config, external dependency) with confidence scoring.
- [Triage] As a triager, I want Asterik to suggest next diagnostic actions (rerun subset, gather logs, check specific components).
- [Platform][Triage] As a platform engineer, I want Asterik to pull CI pipeline context (Jenkins stages, artifacts, env vars) to explain infra-driven failures.
- [QA][Triage] As a user, I want Asterik to export RCA summaries back into Report Portal and/or GitHub issues.
- [Security] As a security/compliance lead, I want Asterik to respect repo access controls and secrets so that analysis is safe.
- [QA][Dev][Triage] As a team lead, I want Asterik to learn from confirmed RCA outcomes so future analyses improve.

## Roadmap (ordered)
### Phase 0 — Foundations
- Ingest Report Portal failures for a run.
- Map failures to relevant code repositories.

### Phase 1 — Evidence Gathering
- Pull commit history and diffs around the failure window.
- Pull CI pipeline context (stages, artifacts, env vars).

### Phase 2 — Correlation & Attribution
- Correlate failures across runs (multi-run history).
- Link failures to suspected code paths or manifests.

### Phase 3 — Triage Intelligence
- Assign probable root cause category with confidence.
- Suggest next diagnostic actions.

### Phase 4 — Reporting & Feedback
- Export RCA summaries back into Report Portal / GitHub issues.
- Learn from confirmed RCA outcomes.

### Phase 5 — Optional / Advanced
- Detect flaky tests via historical patterns.
- Correlate with cluster metrics/events.
- Explain RCA in natural language with citations.

## BDD acceptance criteria templates
Use these for each user story to drive Gherkin specs.

### Template: data ingestion
Given a Report Portal launch exists with failed tests  
When Asterik ingests the launch  
Then failures are listed with suite, labels, environment, timestamps, and artifacts

### Template: repository mapping
Given a failure includes component labels or file paths  
When Asterik resolves repository mappings  
Then mapped repos are listed with rule evidence and confidence

### Template: commit correlation
Given a failure timestamp and mapped repositories  
When Asterik queries commit history for the time window  
Then candidate commits are listed with diffs and authors

### Template: cross-run correlation
Given the same test appears in multiple launches  
When Asterik aggregates failures across runs  
Then flake rate and recurrence are calculated

### Template: classification + actions
Given evidence from logs, commits, and pipeline context  
When Asterik classifies the failure  
Then a category and confidence are shown with recommended actions

### Template: export
Given an RCA summary is generated  
When the user requests export  
Then a Report Portal comment and/or GitHub issue is created

## TDD test matrix (per story)
For every story, implement the following test layers:
- Unit: parsing, mapping, scoring, and classifiers
- Integration: Report Portal API, GitHub/Git providers, Jenkins/Prow
- Contract: API schemas, auth scopes, rate limits
- E2E: full pipeline from RP launch to RCA output
- Security: secret redaction, RBAC scope, audit logging

## Core data inputs and outputs
### Inputs
- Report Portal: launches, test items, logs, attachments
- CI pipeline: Jenkins stages, artifacts, env vars (future: Prow)
- Repositories: commit history, diffs, file paths, ownership metadata
- Cluster context (optional): events, node health, operator status

### Outputs
- RCA summary with evidence links
- Suspected components and commits
- Category + confidence score
- Recommended next actions
- Exported updates to RP and GitHub

## AI‑assisted RCA guidelines
- Ground every conclusion in evidence links (logs, commits, pipeline data).
- Provide confidence scores with rationale.
- Prefer explainable heuristics before opaque model output.
- Track feedback on RCA correctness for continuous improvement.

## Quality gates
- No PII/secrets in outputs (redaction required).
- Deterministic outputs for same inputs (seeded ordering).
- All external calls have timeouts and retries with backoff.
- Clear error messages and partial results when sources fail.
