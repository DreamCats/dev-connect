"""查看文件末尾内容."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def show_tail(
    file: str,
    host_alias: str | None,
    lines: int,
    json_output: bool,
) -> None:
    """查看文件末尾内容."""
    cmd = f"tail -n {lines} {file}"
    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        output = {
            "file": file,
            "lines": lines,
            "content": result.stdout,
            "size": len(result.stdout),
        }
        click.echo(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        click.echo(result.stdout, nl=False)
