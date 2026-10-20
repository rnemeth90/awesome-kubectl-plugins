---
title: "Install"
description: "Build and install the plugins with make."
---

## Requirements

- Go 1.26 or newer
- `kubectl`
- For the AKS plugins: an Azure login (`az login`) or any credential supported by the Azure SDK's default credential chain

## Build and install

```sh
git clone https://github.com/rnemeth90/awesome-kubectl-plugins
cd awesome-kubectl-plugins
make install          # builds into ./bin and installs to ~/.local/bin
```

Make sure the install directory is on your `PATH`. Override it with `INSTALL_DIR`:

```sh
make install INSTALL_DIR=/usr/local/bin
```

Install a single plugin with `make install PLUGIN=count`. See what is installed with `make list`, and remove plugins with `make uninstall`.

## How kubectl finds plugins

kubectl runs any executable named `kubectl-<name>` on your `PATH` when you type `kubectl <name>`. Dashes in a command become underscores in the binary name, so `kubectl aks-versions` runs `kubectl-aks_versions`.

`make install` creates these extra names as symlinks:

| Command | Binary |
|---|---|
| `kubectl np` | `kubectl-np` (alias of `kubectl-nodepool`) |
| `kubectl aks-versions` | `kubectl-aks_versions` |
| `kubectl aks-maintenance` | `kubectl-aks_maintenance` |

Check that kubectl sees them with `kubectl plugin list`.

## Targeting an AKS cluster

The four AKS plugins share the same targeting flags and rules. See [AKS targeting](../plugins/aks-targeting/).
