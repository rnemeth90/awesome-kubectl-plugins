# Plugin Ideas

Candidates for future plugins in this repo. Each is small, single-purpose, and testable with fakes.

## Kubernetes-API plugins (template-friendly)

- **`kubectl can-i-all` / `kubectl access`**: list everything a user or service account can do in a namespace, using SelfSubjectRulesReview.
- **`kubectl resources`**: compare requests and limits with namespace or node capacity, and find pods with no limits.
- **`kubectl age`**: list resources older or newer than a given duration, useful for cleanup.
- **`kubectl nodeinfo`**: node summary with allocatable vs. requested resources, taints and conditions.
- **`kubectl events`**: recent warning events sorted by time, across namespaces.
- **`kubectl secrets-used` / `kubectl whouses`**: given a ConfigMap, Secret or PVC, list the pods and workloads that reference it (inverse of `orphans`).
- **`kubectl diff-ns`**: compare resources between two namespaces or contexts, e.g. staging vs. prod.
- **`kubectl certs`**: list TLS secrets and cert-manager Certificates with expiry dates, soonest first.
- **`kubectl probes`**: find workloads missing readiness or liveness probes, or with risky probe settings.
- **`kubectl pdb`**: show PodDisruptionBudgets that would block a node drain.

## Security and hygiene

- **`kubectl privileged`**: find pods running privileged, as root, with hostPath/hostNetwork, or without a securityContext.
- **`kubectl rbac-who`**: show who can perform a verb on a resource by walking RoleBindings and ClusterRoleBindings.
- **`kubectl deprecated`**: flag resources using API versions deprecated or removed in a target Kubernetes version; useful before AKS upgrades.

## More AKS plugins

- **`kubectl aks-upgrade-check`**: pre-upgrade checks combining `deprecated`, PDBs, node pool surge settings and available versions.
- **`kubectl aks-quota`**: compare regional vCPU quota with node pool usage and autoscaler maximums.

# Done

## Kubernetes-API plugins

- **`kubectl images`** (done): list container images in use with pod and namespace counts, and flag `:latest` tags.
- **`kubectl restarts`** (done): show pods ranked by restart count, with last termination reason (OOMKilled, etc.).
- **`kubectl unhealthy`** (done): list pods that aren't Ready or are in CrashLoopBackOff or Pending, with the reason.
- **`kubectl whoami`** (done): show the current user, groups and context, using SelfSubjectReview.
- **`kubectl orphans`** (done): find unused ConfigMaps, Secrets, PVCs and Services with no endpoints.
- **`kubectl ping`** (done): ping an IPv4 address or DNS name from inside a pod.
- **`kubectl pending`** (done): explain why pods are unschedulable (taints, insufficient CPU/memory, affinity), parsed from the scheduler's message.

## AKS plugins (same Azure pattern as `nodepool`)

- **`kubectl aks`** (done): list, show, describe, upgrade, start, stop, rotate-certs.
- **`kubectl aks-versions`** (done): available Kubernetes versions in a region, with `--patches` for upgrade paths.
- **`kubectl aks-maintenance`** (done): list, show, add, delete for `default`, `auto-upgrade` and `node-os` configurations.

## Suggested starting point

`unhealthy`, `images` and `resources`: most useful day to day, and no new dependencies.

## Other open items

- Add a test for the `Continue` pagination path in `count`.
- Update the README to cover `count` and `nodepool`.
