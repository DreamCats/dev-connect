# AGENTS.md — dev-connect 项目指南

## 项目概述

dev-connect 是远程开发机文件交互 CLI，封装 SSH 操作，对 LLM Agent 友好。Python 实现，uv 管理，支持文件传输、目录浏览、命令执行。

## 架构

```
src/dev_connect/
├── cli.py              # 入口，组装子命令，全局选项（--json, --verbose）
├── common/             # 共享基础设施（被所有模块依赖）
│   ├── config.py       # YAML 配置读写，路径 ~/.config/dev-connect/
│   ├── ssh.py          # SSH 连接管理，ControlMaster 复用
│   └── exceptions.py   # 统一异常体系
├── commands/           # 命令模块
│   ├── ls.py           # 列目录
│   ├── cat.py          # 看文件
│   ├── push.py         # 上传
│   ├── pull.py         # 下载
│   ├── exec.py         # 执行命令
│   ├── tree.py         # 目录树
│   ├── grep.py         # 搜索代码（优先 rg，降级 grep）
│   ├── find.py         # 搜索文件
│   ├── tail.py         # 看文件末尾
│   └── agent.py        # 远程 tmux + agent 会话控制
└── models.py           # 共享数据模型（AppConfig, HostConfig）
```

## 依赖方向

```
cli → commands → common
```

- 上层导入下层，下层禁止导入上层
- commands 之间互不依赖
- 共享逻辑下沉到 common

## 命令设计

所有命令支持 `--host` / `-H` 指定主机，`--json` 输出结构化数据。

```bash
dev ls [PATH] [--host HOST] [--json]
dev cat PATH [--host HOST] [--json]
dev push LOCAL REMOTE [--host HOST]
dev pull REMOTE LOCAL [--host HOST]
dev exec CMD [--host HOST] [--timeout T] [--json]
dev tree [PATH] [--host HOST] [--depth D] [--json]
dev grep PATTERN [PATH] [--host HOST] [--include GLOB] [--json]  # 优先 rg，降级 grep
dev find NAME [PATH] [--host HOST] [--type TYPE] [--json]
dev tail FILE [--host HOST] [--lines N] [--json]
dev agent start TASK --cwd REMOTE_DIR [--agent AGENT] [--message MSG] [--prompt-file FILE] [--wait N] [--host HOST] [--json]
dev agent send TASK [MESSAGE] [--wait N] [--lines N] [--chars N] [--compact] [--host HOST] [--json]
dev agent tail TASK [--lines N] [--chars N] [--compact] [--host HOST] [--json]
dev agent interrupt TASK [--host HOST] [--json]
dev agent status TASK [--preview-lines N] [--preview-chars N] [--host HOST] [--json]
dev agent diff TASK [--stat] [--name-only] [--file PATH] [--max-chars N] [--full] [--host HOST] [--json]
dev agent list [--host HOST] [--json]
dev agent stop TASK [--purge] [--host HOST] [--json]
dev config show|add|set-default
```

## Agent 使用模式

```python
import subprocess, json

def run_dev(cmd: str) -> dict:
    result = subprocess.run(f"dev --json {cmd}", shell=True, capture_output=True, text=True)
    return json.loads(result.stdout)

# 列目录
files = run_dev("ls ~/projects")

# 查看文件
content = run_dev("cat ~/.bashrc")

# 搜索代码
matches = run_dev("grep 'def main' ~/projects --include '*.py'")

# 搜索文件
py_files = run_dev("find '*.py' ~/projects")

# 看日志
logs = run_dev("tail ~/logs/app.log -n 100")

# 执行命令
result = run_dev("exec 'uname -a'")
```

## 配置文件

```
~/.config/dev-connect/config.yaml
```

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

## SSH 优化

依赖 ControlMaster 实现连接复用：

```
# ~/.ssh/config
Host *
    ControlMaster auto
    ControlPath ~/.ssh/sockets/%r@%h-%p
    ControlPersist 600
```

效果：第一次 ~2 秒，后续 ~0.26 秒。

## 开发规则

- 格式化：`ruff format`（行宽 88）
- Lint：`ruff check`
- 类型检查：`mypy --strict`
- 提交前：`make lint && make format`
- 注释只写"为什么"，不写"做什么"
- pydantic 模型放 `models.py`，CLI 逻辑放 `cli.py`，命令放 `commands/`

## 添加新命令

1. 创建 `src/dev_connect/commands/<cmd>.py`
2. 实现命令函数
3. 在 `cli.py` 中注册：`@main.command()` + `def <cmd>()`
4. 遵循依赖方向，SSH 操作通过 `common.ssh` 调用

## Skills

LLM Agent 技能文件，位于 `skills/dev-connect/SKILL.md`：

| Skill | 功能 |
|-------|------|
| dev-connect | 统一技能：文件操作、代码搜索、执行调试、配置管理 |

## 技术栈

Python 3.10+ / uv / click / pydantic / pyyaml / ruff / mypy
