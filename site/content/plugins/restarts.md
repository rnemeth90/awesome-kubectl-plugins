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

## Sample output

```
$ kubectl restarts -A --limit 3
NAMESPACE   NAME                  RESTARTS  CONTAINER  LAST REASON  EXIT  LAST RESTART
monitoring  exporter-5d8f7-k2p9z  41        exporter   OOMKilled    137   6m ago
default     api-7d9c8b6f5-x2k4p   17        api        Error        1     2h ago
jobs        sync-28391-abcde      3         sync       Error        2     1d ago
```

An exit code of `137` with `OOMKilled` means the container hit its memory limit; raise the limit or fix the leak. `Error` with code `1` or `2` is the application exiting, so check `kubectl logs --previous`.

Restart counts and termination info come from the pod status, so they reset when a pod is recreated.
