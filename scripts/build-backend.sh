#!/usr/bin/env bash
# Build the MR Reviewer backend server as a standalone binary using PyInstaller.
# Must be run on the target OS (macOS binary must be built on macOS, etc.)
#
# Usage:
#   bash scripts/build-backend.sh
#
# Output: frontend/resources/backend/mr-reviewer-server

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="$REPO_ROOT/frontend/resources/backend"

cd "$REPO_ROOT"

echo "==> Installing PyInstaller..."
pip install pyinstaller

echo "==> Building backend binary..."
pyinstaller mr-reviewer-server.spec \
    --distpath "$OUTPUT_DIR" \
    --workpath /tmp/mr-reviewer-pyinstaller \
    --noconfirm

BINARY="$OUTPUT_DIR/mr-reviewer-server"
echo "==> Binary built: $BINARY"

echo "==> Verifying binary..."
"$BINARY" --help
echo "==> Build successful!"
