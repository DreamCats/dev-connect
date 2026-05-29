package cli

import (
	"fmt"
	"strconv"

	"github.com/DreamCats/dev-connect/internal/commands"
)

func cmdCodegraph(args []string, out commands.Output) (int, error) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printCodegraphHelp()
		return 0, nil
	}
	if args[0] == "install" {
		return cmdCodegraphInstall(args[1:], out)
	}
	host, cwd, repo := "", "", ""
	timeout := 300
	cgArgs := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cgArgs = append(cgArgs, args[i+1:]...)
			break
		}
		switch arg {
		case "--host", "-H":
			i++
			if i >= len(args) {
				return 1, fmt.Errorf("%s requires value", arg)
			}
			host = args[i]
		case "--cwd":
			i++
			if i >= len(args) {
				return 1, fmt.Errorf("%s requires value", arg)
			}
			cwd = commands.NormalizePath(args[i])
		case "--repo":
			i++
			if i >= len(args) {
				return 1, fmt.Errorf("%s requires value", arg)
			}
			repo = args[i]
		case "--timeout", "-t":
			i++
			if i >= len(args) {
				return 1, fmt.Errorf("%s requires value", arg)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return 1, err
			}
			timeout = n
		default:
			cgArgs = append(cgArgs, arg)
		}
	}
	if len(cgArgs) == 0 {
		return 1, fmt.Errorf("cg requires COMMAND")
	}
	return commands.CodegraphRun(cgArgs, cwd, repo, host, timeout, out)
}

func cmdCodegraphInstall(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("cg install")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	remote := fs.String("remote", "~/.local/bin/codegraph", "远端安装路径")
	source := fs.String("source", "", "本地 codegraph 二进制路径")
	module := fs.String("module", "github.com/DreamCats/codegraph-cli/cmd/codegraph@latest", "go install 模块")
	timeout := fs.Int("timeout", 120, "超时时间")
	fs.IntVar(timeout, "t", 120, "超时时间")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	return commands.CodegraphInstall(*host, *remote, *source, *module, *timeout, out)
}
