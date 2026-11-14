import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


class ImageContextTest(unittest.TestCase):
    def test_checkout_resources_without_host_state(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "scripts").mkdir()
            shutil.copy2("scripts/image-context.py", root / "scripts/image-context.py")
            for name in ("cmd", "internal", "workflows", "fixtures", "container"):
                (root / name).mkdir()
            for name in ("go.mod", "go.sum", "container/Dockerfile"):
                (root / name).write_text("fixture")
            source = root / "skillshare"
            for name in ("implement", "pr", "tdd", "code-review"):
                skill = source / "library" / name
                skill.mkdir(parents=True)
                (skill / "SKILL.md").write_text(f"# {name}\n")
            (source / "library/implement/resource.md").write_text("keep resource")
            (source / "library/implement/auth.json").write_text("synthetic-secret")
            (source / "config").mkdir()
            (source / "config/config.yaml").write_text("must not copy")
            (source / "agents/codex").mkdir(parents=True)
            (source / "agents/codex/reviewer.toml").write_text('name = "reviewer"')
            result = subprocess.run(
                [
                    sys.executable,
                    str(root / "scripts/image-context.py"),
                    "--skills-source",
                    str(source),
                ],
                env={**os.environ, "HOME": str(root / "nonexistent-home")},
                capture_output=True,
                text=True,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            bundle = root / ".local/build/bundle"
            self.assertEqual(len(json.loads((bundle / "skills-manifest.json").read_text())), 4)
            self.assertTrue((bundle / "skills/implement/resource.md").is_file())
            self.assertTrue((bundle / "agents/codex/reviewer.toml").is_file())
            self.assertFalse((bundle / "skills/implement/auth.json").exists())
            self.assertFalse((bundle / "config").exists())
            self.assertFalse((root / ".local/build/skillshare").exists())

            (source / "library/implement/escape").symlink_to(root)
            result = subprocess.run(
                [
                    sys.executable,
                    str(root / "scripts/image-context.py"),
                    "--skills-source",
                    str(source),
                ],
                capture_output=True,
                text=True,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("must not link outside", result.stderr)
