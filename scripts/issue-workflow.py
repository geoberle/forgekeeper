#!/usr/bin/env python3
"""Deterministic command stages for the issue workflow."""

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

STORAGE = Path("/storage/issue-jobs")


def command(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def write_json(path, value):
    temporary = path.with_suffix(".tmp")
    temporary.write_text(json.dumps(value, indent=2) + "\n")
    temporary.replace(path)


def read_json(path):
    return json.loads(path.read_text())


def credentials():
    token = Path("/storage/home/.github-pat")
    if not token.is_file() or not token.read_text().strip():
        raise RuntimeError(
            "The Python helper runs inside fabro-experiment, where the bot PAT is missing. "
            "On the host, provide .local/secrets/github-pat "
            "then recreate the container with "
            "scripts/start.sh. See README.md."
        )
    os.environ["GH_TOKEN"] = token.read_text().strip()
    os.environ.update(
        GIT_CONFIG_COUNT="1",
        GIT_CONFIG_KEY_0="credential.helper",
        GIT_CONFIG_VALUE_0="!gh auth git-credential",
    )


def github(path):
    return json.loads(command("gh", "api", path))


def pages(path):
    items = []
    for page in range(1, 1001):
        batch = github(f"{path}{'&' if '?' in path else '?'}per_page=100&page={page}")
        if not isinstance(batch, list):
            raise RuntimeError("Expected a paginated GitHub list")
        items.extend(batch)
        if len(batch) < 100:
            return items
    raise RuntimeError("GitHub pagination limit exceeded; refusing incomplete eligibility check")


def references(pr, repo, issue):
    text = (pr.get("title") or "") + "\n" + (pr.get("body") or "")
    return (
        re.search(rf"https://github.com/{re.escape(repo)}/issues/{issue}(?![0-9])", text)
        is not None
        or re.search(rf"(?<![\w/]){re.escape(repo)}#{issue}\b", text) is not None
        or re.search(rf"(?<![\w/])#{issue}\b", text) is not None
    )


def eligible(ticket, blockers, prs, repo, issue, own_pr=None):
    if ticket.get("state") != "open" or "pull_request" in ticket:
        raise RuntimeError("Expected an open issue")
    labels = {label["name"] for label in ticket["labels"]}
    if not labels.intersection({"ready-for-test", "ready-for-agent"}) or any(
        x.startswith("wayfinder:") for x in labels
    ):
        raise RuntimeError(
            "Issue needs ready-for-test or ready-for-agent and must not have a wayfinder label"
        )
    if any(blocker.get("state") != "closed" for blocker in blockers):
        raise RuntimeError("Issue has open or unknown blockers")
    for pr in prs:
        if pr.get("state") == "open" and pr["number"] != own_pr and references(pr, repo, issue):
            raise RuntimeError(f"Issue already has an open PR: {pr['html_url']}")


def snapshot(repo, issue, own_pr=None):
    ticket = github(f"repos/{repo}/issues/{issue}")
    blockers = pages(f"repos/{repo}/issues/{issue}/dependencies/blocked_by")
    prs = pages(f"repos/{repo}/pulls?state=all")
    eligible(ticket, blockers, prs, repo, issue, own_pr)
    return dict(
        issue=ticket,
        blockers=blockers,
        comments=pages(f"repos/{repo}/issues/{issue}/comments"),
        pull_requests=[p for p in prs if references(p, repo, issue)],
    )


def context_markdown(context):
    issue = context["issue"]
    lines = [
        f"# Issue #{issue['number']}: {issue['title']}",
        "",
        f"URL: {issue['html_url']}",
        f"State: {issue['state']}",
        "Labels: " + (", ".join(label["name"] for label in issue["labels"]) or "None"),
        "",
        "## Description",
        "",
        issue.get("body") or "(No description)",
        "",
        "## Blockers",
        "",
    ]
    for blocker in context["blockers"]:
        lines.append(f"- [{blocker['title']}]({blocker['html_url']}) — {blocker['state']}")
    if not context["blockers"]:
        lines.append("None.")
    lines.extend(["", "## Comments", ""])
    for comment in context["comments"]:
        lines.extend(
            [
                f"### {comment['user']['login']} — {comment['created_at']}",
                "",
                comment["html_url"],
                "",
                comment.get("body") or "(Empty comment)",
                "",
            ]
        )
    if not context["comments"]:
        lines.append("None.")
    lines.extend(["", "## Related pull requests", ""])
    for pr in context["pull_requests"]:
        state = "merged" if pr.get("merged_at") else pr["state"]
        if pr.get("draft"):
            state += " (draft)"
        lines.extend(
            [
                f"### PR #{pr['number']}: {pr['title']}",
                "",
                pr["html_url"],
                f"State: {state}",
                "",
                pr.get("body") or "(No description)",
                "",
            ]
        )
    if not context["pull_requests"]:
        lines.append("None.")
    return "\n".join(lines).rstrip() + "\n"


def clean():
    # Verification intentionally writes Go/linter caches here. Only untracked
    # cache files are exempt; tracked changes and other new files still fail.
    if command("git", "status", "--porcelain", "--untracked-files=no") or command(
        "git", "ls-files", "--others", "--exclude-standard", "--exclude=.cache/"
    ):
        raise RuntimeError(
            "Workspace has uncommitted/untracked changes; commit or remove generated files first"
        )


def verification_environment():
    allowed = {"PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "TZ"}
    return {key: value for key, value in os.environ.items() if key in allowed}


def verification_secrets():
    values = {
        value
        for key, value in os.environ.items()
        if re.search(r"TOKEN|SECRET|PASSWORD|CREDENTIAL|API_KEY", key, re.I) and value
    }
    pat = Path("/storage/home/.github-pat")
    if pat.is_file():
        values.add(pat.read_text().strip())
    auth = Path(os.environ.get("CODEX_HOME", "/storage/home/.codex")) / "auth.json"
    if auth.is_file():

        def collect(value):
            if isinstance(value, dict):
                for key, item in value.items():
                    if isinstance(item, str) and re.search(r"token|key|secret|password", key, re.I):
                        values.add(item)
                    elif isinstance(item, dict):
                        collect(item)

        collect(read_json(auth))
    return sorted((value for value in values if value), key=len, reverse=True)


def verify(job, config):
    attempt_file = job / "attempt.json"
    attempt = read_json(attempt_file)["count"] + 1 if attempt_file.exists() else 1
    write_json(attempt_file, {"count": attempt})
    (job / "verified.json").unlink(missing_ok=True)
    with (job / "verify.log").open("w") as log:
        secrets = verification_secrets()

        def safe(text):
            for secret in secrets:
                text = text.replace(secret, "[REDACTED]")
            return text

        try:
            clean()
            head = command("git", "rev-parse", "HEAD")
            tree = command("git", "rev-parse", "HEAD^{tree}")
            for check in config["verification"]:
                log.write(safe(f"$ {check}\n"))
                log.flush()
                result = subprocess.run(
                    ["bash", "-o", "pipefail", "-c", check],
                    env=verification_environment(),
                    stdout=subprocess.PIPE,
                    stderr=subprocess.STDOUT,
                    text=True,
                    errors="replace",
                )
                log.write(safe(result.stdout))
                log.flush()
                if result.returncode:
                    raise RuntimeError(
                        f"Verification failed with exit {result.returncode} (attempt {attempt}/3)"
                    )
            clean()
            if head != command("git", "rev-parse", "HEAD") or tree != command(
                "git", "rev-parse", "HEAD^{tree}"
            ):
                raise RuntimeError("Verification changed committed code")
        except (RuntimeError, OSError, subprocess.CalledProcessError) as error:
            message = (
                safe(str(error))
                if isinstance(error, RuntimeError)
                else "Verification command failed"
            )
            log.write(message + "\n")
            log.flush()
            print((job / "verify.log").read_text()[-16000:])
            raise RuntimeError(message) from None
    write_json(job / "verified.json", {"head": head, "tree": tree})
    print(f"Verification passed for {head}")


def stage(action, job_id):
    if not re.fullmatch(r"[a-f0-9]{32}", job_id):
        raise RuntimeError("Invalid job input; use run-issue.sh")
    job = STORAGE / job_id
    config = read_json(job / "config.json")
    if action == "context":
        credentials()
    if action == "context":
        clean()
        markdown = context_markdown(snapshot(config["repository"], config["issue"]))
        (job / "context.md").write_text(markdown)
        artifact = Path(".fabro-context.md")
        if artifact.exists():
            raise RuntimeError("Context artifact path already exists")
        exclude = Path(command("git", "rev-parse", "--git-path", "info/exclude"))
        with exclude.open("a") as stream:
            stream.write("\n/.fabro-context.md\n")
        artifact.write_text(markdown)
        print(markdown, end="")
    elif action == "result":
        result = read_json(job / "result.json")
        if (
            result.get("status") != "ready"
            or not result.get("summary")
            or "remaining" not in result
        ):
            raise RuntimeError(
                "Implementation needs human attention: "
                + str(result.get("summary", "missing result"))
            )
        if not command("git", "diff", "--name-only", config["base_sha"], "HEAD"):
            raise RuntimeError("No contribution to publish; needs human attention")
    elif action == "verify":
        verify(job, config)
    elif action == "retry":
        if read_json(job / "attempt.json")["count"] >= 3:
            raise RuntimeError("Three verification attempts failed; needs human attention")
    else:
        raise RuntimeError("Unknown workflow command")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    for action in ("context", "result", "verify", "retry"):
        sub.add_parser(action).add_argument("job")
    args = parser.parse_args()
    try:
        stage(args.action, args.job)
    except (RuntimeError, OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"Stopped: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
