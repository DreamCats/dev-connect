"""远程交互式 agent 会话控制."""

from __future__ import annotations

import json
import re
import shlex
import sys
from datetime import datetime
from typing import Any

import click

from dev_connect.common.ssh import run_command

STATE_ROOT = "~/.dev-connect/agents"
TASK_RE = re.compile(r"^[A-Za-z0-9_.-]+$")

AGENT_COMMANDS = {
    "claude": "cc",
    "cc": "cc",
    "codex": "codex",
}


def start_agent(
    task: str,
    cwd: str,
    agent: str,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """启动远程 agent 会话."""
    task = _validate_task(task)
    session = _session_name(task)
    state_dir = _state_dir(task)
    agent_cmd = _agent_command(agent)
    now = _now()

    session_data = {
        "task": task,
        "cwd": cwd,
        "agent": agent,
        "agent_command": agent_cmd,
        "tmux_session": session,
        "created_at": now,
        "updated_at": now,
    }
    remote_agent_cmd = f"zsh -ic {shlex.quote(agent_cmd)}"

    cmd = "\n".join(
        [
            "set -e",
            f"test -d {shlex.quote(cwd)}",
            f"mkdir -p {state_dir}",
            _write_remote_file(f"{state_dir}/session.json", session_data),
            "reused=0",
            (
                f"if tmux has-session -t {shlex.quote(session)} 2>/dev/null; "
                "then reused=1; "
                f"else tmux new-session -d -s {shlex.quote(session)} "
                f"-c {shlex.quote(cwd)} {shlex.quote(remote_agent_cmd)}; "
                "fi"
            ),
            'printf "reused=%s\\n" "$reused"',
        ]
    )
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)

    if json_output:
        payload = session_data | {
            "alive": True,
            "reused": _session_reused(result.stdout),
        }
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(f"started {task}: {session}")
        click.echo(f"cwd: {cwd}")
        click.echo(f"agent: {agent_cmd}")


def send_agent(
    task: str,
    message: str,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """向远程 agent 发送消息."""
    task = _validate_task(task)
    session = _session_name(task)
    now = _now()
    state_dir = _state_dir(task)
    quoted_session = shlex.quote(session)
    steps = ["set -e", f"tmux has-session -t {quoted_session}"]
    if message:
        steps.extend(
            [
                (
                    f"printf %s {shlex.quote(message)} "
                    "| tmux load-buffer -b dev-connect-agent -"
                ),
                f"tmux paste-buffer -b dev-connect-agent -t {quoted_session}",
            ]
        )
    steps.extend(
        [
            f"tmux send-keys -t {quoted_session} Enter",
            f"python3 - <<'PY'\n{_update_session_py(state_dir, now, message)}\nPY",
        ]
    )
    cmd = "\n".join(steps)
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)

    if json_output:
        payload = {
            "task": task,
            "tmux_session": session,
            "sent": True,
            "bytes": len(message.encode()),
            "updated_at": now,
        }
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(f"sent to {task}")


def tail_agent(
    task: str,
    lines: int,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """读取远程 agent 最近输出."""
    task = _validate_task(task)
    session = _session_name(task)
    lines = max(lines, 1)
    cmd = (
        f"tmux has-session -t {shlex.quote(session)} "
        f"&& tmux capture-pane -p -t {shlex.quote(session)} -S -{lines}"
    )
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)

    if json_output:
        payload = {
            "task": task,
            "tmux_session": session,
            "alive": True,
            "output": result.stdout,
        }
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(result.stdout, nl=False)


def interrupt_agent(
    task: str,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """打断远程 agent 当前动作."""
    task = _validate_task(task)
    session = _session_name(task)
    cmd = (
        f"tmux has-session -t {shlex.quote(session)} "
        f"&& tmux send-keys -t {shlex.quote(session)} C-c"
    )
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)

    if json_output:
        payload = {"task": task, "tmux_session": session, "interrupted": True}
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(f"interrupted {task}")


def status_agent(
    task: str,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """查看远程 agent 会话状态."""
    task = _validate_task(task)
    session = _session_name(task)
    state_file = f"{_state_dir(task)}/session.json"

    cmd = f"""python3 - <<'PY'
import json
import subprocess
from pathlib import Path

session = {session!r}
state_file = Path({state_file!r}).expanduser()
alive = subprocess.run(
    ["tmux", "has-session", "-t", session],
    stdout=subprocess.DEVNULL,
    stderr=subprocess.DEVNULL,
).returncode == 0

data = {{"task": {task!r}, "tmux_session": session, "alive": alive}}
if state_file.exists():
    data.update(json.loads(state_file.read_text()))

cwd = data.get("cwd")
if cwd:
    branch = subprocess.run(
        ["git", "-C", cwd, "branch", "--show-current"],
        capture_output=True,
        text=True,
    )
    status = subprocess.run(
        ["git", "-C", cwd, "status", "--short"],
        capture_output=True,
        text=True,
    )
    data["branch"] = branch.stdout.strip() if branch.returncode == 0 else ""
    data["git_status"] = status.stdout if status.returncode == 0 else ""
    data["dirty"] = bool(data["git_status"])

print(json.dumps(data, ensure_ascii=False))
PY"""
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)
    payload = json.loads(result.stdout)

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        _print_status(payload)


def _validate_task(task: str) -> str:
    if not TASK_RE.fullmatch(task):
        raise click.ClickException("task 只能包含字母、数字、下划线、点和短横线")
    return task


def _session_name(task: str) -> str:
    return f"dc-agent-{task}"


def _state_dir(task: str) -> str:
    return f"{STATE_ROOT}/{task}"


def _agent_command(agent: str) -> str:
    return AGENT_COMMANDS.get(agent, agent)


def _now() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def _write_remote_file(path: str, data: dict[str, Any]) -> str:
    content = json.dumps(data, ensure_ascii=False, indent=2)
    return "\n".join(
        [
            f"cat > {path} <<'DEV_CONNECT_JSON'",
            content,
            "DEV_CONNECT_JSON",
        ]
    )


def _update_session_py(state_dir: str, updated_at: str, message: str) -> str:
    return f"""import json
from pathlib import Path

state_file = Path({state_dir!r}).expanduser() / "session.json"
if not state_file.exists():
    raise SystemExit(0)
data = json.loads(state_file.read_text())
data["updated_at"] = {updated_at!r}
data["last_sent_at"] = {updated_at!r}
data["last_message"] = {message!r}
state_file.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\\n")
"""


def _exit_on_failure(stderr: str, returncode: int) -> None:
    if returncode == 0:
        return
    if stderr:
        click.echo(stderr, err=True, nl=False)
    sys.exit(returncode)


def _session_reused(stdout: str) -> bool:
    return "reused=1" in stdout.splitlines()


def _print_status(payload: dict[str, Any]) -> None:
    click.echo(f"task: {payload.get('task', '')}")
    click.echo(f"alive: {payload.get('alive', False)}")
    click.echo(f"session: {payload.get('tmux_session', '')}")
    click.echo(f"cwd: {payload.get('cwd', '')}")
    click.echo(f"agent: {payload.get('agent_command') or payload.get('agent', '')}")
    if payload.get("branch"):
        click.echo(f"branch: {payload['branch']}")
    git_status = payload.get("git_status", "")
    if git_status:
        click.echo("git status:")
        click.echo(git_status, nl=False)
    elif payload.get("cwd"):
        click.echo("git status: clean")
