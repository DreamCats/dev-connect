"""比较文件差异."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def diff_files(
    file1: str,
    file2: str,
    host_alias: str | None,
    context_lines: int,
    json_output: bool,
) -> None:
    """比较两个远程文件的差异.

    Args:
        file1: 第一个文件路径
        file2: 第二个文件路径
        host_alias: 主机别名
        context_lines: 显示上下文行数
        json_output: 是否 JSON 输出
    """
    cmd = f"diff -u {file1} {file2} || true"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        output = {
            "file1": file1,
            "file2": file2,
            "diff": result.stdout,
            "has_changes": bool(result.stdout),
        }
        click.echo(json.dumps(output, indent=2, ensure_ascii=False))
    else:
        if result.stdout:
            click.echo(result.stdout, nl=False)
        else:
            click.echo("文件相同")


def diff_with_local(
    remote_file: str,
    local_file: str,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """比较远程文件和本地文件的差异.

    Args:
        remote_file: 远程文件路径
        local_file: 本地文件路径
        host_alias: 主机别名
        json_output: 是否 JSON 输出
    """
    import os
    import tempfile

    # 先下载远程文件到临时位置
    with tempfile.NamedTemporaryFile(mode="w", suffix=".tmp", delete=False) as tmp:
        tmp_path = tmp.name

    try:
        # 下载远程文件
        from dev_connect.common.ssh import download

        result = download(remote_file, tmp_path, host_alias)
        if not result.success:
            click.echo(f"错误: 下载远程文件失败: {result.stderr}", err=True)
            sys.exit(1)

        # 比较本地文件
        cmd = f"diff -u {tmp_path} {local_file} || true"
        import subprocess

        diff_result = subprocess.run(cmd, shell=True, capture_output=True, text=True)

        if json_output:
            output = {
                "remote_file": remote_file,
                "local_file": local_file,
                "diff": diff_result.stdout,
                "has_changes": bool(diff_result.stdout),
            }
            click.echo(json.dumps(output, indent=2, ensure_ascii=False))
        else:
            if diff_result.stdout:
                click.echo(diff_result.stdout, nl=False)
            else:
                click.echo("文件相同")
    finally:
        os.unlink(tmp_path)
