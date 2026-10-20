# kubectl-plugins
Some kubectl plugins I use to make life easier

| Plugin | Description |
|--------|-------------|
| `count` | Count any resource type, e.g. `kubectl count services -n kube-system` |
| `unhealthy` | List pods that are not Ready, crash-looping, pending or failed, with the reason |
| `restarts` | Rank pods by restart count with the last termination reason |
| `images` | List images in use and flag `:latest` or untagged ones |
| `nodepool` (`np`) | Manage AKS node pools |
| `aks` | Inspect and operate AKS clusters |
| `aks-versions` | List available AKS Kubernetes versions |
| `aks-maintenance` | Manage AKS maintenance windows |

## Usage

```sh
make install   # build and install plugins (plus aliases)
make check     # gofmt, vet, tests
make docs      # build the Hugo docs site into site/public
make docs-serve
```

Full documentation lives in [`site/`](site/) and is published to GitHub Pages
by `.github/workflows/docs.yml`.
