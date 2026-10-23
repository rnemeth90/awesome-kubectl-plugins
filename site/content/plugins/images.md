---
title: "images"
description: "List container images in use and flag :latest or untagged ones."
category: Kubernetes
weight: 9
---

```
kubectl images [-n NAMESPACE | -A] [-l SELECTOR] [--latest]
```

Groups every container image (init containers included) used by the selected pods. Each row shows the image, how many pods use it and which namespaces. The most widely used images come first.

Images tagged `:latest` or with no tag are marked `mutable tag`, since they can change underneath you. Images pinned by digest (`@sha256:...`) are not flagged.

## Flags

| Flag | Description |
|---|---|
| `-n, --namespace` | Namespace to scan (default: current context namespace) |
| `-A, --all-namespaces` | Scan every namespace |
| `-l, --selector` | Only scan pods matching a label selector |
| `--latest` | Only show images with a mutable tag |

## Examples

```sh
kubectl images
kubectl images -A
kubectl images -A --latest
kubectl images -l app=web
```

## Sample output

```
$ kubectl images -A
IMAGE                          PODS  NAMESPACES                  NOTE
nginx:1.27                     14    default,web                 
registry.local/api:latest      6     default,staging             mutable tag
busybox                        2     jobs                        mutable tag
quay.io/org/app@sha256:ab12... 1     prod                        

$ kubectl images -A --latest
IMAGE                          PODS  NAMESPACES                  NOTE
registry.local/api:latest      6     default,staging             mutable tag
busybox                        2     jobs                        mutable tag
```

More than three namespaces are shortened to `a,b,c +N more`. Use `--latest` in CI or audits to list only images that should be pinned.
