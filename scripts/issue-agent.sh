#!/usr/bin/env sh
set -eu
GH_TOKEN="$(cat /storage/home/.github-pat)"
export GH_TOKEN
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=credential.helper
export GIT_CONFIG_VALUE_0='!gh auth git-credential'
export INITIAL_AGENT_MODE=agent-full-access
export CODEX_HOME=/storage/home/.codex
exec /usr/local/bin/codex-acp
