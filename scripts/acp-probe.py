"""Standalone ACP integration probe, run only inside the experiment container."""

import json
import os
import pathlib
import queue
import subprocess
import threading
import time

workspace = "/storage/workspaces/acp-" + str(time.time_ns())
subprocess.run(["git", "clone", "/storage/workspaces/fixture", workspace], check=True)
evidence = pathlib.Path("/storage/evidence") / pathlib.Path(workspace).name
evidence.mkdir(parents=True)
p = subprocess.Popen(
    ["codex-acp"],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=open(evidence / "stderr.log", "w"),
    text=True,
    env={**os.environ, "INITIAL_AGENT_MODE": "agent-full-access"},
)
messages = queue.Queue()


def read_messages():
    for line in p.stdout:
        messages.put(line)
    messages.put(None)


threading.Thread(target=read_messages, daemon=True).start()


def send(obj):
    p.stdin.write(json.dumps({"jsonrpc": "2.0", **obj}) + "\n")
    p.stdin.flush()


def request(i, method, params):
    send({"id": i, "method": method, "params": params})
    deadline = time.monotonic() + 240
    while time.monotonic() < deadline:
        try:
            line = messages.get(timeout=1)
        except queue.Empty:
            if p.poll() is not None:
                raise RuntimeError("adapter exited " + str(p.returncode))
            continue
        if not line:
            raise RuntimeError("adapter EOF")
        obj = json.loads(line)
        with open(evidence / "events.jsonl", "a") as f:
            f.write(line)
        if obj.get("id") == i and ("result" in obj or "error" in obj):
            if "error" in obj:
                raise RuntimeError(str(obj["error"]))
            return obj["result"]
        if "method" in obj and "id" in obj:
            if obj["method"] == "session/request_permission":
                opts = obj["params"]["options"]
                option = next(x for x in opts if x["kind"] == "allow_once")
                send(
                    {
                        "id": obj["id"],
                        "result": {
                            "outcome": {"outcome": "selected", "optionId": option["optionId"]}
                        },
                    }
                )
            else:
                send(
                    {
                        "id": obj["id"],
                        "error": {"code": -32601, "message": "unsupported client request"},
                    }
                )
    raise TimeoutError(method)


try:
    result = request(
        1,
        "initialize",
        {
            "protocolVersion": 1,
            "clientCapabilities": {},
            "clientInfo": {"name": "fabro-experiment-probe", "version": "1"},
        },
    )
    print("ACP initialized:", result.get("agentInfo"), flush=True)
    session = request(2, "session/new", {"cwd": workspace, "mcpServers": []})
    print("Session created", flush=True)
    result = request(
        3,
        "session/prompt",
        {
            "sessionId": session["sessionId"],
            "prompt": [
                {
                    "type": "text",
                    "text": "Read /storage/home/.agents/skills/implement/SKILL.md and follow it to fix calculator.add. Existing unittest tests must remain unchanged. If no Skill tool exists, read the tdd and code-review SKILL.md files from that same skills directory. This disposable local repo has no remote. Do not access GitHub. Run python3 -B -m unittest -v and commit the fix.",
                }
            ],
        },
    )
    print("Prompt completed:", result.get("stopReason"), flush=True)
    subprocess.run(["python3", "-B", "-m", "unittest", "-v"], cwd=workspace, check=True)
    original = subprocess.check_output(
        ["git", "show", "HEAD:test_calculator.py"], cwd="/storage/workspaces/fixture"
    )
    assert (pathlib.Path(workspace) / "test_calculator.py").read_bytes() == original, (
        "Agent changed the test fixture"
    )
    print("Evidence:", evidence, flush=True)
finally:
    p.terminate()
    try:
        p.wait(timeout=5)
    except subprocess.TimeoutExpired:
        p.kill()
        p.wait()
