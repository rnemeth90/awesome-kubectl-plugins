---
title: "Install"
description: "Build and install the plugins with make."
---

## Requirements

- Go 1.26 or newer
- `kubectl`
- For the AKS plugins: an Azure login (`az login`) or any credential supported by the Azure SDK's default credential chain

## Quick install (macOS and Linux)

Each plugin's docs page has an install box with a copy-able command. Or run it yourself, naming the plugins you want:

```sh
curl -fsSL https://kubectl-plugins.dev/install.sh | sh -s -- count ping
curl -fsSL https://kubectl-plugins.dev/install.sh | sh -s -- --all
```

The script downloads prebuilt binaries from the [latest release](https://github.com/rnemeth90/awesome-kubectl-plugins/releases/latest), verifies their SHA-256 checksums and installs them into `~/.local/bin` (override with `INSTALL_DIR`, or pin a release with `VERSION=v0.1.0`). Windows users can download `kubectl-NAME_windows_amd64.exe` from the release page.

Releases are built by pushing a tag: `git tag v0.1.0 && git push origin v0.1.0`.

## Build and install from source

```sh
git clone https://github.com/rnemeth90/kubectl-plugins
cd kubectl-plugins
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
