---
title: "count"
description: "Count resources of any type in a namespace or cluster."
category: Kubernetes
weight: 1
---

```
kubectl count RESOURCE [-n NAMESPACE | -A] [-l SELECTOR]
```

`RESOURCE` can be any type the cluster knows about, using the plural, singular or short name (`pods`, `svc`, `deploy`, `resource.group`). The type is resolved through API discovery, so custom resources work too. Counts use metadata-only listing, which keeps them cheap on large clusters.

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace to count in (default: current context namespace) |
| `-A, --all-namespaces` | Count across all namespaces and print a per-namespace breakdown |
| `-l, --selector` | Only count objects matching a label selector |

The standard kubectl flags (`--context`, `--kubeconfig`, and so on) also work. Cluster-scoped resources such as nodes ignore the namespace.

## Examples

```sh
kubectl count pods
kubectl count services -n kube-system
kubectl count deployments -A
kubectl count pods -l app=web
kubectl count nodes
```

Sample output:

```
Pod in namespace "default": 12
```

With `-A` the total is followed by one line per namespace.
