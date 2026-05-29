---
name: dev-connect
description: "远程开发机文件交互 CLI。当用户需要查看远程目录、读取远程文件、传输文件、搜索代码、执行命令、查看日志、写入文件、编辑文件、比较文件差异时使用此 skill。"
---

# dev-connect 远程开发机交互

`dev` 是 Go 实现的远程开发机 CLI，封装 SSH/SCP，并默认兼容
`~/.config/dev-connect/config.yaml`。

优先使用 JSON 输出给 Agent 消费：

```bash
dev --json ls ~/project
dev --json cat --cwd ~/project go.mod main.go
dev repo-status --cwd ~/project --json
```

## 文件读取

```bash
dev ls [PATH] [--host HOST]
dev cat PATH... [--host HOST] [--cwd CWD]
dev slice FILE --range START:END [--cwd CWD] [--host HOST]
dev slice FILE --around TEXT [--lines N] [--context N] [--cwd CWD] [--host HOST]
dev head FILE [--lines N] [--host HOST]
dev tail FILE [--lines N] [--host HOST]
```

大文件优先用 `slice/head/tail`，不要直接 `cat` 全文。

## 搜索

```bash
dev grep PATTERN [PATH] [--include GLOB] [--context N] [--max-matches N] [--group] [--host HOST]
dev find NAME [PATH] [--type f|d] [--host HOST]
dev tree [PATH] [--depth N] [--host HOST]
```

`grep` 会优先使用远端 `rg`，不存在时降级 `grep`。

## 执行命令

```bash
dev exec COMMAND [--host HOST] [--cwd CWD] [--timeout N] [--shell SHELL]
dev exec HOST --cwd CWD -- COMMAND
dev exec --cwd ~/repo "go test ./..." --watch --timeout 300
dev exec-watch "go test ./..." --cwd ~/repo --interval 10 --timeout 300
```

`exec --watch` / `exec-watch` 会输出 `started/running/finished` 事件；JSON
模式下每行一个事件对象。

## 写入和编辑

```bash
dev write PATH -c CONTENT [--append] [--host HOST]
echo "content" | dev write PATH [--host HOST]
dev edit replace PATH OLD NEW [--all] [--host HOST]
dev edit insert PATH LINE CONTENT [--after] [--host HOST]
dev edit delete PATH START [END] [--host HOST]
dev edit line PATH NUM CONTENT [--host HOST]
```

编辑命令使用远端结构化脚本处理路径和内容，适合带空格、引号、斜杠的文本。

## Git 仓库

```bash
dev repo-status --cwd REPO [--host HOST] [--json]
dev repo-diff --cwd REPO [--stat] [--cached] [--name-only] [--host HOST]
dev git-snapshot --cwd REPO [--host HOST]
dev repo resolve ORG/REPO [--host HOST]
dev verify go --cwd REPO --changed [--also PKG] [--timeout N] [--host HOST]
```

`verify go --changed` 只验证变更涉及的 Go package，不默认跑 `go test ./...`。

## 远程代码知识图谱

当公司仓库只在远程开发机上时，用 `dev cg` 在远端安装并执行 `codegraph`：

```bash
dev cg install [--host HOST]
dev cg init --cwd REPO --index [--host HOST]
dev cg index --cwd REPO --quiet [--host HOST]
dev --json cg overview --cwd REPO [--host HOST]
dev --json cg context --cwd REPO "task description" --summary [--host HOST]
dev --json cg callers --cwd REPO SymbolName [--host HOST]
dev --json cg affected --repo ORG/REPO FILE [--host HOST]
```

- `--cwd REPO` 会转成远端 `codegraph --path REPO`。
- `--repo ORG/REPO` 会先复用 `dev repo resolve` 找到远程仓库目录。
- 需要 Agent 消费时使用全局 `dev --json cg ...`，不要把 `--json` 放到远端命令末尾。
- 如果远端没有 `codegraph`，先运行 `dev cg install`；默认安装到 `~/.local/bin/codegraph`。

## Patch

```bash
dev patch --cwd REPO [--check] [--host HOST] < changes.patch
dev --json patch --cwd REPO --check < changes.patch
```

Patch 使用 Codex 结构化格式：

```text
*** Begin Patch
*** Add File: src/new.go
+package src
*** Update File: src/app.go
@@
-old
+new
*** Delete File: old.txt
*** End Patch
```

失败时 JSON 会包含 `error`、`path`、`details`，其中 hunk 不匹配会给出相似候选。

## 配置

```bash
dev config show
dev config add ALIAS HOSTNAME [--user USER] [--shell SHELL] [--exec-timeout N] [--repo-root ROOT] [--default]
dev config set-default ALIAS
dev config set-shell ALIAS zsh|zsh-login|bash|bash-login|none
dev config set-exec-timeout ALIAS N
dev config add-repo-root ALIAS ROOT
dev config clear-repo-roots ALIAS
```

## Agent 注意事项

- 对需要解析的结果加 `--json`。
- 对长命令使用 `--watch` 或 `exec-watch`。
- 对未知大文件先 `slice/head/tail`。
- `exec` 后面的 `--json` 可能是远端命令参数，不会被提升为全局 JSON。
