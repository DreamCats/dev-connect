"""从远程主机下载文件."""

from __future__ import annotations

import sys

import click

from dev_connect.common.ssh import download


def download_file(remote_path: str, local_path: str, host_alias: str | None) -> None:
    """从远程主机下载文件."""
    result = download(remote_path, local_path, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    click.echo(f"已下载: {remote_path} -> {local_path}")
