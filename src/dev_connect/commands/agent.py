"""远程交互式 agent 会话控制."""

from __future__ import annotations

import json
import re
import shlex
import sys
import time
from datetime import datetime
from typing import Any

import click

from dev_connect.common.ssh import run_command

STATE_ROOT = "~/.dev-connect/agents"
TASK_RE = re.compile(r"^[A-Za-z0-9_.-]+$")

AGENT_COMMANDS = {
    "claude": "if whence -w cc >/dev/null 2>&1; then cc; else claude; fi",
    "cc": "cc",
    "codex": "codex",
}


def start_agent(
    task: str,
    cwd: str,
    agent: str,
    initial_message: str | None,
    wait: int,
    lines: int,
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

    payload: dict[str, Any] = session_data | {
        "alive": True,
        "reused": _session_reused(result.stdout),
    }

    if initial_message is not None:
        send_payload = _send_message(task, initial_message, host_alias)
        payload["initial_message"] = send_payload
        if wait > 0:
            time.sleep(wait)
            payload["output"] = _capture_tail(task, lines, None, False, host_alias)

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(f"started {task}: {session}")
        click.echo(f"cwd: {cwd}")
        click.echo(f"agent: {agent_cmd}")
        if initial_message is not None:
            click.echo(f"sent to {task}")
            if wait > 0 and payload.get("output"):
                click.echo(str(payload["output"]), nl=False)


def send_agent(
    task: str,
    message: str,
    wait: int,
    lines: int,
    chars: int | None,
    compact: bool,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """向远程 agent 发送消息."""
    payload = _send_message(task, message, host_alias)

    if wait > 0:
        time.sleep(wait)
        payload["output"] = _capture_tail(task, lines, chars, compact, host_alias)

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(f"sent to {task}")
        if wait > 0 and payload.get("output"):
            click.echo(str(payload["output"]), nl=False)


def _send_message(
    task: str,
    message: str,
    host_alias: str | None,
) -> dict[str, Any]:
    """向远程 tmux session 发送消息并返回结构化结果."""
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

    return {
        "task": task,
        "tmux_session": session,
        "sent": True,
        "bytes": len(message.encode()),
        "updated_at": now,
    }


def _capture_tail(
    task: str,
    lines: int,
    chars: int | None,
    compact: bool,
    host_alias: str | None,
) -> str:
    """读取并格式化 tmux pane 最近输出."""
    task = _validate_task(task)
    session = _session_name(task)
    lines = max(lines, 1)
    cmd = (
        f"tmux has-session -t {shlex.quote(session)} "
        f"&& tmux capture-pane -p -t {shlex.quote(session)} -S -{lines}"
    )
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)
    return _format_output(result.stdout, chars, compact)


def _format_output(output: str, chars: int | None, compact: bool) -> str:
    """对终端输出做机械截断和去空行."""
    if compact:
        output = "\n".join(line for line in output.splitlines() if line.strip())
        if output:
            output += "\n"
    if chars is not None and chars > 0 and len(output) > chars:
        output = output[-chars:]
    return output


def tail_agent(
    task: str,
    lines: int,
    chars: int | None,
    compact: bool,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """读取远程 agent 最近输出."""
    output = _capture_tail(task, lines, chars, compact, host_alias)
    session = _session_name(task)

    if json_output:
        payload = {
            "task": task,
            "tmux_session": session,
            "alive": True,
            "output": output,
        }
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(output, nl=False)


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
    preview_lines: int,
    preview_chars: int,
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

if alive:
    preview = subprocess.run(
        ["tmux", "capture-pane", "-p", "-t", session, "-S", f"-{preview_lines}"],
        capture_output=True,
        text=True,
    )
    if preview.returncode == 0:
        lines = [line for line in preview.stdout.splitlines() if line.strip()]
        text = "\\n".join(lines)
        if text:
            text += "\\n"
        if len(text) > {preview_chars}:
            text = text[-{preview_chars}:]
        data["tail_preview"] = text

print(json.dumps(data, ensure_ascii=False))
PY"""
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)
    payload = json.loads(result.stdout)

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        _print_status(payload)


def diff_agent(
    task: str,
    stat: bool,
    name_only: bool,
    file_path: str | None,
    max_chars: int,
    full: bool,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """查看远程 agent 工作目录的 git diff."""
    task = _validate_task(task)
    state_file = f"{_state_dir(task)}/session.json"
    diff_args = ["diff"]
    if stat:
        diff_args.append("--stat")
    if name_only:
        diff_args.append("--name-only")
    if file_path:
        diff_args.extend(["--", file_path])

    cmd = f"""python3 - <<'PY'
import json
import subprocess
import sys
from pathlib import Path

state_file = Path({state_file!r}).expanduser()
if not state_file.exists():
    print("session state not found", file=sys.stderr)
    raise SystemExit(1)

data = json.loads(state_file.read_text())
cwd = data.get("cwd")
if not cwd:
    print("session cwd not found", file=sys.stderr)
    raise SystemExit(1)

args = ["git", "-C", cwd] + {diff_args!r}
result = subprocess.run(args, capture_output=True, text=True)
stdout = result.stdout
stderr = result.stderr
truncated = False
max_chars = {max_chars}
if not {full!r} and max_chars > 0:
    if len(stdout) > max_chars:
        stdout = (
            stdout[:max_chars]
            + "\\n... truncated; use --full for complete diff ...\\n"
        )
        truncated = True
    if len(stderr) > max_chars:
        stderr = (
            stderr[:max_chars]
            + "\\n... truncated; use --full for complete stderr ...\\n"
        )
        truncated = True
payload = {{
    "task": {task!r},
    "cwd": cwd,
    "command": " ".join(args),
    "returncode": result.returncode,
    "stdout": stdout,
    "stderr": stderr,
    "has_changes": bool(result.stdout),
    "truncated": truncated,
}}
print(json.dumps(payload, ensure_ascii=False))
raise SystemExit(result.returncode)
PY"""
    result = run_command(cmd, host_alias)
    if result.stdout:
        payload = json.loads(result.stdout)
        if json_output:
            click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
        else:
            if payload["stdout"]:
                click.echo(payload["stdout"], nl=False)
            if payload["stderr"]:
                click.echo(payload["stderr"], err=True, nl=False)
        if payload["returncode"] != 0:
            sys.exit(payload["returncode"])
        return
    _exit_on_failure(result.stderr, result.returncode)


def list_agents(host_alias: str | None, json_output: bool) -> None:
    """列出远程 agent 会话."""
    cmd = f"""python3 - <<'PY'
import json
import subprocess
from pathlib import Path

root = Path({STATE_ROOT!r}).expanduser()
items = []
if root.exists():
    for state_file in sorted(root.glob("*/session.json")):
        try:
            data = json.loads(state_file.read_text())
        except Exception as exc:
            data = {{"task": state_file.parent.name, "error": str(exc)}}
        task = data.get("task") or state_file.parent.name
        session = data.get("tmux_session") or f"dc-agent-{{task}}"
        alive = subprocess.run(
            ["tmux", "has-session", "-t", session],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        ).returncode == 0
        data["task"] = task
        data["tmux_session"] = session
        data["alive"] = alive
        items.append(data)

print(json.dumps(items, ensure_ascii=False))
PY"""
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)
    payload = json.loads(result.stdout)

    if json_output:
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        if not payload:
            click.echo("no agent sessions")
            return
        for item in payload:
            alive = "alive" if item.get("alive") else "dead"
            task = item.get("task", "")
            cwd = item.get("cwd", "")
            agent = item.get("agent", "")
            click.echo(f"{task}\t{alive}\t{agent}\t{cwd}")


def stop_agent(
    task: str,
    purge: bool,
    host_alias: str | None,
    json_output: bool,
) -> None:
    """停止远程 agent 会话."""
    task = _validate_task(task)
    session = _session_name(task)
    state_dir = _state_dir(task)
    steps = [
        "alive=0",
        (
            f"if tmux has-session -t {shlex.quote(session)} 2>/dev/null; "
            f"then tmux kill-session -t {shlex.quote(session)}; alive=1; fi"
        ),
    ]
    if purge:
        steps.append(f"rm -rf {state_dir}")
    steps.append('printf "stopped=%s\\n" "$alive"')
    cmd = "\n".join(steps)
    result = run_command(cmd, host_alias)
    _exit_on_failure(result.stderr, result.returncode)
    stopped = "stopped=1" in result.stdout.splitlines()

    if json_output:
        payload = {
            "task": task,
            "tmux_session": session,
            "stopped": stopped,
            "purged": purge,
        }
        click.echo(json.dumps(payload, indent=2, ensure_ascii=False))
    else:
        click.echo(f"stopped {task}" if stopped else f"{task} was not running")
        if purge:
            click.echo(f"purged {_state_dir(task)}")


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
    if payload.get("tail_preview"):
        click.echo("tail preview:")
        click.echo(payload["tail_preview"], nl=False)
