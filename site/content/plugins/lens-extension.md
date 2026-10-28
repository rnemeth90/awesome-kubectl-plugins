---
title: "Freelens / Lens extension"
description: "Run the plugins from a page inside Freelens."
category: Guide
weight: 3
---

The `lens/` directory contains an extension that adds a **kubectl plugins** section to the cluster sidebar in [Freelens](https://freelens.app) 2.x. Each page is a small form that runs the matching plugin against the open cluster and shows its output.

The extension shells out to the plugin binaries, so install them first (`make install`) and make sure they are on your `PATH`. The extension also looks in `~/.local/bin`, `~/go/bin`, `/usr/local/bin` and `/opt/homebrew/bin`.

## Install

```sh
cd lens
npm install
npm run build
npm pack
```

In Freelens, open **Extensions** and install the generated `.tgz` file.

## Included tools

| Section | Tools |
|---|---|
| Pods | unhealthy, pending, restarts, images |
| Cluster | count, orphans, whoami |
| Network | ping |
| AKS | aks show / describe / list, nodepool list / show / describe, aksversions, aksmaintenance list / show |
| AKS changes | aks upgrade / start / stop / rotate-certs, nodepool add / delete / scale / update / upgrade / upgrade-node-image / start / stop / taint / untaint / cancel-operation, aksmaintenance add / delete |
| Cluster | also `settags` (tick Dry run to preview) |

Tools that change things ask for confirmation first (**Run…** then **Confirm and run**). The plugins' own prompts are skipped with `--yes` because the extension has already asked.

## Notes

- Targets Freelens 2.x. The original Lens and OpenLens use an older extension API and are not supported.
- Every run passes the cluster's `--kubeconfig` and `--context`. AKS tools auto-detect the cluster as they do in the terminal; use the Advanced section to override it.
- Not yet tested inside a running Freelens; please report problems.
