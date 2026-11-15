#!/usr/bin/env python3
"""Run a command with repository-scoped Go caches outside its workspace."""

import hashlib
import os
import re
import subprocess
import sys
from pathlib import Path


def cache_environment(repository, root=Path("/storage/caches/go")):
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository):
        raise ValueError("Invalid cache repository")
    key = hashlib.sha256(repository.lower().encode()).hexdigest()
    directory = root / key
    paths = {"GOMODCACHE": directory / "modules", "GOCACHE": directory / "build"}
    for path in paths.values():
        if path.resolve() != path.absolute():
            raise ValueError("Cache path contains a symlink")
        path.mkdir(parents=True, exist_ok=True)
    return {name: str(path) for name, path in paths.items()}


if __name__ == "__main__":
    repository = subprocess.check_output(
        ["git", "config", "--local", "--get", "forgekeeper.repository"], text=True
    ).strip()
    environment = {**os.environ, **cache_environment(repository)}
    os.execvpe(sys.argv[1], sys.argv[1:], environment)
