#!/usr/bin/env bash
set -euo pipefail
podman exec --user root fabro-experiment sh -eu -c '
    mkdir -p /storage/home/.agents /storage/home/.codex /storage/workspaces
    ln -sfn /opt/experiment/bundle/skills /storage/home/.agents/skills
    if [ -d /opt/experiment/bundle/agents/codex ]; then
        ln -sfn /opt/experiment/bundle/agents/codex /storage/home/.codex/agents
    fi
    if [ -f /run/secrets/github-pat ]; then
        cp /run/secrets/github-pat /storage/home/.github-pat
        chmod 600 /storage/home/.github-pat
    else
        rm -f /storage/home/.github-pat
    fi
    chown -R fabro:fabro /storage/home /storage/workspaces
'
podman exec --user fabro fabro-experiment sh -eu -c '
    fabro auth login --server http://127.0.0.1:32276 --dev-token "$FABRO_DEV_TOKEN"
    git config --global user.name "Fabro Experiment"
    git config --global user.email "fabro-experiment@localhost"
    # Podman/macOS shared filesystem ownership differs between exec sessions.
    git config --global --replace-all safe.directory /storage/workspaces/fixture
    if [ ! -d /storage/workspaces/fixture/.git ]; then
        mkdir /storage/workspaces/fixture
        cp /experiment/fixtures/*.py /storage/workspaces/fixture/
        cd /storage/workspaces/fixture
        git init -b main
        git add .
        git commit -m "Add deliberately failing addition fixture"
    fi
'
