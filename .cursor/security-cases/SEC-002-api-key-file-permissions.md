# SEC-002 — api-key-file-permissions

**Status:** open  
**Severity:** medium (CVSS-aligned)  
**OWASP category:** A02 Cryptographic Failures

## Summary

The `.rp-api-key` file stores the ReportPortal API token in plaintext. Asterisk does not create this file (the user does), but it also does not warn when the file has overly permissive permissions (e.g., `0644` world-readable). On shared systems, other users could read the token.

## Root cause analysis

- **Component:** `internal/rp/client.go` (`ReadAPIKey` function)
- **Trust boundary:** Filesystem permissions → secret exposure
- **Root cause:** No file permission check on API key file before reading.

## Impact

- **Confidentiality:** API token could be exposed to other local users.
- **Integrity:** Compromised token could be used to modify RP data.
- **Availability:** N/A.

## Reproduction

```bash
echo "my-secret-token" > .rp-api-key
chmod 644 .rp-api-key
# Any local user can now: cat .rp-api-key
```

## Mitigation

1. Add a permission check in `ReadAPIKey`: warn if file mode is more permissive than `0600`.
2. Document in README that `.rp-api-key` should have `chmod 0600`.
3. If Asterisk ever creates the file, use `os.WriteFile(path, data, 0600)`.
4. Consider supporting environment variables (`ASTERISK_RP_TOKEN`) as an alternative to file-based keys.

## Lessons learned

- Secret files should always validate permissions before use.
- Document security requirements for configuration files.
