# Plugin Ideas

Candidates for future plugins in this repo. Each is small, single-purpose, and testable with fakes.

## Kubernetes-API plugins (template-friendly)

- **`kubectl whoami`**: show the current user, groups and context, using SelfSubjectReview.
- **`kubectl can-i-all` / `kubectl access`**: list everything a user or service account can do in a namespace, using SelfSubjectRulesReview.
- **`kubectl images`**: list container images in use, grouped by namespace or workload, and flag `:latest` tags.
- **`kubectl restarts`**: show pods ranked by restart count, with last termination reason (OOMKilled, etc.).
- **`kubectl unhealthy`**: list pods that aren't Ready or are in CrashLoopBackOff or Pending, with the reason.
- **`kubectl resources`**: compare requests and limits with namespace or node capacity, and find pods with no limits.
- **`kubectl orphans`**: find unused ConfigMaps, Secrets, PVCs and Services with no endpoints.
- **`kubectl age`**: list resources older or newer than a given duration, useful for cleanup.
- **`kubectl nodeinfo`**: node summary with allocatable vs. requested resources, taints and conditions.
- **`kubectl events`**: recent warning events sorted by time, across namespaces.

## AKS plugins (same Azure pattern as `nodepool`)

- **`kubectl aks`** (done): list, show, describe, upgrade, start, stop, rotate-certs.
- **`kubectl aks-versions`** (done): available Kubernetes versions in a region, with `--patches` for upgrade paths.
- **`kubectl aks-maintenance`** (done): list, show, add, delete for `default`, `auto-upgrade` and `node-os` configurations.

## Suggested starting point

`unhealthy`, `images` and `resources`: most useful day to day, and no new dependencies.

## Other open items

- Add a test for the `Continue` pagination path in `count`.
- Update the README to cover `count` and `nodepool`.
