---
title: "aks-versions"
description: "List Kubernetes versions available in an Azure region."
category: AKS
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
# Region of the cluster in your current kube context
kubectl aks-versions

# A specific region, with every patch version and its upgrade targets
kubectl aks-versions --location westeurope --patches

# Machine-readable, for scripts
kubectl aks-versions --location eastus -o json
```

The binary is named `kubectl-aks_versions`; `make install` creates it.
