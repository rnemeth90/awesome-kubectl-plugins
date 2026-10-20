---
title: "nodepool"
description: "Manage AKS node pools (also available as kubectl np)."
category: AKS
weight: 3
---

```
kubectl nodepool <command> [NAME] [flags]
kubectl np <command> [NAME] [flags]
```

Uses the [shared AKS flags](../aks-targeting/). Mutating commands wait for completion unless `--no-wait` is set.

## Commands

| Command | Description |
|---|---|
| `list` | List node pools |
| `show NAME` | Summary of one pool |
| `describe NAME` | Full properties plus latest node image and available upgrade versions |
| `add NAME` | Create a pool |
| `delete NAME` | Delete a pool (asks to confirm unless `-y`) |
| `scale NAME --count N` | Set the node count |
| `update NAME` | Change autoscaler, labels, taints, max surge or mode |
| `upgrade NAME --kubernetes-version V` | Upgrade the Kubernetes version |
| `upgrade-node-image NAME` | Upgrade the node image |
| `start NAME` / `stop NAME` | Start or stop a pool |
| `taint NAME SPEC...` | Add or remove taints |
| `untaint NAME KEY[:EFFECT]...` | Remove taints |
| `cancel-operation NAME` | Cancel the latest operation |

## add

```sh
kubectl nodepool add gpu --vm-size Standard_NC6s_v3 \
  --enable-cluster-autoscaler --min-count 1 --max-count 5 \
  --labels team=ml --taints sku=gpu:NoSchedule
```

Flags: `--vm-size`, `--count`, `--mode User|System`, `--kubernetes-version`, `--labels`, `--taints`, `--enable-cluster-autoscaler`, `--min-count`, `--max-count`, `--max-surge`, `--os-type`, `--max-pods`, `--zones`, `--vnet-subnet-id`, `--os-disk-size`.

## scale and the autoscaler

`scale` refuses a pool that has the cluster autoscaler enabled. Use `update` to change `--min-count` and `--max-count`, or disable the autoscaler first:

```sh
kubectl nodepool update userpool --enable-cluster-autoscaler --min-count 2 --max-count 6
kubectl nodepool update userpool --disable-cluster-autoscaler
```

## Taints

`taint` uses kubectl's syntax. A trailing `-` removes.

```sh
kubectl nodepool taint gpu sku=gpu:NoSchedule    # add (replaces same key and effect)
kubectl nodepool taint gpu sku-                  # remove every taint with key sku
kubectl nodepool taint gpu sku:NoSchedule-       # remove one
kubectl nodepool untaint gpu sku:NoSchedule      # same as the line above
```

Effects are `NoSchedule`, `PreferNoSchedule` and `NoExecute`. Removing a taint that does not exist is an error. `update --taints` replaces the whole list instead; an empty value clears it.
