#!/usr/bin/env python3
"""Bundle a Skillshare checkout or local Codex resources, never host auth/config."""

import argparse
import json
import re
import shutil
import tomllib
from pathlib import Path

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument(
    "--skills-source", type=Path, help="Skillshare checkout (library/ and agents/); no host caches"
)
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
context = root / ".local/build"
context.mkdir(parents=True, exist_ok=True)
shutil.copy2(root / "container/Dockerfile", context / "Dockerfile")
for name in ("go.mod", "go.sum"):
    shutil.copy2(root / name, context / name)
for name in ("cmd", "internal", "scripts", "workflows", "fixtures"):
    target = context / name
    if target.exists():
        shutil.rmtree(target)
    shutil.copytree(root / name, target, ignore=shutil.ignore_patterns("__pycache__"))

bundle = context / "bundle"
if bundle.exists():
    shutil.rmtree(bundle)
(bundle / "skills").mkdir(parents=True)
home = Path.home()
manifest = []
ignore = shutil.ignore_patterns(
    ".git",
    "__pycache__",
    ".DS_Store",
    ".env",
    ".env.*",
    "auth.json",
    "credentials.json",
    "*.pem",
    "node_modules",
    ".mcp.json",
    ".app.json",
)


def expose(source, target, name, prefix=""):
    destination = bundle / "skills" / (prefix + name)
    if destination.exists():
        raise SystemExit(f"Duplicate exposed skill: {destination.name}")
    # Plugin names are namespaced to avoid shadowing personal/system skills.
    text = (target / "SKILL.md").read_text()
    if prefix:
        text, count = re.subn(r"^name:.*$", "name: " + prefix + name, text, count=1, flags=re.M)
        if count != 1:
            raise SystemExit(f"Missing skill name: {source}")
        (target / "SKILL.md").write_text(text)
    if target != destination:
        destination.symlink_to(Path("..") / target.relative_to(bundle), target_is_directory=True)
    manifest.append(
        {
            "name": destination.name,
            "source": str(source),
            "bundled": str(target.relative_to(bundle)),
        }
    )


if args.skills_source:
    source = args.skills_source.resolve()
    for required in ("implement", "pr", "tdd", "code-review"):
        if not (source / "library" / required / "SKILL.md").is_file():
            raise SystemExit(f"Required skill missing: {required}")
    # Reject escaping links before copying; a checkout must be self-contained.
    for directory in (source / "library", source / "agents"):
        for path in directory.rglob("*"):
            if path.is_symlink() and not path.resolve().is_relative_to(directory):
                raise SystemExit(
                    "Skillshare resources must not link outside their source directory"
                )
    shutil.copytree(source / "library", bundle / "skills", dirs_exist_ok=True, ignore=ignore)
    shutil.copytree(source / "agents", bundle / "agents", ignore=ignore)
    for definition in sorted((bundle / "skills").rglob("SKILL.md")):
        relative = definition.parent.relative_to(bundle)
        manifest.append(
            {
                "name": definition.parent.name,
                "source": "library/" + str(definition.parent.relative_to(bundle / "skills")),
                "bundled": str(relative),
            }
        )
else:
    for directory, namespace in [(home / ".agents/skills", ""), (home / ".codex/skills", "codex")]:
        for definition in sorted(directory.rglob("SKILL.md")):
            source = definition.parent
            name = source.name
            prefix = namespace + "--" if (bundle / "skills" / name).exists() else ""
            target = bundle / "skills" / (prefix + name)
            shutil.copytree(source, target, symlinks=False, ignore=ignore)
            # target already exists because this is a real directory, not a plugin link.
            manifest.append(
                {
                    "name": target.name,
                    "source": str(source),
                    "bundled": str(target.relative_to(bundle)),
                }
            )

    config_path = home / ".codex/config.toml"
    config = tomllib.loads(config_path.read_text()) if config_path.exists() else {}
    for marketplace in sorted((home / ".codex/plugins/cache").iterdir()):
        for plugin in sorted(marketplace.iterdir()):
            if (
                not plugin.is_dir()
                or config.get("plugins", {})
                .get(f"{plugin.name}@{marketplace.name}", {})
                .get("enabled")
                is False
            ):
                continue
            versions = [p for p in plugin.iterdir() if p.is_dir() and list(p.rglob("SKILL.md"))]
            if not versions:
                continue
            # Codex currently has one cached skill-bearing version per plugin here.
            # Refuse ambiguity rather than silently bundle an obsolete version.
            if len(versions) != 1:
                raise SystemExit(
                    f"Multiple cached versions for {plugin}; select the active version before building"
                )
            source = versions[0]
            target = bundle / "plugins" / marketplace.name / plugin.name / source.name
            shutil.copytree(source, target, symlinks=False, ignore=ignore)
            definitions = list((source / "skills").glob("*/SKILL.md")) + list(
                (source / ".codex-plugin/migrated-command-skills").glob("*/SKILL.md")
            )
            for definition in sorted(definitions):
                skill = definition.parent
                prefix = plugin.name + "--"
                if (bundle / "skills" / (prefix + skill.name)).exists():
                    prefix = marketplace.name + "--" + prefix
                expose(skill, target / skill.relative_to(source), skill.name, prefix)

(bundle / "skills-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
print(f"Bundled {len(manifest)} skills; inventory: {bundle}/skills-manifest.json")
