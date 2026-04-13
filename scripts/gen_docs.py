from __future__ import annotations

import json
import re
import shutil
import subprocess
from dataclasses import dataclass, field
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
DOCS_REF_DIR = ROOT / "docs" / "reference"


@dataclass
class PackageMeta:
    import_path: str
    rel_path: str
    dir_path: Path
    package_name: str
    doc: str
    files: list[str] = field(default_factory=list)


@dataclass
class Decl:
    kind: str
    name: str
    display_name: str
    signature: str
    doc: str
    file_rel: str
    line: int


def run(cmd: list[str], cwd: Path | None = None) -> str:
    result = subprocess.run(
        cmd,
        cwd=cwd or ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout


def read_module_path() -> str:
    go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
    match = re.search(r"^\s*module\s+(\S+)\s*$", go_mod, re.MULTILINE)
    if not match:
        raise RuntimeError("module path not found in go.mod")
    return match.group(1)


def current_source_ref() -> str:
    try:
        branch = run(["git", "rev-parse", "--abbrev-ref", "HEAD"]).strip()
        if branch and branch != "HEAD":
            return branch
    except Exception:
        pass
    return "main"


def normalize_repo_url(url: str) -> str:
    url = url.strip()
    if url.endswith(".git"):
        url = url[:-4]
    if url.startswith("git@github.com:"):
        return "https://github.com/" + url[len("git@github.com:") :]
    if url.startswith("https://github.com/"):
        return url
    if url.startswith("http://github.com/"):
        return "https://" + url[len("http://") :]
    return url


def repo_url() -> str:
    return normalize_repo_url(run(["git", "config", "--get", "remote.origin.url"]).strip())


def parse_go_list_stream(text: str) -> list[dict]:
    decoder = json.JSONDecoder()
    idx = 0
    length = len(text)
    items: list[dict] = []

    while idx < length:
        while idx < length and text[idx].isspace():
            idx += 1
        if idx >= length:
            break
        obj, next_idx = decoder.raw_decode(text, idx)
        items.append(obj)
        idx = next_idx

    return items


def list_packages(module_path: str) -> list[PackageMeta]:
    output = run(["go", "list", "-json", "./..."])
    raw_items = parse_go_list_stream(output)
    packages: list[PackageMeta] = []

    for item in raw_items:
        import_path = item["ImportPath"]
        dir_path = Path(item["Dir"])
        package_name = item.get("Name", "")
        doc = item.get("Doc", "")
        files = sorted(item.get("GoFiles", []))

        if import_path == module_path:
            rel_path = "root"
        elif import_path.startswith(module_path + "/"):
            rel_path = import_path[len(module_path) + 1 :]
        else:
            rel_path = import_path.replace("/", "_")

        packages.append(
            PackageMeta(
                import_path=import_path,
                rel_path=rel_path,
                dir_path=dir_path,
                package_name=package_name,
                doc=doc.strip(),
                files=files,
            )
        )

    packages.sort(key=lambda pkg: pkg.rel_path)
    return packages


FUNC_RE = re.compile(
    r"^\s*func\s*(\((?P<recv>[^)]*)\)\s*)?(?P<name>[A-Za-z_][A-Za-z0-9_]*)\s*\("
)
TYPE_RE = re.compile(r"^\s*type\s+(?P<name>[A-Z][A-Za-z0-9_]*)\b")
VAR_RE = re.compile(r"^\s*var\s+(?P<name>[A-Z][A-Za-z0-9_]*)\b")
CONST_RE = re.compile(r"^\s*const\s+(?P<name>[A-Z][A-Za-z0-9_]*)\b")


def clean_comment_block(lines: list[str]) -> str:
    cleaned = []
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("//"):
            cleaned.append(stripped[2:].lstrip())
    return "\n".join(cleaned).strip()


def receiver_type(raw: str) -> str:
    raw = raw.strip()
    if not raw:
        return ""
    parts = raw.split()
    if len(parts) < 2:
        return ""
    return parts[1].lstrip("*")


def extract_decls(file_path: Path) -> list[Decl]:
    lines = file_path.read_text(encoding="utf-8").splitlines()
    decls: list[Decl] = []
    pending_comments: list[str] = []

    for idx, line in enumerate(lines, start=1):
        stripped = line.strip()

        if stripped.startswith("//"):
            pending_comments.append(line)
            continue

        if stripped == "":
            pending_comments = []
            continue

        func_match = FUNC_RE.match(line)
        if func_match:
            name = func_match.group("name")
            if name[:1].isupper():
                recv = receiver_type(func_match.group("recv") or "")
                display_name = f"{recv}.{name}" if recv else name
                decls.append(
                    Decl(
                        kind="func",
                        name=name,
                        display_name=display_name,
                        signature=line.rstrip(),
                        doc=clean_comment_block(pending_comments),
                        file_rel=file_path.relative_to(ROOT).as_posix(),
                        line=idx,
                    )
                )
            pending_comments = []
            continue

        type_match = TYPE_RE.match(line)
        if type_match:
            name = type_match.group("name")
            decls.append(
                Decl(
                    kind="type",
                    name=name,
                    display_name=name,
                    signature=line.rstrip(),
                    doc=clean_comment_block(pending_comments),
                    file_rel=file_path.relative_to(ROOT).as_posix(),
                    line=idx,
                )
            )
            pending_comments = []
            continue

        var_match = VAR_RE.match(line)
        if var_match:
            name = var_match.group("name")
            decls.append(
                Decl(
                    kind="var",
                    name=name,
                    display_name=name,
                    signature=line.rstrip(),
                    doc=clean_comment_block(pending_comments),
                    file_rel=file_path.relative_to(ROOT).as_posix(),
                    line=idx,
                )
            )
            pending_comments = []
            continue

        const_match = CONST_RE.match(line)
        if const_match:
            name = const_match.group("name")
            decls.append(
                Decl(
                    kind="const",
                    name=name,
                    display_name=name,
                    signature=line.rstrip(),
                    doc=clean_comment_block(pending_comments),
                    file_rel=file_path.relative_to(ROOT).as_posix(),
                    line=idx,
                )
            )
            pending_comments = []
            continue

        pending_comments = []

    return decls


def package_decls(pkg: PackageMeta) -> list[Decl]:
    decls: list[Decl] = []
    for filename in pkg.files:
        decls.extend(extract_decls(pkg.dir_path / filename))
    decls.sort(key=lambda item: (item.kind, item.display_name.lower(), item.file_rel, item.line))
    return decls


def md_target(pkg: PackageMeta) -> Path:
    return DOCS_REF_DIR / f"{pkg.rel_path}.md"


def source_url(path: str, line: int | None = None) -> str:
    base = f"{repo_url()}/blob/{current_source_ref()}/{path}"
    if line is not None:
        return f"{base}#L{line}"
    return base


def icon_link(url: str, label: str) -> str:
    return f'[{label}]({url}){{ .md-button .md-button--small }}'


def render_package(pkg: PackageMeta) -> str:
    decls = package_decls(pkg)
    title = pkg.rel_path
    lines: list[str] = []

    lines.append(f"# {title}")
    lines.append("")
    lines.append(f"`{pkg.import_path}`")
    lines.append("")

    if pkg.doc:
        lines.append(pkg.doc)
        lines.append("")

    if pkg.files:
        for filename in pkg.files:
            rel = (pkg.dir_path / filename).relative_to(ROOT).as_posix()
            lines.append(f"- [{filename}]({source_url(rel)})")
        lines.append("")

    grouped: dict[str, list[Decl]] = {"type": [], "func": [], "var": [], "const": []}
    for decl in decls:
        grouped.setdefault(decl.kind, []).append(decl)

    for key, heading in [
        ("type", "Types"),
        ("func", "Functions"),
        ("var", "Variables"),
        ("const", "Constants"),
    ]:
        items = grouped.get(key, [])
        if not items:
            continue
        lines.append(f"## {heading}")
        lines.append("")
        for item in items:
            lines.append(f"### `{item.display_name}`")
            lines.append("")
            lines.append("```go")
            lines.append(item.signature)
            lines.append("```")
            lines.append("")
            if item.doc:
                lines.append(item.doc)
                lines.append("")

    if not decls and not pkg.doc and not pkg.files:
        lines.append("This package currently has no exported reference content.")
        lines.append("")

    return "\n".join(lines)


def build_reference_index(packages: list[PackageMeta]) -> str:
    lines: list[str] = []
    lines.append("# Reference")
    lines.append("")
    lines.append("| Package | Page |")
    lines.append("| --- | --- |")
    for pkg in packages:
        rel_link = f"{pkg.rel_path}.md"
        lines.append(f"| `{pkg.import_path}` | [{pkg.rel_path}]({rel_link}) |")
    lines.append("")
    return "\n".join(lines)


def new_nav_node() -> dict:
    return {
        "page": None,
        "children": {},
    }


def tree_insert(tree: dict, rel_path: str) -> None:
    parts = rel_path.split("/")
    node = tree
    for part in parts:
        children = node["children"]
        if part not in children:
            children[part] = new_nav_node()
        node = children[part]
    node["page"] = f"{rel_path}.md"


def render_nav_node(name: str, node: dict, indent: int) -> list[str]:
    pad = "  " * indent
    children = node["children"]
    page = node["page"]

    if not children:
        return [f"{pad}- {name}: {page}"]

    lines: list[str] = [f"{pad}- {name}:"]
    if page is not None:
        lines.append(f"{pad}  - Overview: {page}")

    for child_name in sorted(children):
        lines.extend(render_nav_node(child_name, children[child_name], indent + 1))

    return lines


def build_reference_nav(packages: list[PackageMeta]) -> str:
    tree = new_nav_node()
    for pkg in packages:
        if pkg.rel_path == "root":
            continue
        tree_insert(tree, pkg.rel_path)

    lines: list[str] = []
    lines.append("nav:")
    lines.append("  - Overview: index.md")

    root_pkg = next((pkg for pkg in packages if pkg.rel_path == "root"), None)
    if root_pkg is not None:
        lines.append(f"  - root: {root_pkg.rel_path}.md")

    for top_name in sorted(tree["children"]):
        lines.extend(render_nav_node(top_name, tree["children"][top_name], 1))

    lines.append("")
    return "\n".join(lines)


def main() -> None:
    module_path = read_module_path()
    packages = list_packages(module_path)

    if DOCS_REF_DIR.exists():
        shutil.rmtree(DOCS_REF_DIR)
    DOCS_REF_DIR.mkdir(parents=True, exist_ok=True)

    for pkg in packages:
        target = md_target(pkg)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(render_package(pkg), encoding="utf-8")

    (DOCS_REF_DIR / "index.md").write_text(
        build_reference_index(packages),
        encoding="utf-8",
    )

    (DOCS_REF_DIR / ".nav.yml").write_text(
        build_reference_nav(packages),
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
