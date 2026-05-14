# Dev Connect - 执行计划

## 目标

构建轻量 CLI 工具 `dev`，用于本地与远程开发机之间的文件交互，对 Agent 友好。

## 阶段一：项目初始化

### 1.1 创建项目结构

```
dev-connect/
├── docs/
│   ├── design-decisions.md
│   ├── coding-standards.md
│   └── execution-plan.md
├── src/
│   └── dev_connect/
│       ├── __init__.py
│       ├── cli.py
│       ├── common/
│       │   ├── __init__.py
│       │   ├── config.py
│       │   └── ssh.py
│       ├── commands/
│       │   └── __init__.py
│       └── models.py
├── tests/
├── pyproject.toml
├── Makefile
└── .gitignore
```

**验证**：`uv sync` 成功，`dev --version` 输出版本号

### 1.2 配置管理

实现 `common/config.py`：
- 读取 `~/.config/dev-connect/config.yaml`
- Pydantic 模型：AppConfig, HostConfig
- load() / save() 函数

配置文件示例：
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

**验证**：`dev config show` 输出当前配置

### 1.3 SSH 连接管理

实现 `common/ssh.py`：
- 解析 `@sgdev` 格式的主机标识
- 封装 ssh/scp/rsync 调用
- 依赖 ControlMaster 实现连接复用
- 超时控制（默认 30 秒）
- 错误处理和退出码

**验证**：`dev exec "echo ok"` 连接 sgdev 并返回结果

---

## 阶段二：核心命令

### 2.1 ls - 列目录

```bash
dev ls ~/projects              # 默认主机
dev @sgdev ls ~/projects      # 指定主机
dev ls --json ~/projects      # JSON 输出
```

输出格式：
- 默认：树形或列表格式
- JSON：`[{"name": "dir1", "type": "directory"}, ...]`

**验证**：`dev ls ~` 列出远程 home 目录

### 2.2 cat - 查看文件

```bash
dev cat ~/projects/main.go
dev cat --json ~/config.yaml  # JSON 包装
```

**验证**：`dev cat ~/.bashrc` 显示远程文件内容

### 2.3 push - 上传文件

```bash
dev push ./local.txt ~/remote.txt
dev push ./local_dir/ ~/remote_dir/  # 目录
```

**验证**：上传文件后 `dev cat` 确认内容一致

### 2.4 pull - 下载文件

```bash
dev pull ~/remote.txt ./local.txt
dev pull ~/remote_dir/ ./local_dir/  # 目录
```

**验证**：下载文件后本地 `cat` 确认内容一致

### 2.5 exec - 执行命令

```bash
dev exec "make build"
dev exec "ls -la | grep .py"
```

**验证**：`dev exec "uname -a"` 返回远程系统信息

### 2.6 tree - 目录树

```bash
dev tree ~/projects
dev tree --depth 2 ~/projects
```

**验证**：`dev tree ~` 显示目录树结构

---

## 阶段三：完善与优化

### 3.1 错误处理

- SSH 连接失败：明确错误信息，提示检查网络/配置
- 文件不存在：提示路径错误
- 超时：提示增加 --timeout 或检查网络
- 退出码：0 成功，1 一般错误，2 认证错误

### 3.2 输出格式

- 默认：人类可读格式
- `--json`：结构化 JSON，便于 Agent 解析
- `--quiet`：静默模式，只输出必要内容

### 3.3 配置命令

```bash
dev config show                # 显示当前配置
dev config set default_host sgdev  # 设置默认主机
dev config add sgdev 10.251.233.15  # 添加主机
```

### 3.4 测试

- 单元测试：config 解析、主机解析
- 集成测试：SSH 连接、文件传输（需要远程机器）
- Mock 测试：不依赖真实 SSH 的逻辑测试

---

## 阶段四：发布

### 4.1 安装方式

```bash
# 开发模式
uv sync

# 全局安装
uv tool install .

# 或 pip
pip install .
```

### 4.2 文档

- README.md：安装、使用示例
- `dev --help`：内置帮助
- `dev <command> --help`：命令帮助

---

## 当前状态

- [x] 设计决策文档
- [x] 代码规范文档
- [x] 执行计划文档
- [x] 项目初始化
- [x] 配置管理
- [x] SSH 连接管理
- [x] ls 命令
- [x] cat 命令
- [x] push 命令
- [x] pull 命令
- [x] exec 命令
- [x] tree 命令
- [x] 错误处理
- [x] 测试
- [x] 发布

---

## 风险与应对

| 风险 | 影响 | 应对 |
|------|------|------|
| ControlMaster 不稳定 | 连接变慢 | 添加 --no-reuse 选项禁用复用 |
| 大文件传输慢 | 用户体验差 | 支持 rsync 增量传输 |
| SSH 配置多样 | 兼容性问题 | 读取 ~/.ssh/config，优先使用 |
| 网络波动 | 命令失败 | 重试机制，可配置重试次数 |

---

*创建时间：2026-05-14*
*最后更新：2026-05-14*
