"""配置管理：读写 ~/.config/dev-connect/config.yaml."""

from __future__ import annotations

from pathlib import Path

import yaml

from dev_connect.common.exceptions import HostNotFoundError
from dev_connect.models import AppConfig, HostConfig

CONFIG_DIR = Path.home() / ".config" / "dev-connect"
CONFIG_FILE = CONFIG_DIR / "config.yaml"


def load() -> AppConfig:
    """加载配置，文件不存在时返回空配置."""
    if not CONFIG_FILE.exists():
        return AppConfig()
    data = yaml.safe_load(CONFIG_FILE.read_text()) or {}
    return AppConfig.model_validate(data)


def save(config: AppConfig) -> None:
    """写入配置文件."""
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    CONFIG_FILE.write_text(yaml.dump(config.model_dump(), allow_unicode=True))


def get_host(host_alias: str | None = None) -> HostConfig:
    """获取主机配置，支持 @alias 格式."""
    config = load()

    # 解析 @alias 格式
    if host_alias and host_alias.startswith("@"):
        host_alias = host_alias[1:]

    # 确定主机名
    if host_alias:
        alias = host_alias
    elif config.default_host:
        alias = config.default_host
    else:
        raise HostNotFoundError(
            "未指定主机且未配置默认主机，请使用 @alias 或设置 default_host"
        )

    # 查找主机配置
    if alias not in config.hosts:
        raise HostNotFoundError(
            f"主机 '{alias}' 未在配置中找到，可用主机: {list(config.hosts.keys())}"
        )

    return config.hosts[alias]
