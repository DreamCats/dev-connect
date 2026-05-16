"""远程 agent 命令辅助逻辑测试."""

import pytest
from click import ClickException

from dev_connect.commands.agent import (
    _agent_command,
    _session_name,
    _state_dir,
    _validate_task,
)


def test_validate_task_accepts_safe_name():
    """允许安全的 task 名称."""
    assert _validate_task("task.sales_block-1") == "task.sales_block-1"


def test_validate_task_rejects_path_like_name():
    """拒绝可能影响远程路径或 tmux 名称的 task."""
    with pytest.raises(ClickException):
        _validate_task("../task")


def test_session_paths_are_deterministic():
    """task 能稳定映射到 tmux session 和状态目录."""
    assert _session_name("sales-block") == "dc-agent-sales-block"
    assert _state_dir("sales-block") == "~/.dev-connect/agents/sales-block"


def test_agent_command_uses_known_shortcuts():
    """常用 agent 名称映射为远程启动命令."""
    assert _agent_command("claude") == (
        "if whence -w cc >/dev/null 2>&1; then cc; else claude; fi"
    )
    assert _agent_command("cc") == "cc"
    assert _agent_command("codex") == "codex"


def test_agent_command_allows_explicit_command():
    """允许调用方传入带参数的远程启动命令."""
    assert (
        _agent_command("claude --dangerously-skip-permissions")
        == "claude --dangerously-skip-permissions"
    )


def test_validate_task_accepts_list_safe_name():
    """list/stop/diff 复用同一 task 命名规则."""
    assert _validate_task("agent.demo_2") == "agent.demo_2"
