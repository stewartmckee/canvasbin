#!/usr/bin/env bash
# Builds the OpenAI plugin ZIP from this folder, the shared skill and the web icons.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../.." && pwd)
out=${1:-$root/web/dist/canvasbin-openai-plugin.zip}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cp "$here/plugin.json" "$here/mcp.json" "$work/"
mkdir -p "$work/skills" "$work/assets"
cp -R "$here/skills/get-started" "$work/skills/"
cp -R "$root/skills/canvasbin" "$work/skills/"
cp "$root/web/public/icon-512.png" "$work/assets/logo.png"
cp "$root/web/public/icon-192.png" "$work/assets/icon.png"

mkdir -p "$(dirname "$out")"
rm -f "$out"
(cd "$work" && zip -qr "$out" .)
echo "$out"
