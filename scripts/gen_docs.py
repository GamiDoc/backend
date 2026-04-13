from __future__ import annotations

import re
import shutil
import subprocess
from collections import defaultdict
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
DOCS_REF_DIR = ROOT / "docs" / "reference"


def run(cmd: list[str], cwd: Path | None = None) -> str:
    result = subprocess.run(
        cmd,
        cwd=cwd or ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def read_module_path() -> str:
    go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
    match = re.search(r"^\s*module\s+(\S+)\s*$", go_mod, re.MULTILINE)
    if not match:
        raise RuntimeError("module path not found in go.mod")
    return match.group(1)


def list_packages() -> list[str]:
    output = run(["go", "list", "./..."])
    packages = [line.strip() for line in output.splitlines() if line.strip()]
    packages.sort()
    return packages


def package_rel_path(module_path: str, package_path: str) -> str:
    if package_path == module_path:
        return "root"
    prefix = module_path + "/"
    if package_path.startswith(prefix):
        return package_path[len(prefix):]
    return package_path.replace("/", "_")


def target_md_path(rel_path: str) -> Path:
    return DOCS_REF_DIR / f"{rel_path}.md"


def ensure_parent(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)


def render_package_md(package_path: str, rel_path: str, doc_text: str) -> str:
    title = rel_path
    lines: list[str] = []
    lines.append(f"# {title}")
    lines.append("")
    lines.append(f"**Package:** `{package_path}`")
    lines.append("")
    lines.append("## Go Reference")
    lines.append("")
    lines.append("```text")
    lines.append(doc_text.rstrip())
    lines.append("```")
    lines.append("")
    return "\n".join(lines)


def build_index(entries: list[tuple[str, str]]) -> str:
    groups: dict[str, list[tuple[str, str]]] = defaultdict(list)

    for rel_path, link in entries:
        top = rel_path.split("/", 1)[0]
        groups[top].append((rel_path, link))

    lines: list[str] = []
    lines.append("# Reference")
    lines.append("")
    for group_name in sorted(groups):
        lines.append(f"## {group_name}")
        lines.append("")
        for rel_path, link in sorted(groups[group_name], key=lambda item: item[0]):
            lines.append(f"- [{rel_path}]({link})")
        lines.append("")
    return "\n".join(lines)


def main() -> None:
    module_path = read_module_path()

    if DOCS_REF_DIR.exists():
        shutil.rmtree(DOCS_REF_DIR)
    DOCS_REF_DIR.mkdir(parents=True, exist_ok=True)

    entries: list[tuple[str, str]] = []

    for package_path in list_packages():
        rel_path = package_rel_path(module_path, package_path)
        doc_text = run(["go", "doc", "-all", package_path])
        target = target_md_path(rel_path)
        ensure_parent(target)
        target.write_text(
            render_package_md(package_path, rel_path, doc_text),
            encoding="utf-8",
        )
        entries.append((rel_path, target.relative_to(DOCS_REF_DIR).as_posix()))

    (DOCS_REF_DIR / "index.md").write_text(build_index(entries), encoding="utf-8")


if __name__ == "__main__":
    main()
