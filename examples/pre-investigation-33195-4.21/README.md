# Example: Pre-investigation — Launch 33195 (4.21)

Example data from **Pre-investigation** (fetch from RP via curl; manual structuring). Use for tests/fixture or as reference for Execution Envelope + failure list shape.

**Source:** Report Portal (Execution DB), project `ecosystem-qe`, launch ID **33195** — telco-ft-ran-ptp-4.21.

## Files

| File | Description |
|------|-------------|
| `launch_33195_raw.json` | Raw launch response from RP API (`GET .../api/v1/ecosystem-qe/launch/33195`). |
| `items_33195_page1.json` | Test items for launch (page 1; 180 items). `GET .../item?filter.eq.launchId=33195&page.size=200&page.page=1`. |
| `items_33195_failed.json` | Failed test items only (12 items). `GET .../item?filter.eq.launchId=33195&filter.eq.status=FAILED&page.size=200&page.page=1`. |
| `envelope_33195_4.21.json` | **Manually structured Execution Envelope + failure list** (run_id, name, status, times, statistics, attributes, git placeholder, failure_list). Use as fixture for `analyze` or envelope-from-file tests. |

## Curl (reproduce)

```bash
BASE="https://your-reportportal.example.com"
KEY=$(cat .rp-api-key)
curl -s -H "Authorization: Bearer $KEY" "${BASE}/api/v1/ecosystem-qe/launch/33195" -o launch_33195_raw.json
curl -s -H "Authorization: Bearer $KEY" "${BASE}/api/v1/ecosystem-qe/item?filter.eq.launchId=33195&page.size=200&page.page=1" -o items_33195_page1.json
curl -s -H "Authorization: Bearer $KEY" "${BASE}/api/v1/ecosystem-qe/item?filter.eq.launchId=33195&filter.eq.status=FAILED&page.size=200&page.page=1" -o items_33195_failed.json
```

Envelope built manually from launch + failed items (see `.cursor/notes/pre-investigation-33195-4.21.mdc`).
