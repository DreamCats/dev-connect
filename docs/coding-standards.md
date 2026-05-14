# Dev Connect - 代码规范

参考 byte-helper 项目风格。

## 技术栈

- **Python**: 3.10+
- **包管理**: uv
- **CLI 框架**: click
- **数据模型**: pydantic
- **配置文件**: pyyaml
- **格式化**: ruff (行宽 88)
- **类型检查**: mypy --strict
- **测试**: pytest

## 项目结构

```
src/dev_connect/
├── __init__.py
├── cli.py              # 入口，组装子命令，全局选项（--json, --verbose）
├── common/             # 共享基础设施
│   ├── __init__.py
│   ├── config.py       # YAML 配置读写，路径 ~/.config/dev-connect/
│   └── ssh.py          # SSH 连接管理，ControlMaster 复用
├── commands/           # 命令模块
│   ├── __init__.py
│   ├── ls.py           # 列目录
│   ├── cat.py          # 看文件
│   ├── push.py         # 上传
│   ├── pull.py         # 下载
│   ├── exec.py         # 执行命令
│   └── tree.py         # 目录树
└── models.py           # 共享数据模型
```

## 依赖方向

```
cli → commands → common
```

- 上层导入下层，下层禁止导入上层
- commands 之间互不依赖
- 共享逻辑下沉到 common

## 代码风格

### 类型注解

```python
def get_file(host: str, remote_path: str) -> bytes: ...
def list_dir(host: str, path: str) -> list[dict[str, str]]: ...
```

### Docstring

中文，简洁，只写"为什么"，不写"做什么"：

```python
def parse_host(host_str: str) -> HostConfig:
    """解析 @sgdev 格式的主机标识."""
    ...
```

### Pydantic 模型

```python
from pydantic import BaseModel, Field

class HostConfig(BaseModel):
    hostname: str
    user: str = "maifeng"

class AppConfig(BaseModel):
    default_host: str = ""
    hosts: dict[str, HostConfig] = Field(default_factory=dict)
```

### CLI 命令

```python
import click

@click.command()
@click.argument("path")
@click.option("--json", "json_output", is_flag=True, help="JSON 格式输出")
@click.pass_context
def ls(ctx: click.Context, path: str, json_output: bool) -> None:
    """列目录内容."""
    ...
```

## 配置管理

**路径**: `~/.config/dev-connect/config.yaml`

```python
from pathlib import Path
import yaml
from pydantic import BaseModel

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
```

## Makefile

```makefile
.PHONY: install lint format test build clean

install:
	uv sync

lint:
	uv run ruff check src/ tests/
	uv run mypy src/

format:
	uv run ruff format src/ tests/
	uv run ruff check --fix src/ tests/

test:
	uv run pytest tests/

build:
	uv build

clean:
	rm -rf dist/ build/ *.egg-info
	find . -type d -name __pycache__ -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete
```

## pyproject.toml

```toml
[project]
name = "dev-connect"
version = "0.1.0"
description = "远程开发机文件交互 CLI"
requires-python = ">=3.10"
dependencies = [
    "click>=8.1",
    "pydantic>=2.0",
    "pyyaml>=6.0",
]

[project.optional-dependencies]
dev = [
    "ruff>=0.4",
    "mypy>=1.10",
    "pytest>=8.0",
]

[project.scripts]
dev = "dev_connect.cli:cli_main"

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["src/dev_connect"]

[tool.ruff]
line-length = 88
target-version = "py310"

[tool.ruff.lint]
select = ["E", "F", "I", "UP"]

[tool.mypy]
python_version = "3.10"
strict = true
```

---

*参考项目: byte-helper (/Users/bytedance/Work/tools/bytedance/byte-helper)*
