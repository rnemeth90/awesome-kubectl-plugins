---
title: "pending"
description: "Explain why pods are unschedulable: taints, insufficient CPU or memory, affinity."
category: Kubernetes
weight: 10
---

```
kubectl pending [-n NAMESPACE | -A] [-l SELECTOR]
```

Finds pods the scheduler could not place (phase `Pending`, no node assigned, `PodScheduled=False` with reason `Unschedulable`) and summarizes the scheduler's message into a short cause plus the per-node reasons.

## Causes

| Cause | Meaning |
|---|---|
| `Insufficient cpu`, `Insufficient memory`, ... | No node has enough of that resource free |
| `Taints` | Nodes have taints the pod does not tolerate |
| `NodeAffinity` | Node selector or node affinity matched no node |
| `PodAffinity` | Pod affinity, anti-affinity or topology spread rules cannot be met |
| `Cordoned` | Nodes are marked unschedulable |
| `Volumes` | PersistentVolumeClaim is unbound or volume node affinity conflicts |
| `HostPorts` | Requested host ports are already in use |
| `Other` | Anything not recognized; see DETAIL |

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace to scan (default: current context namespace) |
| `-A, --all-namespaces` | Scan every namespace and add a NAMESPACE column |
| `-l, --selector` | Only scan pods matching a label selector |

## Examples

```sh
kubectl pending
kubectl pending -A
kubectl pending -l app=web
```

Prints `No unschedulable pods.` when nothing is stuck. Pods that are scheduled but still starting (image pulls, init containers) are not shown; use [`unhealthy`](../unhealthy/) for those.
