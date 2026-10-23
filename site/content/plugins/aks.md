---
title: "aks"
description: "Inspect and operate an AKS cluster."
category: AKS
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
# Discover clusters
kubectl aks list
kubectl aks list --subscription 00000000-0000-0000-0000-000000000000
kubectl aks list -g my-rg

# Inspect the cluster behind your current kube context
kubectl aks show
kubectl aks describe
kubectl aks describe -o json

# Upgrade: check what is available, then upgrade
kubectl aks describe
kubectl aks upgrade --kubernetes-version 1.30.4
kubectl aks upgrade --kubernetes-version 1.30.4 --no-wait

# Stop a dev cluster overnight, start it in the morning
kubectl aks stop
kubectl aks start

# Target a cluster explicitly
kubectl aks show --subscription SUB -g my-rg --cluster my-aks

# Rotate certificates without the prompt
kubectl aks rotate-certs -y
```

When `list` searches several subscriptions it skips ones your credential cannot read.
