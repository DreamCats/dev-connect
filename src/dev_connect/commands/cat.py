"""查看文件内容."""

from __future__ import annotations

import json
import sys
from typing import Any

import click

from dev_connect.common.ssh import quote_remote_path, run_command


def show_file(
    paths: tuple[str, ...],
    host_alias: str | None,
    json_output: bool,
    cwd: str | None = None,
) -> None:
    """查看一个或多个文件内容."""
    result = run_command(_build_cat_cmd(paths, cwd), host_alias)

    try:
        payload: dict[str, Any] = json.loads(result.stdout)
    except json.JSONDecodeError:
        if result.stderr:
            click.echo(f"错误: {result.stderr}", err=True)
        else:
            click.echo("错误: 无法解析远程 cat 输出", err=True)
        sys.exit(result.returncode or 1)

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        _output_plain(payload)
        if result.stderr:
            click.echo(result.stderr, err=True, nl=False)

    if not result.success:
        sys.exit(result.returncode)


def _build_cat_cmd(paths: tuple[str, ...], cwd: str | None) -> str:
    """构建远端批量读取脚本."""
    script = f"""
import json
import os
from pathlib import Path
import sys

paths = {json.dumps(list(paths), ensure_ascii=False)!r}
items = []
has_error = False

for path in json.loads(paths):
    try:
        content = Path(path).read_text(errors="replace")
        items.append({{
            "path": path,
            "content": content,
            "size": len(content),
            "success": True,
            "error": "",
        }})
    except Exception as exc:
        has_error = True
        items.append({{
            "path": path,
            "content": "",
            "size": 0,
            "success": False,
            "error": str(exc),
        }})

print(json.dumps({{
    "cwd": os.getcwd(),
    "files": items,
    "count": len(items),
    "success": not has_error,
}}, ensure_ascii=False))
sys.exit(1 if has_error else 0)
"""
    steps = ["set -e"]
    if cwd:
        steps.append(f"cd {quote_remote_path(cwd)}")
    steps.extend(["python3 - <<'PY'", script.strip(), "PY"])
    return "\n".join(steps)


def _output_plain(payload: dict[str, Any]) -> None:
    """输出批量文件内容."""
    files = payload.get("files", [])
    if not isinstance(files, list):
        click.echo("错误: 远程 cat 输出缺少 files", err=True)
        return

    single = len(files) == 1
    for index, item in enumerate(files):
        if not isinstance(item, dict):
            continue
        path = str(item.get("path", ""))
        success = bool(item.get("success"))
        if not single:
            if index:
                click.echo()
            suffix = "" if success else " (error)"
            click.echo(f"===== {path}{suffix} =====")
        if success:
            click.echo(str(item.get("content", "")), nl=False)
        else:
            click.echo(f"错误: {path}: {item.get('error', '')}", err=True)
