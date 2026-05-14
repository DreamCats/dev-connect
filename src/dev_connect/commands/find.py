"""按名称搜索文件."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def find_files(
    name: str,
    path: str,
    host_alias: str | None,
    file_type: str | None,
    json_output: bool,
) -> None:
    """按名称搜索文件."""
    cmd = _build_cmd(name, path, file_type)
    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        _output_json(result.stdout, name, path)
    else:
        click.echo(result.stdout, nl=False)


def _build_cmd(name: str, path: str, file_type: str | None) -> str:
    """构建 find 命令."""
    parts = ["find", path, "-name", f"'{name}'"]
    if file_type:
        parts.extend(["-type", file_type])
    return " ".join(parts)


def _output_json(output: str, name: str, path: str) -> None:
    """将搜索结果转换为 JSON 格式."""
    files = []
    lines = output.strip().split("\n") if output.strip() else []

    for line in lines:
        if not line:
            continue

        files.append({
            "path": line,
            "name": line.split("/")[-1],
        })

    result = {
        "name": name,
        "path": path,
        "files": files,
        "count": len(files),
    }

    click.echo(json.dumps(result, indent=2, ensure_ascii=False))
