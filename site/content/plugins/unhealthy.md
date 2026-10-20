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

Prints `All pods healthy.` when nothing is wrong.
