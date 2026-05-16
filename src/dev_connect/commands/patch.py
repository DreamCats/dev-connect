"""应用远程 git patch."""

from __future__ import annotations

import json
import sys

import click

from dev_connect.common.ssh import quote_remote_path, run_command


def apply_git_patch(
    cwd: str,
    patch_content: str | None,
    check_only: bool,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """在远程仓库中原子应用标准 git patch."""
    if patch_content is None:
        patch_content = sys.stdin.read()

    cmd = _build_patch_cmd(cwd, check_only)
    result = run_command(cmd, host_alias, timeout=120, stdin=patch_content)

    payload = {
        "cwd": cwd,
        "check_only": check_only,
        "applied": result.success and not check_only,
        "returncode": result.returncode,
        "stdout": result.stdout,
        "stderr": result.stderr,
        "success": result.success,
    }

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        if result.stdout:
            click.echo(result.stdout, nl=False)
        if result.stderr:
            click.echo(result.stderr, err=True, nl=False)
        if result.success:
            action = "checked" if check_only else "applied"
            click.echo(f"patch {action}: {cwd}")

    if not result.success:
        sys.exit(result.returncode)


def _build_patch_cmd(cwd: str, check_only: bool) -> str:
    """构建远端 git apply 命令."""
    apply_step = ":" if check_only else 'git apply "$patch_file"'
    return "\n".join(
        [
            "set -e",
            f"cd {quote_remote_path(cwd)}",
            'patch_file="$(mktemp)"',
            "trap 'rm -f \"$patch_file\"' EXIT",
            'cat > "$patch_file"',
            'git apply --check "$patch_file"',
            apply_step,
        ]
    )
