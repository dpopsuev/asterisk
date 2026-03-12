# Offline Harvester Bundle

Pre-staged harvester sources for offline calibration. When running with
`--mode=offline`, the harvester circuit reads from this directory instead
of cloning repos and fetching docs live.

## Structure

```
harvester/
├── repos/
│   ├── linuxptp-daemon/     # relevant source files
│   ├── cloud-event-proxy/   # relevant source files
│   ├── cnf-gotests/         # test code for cascade analysis
│   └── cnf-features-deploy/ # phc2sys config
├── docs/
│   └── ptp/
│       └── architecture.md  # PTP operator architecture doc
└── manifest.yaml            # capture metadata
```

## Capture

Run the capture script to snapshot the current state of all repos:

```bash
just capture-harvester
```

This clones each repo at the branch specified in `source_pack`, extracts
relevant files (based on RCA evidence refs), and freezes them here.

## Refresh

Re-run `just capture-harvester` to update snapshots when repos change.
The manifest records the commit SHA at capture time for reproducibility.
