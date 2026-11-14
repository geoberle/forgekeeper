#!/usr/bin/env bash
set -euo pipefail
workflow="${1:-control}"
case "$workflow" in control | codex) ;; *)
    echo 'Expected control or codex' >&2
    exit 2
    ;;
esac
exec podman exec --user fabro -w /storage/workspaces/fixture fabro-experiment \
    fabro run "/experiment/workflows/$workflow.fabro" \
    --server http://127.0.0.1:32276 --environment local --detach
