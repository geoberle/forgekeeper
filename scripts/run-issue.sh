#!/usr/bin/env bash
set -euo pipefail
if [[ $# -lt 2 || $# -gt 3 || ($# -eq 3 && "$3" != '--check') ]]; then
    echo 'Usage: run-issue.sh OWNER/REPO ISSUE [--check]' >&2
    exit 2
fi
options=(--retry-user "${FORGEKEEPER_RETRY_USER:?Set FORGEKEEPER_RETRY_USER to your GitHub login}" --repo "$1" --issue "$2" --once)
if [[ $# -eq 3 ]]; then
    options+=(--preview)
fi
exec podman exec --user fabro fabro-experiment orchestrator "${options[@]}"
