"""查看文件开头内容."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def show_head(
    path: str,
    host_alias: str | None,
    lines: int,
    json_output: bool,
) -> None:
    """查看文件开头内容.

    Args:
        path: 文件路径
        host_alias: 主机别名
        lines: 显示行数
        json_output: 是否 JSON 输出
    """
    cmd = f"head -n {lines} {path}"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        output = {
            "path": path,
            "lines": lines,
            "content": result.stdout,
            "size": len(result.stdout),
        }
        click.echo(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        click.echo(result.stdout, nl=False)
