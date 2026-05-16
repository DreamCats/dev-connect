# Dev Connect

远程开发机文件交互 CLI，对 Agent 友好。

## 特性

- **快速连接**：SSH ControlMaster 复用，后续连接 ~0.26 秒
- **Agent 友好**：支持 `--json` 结构化输出，无状态设计
- **多主机管理**：配置文件管理多个远程开发机
- **路径智能**：自动处理 `~` 路径（本地 home → 远程 home）

## 安装

### 一键安装（推荐）

```bash
git clone <repo-url>
cd dev-connect
./install.sh
```

自动检测环境、安装依赖、全局注册 `dev` 命令。

### 手动安装

#### 开发模式

```bash
git clone <repo-url>
cd dev-connect
uv sync
```

#### 全局安装

```bash
# 使用 uv tool
uv tool install .

# 或使用 pip
pip install .
```

## 快速开始

### 1. 配置主机

```bash
# 添加主机
dev config add sgdev 10.251.233.15 --default

# 查看配置
dev config show
```

### 2. 基本操作

```bash
# 列目录
dev ls ~/projects
dev ls --host sgdev ~

# 查看文件
dev cat ~/.bashrc
dev cat -H dev ~/.zshrc

# 上传文件
dev push ./local.txt ~/remote.txt
dev push ./local_dir/ ~/remote_dir/

# 下载文件
dev pull ~/remote.txt ./local.txt
dev pull ~/remote_dir/ ./local_dir/

# 执行命令
dev exec "uname -a"
dev exec "ls -la | head -10"

# 目录树
dev tree ~/projects --depth 2

# 远程 agent 会话
dev agent start task-demo --cwd /home/maifeng/project --agent claude
dev agent send task-demo "先阅读代码，给出改动计划"
dev agent tail task-demo
```

### 3. JSON 输出（Agent 友好）

```bash
# 所有命令支持 --json 输出
dev --json ls ~/projects
dev --json cat ~/.bashrc
dev --json exec "echo hello"

# 示例输出
{
  "command": "echo hello",
  "returncode": 0,
  "stdout": "hello\n",
  "stderr": "",
  "success": true
}
```

## 命令参考

### dev ls

列目录内容。

```bash
dev ls [PATH] [--host HOST] [--json]
```

- `PATH`：目录路径，默认 `~`
- `--host, -H`：主机别名
- `--json`：JSON 格式输出

### dev cat

查看文件内容。

```bash
dev cat PATH [--host HOST] [--json]
```

- `PATH`：文件路径
- `--host, -H`：主机别名
- `--json`：JSON 格式输出

### dev push

上传文件到远程主机。

```bash
dev push LOCAL_PATH REMOTE_PATH [--host HOST]
```

- `LOCAL_PATH`：本地文件路径
- `REMOTE_PATH`：远程文件路径
- `--host, -H`：主机别名

### dev pull

从远程主机下载文件。

```bash
dev pull REMOTE_PATH LOCAL_PATH [--host HOST]
```

- `REMOTE_PATH`：远程文件路径
- `LOCAL_PATH`：本地文件路径
- `--host, -H`：主机别名

### dev exec

执行远程命令。

```bash
dev exec COMMAND [--host HOST] [--timeout TIMEOUT] [--json]
```

- `COMMAND`：要执行的命令
- `--host, -H`：主机别名
- `--timeout, -t`：超时时间（秒），默认 30
- `--json`：JSON 格式输出

### dev tree

显示目录树。

```bash
dev tree [PATH] [--host HOST] [--depth DEPTH] [--json]
```

- `PATH`：目录路径，默认 `~`
- `--host, -H`：主机别名
- `--depth, -d`：目录深度，默认 3
- `--json`：JSON 格式输出

### dev grep

搜索代码内容，优先使用 rg (ripgrep)，如果不存在则降级到 grep。

```bash
dev grep PATTERN [PATH] [--host HOST] [--include PATTERN] [--no-line-number] [--json]
```

- `PATTERN`：搜索模式
- `PATH`：搜索路径，默认当前目录
- `--host, -H`：主机别名
- `--include, -g`：文件名匹配，如 `*.py`
- `--no-line-number, -N`：不显示行号
- `--json`：JSON 格式输出

### dev find

按名称搜索文件。

```bash
dev find NAME [PATH] [--host HOST] [--type TYPE] [--json]
```

- `NAME`：文件名模式
- `PATH`：搜索路径，默认当前目录
- `--host, -H`：主机别名
- `--type, -t`：文件类型，`f`(文件) 或 `d`(目录)
- `--json`：JSON 格式输出

### dev tail

查看文件末尾内容。

```bash
dev tail FILE [--host HOST] [--lines N] [--json]
```

- `FILE`：文件路径
- `--host, -H`：主机别名
- `--lines, -n`：显示行数，默认 20
- `--json`：JSON 格式输出

### dev agent

控制远程 `tmux + Claude Code / Codex` 交互式 agent 会话。

```bash
dev agent start TASK --cwd REMOTE_DIR [--agent AGENT] [--host HOST] [--json]
dev agent send TASK MESSAGE [--host HOST] [--json]
dev agent tail TASK [--lines N] [--host HOST] [--json]
dev agent interrupt TASK [--host HOST] [--json]
dev agent status TASK [--host HOST] [--json]
```

- `TASK`：本次远程会话名称，只能包含字母、数字、下划线、点和短横线
- `--cwd`：远程 agent 启动目录
- `--agent`：agent 类型或启动命令，默认 `claude`
- `claude` 会优先启动远程 `cc`，不存在时降级为 `claude`
- `cc` 会直接启动远程 `cc`，适合复用带权限参数的 Claude alias
- `codex` 会直接启动远程 `codex`
- 状态文件保存在远程 `~/.dev-connect/agents/<TASK>/session.json`
- `--host, -H`：主机别名，不传时使用默认主机
- `--json`：JSON 格式输出

### dev config

配置管理。

```bash
dev config show                          # 显示配置
dev config add ALIAS HOSTNAME [--user USER] [--default]  # 添加主机
dev config set-default ALIAS            # 设置默认主机
```

## 配置文件

**路径**：`~/.config/dev-connect/config.yaml`

```yaml
default_host: sgdev
hosts:
  sgdev:
    hostname: 10.251.233.15
    user: maifeng
  dev:
    hostname: 10.37.122.5
    user: maifeng
```

## SSH 配置

建议开启 ControlMaster 以获得最佳性能：

```
# ~/.ssh/config
Host *
    ControlMaster auto
    ControlPath ~/.ssh/sockets/%r@%h-%p
    ControlPersist 600
```

效果：
- 第一次连接：~2 秒
- 后续连接：~0.26 秒（快 7-8 倍）

## Agent 使用示例

```python
import subprocess
import json

def run_dev(cmd: str) -> dict:
    """执行 dev 命令并返回 JSON 结果."""
    result = subprocess.run(
        f"dev --json {cmd}",
        shell=True,
        capture_output=True,
        text=True,
    )
    return json.loads(result.stdout)

# 列目录
files = run_dev("ls ~/projects")

# 查看文件
content = run_dev("cat ~/.bashrc")

# 执行命令
result = run_dev("exec 'uname -a'")
```

## Skills

LLM Agent 技能文件，位于 `skills/dev-connect/SKILL.md`：

| Skill | 功能 |
|-------|------|
| `dev-connect` | 统一技能：文件操作、代码搜索、执行调试、配置管理 |

## 开发

```bash
# 安装依赖
make install

# 代码检查
make lint

# 格式化
make format

# 运行测试
make test

# 构建
make build
```

## 技术栈

- Python 3.10+
- click：CLI 框架
- pydantic：数据模型
- pyyaml：配置文件
- ruff：代码格式化和 lint
- mypy：类型检查
- pytest：测试框架

## 许可证

MIT
