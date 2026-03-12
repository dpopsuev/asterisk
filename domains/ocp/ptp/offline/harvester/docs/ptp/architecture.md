# PTP Operator Architecture

## Component Hierarchy

- **ptp-operator** — Kubernetes operator managing PTP resources
  - Creates and manages `PtpConfig` CRDs
  - Deploys `linuxptp-daemon` DaemonSet on selected nodes
  - Watches for config changes and propagates to daemons

- **linuxptp-daemon** (pod) — DaemonSet running on worker nodes
  - Runs `ptp4l` (PTP protocol engine)
  - Runs `phc2sys` (PHC-to-system clock sync)
  - Runs `ts2phc` (timestamping)
  - Communicates with `cloud-event-proxy` via Unix socket `/cloud-native/events.sock`

- **cloud-event-proxy** — Sidecar container in linuxptp-daemon pod
  - Receives PTP events from daemon via Unix socket
  - Publishes CloudEvents to consumers (HTTP/AMQP)
  - Maintains consumer subscriptions

## Event Flow

```
ptp4l → linuxptp-daemon → events.sock (Unix socket) → cloud-event-proxy → HTTP consumers
```

## Known Failure Modes

1. **Broken pipe (EPIPE)**: Under burst conditions (node reboot, interface down),
   the pipe buffer between daemon and proxy clogs. Daemon gets EPIPE on write,
   events are lost or concatenated at receiver.

2. **Config change hang**: When PTP config changes, daemon kills child processes
   (ptp4l, phc2sys) but may fail to restart them, leaving the node without PTP sync.

3. **Consumer subscription loss**: After pod restart or cold reboot, consumers
   lose their event subscriptions and must re-register.

## Disambiguation

- **linuxptp-daemon** (repo) — the Go source code for the daemon binary
- **linuxptp-daemon** (pod) — the running DaemonSet pod on a node
- These are the same component at different abstraction levels
