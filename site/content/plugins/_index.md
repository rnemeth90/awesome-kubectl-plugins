---
title: "Plugins"
---

Reference for each plugin, with flags and examples.

## Kubernetes plugins

These use only your kubeconfig and the Kubernetes API. All of them accept the standard kubectl flags (`--context`, `--kubeconfig`, `-n`).

| Plugin | Use it to | Example |
|---|---|---|
| [`count`](count/) | Count any resource type | `kubectl count pods -A` |
| [`unhealthy`](unhealthy/) | Find pods that are broken, and why | `kubectl unhealthy -A` |
| [`pending`](pending/) | Explain why pods are unschedulable | `kubectl pending -A` |
| [`restarts`](restarts/) | Rank pods by restarts and last termination reason | `kubectl restarts -A --min 5` |
| [`images`](images/) | List images in use and flag mutable tags | `kubectl images -A --latest` |
| [`orphans`](orphans/) | Find unused ConfigMaps, Secrets, PVCs and Services | `kubectl orphans -A` |
| [`whoami`](whoami/) | Show who the API server thinks you are | `kubectl whoami` |
| [`ping`](ping/) | Ping an address or DNS name from inside a pod | `kubectl ping web-7d4b9 8.8.8.8` |

## AKS plugins

These call Azure's management API. Read [AKS targeting](aks-targeting/) first for the shared flags and authentication.

| Plugin | Use it to | Example |
|---|---|---|
| [`aks`](aks/) | List, describe, upgrade, start and stop clusters | `kubectl aks describe` |
| [`nodepool`](nodepool/) (`np`) | Manage node pools | `kubectl np scale userpool --count 5` |
| [`aks-versions`](aks-versions/) | See available Kubernetes versions | `kubectl aks-versions --patches` |
| [`aks-maintenance`](aks-maintenance/) | Manage maintenance windows | `kubectl aks-maintenance list` |

## Common workflows

**Why is my deployment not running?**

```sh
kubectl pending -l app=web      # stuck in the scheduler?
kubectl unhealthy -l app=web    # scheduled but crashing or not Ready?
kubectl restarts -l app=web     # how often, and why did it last die?
```

**Can this pod reach the outside world?**

```sh
kubectl ping web-7d4b9 8.8.8.8                       # raw egress
kubectl ping web-7d4b9 my-svc.my-ns.svc.cluster.local  # DNS and in-cluster routing
```

**Wrong cluster, wrong user?**

```sh
kubectl whoami --context prod   # confirm identity and groups before changing anything
```

**Cleaning up a namespace**

```sh
kubectl orphans -n staging      # unused ConfigMaps, Secrets, PVCs and dead Services
kubectl images -n staging --latest
```

**Before an AKS upgrade**

```sh
kubectl aks describe                 # current and available versions
kubectl aks-versions --patches       # upgrade paths in this region
kubectl unhealthy -A                 # fix existing problems first
kubectl aks upgrade --kubernetes-version 1.30.4
```
