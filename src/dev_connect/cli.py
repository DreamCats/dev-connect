"""统一 CLI 入口."""

import sys
import traceback
from pathlib import Path

import click

from dev_connect import __version__
from dev_connect.common.config import load, save
from dev_connect.models import HostConfig


def _normalize_path(path: str) -> str:
    """处理路径，将本地 home 目录转换为 ~."""
    home = str(Path.home())
    if path.startswith(home):
        return path.replace(home, "~", 1)
    return path


@click.group()
@click.version_option(version=__version__, prog_name="dev")
@click.option("--json", "json_output", is_flag=True, help="JSON 格式输出")
@click.option("--verbose", is_flag=True, help="详细输出（显示完整错误栈）")
@click.pass_context
def main(ctx: click.Context, json_output: bool, verbose: bool) -> None:
    """远程开发机文件交互 CLI.

    快速连接远程开发机，支持文件传输、目录浏览、命令执行。

    \b
    基本用法：
      dev ls ~/projects              # 列目录
      dev cat ~/.bashrc              # 看文件
      dev push ./local.txt ~/remote  # 上传
      dev pull ~/remote ./local.txt  # 下载
      dev exec "uname -a"            # 执行命令
      dev tree ~/projects            # 目录树

    \b
    搜索：
      dev grep "def main" ~/projects # 搜索代码（优先 rg，降级 grep）
      dev grep "TODO" -g "*.py"      # 只搜 Python 文件
      dev find "*.py" ~/projects     # 按名称搜索文件
      dev tail ~/logs/app.log        # 看日志末尾

    \b
    指定主机：
      dev ls --host sgdev ~/projects
      dev cat -H dev ~/.bashrc

    \b
    JSON 输出（便于 Agent 解析）：
      dev --json ls ~/projects
      dev --json exec "echo hello"

    \b
    配置管理：
      dev config show
      dev config add sgdev 10.251.233.15 --default
    """
    ctx.ensure_object(dict)
    ctx.obj["json_output"] = json_output
    ctx.obj["verbose"] = verbose


@main.command()
@click.argument("path", default="~")
@click.option("--host", "-H", "host_alias", help="主机别名，如 sgdev")
@click.pass_context
def ls(ctx: click.Context, path: str, host_alias: str | None) -> None:
    """列目录内容."""
    from dev_connect.commands.ls import list_dir

    json_output = ctx.obj.get("json_output", False)
    list_dir(_normalize_path(path), host_alias, json_output)


@main.command()
@click.argument("path")
@click.option("--host", "-h", "host_alias", help="主机别名，如 @sgdev")
@click.pass_context
def cat(ctx: click.Context, path: str, host_alias: str | None) -> None:
    """查看文件内容."""
    from dev_connect.commands.cat import show_file

    json_output = ctx.obj.get("json_output", False)
    show_file(_normalize_path(path), host_alias, json_output)


@main.command()
@click.argument("local_path")
@click.argument("remote_path")
@click.option("--host", "-h", "host_alias", help="主机别名，如 @sgdev")
@click.pass_context
def push(
    ctx: click.Context, local_path: str, remote_path: str, host_alias: str | None
) -> None:
    """上传文件到远程主机."""
    from dev_connect.commands.push import upload_file

    upload_file(local_path, _normalize_path(remote_path), host_alias)


@main.command()
@click.argument("remote_path")
@click.argument("local_path")
@click.option("--host", "-h", "host_alias", help="主机别名，如 @sgdev")
@click.pass_context
def pull(
    ctx: click.Context, remote_path: str, local_path: str, host_alias: str | None
) -> None:
    """从远程主机下载文件."""
    from dev_connect.commands.pull import download_file

    download_file(_normalize_path(remote_path), local_path, host_alias)


@main.command()
@click.argument("cmd")
@click.option("--host", "-h", "host_alias", help="主机别名，如 @sgdev")
@click.option("--timeout", "-t", default=30, help="超时时间（秒）")
@click.pass_context
def exec(ctx: click.Context, cmd: str, host_alias: str | None, timeout: int) -> None:
    """执行远程命令."""
    from dev_connect.commands.exec import execute_command

    json_output = ctx.obj.get("json_output", False)
    execute_command(cmd, host_alias, timeout, json_output)


@main.command()
@click.argument("path", default="~")
@click.option("--host", "-h", "host_alias", help="主机别名，如 @sgdev")
@click.option("--depth", "-d", default=3, help="目录深度")
@click.pass_context
def tree(ctx: click.Context, path: str, host_alias: str | None, depth: int) -> None:
    """显示目录树."""
    from dev_connect.commands.tree import show_tree

    json_output = ctx.obj.get("json_output", False)
    show_tree(_normalize_path(path), host_alias, depth, json_output)


@main.command()
@click.argument("pattern")
@click.argument("path", default=".")
@click.option("--host", "-H", "host_alias", help="主机别名，如 sgdev")
@click.option("--include", "-g", help="文件名匹配，如 '*.py'")
@click.option("--no-line-number", "-N", is_flag=True, help="不显示行号")
@click.pass_context
def grep(
    ctx: click.Context,
    pattern: str,
    path: str,
    host_alias: str | None,
    include: str | None,
    no_line_number: bool,
) -> None:
    """搜索代码内容，优先 rg，降级 grep."""
    from dev_connect.commands.grep import search_content

    json_output = ctx.obj.get("json_output", False)
    search_content(
        pattern,
        _normalize_path(path),
        host_alias,
        include,
        not no_line_number,
        json_output,
    )


@main.command()
@click.argument("name")
@click.argument("path", default=".")
@click.option("--host", "-H", "host_alias", help="主机别名，如 sgdev")
@click.option("--type", "-t", "file_type", help="文件类型，f(文件) 或 d(目录)")
@click.pass_context
def find(
    ctx: click.Context,
    name: str,
    path: str,
    host_alias: str | None,
    file_type: str | None,
) -> None:
    """按名称搜索文件."""
    from dev_connect.commands.find import find_files

    json_output = ctx.obj.get("json_output", False)
    find_files(name, _normalize_path(path), host_alias, file_type, json_output)


@main.command()
@click.argument("file")
@click.option("--host", "-H", "host_alias", help="主机别名，如 sgdev")
@click.option("--lines", "-n", default=20, help="显示行数")
@click.pass_context
def tail(
    ctx: click.Context, file: str, host_alias: str | None, lines: int
) -> None:
    """查看文件末尾内容."""
    from dev_connect.commands.tail import show_tail

    json_output = ctx.obj.get("json_output", False)
    show_tail(_normalize_path(file), host_alias, lines, json_output)


@main.group()
def config() -> None:
    """配置管理."""
    pass


@config.command()
@click.pass_context
def show(ctx: click.Context) -> None:
    """显示当前配置."""
    cfg = load()
    json_output = ctx.obj.get("json_output", False)

    if json_output:
        import json

        click.echo(json.dumps(cfg.model_dump(), indent=2, ensure_ascii=False))
    else:
        click.echo(f"默认主机: {cfg.default_host or '(未设置)'}")
        click.echo("\n已配置主机:")
        for alias, host in cfg.hosts.items():
            click.echo(f"  {alias}: {host.user}@{host.hostname}")


@config.command()
@click.argument("alias")
@click.argument("hostname")
@click.option("--user", "-u", default="maifeng", help="用户名")
@click.option("--default", "-d", "set_default", is_flag=True, help="设为默认主机")
@click.pass_context
def add(
    ctx: click.Context, alias: str, hostname: str, user: str, set_default: bool
) -> None:
    """添加主机配置."""
    cfg = load()
    cfg.hosts[alias] = HostConfig(hostname=hostname, user=user)

    if set_default:
        cfg.default_host = alias

    save(cfg)
    click.echo(f"已添加主机: {alias} ({user}@{hostname})")

    if set_default:
        click.echo("已设为默认主机")


@config.command()
@click.argument("alias")
@click.pass_context
def set_default(ctx: click.Context, alias: str) -> None:
    """设置默认主机."""
    cfg = load()

    if alias not in cfg.hosts:
        click.echo(f"错误: 主机 '{alias}' 未配置", err=True)
        sys.exit(1)

    cfg.default_host = alias
    save(cfg)
    click.echo(f"已设置默认主机: {alias}")


@main.command()
def version() -> None:
    """显示版本信息."""
    click.echo(f"dev {__version__}")


def cli_main() -> None:
    """入口函数，统一捕获异常."""
    try:
        main()
    except Exception as e:
        verbose = "--verbose" in sys.argv
        if verbose:
            traceback.print_exc()
        else:
            click.secho(f"错误: {e}", fg="red", err=True)
            click.secho("提示: 使用 --verbose 查看完整错误栈", fg="yellow", err=True)
        sys.exit(1)
