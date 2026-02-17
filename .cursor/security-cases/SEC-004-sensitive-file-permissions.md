# SEC-004 — sensitive-file-permissions

**Status:** open  
**Severity:** medium (CVSS-aligned)  
**OWASP category:** A05 Security Misconfiguration

## Summary

Artifact files, token reports, signal files, manifest files, and briefing files are all written with `0644` permissions (world-readable). On shared systems, these files may contain investigation data, prompt content, or token usage metrics that should not be accessible to other users.

## Root cause analysis

- **Component:** Multiple files across `cmd/asterisk/main.go`, `internal/calibrate/dispatcher.go`, `internal/calibrate/batch_dispatcher.go`, `internal/calibrate/batch_manifest.go`
- **Trust boundary:** File system permissions
- **Root cause:** Default `0644` permissions used consistently without considering data sensitivity.

## Impact

- **Confidentiality:** Investigation data, error messages, RCA details, and token usage could be exposed.
- **Integrity:** Other users could modify artifacts or manifests (0644 doesn't prevent write by owner, but doesn't restrict read).
- **Availability:** N/A.

## Reproduction

Any file written by Asterisk (artifacts, manifests, briefings, token reports) is readable by all local users.

## Mitigation

1. Use `0600` for files containing investigation data or secrets (artifacts, token reports, briefings).
2. Keep `0644` only for coordination files that need multi-process access (signal.json, manifest) if required.
3. Create a `writeSecureFile` helper that defaults to `0600`.
4. Add a `--umask` flag or respect the process umask.

## Lessons learned

- Use principle of least privilege for file permissions.
- Create helper functions that enforce secure defaults.
