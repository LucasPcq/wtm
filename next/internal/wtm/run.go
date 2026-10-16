package wtm

import "strings"

type RunConfig struct {
	Schema           int             `toml:"schema,omitempty"`
	PortOffsetBlock  int             `toml:"port_offset_block,omitempty"`
	PortProbeTimeout int             `toml:"port_probe_timeout,omitempty"`
	Jobs             []JobConfig     `toml:"job"`
	Profiles         []ProfileConfig `toml:"profile,omitempty"`
	EnvPorts         []EnvPortLink   `toml:"env_port,omitempty"`
	EnvValues        []EnvValueLink  `toml:"env,omitempty"`
	Addressing       Addressing      `toml:"addressing,omitempty"`
	Concurrency      string          `toml:"concurrency,omitempty"`
	Isolation        Isolation       `toml:"isolation,omitempty"`
}

type JobKind string

const (
	JobKindService JobKind = "service"
	JobKindTask    JobKind = "task"
)

type JobScope string

const (
	JobScopePerWorktree JobScope = ""
	JobScopeShared      JobScope = "shared"
)

type JobConfig struct {
	Name        string              `toml:"name"`
	Kind        JobKind             `toml:"kind"`
	Cmd         string              `toml:"cmd"`
	Stop        string              `toml:"stop,omitempty"`
	Cwd         string              `toml:"cwd,omitempty"`
	Ports       map[string]int      `toml:"ports,omitempty"`
	URL         *JobURLConfig       `toml:"url,omitempty"`
	Probe       *bool               `toml:"probe,omitempty"`
	BindsNoPort bool                `toml:"binds_no_port,omitempty"`
	Runs        []string            `toml:"runs,omitempty"`
	Touches     []string            `toml:"touches,omitempty"`
	Scope       JobScope            `toml:"scope,omitempty"`
	Namespace   *JobNamespaceConfig `toml:"namespace,omitempty"`
}

type JobURLConfig struct {
	Port string `toml:"port"`
	Host string `toml:"host,omitempty"`
}

type JobNamespaceConfig struct {
	Name   string            `toml:"name"`
	Create string            `toml:"create,omitempty"`
	Remove string            `toml:"remove,omitempty"`
	Env    map[string]string `toml:"env,omitempty"`
}

type ProfileConfig struct {
	Name    string   `toml:"name"`
	Jobs    []string `toml:"jobs"`
	Default bool     `toml:"default"`
}

type EnvPortLink struct {
	File string `toml:"file"`
	Key  string `toml:"key"`
	Job  string `toml:"job"`
	Port string `toml:"port"`
}

type EnvValueLink struct {
	File  string `toml:"file"`
	Key   string `toml:"key"`
	Job   string `toml:"job"`
	Value string `toml:"value"`
}

type Addressing string

const (
	AddressingPorts Addressing = "ports"
	AddressingNames Addressing = "names"
)

func (job JobConfig) HasNamespace() bool {
	return job.Namespace != nil && job.Namespace.Name != "" && job.Namespace.Create != ""
}

func (job JobConfig) RunsCompose() bool {
	for _, cmd := range []string{job.Cmd, job.Stop} {
		if strings.Contains(cmd, "docker compose") || strings.Contains(cmd, "docker-compose") {
			return true
		}
	}
	return false
}
