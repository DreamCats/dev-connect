"""搜索代码内容，优先 rg，降级 grep."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import run_command


def search_content(
    pattern: str,
    path: str,
    host_alias: str | None,
    include: str | None,
    line_number: bool,
    json_output: bool,
) -> None:
    """搜索代码内容."""
    # 检查 rg 是否可用
    rg_check = run_command("which rg", host_alias, timeout=5)
    use_rg = rg_check.success

    cmd = _build_cmd(pattern, path, use_rg, include, line_number)
    result = run_command(cmd, host_alias)

    if not result.success and result.returncode != 1:
        # rg/grep 返回 1 表示无匹配，不是错误
        click.echo(f"错误: {result.stderr}", err=True)
        sys.exit(1)

    if json_output:
        _output_json(result.stdout, pattern, path, use_rg)
    else:
        click.echo(result.stdout, nl=False)


def _build_cmd(
    pattern: str,
    path: str,
    use_rg: bool,
    include: str | None,
    line_number: bool,
) -> str:
    """构建搜索命令."""
    if use_rg:
        # rg 语法
        parts = ["rg"]
        if line_number:
            parts.append("-n")
        if include:
            parts.append(f"--glob '{include}'")
        parts.append(f"'{pattern}'")
        parts.append(path)
    else:
        # grep 语法
        parts = ["grep", "-r"]
        if line_number:
            parts.append("-n")
        if include:
            parts.append(f"--include='{include}'")
        parts.append(f"'{pattern}'")
        parts.append(path)

    return " ".join(parts)


def _output_json(output: str, pattern: str, path: str, use_rg: bool) -> None:
    """将搜索结果转换为 JSON 格式."""
    matches = []
    lines = output.strip().split("\n") if output.strip() else []

    for line in lines:
        if not line:
            continue

        # 解析 file:line:content 格式
        parts = line.split(":", 2)
        if len(parts) >= 3:
            matches.append(
                {
                    "file": parts[0],
                    "line": int(parts[1]) if parts[1].isdigit() else parts[1],
                    "content": parts[2],
                }
            )
        else:
            matches.append(
                {
                    "file": "",
                    "line": 0,
                    "content": line,
                }
            )

    result = {
        "pattern": pattern,
        "path": path,
        "tool": "rg" if use_rg else "grep",
        "matches": matches,
        "count": len(matches),
    }

    click.echo(json.dumps(result, indent=2, ensure_ascii=False))
