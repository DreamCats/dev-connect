# Dev Connect

Go implementation of the `dev` remote development CLI.

`dev` wraps SSH/SCP workflows for agent-friendly remote file access, code search,
command execution, repository inspection, and precise remote edits. It keeps the
same config path and command surface as the Python implementation:
`~/.config/dev-connect/config.yaml`.

## Install

```bash
go install github.com/DreamCats/dev-connect/cmd/dev@latest
```

From source:

```bash
git clone git@github.com:DreamCats/dev-connect.git
cd dev-connect
./install.sh
```

Development build:

```bash
go build -o dev ./cmd/dev
./dev --help
```

Install with Go:

```bash
go install ./cmd/dev
```

## Configuration

```bash
dev config add sgdev <HOSTNAME_OR_IP> --default
dev config add sgdev <HOSTNAME_OR_IP> --repo-root /path/to/code.byted.org
dev config show
dev --json config show
```

Config file:

```yaml
default_host: sgdev
hosts:
  sgdev:
    hostname: <HOSTNAME_OR_IP>
    user: <USER>
    shell: null
    exec_timeout: null
    repo_roots: []
```

## Common Commands

```bash
dev ls ~/projects
dev cat --cwd ~/project pyproject.toml src/app.py
dev slice src/app.py --cwd ~/project --range 120:180
dev grep "func main" ~/project --include "*.go" --context 2
dev find "*.go" ~/project
dev head ~/file.log -n 40
dev tail ~/file.log -n 100
```

```bash
dev exec "uname -a"
dev exec sgdev --cwd ~/repo -- go test ./...
dev exec --cwd ~/repo "go test ./..." --watch --timeout 300
dev exec-watch "go test ./..." --cwd ~/repo --interval 10 --timeout 300
```

```bash
dev repo-status --cwd ~/repo
dev repo-diff --cwd ~/repo --stat
dev git-snapshot --cwd ~/repo
dev repo resolve ttec/project
dev verify go --cwd ~/repo --changed
```

Remote codegraph:

```bash
dev cg install
dev cg init --cwd ~/repo --index
dev cg index --cwd ~/repo --quiet
dev --json cg overview --cwd ~/repo
dev --json cg context --cwd ~/repo "fix login bug" --summary
dev --json cg callers --repo ttec/project SomeFunc
```

`dev cg install` installs `codegraph` on the remote host. It first detects the
remote OS/arch. If the local `codegraph` binary is compatible, it uploads that
binary; otherwise it runs local `go install` for the remote target and uploads
the generated binary to `~/.local/bin/codegraph`.

```bash
dev write ~/test.txt -c "hello"
echo "hello" | dev write ~/test.txt
dev edit replace ~/test.txt "old" "new" --all
dev edit insert ~/test.txt 3 "new line" --after
dev edit delete ~/test.txt 10 20
dev edit line ~/test.txt 5 "replacement"
```

## Structured Patch

`dev patch` applies Codex-style structured patches inside a remote repository.
It does not call `git apply`; it uses a bundled applier that matches old context
uniquely and reports hunk diagnostics on failure.

```bash
dev patch --cwd ~/repo < changes.patch
dev patch --cwd ~/repo --check < changes.patch
dev --json patch --cwd ~/repo --check < changes.patch
```

Patch format:

```text
*** Begin Patch
*** Add File: src/new.py
+value = 1
*** Update File: src/app.py
@@
 def main():
-    return 1
+    return 2
*** Delete File: old.txt
*** End Patch
```

## JSON Output

All core commands support global `--json`, including post-command placement for
non-`exec` commands:

```bash
dev --json ls ~/projects
dev repo-status --cwd ~/repo --json
```

`exec` intentionally does not lift post-command `--json`, because it may be part
of the remote command.

## Development

```bash
go test ./...
go build ./...
gofmt -w .
```
