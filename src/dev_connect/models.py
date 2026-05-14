"""共享数据模型."""

from __future__ import annotations

from pydantic import BaseModel, Field


class HostConfig(BaseModel):
    """远程主机配置."""

    hostname: str
    user: str = "maifeng"


class AppConfig(BaseModel):
    """应用配置."""

    default_host: str = ""
    hosts: dict[str, HostConfig] = Field(default_factory=dict)
