"""查看文件内容."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def show_file(path: str, host_alias: str | None, json_output: bool) -> None:
    """查看文件内容."""
    # 处理 ~ 路径：远程 shell 会自动展开 ~
    cmd = f"cat {path}" if not path.startswith("~") else f"cat {path}"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        output = {
            "path": path,
            "content": result.stdout,
            "size": len(result.stdout),
        }
        click.echo(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        click.echo(result.stdout, nl=False)
