#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output="$root/frontend/resources/backend"
mkdir -p "$output"
python -m pip install -e "$root[all]" "pyinstaller>=6.0,<7"
python -m PyInstaller "$root/mr-reviewer-server.spec" --noconfirm --distpath "$output" --workpath /tmp/mr-reviewer-pyinstaller
"$output/mr-reviewer-server" --help >/dev/null
