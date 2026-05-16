"""远程 Git 仓库状态快照."""

from __future__ import annotations

import json
import sys
from typing import Any

import click

from dev_connect.common.ssh import quote_remote_path, run_command


def show_repo_status(
    cwd: str,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """返回远程仓库的 supervisor 快照."""
    result = run_command(_build_status_cmd(cwd), host_alias, timeout=30)

    try:
        payload: dict[str, Any] = json.loads(result.stdout)
    except json.JSONDecodeError:
        payload = {
            "cwd": cwd,
            "success": False,
            "returncode": result.returncode,
            "stdout": result.stdout,
            "stderr": result.stderr,
        }

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        _output_plain(payload)
        if result.stderr:
            click.echo(result.stderr, err=True, nl=False)

    if not result.success:
        sys.exit(result.returncode)


def _build_status_cmd(cwd: str) -> str:
    """构建远端状态采集脚本."""
    script = r"""
import json
import subprocess
import sys


def run(args):
    result = subprocess.run(args, capture_output=True, text=True)
    return {
        "returncode": result.returncode,
        "stdout": result.stdout,
        "stderr": result.stderr,
    }


top = run(["git", "rev-parse", "--show-toplevel"])
if top["returncode"] != 0:
    print(json.dumps({
        "success": False,
        "error": "not a git repository",
        "stderr": top["stderr"],
    }, ensure_ascii=False))
    sys.exit(top["returncode"])

branch = run(["git", "branch", "--show-current"])
upstream = run(["git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"])
status = run(["git", "status", "--short", "--branch"])
diff_stat = run(["git", "diff", "--stat"])
recent = run(["git", "log", "--oneline", "-5"])

status_lines = [line for line in status["stdout"].splitlines() if line]
dirty = any(not line.startswith("## ") for line in status_lines)

payload = {
    "success": True,
    "repo_root": top["stdout"].strip(),
    "branch": branch["stdout"].strip(),
    "upstream": upstream["stdout"].strip() if upstream["returncode"] == 0 else "",
    "dirty": dirty,
    "status": status["stdout"],
    "diff_stat": diff_stat["stdout"],
    "recent_commits": recent["stdout"],
}
print(json.dumps(payload, ensure_ascii=False))
"""
    return "\n".join(
        [
            "set -e",
            f"cd {quote_remote_path(cwd)}",
            "python3 - <<'PY'",
            script.strip(),
            "PY",
        ]
    )


def _output_plain(payload: dict[str, Any]) -> None:
    """输出便于人读的仓库状态."""
    if not payload.get("success"):
        click.echo(f"错误: {payload.get('error', 'repo-status failed')}", err=True)
        stderr = str(payload.get("stderr", ""))
        if stderr:
            click.echo(stderr, err=True, nl=False)
        return

    click.echo(f"repo: {payload.get('repo_root', '')}")
    click.echo(f"branch: {payload.get('branch', '')}")
    click.echo(f"upstream: {payload.get('upstream') or '(none)'}")
    click.echo(f"dirty: {payload.get('dirty')}")
    click.echo("\nstatus:")
    click.echo(str(payload.get("status") or "(clean)\n"), nl=False)
    click.echo("\ndiff stat:")
    click.echo(str(payload.get("diff_stat") or "(no diff)\n"), nl=False)
    click.echo("\nrecent commits:")
    click.echo(str(payload.get("recent_commits") or ""), nl=False)
