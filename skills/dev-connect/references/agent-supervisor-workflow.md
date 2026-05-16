# dev-connect agent supervisor workflow

本文档描述本地 Codex App 通过 `dev agent` 控制远程交互式 agent 的推荐流程。

## 定位

`dev agent` 是薄基础设施层，不是智能调度器。

- 本地 Codex App / local agent 是 supervisor：负责派活、观察、纠偏、验收。
- 远程 `tmux + Claude Code / Codex` 是 worker：负责在真实远程仓库中读代码、改代码、跑验证。
- 不把自动规划、多 agent 调度、自动摘要、自动提交等复杂逻辑放进 `dev-connect`。

## 推荐节奏

优先小步交互，不要一次性把复杂任务完全交给远程 agent。

```bash
dev agent start TASK --cwd REMOTE_REPO --agent claude
dev agent tail TASK --compact --chars 8000
dev agent send TASK "先不要改代码，阅读相关文件后给出改动计划" --wait 5 --compact
dev agent diff TASK --stat
dev agent status TASK
```

当远程 agent 跑偏、卡住、进入确认提示时：

```bash
dev agent interrupt TASK
dev agent send TASK "暂停。先解释当前状态，不要继续改文件。"
```

结束会话：

```bash
dev agent stop TASK
```

确认不再需要状态记录时：

```bash
dev agent stop TASK --purge
```

## 启动 agent

常用启动：

```bash
dev agent start TASK --cwd /home/maifeng/project --agent claude
```

`--agent` 语义：

- `claude`：优先启动远程 `cc`，不存在时降级为 `claude`。
- `cc`：强制启动远程 `cc`，适合复用带权限参数的 Claude alias。
- `codex`：启动远程 `codex`。
- 其他字符串：作为远程启动命令使用。

启动并发送首条消息：

```bash
dev agent start TASK --cwd /home/maifeng/project --agent claude --message "先读 README，不要改文件"
```

长 prompt 使用文件：

```bash
dev agent start TASK --cwd /home/maifeng/project --prompt-file brief.md --wait 5 --lines 120
```

## 处理 Claude trust prompt

Claude Code 首次进入一个目录时可能出现 trust prompt。此时先看输出：

```bash
dev agent tail TASK --compact --chars 8000
```

若确认目录可信，用空消息发送 Enter：

```bash
dev agent send TASK ""
```

之后再发送实际任务。

## 发送消息

短消息：

```bash
dev agent send TASK "继续，但只改最小范围"
```

长消息用 stdin：

```bash
cat brief.md | dev agent send TASK
```

发送后等待并回读：

```bash
dev agent send TASK "执行下一步" --wait 8 --lines 160 --compact --chars 12000
```

`--wait` 适合短轮询。复杂任务仍建议按 `send -> tail -> 判断 -> send` 的节奏推进。

## 观察输出

普通读取：

```bash
dev agent tail TASK --lines 120
```

降低 tmux UI 噪声：

```bash
dev agent tail TASK --lines 160 --compact --chars 12000
```

`--compact` 只去掉空行，不做智能摘要。语义判断仍由本地 Codex App 完成。

## 独立验收

不要完全相信远程 agent 自述。用 `status` 和 `diff` 独立观察远程仓库状态。

```bash
dev agent status TASK --preview-lines 30 --preview-chars 4000
dev agent diff TASK --stat
dev agent diff TASK --name-only
dev agent diff TASK --file path/to/file.go
```

普通 diff 默认限制输出字符数，避免刷屏：

```bash
dev agent diff TASK --max-chars 20000
```

需要完整 diff 时显式请求：

```bash
dev agent diff TASK --full
```

验证命令建议让远程 agent 自己执行并贴结果：

```bash
dev agent send TASK "请运行必要的验证命令，并贴出结果摘要"
```

`dev-connect` 不提供专门的 `test` 命令，避免把任务语义塞进基础设施层。

## 状态目录

每个 task 在远程有一个状态目录：

```text
~/.dev-connect/agents/<TASK>/session.json
```

它记录 `<TASK>` 到远程 tmux session、cwd、agent 类型、最后发送消息等映射。这样后续命令只需要传 `TASK`：

```bash
dev agent send TASK "继续"
dev agent tail TASK
dev agent diff TASK --stat
```

`stop` 默认只停止 tmux session，保留状态目录用于复盘；`stop --purge` 会删除状态目录。

## 边界

- 不自动拆分多 agent。
- 不自动判断远程 agent 是否跑偏。
- 不自动总结长 transcript。
- 不自动提交、push、创建 PR/MR。
- 不默认执行验证命令；由本地 supervisor 明确发消息要求远程 agent 执行。
