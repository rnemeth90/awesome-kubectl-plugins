#!/bin/sh
# Installs kubectl plugins from the latest GitHub release.
#   curl -fsSL https://kubectl-plugins.dev/install.sh | sh -s -- count ping
#   curl -fsSL https://kubectl-plugins.dev/install.sh | sh -s -- --all
# Environment: INSTALL_DIR (default ~/.local/bin), VERSION (default latest).
set -eu

repo="rnemeth90/awesome-kubectl-plugins"
all="aks aksmaintenance aksversions count images nodepool orphans pending ping restarts settags unhealthy whoami"
dir="${INSTALL_DIR:-$HOME/.local/bin}"

alias_for() {
  case "$1" in
    nodepool) echo np ;;
    aksversions) echo aks_versions ;;
    aksmaintenance) echo aks_maintenance ;;
    settags) echo set_tags ;;
  esac
}

[ $# -gt 0 ] || { echo "usage: install.sh PLUGIN... | --all" >&2; echo "plugins: $all" >&2; exit 2; }
[ "$1" = "--all" ] && set -- $all

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "unsupported OS $(uname -s); download a binary from https://github.com/$repo/releases" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

if [ -n "${RELEASE_BASE_URL:-}" ]; then base="$RELEASE_BASE_URL"
elif [ -n "${VERSION:-}" ]; then base="https://github.com/$repo/releases/download/$VERSION"
else base="https://github.com/$repo/releases/latest/download"; fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"
mkdir -p "$dir"

for name in "$@"; do
  case " $all " in *" $name "*) ;; *) echo "unknown plugin: $name (available: $all)" >&2; exit 1 ;; esac
  file="kubectl-${name}_${os}_${arch}"
  curl -fsSL "$base/$file" -o "$tmp/$file"
  want=$(awk -v f="$file" '$2 == f { print $1 }' "$tmp/checksums.txt")
  if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$tmp/$file" | awk '{print $1}')
  else got=$(shasum -a 256 "$tmp/$file" | awk '{print $1}'); fi
  [ -n "$want" ] && [ "$want" = "$got" ] || { echo "checksum mismatch for $file" >&2; exit 1; }
  install -m 0755 "$tmp/$file" "$dir/kubectl-$name"
  a=$(alias_for "$name")
  [ -z "$a" ] || ln -sf "kubectl-$name" "$dir/kubectl-$a"
  echo "installed $dir/kubectl-$name"
done

case ":$PATH:" in *":$dir:"*) ;; *) echo "note: add $dir to your PATH" >&2 ;; esac
