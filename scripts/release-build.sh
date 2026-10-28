#!/usr/bin/env bash
# Cross-compiles every plugin into dist/ as kubectl-NAME_OS_ARCH[.exe] plus checksums.txt.
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist
rm -rf "$out" && mkdir -p "$out"
version="${VERSION:-dev}"
for dir in plugins/*/; do
  name=$(basename "$dir")
  for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
    os=${target%/*}; arch=${target#*/}
    ext=""; [ "$os" = windows ] && ext=".exe"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w -X main.version=$version" \
      -o "$out/kubectl-${name}_${os}_${arch}${ext}" "./plugins/$name"
  done
done
(cd "$out" && if command -v sha256sum >/dev/null; then sha256sum kubectl-*; else shasum -a 256 kubectl-*; fi > checksums.txt)
