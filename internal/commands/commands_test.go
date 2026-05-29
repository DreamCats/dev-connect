package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DreamCats/dev-connect/internal/model"
)

func TestParseGrepOutputGroupsContext(t *testing.T) {
	raw := "a.py-1-before\na.py:2:match\na.py-3-after\n--\nb.py:10:hit\n"
	matches, files := parseGrepOutput(raw, nil)
	if len(matches) != 2 || len(files) != 2 {
		t.Fatalf("matches=%#v files=%#v", matches, files)
	}
	if matches[0]["line"] != 2 || matches[0]["content"] != "match" {
		t.Fatalf("unexpected first match: %#v", matches[0])
	}
}

func TestBuildGrepCmdQuotesInputs(t *testing.T) {
	max := 3
	got := buildGrepCmd("hello world", "~/repo", true, "*.go", true, 2, &max)
	want := "rg -n -C 2 -m 3 --glob '*.go' 'hello world' ~/repo"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildWatchCmdContainsEventScript(t *testing.T) {
	got := buildWatchCmd("go test ./...", 2, 30, "zsh", 1000, "~/repo")
	for _, want := range []string{`"event":"started"`, `"event":"running"`, `"event":"finished"`, `shell = "zsh"`, `cwd = "~/repo"`} {
		if !containsString(got, want) {
			t.Fatalf("watch cmd missing %q in:\n%s", want, got)
		}
	}
}

func TestPatchApplierReportsDetails(t *testing.T) {
	for _, want := range []string{"hunk_index", "candidates", "git_diff_stat", "multiple operations for one file"} {
		if !containsString(patchApplierScript, want) {
			t.Fatalf("patch applier missing %q", want)
		}
	}
}

func TestPatchApplierAddUpdateDelete(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "app.py"), []byte("def main():\n    return 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "old.txt"), []byte("delete me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := runPatchApplier(t, repo, `*** Begin Patch
*** Add File: src/new.py
+value = 1
*** Update File: src/app.py
@@
 def main():
-    return 1
+    return 2
*** Delete File: old.txt
*** End Patch
`, false)
	if payload["success"] != true {
		t.Fatalf("patch failed: %#v", payload)
	}
	raw, _ := os.ReadFile(filepath.Join(repo, "src", "app.py"))
	if string(raw) != "def main():\n    return 2\n" {
		t.Fatalf("unexpected app.py: %q", raw)
	}
	if _, err := os.Stat(filepath.Join(repo, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old.txt should be deleted, stat err=%v", err)
	}
}

func runPatchApplier(t *testing.T, repo, patch string, check bool) map[string]any {
	t.Helper()
	dir := t.TempDir()
	applier := filepath.Join(dir, "applier.py")
	patchFile := filepath.Join(dir, "change.patch")
	if err := os.WriteFile(applier, []byte(patchApplierScript), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patchFile, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{applier, repo, patchFile}
	if check {
		args = append(args, "--check")
	}
	out, err := exec.Command("python3", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("applier failed: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("invalid json %q: %v", out, err)
	}
	return payload
}

func TestConfigJSONPayloadKeepsEmptyFields(t *testing.T) {
	payload := configJSONPayload(model.AppConfig{
		DefaultHost: "sgdev",
		Hosts: map[string]model.HostConfig{
			"sgdev": {Hostname: "10.0.0.1", User: "maifeng"},
		},
	})
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"shell":null`, `"exec_timeout":null`, `"repo_roots":[]`} {
		if !strings.Contains(text, want) {
			t.Fatalf("payload missing %s: %s", want, text)
		}
	}
}

func TestParseCommandPayloadFallbackMatchesExecSchema(t *testing.T) {
	payload := parseCommandPayload("not json", "/repo", 7, "boom")
	for _, key := range []string{"success", "cwd", "returncode", "stdout", "stderr"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %s in %#v", key, payload)
		}
	}
	if payload["success"] != false || payload["returncode"] != 7 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestBuildEditCmdUsesStructuredScript(t *testing.T) {
	got := buildEditCmd("replace", "~/dir with space/a'b.txt", `{"old":"a/b","new":"c'd","all":false}`)
	for _, want := range []string{"Path(os.path.expanduser(path))", "text.replace", `path = "~/dir with space/a'b.txt"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("edit command missing %q in:\n%s", want, got)
		}
	}
}

func TestCodegraphInjectsPathAfterSubcommand(t *testing.T) {
	got := injectCodegraphPath([]string{"--json", "context", "fix login bug", "--summary"}, "~/repo")
	want := []string{"--json", "context", "fix login bug", "--summary", "--path", "~/repo"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestCodegraphDoesNotInjectPathForListOrExplicitTarget(t *testing.T) {
	for _, args := range [][]string{
		{"list"},
		{"--target", "demo", "overview"},
		{"overview", "--path", "~/repo"},
	} {
		got := injectCodegraphPath(args, "~/other")
		if strings.Join(got, "\x00") != strings.Join(args, "\x00") {
			t.Fatalf("unexpected injection for %#v: %#v", args, got)
		}
	}
}

func containsString(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && strings.Contains(s, sub))
}
