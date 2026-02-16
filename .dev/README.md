# .dev — private and development-only data

This directory is **git-ignored**. Use it for:

- **API responses** — Raw JSON from RP or other APIs (launch, test items, etc.) that may contain sensitive or environment-specific data.
- **Envelope copies** — Local copies of execution envelopes for development; canonical fixtures live in `examples/`.
- **Secrets and keys** — e.g. a copy or symlink of `.rp-api-key` if you prefer to keep it under `.dev/`.
- **Local artifacts** — Artifacts, prompts, or DBs produced during development that you don’t want in the repo.

Do **not** commit `.dev/` or its contents. The repo uses `examples/` for committed fixtures and `internal/*/testdata/` for test data.
