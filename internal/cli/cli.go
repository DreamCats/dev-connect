package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/DreamCats/dev-connect/internal/commands"
	"github.com/DreamCats/dev-connect/internal/config"
	"github.com/DreamCats/dev-connect/internal/model"
	"github.com/DreamCats/dev-connect/internal/stats"
)

var allCommands = map[string]bool{
	"ls": true, "cat": true, "push": true, "pull": true, "exec": true, "exec-watch": true,
	"patch": true, "repo-status": true, "git-snapshot": true, "repo-diff": true, "tree": true,
	"grep": true, "slice": true, "find": true, "head": true, "tail": true, "write": true,
	"diff": true, "repo": true, "verify": true, "edit": true, "config": true, "stats": true,
	"version": true,
}

func Run(args []string) error {
	cfg, rest, err := parseGlobal(normalizeGlobalFlags(args))
	if err != nil {
		return err
	}
	if len(rest) == 0 || rest[0] == "-h" || rest[0] == "--help" || rest[0] == "help" {
		printTopHelp()
		return nil
	}
	cmd := rest[0]
	code, err := runCommand(cfg, cmd, rest[1:])
	if tracked := extractCommand(rest); tracked != "" {
		stats.Record(tracked)
	}
	if err != nil {
		if cfg.Verbose {
			panic(err)
		}
		if code == 0 {
			code = 1
		}
		return exitCode(code, err)
	}
	if code != 0 {
		return exitCode(code, fmt.Errorf("command failed"))
	}
	return nil
}

func Main(args []string) {
	if err := Run(args); err != nil {
		if e, ok := err.(exitError); ok {
			if e.err != nil && e.err.Error() != "command failed" {
				fmt.Fprintf(os.Stderr, "错误: %v\n", e.err)
			}
			os.Exit(e.code)
		}
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func parseGlobal(args []string) (appConfig, []string, error) {
	cfg := appConfig{}
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			cfg.JSON = true
		case "--verbose":
			cfg.Verbose = true
		case "--version":
			rest = append(rest, "version")
			return cfg, rest, nil
		default:
			rest = append(rest, args[i:]...)
			return cfg, rest, nil
		}
	}
	return cfg, rest, nil
}

func normalizeGlobalFlags(args []string) []string {
	if len(args) <= 1 || firstCommand(args) == "exec" || firstCommand(args) == "exec-watch" {
		return args
	}
	pulled := []string{}
	rest := []string{}
	afterDoubleDash := false
	for _, arg := range args {
		if arg == "--" {
			afterDoubleDash = true
			rest = append(rest, arg)
			continue
		}
		if !afterDoubleDash && (arg == "--json" || arg == "--verbose") {
			if !contains(pulled, arg) {
				pulled = append(pulled, arg)
			}
			continue
		}
		rest = append(rest, arg)
	}
	return append(pulled, rest...)
}

func firstCommand(args []string) string {
	skipNext := false
	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == "--json" || arg == "--verbose" {
			continue
		}
		if arg == "--host" || arg == "-H" || arg == "-h" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if allCommands[arg] {
			return arg
		}
		return ""
	}
	return ""
}

func runCommand(cfg appConfig, cmd string, args []string) (int, error) {
	out := commands.NewOutput(cfg.JSON)
	switch cmd {
	case "ls":
		return cmdLS(args, out)
	case "cat":
		return cmdCat(args, out)
	case "push":
		return cmdPush(args, out)
	case "pull":
		return cmdPull(args, out)
	case "exec":
		return cmdExec(args, out)
	case "exec-watch":
		return cmdExecWatch(args, out)
	case "patch":
		return cmdPatch(args, out)
	case "repo-status":
		return cmdRepoStatus(args, out)
	case "git-snapshot":
		return cmdGitSnapshot(args, out)
	case "repo-diff":
		return cmdRepoDiff(args, out)
	case "tree":
		return cmdTree(args, out)
	case "grep":
		return cmdGrep(args, out)
	case "slice":
		return cmdSlice(args, out)
	case "find":
		return cmdFind(args, out)
	case "head":
		return cmdHeadTail("head", args, out)
	case "tail":
		return cmdHeadTail("tail", args, out)
	case "write":
		return cmdWrite(args, out)
	case "diff":
		return cmdDiff(args, out)
	case "repo":
		return cmdRepo(args, out)
	case "verify":
		return cmdVerify(args, out)
	case "edit":
		return cmdEdit(args, out)
	case "config":
		return cmdConfig(args, out)
	case "stats":
		return commands.Stats(out)
	case "version":
		fmt.Printf("dev %s\n", model.Version)
		return 0, nil
	default:
		return 1, fmt.Errorf("unknown command: %s", cmd)
	}
}

func cmdLS(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("ls")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H", "h")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	path := "~"
	if fs.NArg() > 0 {
		path = commands.NormalizePath(fs.Arg(0))
	}
	return commands.ListDir(path, *host, out)
}

func cmdCat(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("cat")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H", "h")
	cwd := fs.String("cwd", "", "远程工作目录")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() == 0 {
		return 1, fmt.Errorf("cat requires PATH")
	}
	paths := make([]string, fs.NArg())
	for i := range paths {
		paths[i] = commands.NormalizePath(fs.Arg(i))
	}
	normCwd := ""
	if *cwd != "" {
		normCwd = commands.NormalizePath(*cwd)
	}
	return commands.Cat(paths, *host, normCwd, out)
}

func cmdPush(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("push")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H", "h")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 2 {
		return 1, fmt.Errorf("push requires LOCAL REMOTE")
	}
	return commands.Push(fs.Arg(0), commands.NormalizePath(fs.Arg(1)), *host, out)
}

func cmdPull(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("pull")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H", "h")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 2 {
		return 1, fmt.Errorf("pull requires REMOTE LOCAL")
	}
	return commands.Pull(commands.NormalizePath(fs.Arg(0)), fs.Arg(1), *host, out)
}

func cmdExec(args []string, out commands.Output) (int, error) {
	if len(args) == 1 && args[0] == "--help" {
		printExecHelp()
		return 0, nil
	}
	host, cwd, shell := "", "", ""
	interval, summaryChars := 10, 20000
	var timeout *int
	watch := false
	cmdParts := []string{}
	workArgs := args
	if len(workArgs) >= 2 && looksLikeExecHost(workArgs[0]) {
		host = workArgs[0]
		workArgs = workArgs[1:]
	}
	for i := 0; i < len(workArgs); i++ {
		arg := workArgs[i]
		if arg == "--" {
			cmdParts = append(cmdParts, workArgs[i+1:]...)
			break
		}
		switch arg {
		case "--host", "-H", "-h":
			i++
			if i >= len(workArgs) {
				return 1, fmt.Errorf("%s requires value", arg)
			}
			host = workArgs[i]
		case "--cwd":
			i++
			cwd = commands.NormalizePath(workArgs[i])
		case "--timeout", "-t":
			i++
			n, err := strconv.Atoi(workArgs[i])
			if err != nil {
				return 1, err
			}
			timeout = &n
		case "--watch", "--wait":
			watch = true
		case "--interval":
			i++
			interval, _ = strconv.Atoi(workArgs[i])
		case "--summary-chars":
			i++
			summaryChars, _ = strconv.Atoi(workArgs[i])
		case "--shell":
			i++
			shell = workArgs[i]
		default:
			cmdParts = append(cmdParts, workArgs[i:]...)
			i = len(workArgs)
		}
	}
	if len(cmdParts) == 0 {
		return 1, fmt.Errorf("缺少 COMMAND。示例: dev exec --host sgdev --cwd ~/repo 'go test ./...'")
	}
	if host == "" && len(cmdParts) >= 2 && looksLikeExecHost(cmdParts[0]) {
		host = cmdParts[0]
		cmdParts = cmdParts[1:]
	}
	command := cmdParts[0]
	if len(cmdParts) > 1 {
		command = shellJoin(cmdParts)
	}
	if watch {
		t := 300
		if timeout != nil {
			t = *timeout
		}
		return commands.ExecWatch(command, host, interval, t, shell, summaryChars, cwd, out)
	}
	return commands.Exec(command, host, timeout, shell, cwd, out)
}

func cmdExecWatch(args []string, out commands.Output) (int, error) {
	if len(args) == 1 && args[0] == "--help" {
		printExecWatchHelp()
		return 0, nil
	}
	fs := newFlagSet("exec-watch")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H", "h")
	cwd := fs.String("cwd", "", "远程工作目录")
	interval := fs.Int("interval", 10, "状态输出间隔")
	timeout := fs.Int("timeout", 300, "总超时时间")
	fs.IntVar(timeout, "t", 300, "总超时时间")
	summaryChars := fs.Int("summary-chars", 20000, "结束摘要最大字符数")
	shell := fs.String("shell", "", "远端 shell 包裹")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 1 {
		return 1, fmt.Errorf("exec-watch requires CMD")
	}
	normCwd := ""
	if *cwd != "" {
		normCwd = commands.NormalizePath(*cwd)
	}
	return commands.ExecWatch(fs.Arg(0), *host, *interval, *timeout, *shell, *summaryChars, normCwd, out)
}

func cmdPatch(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("patch")
	cwd := fs.String("cwd", "", "远程 Git 仓库目录")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	check := fs.Bool("check", false, "只校验")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if *cwd == "" {
		return 1, fmt.Errorf("--cwd is required")
	}
	return commands.Patch(commands.NormalizePath(*cwd), *host, *check, out)
}

func cmdRepoStatus(args []string, out commands.Output) (int, error) {
	cwd, host, err := parseCwdHost("repo-status", args)
	if err != nil {
		return 1, err
	}
	return commands.RepoStatus(cwd, host, out)
}

func cmdGitSnapshot(args []string, out commands.Output) (int, error) {
	cwd, host, err := parseCwdHost("git-snapshot", args)
	if err != nil {
		return 1, err
	}
	return commands.GitSnapshot(cwd, host, out)
}

func cmdRepoDiff(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("repo-diff")
	cwd := fs.String("cwd", "", "远程 Git 仓库目录")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	stat := fs.Bool("stat", false, "只输出 diff stat")
	cached := fs.Bool("cached", false, "查看暂存区 diff")
	nameOnly := fs.Bool("name-only", false, "只输出变更文件名")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if *cwd == "" {
		return 1, fmt.Errorf("--cwd is required")
	}
	return commands.RepoDiff(commands.NormalizePath(*cwd), *host, *stat, *cached, *nameOnly, out)
}

func cmdTree(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("tree")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H", "h")
	depth := fs.Int("depth", 3, "目录深度")
	fs.IntVar(depth, "d", 3, "目录深度")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	path := "~"
	if fs.NArg() > 0 {
		path = commands.NormalizePath(fs.Arg(0))
	}
	return commands.Tree(path, *host, *depth, out)
}

func cmdGrep(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("grep")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	include := fs.String("include", "", "文件名匹配")
	fs.StringVar(include, "g", "", "文件名匹配")
	noLine := fs.Bool("no-line-number", false, "不显示行号")
	fs.BoolVar(noLine, "N", false, "不显示行号")
	context := fs.Int("context", 0, "上下文行数")
	fs.IntVar(context, "C", 0, "上下文行数")
	var maxMatches int
	hasMax := false
	fs.Func("max-matches", "最多返回匹配数", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		maxMatches = n
		hasMax = true
		return nil
	})
	grouped := fs.Bool("group", false, "按文件聚合输出")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 1 {
		return 1, fmt.Errorf("grep requires PATTERN")
	}
	path := "."
	if fs.NArg() > 1 {
		path = commands.NormalizePath(fs.Arg(1))
	}
	var max *int
	if hasMax {
		max = &maxMatches
	}
	return commands.Grep(fs.Arg(0), path, *host, *include, !*noLine, *context, max, *grouped, out)
}

func cmdSlice(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("slice")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	cwd := fs.String("cwd", "", "远程工作目录")
	lineRange := fs.String("range", "", "读取行范围")
	around := fs.String("around", "", "围绕文本")
	match := fs.String("match", "", "围绕匹配文本")
	lines := fs.Int("lines", 80, "窗口行数")
	var ctxVal int
	var ctx *int
	fs.Func("context", "上下文行数", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		ctxVal = n
		ctx = &ctxVal
		return nil
	})
	noLine := fs.Bool("no-line-number", false, "不显示行号")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 1 {
		return 1, fmt.Errorf("slice requires FILE")
	}
	normCwd := ""
	if *cwd != "" {
		normCwd = commands.NormalizePath(*cwd)
	}
	return commands.Slice(commands.NormalizePath(fs.Arg(0)), *host, normCwd, *lineRange, *around, *match, *lines, ctx, !*noLine, out)
}

func cmdFind(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("find")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	fileType := fs.String("type", "", "文件类型")
	fs.StringVar(fileType, "t", "", "文件类型")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 1 {
		return 1, fmt.Errorf("find requires NAME")
	}
	path := "."
	if fs.NArg() > 1 {
		path = commands.NormalizePath(fs.Arg(1))
	}
	return commands.Find(fs.Arg(0), path, *host, *fileType, out)
}

func cmdHeadTail(kind string, args []string, out commands.Output) (int, error) {
	fs := newFlagSet(kind)
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	lines := fs.Int("lines", 20, "显示行数")
	fs.IntVar(lines, "n", 20, "显示行数")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 1 {
		return 1, fmt.Errorf("%s requires FILE", kind)
	}
	return commands.HeadTail(kind, commands.NormalizePath(fs.Arg(0)), *host, *lines, out)
}

func cmdWrite(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("write")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	var contentValue string
	var content *string
	fs.Func("content", "文件内容", func(v string) error {
		contentValue = v
		content = &contentValue
		return nil
	})
	fs.Func("c", "文件内容", func(v string) error {
		contentValue = v
		content = &contentValue
		return nil
	})
	appendMode := fs.Bool("append", false, "追加模式")
	fs.BoolVar(appendMode, "a", false, "追加模式")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 1 {
		return 1, fmt.Errorf("write requires PATH")
	}
	return commands.Write(commands.NormalizePath(fs.Arg(0)), content, *host, *appendMode, out)
}

func cmdDiff(args []string, out commands.Output) (int, error) {
	fs := newFlagSet("diff")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	local := fs.Bool("local", false, "比较远程和本地")
	fs.BoolVar(local, "l", false, "比较远程和本地")
	if parseHelp(fs, args) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return 1, err
	}
	if fs.NArg() < 2 {
		return 1, fmt.Errorf("diff requires FILE1 FILE2")
	}
	return commands.Diff(commands.NormalizePath(fs.Arg(0)), fs.Arg(1), *host, *local, out)
}

func cmdRepo(args []string, out commands.Output) (int, error) {
	if len(args) == 0 {
		return 1, fmt.Errorf("repo requires subcommand")
	}
	switch args[0] {
	case "resolve":
		fs := newFlagSet("repo resolve")
		host := stringFlagAliases(fs, "host", "", "主机别名", "H")
		if parseHelp(fs, args[1:]) {
			return 0, nil
		}
		if err := parseFlagArgs(fs, args[1:]); err != nil {
			return 1, err
		}
		if fs.NArg() < 1 {
			return 1, fmt.Errorf("repo resolve requires NAME")
		}
		return commands.RepoResolve(fs.Arg(0), *host, out)
	default:
		return 1, fmt.Errorf("unknown repo command: %s", args[0])
	}
}

func cmdVerify(args []string, out commands.Output) (int, error) {
	if len(args) == 0 || args[0] != "go" {
		return 1, fmt.Errorf("verify requires go")
	}
	fs := newFlagSet("verify go")
	cwd := fs.String("cwd", "", "远程 Go 仓库目录")
	changed := fs.Bool("changed", false, "只验证变更涉及 package")
	also := multiString{}
	fs.Var(&also, "also", "追加 package")
	timeout := fs.Int("timeout", 300, "超时时间")
	fs.IntVar(timeout, "t", 300, "超时时间")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	if parseHelp(fs, args[1:]) {
		return 0, nil
	}
	if err := parseFlagArgs(fs, args[1:]); err != nil {
		return 1, err
	}
	if *cwd == "" {
		return 1, fmt.Errorf("--cwd is required")
	}
	return commands.VerifyGo(commands.NormalizePath(*cwd), *changed, also, *host, *timeout, out)
}

func cmdEdit(args []string, out commands.Output) (int, error) {
	if len(args) == 0 {
		return 1, fmt.Errorf("edit requires subcommand")
	}
	switch args[0] {
	case "replace":
		fs := newFlagSet("edit replace")
		host := stringFlagAliases(fs, "host", "", "主机别名", "H")
		all := fs.Bool("all", false, "替换所有匹配")
		fs.BoolVar(all, "a", false, "替换所有匹配")
		if parseHelp(fs, args[1:]) {
			return 0, nil
		}
		if err := parseFlagArgs(fs, args[1:]); err != nil {
			return 1, err
		}
		if fs.NArg() < 3 {
			return 1, fmt.Errorf("replace requires PATH OLD NEW")
		}
		editArgs := []string{commands.NormalizePath(fs.Arg(0)), fs.Arg(1), fs.Arg(2)}
		if *all {
			editArgs = append(editArgs, "--all")
		}
		return commands.Edit("replace", editArgs, *host, out)
	case "insert":
		fs := newFlagSet("edit insert")
		host := stringFlagAliases(fs, "host", "", "主机别名", "H")
		after := fs.Bool("after", false, "在行后插入")
		if parseHelp(fs, args[1:]) {
			return 0, nil
		}
		if err := parseFlagArgs(fs, args[1:]); err != nil {
			return 1, err
		}
		if fs.NArg() < 3 {
			return 1, fmt.Errorf("insert requires PATH LINE CONTENT")
		}
		editArgs := []string{commands.NormalizePath(fs.Arg(0)), fs.Arg(1), fs.Arg(2)}
		if *after {
			editArgs = append(editArgs, "--after")
		}
		return commands.Edit("insert", editArgs, *host, out)
	case "delete":
		fs := newFlagSet("edit delete")
		host := stringFlagAliases(fs, "host", "", "主机别名", "H")
		if parseHelp(fs, args[1:]) {
			return 0, nil
		}
		if err := parseFlagArgs(fs, args[1:]); err != nil {
			return 1, err
		}
		if fs.NArg() < 2 {
			return 1, fmt.Errorf("delete requires PATH START [END]")
		}
		editArgs := []string{commands.NormalizePath(fs.Arg(0)), fs.Arg(1)}
		if fs.NArg() > 2 {
			editArgs = append(editArgs, fs.Arg(2))
		}
		return commands.Edit("delete", editArgs, *host, out)
	case "line":
		fs := newFlagSet("edit line")
		host := stringFlagAliases(fs, "host", "", "主机别名", "H")
		if parseHelp(fs, args[1:]) {
			return 0, nil
		}
		if err := parseFlagArgs(fs, args[1:]); err != nil {
			return 1, err
		}
		if fs.NArg() < 3 {
			return 1, fmt.Errorf("line requires PATH NUM CONTENT")
		}
		return commands.Edit("line", []string{commands.NormalizePath(fs.Arg(0)), fs.Arg(1), fs.Arg(2)}, *host, out)
	default:
		return 1, fmt.Errorf("unknown edit command: %s", args[0])
	}
}

func cmdConfig(args []string, out commands.Output) (int, error) {
	if len(args) == 0 {
		return 1, fmt.Errorf("config requires subcommand")
	}
	switch args[0] {
	case "show":
		return commands.ConfigShow(out)
	case "add":
		fs := newFlagSet("config add")
		user := fs.String("user", "maifeng", "用户名")
		fs.StringVar(user, "u", "maifeng", "用户名")
		shell := fs.String("shell", "", "默认 shell")
		var timeout *int
		fs.Func("exec-timeout", "默认超时", func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			timeout = &n
			return nil
		})
		roots := multiString{}
		fs.Var(&roots, "repo-root", "远端代码根目录")
		setDefault := fs.Bool("default", false, "设为默认主机")
		fs.BoolVar(setDefault, "d", false, "设为默认主机")
		if err := parseFlagArgs(fs, args[1:]); err != nil {
			return 1, err
		}
		if fs.NArg() < 2 {
			return 1, fmt.Errorf("config add requires ALIAS HOSTNAME")
		}
		return commands.ConfigAdd(fs.Arg(0), fs.Arg(1), *user, *shell, timeout, roots, *setDefault, out)
	case "set-shell":
		if len(args) < 3 {
			return 1, fmt.Errorf("config set-shell requires ALIAS SHELL")
		}
		return commands.ConfigSet(args[1], "shell", args[2], out)
	case "set-exec-timeout":
		if len(args) < 3 {
			return 1, fmt.Errorf("config set-exec-timeout requires ALIAS TIMEOUT")
		}
		return commands.ConfigSet(args[1], "exec-timeout", args[2], out)
	case "add-repo-root":
		if len(args) < 3 {
			return 1, fmt.Errorf("config add-repo-root requires ALIAS ROOT")
		}
		return commands.ConfigAddRepoRoot(args[1], args[2], out)
	case "clear-repo-roots":
		if len(args) < 2 {
			return 1, fmt.Errorf("config clear-repo-roots requires ALIAS")
		}
		return commands.ConfigSet(args[1], "clear-repo-roots", "", out)
	case "set-default":
		if len(args) < 2 {
			return 1, fmt.Errorf("config set-default requires ALIAS")
		}
		return commands.ConfigSet(args[1], "default", "", out)
	default:
		return 1, fmt.Errorf("unknown config command: %s", args[0])
	}
}

func parseCwdHost(name string, args []string) (string, string, error) {
	fs := newFlagSet(name)
	cwd := fs.String("cwd", "", "远程 Git 仓库目录")
	host := stringFlagAliases(fs, "host", "", "主机别名", "H")
	if parseHelp(fs, args) {
		return "", "", nil
	}
	if err := parseFlagArgs(fs, args); err != nil {
		return "", "", err
	}
	if *cwd == "" {
		return "", "", fmt.Errorf("--cwd is required")
	}
	return commands.NormalizePath(*cwd), *host, nil
}

func printTopHelp() {
	fmt.Println(`Usage: dev [OPTIONS] COMMAND [ARGS]...

Options:
      --json      JSON 格式输出
      --verbose   显示完整错误栈
  -h, --help      Show help
      --version   Show version

Commands:
  ls cat push pull exec exec-watch patch
  repo-status git-snapshot repo-diff repo resolve
  tree grep slice find head tail write diff
  verify go edit config stats version`)
}

func printExecHelp() {
	fmt.Println(`Usage: dev exec COMMAND [OPTIONS]

Options:
      --host, -H, -h HOST      主机别名
      --cwd CWD                远程工作目录
      --timeout, -t SECONDS    超时时间
      --watch, --wait          等待长命令并低频输出状态
      --interval SECONDS       --watch 状态输出间隔
      --summary-chars N        --watch 结束摘要最大字符数
      --shell SHELL            none|zsh|zsh-login|bash|bash-login
      --help                   Show help`)
}

func printExecWatchHelp() {
	fmt.Println(`Usage: dev exec-watch CMD [OPTIONS]

Options:
      --host, -H, -h HOST      主机别名
      --cwd CWD                远程工作目录
      --interval SECONDS       状态输出间隔
      --timeout, -t SECONDS    总超时时间
      --summary-chars N        结束摘要最大字符数
      --shell SHELL            none|zsh|zsh-login|bash|bash-login
      --help                   Show help`)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Printf("Usage: dev %s [OPTIONS]\n", name)
		fs.PrintDefaults()
	}
	return fs
}

func parseHelp(fs *flag.FlagSet, args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fs.Usage()
			return true
		}
	}
	return false
}

func parseFlagArgs(fs *flag.FlagSet, args []string) error {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if eq := strings.Index(name, "="); eq >= 0 {
			name = name[:eq]
		}
		f := fs.Lookup(name)
		if f == nil {
			return fmt.Errorf("unknown flag %s for command %q", arg, fs.Name())
		}
		flags = append(flags, arg)
		if strings.Contains(arg, "=") || isBoolFlag(f) {
			continue
		}
		if i+1 >= len(args) {
			return fmt.Errorf("%s requires value", arg)
		}
		i++
		flags = append(flags, args[i])
	}
	return fs.Parse(append(flags, positionals...))
}

func isBoolFlag(f *flag.Flag) bool {
	type boolFlag interface{ IsBoolFlag() bool }
	bf, ok := f.Value.(boolFlag)
	return ok && bf.IsBoolFlag()
}

func stringFlagAliases(fs *flag.FlagSet, name, value, usage string, aliases ...string) *string {
	ptr := fs.String(name, value, usage)
	for _, alias := range aliases {
		fs.StringVar(ptr, alias, value, usage)
	}
	return ptr
}

func looksLikeExecHost(value string) bool {
	if strings.HasPrefix(value, "@") {
		return true
	}
	cfg, err := config.Load()
	return err == nil && cfg.Hosts[value].Hostname != ""
}

func shellJoin(parts []string) string {
	quoted := make([]string, len(parts))
	for i, p := range parts {
		if strings.ContainsAny(p, " \t\n'\"\\$`|&;<>*?()[]{}!") {
			quoted[i] = "'" + strings.ReplaceAll(p, "'", "'\"'\"'") + "'"
		} else {
			quoted[i] = p
		}
	}
	return strings.Join(quoted, " ")
}

func extractCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if args[0] == "config" || args[0] == "edit" || args[0] == "repo" || args[0] == "verify" {
		if len(args) > 1 {
			return args[0] + " " + args[1]
		}
	}
	if allCommands[args[0]] {
		return args[0]
	}
	return ""
}

type multiString []string

func (m *multiString) String() string { return strings.Join(*m, ",") }
func (m *multiString) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
