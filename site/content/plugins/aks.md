---
title: "aks"
description: "Inspect and operate an AKS cluster."
weight: 4
---

```
kubectl aks <command> [flags]
```

Uses the [shared AKS flags](../aks-targeting/).

| Command | Description |
|---|---|
| `list` | List AKS clusters across your subscriptions (or `--subscription`, `-g`) |
| `show` | Summary of the current cluster |
| `describe` | Summary plus available upgrade versions for the control plane and each pool |
| `upgrade --kubernetes-version V` | Upgrade the cluster |
| `start` / `stop` | Start or stop the cluster; does nothing if it is already in that state |
| `rotate-certs` | Rotate cluster certificates (asks to confirm unless `-y`) |

```sh
kubectl aks list
kubectl aks describe
kubectl aks upgrade --kubernetes-version 1.30.4 --no-wait
```

When `list` searches several subscriptions it skips ones your credential cannot read.
