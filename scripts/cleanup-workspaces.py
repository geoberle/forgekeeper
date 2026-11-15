#!/usr/bin/env python3
"""Archive expired terminal runs and remove only their local workspaces."""

import argparse
import datetime as dt
import json
import os
import re
import shutil
import sys
import urllib.error
import urllib.request
from pathlib import Path

RUN = re.compile(r"[0-9A-HJKMNP-TV-Z]{26}")
JOB = re.compile(r"[a-f0-9]{32}")
ISSUE = re.compile(r"([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([1-9][0-9]*)")


def request(base, token, path, method="GET"):
    req = urllib.request.Request(
        base.rstrip("/") + path,
        headers={"Authorization": "Bearer " + token, "Accept": "application/json"},
        method=method,
        data=b"{}" if method == "POST" else None,
    )
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            body = response.read()
            return json.loads(body) if body else None
    except (OSError, ValueError, urllib.error.HTTPError):
        raise RuntimeError("API request failed; no response body or credentials logged") from None


def list_runs(api):
    result = []
    for offset in range(0, 100000, 100):
        page = api(f"/runs?include_archived=true&page[limit]=100&page[offset]={offset}")
        if not isinstance(page.get("data"), list) or not isinstance(
            page.get("meta", {}).get("has_more"), bool
        ):
            raise RuntimeError("Incomplete Fabro run list")
        result.extend(page["data"])
        if not page["meta"]["has_more"]:
            return result
        if not page["data"]:
            break
    raise RuntimeError("Incomplete Fabro pagination")


def open_prs(github, repository):
    result = []
    for page in range(1, 1001):
        batch = github(f"/repos/{repository}/pulls?state=open&per_page=100&page={page}")
        if not isinstance(batch, list):
            raise RuntimeError("Invalid GitHub PR list")
        result.extend(batch)
        if len(batch) < 100:
            return result
    raise RuntimeError("Incomplete GitHub pagination")


def protected(run, prs):
    issue = run["labels"]["github_issue"]
    repository, number = issue.split("#")
    linked = (run.get("pull_request") or {}).get("number")
    labelled = run.get("labels", {}).get("github_pr")
    for pr in prs:
        if pr["number"] == linked or str(pr["number"]) == labelled:
            return True
        text = (pr.get("title") or "") + "\n" + (pr.get("body") or "")
        patterns = [
            rf"https://github.com/{re.escape(repository)}/issues/{number}(?![0-9])",
            rf"(?<![\w/]){re.escape(issue)}\b",
            rf"(?<![\w/])#{number}\b",
        ]
        if any(re.search(pattern, text) for pattern in patterns):
            return True
    return False


def expired(run, now):
    status = run.get("lifecycle", {}).get("status", {}).get("kind")
    hours = {"succeeded": 24, "failed": 48, "dead": 48}.get(status)
    completed = run.get("timestamps", {}).get("completed_at")
    if hours is None or not completed:
        return False
    timestamp = dt.datetime.fromisoformat(completed.replace("Z", "+00:00"))
    if timestamp.tzinfo is None:
        return False
    return now - timestamp >= dt.timedelta(hours=hours)


def checked_path(root, path):
    # Reject symlinked ancestors as well as paths outside the expected storage root.
    root = root.absolute()
    path = path.absolute()
    if root not in path.parents or path.resolve() != path:
        raise RuntimeError("Unsafe workspace path")
    return path


def workspace_paths(root, run):
    run_id = run["id"]
    if not RUN.fullmatch(run_id):
        raise RuntimeError("Invalid run identity")
    source = Path(run["source_directory"])
    job = source.parent
    if source.name != "source" or job.parent != root / "issue-jobs" or not JOB.fullmatch(job.name):
        raise RuntimeError("Invalid job directory")
    checked_path(root, source)
    identity = json.loads((job / "run.json").read_text())
    config = json.loads((job / "config.json").read_text())
    if (
        identity.get("run_id") != run_id
        or f"{config.get('repository')}#{config.get('issue')}" != run["labels"]["github_issue"]
    ):
        raise RuntimeError("Job identity mismatch")
    paths = [source]
    for scratch in (root / "scratch").glob("*-" + run_id):
        if not re.fullmatch(r"[0-9]{8}-" + run_id, scratch.name):
            raise RuntimeError("Invalid scratch directory")
        paths.append(checked_path(root, scratch / "petri" / "scopes"))
    if len(paths) > 2:
        raise RuntimeError("Ambiguous scratch directory")
    return job, paths


def cleanup(root, api, github, now):
    runs = list_runs(api)
    active_issues = {
        r.get("labels", {}).get("github_issue")
        for r in runs
        if r.get("lifecycle", {}).get("status", {}).get("kind")
        not in {"succeeded", "failed", "dead"}
    }
    for run in runs:
        issue = run.get("labels", {}).get("github_issue", "")
        if not ISSUE.fullmatch(issue) or issue in active_issues or not expired(run, now):
            continue
        repository = issue.split("#")[0]
        if protected(run, open_prs(github, repository)):
            continue
        job, paths = workspace_paths(root, run)
        if (job / "workspace-cleaned.json").exists():
            continue
        print(
            json.dumps(
                {
                    "run": run["id"],
                    "issue": issue,
                    "action": "clean",
                    "paths": [str(p) for p in paths],
                }
            ),
            flush=True,
        )
        # Archiving preserves durable records and disables normal resume/retry operations.
        api(f"/runs/{run['id']}/archive", "POST")
        fresh = api(f"/runs/{run['id']}")
        if not fresh["lifecycle"].get("archived") or not expired(fresh, now):
            raise RuntimeError("Run is not safely archived")
        if protected(fresh, open_prs(github, repository)):
            continue
        _, fresh_paths = workspace_paths(root, fresh)
        if paths != fresh_paths:
            raise RuntimeError("Workspace paths changed")
        for path in paths:
            checked_path(root, path)
            if path.exists():
                if not shutil.rmtree.avoids_symlink_attacks:
                    raise RuntimeError("Safe directory removal is unavailable")
                shutil.rmtree(path)
        marker = job / "workspace-cleaned.tmp"
        marker.write_text(
            json.dumps(
                {
                    "run_id": run["id"],
                    "cleaned_at": now.isoformat(),
                    "retry": "Start a new run with a fresh checkout; do not unarchive/resume this workspace.",
                }
            )
            + "\n"
        )
        marker.replace(job / "workspace-cleaned.json")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", required=True)
    parser.add_argument("--storage", type=Path, default=Path("/storage"))
    args = parser.parse_args()
    token = os.environ["FABRO_DEV_TOKEN"]
    pat = Path("/run/secrets/github-pat").read_text().strip()
    cleanup(
        args.storage,
        lambda path, method="GET": request(
            args.server.rstrip("/") + "/api/v1", token, path, method
        ),
        lambda path: request("https://api.github.com", pat, path),
        dt.datetime.now(dt.timezone.utc),
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print(
            "Workspace cleanup stopped; no credentials or raw exception details logged.",
            file=sys.stderr,
        )
        sys.exit(1)
