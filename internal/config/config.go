package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DreamCats/dev-connect/internal/model"
	"gopkg.in/yaml.v3"
)

func ConfigDir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "dev-connect")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dev-connect")
}

func ConfigFile() string { return filepath.Join(ConfigDir(), "config.yaml") }

func Load() (model.AppConfig, error) {
	cfg := model.AppConfig{Hosts: map[string]model.HostConfig{}}
	data, err := os.ReadFile(ConfigFile())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]model.HostConfig{}
	}
	for alias, host := range cfg.Hosts {
		if host.User == "" {
			host.User = "maifeng"
			cfg.Hosts[alias] = host
		}
	}
	return cfg, nil
}

func Save(cfg model.AppConfig) error {
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]model.HostConfig{}
	}
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(yamlPayload(cfg))
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile(), data, 0o644)
}

func yamlPayload(cfg model.AppConfig) map[string]any {
	hosts := map[string]any{}
	for alias, host := range cfg.Hosts {
		repoRoots := host.RepoRoots
		if repoRoots == nil {
			repoRoots = []string{}
		}
		var shell any
		if host.Shell != "" {
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
	return map[string]any{"default_host": cfg.DefaultHost, "hosts": hosts}
}

func GetHost(hostAlias string) (model.HostConfig, error) {
	cfg, err := Load()
	if err != nil {
		return model.HostConfig{}, err
	}
	alias := strings.TrimPrefix(hostAlias, "@")
	if alias == "" {
		alias = cfg.DefaultHost
	}
	if alias == "" {
		return model.HostConfig{}, fmt.Errorf("未指定主机且未配置默认主机，请使用 @alias 或设置 default_host")
	}
	host, ok := cfg.Hosts[alias]
	if !ok {
		return model.HostConfig{}, fmt.Errorf("主机 '%s' 未在配置中找到，可用主机: %v", alias, keys(cfg.Hosts))
	}
	if host.User == "" {
		host.User = "maifeng"
	}
	return host, nil
}

func keys(m map[string]model.HostConfig) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func NormalizeLocalHomeToTilde(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
