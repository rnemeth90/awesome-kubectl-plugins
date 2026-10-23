---
title: "orphans"
description: "Find unused ConfigMaps, Secrets, PVCs and Services with no endpoints."
category: Kubernetes
weight: 13
---

```
kubectl orphans [-n NAMESPACE | -A] [--type configmaps,secrets,pvcs,services]
```

Reports resources that look unused, which is handy for cleanup:

- **ConfigMaps, Secrets** that no pod references (volumes, projected volumes, `env`, `envFrom`, image pull secrets).
- **PVCs** that no pod mounts.
- **Services** (other than `ExternalName`) with no ready endpoints.

Service account token Secrets, Helm release Secrets and `kube-root-ca.crt` are skipped.

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace to scan (default: current context namespace) |
| `-A, --all-namespaces` | Scan every namespace and add a NAMESPACE column |
| `--type` | Comma-separated types to check (default: all four) |

## Examples

```sh
kubectl orphans
kubectl orphans -A
kubectl orphans --type secrets,pvcs
kubectl orphans -n staging --type services
```

## Sample output

```
$ kubectl orphans -A
NAMESPACE   KIND      NAME             REASON
default     ConfigMap old-settings     not referenced by any pod
default     PVC       data-old-0       not mounted by any pod
staging     Secret    legacy-api-key   not referenced by any pod
staging     Service   canary           no ready endpoints
```

Prints `No orphaned resources found.` when everything is in use. Once you have reviewed the list, delete with `kubectl delete`, for example `kubectl delete pvc data-old-0 -n default`.

Only running pods are checked, so resources used solely by scaled-to-zero workloads or CronJobs between runs are reported too. Review before deleting.
