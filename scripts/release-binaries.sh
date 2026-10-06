#!/usr/bin/env bash
# release-binaries builds the gx command for the platforms of SDD section 3.5
# and writes one archive per platform plus checksums.txt into the output
# directory.
# usage: scripts/release-binaries.sh <tag> <out-dir>
set -euo pipefail
tag="${1:?usage: release-binaries.sh <tag> <out-dir>}"
out="${2:?usage: release-binaries.sh <tag> <out-dir>}"
# GX_SRC names the source tree to build. The default is this repository. The
# release workflow sets it to a checkout of the tag, so a tag that is older
# than this script still builds.
cd "${GX_SRC:-$(dirname "$0")/..}"

version="$(sed -n 's/^const Version = "\(.*\)"$/\1/p' gxcli/gxcli.go)"
if [ "v$version" != "$tag" ]; then
  echo "release-binaries: the tag is $tag and gxcli.Version is $version" >&2
  exit 1
fi

mkdir -p "$out"
out="$(cd "$out" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
for target in darwin/arm64 linux/amd64 linux/arm64 windows/amd64; do
  os="${target%/*}"
  arch="${target#*/}"
  name="gx_${version}_${os}_${arch}"
  dir="$work/$name"
  mkdir -p "$dir"
  bin="gx"
  if [ "$os" = "windows" ]; then
    bin="gx.exe"
  fi
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$dir/$bin" ./cmd/gx
  cp LICENSE README.md "$dir/"
  if [ "$os" = "windows" ]; then
    (cd "$work" && zip -q -r "$name.zip" "$name")
    mv "$work/$name.zip" "$out/"
  else
    tar -C "$work" -czf "$out/$name.tar.gz" "$name"
  fi
done
(cd "$out" && shasum -a 256 gx_"$version"_* > checksums.txt)
ls -l "$out"
