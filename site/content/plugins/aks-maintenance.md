---
title: "aks-maintenance"
description: "Manage AKS planned maintenance windows."
weight: 6
---

```
kubectl aks-maintenance <command> [NAME] [flags]
```

Uses the [shared AKS flags](../aks-targeting/). `NAME` is one of:

| Name | Meaning |
|---|---|
| `default` | Planned maintenance: weekly UTC hour slots |
| `auto-upgrade` | Window for cluster auto-upgrades (`aksManagedAutoUpgradeSchedule`) |
| `node-os` | Window for node OS upgrades (`aksManagedNodeOSUpgradeSchedule`) |

## Commands

`list`, `show NAME`, `add NAME` (create or replace) and `delete NAME`.

## add flags

| Flag | Applies to | Description |
|---|---|---|
| `--day` | all (required) | Day of the week, for example `Sunday` |
| `--start-time` | auto-upgrade, node-os | `HH:mm`, default `02:00` |
| `--duration-hours` | auto-upgrade, node-os | 4 to 24, default 4 |
| `--interval-weeks` | auto-upgrade, node-os | Weeks between windows, default 1 |
| `--utc-offset` | auto-upgrade, node-os | `+/-HH:mm` offset for the start time |
| `--hours` | default | UTC hours 0 to 23, for example `1,2` |

```sh
kubectl aks-maintenance add node-os --day Sunday --start-time 02:00 --duration-hours 4
kubectl aks-maintenance add auto-upgrade --day Saturday --utc-offset -05:00
kubectl aks-maintenance add default --day Monday --hours 1,2
kubectl aks-maintenance list
```

`add` creates weekly schedules only. Existing daily or monthly schedules are shown by `list` and `show` but cannot be created with this plugin.

The binary is named `kubectl-aks_maintenance`; `make install` creates it.
