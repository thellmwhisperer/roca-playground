#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
make prepare "CORE_DIR=${CORE_DIR:-.worktrees/core}"
mkdir -p dist
for platform in darwin-arm64 linux-x64 linux-arm64 windows-x64; do
  system=${platform%-*}
  arch=${platform#*-}
  [ "$arch" != x64 ] || arch=amd64
  package="dist/package-$platform"
  mkdir -p "$package"
  binary=roca-playground
  [ "$system" != windows ] || binary=roca-playground.exe
  CGO_ENABLED=0 GOOS="$system" GOARCH="$arch" go build -modfile=.tmp/build.mod -trimpath -ldflags='-s -w' -o "$package/$binary" ./cmd/roca-playground
  cp plugin.json "$package/"
  (cd "$package" && shasum -a 256 plugin.json "$binary" > checksums.txt)
  tar -czf "dist/roca-playground-v0.1.0-$platform.tar.gz" -C "$package" plugin.json "$binary" checksums.txt
done
(cd dist && shasum -a 256 roca-playground-v0.1.0-*.tar.gz > checksums.txt)
