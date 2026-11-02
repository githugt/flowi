#!/usr/bin/env bash
# Builds the release archives of the flowi command line into cli/dist/: one per system, and checksums.txt.
#
#   scripts/dist.sh 0.4.1-alpha.1        (the version without the v)
#
# This is what the release workflow (.github/workflows/release-cli.yml) runs, so a release can be tried here first.
# The binary has no cgo, so every target is a plain cross-compile. The version is given to the linker because Go
# does not stamp one into a module that lives in a subfolder of the repository.
set -euo pipefail

version="${1:?usage: scripts/dist.sh <version, without the v>}"
cd "$(dirname "$0")/.."

targets=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64)
stamp="-X github.com/githugt/flowi/cli/internal/buildinfo.stamped=v${version}"

rm -rf dist
mkdir dist

for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  name="flowi_${version}_${os}_${arch}"
  binary=flowi
  [ "$os" = windows ] && binary=flowi.exe

  stage="dist/${name}"
  mkdir "$stage"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w ${stamp}" -o "${stage}/${binary}" ./cmd/flowi
  cp LICENSE README.md CHANGELOG.md "$stage/"

  # The files sit at the top of the archive, not inside a folder.
  if [ "$os" = windows ]; then
    (cd "$stage" && zip -qr "../${name}.zip" .)
  else
    tar -C "$stage" -czf "dist/${name}.tar.gz" .
  fi
  rm -rf "$stage"
done

cd dist
if command -v sha256sum >/dev/null; then sha256sum flowi_* > checksums.txt; else shasum -a 256 flowi_* > checksums.txt; fi
ls -1
