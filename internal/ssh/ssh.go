package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/DreamCats/dev-connect/internal/config"
	"github.com/DreamCats/dev-connect/internal/model"
)

type Result struct {
	ReturnCode int
	Stdout     string
	Stderr     string
}

func (r Result) Success() bool { return r.ReturnCode == 0 }

var shellPresets = map[string]string{
	"zsh":        "zsh -ic",
	"zsh-login":  "zsh -lic",
	"bash":       "bash -ic",
	"bash-login": "bash -lic",
}

func RunCommand(cmd, hostAlias string, timeout int, shell string, stdin string) (Result, error) {
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return Result{}, err
	}
	args := BuildSSHCmd(host, WrapShellCmd(cmd, shell))
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}
	c := exec.CommandContext(ctx, args[0], args[1:]...)
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err = c.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{}, fmt.Errorf("命令执行超时（%d 秒）", timeout)
	}
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			return Result{}, fmt.Errorf("执行失败: %w", err)
		}
	}
	return Result{ReturnCode: rc, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func Upload(localPath, remotePath, hostAlias string, timeout int) (Result, error) {
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return Result{}, err
	}
	return runLocal(buildSCPCmd(host, localPath, remotePath, false), timeout, "上传")
}

func Download(remotePath, localPath, hostAlias string, timeout int) (Result, error) {
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return Result{}, err
	}
	return runLocal(buildSCPCmd(host, remotePath, localPath, true), timeout, "下载")
}

func runLocal(args []string, timeout int, name string) (Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{}, fmt.Errorf("%s超时（%d 秒）", name, timeout)
	}
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			return Result{}, fmt.Errorf("%s失败: %w", name, err)
		}
	}
	return Result{ReturnCode: rc, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func BuildSSHCmd(host model.HostConfig, cmd string) []string {
	return []string{
		"ssh",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=~/.ssh/sockets/%r@%h-%p",
		"-o", "ControlPersist=600",
		host.User + "@" + host.Hostname,
		cmd,
	}
}

func WrapShellCmd(cmd, shell string) string {
	if shell == "" || shell == "none" {
		return cmd
	}
	shellCmd := shellPresets[shell]
	if shellCmd == "" {
		shellCmd = shell
	}
	return shellCmd + " " + ShellQuote(cmd)
}

func buildSCPCmd(host model.HostConfig, source, dest string, reverse bool) []string {
	remote := host.User + "@" + host.Hostname
	base := []string{
		"scp",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=~/.ssh/sockets/%r@%h-%p",
		"-o", "ControlPersist=600",
	}
	if reverse {
		return append(base, remote+":"+source, dest)
	}
	return append(base, source, remote+":"+dest)
}

func WrapRemoteCwd(cmd, cwd string) string {
	if cwd == "" {
		return cmd
	}
	return "cd " + QuoteRemotePath(cwd) + " && " + cmd
}

func QuoteRemotePath(path string) string {
	if path == "~" {
		return "~"
	}
	if strings.HasPrefix(path, "~/") {
		rest := strings.TrimPrefix(path, "~/")
		if needsQuoting(rest) {
			return "~/" + ShellQuote(rest)
		}
		return path
	}
	if !needsQuoting(path) {
		return path
	}
	return ShellQuote(path)
}

func ExpandTilde(path string) string {
	if path == "~" {
		return "~"
	}
	if strings.HasPrefix(path, "~/") {
		rest := strings.TrimPrefix(path, "~/")
		if needsQuoting(rest) {
			return "~/" + ShellQuote(rest)
		}
		return path
	}
	if needsQuoting(path) {
		return ShellQuote(path)
	}
	return path
}

var safeRE = regexp.MustCompile(`^[a-zA-Z0-9_./@-]+$`)

func needsQuoting(s string) bool {
	return s == "" || !safeRE.MatchString(s)
}

func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
