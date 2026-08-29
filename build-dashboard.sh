#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DASHBOARD_DIR="$ROOT_DIR/dashboard"
DASHBOARD_BUILD_DIR="$DASHBOARD_DIR/out"
DASHBOARD_EMBED_DIR="$ROOT_DIR/internal/dashboard/dist"

if ! command -v npm >/dev/null 2>&1; then
  echo "npm is required to build the SkillBox Dashboard" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "Node.js is required to build the SkillBox Dashboard" >&2
  exit 1
fi

if [[ ! -f "$DASHBOARD_DIR/package-lock.json" ]]; then
  echo "dashboard/package-lock.json is required for a reproducible build" >&2
  exit 1
fi

echo "building Dashboard"
(
  cd "$DASHBOARD_DIR"
  npm ci --no-audit --no-fund
  NEXT_PUBLIC_API_URL="" npm run build
)

if [[ ! -f "$DASHBOARD_BUILD_DIR/index.html" ]]; then
  echo "Dashboard static export is incomplete: index.html was not generated" >&2
  exit 1
fi

# Keep the tracked explanation file, replace only generated embedded assets.
find "$DASHBOARD_EMBED_DIR" -mindepth 1 -depth ! -name README.txt -delete
cp -R "$DASHBOARD_BUILD_DIR/." "$DASHBOARD_EMBED_DIR/"
