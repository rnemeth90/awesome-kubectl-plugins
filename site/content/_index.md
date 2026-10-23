---
title: "kubectl-plugins"
---

Small, single-purpose [kubectl plugins](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) written in Go. Pick one below, or see [Install](install/) to get started.

A typical triage session:

```sh
kubectl unhealthy -A        # what is broken?
kubectl pending -A          # why is it not scheduled?
kubectl restarts -A         # what keeps crashing, and why?
kubectl images -A --latest  # which workloads use mutable tags?
kubectl count pods -A       # how big is the cluster?
kubectl orphans -A          # what can be cleaned up?
kubectl whoami              # which user and cluster am I talking to?
kubectl ping POD 8.8.8.8    # can this pod reach the network?
```
