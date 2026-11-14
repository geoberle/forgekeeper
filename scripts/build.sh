#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
umask 077
python3 "$root/scripts/image-context.py" "$@"
podman build -t localhost/fabro-experiment:local "$root/.local/build"
