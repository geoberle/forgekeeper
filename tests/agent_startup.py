"""Container smoke test: real launcher and ACP initialization, without a model request."""

import hashlib
import json
import os
import select
import subprocess
import tempfile
import time
from pathlib import Path

with tempfile.TemporaryDirectory() as directory:
    subprocess.run(["git", "init", "-q", directory], check=True)
    subprocess.run(
        ["git", "-C", directory, "config", "--local", "forgekeeper.repository", "smoke/repo"],
        check=True,
    )
    home = Path("/storage/home")
    home.mkdir(parents=True, exist_ok=True)
    (home / ".github-pat").write_text("smoke-test-not-a-credential")
    process = subprocess.Popen(
        ["/experiment/scripts/issue-agent.sh", "implement"],
        cwd=directory,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    try:
        message = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {"protocolVersion": 1, "clientCapabilities": {}},
        }
        process.stdin.write((json.dumps(message) + "\n").encode())
        process.stdin.flush()
        deadline = time.monotonic() + 30
        buffer = b""
        initialized = False
        while time.monotonic() < deadline and not initialized:
            if not select.select([process.stdout], [], [], max(0, deadline - time.monotonic()))[0]:
                break
            chunk = os.read(process.stdout.fileno(), 65536)
            if not chunk:
                break
            buffer += chunk
            while b"\n" in buffer:
                line, buffer = buffer.split(b"\n", 1)
                response = json.loads(line)
                if response.get("id") == 1:
                    assert "error" not in response, "ACP initialization failed"
                    assert response["result"]["protocolVersion"] == 1
                    initialized = True
        assert initialized, "Launcher did not complete ACP initialization"
        environment = Path(f"/proc/{process.pid}/environ").read_bytes().split(b"\0")
        key = hashlib.sha256(b"smoke/repo").hexdigest()
        for name, subdirectory in (("GOCACHE", "build"), ("GOMODCACHE", "modules")):
            expected = f"{name}=/storage/caches/go/{key}/{subdirectory}".encode()
            assert expected in environment, f"Launcher did not set {name}"
        print("Real agent launcher: ACP initialized and repository caches set")
    finally:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
