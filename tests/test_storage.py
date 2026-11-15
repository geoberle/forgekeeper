import datetime as dt
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


def load(name, file):
    spec = importlib.util.spec_from_file_location(
        name, Path(__file__).parents[1] / "scripts" / file
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


cleanup = load("cleanup", "cleanup-workspaces.py")
cache = load("cache", "shared_cache.py")


class StorageTests(unittest.TestCase):
    def test_retention(self):
        now = dt.datetime(2026, 11, 15, tzinfo=dt.timezone.utc)
        for status, age, expected in [
            ("succeeded", 23, False),
            ("succeeded", 24, True),
            ("failed", 47, False),
            ("failed", 48, True),
            ("dead", 48, True),
            ("running", 100, False),
            ("runnable", 100, False),
        ]:
            with self.subTest(status=status, age=age):
                run = {
                    "lifecycle": {"status": {"kind": status}},
                    "timestamps": {"completed_at": (now - dt.timedelta(hours=age)).isoformat()},
                }
                self.assertEqual(cleanup.expired(run, now), expected)

    def test_cache_partition_and_reuse(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            first = cache.cache_environment("owner/repo", root)
            self.assertEqual(first, cache.cache_environment("OWNER/repo", root))
            self.assertNotEqual(first, cache.cache_environment("other/repo", root))
            for path in first.values():
                self.assertTrue(Path(path).is_dir())

    def test_symlink_cache_rejected_before_creation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            (root / "link").symlink_to(root / "outside")
            with self.assertRaises(ValueError):
                cache.cache_environment("owner/repo", root / "link")
            self.assertFalse((root / "outside").exists())

    def test_cleanup_preserves_history_and_retries_idempotently(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            job = root / "issue-jobs" / ("a" * 32)
            source = job / "source"
            source.mkdir(parents=True)
            run_id = "01M7FW6RXA3FMHMK4PATJV13NF"
            scratch = root / "scratch" / ("20261113-" + run_id)
            scope = scratch / "petri/scopes"
            scope.mkdir(parents=True)
            (scope / "large-cache").write_text("cache")
            scope.chmod(0o555)
            (scratch / "events.json").write_text("history")
            (job / "review.json").write_text("evidence")
            (job / "run.json").write_text(json.dumps({"run_id": run_id}))
            (job / "config.json").write_text(json.dumps({"repository": "owner/repo", "issue": 8}))
            run = {
                "id": run_id,
                "source_directory": str(source),
                "labels": {"github_issue": "owner/repo#8"},
                "lifecycle": {"status": {"kind": "succeeded"}, "archived": False},
                "timestamps": {"completed_at": "2026-11-13T00:00:00Z"},
            }
            calls = []

            def api(path, method="GET"):
                calls.append((path, method))
                if path.startswith("/runs?"):
                    return {"data": [run], "meta": {"has_more": False}}
                if path.endswith("/archive"):
                    run["lifecycle"]["archived"] = True
                return run

            now = dt.datetime(2026, 11, 15, tzinfo=dt.timezone.utc)
            cleanup.cleanup(root, api, lambda _: [{"number": 49, "body": "Part of #8"}], now)
            self.assertTrue(source.exists())
            self.assertFalse(run["lifecycle"]["archived"])
            cleanup.cleanup(root, api, lambda _: [], now)
            self.assertFalse(source.exists())
            self.assertFalse(scope.exists())
            self.assertTrue((scratch / "events.json").exists())
            self.assertTrue((job / "review.json").exists())
            self.assertTrue((job / "workspace-cleaned.json").exists())
            cleanup.cleanup(root, api, lambda _: [], now)
            self.assertEqual(sum(method == "POST" for _, method in calls), 1)

    def test_active_issue_and_failed_archive_prevent_removal(self):
        now = dt.datetime(2026, 11, 15, tzinfo=dt.timezone.utc)
        run = {
            "id": "01M7FW6RXA3FMHMK4PATJV13NF",
            "labels": {"github_issue": "owner/repo#8"},
            "lifecycle": {"status": {"kind": "succeeded"}},
            "timestamps": {"completed_at": "2026-11-13T00:00:00Z"},
        }
        active = {
            "labels": {"github_issue": "owner/repo#8"},
            "lifecycle": {"status": {"kind": "running"}},
        }
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            source = root / "source"
            source.mkdir()

            def api(path, method="GET"):
                if path.startswith("/runs?"):
                    return {"data": [run, active], "meta": {"has_more": False}}
                raise RuntimeError("archive unavailable")

            with patch.object(cleanup, "workspace_paths", return_value=(root, [source])) as paths:
                cleanup.cleanup(root, api, lambda _: [], now)
                paths.assert_not_called()
                active["lifecycle"]["status"]["kind"] = "succeeded"
                with self.assertRaisesRegex(RuntimeError, "archive unavailable"):
                    cleanup.cleanup(root, api, lambda _: [], now)
                self.assertTrue(source.exists())

    def test_pr_protection(self):
        run = {"labels": {"github_issue": "owner/repo#8", "github_pr": "49"}}
        self.assertTrue(cleanup.protected(run, [{"number": 49}]))
        self.assertTrue(cleanup.protected(run, [{"number": 50, "body": "Fixes #8"}]))
        self.assertFalse(cleanup.protected(run, [{"number": 50, "body": "Fixes #80"}]))
