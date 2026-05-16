"""远程 patch 命令辅助逻辑测试."""

from dev_connect.commands.patch import _build_patch_cmd


def test_build_patch_cmd_checks_before_apply():
    """patch 默认先校验再应用."""
    cmd = _build_patch_cmd("/repo", False)

    assert "cd /repo" in cmd
    assert 'git apply --check "$patch_file"' in cmd
    assert 'git apply "$patch_file"' in cmd


def test_build_patch_cmd_check_only_skips_apply():
    """check-only 不执行 git apply."""
    cmd = _build_patch_cmd("/repo", True)

    assert 'git apply --check "$patch_file"' in cmd
    assert 'git apply "$patch_file"' not in cmd
