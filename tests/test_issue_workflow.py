import importlib.util
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "issue_workflow", Path(__file__).parents[1] / "scripts/issue-workflow.py"
)
workflow = importlib.util.module_from_spec(spec)
spec.loader.exec_module(workflow)


class EligibilityTests(unittest.TestCase):
    def test_ready_for_agent_authorizes_issue(self):
        workflow.eligible(
            {"state": "open", "labels": [{"name": "ready-for-agent"}]}, [], [], "owner/repo", 15
        )

    def test_missing_readiness_label_prevents_work(self):
        with self.assertRaisesRegex(RuntimeError, "ready-for-test or ready-for-agent"):
            workflow.eligible({"state": "open", "labels": []}, [], [], "owner/repo", 15)

    def test_open_native_blocker_prevents_work(self):
        with self.assertRaisesRegex(RuntimeError, "blockers"):
            workflow.eligible(
                {"state": "open", "labels": [{"name": "ready-for-test"}]},
                [{"state": "open"}],
                [],
                "owner/repo",
                15,
            )

    def test_open_draft_pr_prevents_second_contribution(self):
        with self.assertRaisesRegex(RuntimeError, "open PR"):
            workflow.eligible(
                {"state": "open", "labels": [{"name": "ready-for-test"}]},
                [],
                [
                    {
                        "number": 40,
                        "state": "open",
                        "draft": True,
                        "body": "Part of #15",
                        "html_url": "https://github.com/owner/repo/pull/40",
                    }
                ],
                "owner/repo",
                15,
            )

    def test_merged_partial_pr_does_not_block_new_run(self):
        workflow.eligible(
            {"state": "open", "labels": [{"name": "ready-for-test"}]},
            [{"state": "closed"}],
            [{"number": 40, "state": "closed", "body": "Part of #15"}],
            "owner/repo",
            15,
        )

    def test_issue_closed_after_final_pr_prevents_new_run(self):
        with self.assertRaisesRegex(RuntimeError, "open issue"):
            workflow.eligible(
                {"state": "closed", "labels": [{"name": "ready-for-test"}]},
                [],
                [],
                "owner/repo",
                15,
            )

    def test_other_issue_number_or_repository_is_not_our_pr(self):
        self.assertFalse(
            workflow.references({"body": "Part of #150; also other/repo#15"}, "owner/repo", 15)
        )
        self.assertTrue(workflow.references({"body": "Fixes owner/repo#15"}, "owner/repo", 15))
        self.assertFalse(
            workflow.references(
                {"body": "https://github.com/owner/repo/issues/150"}, "owner/repo", 15
            )
        )

    def test_wayfinder_planning_is_not_implementation(self):
        with self.assertRaisesRegex(RuntimeError, "wayfinder"):
            workflow.eligible(
                {
                    "state": "open",
                    "labels": [{"name": "ready-for-test"}, {"name": "wayfinder:map"}],
                },
                [],
                [],
                "owner/repo",
                15,
            )


class VerificationTests(unittest.TestCase):
    def test_dirty_first_attempt_has_diagnostics_and_can_dispatch_repair(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            job = root / ("a" * 32)
            job.mkdir()
            (job / "config.json").write_text("{}")
            (job / "verified.json").write_text('{"head":"old"}')
            with patch.object(workflow, "clean", side_effect=RuntimeError("Workspace is dirty")):
                with self.assertRaisesRegex(RuntimeError, "Workspace is dirty"):
                    workflow.verify(job, {"verification": ["exit 0"]})
            self.assertEqual(json.loads((job / "attempt.json").read_text()), {"count": 1})
            self.assertIn("Workspace is dirty", (job / "verify.log").read_text())
            self.assertFalse((job / "verified.json").exists())
            with patch.object(workflow, "STORAGE", root):
                workflow.stage("retry", job.name)

    def test_credentials_are_removed_from_environment_and_redacted_in_both_outputs(self):
        from contextlib import redirect_stdout
        from io import StringIO

        for exit_code in (0, 7):
            with self.subTest(exit_code=exit_code), tempfile.TemporaryDirectory() as directory:
                job = Path(directory)
                auth = job / "auth.json"
                auth.write_text('{"tokens":{"access_token":"synthetic-codex-secret"}}')
                check = (
                    'test -z "${FABRO_DEV_TOKEN+x}" && test -z "${SESSION_SECRET+x}" '
                    '&& test -z "${OPENROUTER_API_KEY+x}" && test -z "${BASH_ENV+x}" '
                    '&& test -z "${GIT_CONFIG_VALUE_0+x}" || exit 99; '
                    'printf "synthetic-fabro-secret\\nsynthetic-session-secret\\n'
                    'synthetic-provider-secret\\nsynthetic-codex-secret\\n"; '
                    f"exit {exit_code}"
                )
                output = StringIO()
                with (
                    patch.dict(
                        os.environ,
                        {
                            "FABRO_DEV_TOKEN": "synthetic-fabro-secret",
                            "SESSION_SECRET": "synthetic-session-secret",
                            "OPENROUTER_API_KEY": "synthetic-provider-secret",
                            "BASH_ENV": "/no/such/startup-file",
                            "GIT_CONFIG_VALUE_0": "credential helper",
                            "CODEX_HOME": str(job),
                        },
                    ),
                    patch.object(workflow, "clean"),
                    patch.object(workflow, "command", return_value="sha"),
                    redirect_stdout(output),
                ):
                    if exit_code:
                        with self.assertRaisesRegex(RuntimeError, "exit 7"):
                            workflow.verify(job, {"verification": [check]})
                    else:
                        workflow.verify(job, {"verification": [check]})
                log = (job / "verify.log").read_text()
                for secret in (
                    "synthetic-fabro-secret",
                    "synthetic-session-secret",
                    "synthetic-provider-secret",
                    "synthetic-codex-secret",
                ):
                    self.assertNotIn(secret, log)
                    self.assertNotIn(secret, output.getvalue())
                self.assertIn("[REDACTED]", log)
                if exit_code:
                    self.assertIn("[REDACTED]", output.getvalue())

    def test_untracked_cache_is_allowed_but_new_code_is_not(self):
        with tempfile.TemporaryDirectory() as directory:
            repo = Path(directory)
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            (repo / ".cache").mkdir()
            (repo / ".cache/build-artifact").write_text("generated")
            previous = Path.cwd()
            try:
                os.chdir(repo)
                workflow.clean()
                (repo / "new_code.py").write_text('print("not committed")')
                with self.assertRaisesRegex(RuntimeError, "uncommitted/untracked"):
                    workflow.clean()
            finally:
                os.chdir(previous)

    def test_failed_check_never_leaves_success_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            job = Path(directory)
            (job / "verified.json").write_text("{}")
            with (
                patch.object(workflow, "clean"),
                patch.object(workflow, "command", return_value="sha"),
            ):
                with self.assertRaisesRegex(RuntimeError, "exit 7"):
                    workflow.verify(job, {"verification": ["exit 7"]})
            self.assertFalse((job / "verified.json").exists())
            self.assertEqual(json.loads((job / "attempt.json").read_text())["count"], 1)

    def test_three_failures_do_not_dispatch_another_repair(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            job = root / ("a" * 32)
            job.mkdir()
            (job / "config.json").write_text("{}")
            (job / "attempt.json").write_text('{"count":3}')
            with patch.object(workflow, "STORAGE", root):
                with self.assertRaisesRegex(RuntimeError, "Three verification"):
                    workflow.stage("retry", job.name)

    def test_successful_commands_record_actual_git_tree(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repo, job = root / "repo", root / "job"
            repo.mkdir()
            job.mkdir()
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            (repo / "calculator.py").write_text("assert 1 + 1 == 2\n")
            subprocess.run(["git", "-C", str(repo), "add", "."], check=True)
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(repo),
                    "-c",
                    "user.name=Test",
                    "-c",
                    "user.email=test@example.com",
                    "-c",
                    "commit.gpgsign=false",
                    "commit",
                    "-qm",
                    "fixture",
                ],
                check=True,
            )
            previous = Path.cwd()
            try:
                os.chdir(repo)
                workflow.verify(job, {"verification": ["python3 -B calculator.py"]})
                self.assertEqual(
                    json.loads((job / "verified.json").read_text())["tree"],
                    workflow.command("git", "rev-parse", "HEAD^{tree}"),
                )
            finally:
                os.chdir(previous)


class ContextTests(unittest.TestCase):
    def test_context_preserves_markdown_and_github_history(self):
        fixtures = Path(__file__).parent / "fixtures"
        context = json.loads((fixtures / "context.json").read_text())
        self.assertEqual(workflow.context_markdown(context), (fixtures / "context.md").read_text())

    def test_context_is_printed_and_captured_without_dirtying_git(self):
        from contextlib import redirect_stdout
        from io import StringIO

        fixtures = Path(__file__).parent / "fixtures"
        context = json.loads((fixtures / "context.json").read_text())
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            job = root / ("a" * 32)
            job.mkdir()
            (job / "config.json").write_text('{"repository":"owner/repo","issue":8}')
            repo = root / "repo"
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            previous = Path.cwd()
            output = StringIO()
            try:
                os.chdir(repo)
                with (
                    patch.object(workflow, "STORAGE", root),
                    patch.object(workflow, "credentials"),
                    patch.object(workflow, "snapshot", return_value=context),
                    redirect_stdout(output),
                ):
                    workflow.stage("context", job.name)
                self.assertEqual(output.getvalue(), (job / "context.md").read_text())
                self.assertEqual(output.getvalue(), (repo / ".fabro-context.md").read_text())
                workflow.clean()
                subprocess.run(["git", "add", "."], check=True)
                self.assertEqual(workflow.command("git", "ls-files"), "")
            finally:
                os.chdir(previous)


if __name__ == "__main__":
    unittest.main()
