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
dev cat PATH [--host HOST]
```

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

### 查看日志末尾

```bash
dev tail FILE [--host HOST] [--lines N]
```

- 示例：`dev tail ~/logs/app.log -n 100`

## 配置管理

```bash
dev config show                                    # 查看配置
dev config add ALIAS HOSTNAME [--user USER] [--default]  # 添加主机
dev config set-default ALIAS                       # 设置默认主机
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
