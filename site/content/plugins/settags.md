---
title: "settags"
description: "Update image tags for named deployments across many namespaces."
category: Kubernetes
weight: 10
---

```
kubectl set-tags [-n NS[,NS...] | -A] [-f FILE] [--dry-run] [-y] NAME=TAG [NAME=TAG ...]
```

Finds every deployment whose name matches a `NAME` and changes the image tag to `TAG`. Only the tag is replaced; the repository is kept. Searches all namespaces unless `-n` is given.

If a deployment has several containers, those sharing the first container's image repository are updated and the rest (sidecars) are left alone. A plan table is shown and confirmation is requested before anything changes.

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Only these namespaces (comma-separated). Default: all |
| `-A, --all-namespaces` | All namespaces (the default; overrides `-n`) |
| `-f, --file` | File with one `NAME=TAG` per line (`#` comments allowed) |
| `--dry-run` | Show the plan and exit |
| `-y, --yes` | Skip the confirmation prompt |

## Examples

```sh
kubectl set-tags api=v1.4.2 web=v2.0.0
kubectl set-tags -f tags.txt --dry-run
kubectl set-tags -n team-a,team-b -y api=v1.4.2
```
