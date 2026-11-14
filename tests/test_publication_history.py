import importlib.util
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "issue_workflow", Path(__file__).parents[1] / "scripts/issue-workflow.py"
)
workflow = importlib.util.module_from_spec(spec)
spec.loader.exec_module(workflow)


class PublicationHistoryTests(unittest.TestCase):
    def test_filtered_history_preserves_content_commits_and_run_workspace(self):
        for case in ("initial", "followup", "nonempty_checkpoint", "ordinary_empty", "lookalike"):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                repo = Path(directory) / "repo"
                job = Path(directory) / ("a" * 32)
                job.mkdir()
                subprocess.run(["git", "init", "-q", str(repo)], check=True)
                previous = Path.cwd()
                try:
                    os.chdir(repo)
                    workflow.command("git", "config", "user.name", "Implementation Author")
                    workflow.command("git", "config", "user.email", "author@example.com")
                    workflow.command("git", "config", "commit.gpgsign", "false")
                    Path("code.txt").write_text("base\n")
                    workflow.command("git", "add", "code.txt")
                    workflow.command("git", "commit", "-qm", "Base")
                    base = workflow.command("git", "rev-parse", "HEAD")
                    checkpoint = (
                        "fabro(01M7DHCEZ15H3HEKNBPNX9WH2F): context (success)\n\n"
                        "Fabro-Run: 01M7DHCEZ15H3HEKNBPNX9WH2F\n"
                        "Fabro-Execution: 0\nFabro-Firing: 1\nFabro-Attempt: 1\n"
                    )
                    workflow.command("git", "commit", "--allow-empty", "-qm", checkpoint)
                    expected_base = (
                        workflow.command("git", "rev-parse", "HEAD") if case == "followup" else base
                    )
                    Path("code.txt").write_text("first implementation\n")
                    workflow.command(
                        "git", "commit", "-qam", "Implement feature\n\nKeep this explanation."
                    )
                    implementation = workflow.command("git", "rev-parse", "HEAD")
                    if case == "nonempty_checkpoint":
                        Path("code.txt").write_text("uncommitted agent edits\n")
                        workflow.command("git", "add", "code.txt")
                    workflow.command("git", "commit", "--allow-empty", "-qm", checkpoint)
                    if case == "ordinary_empty":
                        workflow.command(
                            "git",
                            "commit",
                            "--allow-empty",
                            "-qm",
                            "Document implementation decision",
                        )
                    if case == "lookalike":
                        workflow.command(
                            "git", "commit", "--allow-empty", "-qm", checkpoint.split("\n")[0]
                        )
                    Path("code.txt").write_text("final implementation\n")
                    workflow.command("git", "commit", "-qam", "Polish implementation")
                    workflow.command("git", "commit", "--allow-empty", "-qm", checkpoint)
                    head = workflow.command("git", "rev-parse", "HEAD")
                    tree = workflow.command("git", "rev-parse", "HEAD^{tree}")
                    workflow.write_json(
                        job / "publication.json", {"head": head, "tree": tree, "mode": "draft"}
                    )
                    config = {"base_sha": base}
                    if case == "followup":
                        config.update(pr=49, expected_head=expected_base)
                    workflow.publication_history(job, config)
                    result = json.loads((job / "publication.json").read_text())
                    published = result["publish_head"]
                    self.assertEqual(
                        workflow.command("git", "rev-parse", f"{published}^{{tree}}"), tree
                    )
                    self.assertEqual(workflow.command("git", "rev-parse", "HEAD"), head)
                    self.assertEqual(Path("code.txt").read_text(), "final implementation\n")
                    self.assertEqual(workflow.command("git", "status", "--porcelain"), "")
                    commits = workflow.command(
                        "git", "rev-list", "--reverse", f"{expected_base}..{published}"
                    ).splitlines()
                    self.assertEqual(
                        len(commits),
                        3 if case in ("nonempty_checkpoint", "ordinary_empty", "lookalike") else 2,
                    )
                    self.assertEqual(
                        workflow.command("git", "show", "-s", "--format=%B", commits[0]),
                        "Implement feature\n\nKeep this explanation.",
                    )
                    self.assertEqual(
                        workflow.command("git", "show", "-s", "--format=%an <%ae> %aI", commits[0]),
                        workflow.command(
                            "git", "show", "-s", "--format=%an <%ae> %aI", implementation
                        ),
                    )
                    workflow.command("git", "merge-base", "--is-ancestor", expected_base, published)
                    # Re-running produces exactly the same publication history.
                    workflow.publication_history(job, config)
                    self.assertEqual(
                        json.loads((job / "publication.json").read_text())["publish_head"],
                        published,
                    )
                    if case == "nonempty_checkpoint":
                        self.assertEqual(
                            workflow.command("git", "show", f"{commits[1]}:code.txt"),
                            "uncommitted agent edits",
                        )
                finally:
                    os.chdir(previous)

    def test_merge_history_stops_without_publishing(self):
        from unittest.mock import patch

        with tempfile.TemporaryDirectory() as directory:
            job = Path(directory)
            workflow.write_json(job / "publication.json", {"tree": "reviewed"})
            with (
                patch.object(workflow, "clean"),
                patch.object(
                    workflow,
                    "command",
                    side_effect=["head", "reviewed", "", "merge", "parent1 parent2"],
                ),
            ):
                with self.assertRaisesRegex(RuntimeError, "merge history"):
                    workflow.publication_history(job, {"base_sha": "a" * 40})
            self.assertNotIn("publish_head", workflow.read_json(job / "publication.json"))

    def test_changed_tree_rejects_stale_publication_proof(self):
        from unittest.mock import patch

        with tempfile.TemporaryDirectory() as directory:
            job = Path(directory)
            workflow.write_json(
                job / "publication.json", {"tree": "reviewed", "publish_head": "stale"}
            )
            with (
                patch.object(workflow, "clean"),
                patch.object(workflow, "command", side_effect=["head", "changed"]),
            ):
                with self.assertRaisesRegex(RuntimeError, "tree changed"):
                    workflow.publication_history(job, {"base_sha": "a" * 40})
            self.assertNotIn("publish_head", workflow.read_json(job / "publication.json"))
