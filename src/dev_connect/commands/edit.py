"""精确编辑远程文件."""

from __future__ import annotations

import sys

import click

from dev_connect.common.ssh import run_command


def replace_content(
    path: str,
    old: str,
    new: str,
    host_alias: str | None,
    all_occurrences: bool,
) -> None:
    """替换文件内容.

    Args:
        path: 远程文件路径
        old: 要替换的内容
        new: 新内容
        host_alias: 主机别名
        all_occurrences: 是否替换所有匹配
    """
    # 转义 sed 分隔符
    old_escaped = old.replace("/", "\\/").replace("&", "\\&")
    new_escaped = new.replace("/", "\\/").replace("&", "\\&")

    flag = "g" if all_occurrences else ""
    cmd = f"sed -i 's/{old_escaped}/{new_escaped}/{flag}' {path}"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    scope = "所有" if all_occurrences else "首次"
    click.echo(f"已替换 {scope}匹配: '{old}' -> '{new}'")


def insert_lines(
    path: str,
    line_num: int,
    content: str,
    host_alias: str | None,
    after: bool,
) -> None:
    """在指定行插入内容.

    Args:
        path: 远程文件路径
        line_num: 行号
        content: 要插入的内容
        host_alias: 主机别名
        after: True 表示在行后插入，False 表示在行前插入
    """
    # 处理多行内容
    lines = content.split("\n")
    escaped = (line.replace("/", "\\/").replace("&", "\\&") for line in lines)
    sed_lines = "\\n".join(escaped)

    cmd_type = "a" if after else "i"
    cmd = f"sed -i '{line_num}{cmd_type} {sed_lines}' {path}"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    position = "后" if after else "前"
    click.echo(f"已在第 {line_num} 行{position}插入内容")


def delete_lines(
    path: str,
    start_line: int,
    end_line: int | None,
    host_alias: str | None,
) -> None:
    """删除指定行.

    Args:
        path: 远程文件路径
        start_line: 起始行号
        end_line: 结束行号，None 表示只删除一行
        host_alias: 主机别名
    """
    if end_line is None:
        line_range = str(start_line)
    else:
        line_range = f"{start_line},{end_line}"

    cmd = f"sed -i '{line_range}d' {path}"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if end_line is None:
        click.echo(f"已删除第 {start_line} 行")
    else:
        click.echo(f"已删除第 {start_line}-{end_line} 行")


def update_line(
    path: str,
    line_num: int,
    content: str,
    host_alias: str | None,
) -> None:
    """修改指定行内容.

    Args:
        path: 远程文件路径
        line_num: 行号
        content: 新的行内容
        host_alias: 主机别名
    """
    content_escaped = content.replace("/", "\\/").replace("&", "\\&")
    cmd = f"sed -i '{line_num}s/.*/{content_escaped}/' {path}"

    result = run_command(cmd, host_alias)

    if not result.success:
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    click.echo(f"已修改第 {line_num} 行")
