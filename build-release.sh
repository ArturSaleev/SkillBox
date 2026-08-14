#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${DIST_DIR:-$(cd "$ROOT_DIR/.." && pwd)/release}"
RELEASE_NAME="SkillBox"
MODE="${1:-host}"

case "$MODE" in
  host)
    targets="$(go env GOOS)/$(go env GOARCH)"
    ;;
  all)
    targets="darwin/arm64 darwin/amd64 linux/arm64 linux/amd64"
    ;;
  *)
    echo "usage: ./build-release.sh [host|all]" >&2
    exit 2
    ;;
esac

for target in $targets; do
  os="${target%/*}"
  arch="${target#*/}"
  bundle="$DIST_DIR/$os/$arch/$RELEASE_NAME"
  mkdir -p "$bundle/configs" "$bundle/docs"
  binary="$RELEASE_NAME"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o "$bundle/$binary" ./cmd/skillbox
  cp configs/skillbox.example.yaml "$bundle/configs/skillbox.yaml"
  cp README.md "$bundle/README.md"
  cp docs/*.md "$bundle/docs/"
  echo "built $bundle"
done
