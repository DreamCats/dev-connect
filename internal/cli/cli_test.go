package cli

import "testing"

func TestNormalizeGlobalFlagsAllowsPostCommandJSON(t *testing.T) {
	got := normalizeGlobalFlags([]string{"repo-status", "--cwd", "~/repo", "--json"})
	want := []string{"--json", "repo-status", "--cwd", "~/repo"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestNormalizeGlobalFlagsSkipsExecCommandParts(t *testing.T) {
	got := normalizeGlobalFlags([]string{"exec", "echo --json"})
	if got[0] != "exec" || got[1] != "echo --json" {
		t.Fatalf("unexpected exec normalization: %#v", got)
	}
}

func TestShellJoin(t *testing.T) {
	got := shellJoin([]string{"go", "test", "./pkg with space"})
	want := "go test './pkg with space'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestExecHostFirstAllowsOptionsBeforeDoubleDash(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := runCommand(appConfig{}, "config", []string{"add", "sgdev", "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	workArgs := []string{"sgdev", "--cwd", "/repo", "--", "go", "test", "./..."}
	if !looksLikeExecHost(workArgs[0]) {
		t.Fatal("expected configured host to be recognized")
	}
	got := shellJoin(workArgs[4:])
	if got != "go test ./..." {
		t.Fatalf("unexpected shell join: %s", got)
	}
}
