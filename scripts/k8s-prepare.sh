#!/usr/bin/env sh
set -eu
umask 077
mkdir -p "$HOME/.agents" "$CODEX_HOME" /storage/issue-jobs
ln -sfn /opt/experiment/bundle/skills "$HOME/.agents/skills"
if [ -d /opt/experiment/bundle/agents/codex ]; then
    ln -sfn /opt/experiment/bundle/agents/codex "$CODEX_HOME/agents"
fi
cp /run/secrets/github-pat "$HOME/.github-pat"
chmod 600 "$HOME/.github-pat"
git config --global user.name 'Fabro Bot'
git config --global user.email 'fabro-bot@localhost'
# This stores CLI credentials locally; it does not contact the server.
if ! fabro auth login --server "$1" --dev-token "$FABRO_DEV_TOKEN" >/dev/null 2>&1; then
    echo 'Cannot initialize Fabro CLI authentication' >&2
    exit 1
fi
