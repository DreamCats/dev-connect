package model

const Version = "0.1.0"

type HostConfig struct {
	Hostname    string   `yaml:"hostname" json:"hostname"`
	User        string   `yaml:"user" json:"user"`
	Shell       string   `yaml:"shell,omitempty" json:"shell,omitempty"`
	ExecTimeout *int     `yaml:"exec_timeout,omitempty" json:"exec_timeout,omitempty"`
	RepoRoots   []string `yaml:"repo_roots,omitempty" json:"repo_roots,omitempty"`
}

type AppConfig struct {
	DefaultHost string                `yaml:"default_host" json:"default_host"`
	Hosts       map[string]HostConfig `yaml:"hosts" json:"hosts"`
}
