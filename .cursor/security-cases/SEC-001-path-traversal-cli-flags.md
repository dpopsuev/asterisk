# SEC-001 — path-traversal-cli-flags

**Status:** open  
**Severity:** high (CVSS-aligned)  
**OWASP category:** A01 Broken Access Control

## Summary

CLI flags (`--rp-api-key`, `--launch`, `-o`, `-f`, `--workspace`) accept arbitrary file paths without validation. User-supplied paths are passed directly to `os.ReadFile` and `os.WriteFile` without `filepath.Clean`, `filepath.Abs`, or directory containment checks. This allows reading arbitrary files (e.g., `--rp-api-key=../../../etc/passwd`) or writing to arbitrary locations (e.g., `-o ../../../tmp/overwrite`).

## Root cause analysis

- **Component:** `cmd/asterisk/main.go` (CLI flag processing)
- **Trust boundary:** CLI input → filesystem operations
- **Root cause:** Missing input validation on user-supplied paths. The CLI trusts that the user provides reasonable paths, but no enforcement exists.

## Impact

- **Confidentiality:** Can read any file the process user has access to (API keys, system files).
- **Integrity:** Can overwrite files at arbitrary paths.
- **Availability:** Overwriting critical files could disrupt other services.

Note: This is a CLI tool, not a network service. The attacker must have local execution access to run the CLI with malicious flags. Severity is high for shared-system deployments; lower for single-user workstation use.

## Reproduction

```bash
# Read an arbitrary file via --rp-api-key
asterisk analyze --launch=1 --rp-api-key=/etc/shadow -o /dev/null

# Write to an arbitrary location
asterisk analyze --launch=examples/envelope.json -o /tmp/arbitrary-write.json
```

## Mitigation

1. Apply `filepath.Clean()` and `filepath.Abs()` to all user-supplied paths.
2. For `--rp-api-key`, restrict to CWD or `~/.asterisk/` directory.
3. For artifact output (`-o`, `-f`), ensure resolved path stays under CWD or a configurable root.
4. Add unit tests for path traversal attempts (e.g., `../../etc/passwd`).

## Lessons learned

- All CLI path flags should have validation as a standard practice.
- Add a `validatePath(basedir, userPath)` helper to the codebase.
