---
title: "unhealthy"
description: "List pods that are not Ready, crash-looping, pending or failed, with the reason."
category: Kubernetes
weight: 7
---

```
kubectl unhealthy [-n NAMESPACE | -A] [-l SELECTOR]
```

Scans pods and prints only the ones that need attention. Completed pods (`Succeeded`) and fully Ready pods are skipped.

## What counts as unhealthy

| Status | Meaning |
|---|---|
| `CrashLoopBackOff`, `ImagePullBackOff`, ... | A container is waiting with that reason (init containers included) |
| `Error`, `OOMKilled`, ... | A container terminated with a non-zero exit code |
| `Failed` | The pod phase is Failed |
| `Pending` | Not scheduled or not started, with the scheduler's reason when available |
| `Terminating` | The pod has a deletion timestamp |
| `NotReady` | Running, but not all containers are Ready |

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace to scan (default: current context namespace) |
| `-A, --all-namespaces` | Scan every namespace and add a NAMESPACE column |
| `-l, --selector` | Only scan pods matching a label selector |

## Examples

```sh
kubectl unhealthy
kubectl unhealthy -A
kubectl unhealthy -l app=web
```

## Sample output

```
$ kubectl unhealthy -A
NAMESPACE   NAME                    READY  STATUS            REASON
default     api-7d9c8b6f5-x2k4p     0/1    CrashLoopBackOff  container api: back-off 5m0s restarting failed container
default     web-5c6d7f9b8-q9w7z     0/1    ImagePullBackOff  container web: Back-off pulling image "web:v9"
jobs        report-28391           0/1    OOMKilled         container report exited with code 137
staging     worker-6f4b5c7d9-m3n8b  1/2    NotReady          containers with unready status: [sidecar]
```

Prints `All pods healthy.` when nothing is wrong.

## Tips

- Combine with `-l` to check one app: `kubectl unhealthy -l app=api`.
- For pods stuck in `Pending` because of scheduling, [`pending`](../pending/) gives a more specific breakdown.
- For pods that keep dying, [`restarts`](../restarts/) shows how often and the last exit reason.
