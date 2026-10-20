---
title: "restarts"
description: "Rank pods by container restarts, with the last termination reason."
category: Kubernetes
weight: 8
---

```
kubectl restarts [-n NAMESPACE | -A] [-l SELECTOR] [--min N] [--limit N]
```

Lists pods sorted by total container restarts (init containers included), highest first. For each pod it shows the container that terminated most recently, why (`OOMKilled`, `Error`, ...), its exit code and when it happened.

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace to scan (default: current context namespace) |
| `-A, --all-namespaces` | Scan every namespace and add a NAMESPACE column |
| `-l, --selector` | Only scan pods matching a label selector |
| `--min` | Only show pods with at least this many restarts (default 1) |
| `--limit` | Maximum rows to show (default 20, 0 for no limit) |

## Examples

```sh
kubectl restarts
kubectl restarts -A --limit 10
kubectl restarts -A --min 5
```

Restart counts and termination info come from the pod status, so they reset when a pod is recreated.
