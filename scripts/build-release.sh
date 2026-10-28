#!/bin/sh
# Builds the release binaries, archives and checksums (CLI-11):
#   scripts/build-release.sh <version, e.g. 1.2.3> [dist-dir]
# dist/bin/<goos>_<goarch>/specfork[.exe]   raw binaries (input of npm/build.js)
# dist/specfork_<goos>_<goarch>.tar.gz|zip   release assets
# dist/checksums.txt                         SHA-256 of every asset
set -eu

version="${1:?usage: build-release.sh <version> [dist-dir]}"
version="${version#v}"
dist="${2:-dist}"
root="$(cd "$(dirname "$0")/.." && pwd)"

rm -rf "$dist"
mkdir -p "$dist/bin"
dist="$(cd "$dist" && pwd)"

cd "$root"
for goos in linux darwin windows; do
  for goarch in amd64 arm64; do
    exe=specfork
    [ "$goos" = windows ] && exe=specfork.exe
    out="$dist/bin/${goos}_${goarch}"
    mkdir -p "$out"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
      go build -trimpath -ldflags "-s -w -X main.buildVersion=$version" -o "$out/$exe" ./cmd/specfork
    if [ "$goos" = windows ]; then
      (cd "$out" && zip -q "$dist/specfork_${goos}_${goarch}.zip" "$exe")
    else
      tar -C "$out" -czf "$dist/specfork_${goos}_${goarch}.tar.gz" "$exe"
    fi
  done
done

cd "$dist"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum specfork_*.tar.gz specfork_*.zip > checksums.txt
else
  shasum -a 256 specfork_*.tar.gz specfork_*.zip > checksums.txt
fi
