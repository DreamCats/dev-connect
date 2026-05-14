"""配置管理测试."""


import pytest

from dev_connect.common.config import get_host, load, save
from dev_connect.common.exceptions import HostNotFoundError
from dev_connect.models import AppConfig, HostConfig


@pytest.fixture
def temp_config_dir(tmp_path):
    """创建临时配置目录."""
    config_dir = tmp_path / ".config" / "dev-connect"
    config_dir.mkdir(parents=True)
    return config_dir


@pytest.fixture
def sample_config():
    """示例配置."""
    return AppConfig(
        default_host="sgdev",
        hosts={
            "sgdev": HostConfig(hostname="10.251.233.15", user="maifeng"),
            "dev": HostConfig(hostname="10.37.122.5", user="maifeng"),
        },
    )


def test_load_empty_config(temp_config_dir, monkeypatch):
    """测试加载空配置."""
    monkeypatch.setattr("dev_connect.common.config.CONFIG_DIR", temp_config_dir)
    monkeypatch.setattr(
        "dev_connect.common.config.CONFIG_FILE", temp_config_dir / "config.yaml"
    )

    config = load()
    assert config.default_host == ""
    assert config.hosts == {}


def test_save_and_load(temp_config_dir, monkeypatch, sample_config):
    """测试保存和加载配置."""
    monkeypatch.setattr("dev_connect.common.config.CONFIG_DIR", temp_config_dir)
    monkeypatch.setattr(
        "dev_connect.common.config.CONFIG_FILE", temp_config_dir / "config.yaml"
    )

    save(sample_config)
    loaded = load()

    assert loaded.default_host == "sgdev"
    assert "sgdev" in loaded.hosts
    assert loaded.hosts["sgdev"].hostname == "10.251.233.15"


def test_get_host_with_alias(sample_config, temp_config_dir, monkeypatch):
    """测试通过别名获取主机."""
    monkeypatch.setattr("dev_connect.common.config.CONFIG_DIR", temp_config_dir)
    monkeypatch.setattr(
        "dev_connect.common.config.CONFIG_FILE", temp_config_dir / "config.yaml"
    )
    save(sample_config)

    host = get_host("sgdev")
    assert host.hostname == "10.251.233.15"


def test_get_host_default(sample_config, temp_config_dir, monkeypatch):
    """测试获取默认主机."""
    monkeypatch.setattr("dev_connect.common.config.CONFIG_DIR", temp_config_dir)
    monkeypatch.setattr(
        "dev_connect.common.config.CONFIG_FILE", temp_config_dir / "config.yaml"
    )
    save(sample_config)

    host = get_host()
    assert host.hostname == "10.251.233.15"


def test_get_host_not_found(sample_config, temp_config_dir, monkeypatch):
    """测试主机未找到."""
    monkeypatch.setattr("dev_connect.common.config.CONFIG_DIR", temp_config_dir)
    monkeypatch.setattr(
        "dev_connect.common.config.CONFIG_FILE", temp_config_dir / "config.yaml"
    )
    save(sample_config)

    with pytest.raises(HostNotFoundError):
        get_host("nonexistent")
