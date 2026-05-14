"""上传文件到远程主机."""

from __future__ import annotations

import sys

import click

from dev_connect.common.ssh import upload


def upload_file(local_path: str, remote_path: str, host_alias: str | None) -> None:
    """上传文件到远程主机."""
    # 处理远程路径中的 ~
    remote_path = remote_path.replace("~", "~")

    result = upload(local_path, remote_path, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    click.echo(f"已上传: {local_path} -> {remote_path}")
