"""列目录内容."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def list_dir(path: str, host_alias: str | None, json_output: bool) -> None:
    """列目录内容."""
    # 处理 ~ 路径：远程 shell 会自动展开 ~
    cmd_path = path if path != "~" else ""
    cmd = f"ls -la {cmd_path}" if cmd_path else "ls -la"

    # 执行 ls -la 获取详细信息
    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        _output_json(result.stdout, path)
    else:
        click.echo(result.stdout)


def _output_json(ls_output: str, path: str) -> None:
    """将 ls 输出转换为 JSON 格式."""
    items = []
    lines = ls_output.strip().split("\n")

    # 跳过 total 行
    for line in lines[1:]:
        if not line.strip():
            continue

        parts = line.split()
        if len(parts) < 9:
            continue

        permissions = parts[0]
        size = parts[4]
        name = " ".join(parts[8:])

        # 跳过 . 和 ..
        if name in (".", ".."):
            continue

        item_type = "directory" if permissions.startswith("d") else "file"
        items.append(
            {
                "name": name,
                "type": item_type,
                "permissions": permissions,
                "size": size,
            }
        )

    output = {
        "path": path,
        "items": items,
        "count": len(items),
    }

    click.echo(json.dumps(output, indent=2, ensure_ascii=False))
