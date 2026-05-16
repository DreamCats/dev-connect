"""远程 exec 命令辅助逻辑测试."""

from dev_connect.commands.exec import _resolve_timeout
from dev_connect.common.ssh import _wrap_shell_cmd


def test_wrap_shell_cmd_keeps_default_direct():
    """未指定 shell 时保持原命令."""
    assert _wrap_shell_cmd("echo hello", None) == "echo hello"
    assert _wrap_shell_cmd("echo hello", "none") == "echo hello"


def test_wrap_shell_cmd_uses_zsh_interactive_preset():
    """zsh 预设会加载远端 ~/.zshrc."""
    assert _wrap_shell_cmd("echo $PATH", "zsh") == "zsh -ic 'echo $PATH'"


def test_wrap_shell_cmd_quotes_nested_command():
    """复杂命令作为一个参数交给远端 shell."""
    assert _wrap_shell_cmd("echo 'hello world'", "zsh-login") == (
        "zsh -lic 'echo '\"'\"'hello world'\"'\"''"
    )


def test_resolve_timeout_uses_cli_value():
    """命令行 timeout 优先."""
    assert _resolve_timeout(None, 90) == 90
