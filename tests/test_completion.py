import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).parents[1] / "scripts"))
spec = importlib.util.spec_from_file_location(
    "issue_workflow", Path(__file__).parents[1] / "scripts/issue-workflow.py"
)
workflow = importlib.util.module_from_spec(spec)
spec.loader.exec_module(workflow)


class CompletionTests(unittest.TestCase):
    def test_closure_requires_current_review_verification_and_eligibility(self):
        cases = [
            "approved",
            "already closed",
            "comment exists",
            "tree changed",
            "stale review",
            "missing verification",
            "evidence changed",
            "issue changed",
            "base changed",
            "not ready",
            "open pr",
            "blocked",
            "followup",
            "needs human",
            "write failed",
        ]
        for case in cases:
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                job = Path(directory)
                config = {"repository": "owner/repo", "issue": 67, "base_sha": "base"}
                ticket = {
                    "id": 67,
                    "title": "Done",
                    "body": "Criteria",
                    "state": "open",
                    "labels": [{"name": "ready-for-agent"}],
                }
                workflow.write_json(job / "issue-snapshot.json", ticket)
                workflow.write_json(job / "result.json", {"status": "already_satisfied"})
                workflow.write_json(job / "run.json", {"run_id": "run"})
                (job / "acceptance.md").write_text("Criterion: satisfied by existing test")
                (job / "review.md").write_text("Steve and David independently approve")
                approval = {
                    "tree": "base-tree",
                    "completion_digest": workflow.completion_digest(job),
                }
                workflow.write_json(job / "verified.json", {"tree": "base-tree"})
                if case == "stale review":
                    approval["tree"] = "old-tree"
                if case == "missing verification":
                    (job / "verified.json").unlink()
                if case == "evidence changed":
                    (job / "acceptance.md").write_text("Changed after review")
                if case == "issue changed":
                    ticket["body"] = "New criteria"
                if case == "already closed":
                    ticket["state"] = "closed"
                if case == "not ready":
                    ticket["labels"] = []
                if case == "followup":
                    config["pr"] = 49
                if case == "needs human":
                    workflow.write_json(job / "result.json", {"status": "needs_human"})
                workflow.write_json(job / "review-approved.json", approval)

                def github(path):
                    if path == "repos/owner/repo/issues/67":
                        return ticket
                    if path == "repos/owner/repo":
                        return {"default_branch": "main"}
                    if path == "repos/owner/repo/commits/main":
                        return {"sha": "new-base" if case == "base changed" else "base"}
                    if path == "user":
                        return {"id": 123}
                    self.fail(path)

                def pages(path):
                    if path.endswith("/blocked_by") and case == "blocked":
                        return [{"state": "open"}]
                    if path.endswith("pulls?state=open") and case == "open pr":
                        return [
                            {"number": 49, "state": "open", "body": "Part of #67", "html_url": "pr"}
                        ]
                    if path.endswith("/comments") and case == "comment exists":
                        return [
                            {
                                "user": {"id": 123},
                                "body": "<!-- forgekeeper:already-satisfied:run -->",
                            }
                        ]
                    return []

                with (
                    patch.object(workflow, "clean"),
                    patch.object(
                        workflow,
                        "command",
                        side_effect=[
                            "different" if case == "tree changed" else "base-tree",
                            "base-tree",
                        ],
                    ),
                    patch.object(workflow, "github", side_effect=github),
                    patch.object(workflow, "pages", side_effect=pages),
                    patch.object(workflow, "verification_secrets", return_value=[]),
                    patch.object(workflow, "github_write") as write,
                ):
                    if case == "write failed":
                        write.side_effect = RuntimeError("write failed")
                    if case in ("approved", "already closed", "comment exists"):
                        workflow.close_satisfied_issue(job, config)
                        if case == "already closed":
                            write.assert_not_called()
                        else:
                            self.assertEqual(
                                write.call_args_list[-1].args,
                                (
                                    "repos/owner/repo/issues/67",
                                    "PATCH",
                                    {"state": "closed", "state_reason": "completed"},
                                ),
                            )
                            self.assertEqual(write.call_count, 1 if case == "comment exists" else 2)
                    else:
                        with self.assertRaises((RuntimeError, FileNotFoundError)):
                            workflow.close_satisfied_issue(job, config)
                        if case != "write failed":
                            write.assert_not_called()
                        else:
                            self.assertEqual(write.call_count, 1)

    def test_both_reviewers_must_explicitly_confirm_completion(self):
        for verdict in (True, False, None):
            with self.subTest(verdict=verdict), tempfile.TemporaryDirectory() as directory:
                job = Path(directory)
                workflow.write_json(job / "result.json", {"status": "already_satisfied"})
                workflow.write_json(job / "issue-snapshot.json", {"body": "Criteria"})
                (job / "acceptance.md").write_text("Evidence")
                workflow.write_json(
                    job / "review-request.json",
                    {
                        "head": "head",
                        "tree": "tree",
                        "round": 1,
                        "completion_digest": workflow.completion_digest(job),
                    },
                )
                workflow.write_json(
                    job / "review.json",
                    {
                        "head": "head",
                        "reviewers": {
                            "steve": {
                                "status": "complete",
                                "summary": "Satisfied",
                                "findings": [],
                                "issue_complete": True,
                            },
                            "david": {
                                "status": "complete",
                                "summary": "Audit",
                                "findings": [],
                                "issue_complete": verdict,
                            },
                        },
                    },
                )
                with (
                    patch.object(workflow, "clean"),
                    patch.object(workflow, "command", side_effect=["head", "tree"]),
                    patch.object(workflow, "verification_secrets", return_value=[]),
                    patch.object(workflow.Path, "write_text", autospec=True) as write_text,
                    patch.object(workflow, "write_json") as write_json,
                ):
                    workflow.review_gate(job)
                    approvals = [
                        call
                        for call in write_json.call_args_list
                        if call.args[0].name == "review-approved.json"
                    ]
                    self.assertEqual(bool(approvals), verdict is True)
                    self.assertTrue(write_text.called)
                    if approvals:
                        self.assertEqual(
                            approvals[0].args[1]["completion_digest"],
                            workflow.completion_digest(job),
                        )

    def test_result_rejects_changes_or_missing_acceptance_evidence(self):
        for case in ("unchanged", "changed", "missing evidence", "followup"):
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                job = root / ("a" * 32)
                job.mkdir()
                config = {"base_sha": "base"}
                if case == "followup":
                    config["pr"] = 49
                workflow.write_json(job / "config.json", config)
                workflow.write_json(
                    job / "result.json",
                    {
                        "status": "already_satisfied",
                        "summary": "All criteria satisfied",
                        "remaining": "",
                    },
                )
                (job / "acceptance.md").write_text("" if case == "missing evidence" else "Evidence")
                with (
                    patch.object(workflow, "STORAGE", root),
                    patch.object(workflow, "clean"),
                    patch.object(
                        workflow, "command", return_value="code.go" if case == "changed" else ""
                    ),
                ):
                    if case == "unchanged":
                        workflow.stage("result", job.name)
                    else:
                        with self.assertRaises(RuntimeError):
                            workflow.stage("result", job.name)

    def test_route_only_selects_completion_for_already_satisfied(self):
        for status in ("ready", "needs_human", "already_satisfied"):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                job = root / ("a" * 32)
                job.mkdir()
                workflow.write_json(job / "config.json", {})
                workflow.write_json(job / "result.json", {"status": status})
                with patch.object(workflow, "STORAGE", root):
                    if status == "already_satisfied":
                        workflow.stage("completion-route", job.name)
                    else:
                        with self.assertRaises(RuntimeError):
                            workflow.stage("completion-route", job.name)
