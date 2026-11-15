#!/usr/bin/env sh
set -eu
GH_TOKEN="$(cat /storage/home/.github-pat)"
export GH_TOKEN
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=credential.helper
export GIT_CONFIG_VALUE_0='!gh auth git-credential'
export INITIAL_AGENT_MODE=agent-full-access
export CODEX_HOME=/storage/home/.codex
case "${1:-implement}" in
implement | repair)
    default_config='{"model":"gpt-6.1-sol","model_reasoning_effort":"medium"}'
    CODEX_CONFIG="${CODEX_IMPLEMENT_CONFIG:-$default_config}"
    ;;
review)
    default_config='{"model":"gpt-6-astra","model_reasoning_effort":"low"}'
    CODEX_CONFIG="${CODEX_REVIEW_CONFIG:-$default_config}"
    ;;
pr)
    default_config='{"model":"gpt-6.1-sol","model_reasoning_effort":"low"}'
    CODEX_CONFIG="${CODEX_PR_CONFIG:-$default_config}"
    ;;
*)
    echo "Unknown issue-agent step: $1" >&2
    exit 2
    ;;
esac
export CODEX_CONFIG
exec python3 /experiment/scripts/shared_cache.py "${2:?repository required}" /usr/local/bin/codex-acp
