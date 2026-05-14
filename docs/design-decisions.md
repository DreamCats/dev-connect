# Dev Connect CLI - 设计决策记录

## 背景

**问题**：经常使用远程开发机（sgdev: 10.251.233.15），需要在本地和远程之间传输文件、查看远程目录和文件内容。不想每次都打开 VS Code + Remote 插件。

**需求**：一个轻量 CLI 工具，随时随地与远程开发机交互文件。

## SSH 连接优化

### 问题

每次 SSH 连接需要 ~2 秒（完整握手），如果 CLI 每次操作都要重新连接，体验很差。

### 解决方案：ControlMaster（连接复用）

在 `~/.ssh/config` 中添加：

```
Host *
    ControlMaster auto
    ControlPath ~/.ssh/sockets/%r@%h-%p
    ControlPersist 600
```

**效果**：
- 第一次连接：~2 秒（建立 master）
- 后续连接：~0.26 秒（复用 socket，快 7-8 倍）
- 连接保持：600 秒（10 分钟内不需重新握手）

**状态**：已配置并验证有效。

## 设计决策：无状态 CLI

### 方案对比

| 方案 | 优点 | 缺点 | Agent 友好度 |
|------|------|------|-------------|
| 交互式会话 | 理论上更快 | 状态管理复杂，输出解析困难 | ❌ 差 |
| 无状态 CLI + ControlMaster | 简单可靠，输出可预测 | 每次调用独立进程 | ✅ 好 |

### 决策：采用无状态 CLI

**理由**：

1. **Agent 使用模式是"调用→结果→决策→再调用"**，每次调用独立
2. **交互式会话对 Agent 不友好**：
   - 需要维护 stdin/stdout 管道状态
   - 输出解析困难（区分命令结果和 shell prompt）
   - 错误恢复复杂
   - 不支持并发操作
3. **ControlMaster 解决了性能问题**：虽然每次都是独立 SSH 命令，但复用 socket 后只需 ~0.26 秒

### Agent 友好设计原则

1. **无状态**：每次调用独立，不维护连接
2. **可预测**：输出格式固定，支持 `--json` 格式化输出
3. **快速**：ControlMaster 保证每次调用 ~0.26 秒
4. **可靠**：明确的退出码（0 成功，非 0 失败），可配置超时

## CLI 命令设计

```bash
dev ls <path>              # 列目录内容
dev cat <path>             # 查看文件内容
dev push <local> <remote>  # 上传文件
dev pull <remote> <local>  # 下载文件
dev exec <command>         # 执行远程命令
dev tree <path>            # 显示目录树
```

每个命令特性：
- 独立进程执行
- 支持 `--json` 输出（便于 Agent 解析）
- 明确的退出码
- 可配置超时（默认 30 秒）

## 主机选择设计

### 问题

用户可能有多个远程开发机（sgdev、dev 等），Agent 需要知道操作哪台。

### 方案：@ 语法 + 默认主机

```bash
dev ls ~/projects              # 默认主机（配置文件指定）
dev @sgdev ls ~/projects      # 指定 sgdev
dev @dev ls ~/projects        # 指定 dev
```

**优点**：
- 简洁：`@sgdev` 比 `--host sgdev` 短
- 直观：一看就知道是"哪台机器"
- Agent 友好：用户说"看下 sgdev 的 projects" → `dev @sgdev ls ~/projects`

### 配置文件

**路径**：`~/.config/dev-connect/config.yaml`（符合 XDG 规范）

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

## 技术选型

- **语言**：Python
- **CLI 框架**：Typer
- **SSH 封装**：subprocess 调用 ssh/scp/rsync
- **配置**：`~/.config/dev-connect/config.yaml`，支持多主机
- **分发**：单文件脚本或 pip 包

## 后续扩展（暂不实现）

- 文件同步（watch + rsync）
- 批量操作
- 端口转发
- 多主机管理

---

*记录时间：2026-05-14*
*决策依据：Agent 友好性 > 交互式便利性*
