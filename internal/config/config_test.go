package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DreamCats/dev-connect/internal/model"
)

func TestLoadSaveAndGetHost(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultHost = "sgdev"
	timeout := 42
	cfg.Hosts["sgdev"] = model.HostConfig{
		Hostname:    "10.0.0.1",
		User:        "maifeng",
		Shell:       "zsh",
		ExecTimeout: &timeout,
		RepoRoots:   []string{"/repo"},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	host, err := GetHost("@sgdev")
	if err != nil {
		t.Fatal(err)
	}
	if host.Hostname != "10.0.0.1" || host.User != "maifeng" || host.ExecTimeout == nil || *host.ExecTimeout != 42 {
		t.Fatalf("unexpected host: %#v", host)
	}
}

func TestSaveKeepsPythonLikeEmptyFields(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Hosts["sgdev"] = model.HostConfig{Hostname: "10.0.0.1", User: "maifeng"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"shell: null", "exec_timeout: null", "repo_roots: []"} {
		if !strings.Contains(text, want) {
			t.Fatalf("config yaml missing %q:\n%s", want, text)
		}
	}
}
