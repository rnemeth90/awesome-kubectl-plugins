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
kubectl whoami --kubeconfig ~/.kube/other-config
```

## Sample output

```
$ kubectl whoami
Context:  prod-aks
Cluster:  prod-aks
Server:   https://prod-aks-dns-1a2b3c.hcp.eastus.azmk8s.io:443
Username: jane@example.com
UID:      3f5c8a1e-0000-0000-0000-000000000000
Groups:
  system:authenticated
  4b1d0c2e-0000-0000-0000-000000000000
Extra:
  oid: [3f5c8a1e-0000-0000-0000-000000000000]
```

Use `-o json` to feed the result into scripts. If the command fails on an older cluster, the API server does not support `SelfSubjectReview`.
