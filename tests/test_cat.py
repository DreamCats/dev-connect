"""远程 cat 命令辅助逻辑测试."""

from dev_connect.commands.cat import _build_cat_cmd


def test_build_cat_cmd_uses_cwd_and_paths():
    """批量读取脚本进入 cwd 后读取相对路径."""
    cmd = _build_cat_cmd(("pyproject.toml", "src/app.py"), "~/repo")

    assert "cd $HOME/repo" in cmd
    assert "pyproject.toml" in cmd
    assert "src/app.py" in cmd
    assert "Path(path).read_text" in cmd
