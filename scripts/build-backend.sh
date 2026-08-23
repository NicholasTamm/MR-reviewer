#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
output="$root/frontend/resources/backend"
mkdir -p "$output"
go build -o "$output/mr-reviewer-server" "$root/cmd/mr-reviewer"
"$output/mr-reviewer-server" help >/dev/null
