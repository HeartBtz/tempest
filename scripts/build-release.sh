#!/usr/bin/env bash
set -euo pipefail
umask 022

tag=${1:-}
[[ $tag =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || {
	printf '%s\n' 'usage: scripts/build-release.sh vMAJOR.MINOR.PATCH' >&2
	exit 2
}
version=${tag#v}
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
dist=$repo_root/dist
stage=$(mktemp -d)
trap 'rm -rf -- "$stage"' EXIT

rm -rf -- "$dist"
mkdir -p -- "$dist" "$stage/tempest-$version/web"

(
	cd "$repo_root/web"
	npm ci
	npm test
	npm run build
)
(
	cd "$repo_root"
	go test ./...
	CGO_ENABLED=1 go build \
		-ldflags "-s -w -X github.com/HeartBtz/tempest/internal/buildinfo.Version=$version" \
		-o "$stage/tempest-$version/tempest" ./cmd/tempest
)

actual=$("$stage/tempest-$version/tempest" --version)
[[ $actual == *"v$version" ]] || {
	printf 'built binary reported unexpected version: %s\n' "$actual" >&2
	exit 1
}
cp -a -- "$repo_root/web/dist/." "$stage/tempest-$version/web/"
cp -- "$repo_root/LICENSE" "$repo_root/README.md" "$stage/tempest-$version/"

tar --sort=name --owner=0 --group=0 --numeric-owner \
	--mtime="@${SOURCE_DATE_EPOCH:-0}" -C "$stage" -czf "$dist/tempest-$version.tar.gz" "tempest-$version"
(
	cd "$dist"
	sha256sum "tempest-$version.tar.gz" >"tempest-$version.tar.gz.sha256"
)
