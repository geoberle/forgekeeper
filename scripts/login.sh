#!/usr/bin/env bash
set -euo pipefail
exec podman exec -it --user fabro fabro-experiment codex login --device-auth
