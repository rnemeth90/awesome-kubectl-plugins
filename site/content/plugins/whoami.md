---
title: "whoami"
description: "Show the current user, groups and context."
category: Kubernetes
weight: 12
---

```
kubectl whoami [-o json]
```

Asks the API server who it thinks you are (via a `SelfSubjectReview`) and prints the kubeconfig context, cluster, server, username, UID, groups and any extra attributes. Requires Kubernetes 1.28 or newer.

## Flags

| Flag | Description |
|---|---|
| `-o, --output` | Output format: `json` |
| `--context` | Kubeconfig context to use |

## Examples

```sh
kubectl whoami
kubectl whoami --context prod
kubectl whoami -o json
```
