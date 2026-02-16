# RCA prompt

**Launch:** {{.LaunchID}}  
**Case / failure:** {{.CaseID}} — {{.FailedTestName}}  
**Workspace:** {{.WorkspacePath}}  
**Artifact output path:** {{.ArtifactPath}}

---

Perform root-cause analysis for the failed test above using the context workspace (repos, logs, and code at the paths/URLs listed there). Use the envelope’s branch/commit when resolving refs unless the workspace file overrides per repo.

**Output:** Produce:

1. **RCA message** — Short summary of the root cause.
2. **Convergence score** — Confidence 0–1 that the cause is correct.
3. **Defect type** — One of: To investigate (ti001), Product bug (pb001), Automation bug (ab001), System issue (si001), No defect (nd001), or project-specific subtype.
4. **Evidence refs** — Paths or links to logs, commits, or files that support the RCA.

Write the result in **artifact** format (JSON): `launch_id`, `case_ids`, `rca_message`, `defect_type`, `convergence_score`, `evidence_refs`. See `docs/artifact-schema.mdc`. Save to `{{.ArtifactPath}}` or return the JSON so the CLI can write it.
