#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="$(tr -d '\r\n' < "$ROOT_DIR/VERSION")"
ELECTRON_VERSION="${VERSION#v}"

if [[ -z "$VERSION" ]]; then
  echo "VERSION must not be empty" >&2
  exit 1
fi

echo "Step 1: Building frontend..."
cd "$ROOT_DIR/web"
bun install --frozen-lockfile
cd "$ROOT_DIR/web/default"
DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION="$VERSION" bun run build
cd "$ROOT_DIR/web/classic"
VITE_REACT_APP_VERSION="$VERSION" bun run build

case "$(go env GOOS)" in
  darwin)
    binary_name="new-api"
    electron_target="build:mac"
    ;;
  linux)
    binary_name="new-api"
    electron_target="build:linux"
    ;;
  windows)
    binary_name="new-api.exe"
    electron_target="build:win"
    ;;
  *)
    echo "Unsupported operating system: $(go env GOOS)" >&2
    exit 1
    ;;
esac

echo "Step 2: Building Go backend..."
cd "$ROOT_DIR"
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$VERSION'" \
  -o "$binary_name"

echo "Step 3: Testing and packaging Electron app..."
cd "$SCRIPT_DIR"
npm ci
npm test
npm run "$electron_target" -- --config.extraMetadata.version="$ELECTRON_VERSION"

echo "Build complete. Artifacts are in electron/dist/."