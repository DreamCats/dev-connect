"""执行远程命令."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def execute_command(
    cmd: str,
    host_alias: str | None,
    timeout: int,
    json_output: bool,
) -> None:
    """执行远程命令."""
    result = run_command(cmd, host_alias, timeout)

    if json_output:
        output = {
            "command": cmd,
            "returncode": result.returncode,
            "stdout": result.stdout,
            "stderr": result.stderr,
            "success": result.success,
        }
        click.echo(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        if result.stdout:
            click.echo(result.stdout, nl=False)
        if result.stderr:
            click.echo(result.stderr, err=True, nl=False)

    if not result.success:
        sys.exit(result.returncode)
