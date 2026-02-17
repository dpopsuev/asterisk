# SEC-005 — unsigned-manifests-artifacts

**Status:** accepted-risk  
**Severity:** medium (CVSS-aligned)  
**OWASP category:** A08 Data Integrity

## Summary

Batch manifest (`batch-manifest.json`), signal files (`signal.json`), and artifact files have no integrity verification (signatures, HMACs, or checksums). A local attacker with write access to the calibration directory could tamper with manifests to redirect signals, modify artifacts to change RCA conclusions, or inject false budget status.

## Root cause analysis

- **Component:** `internal/calibrate/batch_manifest.go`, `internal/calibrate/dispatcher.go`
- **Trust boundary:** Filesystem → application logic
- **Root cause:** The protocol trusts filesystem integrity. No cryptographic verification is applied to any coordination or data files.

## Impact

- **Confidentiality:** N/A (no secrets in manifests).
- **Integrity:** Tampered artifacts lead to incorrect RCA conclusions. Tampered manifests could redirect subagent work.
- **Availability:** Corrupted manifests could halt the pipeline.

## Reproduction

```bash
# Modify an artifact to change the RCA
echo '{"dispatch_id":1,"data":{"match":true,"confidence":0.99}}' > .asterisk/calibrate/1/1/recall-result.json
```

## Mitigation

Accepted risk for the current PoC/calibration phase:
- The calibration directory is local and controlled by the developer.
- Adding HMAC/signatures would add complexity without clear benefit for the current threat model.
- Document that the calibration directory must be trusted.

For future production use:
1. Add HMAC to manifest and artifact wrappers using a session key.
2. Verify HMAC before processing any file.
3. Use filesystem ACLs to restrict write access to the calibration directory.

## Lessons learned

- Document trust assumptions explicitly in protocol docs.
- Plan integrity checks for production deployments.
