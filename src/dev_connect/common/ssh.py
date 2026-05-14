"""SSH 连接管理，封装 ssh/scp/rsync 调用."""

from __future__ import annotations

import subprocess
from dataclasses import dataclass

from dev_connect.common.config import get_host
from dev_connect.common.exceptions import CommandError, TimeoutError, TransferError
from dev_connect.models import HostConfig


@dataclass
class SSHResult:
    """SSH 命令执行结果."""

    returncode: int
    stdout: str
    stderr: str

    @property
    def success(self) -> bool:
        return self.returncode == 0


def run_command(
    cmd: str,
    host_alias: str | None = None,
    timeout: int = 30,
) -> SSHResult:
    """执行远程命令."""
    host = get_host(host_alias)
    ssh_cmd = _build_ssh_cmd(host, cmd)

    try:
        result = subprocess.run(
            ssh_cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return SSHResult(
            returncode=result.returncode,
            stdout=result.stdout,
            stderr=result.stderr,
        )
    except subprocess.TimeoutExpired:
        raise TimeoutError(f"命令执行超时（{timeout} 秒）")
    except Exception as e:
        raise CommandError(f"执行失败: {e}")


def upload(
    local_path: str,
    remote_path: str,
    host_alias: str | None = None,
    timeout: int = 60,
) -> SSHResult:
    """上传文件到远程主机."""
    host = get_host(host_alias)
    scp_cmd = _build_scp_cmd(host, local_path, remote_path)

    try:
        result = subprocess.run(
            scp_cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return SSHResult(
            returncode=result.returncode,
            stdout=result.stdout,
            stderr=result.stderr,
        )
    except subprocess.TimeoutExpired:
        raise TimeoutError(f"上传超时（{timeout} 秒）")
    except Exception as e:
        raise TransferError(f"上传失败: {e}")


def download(
    remote_path: str,
    local_path: str,
    host_alias: str | None = None,
    timeout: int = 60,
) -> SSHResult:
    """从远程主机下载文件."""
    host = get_host(host_alias)
    scp_cmd = _build_scp_cmd(host, remote_path, local_path, reverse=True)

    try:
        result = subprocess.run(
            scp_cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return SSHResult(
            returncode=result.returncode,
            stdout=result.stdout,
            stderr=result.stderr,
        )
    except subprocess.TimeoutExpired:
        raise TimeoutError(f"下载超时（{timeout} 秒）")
    except Exception as e:
        raise TransferError(f"下载失败: {e}")


def _build_ssh_cmd(host: HostConfig, cmd: str) -> list[str]:
    """构建 SSH 命令."""
    return [
        "ssh",
        "-o",
        "ControlMaster=auto",
        "-o",
        "ControlPath=~/.ssh/sockets/%r@%h-%p",
        "-o",
        "ControlPersist=600",
        f"{host.user}@{host.hostname}",
        cmd,
    ]


def _build_scp_cmd(
    host: HostConfig,
    source: str,
    dest: str,
    reverse: bool = False,
) -> list[str]:
    """构建 SCP 命令."""
    remote = f"{host.user}@{host.hostname}"

    if reverse:
        # 下载：远程 -> 本地
        return [
            "scp",
            "-o",
            "ControlMaster=auto",
            "-o",
            "ControlPath=~/.ssh/sockets/%r@%h-%p",
            "-o",
            "ControlPersist=600",
            f"{remote}:{source}",
            dest,
        ]
    else:
        # 上传：本地 -> 远程
        return [
            "scp",
            "-o",
            "ControlMaster=auto",
            "-o",
            "ControlPath=~/.ssh/sockets/%r@%h-%p",
            "-o",
            "ControlPersist=600",
            source,
            f"{remote}:{dest}",
        ]
