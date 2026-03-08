# PTP Operator Architecture — Agent Context

Source: https://docs.redhat.com/en/documentation/openshift_container_platform/{version}/html/advanced_networking/using-ptp-hardware

## Pod topology

The PTP Operator manages a DaemonSet that creates `linuxptp-daemon-XXXXX` pods on
every PTP-capable node. Each pod runs up to 3 containers:

| # | Container              | Process(es)                  | Function                                                    |
|---|------------------------|------------------------------|-------------------------------------------------------------|
| 1 | linuxptp-daemon-container | ptp4l, phc2sys, ts2phc      | PTP protocol: clock sync, boundary clock, grandmaster clock |
| 2 | cloud-event-proxy      | cloud-event-proxy             | Sidecar: publishes PTP events via REST API v2               |
| 3 | gpsd (optional)        | gpsd, ubxtool                 | GNSS receiver daemon for T-GM configurations                |

When PTP fast event notifications are enabled, `linuxptp-daemon` pods show `3/3`
ready containers. The third container is `cloud-event-proxy`.

## Naming disambiguation

The term "linuxptp-daemon" is overloaded. It refers to three different things
depending on context:

| Context           | "linuxptp-daemon" means                                                        |
|-------------------|--------------------------------------------------------------------------------|
| **Pod name**      | The DaemonSet-managed pod `linuxptp-daemon-XXXXX` containing ALL containers    |
| **Container**     | Container #1 inside the pod, running ptp4l/phc2sys/ts2phc                      |
| **Repository**    | The Go source repo `github.com/openshift/linuxptp-daemon` (container #1 code)  |

CRITICAL: When error logs or test output reference "linuxptp-daemon", determine
which meaning applies before selecting a repository.

## Component-to-repository mapping

| Component              | Repository           | Contains                                               |
|------------------------|----------------------|--------------------------------------------------------|
| PTP Operator           | ptp-operator         | Operator lifecycle, CRDs, DaemonSet management         |
| linuxptp daemon        | linuxptp-daemon      | ptp4l, phc2sys, ts2phc wrappers; config change handler |
| Cloud Event Proxy      | cloud-event-proxy    | PTP event publisher; REST API v2; metrics exporter     |
| PTP test suite         | cnf-gotests          | Ginkgo test cases, assertions, test helpers            |
| ZTP/deployment config  | cnf-features-deploy  | PtpConfig CRs, phc2sys options, deployment manifests   |

## Common confusion patterns

| Error mentions                          | Naive choice        | Correct choice       | Why                                                        |
|-----------------------------------------|---------------------|----------------------|------------------------------------------------------------|
| "cloud events", "HTTP publisher"        | linuxptp-daemon     | cloud-event-proxy    | Event proxy is a sidecar IN the linuxptp-daemon pod        |
| "GNSS sync state not mapped to event"   | linuxptp-daemon     | cloud-event-proxy    | Event mapping logic lives in cloud-event-proxy code        |
| "metrics missing after sidecar restart" | linuxptp-daemon     | cloud-event-proxy    | "sidecar" = cloud-event-proxy container                    |
| "process status collected too early"    | linuxptp-daemon     | cloud-event-proxy    | Status collection happens in the event proxy               |
| "ptp4l FREERUN", "phc2sys offset"       | cloud-event-proxy   | linuxptp-daemon      | ptp4l/phc2sys are daemon processes, not proxy              |
| "PtpConfig CR", "DaemonSet management"  | linuxptp-daemon     | ptp-operator         | Operator manages config; daemon executes it                |

## Repo selection guidance

| Triage hypothesis        | Error keywords                                    | Preferred repo      |
|--------------------------|---------------------------------------------------|---------------------|
| Product bug (pb001)      | ptp4l, phc2sys, ts2phc, clock sync, offset        | linuxptp-daemon     |
| Product bug (pb001)      | cloud event, HTTP, REST API, metrics, publisher   | cloud-event-proxy   |
| Product bug (pb001)      | operator, CRD, DaemonSet, PtpConfig               | ptp-operator        |
| Automation bug (au001)   | test assertion, Ginkgo, test helper, framework    | cnf-gotests         |
| Environment issue (en001)| NTP, GNSS, node, network, cluster config          | cnf-features-deploy |
| Firmware issue (fw001)   | NIC, FPGA, PHC, E810, DPLL, hardware              | linuxptp-daemon     |

## Key architecture facts

- The PTP Operator uses `NodePtpDevice` CRD to discover PTP-capable NICs.
- `PtpConfig` CR configures `linuxptp` services per node profile.
- `cloud-event-proxy` communicates with `linuxptp-daemon-container` via Unix socket.
- PTP events REST API v2 is served by `cloud-event-proxy` on each node.
- NTP (`chronyd`) can be configured as failover for GNSS in T-GM configurations.
- Intel E810 Westport Channel NICs are required for GNSS/grandmaster clock support.
