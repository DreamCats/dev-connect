"""远程 repo-status 命令辅助逻辑测试."""

from dev_connect.commands.repo_status import _build_status_cmd


def test_build_status_cmd_collects_supervisor_snapshot():
    """状态脚本采集 supervisor 常用字段."""
    cmd = _build_status_cmd("/repo")

    assert "cd /repo" in cmd
    assert "rev-parse" in cmd
    assert "status" in cmd
    assert "diff" in cmd
    assert "log" in cmd
