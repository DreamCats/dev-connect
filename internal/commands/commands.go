package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/DreamCats/dev-connect/internal/config"
	"github.com/DreamCats/dev-connect/internal/model"
	"github.com/DreamCats/dev-connect/internal/ssh"
	"github.com/DreamCats/dev-connect/internal/stats"
)

type Output struct {
	JSON bool
	Out  io.Writer
	Err  io.Writer
	In   io.Reader
}

func NewOutput(json bool) Output {
	return Output{JSON: json, Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
}

func (o Output) PrintJSON(payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(o.Out, string(data))
	return err
}

func NormalizePath(path string) string { return config.NormalizeLocalHomeToTilde(path) }

func ListDir(path, hostAlias string, out Output) (int, error) {
	target := ssh.ExpandTilde(path)
	if target == "~" {
		target = "~/"
	}
	result, err := ssh.RunCommand("ls -la "+target, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	if out.JSON {
		return 0, out.PrintJSON(parseLS(result.Stdout, path))
	}
	fmt.Fprintln(out.Out, result.Stdout)
	return 0, nil
}

func parseLS(raw, path string) map[string]any {
	items := []map[string]string{}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 9 {
			continue
		}
		name := strings.Join(parts[8:], " ")
		if name == "." || name == ".." {
			continue
		}
		typ := "file"
		if strings.HasPrefix(parts[0], "d") {
			typ = "directory"
		}
		items = append(items, map[string]string{
			"name": name, "type": typ, "permissions": parts[0], "size": parts[4],
		})
	}
	return map[string]any{"path": path, "items": items, "count": len(items)}
}

func Cat(paths []string, hostAlias, cwd string, out Output) (int, error) {
	result, err := ssh.RunCommand(buildCatCmd(paths, cwd), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil {
		if result.Stderr != "" {
			fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		} else {
			fmt.Fprintln(out.Err, "错误: 无法解析远程 cat 输出")
		}
		return fallbackCode(result.ReturnCode), nil
	}
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		printCatPlain(payload, out)
		if result.Stderr != "" {
			fmt.Fprint(out.Err, result.Stderr)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func buildCatCmd(paths []string, cwd string) string {
	raw, _ := json.Marshal(paths)
	script := fmt.Sprintf(`
import json
import os
from pathlib import Path
import sys

paths = %q
items = []
has_error = False
for path in json.loads(paths):
    try:
        content = Path(os.path.expanduser(path)).read_text(errors="replace")
        items.append({"path": path, "content": content, "size": len(content), "success": True, "error": ""})
    except Exception as exc:
        has_error = True
        items.append({"path": path, "content": "", "size": 0, "success": False, "error": str(exc)})
print(json.dumps({"cwd": os.getcwd(), "files": items, "count": len(items), "success": not has_error}, ensure_ascii=False))
sys.exit(1 if has_error else 0)
`, string(raw))
	steps := []string{"set -e"}
	if cwd != "" {
		steps = append(steps, "cd "+ssh.QuoteRemotePath(cwd))
	}
	steps = append(steps, "python3 - <<'PY'", strings.TrimSpace(script), "PY")
	return strings.Join(steps, "\n")
}

func printCatPlain(payload map[string]any, out Output) {
	files, ok := payload["files"].([]any)
	if !ok {
		fmt.Fprintln(out.Err, "错误: 远程 cat 输出缺少 files")
		return
	}
	single := len(files) == 1
	for i, raw := range files {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path := fmt.Sprint(item["path"])
		success, _ := item["success"].(bool)
		if !single {
			if i > 0 {
				fmt.Fprintln(out.Out)
			}
			suffix := ""
			if !success {
				suffix = " (error)"
			}
			fmt.Fprintf(out.Out, "===== %s%s =====\n", path, suffix)
		}
		if success {
			fmt.Fprint(out.Out, fmt.Sprint(item["content"]))
		} else {
			fmt.Fprintf(out.Err, "错误: %s: %v\n", path, item["error"])
		}
	}
}

func Push(localPath, remotePath, hostAlias string, out Output) (int, error) {
	result, err := ssh.Upload(localPath, remotePath, hostAlias, 60)
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	fmt.Fprintf(out.Out, "已上传: %s -> %s\n", localPath, remotePath)
	return 0, nil
}

func Pull(remotePath, localPath, hostAlias string, out Output) (int, error) {
	result, err := ssh.Download(remotePath, localPath, hostAlias, 60)
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	fmt.Fprintf(out.Out, "已下载: %s -> %s\n", remotePath, localPath)
	return 0, nil
}

func Exec(command, hostAlias string, timeout *int, shell, cwd string, out Output) (int, error) {
	activeShell, activeTimeout, err := resolveExecOptions(hostAlias, timeout, shell)
	if err != nil {
		return 1, err
	}
	effective := ssh.WrapRemoteCwd(command, cwd)
	result, err := ssh.RunCommand(effective, hostAlias, activeTimeout, activeShell, "")
	if err != nil {
		return 1, err
	}
	if out.JSON {
		_ = out.PrintJSON(map[string]any{
			"command": command, "effective_command": effective, "cwd": nullableString(cwd),
			"shell": nullableString(activeShell), "timeout": activeTimeout, "returncode": result.ReturnCode,
			"stdout": result.Stdout, "stderr": result.Stderr, "success": result.Success(),
		})
	} else {
		if result.Stdout != "" {
			fmt.Fprint(out.Out, result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(out.Err, result.Stderr)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func resolveExecOptions(hostAlias string, timeout *int, shellOpt string) (string, int, error) {
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return "", 0, err
	}
	activeShell := host.Shell
	if shellOpt == "none" {
		activeShell = ""
	} else if shellOpt != "" {
		activeShell = shellOpt
	}
	activeTimeout := 30
	if host.ExecTimeout != nil {
		activeTimeout = *host.ExecTimeout
	}
	if timeout != nil {
		activeTimeout = *timeout
	}
	return activeShell, activeTimeout, nil
}

func Grep(pattern, path, hostAlias, include string, lineNumber bool, context int, maxMatches *int, grouped bool, out Output) (int, error) {
	if context < 0 {
		return 1, errors.New("--context 不能小于 0")
	}
	if maxMatches != nil && *maxMatches < 1 {
		return 1, errors.New("--max-matches 必须大于 0")
	}
	rgCheck, err := ssh.RunCommand("which rg", hostAlias, 5, "", "")
	if err != nil {
		return 1, err
	}
	useRG := rgCheck.Success()
	effectiveLineNumber := lineNumber || out.JSON || context > 0 || grouped
	cmd := buildGrepCmd(pattern, path, useRG, include, effectiveLineNumber, context, maxMatches)
	result, err := ssh.RunCommand(cmd, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() && result.ReturnCode != 1 {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	if out.JSON {
		matches, files := parseGrepOutput(result.Stdout, maxMatches)
		return 0, out.PrintJSON(map[string]any{
			"pattern": pattern, "path": path, "tool": ternary(useRG, "rg", "grep"), "context": context,
			"matches": matches, "files": files, "count": len(matches), "file_count": len(files),
		})
	}
	if grouped {
		_, files := parseGrepOutput(result.Stdout, maxMatches)
		printGrepGrouped(files, out)
	} else {
		fmt.Fprint(out.Out, result.Stdout)
	}
	return 0, nil
}

func buildGrepCmd(pattern, path string, useRG bool, include string, lineNumber bool, context int, maxMatches *int) string {
	parts := []string{}
	if useRG {
		parts = append(parts, "rg")
		if lineNumber {
			parts = append(parts, "-n")
		}
		if context > 0 {
			parts = append(parts, "-C", strconv.Itoa(context))
		}
		if maxMatches != nil {
			parts = append(parts, "-m", strconv.Itoa(*maxMatches))
		}
		if include != "" {
			parts = append(parts, "--glob", ssh.ShellQuote(include))
		}
	} else {
		parts = append(parts, "grep", "-r")
		if lineNumber {
			parts = append(parts, "-n")
		}
		if context > 0 {
			parts = append(parts, "-C", strconv.Itoa(context))
		}
		if maxMatches != nil {
			parts = append(parts, "-m", strconv.Itoa(*maxMatches))
		}
		if include != "" {
			parts = append(parts, "--include="+ssh.ShellQuote(include))
		}
	}
	parts = append(parts, ssh.ShellQuote(pattern), ssh.ExpandTilde(path))
	return strings.Join(parts, " ")
}

type parsedLine struct {
	file    string
	line    int
	content string
	match   bool
}

func parseGrepOutput(output string, maxMatches *int) ([]map[string]any, []map[string]any) {
	matches := []map[string]any{}
	files := map[string]map[string]any{}
	order := []string{}
	pending := map[string][]map[string]any{}
	last := map[string]map[string]any{}
	for _, raw := range strings.Split(output, "\n") {
		if raw == "--" {
			pending = map[string][]map[string]any{}
			last = map[string]map[string]any{}
			continue
		}
		item, ok := parseGrepLine(raw)
		if !ok {
			continue
		}
		if _, ok := files[item.file]; !ok {
			files[item.file] = map[string]any{"file": item.file, "matches": []map[string]any{}, "count": 0}
			order = append(order, item.file)
		}
		if item.match {
			if maxMatches != nil && len(matches) >= *maxMatches {
				break
			}
			match := map[string]any{"file": item.file, "line": item.line, "content": item.content, "before": pending[item.file], "after": []map[string]any{}}
			matches = append(matches, match)
			bucket := files[item.file]
			bucket["matches"] = append(bucket["matches"].([]map[string]any), match)
			bucket["count"] = bucket["count"].(int) + 1
			last[item.file] = match
			delete(pending, item.file)
			continue
		}
		ctx := map[string]any{"line": item.line, "content": item.content}
		if m := last[item.file]; m != nil {
			m["after"] = append(m["after"].([]map[string]any), ctx)
			pending[item.file] = append(pending[item.file], ctx)
		} else {
			pending[item.file] = append(pending[item.file], ctx)
		}
	}
	grouped := make([]map[string]any, 0, len(order))
	for _, file := range order {
		grouped = append(grouped, files[file])
	}
	return matches, grouped
}

func parseGrepLine(line string) (parsedLine, bool) {
	if line == "" || line == "--" {
		return parsedLine{}, false
	}
	colon, okC := splitNumberedLine(line, ":")
	dash, okD := splitNumberedLine(line, "-")
	if !okC && !okD {
		return parsedLine{content: line, match: true}, true
	}
	if !okC {
		dash.match = false
		return dash, true
	}
	if !okD {
		colon.match = true
		return colon, true
	}
	colonIndex := len(colon.file) + len(strconv.Itoa(colon.line)) + 1
	dashIndex := len(dash.file) + len(strconv.Itoa(dash.line)) + 1
	if colonIndex <= dashIndex {
		colon.match = true
		return colon, true
	}
	dash.match = false
	return dash, true
}

func splitNumberedLine(line, sep string) (parsedLine, bool) {
	first := strings.Index(line, sep)
	for first != -1 {
		second := strings.Index(line[first+1:], sep)
		if second == -1 {
			return parsedLine{}, false
		}
		second += first + 1
		n, err := strconv.Atoi(line[first+1 : second])
		if err == nil {
			return parsedLine{file: line[:first], line: n, content: line[second+1:]}, true
		}
		next := strings.Index(line[first+1:], sep)
		if next == -1 {
			break
		}
		first += next + 1
	}
	return parsedLine{}, false
}

func printGrepGrouped(files []map[string]any, out Output) {
	for i, file := range files {
		if i > 0 {
			fmt.Fprintln(out.Out)
		}
		fmt.Fprintf(out.Out, "===== %s (%v) =====\n", file["file"], file["count"])
		for _, raw := range file["matches"].([]map[string]any) {
			for _, ctx := range raw["before"].([]map[string]any) {
				fmt.Fprintf(out.Out, "%v- %v\n", ctx["line"], ctx["content"])
			}
			fmt.Fprintf(out.Out, "%v: %v\n", raw["line"], raw["content"])
			for _, ctx := range raw["after"].([]map[string]any) {
				fmt.Fprintf(out.Out, "%v- %v\n", ctx["line"], ctx["content"])
			}
		}
	}
}

func Find(name, path, hostAlias, fileType string, out Output) (int, error) {
	parts := []string{"find", "-L", ssh.ExpandTilde(path), "-name", ssh.ShellQuote(name)}
	if fileType != "" {
		parts = append(parts, "-type", fileType)
	}
	result, err := ssh.RunCommand(strings.Join(parts, " "), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	if out.JSON {
		files := []map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
			if line == "" {
				continue
			}
			files = append(files, map[string]string{"path": line, "name": filepath.Base(line)})
		}
		return 0, out.PrintJSON(map[string]any{"name": name, "path": path, "files": files, "count": len(files)})
	}
	fmt.Fprint(out.Out, result.Stdout)
	return 0, nil
}

func HeadTail(cmdName, path, hostAlias string, lines int, out Output) (int, error) {
	cmd := fmt.Sprintf("%s -n %d %s", cmdName, lines, ssh.ExpandTilde(path))
	result, err := ssh.RunCommand(cmd, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	if out.JSON {
		key := "path"
		if cmdName == "tail" {
			key = "file"
		}
		return 0, out.PrintJSON(map[string]any{key: path, "lines": lines, "content": result.Stdout, "size": len(result.Stdout)})
	}
	fmt.Fprint(out.Out, result.Stdout)
	return 0, nil
}

func Tree(path, hostAlias string, depth int, out Output) (int, error) {
	cmd := fmt.Sprintf("find -L %s -maxdepth %d -type f -o -type d | head -100", ssh.ExpandTilde(path), depth)
	result, err := ssh.RunCommand(cmd, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	if out.JSON {
		items := []map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			name := filepath.Base(strings.TrimSuffix(line, "/"))
			typ := "file"
			if strings.HasSuffix(line, "/") {
				typ = "directory"
			}
			items = append(items, map[string]string{"path": line, "name": name, "type": typ})
		}
		return 0, out.PrintJSON(map[string]any{"path": path, "depth": depth, "items": items, "count": len(items)})
	}
	printTree(result.Stdout, path, out)
	return 0, nil
}

func printTree(raw, base string, out Output) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	baseParts := strings.Split(strings.TrimRight(base, "/"), "/")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "/")
		relative := strings.Join(parts[min(len(baseParts), len(parts)):], "/")
		if relative == "" {
			fmt.Fprintf(out.Out, "%s/\n", base)
			continue
		}
		depth := strings.Count(relative, "/")
		name := filepath.Base(relative)
		suffix := ""
		if strings.HasSuffix(line, "/") {
			suffix = "/"
		}
		fmt.Fprintf(out.Out, "%s%s%s\n", strings.Repeat("  ", depth), name, suffix)
	}
}

func Slice(path, hostAlias, cwd, lineRange, around, match string, lines int, context *int, lineNumber bool, out Output) (int, error) {
	selectors := 0
	for _, v := range []string{lineRange, around, match} {
		if v != "" {
			selectors++
		}
	}
	if selectors != 1 {
		return 1, errors.New("--range、--around、--match 必须且只能指定一个")
	}
	if lines < 1 {
		return 1, errors.New("--lines 必须大于 0")
	}
	if context != nil && *context < 0 {
		return 1, errors.New("--context 不能小于 0")
	}
	result, err := ssh.RunCommand(buildSliceCmd(path, cwd, lineRange, around, match, lines, context), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &payload); err != nil {
		if result.Stderr != "" {
			fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		} else {
			fmt.Fprintln(out.Err, "错误: 无法解析远程 slice 输出")
		}
		return fallbackCode(result.ReturnCode), nil
	}
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		printSlicePlain(payload, lineNumber, out)
		if result.Stderr != "" {
			fmt.Fprint(out.Err, result.Stderr)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func buildSliceCmd(path, cwd, lineRange, around, match string, lines int, context *int) string {
	ctxLiteral := "None"
	if context != nil {
		ctxLiteral = strconv.Itoa(*context)
	}
	script := fmt.Sprintf(`
import json
import os
from pathlib import Path
import sys
path = %q
line_range = %s
around = %s
match = %s
window_lines = %d
context = %s
def fail(message):
    print(json.dumps({"path": path, "cwd": os.getcwd(), "success": False, "error": message}, ensure_ascii=False))
    sys.exit(1)
try:
    text = Path(os.path.expanduser(path)).read_text(errors="replace")
except Exception as exc:
    fail(str(exc))
all_lines = text.splitlines()
total = len(all_lines)
matched_line = None
selector = {}
if line_range is not None:
    selector = {"type": "range", "value": line_range}
    raw = line_range.strip()
    if ":" not in raw:
        fail("--range must use START:END")
    left, right = raw.split(":", 1)
    try:
        start = int(left) if left else 1
        end = int(right) if right else total
    except ValueError:
        fail("--range bounds must be integers")
    if start < 1 or end < start:
        fail("--range must satisfy 1 <= START <= END")
else:
    needle = around if around is not None else match
    selector = {"type": "around" if around is not None else "match", "value": needle}
    for index, line in enumerate(all_lines, start=1):
        if needle in line:
            matched_line = index
            break
    if matched_line is None:
        fail(f"pattern not found: {needle}")
    if context is not None:
        start = matched_line - context
        end = matched_line + context
    else:
        before = max((window_lines - 1) // 2, 0)
        after = max(window_lines - before - 1, 0)
        start = matched_line - before
        end = matched_line + after
start = max(start, 1)
end = min(end, total)
items = [{"number": number, "text": all_lines[number - 1]} for number in range(start, end + 1)]
content = "\n".join(item["text"] for item in items)
if items and text.endswith("\n") and end == total:
    content += "\n"
print(json.dumps({"path": path, "cwd": os.getcwd(), "selector": selector, "start": start, "end": end, "total_lines": total, "matched_line": matched_line, "content": content, "lines": items, "count": len(items), "success": True, "error": ""}, ensure_ascii=False))
`, path, pyStringOrNone(lineRange), pyStringOrNone(around), pyStringOrNone(match), lines, ctxLiteral)
	steps := []string{"set -e"}
	if cwd != "" {
		steps = append(steps, "cd "+ssh.QuoteRemotePath(cwd))
	}
	steps = append(steps, "python3 - <<'PY'", strings.TrimSpace(script), "PY")
	return strings.Join(steps, "\n")
}

func printSlicePlain(payload map[string]any, lineNumber bool, out Output) {
	success, _ := payload["success"].(bool)
	if !success {
		fmt.Fprintf(out.Err, "错误: %v\n", payload["error"])
		return
	}
	if !lineNumber {
		fmt.Fprint(out.Out, fmt.Sprint(payload["content"]))
		return
	}
	lines, ok := payload["lines"].([]any)
	if !ok {
		fmt.Fprint(out.Out, fmt.Sprint(payload["content"]))
		return
	}
	end := intFromAny(payload["end"])
	width := len(strconv.Itoa(end))
	if width < 1 {
		width = 1
	}
	for _, raw := range lines {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		number := intFromAny(item["number"])
		fmt.Fprintf(out.Out, "%*d | %v\n", width, number, item["text"])
	}
}

func Write(path string, content *string, hostAlias string, appendMode bool, out Output) (int, error) {
	value := ""
	if content == nil {
		raw, err := io.ReadAll(out.In)
		if err != nil {
			return 1, err
		}
		value = string(raw)
	} else {
		value = *content
	}
	operator := ">"
	if appendMode {
		operator = ">>"
	}
	cmd := fmt.Sprintf("cat %s %s << 'DEV_CONNECT_EOF'\n%s\nDEV_CONNECT_EOF", operator, ssh.ExpandTilde(path), value)
	result, err := ssh.RunCommand(cmd, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	mode := "写入"
	if appendMode {
		mode = "追加"
	}
	fmt.Fprintf(out.Out, "已%s: %s (%d 字节)\n", mode, path, len(value))
	return 0, nil
}

func Edit(kind string, args []string, hostAlias string, out Output) (int, error) {
	switch kind {
	case "replace":
		if len(args) < 3 {
			return 1, errors.New("replace requires PATH OLD NEW")
		}
		all := contains(args[3:], "--all") || contains(args[3:], "-a")
		code, err := runEditScript("replace", args[0], map[string]any{
			"old": args[1], "new": args[2], "all": all,
		}, hostAlias, out)
		if code != 0 || err != nil {
			return code, err
		}
		scope := "首次"
		if all {
			scope = "所有"
		}
		fmt.Fprintf(out.Out, "已替换 %s匹配: '%s' -> '%s'\n", scope, args[1], args[2])
		return 0, nil
	case "insert":
		return editInsert(args, hostAlias, out)
	case "delete":
		return editDelete(args, hostAlias, out)
	case "line":
		return editLine(args, hostAlias, out)
	default:
		return 1, fmt.Errorf("unknown edit command: %s", kind)
	}
}

func editInsert(args []string, hostAlias string, out Output) (int, error) {
	if len(args) < 3 {
		return 1, errors.New("insert requires PATH LINE CONTENT")
	}
	after := contains(args[3:], "--after")
	line, err := strconv.Atoi(args[1])
	if err != nil {
		return 1, err
	}
	position := "前"
	if after {
		position = "后"
	}
	code, err := runEditScript("insert", args[0], map[string]any{
		"line": line, "content": args[2], "after": after,
	}, hostAlias, out)
	if code != 0 || err != nil {
		return code, err
	}
	fmt.Fprintf(out.Out, "已在第 %d 行%s插入内容\n", line, position)
	return 0, nil
}

func editDelete(args []string, hostAlias string, out Output) (int, error) {
	if len(args) < 2 {
		return 1, errors.New("delete requires PATH START [END]")
	}
	start, err := strconv.Atoi(args[1])
	if err != nil {
		return 1, err
	}
	lineRange := strconv.Itoa(start)
	if len(args) > 2 {
		end, err := strconv.Atoi(args[2])
		if err != nil {
			return 1, err
		}
		lineRange = fmt.Sprintf("%d,%d", start, end)
	}
	code, err := runEditScript("delete", args[0], map[string]any{
		"start": start, "end": lineRange,
	}, hostAlias, out)
	if code != 0 || err != nil {
		return code, err
	}
	if len(args) > 2 {
		fmt.Fprintf(out.Out, "已删除第 %s 行\n", strings.ReplaceAll(lineRange, ",", "-"))
	} else {
		fmt.Fprintf(out.Out, "已删除第 %d 行\n", start)
	}
	return 0, nil
}

func editLine(args []string, hostAlias string, out Output) (int, error) {
	if len(args) < 3 {
		return 1, errors.New("line requires PATH NUM CONTENT")
	}
	line, err := strconv.Atoi(args[1])
	if err != nil {
		return 1, err
	}
	code, err := runEditScript("line", args[0], map[string]any{
		"line": line, "content": args[2],
	}, hostAlias, out)
	if code != 0 || err != nil {
		return code, err
	}
	fmt.Fprintf(out.Out, "已修改第 %d 行\n", line)
	return 0, nil
}

func runEditScript(action, path string, params map[string]any, hostAlias string, out Output) (int, error) {
	paramsJSON, _ := json.Marshal(params)
	cmd := buildEditCmd(action, path, string(paramsJSON))
	result, err := ssh.RunCommand(cmd, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	return 0, nil
}

func buildEditCmd(action, path, paramsJSON string) string {
	script := fmt.Sprintf(`
import json
import os
from pathlib import Path

action = %s
path = %s
params = json.loads(%s)
target = Path(os.path.expanduser(path))
text = target.read_text()

if action == "replace":
    count = -1 if params["all"] else 1
    text = text.replace(params["old"], params["new"], count)
elif action == "insert":
    lines = text.splitlines(keepends=True)
    line = int(params["line"])
    if 1 <= line <= len(lines):
        index = line if params["after"] else line - 1
        content = params["content"]
        if not content.endswith("\n"):
            content += "\n"
        lines.insert(index, content)
        text = "".join(lines)
elif action == "delete":
    lines = text.splitlines(keepends=True)
    start = int(params["start"])
    raw_end = str(params["end"])
    end = int(raw_end.split(",", 1)[1]) if "," in raw_end else start
    if start <= len(lines) and end >= start:
        del lines[max(start - 1, 0):min(end, len(lines))]
        text = "".join(lines)
elif action == "line":
    lines = text.splitlines(keepends=True)
    line = int(params["line"])
    if 1 <= line <= len(lines):
        newline = "\n" if lines[line - 1].endswith("\n") else ""
        lines[line - 1] = params["content"] + newline
        text = "".join(lines)
target.write_text(text)
`, pyString(action), pyString(path), pyString(paramsJSON))
	return strings.Join([]string{"python3 - <<'PY'", strings.TrimSpace(script), "PY"}, "\n")
}

func Diff(file1, file2, hostAlias string, local bool, out Output) (int, error) {
	if local {
		tmp, err := os.CreateTemp("", "dev-connect-*.tmp")
		if err != nil {
			return 1, err
		}
		tmp.Close()
		defer os.Remove(tmp.Name())
		result, err := ssh.Download(file1, tmp.Name(), hostAlias, 60)
		if err != nil {
			return 1, err
		}
		if !result.Success() {
			fmt.Fprintf(out.Err, "错误: 下载远程文件失败: %s", result.Stderr)
			return 1, nil
		}
		diff, _ := exec.Command("diff", "-u", tmp.Name(), file2).CombinedOutput()
		text := string(diff)
		if out.JSON {
			return 0, out.PrintJSON(map[string]any{"remote_file": file1, "local_file": file2, "diff": text, "has_changes": text != ""})
		}
		if text == "" {
			fmt.Fprintln(out.Out, "文件相同")
		} else {
			fmt.Fprint(out.Out, text)
		}
		return 0, nil
	}
	result, err := ssh.RunCommand(fmt.Sprintf("diff -u %s %s || true", ssh.ExpandTilde(file1), ssh.ExpandTilde(file2)), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	if !result.Success() {
		fmt.Fprintf(out.Err, "错误: %s", result.Stderr)
		return 1, nil
	}
	if out.JSON {
		return 0, out.PrintJSON(map[string]any{"file1": file1, "file2": file2, "diff": result.Stdout, "has_changes": result.Stdout != ""})
	}
	if result.Stdout == "" {
		fmt.Fprintln(out.Out, "文件相同")
	} else {
		fmt.Fprint(out.Out, result.Stdout)
	}
	return 0, nil
}

func RepoDiff(cwd, hostAlias string, stat, cached, nameOnly bool, out Output) (int, error) {
	if stat && nameOnly {
		return 1, errors.New("--stat 和 --name-only 不能同时使用")
	}
	args := []string{"git", "diff"}
	if cached {
		args = append(args, "--cached")
	}
	if stat {
		args = append(args, "--stat")
	}
	if nameOnly {
		args = append(args, "--name-only")
	}
	cmd := strings.Join([]string{"set -e", "cd " + ssh.QuoteRemotePath(cwd), strings.Join(args, " ")}, "\n")
	result, err := ssh.RunCommand(cmd, hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	payload := map[string]any{"cwd": cwd, "cached": cached, "stat": stat, "name_only": nameOnly, "returncode": result.ReturnCode, "stdout": result.Stdout, "stderr": result.Stderr, "success": result.Success()}
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		fmt.Fprint(out.Out, result.Stdout)
		fmt.Fprint(out.Err, result.Stderr)
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func RepoResolve(repo, hostAlias string, out Output) (int, error) {
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return 1, err
	}
	result, err := ssh.RunCommand(buildRepoResolveCmd(repo, host.RepoRoots, host.User), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	payload := parseJSONResult(result.Stdout, repo)
	stderr := filterSSHNoise(result.Stderr)
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		if ok, _ := payload["success"].(bool); ok {
			fmt.Fprintln(out.Out, payload["path"])
		} else {
			fmt.Fprintln(out.Err, firstNonEmpty(fmt.Sprint(payload["error"]), "repo not found"))
			if roots, ok := payload["searched_roots"].([]any); ok && len(roots) > 0 {
				fmt.Fprintf(out.Err, "searched roots: %s\n", joinAny(roots, ", "))
			}
		}
		if stderr != "" {
			fmt.Fprint(out.Err, stderr)
		}
	}
	if ok, _ := payload["success"].(bool); !ok {
		return 1, nil
	}
	return 0, nil
}

func buildRepoResolveCmd(repo string, repoRoots []string, user string) string {
	roots := dedupe(append(repoRoots, defaultRepoRoots(user)...))
	rawRoots, _ := json.Marshal(roots)
	script := `
import json
import os
import subprocess
import sys
from pathlib import Path
repo = sys.argv[1].strip("/")
roots = json.loads(sys.argv[2])
if repo.startswith("/") or repo.startswith("~/"):
    path = Path(os.path.expanduser(repo))
    print(json.dumps({"success": path.exists(), "input": repo, "path": str(path) if path.exists() else "", "searched_roots": [], "source": "direct", "error": "" if path.exists() else "path does not exist"}, ensure_ascii=False))
    sys.exit(0 if path.exists() else 1)
expanded_roots = []
for root in roots:
    expanded = os.path.expanduser(root)
    if expanded not in expanded_roots:
        expanded_roots.append(expanded)
for root in expanded_roots:
    candidate = Path(root) / repo
    if candidate.exists():
        print(json.dumps({"success": True, "input": repo, "path": str(candidate), "searched_roots": expanded_roots, "source": "root"}, ensure_ascii=False))
        sys.exit(0)
parts = repo.split("/")
suffix = os.path.join(*parts[-2:]) if len(parts) >= 2 else repo
for root in expanded_roots:
    if not Path(root).exists():
        continue
    find = subprocess.run(["find", root, "-maxdepth", "4", "-type", "d", "-path", f"*/{suffix}"], capture_output=True, text=True, timeout=10)
    matches = [line for line in find.stdout.splitlines() if line]
    if matches:
        print(json.dumps({"success": True, "input": repo, "path": matches[0], "searched_roots": expanded_roots, "source": "search", "matches": matches[:10]}, ensure_ascii=False))
        sys.exit(0)
print(json.dumps({"success": False, "input": repo, "path": "", "searched_roots": expanded_roots, "source": "search", "error": "repo not found"}, ensure_ascii=False))
sys.exit(1)
`
	return strings.Join([]string{
		"python3 - " + ssh.QuoteRemotePath(repo) + " " + ssh.QuoteRemotePath(string(rawRoots)) + " <<'PY'",
		strings.TrimSpace(script),
		"PY",
	}, "\n")
}

func RepoStatus(cwd, hostAlias string, out Output) (int, error) {
	result, err := ssh.RunCommand(buildRepoStatusCmd(cwd), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	payload := parseJSONResult(result.Stdout, cwd)
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		printRepoStatus(payload, out)
		if result.Stderr != "" {
			fmt.Fprint(out.Err, result.Stderr)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func buildRepoStatusCmd(cwd string) string {
	script := `
import json
import subprocess
import sys
from pathlib import Path
def run(args):
    r = subprocess.run(args, capture_output=True, text=True)
    return {"returncode": r.returncode, "stdout": r.stdout, "stderr": r.stderr}
def parse_status(text):
    staged, unstaged, untracked, entries = [], [], [], []
    for line in text.splitlines():
        if not line or line.startswith("## "):
            continue
        code = line[:2]
        raw_path = line[3:]
        path = raw_path.split(" -> ", 1)[-1] if " -> " in raw_path else raw_path
        entry = {"code": code, "path": path}
        entries.append(entry)
        if code == "??":
            untracked.append(entry)
            continue
        if code[0] != " ": staged.append(entry)
        if code[1] != " ": unstaged.append(entry)
    return entries, staged, unstaged, untracked
def collect_ahead_behind():
    upstream = run(["git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"])
    if upstream["returncode"] != 0:
        return "", {"ahead": None, "behind": None}, ""
    counts = run(["git", "rev-list", "--left-right", "--count", "@{u}...HEAD"])
    ahead_behind = {"ahead": None, "behind": None}
    if counts["returncode"] == 0:
        parts = counts["stdout"].split()
        if len(parts) == 2:
            ahead_behind = {"behind": int(parts[0]), "ahead": int(parts[1])}
    unpushed = run(["git", "log", "--oneline", "@{u}..HEAD"])
    return upstream["stdout"].strip(), ahead_behind, unpushed["stdout"] if unpushed["returncode"] == 0 else ""
def suspicious_reasons(path, code):
    reasons = []
    normalized = path.replace("\\", "/")
    name = normalized.rsplit("/", 1)[-1]
    build_dirs = ("node_modules/", "dist/", "build/", "coverage/", "target/", ".next/", ".turbo/")
    generated_suffixes = (".pb.go", ".gen.go", ".generated.go", "_generated.go", ".min.js", ".map")
    artifact_suffixes = (".log", ".tmp", ".out", ".test", ".o", ".so", ".dylib", ".class", ".pyc")
    if any(part in normalized for part in build_dirs):
        reasons.append("build-artifact")
    if name in {".DS_Store", "coverage.out"} or name.endswith(artifact_suffixes):
        reasons.append("artifact")
    if name.endswith(generated_suffixes):
        reasons.append("generated")
    size_bytes = None
    try:
        stat = Path(path).stat()
        size_bytes = stat.st_size
        if stat.st_size >= 5 * 1024 * 1024:
            reasons.append("large-file")
    except OSError:
        pass
    if code == "??" and reasons:
        reasons.insert(0, "untracked")
    return reasons, size_bytes
def collect_suspicious(entries):
    items = []
    for entry in entries:
        reasons, size_bytes = suspicious_reasons(entry["path"], entry["code"])
        if reasons:
            items.append({"path": entry["path"], "code": entry["code"], "reasons": reasons, "size_bytes": size_bytes})
    return items
top = run(["git", "rev-parse", "--show-toplevel"])
if top["returncode"] != 0:
    print(json.dumps({"success": False, "error": "not a git repository", "stderr": top["stderr"]}, ensure_ascii=False))
    sys.exit(top["returncode"])
branch = run(["git", "branch", "--show-current"])
status = run(["git", "status", "--short", "--branch"])
porcelain = run(["git", "status", "--porcelain"])
diff_stat = run(["git", "diff", "--stat"])
staged_diff_stat = run(["git", "diff", "--cached", "--stat"])
recent = run(["git", "log", "--oneline", "-5"])
entries, staged, unstaged, untracked = parse_status(porcelain["stdout"])
upstream, ahead_behind, unpushed = collect_ahead_behind()
payload = {"success": True, "repo_root": top["stdout"].strip(), "branch": branch["stdout"].strip(), "upstream": upstream, "ahead_behind": ahead_behind, "has_unpushed_commits": bool(unpushed.strip()), "unpushed_commits": unpushed, "dirty": any(line and not line.startswith("## ") for line in status["stdout"].splitlines()), "status": status["stdout"], "status_entries": entries, "staged": staged, "unstaged": unstaged, "untracked": untracked, "diff_stat": diff_stat["stdout"], "staged_diff_stat": staged_diff_stat["stdout"], "suspicious_files": collect_suspicious(entries), "recent_commits": recent["stdout"]}
print(json.dumps(payload, ensure_ascii=False))
`
	return strings.Join([]string{"set -e", "cd " + ssh.QuoteRemotePath(cwd), "python3 - <<'PY'", strings.TrimSpace(script), "PY"}, "\n")
}

func printRepoStatus(payload map[string]any, out Output) {
	if ok, _ := payload["success"].(bool); !ok {
		fmt.Fprintln(out.Err, firstNonEmpty(fmt.Sprint(payload["error"]), "status failed"))
		return
	}
	fmt.Fprintf(out.Out, "repo: %v\nbranch: %v\ndirty: %v\n", payload["repo_root"], payload["branch"], payload["dirty"])
	if s := fmt.Sprint(payload["status"]); s != "" {
		fmt.Fprintln(out.Out, "\nstatus:")
		fmt.Fprint(out.Out, s)
	}
	if s := fmt.Sprint(payload["diff_stat"]); s != "" {
		fmt.Fprintln(out.Out, "\ndiff stat:")
		fmt.Fprint(out.Out, s)
	}
}

func GitSnapshot(cwd, hostAlias string, out Output) (int, error) {
	result, err := ssh.RunCommand(buildGitSnapshotCmd(cwd), hostAlias, 30, "", "")
	if err != nil {
		return 1, err
	}
	payload := parseJSONResult(result.Stdout, cwd)
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		printGitSnapshot(payload, out)
		if result.Stderr != "" {
			fmt.Fprint(out.Err, result.Stderr)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func buildGitSnapshotCmd(cwd string) string {
	script := `
import json
import subprocess
import sys
def run(args):
    r = subprocess.run(args, capture_output=True, text=True)
    return {"returncode": r.returncode, "stdout": r.stdout, "stderr": r.stderr}
def split_lines(text):
    return [line for line in text.splitlines() if line]
top = run(["git", "rev-parse", "--show-toplevel"])
if top["returncode"] != 0:
    print(json.dumps({"success": False, "error": "not a git repository", "stderr": top["stderr"]}, ensure_ascii=False))
    sys.exit(top["returncode"])
branch = run(["git", "branch", "--show-current"])
head = run(["git", "rev-parse", "--short", "HEAD"])
head_subject = run(["git", "log", "-1", "--pretty=%s"])
status = run(["git", "status", "--short", "--branch"])
diff_stat = run(["git", "diff", "--stat"])
staged_diff_stat = run(["git", "diff", "--cached", "--stat"])
name_only = run(["git", "diff", "--name-only"])
staged_name_only = run(["git", "diff", "--cached", "--name-only"])
recent = run(["git", "log", "--oneline", "-5"])
payload = {"success": True, "repo_root": top["stdout"].strip(), "branch": branch["stdout"].strip(), "head": head["stdout"].strip(), "head_subject": head_subject["stdout"].strip(), "status": status["stdout"], "dirty": any(line and not line.startswith("## ") for line in status["stdout"].splitlines()), "diff_stat": diff_stat["stdout"], "staged_diff_stat": staged_diff_stat["stdout"], "changed_files": split_lines(name_only["stdout"]), "staged_changed_files": split_lines(staged_name_only["stdout"]), "recent_commits": recent["stdout"], "verification": "not_run"}
print(json.dumps(payload, ensure_ascii=False))
`
	return strings.Join([]string{"set -e", "cd " + ssh.QuoteRemotePath(cwd), "python3 - <<'PY'", strings.TrimSpace(script), "PY"}, "\n")
}

func printGitSnapshot(payload map[string]any, out Output) {
	if ok, _ := payload["success"].(bool); !ok {
		fmt.Fprintln(out.Err, firstNonEmpty(fmt.Sprint(payload["error"]), "snapshot failed"))
		return
	}
	fmt.Fprintf(out.Out, "repo: %v\nbranch: %v\nhead: %v %v\ndirty: %v\n", payload["repo_root"], payload["branch"], payload["head"], payload["head_subject"], payload["dirty"])
	for _, section := range []string{"status", "diff_stat", "staged_diff_stat"} {
		if s := fmt.Sprint(payload[section]); s != "" {
			fmt.Fprintf(out.Out, "\n%s:\n%s", strings.ReplaceAll(section, "_", " "), s)
		}
	}
	fmt.Fprintln(out.Out, "\nverification: not_run")
}

func VerifyGo(cwd string, changed bool, also []string, hostAlias string, timeout int, out Output) (int, error) {
	if !changed {
		return 1, errors.New("当前只支持 --changed")
	}
	result, err := ssh.RunCommand(buildVerifyGoCmd(cwd, also), hostAlias, timeout, "", "")
	if err != nil {
		return 1, err
	}
	payload := parseCommandPayload(result.Stdout, cwd, result.ReturnCode, result.Stderr)
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		if skipped, _ := payload["skipped"].(bool); skipped {
			fmt.Fprintf(out.Out, "verify go skipped: %v\n", payload["reason"])
		} else {
			fmt.Fprintf(out.Out, "verify go: %v\n", payload["command"])
			if pkgs, ok := payload["packages"].([]any); ok && len(pkgs) > 0 {
				fmt.Fprintln(out.Out, "packages:")
				for _, pkg := range pkgs {
					fmt.Fprintf(out.Out, "  %v\n", pkg)
				}
			}
			fmt.Fprint(out.Out, fmt.Sprint(payload["stdout"]))
		}
		if s := fmt.Sprint(payload["stderr"]); s != "" {
			fmt.Fprint(out.Err, s)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func buildVerifyGoCmd(cwd string, also []string) string {
	rawAlso, _ := json.Marshal(also)
	script := fmt.Sprintf(`
import json
import subprocess
import sys
def run(args):
    r = subprocess.run(args, capture_output=True, text=True)
    return {"returncode": r.returncode, "stdout": r.stdout, "stderr": r.stderr}
def lines(result):
    return [line for line in str(result["stdout"]).splitlines() if line.endswith(".go")]
def package_for(path):
    directory = path.rsplit("/", 1)[0] if "/" in path else "."
    return "." if directory in {"", "."} else "./" + directory
top = run(["git", "rev-parse", "--show-toplevel"])
if top["returncode"] != 0:
    print(json.dumps({"success": False, "error": "not a git repository", "stderr": top["stderr"]}, ensure_ascii=False))
    sys.exit(top["returncode"])
diff = run(["git", "diff", "--name-only", "--diff-filter=ACMR"])
cached = run(["git", "diff", "--cached", "--name-only", "--diff-filter=ACMR"])
untracked = run(["git", "ls-files", "--others", "--exclude-standard"])
also_packages = json.loads(%q)
changed_files = sorted(set(lines(diff) + lines(cached) + lines(untracked)))
packages = sorted({package_for(path) for path in changed_files} | set(also_packages))
if not packages:
    print(json.dumps({"success": True, "skipped": True, "reason": "no changed go files", "changed_files": [], "packages": [], "command": "", "returncode": 0, "stdout": "", "stderr": ""}, ensure_ascii=False))
    sys.exit(0)
go_list = run(["go", "list", *packages])
if go_list["returncode"] != 0:
    print(json.dumps({"success": False, "skipped": False, "changed_files": changed_files, "packages": packages, "command": "go list " + " ".join(packages), "returncode": go_list["returncode"], "stdout": go_list["stdout"], "stderr": go_list["stderr"]}, ensure_ascii=False))
    sys.exit(go_list["returncode"])
command = ["go", "test", *packages]
result = run(command)
print(json.dumps({"success": result["returncode"] == 0, "skipped": False, "changed_files": changed_files, "packages": packages, "command": " ".join(command), "returncode": result["returncode"], "stdout": result["stdout"], "stderr": result["stderr"]}, ensure_ascii=False))
sys.exit(result["returncode"])
`, string(rawAlso))
	return strings.Join([]string{"set -e", "cd " + ssh.QuoteRemotePath(cwd), "python3 - <<'PY'", strings.TrimSpace(script), "PY"}, "\n")
}

func ExecWatch(command, hostAlias string, interval, timeout int, shell string, summaryChars int, cwd string, out Output) (int, error) {
	if interval <= 0 {
		return 1, errors.New("--interval 必须大于 0")
	}
	if timeout <= 0 {
		return 1, errors.New("--timeout 必须大于 0")
	}
	host, err := config.GetHost(hostAlias)
	if err != nil {
		return 1, err
	}
	activeShell := host.Shell
	if shell == "none" {
		activeShell = ""
	} else if shell != "" {
		activeShell = shell
	}
	effectiveCommand := ssh.WrapRemoteCwd(command, cwd)
	remoteCommand := buildWatchCmd(effectiveCommand, interval, timeout, activeShell, summaryChars, cwd)
	args := ssh.BuildSSHCmd(host, remoteCommand)
	process := exec.Command(args[0], args[1:]...)
	stdout, err := process.StdoutPipe()
	if err != nil {
		return 1, err
	}
	stderr, err := process.StderrPipe()
	if err != nil {
		return 1, err
	}
	if err := process.Start(); err != nil {
		return 1, fmt.Errorf("执行失败: %w", err)
	}
	finalReturnCode := 1
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			fmt.Fprintln(out.Out, line)
			continue
		}
		if payload["event"] == "finished" {
			finalReturnCode = intFromAny(payload["returncode"])
		}
		outputWatchEvent(payload, out)
	}
	stderrRaw, _ := io.ReadAll(stderr)
	waitErr := process.Wait()
	if len(stderrRaw) > 0 {
		fmt.Fprint(out.Err, string(stderrRaw))
	}
	if waitErr != nil {
		if ee, ok := waitErr.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return 1, waitErr
	}
	if finalReturnCode != 0 {
		return finalReturnCode, nil
	}
	return 0, nil
}

func buildWatchCmd(command string, interval, timeout int, shell string, summaryChars int, cwd string) string {
	script := strings.NewReplacer(
		"__COMMAND__", pyString(command),
		"__INTERVAL__", strconv.Itoa(interval),
		"__TIMEOUT__", strconv.Itoa(timeout),
		"__SHELL__", pyStringOrNone(shell),
		"__SUMMARY_CHARS__", strconv.Itoa(summaryChars),
		"__CWD__", pyStringOrNone(cwd),
	).Replace(watchScript)
	return strings.Join([]string{"python3 - <<'PY'", script, "PY"}, "\n")
}

func outputWatchEvent(payload map[string]any, out Output) {
	if out.JSON {
		raw, _ := json.Marshal(payload)
		fmt.Fprintln(out.Out, string(raw))
		return
	}
	switch payload["event"] {
	case "started":
		fmt.Fprintf(out.Out, "started: %v\n", payload["command"])
	case "running":
		fmt.Fprintf(out.Out, "[%vs] running lines=%v last=%v\n", payload["elapsed_seconds"], payload["output_lines"], payload["last_line"])
	case "finished":
		status := "ok"
		if success, _ := payload["success"].(bool); !success {
			status = "failed"
		}
		if timedOut, _ := payload["timed_out"].(bool); timedOut {
			status = "timed out"
		}
		fmt.Fprintf(out.Out, "finished: %s rc=%v elapsed=%vs lines=%v\n", status, payload["returncode"], payload["elapsed_seconds"], payload["output_lines"])
		if output := fmt.Sprint(payload["output"]); output != "" {
			fmt.Fprint(out.Out, output)
		}
	}
}

const watchScript = `
from __future__ import annotations
import json
import os
import selectors
import subprocess
import time
def build_args(command, shell):
    if shell in {None, "", "none"}: return command
    if shell == "zsh": return ["zsh", "-ic", command]
    if shell == "zsh-login": return ["zsh", "-lic", command]
    if shell == "bash": return ["bash", "-ic", command]
    if shell == "bash-login": return ["bash", "-lic", command]
    return [shell, "-c", command]
def emit(payload):
    print(json.dumps(payload, ensure_ascii=False), flush=True)
command = __COMMAND__
interval = __INTERVAL__
timeout = __TIMEOUT__
shell = __SHELL__
summary_chars = __SUMMARY_CHARS__
cwd = __CWD__
start = time.monotonic()
next_tick = start + interval
output_parts = []
output_lines = 0
last_line = ""
timed_out = False
args = build_args(command, shell)
process = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, shell=isinstance(args, str))
selector = selectors.DefaultSelector()
if process.stdout is not None:
    selector.register(process.stdout, selectors.EVENT_READ)
emit({"event":"started","command":command,"cwd":cwd,"shell":shell,"pid":process.pid,"elapsed_seconds":0})
while True:
    now = time.monotonic()
    if timeout and now - start >= timeout and process.poll() is None:
        timed_out = True
        process.terminate()
        try: process.wait(timeout=5)
        except subprocess.TimeoutExpired: process.kill()
    wait_for = max(0.0, min(0.2, next_tick - now))
    for key, _ in selector.select(wait_for):
        chunk = os.read(key.fileobj.fileno(), 4096)
        if not chunk: continue
        text = chunk.decode(errors="replace")
        output_parts.append(text)
        output_lines += text.count("\n")
        for line in text.splitlines():
            if line.strip(): last_line = line.rstrip("\n")
    now = time.monotonic()
    if now >= next_tick and process.poll() is None:
        emit({"event":"running","elapsed_seconds":round(now-start,1),"output_lines":output_lines,"last_line":last_line})
        next_tick += interval
    if process.poll() is not None:
        if process.stdout is not None:
            while True:
                ready = selector.select(0)
                if not ready: break
                chunk = os.read(process.stdout.fileno(), 4096)
                if not chunk: break
                text = chunk.decode(errors="replace")
                output_parts.append(text)
                output_lines += text.count("\n")
                for line in text.splitlines():
                    if line.strip(): last_line = line.rstrip("\n")
        break
returncode = process.returncode
if timed_out: returncode = 124
output = "".join(output_parts)
truncated = False
if summary_chars > 0 and len(output) > summary_chars:
    output = output[-summary_chars:]
    truncated = True
emit({"event":"finished","command":command,"cwd":cwd,"shell":shell,"returncode":returncode,"success":returncode==0,"timed_out":timed_out,"elapsed_seconds":round(time.monotonic()-start,1),"output_lines":output_lines,"last_line":last_line,"output":output,"truncated":truncated})
raise SystemExit(returncode)
`

func Patch(cwd, hostAlias string, checkOnly bool, out Output) (int, error) {
	raw, err := io.ReadAll(out.In)
	if err != nil {
		return 1, err
	}
	result, err := ssh.RunCommand(buildPatchCmd(cwd, checkOnly), hostAlias, 120, "", string(raw))
	if err != nil {
		return 1, err
	}
	payload := parseJSONResult(result.Stdout, cwd)
	payload["cwd"] = cwd
	payload["check_only"] = checkOnly
	payload["returncode"] = result.ReturnCode
	payload["stdout"] = result.Stdout
	payload["stderr"] = result.Stderr
	if out.JSON {
		_ = out.PrintJSON(payload)
	} else {
		if ok, _ := payload["success"].(bool); !ok {
			if path := fmt.Sprint(payload["path"]); path != "" && path != "<nil>" {
				fmt.Fprintf(out.Err, "patch failed: %s: %v\n", path, payload["error"])
			} else {
				fmt.Fprintf(out.Err, "patch failed: %v\n", payload["error"])
			}
			printPatchFailureDetails(payload, out)
		} else {
			action := "applied"
			if checkOnly {
				action = "checked"
			}
			fmt.Fprintf(out.Out, "patch %s: %s\n", action, cwd)
			printChangedFiles(payload, out)
			if s := fmt.Sprint(payload["git_diff_stat"]); s != "" {
				fmt.Fprintln(out.Out, "git diff stat:")
				fmt.Fprint(out.Out, s)
			} else if s := fmt.Sprint(payload["patch_stat"]); s != "" {
				label := "patch stat"
				if checkOnly {
					label += " (check-only, not git diff)"
				}
				fmt.Fprintf(out.Out, "%s:\n", label)
				fmt.Fprint(out.Out, s)
			}
		}
		if result.Stderr != "" {
			fmt.Fprint(out.Err, result.Stderr)
		}
	}
	if !result.Success() {
		return result.ReturnCode, nil
	}
	return 0, nil
}

func printChangedFiles(payload map[string]any, out Output) {
	files, ok := payload["changed_files"].([]any)
	if !ok || len(files) == 0 {
		return
	}
	fmt.Fprintln(out.Out, "changed files:")
	for _, raw := range files {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		suffix := ""
		if action := fmt.Sprint(item["action"]); action != "" && action != "<nil>" {
			suffix = " [" + action + "]"
		}
		fmt.Fprintf(out.Out, "  %v%s\n", item["path"], suffix)
	}
}

func printPatchFailureDetails(payload map[string]any, out Output) {
	details, ok := payload["details"].(map[string]any)
	if !ok || len(details) == 0 {
		return
	}
	if hunk := details["hunk_index"]; hunk != nil {
		fmt.Fprintf(out.Err, "hunk: %v\n", hunk)
	}
	if lines, ok := details["match_lines"].([]any); ok && len(lines) > 0 {
		fmt.Fprintf(out.Err, "matched lines: %v\n", lines)
	}
	candidates, ok := details["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return
	}
	fmt.Fprintln(out.Err, "similar candidates:")
	for _, raw := range candidates {
		candidate, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		fmt.Fprintf(out.Err, "  line %v, score %v\n", candidate["start_line"], candidate["score"])
		if snippet, ok := candidate["snippet"].([]any); ok {
			for _, line := range snippet {
				fmt.Fprintf(out.Err, "    %v\n", line)
			}
		}
	}
}

func buildPatchCmd(cwd string, checkOnly bool) string {
	checkArg := ""
	if checkOnly {
		checkArg = " --check"
	}
	return strings.Join([]string{
		"set -e",
		"cd " + ssh.QuoteRemotePath(cwd),
		`patch_file="$(mktemp)"`,
		`applier_file="$(mktemp)"`,
		`trap 'rm -f "$patch_file" "$applier_file"' EXIT`,
		`cat > "$patch_file"`,
		`cat > "$applier_file" <<'PYAPPLIER'`,
		patchApplierScript,
		"PYAPPLIER",
		`python3 "$applier_file" "$PWD" "$patch_file"` + checkArg,
	}, "\n")
}

const patchApplierScript = `
from __future__ import annotations
import difflib, json, posixpath, subprocess, sys
from pathlib import Path
class PatchError(Exception):
    def __init__(self, message, path=None, details=None):
        super().__init__(message); self.path = path; self.details = details or {}
def norm(path):
    if not path or path.startswith("/"): raise PatchError(f"unsafe path: {path}")
    n = posixpath.normpath(path)
    if n in {"", "."} or n.startswith("../") or ".." in n.split("/") or n.split("/")[0] == ".git":
        raise PatchError(f"unsafe path: {path}")
    return n
def is_action(line):
    return line.startswith("*** Add File: ") or line.startswith("*** Update File: ") or line.startswith("*** Delete File: ") or line.startswith("*** End Patch")
def parse(text):
    lines = text.splitlines()
    if not lines or lines[0] != "*** Begin Patch": raise PatchError("patch must start with '*** Begin Patch'")
    if lines[-1] != "*** End Patch": raise PatchError("patch must end with '*** End Patch'")
    ops=[]; i=1
    while i < len(lines)-1:
        line=lines[i]
        if line.startswith("*** Add File: "):
            path=norm(line.removeprefix("*** Add File: ")); i+=1; content=[]
            while i < len(lines)-1 and not is_action(lines[i]):
                if not lines[i].startswith("+"): raise PatchError("add-file lines must start with '+'", path)
                content.append(lines[i][1:]+"\n"); i+=1
            ops.append(("add", path, content)); continue
        if line.startswith("*** Delete File: "):
            ops.append(("delete", norm(line.removeprefix("*** Delete File: ")), None)); i+=1; continue
        if line.startswith("*** Update File: "):
            path=norm(line.removeprefix("*** Update File: ")); i+=1; hunks=[]
            while i < len(lines)-1 and not is_action(lines[i]):
                if not lines[i].startswith("@@"): raise PatchError("update hunk must start with '@@'", path)
                i+=1; h=[]
                while i < len(lines)-1 and not lines[i].startswith("@@") and not is_action(lines[i]):
                    if lines[i][:1] not in {" ","-","+"}: raise PatchError("update lines must start with ' ', '-' or '+'", path)
                    h.append((lines[i][0], lines[i][1:]+"\n")); i+=1
                if not h: raise PatchError("empty update hunk", path)
                hunks.append(h)
            ops.append(("update", path, hunks)); continue
        raise PatchError(f"unknown patch directive: {line}")
    if not ops: raise PatchError("patch contains no operations")
    return ops
def trim_lines(lines, limit=8):
    visible = [line.rstrip("\n") for line in lines[:limit]]
    if len(lines) > limit: visible.append("...")
    return visible
def similar_windows(lines, needle):
    if not needle or not lines: return []
    width = len(needle); expected = "".join(needle); out = []
    for start in range(max(len(lines)-width, 0)+1):
        window = lines[start:start+width]
        score = difflib.SequenceMatcher(None, expected, "".join(window)).ratio()
        out.append({"start_line": start+1, "score": round(score, 3), "snippet": trim_lines(window)})
    out.sort(key=lambda item: item["score"], reverse=True)
    return out[:3]
def apply_hunk(lines, hunk, path, hunk_index):
    old=[v for m,v in hunk if m in {" ","-"}]; new=[v for m,v in hunk if m in {" ","+"}]
    matches=[i for i in range(len(lines)-len(old)+1) if lines[i:i+len(old)] == old]
    if not matches:
        raise PatchError("hunk context did not match", path, {"hunk_index": hunk_index, "expected": trim_lines(old), "candidates": similar_windows(lines, old)})
    if len(matches)>1:
        raise PatchError("hunk context matched multiple locations", path, {"hunk_index": hunk_index, "match_lines": [m+1 for m in matches[:10]], "match_count": len(matches)})
    i=matches[0]; return lines[:i]+new+lines[i+len(old):]
def count_changed(before, after):
    prefix = 0; max_prefix = min(len(before), len(after))
    while prefix < max_prefix and before[prefix] == after[prefix]: prefix += 1
    before_suffix = len(before); after_suffix = len(after)
    while before_suffix > prefix and after_suffix > prefix and before[before_suffix-1] == after[after_suffix-1]:
        before_suffix -= 1; after_suffix -= 1
    return after_suffix - prefix, before_suffix - prefix
def patch_stat(changed):
    lines=[]; total_add=0; total_del=0
    for item in changed:
        add=int(item["additions"]); delete=int(item["deletions"]); total_add += add; total_del += delete
        lines.append(f" {item['path']} | {add+delete} {'+'*min(add,30)}{'-'*min(delete,30)}")
    if changed:
        lines.append(f" {len(changed)} files changed, {total_add} insertions(+), {total_del} deletions(-)")
    return "\n".join(lines) + ("\n" if lines else "")
def git_diff_stat(repo, changed):
    paths=[str(item["path"]) for item in changed]
    if not paths: return ""
    result=subprocess.run(["git","diff","--stat","--",*paths], cwd=repo, capture_output=True, text=True)
    return result.stdout if result.returncode == 0 else ""
def prune_empty_dirs(path, repo):
    while path != repo:
        try: path.rmdir()
        except OSError: return
        path = path.parent
def main():
    repo=Path(sys.argv[1]).resolve(); patch=Path(sys.argv[2]).read_text(); check=len(sys.argv)>3 and sys.argv[3]=="--check"
    changed=[]; pending={}; touched=set()
    try:
        for action,path,data in parse(patch):
            if path in touched: raise PatchError("multiple operations for one file are not supported", path)
            touched.add(path)
            target=(repo/path).resolve()
            try: target.relative_to(repo)
            except ValueError: raise PatchError("path escapes repository", path)
            if action=="add":
                if target.exists(): raise PatchError("add target already exists", path)
                pending[path]=data; changed.append({"path":path,"action":"add","additions":len(data),"deletions":0})
            elif action=="delete":
                if not target.is_file(): raise PatchError("delete target does not exist or is not a file", path)
                lines=target.read_text().splitlines(keepends=True); pending[path]=None; changed.append({"path":path,"action":"delete","additions":0,"deletions":len(lines)})
            else:
                if not target.is_file(): raise PatchError("update target does not exist or is not a file", path)
                before=target.read_text().splitlines(keepends=True); lines=before
                for hunk_index,h in enumerate(data, start=1): lines=apply_hunk(lines,h,path,hunk_index)
                additions,deletions=count_changed(before,lines)
                pending[path]=lines; changed.append({"path":path,"action":"update","additions":additions,"deletions":deletions})
        if not check:
            for path,lines in pending.items():
                target=repo/path
                if lines is None:
                    target.unlink(); prune_empty_dirs(target.parent, repo)
                else: target.parent.mkdir(parents=True, exist_ok=True); target.write_text("".join(lines))
        pstat=patch_stat(changed); gstat="" if check else git_diff_stat(repo, changed)
        print(json.dumps({"success":True,"applied":not check,"changed_files":changed,"patch_stat":pstat,"git_diff_stat":gstat,"diff_stat":gstat or pstat}, ensure_ascii=False)); return 0
    except PatchError as e:
        print(json.dumps({"success":False,"applied":False,"changed_files":[],"patch_stat":"","git_diff_stat":"","diff_stat":"","error":str(e),"path":e.path,"details":e.details}, ensure_ascii=False)); return 1
if __name__ == "__main__": raise SystemExit(main())
`

func ConfigShow(out Output) (int, error) {
	cfg, err := config.Load()
	if err != nil {
		return 1, err
	}
	if out.JSON {
		return 0, out.PrintJSON(configJSONPayload(cfg))
	}
	fmt.Fprintf(out.Out, "默认主机: %s\n", firstNonEmpty(cfg.DefaultHost, "(未设置)"))
	fmt.Fprintln(out.Out, "\n已配置主机:")
	names := make([]string, 0, len(cfg.Hosts))
	for alias := range cfg.Hosts {
		names = append(names, alias)
	}
	sort.Strings(names)
	for _, alias := range names {
		host := cfg.Hosts[alias]
		extras := []string{}
		if host.Shell != "" {
			extras = append(extras, "shell="+host.Shell)
		}
		if host.ExecTimeout != nil {
			extras = append(extras, fmt.Sprintf("exec_timeout=%d", *host.ExecTimeout))
		}
		if len(host.RepoRoots) > 0 {
			extras = append(extras, fmt.Sprintf("repo_roots=%v", host.RepoRoots))
		}
		suffix := ""
		if len(extras) > 0 {
			suffix = " (" + strings.Join(extras, ", ") + ")"
		}
		fmt.Fprintf(out.Out, "  %s: %s@%s%s\n", alias, host.User, host.Hostname, suffix)
	}
	return 0, nil
}

func configJSONPayload(cfg model.AppConfig) map[string]any {
	hosts := map[string]any{}
	for alias, host := range cfg.Hosts {
		repoRoots := host.RepoRoots
		if repoRoots == nil {
			repoRoots = []string{}
		}
		var shell any
		if host.Shell == "" {
			shell = nil
		} else {
			shell = host.Shell
		}
		var execTimeout any
		if host.ExecTimeout != nil {
			execTimeout = *host.ExecTimeout
		}
		hosts[alias] = map[string]any{
			"hostname":     host.Hostname,
			"user":         firstNonEmpty(host.User, "maifeng"),
			"shell":        shell,
			"exec_timeout": execTimeout,
			"repo_roots":   repoRoots,
		}
	}
	return map[string]any{
		"default_host": cfg.DefaultHost,
		"hosts":        hosts,
	}
}

func ConfigAdd(alias, hostname, user, shell string, execTimeout *int, repoRoots []string, setDefault bool, out Output) (int, error) {
	cfg, err := config.Load()
	if err != nil {
		return 1, err
	}
	if user == "" {
		user = "maifeng"
	}
	if shell == "none" {
		shell = ""
	}
	cfg.Hosts[alias] = model.HostConfig{Hostname: hostname, User: user, Shell: shell, ExecTimeout: execTimeout, RepoRoots: repoRoots}
	if setDefault {
		cfg.DefaultHost = alias
	}
	if err := config.Save(cfg); err != nil {
		return 1, err
	}
	fmt.Fprintf(out.Out, "已添加主机: %s (%s@%s)\n", alias, user, hostname)
	if setDefault {
		fmt.Fprintln(out.Out, "已设为默认主机")
	}
	return 0, nil
}

func ConfigSet(alias, field, value string, out Output) (int, error) {
	cfg, err := config.Load()
	if err != nil {
		return 1, err
	}
	host, ok := cfg.Hosts[alias]
	if !ok {
		fmt.Fprintf(out.Err, "错误: 主机 '%s' 未配置\n", alias)
		return 1, nil
	}
	switch field {
	case "shell":
		if value == "none" {
			host.Shell = ""
			fmt.Fprintf(out.Out, "已清除 %s 的 dev exec 默认 shell\n", alias)
		} else {
			host.Shell = value
			fmt.Fprintf(out.Out, "已设置 %s 的 dev exec 默认 shell: %s\n", alias, value)
		}
	case "exec-timeout":
		n, err := strconv.Atoi(value)
		if err != nil {
			return 1, err
		}
		if n <= 0 {
			host.ExecTimeout = nil
			fmt.Fprintf(out.Out, "已清除 %s 的 dev exec 默认超时\n", alias)
		} else {
			host.ExecTimeout = &n
			fmt.Fprintf(out.Out, "已设置 %s 的 dev exec 默认超时: %d 秒\n", alias, n)
		}
	case "default":
		cfg.DefaultHost = alias
		fmt.Fprintf(out.Out, "已设置默认主机: %s\n", alias)
	case "clear-repo-roots":
		host.RepoRoots = nil
		fmt.Fprintf(out.Out, "已清空 %s 的 repo roots\n", alias)
	default:
		return 1, fmt.Errorf("unknown config field: %s", field)
	}
	cfg.Hosts[alias] = host
	return 0, config.Save(cfg)
}

func ConfigAddRepoRoot(alias, root string, out Output) (int, error) {
	cfg, err := config.Load()
	if err != nil {
		return 1, err
	}
	host, ok := cfg.Hosts[alias]
	if !ok {
		fmt.Fprintf(out.Err, "错误: 主机 '%s' 未配置\n", alias)
		return 1, nil
	}
	if !contains(host.RepoRoots, root) {
		host.RepoRoots = append(host.RepoRoots, root)
		cfg.Hosts[alias] = host
		if err := config.Save(cfg); err != nil {
			return 1, err
		}
	}
	fmt.Fprintf(out.Out, "已添加 %s 的 repo root: %s\n", alias, root)
	return 0, nil
}

func Stats(out Output) (int, error) {
	data := stats.Load()
	if out.JSON {
		return 0, out.PrintJSON(data)
	}
	if len(data) == 0 {
		fmt.Fprintln(out.Out, "暂无使用记录")
		return 0, nil
	}
	type row struct {
		name string
		info stats.Entry
	}
	rows := make([]row, 0, len(data))
	total, maxCount := 0, 0
	for name, info := range data {
		rows = append(rows, row{name, info})
		total += info.Count
		if info.Count > maxCount {
			maxCount = info.Count
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].info.Count > rows[j].info.Count })
	fmt.Fprintf(out.Out, "命令使用统计 (共 %d 次):\n\n", total)
	for _, row := range rows {
		pct := float64(row.info.Count) / float64(total) * 100
		barLen := 0
		if maxCount > 0 {
			barLen = int(float64(row.info.Count)/float64(maxCount)*25 + 0.5)
		}
		fmt.Fprintf(out.Out, "  %-16s %4d 次 (%4.1f%%)  %s\n", row.name, row.info.Count, pct, strings.Repeat("█", barLen))
	}
	return 0, nil
}

func fallbackCode(code int) int {
	if code == 0 {
		return 1
	}
	return code
}

func pyStringOrNone(s string) string {
	if s == "" {
		return "None"
	}
	return pyString(s)
}

func pyString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	default:
		return 0
	}
}

func sedEscape(s string) string {
	s = strings.ReplaceAll(s, `/`, `\/`)
	return strings.ReplaceAll(s, `&`, `\&`)
}

func mapStrings(in []string, fn func(string) string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func parseJSONResult(stdout, fallback string) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err == nil && payload != nil {
		return payload
	}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isSSHNoiseLine(line) {
			continue
		}
		payload = nil
		if err := json.Unmarshal([]byte(line), &payload); err == nil && payload != nil {
			return payload
		}
	}
	return map[string]any{"success": false, "target": fallback, "stdout": stdout}
}

func parseCommandPayload(stdout, cwd string, returnCode int, stderr string) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err == nil && payload != nil {
		if _, ok := payload["cwd"]; !ok {
			payload["cwd"] = cwd
		}
		return payload
	}
	return map[string]any{
		"success":    returnCode == 0,
		"cwd":        cwd,
		"returncode": returnCode,
		"stdout":     stdout,
		"stderr":     stderr,
	}
}

func filterSSHNoise(stderr string) string {
	var out strings.Builder
	for _, line := range strings.SplitAfter(stderr, "\n") {
		if isSSHNoiseLine(strings.TrimSpace(line)) {
			continue
		}
		out.WriteString(line)
	}
	return out.String()
}

func isSSHNoiseLine(line string) bool {
	return strings.Contains(line, "ControlSocket") && strings.Contains(line, "already exists, disabling multiplexing")
}

func joinAny(values []any, sep string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprint(value))
	}
	return strings.Join(parts, sep)
}

func defaultRepoRoots(user string) []string {
	return []string{
		"~/go/src/code.byted.org",
		"/home/" + user + "/go/src/code.byted.org",
		"/data00/home/" + user + "/go/src/code.byted.org",
	}
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func ReadAllLines(r io.Reader) []string {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func BufferOutput(json bool) (Output, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return Output{JSON: json, Out: &stdout, Err: &stderr, In: strings.NewReader("")}, &stdout, &stderr
}
