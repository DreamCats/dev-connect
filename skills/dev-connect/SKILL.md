---
name: dev-connect
description: "远程开发机文件交互 CLI。当用户需要查看远程目录、读取远程文件、传输文件、搜索代码、执行命令、查看日志、写入文件、编辑文件、比较文件差异时使用此 skill。"
---

# dev-connect 远程开发机交互

快速连接远程开发机，支持文件传输、目录浏览、命令执行、代码搜索。

## 文件操作

### 列目录

```bash
dev ls [PATH] [--host HOST]
```

### 查看文件

```bash
dev cat PATH... [--host HOST] [--cwd CWD]
```

- 支持一次读取多个文件
- `--cwd` 用于在远程仓库目录下读取相对路径

### 上传文件

```bash
dev push LOCAL_PATH REMOTE_PATH [--host HOST]
```

### 下载文件

```bash
dev pull REMOTE_PATH LOCAL_PATH [--host HOST]
```

### 目录树

```bash
dev tree [PATH] [--host HOST] [--depth N]
```

## 搜索

### 搜索代码内容

```bash
dev grep PATTERN [PATH] [--host HOST] [--include GLOB]
```

- 自动检测 rg (ripgrep)，不存在则降级 grep
- 示例：`dev grep "def main" ~/projects --include "*.py"`

### 按名称搜索文件

```bash
dev find NAME [PATH] [--host HOST] [--type TYPE]
```

- 示例：`dev find "*.py" ~/projects`

## 写入/编辑

### 写入文件

```bash
dev write PATH [--host HOST] [--content CONTENT] [--append]
```

- 写入内容到远程文件，支持覆盖和追加模式
- 可通过 `--content` 参数或 stdin 传入内容
- 示例：
  ```bash
  dev write ~/test.txt -c "hello world"
  dev write ~/log.txt -c "new log entry" --append
  echo "content" | dev write ~/test.txt
  ```

### 精确编辑

```bash
# 替换内容
dev edit replace PATH OLD NEW [--host HOST] [--all]

# 在指定行插入
dev edit insert PATH LINE CONTENT [--host HOST] [--after]

# 删除指定行
dev edit delete PATH START [END] [--host HOST]

# 修改指定行
dev edit line PATH NUM CONTENT [--host HOST]
```

- 示例：
  ```bash
  dev edit replace ~/config.py "old_value" "new_value"
  dev edit replace ~/config.py "foo" "bar" --all  # 替换所有
  dev edit insert ~/test.py 10 "new line"  # 在第 10 行前插入
  dev edit insert ~/test.py 10 "after line" --after  # 在第 10 行后插入
  dev edit delete ~/test.py 5  # 删除第 5 行
  dev edit delete ~/test.py 10 20  # 删除第 10-20 行
  dev edit line ~/test.py 5 "new content"  # 修改第 5 行
  ```

## 比较

### 比较文件差异

```bash
dev diff FILE1 FILE2 [--host HOST] [--local]
```

- 比较两个远程文件的差异
- 使用 `--local` 比较远程文件和本地文件
- 示例：
  ```bash
  dev diff ~/old.py ~/new.py                # 比较两个远程文件
  dev diff ~/remote.py ./local.py --local   # 比较远程和本地文件
  dev --json diff ~/old.py ~/new.py         # JSON 输出
  ```

### 应用远程 patch

```bash
dev patch --cwd REPO [--host HOST] [--check] < changes.patch
```

- 远端执行 `git apply --check`，通过后再 `git apply`
- 失败时返回完整 stdout/stderr，不应用半截 patch
- `--check` 只校验不应用

### 仓库状态快照

```bash
dev repo-status --cwd REPO [--host HOST]
```

- 一次返回 branch、upstream、dirty、status、diff stat、最近 commits

## 执行调试

### 查看文件开头

```bash
dev head FILE [--host HOST] [--lines N]
```

- 查看文件开头内容，默认 20 行
- 示例：`dev head ~/config.py -n 50`

### 执行远程命令

```bash
dev exec COMMAND [--host HOST] [--timeout TIMEOUT]
```

- 示例：`dev exec "ps aux | grep python"`
- `--timeout` 未指定时使用主机配置 `exec_timeout`，再降级到 30 秒

### 查看日志末尾

```bash
dev tail FILE [--host HOST] [--lines N]
```

- 示例：`dev tail ~/logs/app.log -n 100`

## 远程 agent 会话

本地 Codex App 作为 supervisor 控制远程 Claude Code / Codex 时，先阅读
`references/agent-supervisor-workflow.md`。

### 启动会话

```bash
dev agent start TASK --cwd REMOTE_DIR [--agent AGENT] [--message MSG] [--prompt-file FILE] [--wait N] [--host HOST]
```

- 底层使用远程 `tmux` 启动交互式 agent
- `--agent claude` 会优先启动远程 `cc`，不存在时降级为 `claude`
- `--agent cc` 会直接启动远程 `cc`，适合复用带权限参数的 Claude alias
- `--agent codex` 会启动远程 `codex`
- 状态记录在远程 `~/.dev-connect/agents/<TASK>/session.json`

### 发送指令

```bash
dev agent send TASK [MESSAGE] [--wait N] [--lines N] [--chars N] [--compact] [--host HOST]
```

不传 `MESSAGE` 时从 stdin 读取；stdin 为空时只发送 Enter。

### 读取输出

```bash
dev agent tail TASK [--lines N] [--chars N] [--compact] [--host HOST]
```

### 打断会话

```bash
dev agent interrupt TASK [--host HOST]
```

### 查看状态

```bash
dev agent status TASK [--preview-lines N] [--preview-chars N] [--host HOST]
```

### 查看 diff

```bash
dev agent diff TASK [--stat] [--name-only] [--file PATH] [--max-chars N] [--full] [--host HOST]
```

基于状态文件中的 `cwd` 执行远程 `git diff`，默认限制输出字符数。

### 列出会话

```bash
dev agent list [--host HOST]
```

### 停止会话

```bash
dev agent stop TASK [--purge] [--host HOST]
```

默认只停止 tmux session；`--purge` 会删除远程状态目录。

## 配置管理

```bash
dev config show                                    # 查看配置
dev config add ALIAS HOSTNAME [--user USER] [--shell SHELL] [--exec-timeout N] [--default]
dev config set-default ALIAS                       # 设置默认主机
dev config set-shell ALIAS zsh                     # 设置 dev exec 默认 shell
dev config set-exec-timeout ALIAS 120              # 设置 dev exec 默认超时
```

## 通用选项

- `--host, -H`：指定主机别名
- `--json`：JSON 格式输出（便于 Agent 解析）

## 配置文件

路径：`~/.config/dev-connect/config.yaml`

```yaml
default_host: sgdev
hosts:
  sgdev:
    hostname: 10.251.233.15
    user: maifeng
```

## Agent 使用示例

```python
import subprocess, json

def run_dev(cmd: str) -> dict:
    result = subprocess.run(f"dev --json {cmd}", shell=True, capture_output=True, text=True)
    return json.loads(result.stdout)

# 列目录
files = run_dev("ls ~/projects")

# 搜索代码
matches = run_dev("grep 'def main' ~/projects --include '*.py'")

# 写入文件
subprocess.run("dev write ~/test.txt -c 'hello'", shell=True)

# 替换内容
subprocess.run('dev edit replace ~/config.py "old" "new"', shell=True)

# 删除行
subprocess.run("dev edit delete ~/test.py 10 20", shell=True)

# 执行命令
result = run_dev("exec 'uname -a'")
```
