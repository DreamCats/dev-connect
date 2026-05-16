"""执行远程命令."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.config import get_host
from dev_connect.common.ssh import run_command


def execute_command(
    cmd: str,
    host_alias: str | None,
    timeout: int | None,
    json_output: bool,
    shell: str | None,
) -> None:
    """执行远程命令."""
    active_shell = _resolve_shell(host_alias, shell)
    active_timeout = _resolve_timeout(host_alias, timeout)
    result = run_command(cmd, host_alias, active_timeout, shell=active_shell)

    if json_output:
        output = {
            "command": cmd,
            "shell": active_shell,
            "timeout": active_timeout,
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


def _resolve_shell(host_alias: str | None, shell: str | None) -> str | None:
    """命令行参数优先，其次使用主机配置."""
    if shell == "none":
        return None
    if shell:
        return shell
    configured = get_host(host_alias).shell
    if configured == "none":
        return None
    return configured


def _resolve_timeout(host_alias: str | None, timeout: int | None) -> int:
    """命令行参数优先，其次使用主机配置."""
    if timeout is not None:
        return timeout
    return get_host(host_alias).exec_timeout or 30
