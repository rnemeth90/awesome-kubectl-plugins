---
title: "aks-versions"
description: "List Kubernetes versions available in an Azure region."
weight: 5
---

```
kubectl aks-versions [--location REGION] [--patches]
```

Prints the versions AKS offers in a region, newest first, with default and preview flags, support plan and patch versions. Without `--location` it uses the region of the cluster in your current kube context (see [AKS targeting](../aks-targeting/)).

| Flag | Description |
|---|---|
| `--location` | Azure region, for example `eastus` |
| `--patches` | One row per patch version, with the versions it can upgrade to |
| `-o json` | JSON output |

```sh
kubectl aks-versions
kubectl aks-versions --location westeurope --patches
```

The binary is named `kubectl-aks_versions`; `make install` creates it.
