#!/usr/bin/env python3
"""Call the loopback experiment API without printing its credential."""

import json
import pathlib
import sys
import urllib.error
import urllib.request

root = pathlib.Path(__file__).resolve().parents[1]
values = dict(line.split("=", 1) for line in (root / ".local/server.env").read_text().splitlines())
method, path = sys.argv[1:3]
if not path.startswith("/"):
    raise SystemExit("Expected an API path starting with /")
body = sys.stdin.buffer.read() if method == "POST" else None
request = urllib.request.Request(
    "http://127.0.0.1:32277/api/v1" + path,
    data=body,
    method=method,
    headers={
        "Authorization": "Bearer " + values["FABRO_DEV_TOKEN"],
        "Content-Type": "application/json",
    },
)
try:
    with urllib.request.urlopen(request, timeout=15) as response:
        content = response.read()
        if content:
            print(json.dumps(json.loads(content), indent=2))
        else:
            print(response.status)
except urllib.error.HTTPError as error:
    raise SystemExit(f"HTTP {error.code}: {error.read().decode()}")
