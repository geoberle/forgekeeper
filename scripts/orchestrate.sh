#!/usr/bin/env bash
set -euo pipefail
exec podman exec --user fabro fabro-experiment orchestrator "$@"
