"""显示目录树."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def show_tree(
    path: str,
    host_alias: str | None,
    depth: int,
    json_output: bool,
) -> None:
    """显示目录树."""
    # 使用 find 命令获取目录树
    cmd = f"find {path} -maxdepth {depth} -type f -o -type d | head -100"
    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        _output_json(result.stdout, path, depth)
    else:
        _output_tree(result.stdout, path)


def _output_tree(find_output: str, base_path: str) -> None:
    """将 find 输出转换为树形格式."""
    lines = find_output.strip().split("\n")
    base_parts = base_path.rstrip("/").split("/")

    for line in lines:
        if not line.strip():
            continue

        # 计算相对路径
        parts = line.split("/")
        relative = "/".join(parts[len(base_parts) :])

        if not relative:
            # 根目录
            click.echo(f"{base_path}/")
        else:
            # 计算缩进
            depth = relative.count("/")
            indent = "  " * depth
            name = relative.split("/")[-1]

            # 判断是否是目录
            is_dir = not name or line.endswith("/")
            suffix = "/" if is_dir else ""

            click.echo(f"{indent}{name}{suffix}")


def _output_json(find_output: str, base_path: str, depth: int) -> None:
    """将 find 输出转换为 JSON 格式."""
    lines = find_output.strip().split("\n")
    items = []

    for line in lines:
        if not line.strip():
            continue

        # 判断类型
        is_dir = line.endswith("/")
        name = line.rstrip("/").split("/")[-1]

        items.append(
            {
                "path": line,
                "name": name,
                "type": "directory" if is_dir else "file",
            }
        )

    output = {
        "path": base_path,
        "depth": depth,
        "items": items,
        "count": len(items),
    }

    click.echo(json.dumps(output, indent=2, ensure_ascii=False))
