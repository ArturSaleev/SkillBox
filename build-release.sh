#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${DIST_DIR:-$ROOT_DIR/release}"
RELEASE_NAME="SkillBox"
MODE="${1:-host}"

cd "$ROOT_DIR"

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

"$ROOT_DIR/build-dashboard.sh"

for target in $targets; do
  os="${target%/*}"
  arch="${target#*/}"
  bundle="$DIST_DIR/$os/$arch/$RELEASE_NAME"
  mkdir -p "$bundle/configs" "$bundle/docs" "$bundle/benchmark"
  binary="$RELEASE_NAME"
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o "$bundle/$binary" ./cmd/skillbox
  GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o "$bundle/skillbox-bench" ./benchmark/cmd/skillbox-bench
  if [[ ! -f "$bundle/configs/skillbox.yaml" ]]; then
    cp configs/skillbox.example.yaml "$bundle/configs/skillbox.yaml"
  fi
  cp benchmark/config.example.yaml "$bundle/benchmark/config.example.yaml"
  cp benchmark/README.md "$bundle/benchmark/README.md"
  cp README.md "$bundle/README.md"
  cp docs/*.md "$bundle/docs/"
  echo "built $bundle"
done
