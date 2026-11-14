#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
umask 077
mkdir -p "$root/.local/data"
python3 - "$root/.local/server.env" <<'PY'
import pathlib, secrets, sys
path = pathlib.Path(sys.argv[1])
if not path.exists():
    path.write_text('SESSION_SECRET=' + secrets.token_hex(32) + '\n'
                    + 'FABRO_DEV_TOKEN=fabro_dev_' + secrets.token_hex(32) + '\n')
    path.chmod(0o600)
PY
if podman container exists fabro-experiment; then
    podman start fabro-experiment >/dev/null
else
    mounts=()
    pat_file="${FABRO_GITHUB_PAT_FILE:-$root/.local/secrets/github-pat}"
    if [[ -n "${FABRO_GITHUB_PAT_FILE:-}" && ! -s "$pat_file" ]]; then
        echo "Bot PAT file is missing or empty: $pat_file" >&2
        exit 1
    fi
    if [[ -f "$pat_file" ]]; then
        pat_file="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$pat_file")"
        mounts+=(-v "$pat_file:/run/secrets/github-pat:ro")
    fi
    podman run -d --name fabro-experiment \
        --security-opt=no-new-privileges \
        -p 127.0.0.1:32277:32276 \
        --env-file "$root/.local/server.env" \
        -v "$root/.local/data:/storage" \
        -v "$root/settings.toml:/experiment/settings.toml:ro" \
        -v "$root/workflows:/experiment/workflows:ro" \
        -v "$root/fixtures:/experiment/fixtures:ro" \
        -v "$root/scripts:/experiment/scripts:ro" \
        -v "$root/projects:/experiment/projects:ro" \
        "${mounts[@]}" \
        localhost/fabro-experiment:local \
        fabro server start --foreground --config /experiment/settings.toml
fi
python3 - <<'PY'
import time, urllib.request, urllib.error
for attempt in range(30):
    try:
        urllib.request.urlopen('http://127.0.0.1:32277/', timeout=1)
        break
    except urllib.error.HTTPError:
        break
    except (urllib.error.URLError, TimeoutError):
        time.sleep(1)
else:
    raise SystemExit('Fabro did not start. Inspect: podman logs fabro-experiment')
PY
"$root/scripts/prepare.sh"
echo 'Fabro UI: http://localhost:32277'
echo 'No workflows are started automatically.'
