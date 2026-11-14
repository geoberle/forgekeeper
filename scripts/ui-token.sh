#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
python3 - "$root/.local/server.env" <<'PY'
import pathlib, subprocess, sys
values = dict(line.split('=', 1) for line in pathlib.Path(sys.argv[1]).read_text().splitlines())
subprocess.run(['pbcopy'], input=values['FABRO_DEV_TOKEN'].encode(), check=True)
print('Fabro dev-token copied to clipboard. Paste it into the local UI login.')
PY
