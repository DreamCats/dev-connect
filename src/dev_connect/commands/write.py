"""写入文件内容到远程主机."""

from __future__ import annotations

import sys

import click

from dev_connect.common.ssh import run_command


def write_file(
    path: str,
    content: str | None,
    host_alias: str | None,
    append: bool,
) -> None:
    """写入文件内容到远程主机.

    Args:
        path: 远程文件路径
        content: 文件内容，None 表示从 stdin 读取
        host_alias: 主机别名
        append: 是否追加模式
    """
    if content is None:
        content = sys.stdin.read()

    # 使用 heredoc 写入，避免转义问题
    operator = ">>" if append else ">"
    # 用 cat + heredoc 写入内容，delimiter 用随机字符串避免冲突
    cmd = f"cat {operator} {path} << 'DEV_CONNECT_EOF'\n{content}\nDEV_CONNECT_EOF"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    mode = "追加" if append else "写入"
    click.echo(f"已{mode}: {path} ({len(content)} 字节)")
