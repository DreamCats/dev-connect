package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/DreamCats/dev-connect/internal/config"
	"github.com/DreamCats/dev-connect/internal/ssh"
)

const defaultCodegraphRemote = "~/.local/bin/codegraph"

func CodegraphInstall(hostAlias, remotePath, sourcePath, module string, timeout int, out Output) (int, error) {
	if remotePath == "" {
		remotePath = defaultCodegraphRemote
	}
	if module == "" {
		module = "github.com/DreamCats/codegraph-cli/cmd/codegraph@latest"
	}
	target, err := remoteGoTarget(hostAlias)
	if err != nil {
		return 1, err
	}
	localPath, cleanup, err := codegraphBinaryForTarget(sourcePath, module, target.goos, target.goarch)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return 1, err
	}
	if code, err := ensureRemoteDir(remotePath, hostAlias, timeout, out); code != 0 || err != nil {
		return code, err
	}
	result, err := ssh.Upload(localPath, remotePath, hostAlias, timeout)
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return fallbackCode(result.ReturnCode), nil
	}
	verify := strings.Join([]string{
		"chmod +x " + ssh.ExpandTilde(remotePath),
		ssh.ExpandTilde(remotePath) + " --version",
	}, " && ")
	verified, err := ssh.RunCommand(verify, hostAlias, timeout, "", "")
	if err != nil {
		return 1, err
	}
	payload := map[string]any{
		"success": verified.Success(), "remote_path": remotePath, "local_path": localPath,
		"goos": target.goos, "goarch": target.goarch, "returncode": verified.ReturnCode,
		"stdout": verified.Stdout, "stderr": verified.Stderr,
	}
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else if verified.Success() {
		fmt.Fprintf(out.Out, "已安装 codegraph: %s\n%s", remotePath, verified.Stdout)
	} else {
		fmt.Fprintf(out.Err, "错误: codegraph 安装后验证失败\n%s", verified.Stderr)
	}
	if !verified.Success() {
		return fallbackCode(verified.ReturnCode), nil
	}
	return 0, nil
}

func CodegraphRun(args []string, cwd, repo, hostAlias string, timeout int, out Output) (int, error) {
	if repo != "" {
		resolved, err := ResolveRepoPath(repo, hostAlias)
		if err != nil {
			return 1, err
		}
		cwd = resolved
	}
	if out.JSON && !hasArg(args, "--json") {
		args = append([]string{"--json"}, args...)
	}
	if cwd != "" {
		args = injectCodegraphPath(args, cwd)
	}
	result, err := ssh.RunCommand(buildCodegraphProxyCmd(args), hostAlias, timeout, "", "")
	if err != nil {
		return 1, err
	}
	fmt.Fprint(out.Out, result.Stdout)
	fmt.Fprint(out.Err, result.Stderr)
	if !result.Success() {
		return fallbackCode(result.ReturnCode), nil
	}
	return 0, nil
}

func ResolveRepoPath(repo, hostAlias string) (string, error) {
	host, err := configGetHost(hostAlias)
	if err != nil {
		return "", err
	}
	result, err := ssh.RunCommand(buildRepoResolveCmd(repo, host.RepoRoots, host.User), hostAlias, 30, "", "")
	if err != nil {
		return "", err
	}
	payload := parseJSONResult(result.Stdout, repo)
	if ok, _ := payload["success"].(bool); !ok {
		return "", errors.New(firstNonEmpty(fmt.Sprint(payload["error"]), "repo not found"))
	}
	path, _ := payload["path"].(string)
	if path == "" {
		return "", errors.New("repo resolver returned empty path")
	}
	return path, nil
}

type goTarget struct {
	goos   string
	goarch string
}

func remoteGoTarget(hostAlias string) (goTarget, error) {
	result, err := ssh.RunCommand("uname -s && uname -m", hostAlias, 10, "", "")
	if err != nil {
		return goTarget{}, err
	}
	if !result.Success() {
		return goTarget{}, fmt.Errorf("无法探测远端平台: %s", result.Stderr)
	}
	lines := strings.Fields(result.Stdout)
	if len(lines) < 2 {
		return goTarget{}, fmt.Errorf("无法解析远端平台: %q", result.Stdout)
	}
	goos, goarch := mapGOOS(lines[0]), mapGOARCH(lines[1])
	if goos == "" || goarch == "" {
		return goTarget{}, fmt.Errorf("暂不支持远端平台: %s/%s", lines[0], lines[1])
	}
	return goTarget{goos: goos, goarch: goarch}, nil
}

func mapGOOS(v string) string {
	switch strings.ToLower(v) {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	default:
		return ""
	}
}

func mapGOARCH(v string) string {
	switch strings.ToLower(v) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return ""
	}
}

func codegraphBinaryForTarget(sourcePath, module, goos, goarch string) (string, func(), error) {
	if sourcePath != "" {
		return sourcePath, nil, nil
	}
	if goos == runtime.GOOS && goarch == runtime.GOARCH {
		if path := findLocalCodegraphBinary(); path != "" {
			return path, nil, nil
		}
	}
	dir, err := os.MkdirTemp("", "dev-codegraph-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "codegraph")
	if err := buildModuleCommand(dir, module, path, goos, goarch); err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := os.Stat(path); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func buildModuleCommand(workDir, moduleSpec, outputPath, goos, goarch string) error {
	pkg, version := splitModuleSpec(moduleSpec)
	root := moduleRootForPackage(pkg)
	rootSpec := root
	if version != "" {
		rootSpec += "@" + version
	}
	steps := [][]string{
		{"go", "mod", "init", "dev-codegraph-install"},
		{"go", "get", rootSpec},
		{"go", "get", pkg},
		{"go", "build", "-o", outputPath, pkg},
	}
	for _, step := range steps {
		cmd := exec.Command(step[0], step[1:]...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s failed: %w\n%s", strings.Join(step, " "), err, string(out))
		}
	}
	return nil
}

func splitModuleSpec(spec string) (string, string) {
	if pkg, version, ok := strings.Cut(spec, "@"); ok {
		return pkg, version
	}
	return spec, "latest"
}

func moduleRootForPackage(pkg string) string {
	if idx := strings.Index(pkg, "/cmd/"); idx >= 0 {
		return pkg[:idx]
	}
	return pkg
}

func findLocalCodegraphBinary() string {
	candidates := []string{}
	if env := os.Getenv("CODEGRAPH_BIN"); env != "" {
		candidates = append(candidates, env)
	}
	if path, err := exec.LookPath("codegraph"); err == nil {
		candidates = append(candidates, path)
	}
	if gobin := goEnv("GOBIN"); gobin != "" {
		candidates = append(candidates, filepath.Join(gobin, "codegraph"))
	}
	if gopath := goEnv("GOPATH"); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "bin", "codegraph"))
	}
	for _, path := range candidates {
		if isUsableBinary(path) {
			return path
		}
	}
	return ""
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isUsableBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return false
	}
	return !strings.HasPrefix(string(raw[:min(len(raw), 2)]), "#!")
}

func ensureRemoteDir(remotePath, hostAlias string, timeout int, out Output) (int, error) {
	dir := remoteDir(remotePath)
	result, err := ssh.RunCommand("mkdir -p "+ssh.ExpandTilde(dir), hostAlias, timeout, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return fallbackCode(result.ReturnCode), nil
	}
	return 0, nil
}

func remoteDir(path string) string {
	clean := strings.TrimRight(path, "/")
	if clean == "" || clean == "~" {
		return "~"
	}
	idx := strings.LastIndex(clean, "/")
	if idx < 0 {
		return "."
	}
	if idx == 0 {
		return "/"
	}
	return clean[:idx]
}

func buildCodegraphProxyCmd(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, ssh.ShellQuote(arg))
	}
	return strings.Join([]string{
		`if command -v codegraph >/dev/null 2>&1; then CG="$(command -v codegraph)"; elif [ -x "$HOME/.local/bin/codegraph" ]; then CG="$HOME/.local/bin/codegraph"; else echo "codegraph not found; run: dev cg install" >&2; exit 127; fi`,
		`"$CG" ` + strings.Join(quoted, " "),
	}, "\n")
}

func injectCodegraphPath(args []string, cwd string) []string {
	if len(args) == 0 || hasPathOrTarget(args) {
		return args
	}
	cmdIndex := codegraphCommandIndex(args)
	if cmdIndex < 0 || !codegraphAcceptsPath(args[cmdIndex]) {
		return args
	}
	out := append([]string{}, args...)
	return append(out, "--path", cwd)
}

func codegraphCommandIndex(args []string) int {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--json" || arg == "--verbose" {
			continue
		}
		if arg == "--target" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--target=") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return i
	}
	return -1
}

func codegraphAcceptsPath(cmd string) bool {
	switch cmd {
	case "init", "index", "sync", "query", "files", "status", "resolve", "callers", "callees", "impact", "affected", "context", "overview", "architecture":
		return true
	default:
		return false
	}
}

func hasPathOrTarget(args []string) bool {
	return hasArg(args, "--path") || hasArgPrefix(args, "--path=") || hasArg(args, "--target") || hasArgPrefix(args, "--target=")
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func hasArgPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

type repoHostConfig struct {
	RepoRoots []string
	User      string
}

func configGetHost(hostAlias string) (repoHostConfig, error) {
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return repoHostConfig{}, err
	}
	return repoHostConfig{RepoRoots: host.RepoRoots, User: host.User}, nil
}
